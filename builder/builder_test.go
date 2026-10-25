package builder_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/go-msi/builder"
	"github.com/jkroepke/go-msi/cab"
	"github.com/jkroepke/go-msi/cfb"
	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/internal/testpe"
	"github.com/jkroepke/go-msi/msidb"
	"github.com/jkroepke/go-msi/summary"
)

var buildTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// example copies examples/<name>/msi.yaml into a temporary directory next to
// the given input files and loads it.
func example(t *testing.T, name string, vars map[string]string, inputs map[string][]byte) *config.Config {
	t.Helper()

	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join("..", "examples", name, "msi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "msi.yaml"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	for n, data := range inputs {
		if err := os.WriteFile(filepath.Join(dir, n), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{Vars: vars})
	if err != nil {
		t.Fatal(err)
	}

	return cfg
}

func mustBuild(t *testing.T, cfg *config.Config) *builder.Result {
	t.Helper()

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	for _, d := range res.Diagnostics {
		t.Logf("diagnostic: %s: %s", d.Path, d.Message)
	}

	return res
}

func rows(t *testing.T, db *msidb.Database, table string) [][]any {
	t.Helper()

	tb := db.Table(table)
	if tb == nil {
		t.Fatalf("table %s is missing", table)
	}

	return tb.Rows
}

func cell(t *testing.T, db *msidb.Database, table, column string, match func([]any) bool) any {
	t.Helper()

	tb := db.Table(table)
	if tb == nil {
		t.Fatalf("table %s is missing", table)
	}

	col := tb.Column(column)
	if col < 0 {
		t.Fatalf("%s has no column %s", table, column)
	}

	for _, r := range tb.Rows {
		if match(r) {
			return r[col]
		}
	}

	t.Fatalf("%s: no matching row", table)

	return nil
}

func TestMinimal(t *testing.T) {
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	res := mustBuild(t, cfg)

	if len(res.Package) == 0 || res.Package[0] != 0xD0 {
		t.Fatal("no package written")
	}

	f, err := cfb.Read(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if f.CLSID != cfb.CLSIDMSI {
		t.Errorf("CLSID %x", f.CLSID)
	}

	db, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if len(db.Problems) > 0 {
		t.Errorf("problems: %v", db.Problems)
	}

	// File row: versioned x64 executable, short|long name, sequence 1.
	fr := rows(t, db, "File")
	if len(fr) != 1 {
		t.Fatalf("File rows: %d", len(fr))
	}

	if fr[0][0] != "hello.exe" || fr[0][4] != "1.0.0.0" || fr[0][5] != "1033" || fr[0][7] != 1 {
		t.Errorf("File row %v", fr[0])
	}

	if _, ok := fr[0][2].(string); !ok || fr[0][2] != "hello.exe" {
		t.Errorf("FileName %v, want the valid short name as is", fr[0][2])
	}

	if cell(t, db, "Media", "LastSequence", func([]any) bool { return true }) != 1 {
		t.Error("Media.LastSequence")
	}

	cabName := cell(t, db, "Media", "Cabinet", func([]any) bool { return true })
	if cabName != "#Hello.cab" {
		t.Errorf("Media.Cabinet %v", cabName)
	}

	data, ok := db.Stream("Hello.cab")
	if !ok {
		t.Fatal("cabinet stream missing")
	}

	entries, err := cab.Read(data)
	if err != nil || len(entries) != 1 || !bytes.Equal(entries[0].Data, exe) {
		t.Fatalf("cabinet: %v, %d entries", err, len(entries))
	}

	// Component: 64-bit attribute, key path is the file.
	attrs := cell(t, db, "Component", "Attributes", func(r []any) bool { return r[0] == "Hello" })
	if attrs != 256 {
		t.Errorf("Component.Attributes %v", attrs)
	}

	// Directory tree: TARGETDIR/SourceDir, ProgramFiles64Folder, INSTALLDIR.
	if cell(t, db, "Directory", "DefaultDir", func(r []any) bool { return r[0] == "TARGETDIR" }) != "SourceDir" {
		t.Error("TARGETDIR")
	}

	if cell(t, db, "Directory", "DefaultDir", func(r []any) bool { return r[0] == "ProgramFiles64Folder" }) != "PFiles64" {
		t.Error("ProgramFiles64Folder")
	}

	// Configurable directory → secure property.
	secure := cell(t, db, "Property", "Value", func(r []any) bool { return r[0] == "SecureCustomProperties" })
	if s := fmt.Sprint(secure); !strings.Contains(s, "INSTALLDIR") || !strings.Contains(s, "UPGRADE") {
		t.Errorf("SecureCustomProperties %v", secure)
	}

	if cell(t, db, "Property", "Value", func(r []any) bool { return r[0] == "ALLUSERS" }) != "1" {
		t.Error("ALLUSERS")
	}

	if cell(t, db, "Property", "Value", func(r []any) bool { return r[0] == "ProductCode" }) != res.ProductCode {
		t.Error("ProductCode")
	}

	// Sequences: the always set plus the file actions and the upgrade.
	ies := rows(t, db, "InstallExecuteSequence")
	want := map[string]int{
		"CostInitialize": 800, "InstallFiles": 4000, "RemoveFiles": 3500, "CreateFolders": 3700, "RemoveFolders": 3600,
		"InstallFinalize": 6600, "FindRelatedProducts": 25, "RemoveExistingProducts": 1501, "MigrateFeatureStates": 1200,
		"LaunchConditions": 100, "PublishProduct": 6400,
	}

	for _, r := range ies {
		if n, ok := want[fmt.Sprint(r[0])]; ok {
			if r[2] != n {
				t.Errorf("%s at %v, want %d", r[0], r[2], n)
			}

			delete(want, fmt.Sprint(r[0]))
		}
	}

	for name := range want {
		t.Errorf("%s is not scheduled", name)
	}

	for _, tbl := range []string{"InstallUISequence", "AdminExecuteSequence", "AdminUISequence", "AdvtExecuteSequence", "_Validation", "Upgrade", "LaunchCondition", "Feature", "FeatureComponents"} {
		if len(rows(t, db, tbl)) == 0 {
			t.Errorf("%s is empty", tbl)
		}
	}

	// Summary information.
	if summary.Get(db.Summary, summary.PIDTemplate) != "x64;1033" {
		t.Errorf("template %v", summary.Get(db.Summary, summary.PIDTemplate))
	}

	if summary.Get(db.Summary, summary.PIDPageCount) != 500 {
		t.Errorf("page count %v", summary.Get(db.Summary, summary.PIDPageCount))
	}

	if summary.Get(db.Summary, summary.PIDCodepage) != 1252 {
		t.Errorf("summary codepage %v", summary.Get(db.Summary, summary.PIDCodepage))
	}

	if summary.Get(db.Summary, summary.PIDRevision) != res.PackageCode || res.PackageCode == "" || res.PackageCode[0] != '{' {
		t.Errorf("package code %q / %v", res.PackageCode, summary.Get(db.Summary, summary.PIDRevision))
	}

	if ts, ok := summary.Get(db.Summary, summary.PIDCreated).(time.Time); !ok || !ts.Equal(buildTime) {
		t.Errorf("created %v", ts)
	}
}

func TestDeterministic(t *testing.T) {
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	a := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe}))
	b := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe}))

	if !bytes.Equal(a.Package, b.Package) {
		t.Error("two builds of the same inputs differ")
	}

	// Changed content gives a new product code and package code.
	exe2 := testpe.Build(testpe.Options{FileVersion: "1.0.0.1", Languages: []int{1033}})
	c := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe2}))

	if c.ProductCode == a.ProductCode {
		t.Error("product code did not change with the input")
	}

	if c.PackageCode == a.PackageCode {
		t.Error("package code did not change with the input")
	}
}

