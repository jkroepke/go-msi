package builder_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/go-msi/internal/testpe"
	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/cab"
	"github.com/jkroepke/go-msi/pkg/cfb"
	"github.com/jkroepke/go-msi/pkg/config"
	"github.com/jkroepke/go-msi/pkg/msidb"
	"github.com/jkroepke/go-msi/pkg/summary"
)

var buildTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) //nolint:gochecknoglobals // lookup table

// example copies examples/<name>/msi.yaml into a temporary directory next to
// the given input files and loads it.
func example(t *testing.T, name string, vars map[string]string, inputs map[string][]byte) *config.Config {
	t.Helper()

	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join("..", "..", "examples", name, "msi.yaml"))
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

	tbl := db.Table(table)
	if tbl == nil {
		t.Fatalf("table %s is missing", table)
	}

	col := tbl.Column(column)
	if col < 0 {
		t.Fatalf("%s has no column %s", table, column)
	}

	for _, r := range tbl.Rows {
		if match(r) {
			return r[col]
		}
	}

	t.Fatalf("%s: no matching row", table)

	return nil
}

func TestMinimal(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	res := mustBuild(t, cfg)

	if len(res.Package) == 0 || res.Package[0] != 0xD0 {
		t.Fatal("no package written")
	}

	file, err := cfb.Read(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if file.CLSID != cfb.CLSIDMSI {
		t.Errorf("CLSID %x", file.CLSID)
	}

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if len(database.Problems) > 0 {
		t.Errorf("problems: %v", database.Problems)
	}

	// File row: versioned x64 executable, short|long name, sequence 1.
	fileRows := rows(t, database, "File")
	if len(fileRows) != 1 {
		t.Fatalf("File rows: %d", len(fileRows))
	}

	if fileRows[0][0] != "hello.exe" || fileRows[0][4] != "1.0.0.0" || fileRows[0][5] != "1033" || fileRows[0][7] != 1 {
		t.Errorf("File row %v", fileRows[0])
	}

	if _, ok := fileRows[0][2].(string); !ok || fileRows[0][2] != "hello.exe" {
		t.Errorf("FileName %v, want the valid short name as is", fileRows[0][2])
	}

	if cell(t, database, "Media", "LastSequence", func([]any) bool { return true }) != 1 {
		t.Error("Media.LastSequence")
	}

	cabName := cell(t, database, "Media", "Cabinet", func([]any) bool { return true })
	if cabName != "#Hello.cab" {
		t.Errorf("Media.Cabinet %v", cabName)
	}

	data, ok := database.Stream("Hello.cab")
	if !ok {
		t.Fatal("cabinet stream missing")
	}

	entries, err := cab.Read(data)
	if err != nil || len(entries) != 1 || !bytes.Equal(entries[0].Data, exe) {
		t.Fatalf("cabinet: %v, %d entries", err, len(entries))
	}

	// Component: 64-bit attribute, key path is the file.
	attrs := cell(t, database, "Component", "Attributes", func(r []any) bool { return r[0] == "Hello" })
	if attrs != 256 {
		t.Errorf("Component.Attributes %v", attrs)
	}

	// Directory tree: TARGETDIR/SourceDir, ProgramFiles64Folder, INSTALLDIR.
	if cell(t, database, "Directory", "DefaultDir", func(r []any) bool { return r[0] == "TARGETDIR" }) != "SourceDir" {
		t.Error("TARGETDIR")
	}

	if cell(t, database, "Directory", "DefaultDir", func(r []any) bool { return r[0] == "ProgramFiles64Folder" }) != "PFiles64" {
		t.Error("ProgramFiles64Folder")
	}

	// Configurable directory → secure property.
	secure := cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "SecureCustomProperties" })
	if s := fmt.Sprint(secure); !strings.Contains(s, "INSTALLDIR") || !strings.Contains(s, "UPGRADE") {
		t.Errorf("SecureCustomProperties %v", secure)
	}

	if cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "ALLUSERS" }) != "1" {
		t.Error("ALLUSERS")
	}

	if cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "ProductCode" }) != res.ProductCode {
		t.Error("ProductCode")
	}

	// Sequences: the always set plus the file actions and the upgrade.
	ies := rows(t, database, "InstallExecuteSequence")
	want := map[string]int{
		"CostInitialize": 800, "InstallFiles": 4000, "RemoveFiles": 3500, "CreateFolders": 3700, "RemoveFolders": 3600,
		"InstallFinalize": 6600, "FindRelatedProducts": 25, "RemoveExistingProducts": 1501, "MigrateFeatureStates": 1200,
		"LaunchConditions": 100, "PublishProduct": 6400,
	}

	for _, row := range ies {
		if n, ok := want[fmt.Sprint(row[0])]; ok {
			if row[2] != n {
				t.Errorf("%s at %v, want %d", row[0], row[2], n)
			}

			delete(want, fmt.Sprint(row[0]))
		}
	}

	for name := range want {
		t.Errorf("%s is not scheduled", name)
	}

	for _, tbl := range []string{
		"InstallUISequence", "AdminExecuteSequence", "AdminUISequence", "AdvtExecuteSequence",
		"_Validation", "Upgrade", "LaunchCondition", "Feature", "FeatureComponents",
	} {
		if len(rows(t, database, tbl)) == 0 {
			t.Errorf("%s is empty", tbl)
		}
	}

	// Summary information.
	if summary.Get(database.Summary, summary.PIDTemplate) != "x64;1033" {
		t.Errorf("template %v", summary.Get(database.Summary, summary.PIDTemplate))
	}

	if summary.Get(database.Summary, summary.PIDPageCount) != 500 {
		t.Errorf("page count %v", summary.Get(database.Summary, summary.PIDPageCount))
	}

	if summary.Get(database.Summary, summary.PIDCodepage) != 1252 {
		t.Errorf("summary codepage %v", summary.Get(database.Summary, summary.PIDCodepage))
	}

	if summary.Get(database.Summary, summary.PIDRevision) != res.PackageCode || res.PackageCode == "" || res.PackageCode[0] != '{' {
		t.Errorf("package code %q / %v", res.PackageCode, summary.Get(database.Summary, summary.PIDRevision))
	}

	if ts, ok := summary.Get(database.Summary, summary.PIDCreated).(time.Time); !ok || !ts.Equal(buildTime) {
		t.Errorf("created %v", ts)
	}
}

