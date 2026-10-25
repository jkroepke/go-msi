package main

// Builds test packages through the native importer (MsiDatabaseImportW).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type tbl struct {
	Name string
	Cols []string // "Name type", e.g. "Property s72"
	Keys []string
	Rows [][]string // "" = NULL
}

var schema = map[string]tbl{
	"Property":               {Cols: []string{"Property s72", "Value l0"}, Keys: []string{"Property"}},
	"Directory":              {Cols: []string{"Directory s72", "Directory_Parent S72", "DefaultDir l255"}, Keys: []string{"Directory"}},
	"Component":              {Cols: []string{"Component s72", "ComponentId S38", "Directory_ s72", "Attributes i2", "Condition S255", "KeyPath S72"}, Keys: []string{"Component"}},
	"File":                   {Cols: []string{"File s72", "Component_ s72", "FileName l255", "FileSize i4", "Version S72", "Language S20", "Attributes I2", "Sequence i4"}, Keys: []string{"File"}},
	"Feature":                {Cols: []string{"Feature s38", "Feature_Parent S38", "Title L64", "Description L255", "Display I2", "Level i2", "Directory_ S72", "Attributes i2"}, Keys: []string{"Feature"}},
	"FeatureComponents":      {Cols: []string{"Feature_ s38", "Component_ s72"}, Keys: []string{"Feature_", "Component_"}},
	"Media":                  {Cols: []string{"DiskId i2", "LastSequence i4", "DiskPrompt L64", "Cabinet S255", "VolumeLabel S32", "Source S72"}, Keys: []string{"DiskId"}},
	"InstallExecuteSequence": {Cols: []string{"Action s72", "Condition S255", "Sequence I2"}, Keys: []string{"Action"}},
	"Upgrade":                {Cols: []string{"UpgradeCode s38", "VersionMin S20", "VersionMax S20", "Language S255", "Attributes i4", "Remove S255", "ActionProperty s72"}, Keys: []string{"UpgradeCode", "VersionMin", "VersionMax", "Language", "Attributes"}},
	"LaunchCondition":        {Cols: []string{"Condition s255", "Description l255"}, Keys: []string{"Condition"}},
	"Registry":               {Cols: []string{"Registry s72", "Root i2", "Key l255", "Name L255", "Value L0", "Component_ s72"}, Keys: []string{"Registry"}},
	"ServiceInstall":         {Cols: []string{"ServiceInstall s72", "Name s255", "DisplayName L255", "ServiceType i4", "StartType i4", "ErrorControl i4", "LoadOrderGroup S255", "Dependencies S255", "StartName S255", "Password S255", "Arguments S255", "Component_ s72", "Description L255"}, Keys: []string{"ServiceInstall"}},
	"ServiceControl":         {Cols: []string{"ServiceControl s72", "Name l255", "Event i2", "Arguments L255", "Wait I2", "Component_ s72"}, Keys: []string{"ServiceControl"}},
	"CustomAction":           {Cols: []string{"Action s72", "Type i2", "Source S72", "Target S255", "ExtendedType I4"}, Keys: []string{"Action"}},
	"Environment":            {Cols: []string{"Environment s72", "Name l255", "Value L255", "Component_ s72"}, Keys: []string{"Environment"}},
	"MsiFileHash":            {Cols: []string{"File_ s72", "Options i2", "HashPart1 i4", "HashPart2 i4", "HashPart3 i4", "HashPart4 i4"}, Keys: []string{"File_"}},
	"AppSearch":              {Cols: []string{"Property s72", "Signature_ s72"}, Keys: []string{"Property", "Signature_"}},
	"RegLocator":             {Cols: []string{"Signature_ s72", "Root i2", "Key s255", "Name S255", "Type I2"}, Keys: []string{"Signature_"}},
	"RemoveFile":             {Cols: []string{"FileKey s72", "Component_ s72", "FileName L255", "DirProperty s72", "InstallMode i2"}, Keys: []string{"FileKey"}},
	"CreateFolder":           {Cols: []string{"Directory_ s72", "Component_ s72"}, Keys: []string{"Directory_", "Component_"}},
}

type payload struct {
	Key  string
	Name string // long name in package
	Data []byte
}

