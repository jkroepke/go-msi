package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestFileTypes writes a file type as HKCR Registry rows in the file's
// component: the ProgID, its icon and verbs, the extensions that point at
// it and its entry in their OpenWithProgids lists. An openWithOnly
// extension gets only the list entry, also for an extension another type
// owns.
func TestFileTypes(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
        fileTypes:
          - progId: Acme.Document
            description: '#1 [Beta] document'
            iconIndex: 1
            extensions:
              - { extension: acmedoc, contentType: application/x-acme }
              - extension: acmd
            verbs:
              - { id: open, text: '&Open' }
              - { id: print, arguments: '/p "%1"' }
              - { id: run, arguments: '' }
          - progId: Acme.Plain
            extensions:
              - { extension: acmedoc, openWithOnly: true }
              - { extension: txt, openWithOnly: true }
`)
	res := mustBuild(t, cfg)
	database := res.Database

	if len(res.Diagnostics) != 0 {
		t.Errorf("diagnostics: %v", res.Diagnostics)
	}

	// Each extension default lives in a permanent component of its own
	// beside App, with the default as key path, in App's features.
	appDir := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == "App" })[2]
	carriers := make(map[any]string)

	for _, row := range rows(t, database, "Component") {
		if row[0] == "App" {
			continue
		}

		if attrs, ok := row[3].(int); !ok || attrs&(16|4) != 16|4 || row[2] != appDir {
			t.Errorf("defaults component %v: want Permanent and RegistryKeyPath in App's folder", row)
		}

		rowWhere(t, database, "FeatureComponents", func(r []any) bool { return r[1] == row[0] })

		carriers[row[0]] = fmt.Sprint(row[5])
	}

	if len(carriers) != 2 {
		t.Errorf("%d defaults components, want one per extension default: %v", len(carriers), carriers)
	}

	got := make(map[string]string)
	inCarrier := make(map[string]bool)

	for _, row := range rows(t, database, "Registry") {
		keyPath, carried := carriers[row[5]]
		if row[5] != "App" && !carried {
			continue
		}

		if carried {
			inCarrier[fmt.Sprintf("%v|%v", row[2], row[3])] = true

			if row[0] == keyPath && row[3] != nil {
				t.Errorf("key path %v is not an extension default", row)
			}
		}

		if fmt.Sprint(row[1]) != "0" {
			t.Errorf("row %v is not under HKCR", row)
		}

		got[fmt.Sprintf("%v|%v", row[2], row[3])] = fmt.Sprint(row[4])
	}

	want := map[string]string{
		`Acme.Document|<nil>`:                     `##1 [\[]Beta[\]] document`,
		`Acme.Document\DefaultIcon|<nil>`:         `"[#app.exe]",1`,
		`Acme.Document\shell\open|<nil>`:          "&Open",
		`Acme.Document\shell\open\command|<nil>`:  `"[#app.exe]" "%1"`,
		`Acme.Document\shell\print\command|<nil>`: `"[#app.exe]" /p "%1"`,
		`Acme.Document\shell\run\command|<nil>`:   `"[#app.exe]"`,
		`.acmedoc|<nil>`:                          "Acme.Document",
		`.acmedoc|Content Type`:                   "application/x-acme",
		`.acmd|<nil>`:                             "Acme.Document",
		`.acmedoc\OpenWithProgids|Acme.Document`:  "<nil>",
		`.acmd\OpenWithProgids|Acme.Document`:     "<nil>",
		`.acmedoc\OpenWithProgids|Acme.Plain`:     "<nil>",
		`.txt\OpenWithProgids|Acme.Plain`:         "<nil>",
		`Acme.Plain\shell\open\command|<nil>`:     `"[#app.exe]" "%1"`,
	}

	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}

	if len(got) != len(want) {
		t.Errorf("got %d rows, want %d: %v", len(got), len(want), got)
	}

	for key := range inCarrier {
		if key != `.acmedoc|<nil>` && key != `.acmedoc|Content Type` && key != `.acmd|<nil>` {
			t.Errorf("%s is in a permanent component", key)
		}
	}

	if len(inCarrier) != 3 {
		t.Errorf("permanent components hold %v, want the three extension rows", inCarrier)
	}

	// The shell hears about the change right after InstallFinalize, from
	// the run DLL, whatever the action returns.
	action := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == "BuilderAssocChanged" })
	if action[1] != 1|0x40 || action[2] != "BuilderRun" || action[3] != "BuilderAssocChanged" {
		t.Errorf("CustomAction %v", action)
	}

	rowWhere(t, database, "Binary", func(r []any) bool { return r[0] == "BuilderRun" })

	ies := rows(t, database, "InstallExecuteSequence")
	sequence := map[string]int{}

	for _, row := range ies {
		if n, ok := row[2].(int); ok {
			sequence[fmt.Sprint(row[0])] = n
		}
	}

	if sequence["BuilderAssocChanged"] <= sequence["InstallFinalize"] {
		t.Errorf("BuilderAssocChanged at %d, InstallFinalize at %d", sequence["BuilderAssocChanged"], sequence["InstallFinalize"])
	}
}