func TestDeterministic(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	first := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe}))
	b := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe}))

	if !bytes.Equal(first.Package, b.Package) {
		t.Error("two builds of the same inputs differ")
	}

	// Changed content gives a new product code and package code.
	exe2 := testpe.Build(testpe.Options{FileVersion: "1.0.0.1", Languages: []int{1033}})
	changed := mustBuild(t, example(t, "minimal", nil, map[string][]byte{"hello.exe": exe2}))

	if changed.ProductCode == first.ProductCode {
		t.Error("product code did not change with the input")
	}

	if changed.PackageCode == first.PackageCode {
		t.Error("package code did not change with the input")
	}
}

func TestCFBVersion3(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "0.32.0.5", Languages: []int{1033}})

	icon := append([]byte{0, 0, 1, 0, 1, 0}, bytes.Repeat([]byte{0xAB}, 64)...)

	cfg := example(t, "windows_exporter", map[string]string{"VERSION": "0.32.0.5"}, map[string][]byte{
		"windows_exporter.exe": exe,
		"icon.ico":             icon,
	})

	res := mustBuild(t, cfg)

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	if len(database.Problems) > 0 {
		t.Errorf("problems: %v", database.Problems)
	}

	// Services.
	if len(rows(t, database, "ServiceInstall")) != 1 || len(rows(t, database, "ServiceControl")) != 1 {
		t.Error("service rows")
	}

	if len(rows(t, database, "MsiServiceConfig")) != 2 {
		t.Errorf("MsiServiceConfig rows: %d", len(rows(t, database, "MsiServiceConfig")))
	}

	hidden := cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "MsiHiddenProperties" })
	if !strings.Contains(fmt.Sprint(hidden), "SERVICE_PASSWORD") {
		t.Errorf("MsiHiddenProperties %v", hidden)
	}

	// Event log registry rows.
	v := cell(t, database, "Registry", "Value", func(r []any) bool { return r[3] == "TypesSupported" })
	if v != "#7" {
		t.Errorf("TypesSupported %v", v)
	}

	// Icon table and ARP icon.
	if len(rows(t, database, "Icon")) != 1 {
		t.Error("Icon rows")
	}

	// Firewall and recovery actions exist, are deferred and sit inside the
	// InstallInitialize … InstallFinalize window.
	ies := rows(t, database, "InstallExecuteSequence")
	seqOf := map[string]int{}

	for _, row := range ies {
		n, ok := row[2].(int)
		if !ok {
			t.Fatalf("sequence %v is not an int", row[2])
		}

		seqOf[fmt.Sprint(row[0])] = n
	}

	deferredActions := 0

	for _, row := range rows(t, database, "CustomAction") {
		name := fmt.Sprint(row[0])
		if !strings.HasPrefix(name, "Fw") && !strings.HasPrefix(name, "ServiceRecovery") {
			continue
		}

		deferredActions++

		if typ, ok := row[1].(int); !ok || typ&1024 == 0 {
			t.Errorf("%s is not deferred: type %v", name, row[1])
		}

		n, ok := seqOf[name]
		if !ok || n <= seqOf["InstallInitialize"] || n >= seqOf["InstallFinalize"] {
			t.Errorf("%s at %d is outside the script window", name, n)
		}
	}

	// Every EXE action's working directory has a Directory row (error 2727
	// otherwise), including standard folders added after the tree walk.
	dirs := map[any]bool{}
	for _, row := range rows(t, database, "Directory") {
		dirs[row[0]] = true
	}

	for _, row := range rows(t, database, "CustomAction") {
		if typ, ok := row[1].(int); ok && typ&0x3F == 34 && !dirs[row[2]] {
			t.Errorf("%s runs in %v, which has no Directory row", row[0], row[2])
		}
	}

	if deferredActions != 6 {
		t.Errorf("%d firewall/recovery actions, want 6", deferredActions)
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

	if summary.Get(database.Summary, summary.PIDTemplate) != "x64;1033" {
		t.Errorf("template %v", summary.Get(database.Summary, summary.PIDTemplate))
	}

	if strings.Contains(fmt.Sprint(res.Diagnostics), "wizard") {
		t.Errorf("unexpected wizard diagnostic: %s", res.Diagnostics)
	}
}

