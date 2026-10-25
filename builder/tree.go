package builder

import (
	"bytes"
	"crypto/md5" //nolint:gosec // MsiFileHash is defined as MD5
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/pe"
)

// Component attribute bits (section 8.4).
const (
	compSourceOnly        = 1
	compOptional          = 2
	compRegistryKeyPath   = 4
	compSharedDLLRefCount = 8
	compPermanent         = 16
	compTransitive        = 64
	compNeverOverwrite    = 128
	comp64Bit             = 256
)

// File attribute bits (section 11.5).
const (
	fileReadOnly = 0x1
	fileHidden   = 0x2
	fileSystem   = 0x4
	fileVital    = 0x200
	fileChecksum = 0x400
)

// standardFolderNames are the fixed 8.3 names of the standard folders
// (section 11.3, the reference profile).
var standardFolderNames = map[string]string{
	"AdminToolsFolder": "Admin", "AppDataFolder": "AppData", "CommonAppDataFolder": "CommApp",
	"CommonFilesFolder": "CFiles", "CommonFiles64Folder": "CFiles64", "DesktopFolder": "Desktop",
	"FavoritesFolder": "Favs", "FontsFolder": "Fonts", "LocalAppDataFolder": "LocalApp",
	"MyPicturesFolder": "Pictures", "NetHoodFolder": "NetHood", "PersonalFolder": "Personal",
	"PrintHoodFolder": "Printers", "ProgramFilesFolder": "PFiles", "ProgramFiles64Folder": "PFiles64",
	"ProgramMenuFolder": "PMenu", "RecentFolder": "Recent", "SendToFolder": "SendTo",
	"StartMenuFolder": "StrtMenu", "StartupFolder": "StartUp", "SystemFolder": "System",
	"System16Folder": "System16", "System64Folder": "System64", "TempFolder": "Temp",
	"TemplateFolder": "Template", "WindowsFolder": "Windows", "WindowsVolume": ".",
}

func (b *build) identity() error {
	var err error

	b.upgradeCode, err = config.NormalizeGUID(b.cfg.Product.UpgradeCode)
	if err != nil {
		return err
	}

	b.version, err = config.ParseVersion(b.cfg.Product.Version)
	if err != nil {
		return err
	}

	switch b.cfg.Product.ProductCode {
	case config.Auto:
		// Derived later, once every input is known (inputDigest).
	default:
		b.productCode, err = config.NormalizeGUID(b.cfg.Product.ProductCode)
		if err != nil {
			return err
		}
	}

	if b.cfg.Product.PackageCode != config.Auto {
		b.packageCode, err = config.NormalizeGUID(b.cfg.Product.PackageCode)
		if err != nil {
			return err
		}
	}

	return nil
}

// deriveProductCode computes productCode: auto from the upgrade code, the
// platform, the version and a digest of the configuration and every input
// file, so that changed content gives a new code (RUN-11) and unchanged
// content the same one.
func (b *build) deriveProductCode() error {
	if b.productCode != "" {
		return nil
	}

	cfgCopy := *b.cfg
	cfgCopy.BaseDir = ""

	y, err := yaml.Marshal(&cfgCopy)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(y)

	for _, f := range b.files {
		sum := sha256.Sum256(f.Data)
		h.Write([]byte(f.Key))
		h.Write(sum[:])
	}

	for _, name := range sortedStringKeys(b.cfg.Icons) {
		h.Write([]byte("icon:" + name))
	}

	digest := hex.EncodeToString(h.Sum(nil))
	b.productCode = uuid5(namespaceProduct, "product|"+b.upgradeCode+"|"+b.cfg.Product.Version+"|"+string(b.cfg.Package.Platform)+"|"+digest)

	return nil
}

func sortedStringKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

// directories builds the Directory table.
func (b *build) directories() error {
	b.addDir(&dirInfo{id: "TARGETDIR", canonical: "TARGETDIR", standard: true})

	var walk func(ds []*config.Directory, parent *dirInfo)

	walk = func(ds []*config.Directory, parent *dirInfo) {
		for _, d := range ds {
			if _, dup := b.dirs[d.ID]; dup {
				continue
			}

			info := &dirInfo{id: d.ID}

			switch {
			case parent == nil && config.IsStandardFolder(d.ID):
				info.parent = "TARGETDIR"
				info.standard = true
				info.canonical = d.ID
			case parent == nil:
				info.parent = "TARGETDIR"
				info.long = d.Name
				info.canonical = "TARGETDIR\\" + strings.ToLower(longPart(d.Name))
			default:
				info.parent = parent.id
				info.long = d.Name
				info.canonical = parent.canonical + "\\" + strings.ToLower(longPart(d.Name))
			}

			if d.Configurable {
				b.secure[d.ID] = true
			}

			b.addDir(info)
			walk(d.Children, info)
		}
	}

	walk(b.cfg.Directories, nil)

	return nil
}

