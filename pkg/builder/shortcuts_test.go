package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/msidb"
)

// Shortcut table columns.
const (
	scDirectory = 1
	scName      = 2
	scComponent = 3
	scTarget    = 4
	scArguments = 5
	scDesc      = 6
	scIcon      = 8
	scIconIndex = 9
	scShowCmd   = 10
	scWkDir     = 11
)

// shortcutConfig parses a package with app.exe and the given components
// section.
func shortcutConfig(t *testing.T, components string) *config.Config {
	t.Helper()

	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	for name, data := range map[string][]byte{"app.exe": exe, "tool.exe": exe, "app.ico": {0, 0, 1, 0}} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: Acme App, manufacturer: Acme, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
icons: { app.ico: app.ico }
components:
`+components), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	return cfg
}

func rowWhere(t *testing.T, database *msidb.Database, table string, match func([]any) bool) []any {
	t.Helper()

	for _, row := range rows(t, database, table) {
		if match(row) {
			return row
		}
	}

	t.Fatalf("no %s row matches: %v", table, rows(t, database, table))

	return nil
}

func longOf(name any) string {
	s := fmt.Sprint(name)
	if _, long, ok := strings.Cut(s, "|"); ok {
		return long
	}

	return s
}

func TestShortcutsDesktopExample(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, desktop(t))
	database := res.Database

	shortcuts := rows(t, database, "Shortcut")
	if len(shortcuts) != 2 {
		t.Fatalf("Shortcut rows: %v", shortcuts)
	}

	start := rowWhere(t, database, "Shortcut", func(r []any) bool { return r[scDirectory] == "ProgramMenuFolder" })
	desk := rowWhere(t, database, "Shortcut", func(r []any) bool { return r[scDirectory] == "DesktopFolder" })

	for _, row := range [][]any{start, desk} {
		if longOf(row[scName]) != "Example Viewer" || row[scTarget] != "[#viewer.exe]" || row[scWkDir] != "INSTALLDIR" {
			t.Errorf("shortcut row %v", row)
		}

		if row[scIcon] != nil || row[scIconIndex] != nil || row[scShowCmd] != nil || row[scArguments] != nil {
			t.Errorf("unset columns are not null: %v", row)
		}
	}

	if start[scDesc] != "View example files" || desk[scDesc] != nil {
		t.Errorf("descriptions %v / %v", start[scDesc], desk[scDesc])
	}

	// Both shortcuts share one generated component with an HKCU key path,
	// in the folder of the first shortcut, in the feature of viewer.exe.
	carrier := start[scComponent]
	if desk[scComponent] != carrier || carrier == "" {
		t.Fatalf("carriers %v / %v", carrier, desk[scComponent])
	}

	comp := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == carrier })
	if comp[2] != "ProgramMenuFolder" || comp[3] != 4|256 {
		t.Errorf("carrier component %v", comp)
	}

	reg := rowWhere(t, database, "Registry", func(r []any) bool { return r[0] == comp[5] })
	if reg[1] != 1 || reg[2] != `Software\[Manufacturer]\[ProductName]` || reg[4] != "#1" || reg[5] != carrier {
		t.Errorf("key path value %v", reg)
	}

	viewerComp := rowWhere(t, database, "File", func(r []any) bool { return r[0] == "viewer.exe" })[1]
	feature := rowWhere(t, database, "FeatureComponents", func(r []any) bool { return r[1] == viewerComp })[0]
	rowWhere(t, database, "FeatureComponents", func(r []any) bool { return r[0] == feature && r[1] == carrier })

	sequence := rows(t, database, "InstallExecuteSequence")
	actions := make([]string, 0, len(sequence))

	for _, row := range sequence {
		actions = append(actions, fmt.Sprint(row[0]))
	}

	for _, action := range []string{"CreateShortcuts", "RemoveShortcuts", "WriteRegistryValues", "RemoveRegistryValues"} {
		if !slices.Contains(actions, action) {
			t.Errorf("InstallExecuteSequence lacks %s", action)
		}
	}

	// A shortcut directly in a standard folder needs no RemoveFile row.
	if database.Table("RemoveFile") != nil && len(rows(t, database, "RemoveFile")) > 0 {
		t.Errorf("RemoveFile rows %v", rows(t, database, "RemoveFile"))
	}
}

func TestShortcutOptions(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files:
      - source: app.exe
        shortcuts:
          - { id: AppMenu, name: "Acme|Acme Console", directory: ProgramMenuFolder/Acme/Tools, arguments: "--user [LogonUser]",
              workingDirectory: ProgramMenuFolder, icon: app.ico, iconIndex: 2, show: minimized }
          - { directory: ProgramMenuFolder/Acme, show: maximized }
`)
	res := mustBuild(t, cfg)
	database := res.Database

	console := rowWhere(t, database, "Shortcut", func(r []any) bool { return r[0] == "AppMenu" })
	if console[scName] != "Acme|Acme Console" || console[scArguments] != "--user [LogonUser]" || console[scIcon] != "app.ico" ||
		console[scIconIndex] != 2 || console[scShowCmd] != 7 || console[scWkDir] != "ProgramMenuFolder" {
		t.Errorf("shortcut row %v", console)
	}

	plain := rowWhere(t, database, "Shortcut", func(r []any) bool { return r[0] != "AppMenu" })
	if longOf(plain[scName]) != "Acme App" || plain[scShowCmd] != 3 {
		t.Errorf("shortcut row %v", plain)
	}

	// The path made two folders below the Start menu; the carrier removes
	// both, once each.
	tools := console[scDirectory]
	acme := plain[scDirectory]

	if parent := cell(t, database, "Directory", "Directory_Parent", func(r []any) bool { return r[0] == tools }); parent != acme {
		t.Errorf("%v is below %v, want %v", tools, parent, acme)
	}

	removes := rows(t, database, "RemoveFile")
	if len(removes) != 2 {
		t.Fatalf("RemoveFile rows %v", removes)
	}

	for _, dir := range []any{tools, acme} {
		row := rowWhere(t, database, "RemoveFile", func(r []any) bool { return r[3] == dir })
		if row[1] != console[scComponent] || row[2] != nil || row[4] != 2 {
			t.Errorf("RemoveFile row %v", row)
		}
	}
}