type pkg struct {
	Name           string
	ProductCode    string
	UpgradeCode    string
	PackageCode    string
	Version        string
	Template       string
	PageCount      int32
	WordCount      int32
	Is64           bool
	InstallDirName string
	LongOnlyDir    bool // write DefaultDir without a short name part (invalid 8.3)
	PerUser        bool // leave ALLUSERS out of the Property table
	Files          []payload
	Extra          map[string][][]string // extra rows per table
	ExtraProps     [][]string
	Sequence       [][]string // overrides default execute sequence when set
	ExtraSequence  [][]string
	CabMode        string // "makecab-mszip" (default), "makecab-none", "go-mszip", "go-stored", "go-nodict"
	CabOverride    []byte
	SkipCab        bool
	CompAttr       map[string]int // extra Component attribute bits per file key
}

func newGUID() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "[guid]::NewGuid().ToString('B').ToUpper()").Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(out))
}

var defaultSeq = [][]string{
	{"FindRelatedProducts", "", "25"},
	{"LaunchConditions", "", "100"},
	{"ValidateProductID", "", "700"},
	{"CostInitialize", "", "800"},
	{"FileCost", "", "900"},
	{"CostFinalize", "", "1000"},
	{"InstallValidate", "", "1400"},
	{"InstallInitialize", "", "1500"},
	{"ProcessComponents", "", "1600"},
	{"UnpublishFeatures", "", "1800"},
	{"StopServices", "VersionNT", "1900"},
	{"DeleteServices", "VersionNT", "2000"},
	{"RemoveRegistryValues", "", "2600"},
	{"RemoveFiles", "", "3500"},
	{"RemoveFolders", "", "3600"},
	{"CreateFolders", "", "3700"},
	{"InstallFiles", "", "4000"},
	{"WriteRegistryValues", "", "5000"},
	{"WriteEnvironmentStrings", "", "5200"},
	{"InstallServices", "VersionNT", "5800"},
	{"StartServices", "VersionNT", "5900"},
	{"RegisterUser", "", "6000"},
	{"RegisterProduct", "", "6100"},
	{"PublishFeatures", "", "6300"},
	{"PublishProduct", "", "6400"},
	{"InstallFinalize", "", "6600"},
}

func idtEscape(s string) string {
	s = strings.ReplaceAll(s, "\t", "\x10")
	s = strings.ReplaceAll(s, "\r", "\x11")
	s = strings.ReplaceAll(s, "\n", "\x19")
	return s
}

func writeIDT(dir string, t tbl) (string, error) {
	var b strings.Builder
	var names, types []string
	for _, c := range t.Cols {
		p := strings.SplitN(c, " ", 2)
		names = append(names, p[0])
		types = append(types, p[1])
	}
	b.WriteString(strings.Join(names, "\t") + "\r\n")
	b.WriteString(strings.Join(types, "\t") + "\r\n")
	b.WriteString(t.Name + "\t" + strings.Join(t.Keys, "\t") + "\r\n")
	for _, r := range t.Rows {
		var cells []string
		for _, c := range r {
			cells = append(cells, idtEscape(c))
		}
		b.WriteString(strings.Join(cells, "\t") + "\r\n")
	}
	name := t.Name + ".idt"
	out := []byte(b.String())
	if forcedCodepage > 0 && forcedCodepage != 65001 {
		enc, err := cpEncode(b.String(), forcedCodepage)
		if err != nil {
			return "", err
		}
		out = enc
	}
	return name, os.WriteFile(filepath.Join(dir, name), out, 0o644)
}

func importTable(db uint32, dir string, t tbl) error {
	f, err := writeIDT(dir, t)
	if err != nil {
		return err
	}
	return importIDT(db, dir, f)
}

func fixedTime() [2]uint32 {
	// 2026-01-01T00:00:00Z
	ft := uint64(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()+11644473600) * 10000000
	return [2]uint32{uint32(ft), uint32(ft >> 32)}
}

func (p *pkg) defaults() {
	if p.ProductCode == "" {
		p.ProductCode = newGUID()
	}
	if p.UpgradeCode == "" {
		p.UpgradeCode = "{5E7A1C2B-0D3F-4A6E-9B8C-7F1E2D3C4B5A}"
	}
	if p.PackageCode == "" {
		p.PackageCode = newGUID()
	}
	if p.Version == "" {
		p.Version = "1.0.0"
	}
	if p.Template == "" {
		if p.Is64 {
			p.Template = "x64;1033"
		} else {
			p.Template = "Intel;1033"
		}
	}
	if p.PageCount == 0 {
		p.PageCount = 200
	}
	if p.WordCount == 0 {
		p.WordCount = 2
	}
	if p.InstallDirName == "" {
		p.InstallDirName = "MsiSpecTest"
	}
	if p.Files == nil {
		p.Files = []payload{{Key: "f_readme", Name: "readme.txt", Data: []byte("Hello from a clean-room MSI.\r\n")}}
	}
	if p.CabMode == "" {
		p.CabMode = "makecab-mszip"
	}
}