func longPart(name string) string {
	if _, long, ok := strings.Cut(name, "|"); ok {
		return long
	}

	return name
}

func (b *build) addDir(d *dirInfo) {
	b.dirs[d.id] = d
	b.dirOrder = append(b.dirOrder, d)
}

// ensureStandardDir adds a standard folder row when an action needs it as a
// working directory.
func (b *build) ensureStandardDir(id string) {
	if _, ok := b.dirs[id]; ok {
		return
	}

	b.addDir(&dirInfo{id: id, parent: "TARGETDIR", standard: true, canonical: id})
}

// writeDirectoryTable emits the rows with generated short names. Every
// directory is written: the engine resolves the whole chain.
func (b *build) writeDirectoryTable() {
	t := b.table("Directory")
	taken := make(map[string]map[string]bool)

	for _, d := range b.dirOrder {
		switch {
		case d.id == "TARGETDIR":
			t.Add("TARGETDIR", nil, "SourceDir")
		case d.standard:
			name, ok := standardFolderNames[d.id]
			if !ok {
				name = "."
			}

			t.Add(d.id, d.parent, name)
		default:
			if taken[d.parent] == nil {
				taken[d.parent] = make(map[string]bool)
			}

			t.Add(d.id, d.parent, filename(d.parent, d.long, true, taken[d.parent], config.ValidShortName))
		}
	}
}

// writeComponents prepares the component records; rows are written after the
// files are read, because the key path and GUID may depend on them.
func (b *build) writeComponents() error {
	for i, c := range b.cfg.Components {
		info := &componentInfo{cfg: c, index: i, dir: b.dirs[c.Directory]}
		if info.dir == nil {
			return fmt.Errorf("components[%d]: directory %s is not defined", i, c.Directory)
		}

		info.dir.used = true

		switch c.Bitness {
		case "64":
			info.is64 = true
		case "32":
			info.is64 = false
		default:
			info.is64 = b.is64
		}

		if info.is64 {
			info.attributes |= comp64Bit
		}

		if c.Permanent {
			info.attributes |= compPermanent
		}

		if c.Transitive {
			info.attributes |= compTransitive
		}

		if c.NeverOverwrite {
			info.attributes |= compNeverOverwrite
		}

		if c.SharedDLLRefCount {
			info.attributes |= compSharedDLLRefCount
		}

		b.components[c.ID] = info
	}

	return nil
}

// inputs reads every payload file and derives version, language, hash and
// attributes, then writes the Component, File, MsiFileHash, CreateFolder,
// Registry, Environment and service tables.
func (b *build) inputs() error {
	for _, c := range b.cfg.Components {
		info := b.components[c.ID]
		p := fmt.Sprintf("components[%d]", info.index)

		for j, f := range c.Files {
			fp := fmt.Sprintf("%s.files[%d]", p, j)

			fi, err := b.readFile(fp, f, info)
			if err != nil {
				return err
			}

			if fi == nil {
				continue
			}

			if _, dup := b.fileByKey[fi.Key]; dup {
				b.errorf(fp+".id", "file key %s is used twice", fi.Key)

				continue
			}

			b.fileByKey[fi.Key] = fi
			b.files = append(b.files, fi)
		}
	}

	if err := b.diags.Err(); err != nil {
		return err
	}

	// Companion files: Version names another File key.
	for _, fi := range b.files {
		if v, ok := fi.Version.(string); ok && fi.cfg.Version != config.Auto && fi.cfg.Version != config.None {
			if _, err := config.ParseVersion(v); err != nil {
				if _, exists := b.fileByKey[v]; !exists {
					b.errorf(fmt.Sprintf("components[%d].files", fi.Component.index), "companion file %s is not defined", v)
				}
			}
		}
	}

	return b.diags.Err()
}