func TestShortcutCarrierPerComponent(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files: [{ source: app.exe, shortcuts: [{ name: App }] }]
  - condition: INSTALLTOOL
    files: [{ source: tool.exe, shortcuts: [{ name: Tool }, { name: Tool, directory: DesktopFolder }] }]
`)
	database := mustBuild(t, cfg).Database

	app := rowWhere(t, database, "Shortcut", func(r []any) bool { return longOf(r[scName]) == "App" })
	tool := rowWhere(t, database, "Shortcut", func(r []any) bool { return longOf(r[scName]) == "Tool" && r[scDirectory] == "DesktopFolder" })

	if app[scComponent] == tool[scComponent] {
		t.Fatal("two components share a shortcut carrier")
	}

	appComp := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == app[scComponent] })
	toolComp := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == tool[scComponent] })

	if appComp[1] == toolComp[1] {
		t.Errorf("carriers share ComponentId %v", appComp[1])
	}

	// The carrier follows the condition of the file's component.
	if appComp[4] != nil || toolComp[4] != "INSTALLTOOL" {
		t.Errorf("carrier conditions %v / %v", appComp[4], toolComp[4])
	}

	if len(rows(t, database, "Shortcut")) != 3 || len(rows(t, database, "Registry")) != 2 {
		t.Errorf("Shortcut %v, Registry %v", rows(t, database, "Shortcut"), rows(t, database, "Registry"))
	}
}

func TestShortcutsDeterministic(t *testing.T) {
	t.Parallel()

	components := `
  - files: [{ source: app.exe, shortcuts: [{}, { directory: DesktopFolder }] }]
`
	first := mustBuild(t, shortcutConfig(t, components))
	second := mustBuild(t, shortcutConfig(t, components))

	if string(first.Package) != string(second.Package) {
		t.Error("two builds of one config differ")
	}
}

func TestShortcutValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ shortcut, path, message string }{
		{`{ icon: other.ico }`, ".icon", "not defined in icons"},
		{`{ show: hidden }`, ".show", "hidden"},
		{`{ workingDirectory: NOWHERE }`, ".workingDirectory", "not defined"},
		{`{ directory: NOWHERE }`, ".directory", "not defined"},
		{`{ name: "a/b" }`, ".name", "not a valid file name"},
		{`{ id: "1x" }`, ".id", "not an identifier"},
		{`{ icon: app.ico, iconIndex: 40000 }`, ".iconIndex", "out of range"},
		{`{ iconIndex: 3 }`, ".iconIndex", "needs icon"},
		{`{ name: "` + strings.Repeat("A", 127) + `" }`, ".name", "at most 128 fit"},
	} {
		cfg := shortcutConfig(t, "  - files: [{ source: app.exe, shortcuts: ["+tc.shortcut+"] }]\n")

		_, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(err.Error(), "components[0].files[0].shortcuts[0]"+tc.path) || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%s: %v", tc.shortcut, err)
		}
	}

	// Shortcut.Target holds [#key] in 72 characters.
	cfg := shortcutConfig(t, "  - files: [{ source: app.exe, id: "+strings.Repeat("a", 70)+", shortcuts: [{}] }]\n")
	if _, err := builder.Build(cfg, builder.Options{BuildTime: buildTime}); err == nil || !strings.Contains(err.Error(), "files[0].shortcuts: the shortcut target") {
		t.Errorf("a file id too long for the target: %v", err)
	}

	cfg = shortcutConfig(t, "  - files: [{ source: app.exe, id: "+strings.Repeat("a", 69)+", shortcuts: [{}] }]\n")
	if _, err := builder.Build(cfg, builder.Options{BuildTime: buildTime}); err != nil {
		t.Errorf("a file id that just fits the target: %v", err)
	}

	cfg = shortcutConfig(t, "  - files: [{ source: app.exe, shortcuts: [{ name: A }, { name: a }] }]\n")
	if _, err := builder.Build(cfg, builder.Options{BuildTime: buildTime}); err == nil || !strings.Contains(err.Error(), "shortcuts[1].name") {
		t.Errorf("two shortcuts of one name in one folder: %v", err)
	}

	cfg = shortcutConfig(t, "  - files: [{ source: app.exe, shortcuts: [{ name: A.lnk }] }]\n")

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err != nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "adds .lnk") {
		t.Errorf("a name ending in .lnk: %v %s", err, res.Diagnostics)
	}
}

func TestShortcutNullEntry(t *testing.T) {
	t.Parallel()

	for _, files := range []string{"[{ source: app.exe, shortcuts: [~] }]", "[~]"} {
		_, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
components:
  - files: `+files+"\n"), config.Options{})
		if err == nil || !strings.Contains(err.Error(), "empty entry") {
			t.Errorf("files %s: %v", files, err)
		}
	}

	// A nil shortcut set from Go is reported, not dereferenced.
	cfg := shortcutConfig(t, "  - files: [{ source: app.exe }]\n")
	cfg.Components[0].Files[0].Shortcuts = []*config.Shortcut{nil}

	if _, err := builder.Build(cfg, builder.Options{BuildTime: buildTime}); err == nil || !strings.Contains(err.Error(), "shortcuts[0]: empty entry") {
		t.Errorf("nil shortcut: %v", err)
	}
}

