package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestFileTypes writes the extensions a file type owns into the ProgId,
// Extension and Verb tables, which bring the engine's register and
// unregister actions, and the rest as HKCR Registry rows in the file's
// component: DefaultIcon, Content Type and the OpenWithProgids entries. A
// ProgID whose extensions are all openWithOnly is Registry rows too.
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
              - { id: open, text: '&Open [Now]' }
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

	if n := len(rows(t, database, "Component")); n != 1 {
		t.Errorf("%d components, want only App", n)
	}

	progIDs := rows(t, database, "ProgId")
	if len(progIDs) != 1 || fmt.Sprint(progIDs[0]) != "[Acme.Document <nil> <nil> #1 [Beta] document <nil> <nil>]" {
		t.Errorf("ProgId %v", progIDs)
	}

	feature := fmt.Sprint(rowWhere(t, database, "FeatureComponents", func(r []any) bool { return r[1] == "App" })[0])
	extensions := make(map[string]string)

	for _, row := range rows(t, database, "Extension") {
		extensions[fmt.Sprint(row[0])] = fmt.Sprint(row[1:]...)
	}

	for _, ext := range []string{"acmedoc", "acmd"} {
		if want := fmt.Sprint("App", "Acme.Document", nil, feature); extensions[ext] != want {
			t.Errorf("Extension %s = %q, want %q", ext, extensions[ext], want)
		}
	}

	if len(extensions) != 2 {
		t.Errorf("Extension rows %v, want only the owned extensions", extensions)
	}

	verbs := make(map[string]string)

	for _, row := range rows(t, database, "Verb") {
		verbs[fmt.Sprintf("%v|%v", row[0], row[1])] = fmt.Sprint(row[2:]...)
	}

	for _, ext := range []string{"acmedoc", "acmd"} {
		for verb, want := range map[string]string{
			"open":  fmt.Sprint(1, `&Open [\[]Now[\]]`, `"%1"`),
			"print": fmt.Sprint(2, nil, `/p "%1"`),
			"run":   fmt.Sprint(3, nil, nil),
		} {
			if got := verbs[ext+"|"+verb]; got != want {
				t.Errorf("Verb %s %s = %q, want %q", ext, verb, got, want)
			}
		}
	}

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
		`Acme.Document\DefaultIcon|<nil>`:        `"[#app.exe]",1`,
		`.acmedoc|Content Type`:                  "application/x-acme",
		`.acmedoc\OpenWithProgids|Acme.Document`: "<nil>",
		`.acmd\OpenWithProgids|Acme.Document`:    "<nil>",
		`.acmedoc\OpenWithProgids|Acme.Plain`:    "<nil>",
		`.txt\OpenWithProgids|Acme.Plain`:        "<nil>",
		`Acme.Plain\shell\open\command|<nil>`:    `"[#app.exe]" "%1"`,
	}

	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}

	if len(got) != len(want) {
		t.Errorf("got %d rows, want %d: %v", len(got), len(want), got)
	}

	sequence := map[string]int{}

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if n, ok := row[2].(int); ok {
			sequence[fmt.Sprint(row[0])] = n
		}
	}

	for _, action := range []string{"UnregisterExtensionInfo", "UnregisterProgIdInfo", "RegisterExtensionInfo", "RegisterProgIdInfo"} {
		if sequence[action] == 0 {
			t.Errorf("%s is not scheduled", action)
		}
	}

	if hasRows(t, database, "CustomAction") || hasRows(t, database, "Binary") {
		t.Error("file types brought a custom action or a binary")
	}
}

// TestFileTypesOpenWithOnly writes only Registry rows when every extension
// is openWithOnly.
func TestFileTypesOpenWithOnly(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
        fileTypes: [{ progId: Acme.Plain, extensions: [{ extension: txt, openWithOnly: true }] }]
`)
	database := mustBuild(t, cfg).Database

	for _, table := range []string{"ProgId", "Extension", "Verb"} {
		if hasRows(t, database, table) {
			t.Errorf("%s has rows", table)
		}
	}

	rowWhere(t, database, "Registry", func(r []any) bool { return r[2] == `Acme.Plain\shell\open\command` })
}

// TestFileTypesKeyPath reports a file type on a file that is not its
// component's key path: the engine runs the verbs with the key path.
func TestFileTypesKeyPath(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - id: App
    files:
      - source: app.exe
      - source: tool.exe
        fileTypes: [{ progId: Acme.Document, extensions: [{ extension: acmedoc }] }]
`)

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "components[0].files[1].fileTypes[0]") {
		t.Errorf("err %v, diagnostics %s: want the key path error at the file type", err, res.Diagnostics)
	}

	cfg = shortcutConfig(t, `
  - id: App
    keyPath: tool.exe
    files:
      - source: app.exe
      - source: tool.exe
        fileTypes: [{ progId: Acme.Document, extensions: [{ extension: acmedoc }] }]
`)
	mustBuild(t, cfg)
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
        fileTypes: [{ progId: Acme.Document, extensions: [{ extension: acmedoc, contentType: text/x-acme }] }]
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

// hasRows reports whether the database has table with at least one row.
func hasRows(t *testing.T, db *msidb.Database, table string) bool {
	t.Helper()

	tbl := db.Table(table)

	return tbl != nil && len(tbl.Rows) > 0
}