// TestFileTypesOpenWithOnly writes no permanent component when every
// extension is openWithOnly, but still notifies the shell.
func TestFileTypesOpenWithOnly(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
        fileTypes: [{ progId: Acme.Plain, extensions: [{ extension: txt, openWithOnly: true }] }]
`)
	database := mustBuild(t, cfg).Database

	if n := len(rows(t, database, "Component")); n != 1 {
		t.Errorf("%d components, want only App", n)
	}

	rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == "BuilderAssocChanged" })
}

// TestFileTypesPerUser names HKCU\Software\Classes in a per-user package.
func TestFileTypesPerUser(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: Acme App, manufacturer: Acme, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64, scope: perUser }
components:
  - id: App
    files:
      - source: a.txt
        fileTypes: [{ progId: Acme.Document, extensions: [{ extension: acmedoc }] }]
`), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.BaseDir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	database := mustBuild(t, cfg).Database

	row := rowWhere(t, database, "Registry", func(r []any) bool { return r[2] == `Software\Classes\.acmedoc` })
	if row == nil || fmt.Sprint(row[1]) != "1" {
		t.Errorf("per-user extension row %v", row)
	}
}

// TestFileTypesAssocChangedAfterRemoval schedules the shell notification
// after RemoveExistingProducts when the old product is removed after
// InstallFinalize, and reserves the action's name.
func TestFileTypesAssocChangedAfterRemoval(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
        fileTypes: [{ progId: Acme.Document, extensions: [{ extension: acmedoc }] }]
`)
	cfg.Upgrade.RemoveOld = "afterInstallFinalize"
	database := mustBuild(t, cfg).Database
	sequence := map[string]int{}

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if n, ok := row[2].(int); ok {
			sequence[fmt.Sprint(row[0])] = n
		}
	}

	if sequence["BuilderAssocChanged"] <= sequence["RemoveExistingProducts"] || sequence["RemoveExistingProducts"] <= sequence["InstallFinalize"] {
		t.Errorf("InstallFinalize %d, RemoveExistingProducts %d, BuilderAssocChanged %d",
			sequence["InstallFinalize"], sequence["RemoveExistingProducts"], sequence["BuilderAssocChanged"])
	}

	cfg = shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
        fileTypes: [{ progId: Acme.Document, extensions: [{ extension: acmedoc }] }]
customActions:
  - id: BuilderAssocChanged
    error: x
    schedule: [{ sequence: execute, before: InstallInitialize }]
`)

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "customActions[0].id") {
		t.Errorf("err %v, diagnostics %s: want the reserved name at customActions[0].id", err, res.Diagnostics)
	}
}