func (b *build) readFile(fp string, f *config.File, comp *componentInfo) (*fileInfo, error) {
	path := f.Source
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.cfg.BaseDir, filepath.FromSlash(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		b.errorf(fp+".source", "cannot read %s: %v", f.Source, err)

		return nil, nil //nolint:nilnil // reported as a diagnostic
	}

	if len(data) > 0x7FFFFFFF {
		b.errorf(fp+".source", "%s is larger than 2 GB", f.Source)

		return nil, nil //nolint:nilnil
	}

	fi := &fileInfo{
		Key:       config.FileKey(f),
		Component: comp,
		cfg:       f,
		path:      path,
		Data:      data,
		LongName:  config.FileLongName(f),
	}

	info, perr := pe.Parse(bytes.NewReader(data))
	fi.isPE = perr == nil

	switch f.Version {
	case config.Auto:
		if fi.isPE {
			if !info.HasVersion {
				if !f.AllowUnversionedExecutable {
					b.errorf(fp+".version", "%s is a PE file without a version resource; an upgrade can delete it (RUN-02). "+
						"Set allowUnversionedExecutable: true if this is intended", fi.LongName)
				}
			} else {
				fi.Version = info.FileVersion
			}

			if info.Machine == pe.MachineI386 && comp.is64 {
				b.warnf(fp+".source", "%s is a 32-bit executable in a 64-bit component", fi.LongName)
			}
		}
	case config.None:
	default:
		fi.Version = f.Version
	}

	switch f.Language {
	case config.Auto:
		if fi.isPE && info.HasVersion && len(info.Languages) > 0 {
			fi.Language = info.Language()
		}
	case config.None:
	default:
		fi.Language = f.Language
	}

	if fi.Version == nil {
		fi.Hash = fileHash(data)
	}

	if *f.Vital {
		fi.Attrs |= fileVital
	}

	if f.Checksum {
		if fi.isPE && info.HasChecksum {
			fi.Attrs |= fileChecksum
		} else {
			b.warnf(fp+".checksum", "%s has no PE checksum to verify; the checksum attribute is not set", fi.LongName)
		}
	}

	if f.ReadOnly {
		fi.Attrs |= fileReadOnly
	}

	if f.Hidden {
		fi.Attrs |= fileHidden
	}

	if f.System {
		fi.Attrs |= fileSystem
	}

	return fi, nil
}

// fileHash computes the MsiFileHash words: MD5 as four little-endian int32,
// zeros for an empty file (RUN-12).
func fileHash(data []byte) *[4]int32 {
	var out [4]int32

	if len(data) == 0 {
		return &out
	}

	sum := md5.Sum(data) //nolint:gosec // MsiFileHash is defined as MD5
	for i := range out {
		out[i] = int32(binary.LittleEndian.Uint32(sum[4*i:])) //nolint:gosec // the hash is stored as four signed words
	}

	return &out
}

// sequenceFiles orders the files (canonical directory, name, key), numbers
// them and writes the file-related tables and the Component table.
func (b *build) sequenceFiles() error {
	slices.SortStableFunc(b.files, func(a, c *fileInfo) int {
		if a.Component.dir.canonical != c.Component.dir.canonical {
			return strings.Compare(a.Component.dir.canonical, c.Component.dir.canonical)
		}

		if a.LongName != c.LongName {
			return strings.Compare(a.LongName, c.LongName)
		}

		return strings.Compare(a.Key, c.Key)
	})

	taken := make(map[string]map[string]bool)

	for i, fi := range b.files {
		fi.Sequence = i + 1

		dir := fi.Component.dir.id
		if taken[dir] == nil {
			taken[dir] = make(map[string]bool)
		}

		name := fi.cfg.Name
		if name == "" {
			name = fi.LongName
		}

		fi.FileName = filename(dir, name, false, taken[dir], config.ValidShortName)
	}

	if err := b.deriveProductCode(); err != nil {
		return err
	}

	b.writeDirectoryTable()

	// Components in config order; rows are sorted by msidb anyway.
	for _, c := range b.cfg.Components {
		info := b.components[c.ID]
		p := fmt.Sprintf("components[%d]", info.index)

		if err := b.resolveKeyPath(p, info); err != nil {
			return err
		}

		if err := b.componentGUID(p, info); err != nil {
			return err
		}

		b.table("Component").Add(c.ID, info.guid, c.Directory, info.attributes, nullable(c.Condition), info.keyPath)

		if info.keyPath == nil || c.CreateFolder {
			b.table("CreateFolder").Add(c.Directory, c.ID)
		}

		b.writeRegistry(info)
		b.writeEnvironment(info)
		b.writeServices(info)
	}

	for _, fi := range b.files {
		b.table("File").Add(fi.Key, fi.Component.cfg.ID, fi.FileName, len(fi.Data), fi.Version, fi.Language, fi.Attrs, fi.Sequence)

		if fi.Hash != nil {
			b.table("MsiFileHash").Add(fi.Key, 0, int(fi.Hash[0]), int(fi.Hash[1]), int(fi.Hash[2]), int(fi.Hash[3]))
		}
	}

	media := b.table("Media")
	if len(b.files) > 0 {
		media.Add(1, len(b.files), nil, "#"+b.cfg.Package.Cabinet, nil, nil)
	} else {
		media.Add(1, 0, nil, nil, nil, nil)
	}

	return nil
}

