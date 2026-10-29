package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// shortNameConfig parses a package with one.txt and two.txt and the given
// directories and components sections.
func shortNameConfig(t *testing.T, sections string) *config.Config {
	t.Helper()

	dir := t.TempDir()

	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: Acme App, manufacturer: Acme, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
directories:
  - id: ProgramFiles64Folder
    children:
      - id: APPDIR
        name: Acme App
`+sections), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	return cfg
}

// TestShortNamePairRejected checks that a hand-written short|long name is
// an error wherever a name is given: msi-builder generates every short name.
func TestShortNamePairRejected(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ sections, path string }{
		"file": {`components:
  - directory: APPDIR
    files:
      - { source: one.txt, name: "PROG~1.TXT|Program One.txt" }
`, "components[0].files[0].name"},
		"directory": {`        children:
          - id: DATADIR
            name: "DATA~1|Data Files"
components:
  - directory: DATADIR
    files:
      - { source: one.txt }
`, "directories[0].children[0].children[0].name"},
		"shorthand directory": {`components:
  - directory: APPDIR/DATA~1|Data One
    files:
      - { source: one.txt }
`, "components[0].directory"},
		"shortcut": {`components:
  - directory: APPDIR
    files:
      - source: one.txt
        shortcuts:
          - { name: "ACME~1|Acme App" }
`, "components[0].files[0].shortcuts[0].name"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res, err := builder.Build(shortNameConfig(t, tc.sections), builder.Options{BuildTime: buildTime})
			if err == nil {
				t.Fatal("build succeeded with a short|long name")
			}

			for _, d := range res.Diagnostics {
				if d.Path == tc.path && strings.Contains(d.Message, "short names are generated") {
					return
				}
			}

			t.Errorf("no short name error at %s: %v", tc.path, res.Diagnostics)
		})
	}
}

// TestGeneratedShortNameAvoidsLiteralName checks that a generated short
// name never takes a literal one in the same directory, even when the row
// with the literal name is written later: the long names here sort the
// generated one first.
func TestGeneratedShortNameAvoidsLiteralName(t *testing.T) {
	t.Parallel()

	first := mustBuild(t, shortNameConfig(t, `components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: "!Alpha Long Name.txt" }
`))

	generated, _, _ := strings.Cut(fmt.Sprint(cell(t, first.Database, "File", "FileName", func([]any) bool { return true })), "|")

	second := mustBuild(t, shortNameConfig(t, `components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: "!Alpha Long Name.txt" }
      - { source: two.txt, id: Two, name: `+generated+` }
`))

	seen := map[string]string{}

	for _, row := range rows(t, second.Database, "File") {
		short, _, _ := strings.Cut(fmt.Sprint(row[2]), "|")
		if other, dup := seen[strings.ToLower(short)]; dup {
			t.Errorf("%s and %s share the short name %s", other, row[0], short)
		}

		seen[strings.ToLower(short)] = fmt.Sprint(row[0])
	}

	if len(seen) != 2 {
		t.Errorf("File rows: %v", rows(t, second.Database, "File"))
	}
}

// TestShortcutKeepsItsOwnShortNames guards against adding shortcuts to the
// short-name set of their directory: a shortcut is written as name.lnk, so a
// shortcut and a folder of the same name get the same generated short name.
func TestShortcutKeepsItsOwnShortNames(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, shortNameConfig(t, `        children:
          - id: DATADIR
            name: Acme Tools
components:
  - directory: DATADIR
    files:
      - source: one.txt
        shortcuts:
          - { name: Acme Tools, directory: APPDIR }
`))

	dirShort, _, _ := strings.Cut(fmt.Sprint(cell(t, res.Database, "Directory", "DefaultDir", func(row []any) bool { return row[0] == "DATADIR" })), "|")
	shortcutShort, _, _ := strings.Cut(fmt.Sprint(cell(t, res.Database, "Shortcut", "Name", func([]any) bool { return true })), "|")

	if shortcutShort != dirShort {
		t.Errorf("Shortcut short name %s, want the folder's %s", shortcutShort, dirShort)
	}
}

// TestShortNamesAcrossKinds checks that a file's generated short name
// avoids a directory whose long name is a valid short name, and the other
// way round.
func TestShortNamesAcrossKinds(t *testing.T) {
	t.Parallel()

	first := mustBuild(t, shortNameConfig(t, `components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: Readme Long }
`))

	generated, _, _ := strings.Cut(fmt.Sprint(cell(t, first.Database, "File", "FileName", func([]any) bool { return true })), "|")

	second := mustBuild(t, shortNameConfig(t, `        children:
          - id: DATADIR
            name: `+generated+`
components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: Readme Long }
  - directory: DATADIR
    files:
      - { source: two.txt }
`))

	name := fmt.Sprint(cell(t, second.Database, "File", "FileName", func(row []any) bool { return row[0] == "One" }))
	if short, _, _ := strings.Cut(name, "|"); strings.EqualFold(short, generated) {
		t.Errorf("File.FileName %s takes the directory's short name %s", name, generated)
	}

	dirOnly := mustBuild(t, shortNameConfig(t, `        children:
          - id: DATADIR
            name: Data Files
components:
  - directory: DATADIR
    files:
      - { source: two.txt }
`))

	isDataDir := func(row []any) bool { return row[0] == "DATADIR" }
	dirShort, _, _ := strings.Cut(fmt.Sprint(cell(t, dirOnly.Database, "Directory", "DefaultDir", isDataDir)), "|")

	withFile := mustBuild(t, shortNameConfig(t, `        children:
          - id: DATADIR
            name: Data Files
components:
  - directory: DATADIR
    files:
      - { source: two.txt }
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: `+dirShort+` }
`))

	defaultDir := fmt.Sprint(cell(t, withFile.Database, "Directory", "DefaultDir", isDataDir))
	if short, _, _ := strings.Cut(defaultDir, "|"); strings.EqualFold(short, dirShort) {
		t.Errorf("Directory.DefaultDir %s takes the file's short name %s", defaultDir, dirShort)
	}
}