// TestFeatureDisplay checks the Feature.Display parity the selection tree
// relies on: even values show a feature collapsed, odd values expanded and 0
// hides it, whatever the display state of the features before it.
func TestFeatureDisplay(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	src := `schema: go-msi/v1
product:
  name: Hello
  manufacturer: Example
  version: 1.0.0
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
package:
  platform: x64
upgrade: {}
directories:
  - id: ProgramFiles64Folder
    children:
      - id: INSTALLDIR
        name: Hello
features:
  - id: Collapsed
    title: Collapsed
    components: [Hello]
    features:
      - id: Expanded
        title: Expanded
        display: expanded
        components: [Hello]
      - id: Hidden
        title: Hidden
        display: hidden
        components: [Hello]
  - id: Expanded2
    title: Expanded2
    display: expanded
    components: [Hello]
  - id: Collapsed2
    title: Collapsed2
    components: [Hello]
components:
  - id: Hello
    directory: INSTALLDIR
    files:
      - source: hello.exe
`

	if err := os.WriteFile(filepath.Join(dir, "msi.yaml"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(dir, "hello.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	database, err := msidb.Unmarshal(mustBuild(t, cfg).Package)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]int{
		"Collapsed":  2,
		"Expanded":   5,
		"Hidden":     0,
		"Expanded2":  9,
		"Collapsed2": 10,
	}

	seen := map[int]bool{}

	for _, row := range rows(t, database, "Feature") {
		id := fmt.Sprint(row[0])

		display, ok := row[4].(int)
		if !ok {
			t.Fatalf("%s: Display %v is not an int", id, row[4])
		}

		expected, ok := want[id]
		if !ok {
			t.Fatalf("unexpected feature %s", id)
		}

		delete(want, id)

		if display != expected {
			t.Errorf("%s: Display %d, want %d", id, display, expected)
		}

		if display != 0 && seen[display] {
			t.Errorf("%s: Display %d is used twice", id, display)
		}

		seen[display] = true
	}

	for id := range want {
		t.Errorf("feature %s is missing", id)
	}
}
