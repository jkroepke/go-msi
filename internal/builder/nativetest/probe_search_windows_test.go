//go:build windows

package nativetest_test

// Temporary probe for SEARCH-05: how AppSearch treats DrLocator, CompLocator,
// file signatures and RegLocator file searches. It logs what each property
// holds and fails on purpose so that CI prints the log. Not for merging.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/pe"
	"github.com/jkroepke/msi-builder/internal/schema"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

const probeBaseGUID = "{6A0D3C2E-5B1F-4E7A-9C3D-2F8E1B4A7C60}"

const probeBaseConfig = `schema: msi-builder/v1
product: { name: MsiBuilderProbeBase, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f }
package: { platform: x64 }
installDir: { name: MsiBuilderProbeBase }
components:
  - id: BaseExe
    guid: 6A0D3C2E-5B1F-4E7A-9C3D-2F8E1B4A7C60
    files: [{ source: base.exe }]
`

const probeConfig = `schema: msi-builder/v1
product: { name: MsiBuilderProbe, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 2d3e4f5a-6b7c-4d8e-9fa0-1b2c3d4e5f60 }
package: { platform: x64 }
installDir: { name: MsiBuilderProbe }
components:
  - files: [{ source: app.exe }]
properties:
  P_REGDIR: { search: { registry: { root: HKLM, key: 'SOFTWARE\MsiBuilderProbe', name: Dir, type: directory } } }
`

func probeBuild(t *testing.T, dir, yaml string, files map[string][]byte) []byte {
	t.Helper()

	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(yaml), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	return res.Package
}

func dosDate(tm time.Time) int {
	return ((tm.Year()-1980)*512+int(tm.Month())*32+tm.Day())*65536 + tm.Hour()*2048 + tm.Minute()*32 + tm.Second()/2
}