func makecab(dir string, files []payload, compress string) ([]byte, error) {
	src := filepath.Join(dir, "cabsrc")
	os.MkdirAll(src, 0o755)
	var ddf strings.Builder
	ddf.WriteString(".Set CabinetNameTemplate=cab1.cab\r\n.Set DiskDirectoryTemplate=" + dir + "\r\n")
	ddf.WriteString(".Set CompressionType=" + compress + "\r\n.Set Cabinet=on\r\n.Set Compress=on\r\n.Set MaxDiskSize=0\r\n.Set RptFileName=nul\r\n.Set InfFileName=nul\r\n")
	if compress == "NONE" {
		ddf.Reset()
		ddf.WriteString(".Set CabinetNameTemplate=cab1.cab\r\n.Set DiskDirectoryTemplate=" + dir + "\r\n.Set Cabinet=on\r\n.Set Compress=off\r\n.Set MaxDiskSize=0\r\n.Set RptFileName=nul\r\n.Set InfFileName=nul\r\n")
	}
	for _, f := range files {
		fp := filepath.Join(src, f.Key)
		if err := os.WriteFile(fp, f.Data, 0o644); err != nil {
			return nil, err
		}
		ddf.WriteString("\"" + fp + "\" \"" + f.Key + "\"\r\n")
	}
	ddfPath := filepath.Join(dir, "cab.ddf")
	os.WriteFile(ddfPath, []byte(ddf.String()), 0o644)
	cmd := exec.Command("makecab.exe", "/F", ddfPath)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("makecab: %v: %s", err, out)
	}
	return os.ReadFile(filepath.Join(dir, "cab1.cab"))
}

func buildCab(p *pkg, dir string) ([]byte, error) {
	if p.CabOverride != nil {
		return p.CabOverride, nil
	}
	switch p.CabMode {
	case "makecab-mszip":
		return makecab(dir, p.Files, "MSZIP")
	case "makecab-none":
		return makecab(dir, p.Files, "NONE")
	}
	var cf []cabFile
	for _, f := range p.Files {
		cf = append(cf, cabFile{Name: f.Key, Data: f.Data, Date: 0x0021, Attrib: 0x20})
	}
	switch p.CabMode {
	case "go-mszip":
		return writeCab(cf, cabOpts{Compress: 1})
	case "go-stored":
		return writeCab(cf, cabOpts{Compress: 0})
	case "go-nodict":
		return writeCab(cf, cabOpts{Compress: 1, NoDictCarry: true})
	case "go-wrongdict":
		return writeCab(cf, cabOpts{Compress: 1, WrongDict: true})
	}
	return nil, fmt.Errorf("unknown cab mode %s", p.CabMode)
}

