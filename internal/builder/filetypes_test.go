package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

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

	// Every row is in the file's component, and file types need no
	// custom action or extra component.
	if n := len(rows(t, database, "Component")); n != 1 {
		t.Errorf("%d components, want only App", n)
	}

	for _, table := range []string{"CustomAction", "Binary"} {
		if tbl := database.Table(table); tbl != nil && len(tbl.Rows) != 0 {
			t.Errorf("%s rows %v, want none", table, tbl.Rows)
		}
	}

	got := make(map[string]string)

	for _, row := range rows(t, database, "Registry") {
		if row[5] != "App" {
			t.Errorf("row %v is not in App", row)

			continue
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