// resolveKeyPath picks the component's key path: the explicit one, else the
// first file, else the first registry value, else the directory.
func (b *build) resolveKeyPath(p string, info *componentInfo) error {
	c := info.cfg

	var files []*fileInfo

	for _, f := range b.files {
		if f.Component == info {
			files = append(files, f)
		}
	}

	byRef := func(ref string) *fileInfo {
		for _, f := range files {
			if f.Key == ref || f.LongName == ref {
				return f
			}
		}

		return nil
	}

	switch kp := c.KeyPath; {
	case kp == "directory":
		info.keyPath = nil
	case kp == "":
		if len(files) > 0 {
			info.keyFile = files[0]
		} else if len(c.Registry) > 0 {
			info.keyPath = b.registryID(info, c.Registry[0])
			info.attributes |= compRegistryKeyPath
		}
	case byRef(kp) != nil:
		info.keyFile = byRef(kp)
	default:
		for _, r := range c.Registry {
			if r.ID == kp {
				info.keyPath = kp
				info.attributes |= compRegistryKeyPath
			}
		}

		if info.keyPath == nil {
			return fmt.Errorf("%s.keyPath: %q is neither a file nor a registry id", p, kp)
		}
	}

	if info.keyFile != nil {
		info.keyPath = info.keyFile.Key

		// A companion file never is a key path (section 8.6).
		if v, ok := info.keyFile.Version.(string); ok {
			if _, err := config.ParseVersion(v); err != nil {
				b.errorf(p+".keyPath", "the key file %s is a companion file of %s", info.keyFile.Key, v)
			}
		}
	}

	if len(c.Services) > 0 && info.keyFile == nil {
		b.errorf(p+".services", "a component with services needs the service executable as its key file")
	}

	return nil
}

// componentGUID derives the component code: pinned, none, or UUIDv5 over the
// upgrade code, the bitness and the key path's canonical location.
func (b *build) componentGUID(p string, info *componentInfo) error {
	c := info.cfg

	switch c.GUID {
	case config.None:
		info.guid = nil

		return nil
	case config.Auto:
	default:
		g, err := config.NormalizeGUID(c.GUID)
		if err != nil {
			return fmt.Errorf("%s.guid: %w", p, err)
		}

		info.guid = g

		return nil
	}

	bitness := "32"
	if info.is64 {
		bitness = "64"
	}

	var location string

	switch {
	case info.keyFile != nil:
		location = "file|" + info.dir.canonical + "\\" + strings.ToLower(info.keyFile.LongName)
	case info.keyPath != nil:
		for _, r := range c.Registry {
			if b.registryID(info, r) == info.keyPath {
				location = "registry|" + strings.ToLower(r.Root+"\\"+r.Key+"\\"+r.Name)
			}
		}
	default:
		location = "directory|" + info.dir.canonical
	}

	info.guid = uuid5(namespaceComponent, b.upgradeCode+"|"+bitness+"|"+location)

	return nil
}

// features writes Feature and FeatureComponents.
func (b *build) features() error {
	t := b.table("Feature")
	fc := b.table("FeatureComponents")
	display := 0

	config.WalkFeatures(b.cfg.Features, func(f, parent *config.Feature) {
		var parentID any
		if parent != nil {
			parentID = parent.ID
		}

		display += 2

		var disp any

		switch f.Display {
		case "hidden":
			disp = 0
		case "expanded":
			disp = display + 1
		default:
			disp = display
		}

		display++

		attrs := 0

		if !*f.AllowAbsent {
			attrs |= 16 // UIDisallowAbsent
		}

		if !f.AllowAdvertise {
			attrs |= 8 // DisallowAdvertise
		}

		t.Add(f.ID, parentID, nullable(f.Title), nullable(f.Description), disp, *f.Level, nullable(f.Directory), attrs)

		for _, c := range f.Components {
			fc.Add(f.ID, c)
		}
	})

	return nil
}
