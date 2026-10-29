package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestFileTypes writes a file type as HKCR Registry rows in the file's
// component: the ProgID, its icon and verbs, and the extensions that point
// at it.
func TestFileTypes(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
        fileTypes:
          - progId: Acme.Document
            description: '#1 document'
            iconIndex: 1
            extensions:
              - { extension: acmedoc, contentType: application/x-acme }
              - extension: acmd
            verbs:
              - { id: open, text: '&Open' }
              - { id: print, arguments: '/p "%1"' }
              - { id: run, arguments: '' }
          - progId: Acme.Plain
`)
	database := mustBuild(t, cfg).Database

	got := make(map[string]string)

	for _, row := range rows(t, database, "Registry") {
		if row[5] != "App" {
			continue
		}

		if fmt.Sprint(row[1]) != "0" {
			t.Errorf("row %v is not under HKCR", row)
		}

		got[fmt.Sprintf("%v|%v", row[2], row[3])] = fmt.Sprint(row[4])
	}

	want := map[string]string{
		`Acme.Document|<nil>`:                     "##1 document",
		`Acme.Document\DefaultIcon|<nil>`:         `"[#app.exe]",1`,
		`Acme.Document\shell\open|<nil>`:          "&Open",
		`Acme.Document\shell\open\command|<nil>`:  `"[#app.exe]" "%1"`,
		`Acme.Document\shell\print\command|<nil>`: `"[#app.exe]" /p "%1"`,
		`Acme.Document\shell\run\command|<nil>`:   `"[#app.exe]"`,
		`.acmedoc|<nil>`:                          "Acme.Document",
		`.acmedoc|Content Type`:                   "application/x-acme",
		`.acmd|<nil>`:                             "Acme.Document",
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