// build writes the package to out and returns the work dir.
func (p *pkg) build(out string) error {
	p.defaults()
	dir, err := os.MkdirTemp("", "msib-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	os.Remove(out)
	db, err := openDB(out, dbCreate)
	if err != nil {
		return err
	}
	defer closeHandle(db)

	pf := "ProgramFilesFolder"
	compAttr := "0"
	if p.Is64 {
		pf = "ProgramFiles64Folder"
		compAttr = "256"
	}
	tables := map[string][][]string{}
	tables["Property"] = append([][]string{
		{"ProductCode", p.ProductCode},
		{"ProductName", "MsiSpec Test " + p.Name},
		{"ProductVersion", p.Version},
		{"Manufacturer", "msitest"},
		{"ProductLanguage", "1033"},
		{"UpgradeCode", p.UpgradeCode},
	}, p.ExtraProps...)
	if !p.PerUser {
		tables["Property"] = append(tables["Property"], []string{"ALLUSERS", "1"})
	}
	tables["Directory"] = [][]string{
		{"TARGETDIR", "", "SourceDir"},
		{pf, "TARGETDIR", "."},
		{"INSTALLDIR", pf, p.defaultDir()},
	}
	tables["Feature"] = [][]string{{"Main", "", "Main", "", "1", "1", "INSTALLDIR", "0"}}
	seq := 0
	for _, f := range p.Files {
		seq++
		ck := "C_" + f.Key
		attr, _ := strconv.Atoi(compAttr)
		tables["Component"] = append(tables["Component"], []string{ck, guidFor(p.Name + f.Key + p.UpgradeCode), "INSTALLDIR", strconv.Itoa(attr | p.CompAttr[f.Key]), "", f.Key})
		tables["File"] = append(tables["File"], []string{f.Key, ck, f.Name, fmt.Sprint(len(f.Data)), "", "", "0", fmt.Sprint(seq)})
		tables["FeatureComponents"] = append(tables["FeatureComponents"], []string{"Main", ck})
	}
	if !p.SkipCab {
		tables["Media"] = [][]string{{"1", fmt.Sprint(seq), "", "#cab1.cab", "", ""}}
	} else {
		tables["Media"] = [][]string{{"1", fmt.Sprint(seq), "", "", "", ""}}
	}
	if p.Sequence != nil {
		tables["InstallExecuteSequence"] = p.Sequence
	} else {
		tables["InstallExecuteSequence"] = append(append([][]string{}, defaultSeq...), p.ExtraSequence...)
	}
	for k, v := range p.Extra {
		tables[k] = append(tables[k], v...)
	}
	if forcedCodepage >= 0 {
		fc := filepath.Join(dir, "_ForceCodepage.idt")
		os.WriteFile(fc, []byte(fmt.Sprintf("\r\n\r\n%d\t_ForceCodepage\r\n", forcedCodepage)), 0o644)
		if err := importIDT(db, dir, "_ForceCodepage.idt"); err != nil {
			return err
		}
	}
	var names []string
	for k := range tables {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		t := schema[n]
		t.Name = n
		t.Rows = tables[n]
		if err := importTable(db, dir, t); err != nil {
			return err
		}
	}
	if !p.SkipCab {
		cab, err := buildCab(p, dir)
		if err != nil {
			return err
		}
		cp := filepath.Join(dir, "embedded.cab")
		os.WriteFile(cp, cab, 0o644)
		if err := insertStream(db, "_Streams", "cab1.cab", cp); err != nil {
			return err
		}
	}
	si, err := openSummary(db, "", 20)
	if err != nil {
		return err
	}
	ft := fixedTime()
	props := []sumProp{
		{PID: 1, VT: vtI2, Int: summaryCP()},
		{PID: 2, VT: vtLPSTR, Text: "Installation Database"},
		{PID: 3, VT: vtLPSTR, Text: "MsiSpec Test " + p.Name},
		{PID: 4, VT: vtLPSTR, Text: "msitest"},
		{PID: 7, VT: vtLPSTR, Text: p.Template},
		{PID: 9, VT: vtLPSTR, Text: p.PackageCode},
		{PID: 12, VT: vtFILETIME, FT: ft},
		{PID: 13, VT: vtFILETIME, FT: ft},
		{PID: 14, VT: vtI4, Int: p.PageCount},
		{PID: 15, VT: vtI4, Int: max(p.WordCount, 0)},
		{PID: 18, VT: vtLPSTR, Text: "msitest"},
		{PID: 19, VT: vtI4, Int: 2},
	}
	for _, sp := range props {
		if err := setSummary(si, sp); err != nil {
			closeHandle(si)
			return err
		}
	}
	if err := persistSummary(si); err != nil {
		closeHandle(si)
		return err
	}
	closeHandle(si)
	return commit(db)
}

func guidFor(s string) string {
	// deterministic braced uppercase GUID from a string (MD5-based, v3 layout)
	h := md5sum([]byte(s))
	h[6] = h[6]&0x0F | 0x30
	h[8] = h[8]&0x3F | 0x80
	return strings.ToUpper(fmt.Sprintf("{%x-%x-%x-%x-%x}", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16]))
}

func (p *pkg) defaultDir() string {
	if p.LongOnlyDir || len(p.InstallDirName) <= 8 {
		return p.InstallDirName
	}
	return "msispect|" + p.InstallDirName
}

// forcedCodepage, when >= 0, is imported as _ForceCodepage before any table.
var forcedCodepage = -1

func summaryCP() int32 {
	switch forcedCodepage {
	case 874, 932, 936, 949, 950, 1250, 1251, 1252, 1253, 1254, 1255, 1256, 1257, 1258:
		return int32(forcedCodepage)
	}
	return 1252
}