func TestCFBVersion3(t *testing.T) {
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime, CFBVersion: 3})
	if err != nil {
		t.Fatal(err)
	}

	if res.Package[0x1A] != 3 || len(res.Package)%512 != 0 {
		t.Errorf("not a v3 file: version %d, size %d", res.Package[0x1A], len(res.Package))
	}
}

func TestMissingInput(t *testing.T) {
	cfg := example(t, "minimal", nil, nil)

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil {
		t.Fatal("expected an error for the missing hello.exe")
	}

	if !strings.Contains(fmt.Sprint(res.Diagnostics), "hello.exe") {
		t.Errorf("diagnostics do not name the file: %s", res.Diagnostics)
	}
}

func TestWindowsExporter(t *testing.T) {
	exe := testpe.Build(testpe.Options{FileVersion: "0.32.0.5", Languages: []int{1033}})

	icon := append([]byte{0, 0, 1, 0, 1, 0}, bytes.Repeat([]byte{0xAB}, 64)...)

	cfg := example(t, "windows_exporter", map[string]string{"VERSION": "0.32.0.5"}, map[string][]byte{
		"windows_exporter.exe": exe,
		"icon.ico":             icon,
	})

	res := mustBuild(t, cfg)

	db, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if len(db.Problems) > 0 {
		t.Errorf("problems: %v", db.Problems)
	}

	// Services.
	if len(rows(t, db, "ServiceInstall")) != 1 || len(rows(t, db, "ServiceControl")) != 1 {
		t.Error("service rows")
	}

	if len(rows(t, db, "MsiServiceConfig")) != 2 {
		t.Errorf("MsiServiceConfig rows: %d", len(rows(t, db, "MsiServiceConfig")))
	}

	hidden := cell(t, db, "Property", "Value", func(r []any) bool { return r[0] == "MsiHiddenProperties" })
	if !strings.Contains(fmt.Sprint(hidden), "SERVICE_PASSWORD") {
		t.Errorf("MsiHiddenProperties %v", hidden)
	}

	// Event log registry rows.
	v := cell(t, db, "Registry", "Value", func(r []any) bool { return r[3] == "TypesSupported" })
	if v != "#7" {
		t.Errorf("TypesSupported %v", v)
	}

	// Icon table and ARP icon.
	if len(rows(t, db, "Icon")) != 1 {
		t.Error("Icon rows")
	}

	// Firewall and recovery actions exist, are deferred and sit inside the
	// InstallInitialize … InstallFinalize window.
	ies := rows(t, db, "InstallExecuteSequence")
	seqOf := map[string]int{}

	for _, r := range ies {
		n, ok := r[2].(int)
		if !ok {
			t.Fatalf("sequence %v is not an int", r[2])
		}

		seqOf[fmt.Sprint(r[0])] = n
	}

	fw := 0

	for _, r := range rows(t, db, "CustomAction") {
		name := fmt.Sprint(r[0])
		if !strings.HasPrefix(name, "Fw") && !strings.HasPrefix(name, "ServiceRecovery") {
			continue
		}

		fw++

		if typ, ok := r[1].(int); !ok || typ&1024 == 0 {
			t.Errorf("%s is not deferred: type %v", name, r[1])
		}

		n, ok := seqOf[name]
		if !ok || n <= seqOf["InstallInitialize"] || n >= seqOf["InstallFinalize"] {
			t.Errorf("%s at %d is outside the script window", name, n)
		}
	}

	if fw != 6 {
		t.Errorf("%d firewall/recovery actions, want 6", fw)
	}

	// REP after InstallInitialize (window 2) unless the example says otherwise.
	if rep := seqOf["RemoveExistingProducts"]; rep == 0 {
		t.Error("RemoveExistingProducts missing")
	}

	// Custom actions from the config are scheduled.
	for _, name := range []string{"SetMaintenance", "SetReinstallAll", "SetReinstallMode"} {
		if _, ok := seqOf[name]; !ok {
			t.Errorf("%s not scheduled", name)
		}
	}

	if summary.Get(db.Summary, summary.PIDTemplate) != "x64;1033" {
		t.Errorf("template %v", summary.Get(db.Summary, summary.PIDTemplate))
	}

	// The example asks for a wizard the builder does not have yet.
	if !strings.Contains(fmt.Sprint(res.Diagnostics), "wizard") {
		t.Error("expected a wizard warning")
	}
}