func TestProbeSearchLocators(t *testing.T) { //nolint:paralleltest,maintidx // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	root := `C:\MsiBuilderProbe`
	work := t.TempDir()

	ctx := context.Background()
	cleanup := func() {
		_ = os.RemoveAll(root)
		_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\MsiBuilderProbe`, "/f", "/reg:64").Run()
	}

	cleanup()
	t.Cleanup(cleanup)

	appExe := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}})
	neutral := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{0}})
	nolang := testpe.Build(testpe.Options{FileVersion: "1.0.0.0"})

	for name, data := range map[string][]byte{
		`sub\app.exe`:             appExe,
		`neutral.exe`:             neutral,
		`nolang.exe`:              nolang,
		`deep\a\b\deep.txt`:       []byte("deep"),
		`plain.txt`:               []byte("x"),
		`long file name here.txt`: []byte("long"),
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	stat, err := os.Stat(filepath.Join(root, `sub\app.exe`))
	if err != nil {
		t.Fatal(err)
	}

	appSize := int(stat.Size())
	t.Logf("PROBE app.exe size %d modtime local %s utc %s", appSize, stat.ModTime().Local(), stat.ModTime().UTC())

	for _, args := range [][]string{
		{"/v", "Dir", "/d", root},
		{"/v", "File", "/d", root + `\sub\app.exe`},
		{"/v", "DirSlash", "/d", root + `\`},
	} {
		//nolint:gosec // test-controlled arguments
		if out, err := exec.CommandContext(t.Context(), "reg.exe", append([]string{"add", `HKLM\SOFTWARE\MsiBuilderProbe`, "/f", "/reg:64"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("reg add %v: %v\n%s", args, err, out)
		}
	}

	// The package that owns the component the CompLocator probes find.
	baseDir := filepath.Join(work, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	basePkg := probeBuild(t, baseDir, probeBaseConfig, map[string][]byte{
		"base.exe": testpe.Build(testpe.Options{FileVersion: "3.0.0.0", Languages: []int{1033}}),
	})
	basePath := filepath.Join(baseDir, "base.msi")

	if err := os.WriteFile(basePath, basePkg, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = exec.CommandContext(ctx, "msiexec", "/x", basePath, "/qn").Run() })

	if code := msiexecCode(t, filepath.Join(baseDir, "install.log"), "/i", basePath); code != 0 {
		t.Fatalf("base install returned %d", code)
	}

	probeDir := filepath.Join(work, "probe")
	if err := os.MkdirAll(probeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pkg := probeBuild(t, probeDir, probeConfig, map[string][]byte{"app.exe": appExe})

	db, err := msidb.Unmarshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	tables := map[string]*msidb.Table{}
	for _, name := range []string{"AppSearch", "Signature", "RegLocator", "DrLocator", "CompLocator"} {
		tbl := db.Table(name)
		if tbl == nil {
			tbl = schema.New(name)
			db.Tables = append(db.Tables, tbl)
		}

		tables[name] = tbl
	}

	var props []string

	search := func(prop, sig string) {
		props = append(props, prop)
		tables["AppSearch"].Add(prop, sig)
	}
	signature := func(sig, name string, cells ...any) {
		row := []any{sig, name, nil, nil, nil, nil, nil, nil, nil}
		copy(row[2:], cells)
		tables["Signature"].Add(row...)
	}
	dr := func(sig string, parent, path any, depth any) { tables["DrLocator"].Add(sig, parent, path, depth) }

	sub := root + `\sub`
	now := stat.ModTime()

	// Directory searches.
	dr("S_DIR_ABS", nil, root, nil)
	search("P_DIR_ABS", "S_DIR_ABS")
	dr("S_DIR_SLASH", nil, root+`\`, nil)
	search("P_DIR_SLASH", "S_DIR_SLASH")
	dr("S_DIR_PROP", nil, `[WindowsFolder]System32`, 0)
	search("P_DIR_PROP", "S_DIR_PROP")
	dr("S_DIR_MISSING", nil, `C:\MsiBuilderNoSuchFolder`, nil)
	search("P_DIR_MISSING", "S_DIR_MISSING")
	dr("S_DIR_FILE", nil, root+`\plain.txt`, nil)
	search("P_DIR_FILE", "S_DIR_FILE")
	// A parent that has no AppSearch row of its own.
	tables["RegLocator"].Add("S_REGDIR_PARENT", 2, `SOFTWARE\MsiBuilderProbe`, "Dir", 0|16)
	dr("S_DIR_UNDER_REG", "S_REGDIR_PARENT", "sub", nil)
	search("P_DIR_UNDER_REG", "S_DIR_UNDER_REG")
	dr("S_DIR_UNDER_REG_MISS", "S_REGDIR_PARENT", "nosuch", nil)
	search("P_DIR_UNDER_REG_MISS", "S_DIR_UNDER_REG_MISS")
	dr("S_DIR_PARENT", nil, root, nil)
	dr("S_DIR_UNDER_DIR_DEPTH2", "S_DIR_PARENT", "b", 2)
	search("P_DIR_UNDER_DIR_DEPTH2", "S_DIR_UNDER_DIR_DEPTH2")
	dr("S_DIR_UNDER_DIR_DEPTH0", "S_DIR_PARENT", "b", 0)
	search("P_DIR_UNDER_DIR_DEPTH0", "S_DIR_UNDER_DIR_DEPTH0")
	dr("S_DIR_UNDER_DIR_MULTI", "S_DIR_PARENT", `deep\a`, 0)
	search("P_DIR_UNDER_DIR_MULTI", "S_DIR_UNDER_DIR_MULTI")
	dr("S_DIR_ABS_DEPTH", nil, `C:\MsiBuilderProbe\b`, 3)
	search("P_DIR_ABS_DEPTH", "S_DIR_ABS_DEPTH")

	// File searches in a folder.
	fileSearch := func(prop, name string, parent, path, depth any, cells ...any) {
		sig := "S_" + strings.TrimPrefix(prop, "P_")
		signature(sig, name, cells...)
		dr(sig, parent, path, depth)
		search(prop, sig)
	}
	fileSearch("P_FILE_ABS", "app.exe", nil, sub, 0)
	fileSearch("P_FILE_NODEPTH", "app.exe", nil, sub, nil)
	fileSearch("P_FILE_CASE", "APP.EXE", nil, sub, 0)
	fileSearch("P_FILE_DEPTH0_MISS", "app.exe", nil, root, 0)
	fileSearch("P_FILE_DEPTH1", "app.exe", nil, root, 1)
	fileSearch("P_FILE_DEEP3", "deep.txt", nil, root, 3)
	fileSearch("P_FILE_DEEP2", "deep.txt", nil, root, 2)
	fileSearch("P_FILE_LONG", "long file name here.txt", nil, root, 0)
	fileSearch("P_FILE_PATH_FILE", "app.exe", nil, sub+`\app.exe`, 0)
	fileSearch("P_FILE_MISSING", "nosuch.exe", nil, sub, 0)
	fileSearch("P_FILE_UNDER_REG", "app.exe", "S_REGDIR_PARENT", "sub", 0)
	fileSearch("P_FILE_UNDER_REG_DEPTH", "app.exe", "S_REGDIR_PARENT", nil, 1)
	fileSearch("P_FILE_UNDER_DIR", "deep.txt", "S_DIR_PARENT", `deep\a\b`, 0)

	// Version, size, date and language bounds.
	fileSearch("P_VER_MIN_LOW", "app.exe", nil, sub, 0, "1.0.0.0")
	fileSearch("P_VER_MIN_EQ_NOLANG", "app.exe", nil, sub, 0, "1.2.3.4")
	fileSearch("P_VER_MIN_EQ_1033", "app.exe", nil, sub, 0, "1.2.3.4", nil, nil, nil, nil, nil, "1033")
	fileSearch("P_VER_MIN_EQ_1031", "app.exe", nil, sub, 0, "1.2.3.4", nil, nil, nil, nil, nil, "1031")
	fileSearch("P_VER_MIN_LOW_1031", "app.exe", nil, sub, 0, "1.0.0.0", nil, nil, nil, nil, nil, "1031")
	fileSearch("P_VER_MIN_HIGH", "app.exe", nil, sub, 0, "1.2.3.5")
	fileSearch("P_VER_MIN_SHORT", "app.exe", nil, sub, 0, "1.2")
	fileSearch("P_VER_MAX_LOW", "app.exe", nil, sub, 0, nil, "1.2.3.3")
	fileSearch("P_VER_MAX_EQ", "app.exe", nil, sub, 0, nil, "1.2.3.4")
	fileSearch("P_VER_MAX_HIGH", "app.exe", nil, sub, 0, nil, "2.0.0.0")
	fileSearch("P_VER_UNVERSIONED", "plain.txt", nil, root, 0, "0.0.0.1")
	fileSearch("P_VER_NEUTRAL_EQ", "neutral.exe", nil, root, 0, "1.0.0.0")
	fileSearch("P_VER_NOLANG_EQ", "nolang.exe", nil, root, 0, "1.0.0.0")
	fileSearch("P_SIZE_MIN_OVER", "app.exe", nil, sub, 0, nil, nil, appSize+1)
	fileSearch("P_SIZE_MIN_EQ", "app.exe", nil, sub, 0, nil, nil, appSize)
	fileSearch("P_SIZE_MAX_EQ", "app.exe", nil, sub, 0, nil, nil, nil, appSize)
	fileSearch("P_SIZE_MAX_UNDER", "app.exe", nil, sub, 0, nil, nil, nil, appSize-1)
	fileSearch("P_DATE_MIN_PAST", "app.exe", nil, sub, 0, nil, nil, nil, nil, dosDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)))
	fileSearch("P_DATE_MIN_FUTURE", "app.exe", nil, sub, 0, nil, nil, nil, nil, dosDate(time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)))
	fileSearch("P_DATE_MAX_PAST", "app.exe", nil, sub, 0, nil, nil, nil, nil, nil, dosDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)))
	fileSearch("P_DATE_MAX_FUTURE", "app.exe", nil, sub, 0, nil, nil, nil, nil, nil, dosDate(time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)))
	// One hour around the modification time, in local and in UTC terms.
	fileSearch("P_DATE_MIN_UTC_MINUS1H", "app.exe", nil, sub, 0, nil, nil, nil, nil, dosDate(now.UTC().Add(-time.Hour)))
	fileSearch("P_DATE_MIN_UTC_PLUS1H", "app.exe", nil, sub, 0, nil, nil, nil, nil, dosDate(now.UTC().Add(time.Hour)))
	fileSearch("P_DATE_MIN_LOCAL_MINUS1H", "app.exe", nil, sub, 0, nil, nil, nil, nil, dosDate(now.Local().Add(-time.Hour)))

	// Versions of an operating system file (SEARCH-02, Windows 10+ shim).
	sys := filepath.Join(os.Getenv("SystemRoot"), "System32", "kernel32.dll")
	if f, err := os.Open(sys); err == nil {
		info, err := pe.Parse(f)
		_ = f.Close()

		if err == nil {
			t.Logf("PROBE kernel32.dll version %s languages %v", info.FileVersion, info.Languages)

			parts := strings.Split(info.FileVersion, ".")
			if len(parts) == 4 {
				fileSearch("P_OS_MIN_REAL_MAJOR", "kernel32.dll", nil, `[System64Folder]`, 0, parts[0]+".0.0.0")
				fileSearch("P_OS_MAX_63", "kernel32.dll", nil, `[System64Folder]`, 0, nil, "6.3.0.0")
				fileSearch("P_OS_MIN_REAL_BUILD", "kernel32.dll", nil, `[System64Folder]`, 0, parts[0]+"."+parts[1]+"."+parts[2]+".0")
			}
		}
	}

	// RegLocator file searches (Type 1): the value names a file.
	tables["RegLocator"].Add("S_REG_FILE", 2, `SOFTWARE\MsiBuilderProbe`, "File", 1|16)
	signature("S_REG_FILE", "app.exe")
	search("P_REG_FILE", "S_REG_FILE")
	tables["RegLocator"].Add("S_REG_FILE_WRONGNAME", 2, `SOFTWARE\MsiBuilderProbe`, "File", 1|16)
	signature("S_REG_FILE_WRONGNAME", "other.exe")
	search("P_REG_FILE_WRONGNAME", "S_REG_FILE_WRONGNAME")
	tables["RegLocator"].Add("S_REG_FILE_NOSIG", 2, `SOFTWARE\MsiBuilderProbe`, "File", 1|16)
	search("P_REG_FILE_NOSIG", "S_REG_FILE_NOSIG")
	tables["RegLocator"].Add("S_REG_DIRSIG", 2, `SOFTWARE\MsiBuilderProbe`, "Dir", 0|16)
	signature("S_REG_DIRSIG", "plain.txt")
	search("P_REG_DIRSIG", "S_REG_DIRSIG")
	tables["RegLocator"].Add("S_REG_FILE_MINVER", 2, `SOFTWARE\MsiBuilderProbe`, "File", 1|16)
	signature("S_REG_FILE_MINVER", "app.exe", "1.2.3.5")
	search("P_REG_FILE_MINVER", "S_REG_FILE_MINVER")
	tables["RegLocator"].Add("S_REG_DIRSLASH", 2, `SOFTWARE\MsiBuilderProbe`, "DirSlash", 0|16)
	search("P_REG_DIRSLASH", "S_REG_DIRSLASH")

	// CompLocator searches of the base package's component.
	comp := func(prop string, guid string, typ any, fileName string) {
		sig := "S_" + strings.TrimPrefix(prop, "P_")
		tables["CompLocator"].Add(sig, guid, typ)

		if fileName != "" {
			signature(sig, fileName)
		}

		search(prop, sig)
	}
	comp("P_COMP_T0", probeBaseGUID, 0, "")
	comp("P_COMP_T1", probeBaseGUID, 1, "")
	comp("P_COMP_TNULL", probeBaseGUID, nil, "")
	comp("P_COMP_T1_SIG", probeBaseGUID, 1, "base.exe")
	comp("P_COMP_T1_SIG_WRONG", probeBaseGUID, 1, "other.exe")
	comp("P_COMP_T0_SIG", probeBaseGUID, 0, "base.exe")
	comp("P_COMP_MISSING", "{00000000-1111-2222-3333-444444444444}", 1, "")
	comp("P_COMP_LOWER", strings.ToLower(probeBaseGUID), 1, "")
	tables["CompLocator"].Add("S_COMP_PARENT1", probeBaseGUID, 1)
	fileSearch("P_FILE_UNDER_COMP1", "base.exe", "S_COMP_PARENT1", nil, 0)
	tables["CompLocator"].Add("S_COMP_PARENT0", probeBaseGUID, 0)
	fileSearch("P_FILE_UNDER_COMP0", "base.exe", "S_COMP_PARENT0", nil, 0)

	// Several searches into one property: which one wins?
	dr("S_MULTI_A_MISS", nil, `C:\MsiBuilderNoSuchFolder`, nil)
	dr("S_MULTI_B_HIT", nil, root, nil)
	dr("S_MULTI_C_HIT", nil, sub, nil)
	search("P_MULTI_MISS_HIT", "S_MULTI_A_MISS")
	tables["AppSearch"].Add("P_MULTI_MISS_HIT", "S_MULTI_B_HIT")
	search("P_MULTI_HIT_HIT", "S_MULTI_B_HIT")
	tables["AppSearch"].Add("P_MULTI_HIT_HIT", "S_MULTI_C_HIT")
	search("P_MULTI_HIT_MISS", "S_MULTI_C_HIT")
	tables["AppSearch"].Add("P_MULTI_HIT_MISS", "S_MULTI_A_MISS")

	// The same signature in two locator tables: CompLocator first.
	tables["CompLocator"].Add("S_ORDER", "{00000000-1111-2222-3333-444444444444}", 1)
	dr("S_ORDER", nil, sub, nil)
	search("P_ORDER_COMPMISS_DR", "S_ORDER")

	// A property set in the Property table and found.
	db.Table("Property").Add("P_PRESET", "preset")
	dr("S_PRESET", nil, root, nil)
	search("P_PRESET", "S_PRESET")

	// SecureCustomProperties so the values reach the server.
	propTable := db.Table("Property")
	for i, row := range propTable.Rows {
		if row[0] == "SecureCustomProperties" {
			propTable.Rows[i][1] = fmt.Sprint(row[1]) + ";" + strings.Join(props, ";")
		}
	}

	var plain []*msidb.Table
	for _, tbl := range db.Tables {
		if tbl.Name != "_Validation" {
			plain = append(plain, tbl)
		}
	}

	db.Tables = append(plain, schema.Validation(plain))

	out, err := db.Marshal(msidb.Options{})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(probeDir, "probe.msi")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = exec.CommandContext(ctx, "msiexec", "/x", path, "/qn").Run() })

	log := filepath.Join(probeDir, "install.log")
	code := msiexecCode(t, log, "/i", path)
	text := readLog(log)
	t.Logf("PROBE install exit %d", code)

	server := serverProperties(text)

	slices.Sort(props)
	props = slices.Compact(props)

	for _, name := range props {
		value, ok := server[name]
		if !ok {
			value = "<unset>"
		}

		t.Logf("PROBE %s = %q", name, value)
	}

	for line := range strings.Lines(text) {
		if strings.Contains(line, "AppSearch") || strings.Contains(line, "Note:") || strings.Contains(line, "Error") {
			t.Logf("PROBE LOG %s", strings.TrimRight(line, "\r\n"))
		}
	}

	t.Error("probe finished; see the PROBE lines")
}