// TestShortcutCarrierIdentity checks that components whose ids differ only
// in case, and two products with the same names, get distinct key paths.
func TestShortcutCarrierIdentity(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - { id: App, files: [{ source: app.exe, shortcuts: [{ name: One }] }] }
  - { id: app, files: [{ source: tool.exe, shortcuts: [{ name: Two }] }] }
`)
	database := mustBuild(t, cfg).Database

	comps := map[any]bool{}
	values := map[string]bool{}

	for _, row := range rows(t, database, "Shortcut") {
		comp := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == row[scComponent] })
		comps[comp[1]] = true

		reg := rowWhere(t, database, "Registry", func(r []any) bool { return r[0] == comp[5] })
		values[strings.ToLower(fmt.Sprint(reg[2], `\`, reg[3]))] = true
	}

	if len(comps) != 2 || len(values) != 2 {
		t.Errorf("ComponentIds %v, key path values %v", comps, values)
	}

	other := shortcutConfig(t, "  - { id: App, files: [{ source: app.exe, shortcuts: [{ name: One }] }] }\n")
	other.Product.UpgradeCode = "3f2504e0-4f89-41d3-9a0c-0305e82c3399"
	otherDB := mustBuild(t, other).Database

	reg := rows(t, otherDB, "Registry")[0]
	if values[strings.ToLower(fmt.Sprint(reg[2], `\`, reg[3]))] {
		t.Errorf("another product uses the same key path value %v", reg)
	}
}

func TestShortcutFolderRules(t *testing.T) {
	t.Parallel()

	// A carrier in a 32-bit folder is a 32-bit component (ICE80).
	cfg := shortcutConfig(t, "  - files: [{ source: app.exe, shortcuts: [{ directory: ProgramFilesFolder/Acme }] }]\n")
	database := mustBuild(t, cfg).Database
	shortcut := rows(t, database, "Shortcut")[0]

	if comp := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == shortcut[scComponent] }); comp[3] != 4 {
		t.Errorf("carrier in ProgramFilesFolder has attributes %v", comp[3])
	}

	if database.Table("RemoveFile") != nil && len(rows(t, database, "RemoveFile")) > 0 {
		t.Errorf("RemoveFile rows outside the profile: %v", rows(t, database, "RemoveFile"))
	}

	// A per-user package cannot put the carrier in a per-machine folder.
	cfg = shortcutConfig(t, "  - files: [{ source: app.exe, shortcuts: [{ directory: CommonAppDataFolder/Acme }] }]\n")
	cfg.Package.Scope = config.ScopePerUser
	cfg.Directories = nil

	_, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil || !strings.Contains(err.Error(), "shortcuts[0].directory") || !strings.Contains(err.Error(), "ICE105") {
		t.Errorf("per-user shortcut in CommonAppDataFolder: %v", err)
	}
}
