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

func TestDuplicateShortNamesFail(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ sections, path string }{
		"files": {`components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: "PROG~1.TXT|Program One.txt" }
      - { source: two.txt, id: Two, name: "prog~1.txt|Program Two.txt" }
`, "components[0].files[1].name"},
		"files in two components": {`components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: "PROG~1.TXT|Program One.txt" }
  - directory: APPDIR
    files:
      - { source: two.txt, id: Two, name: PROG~1.TXT }
`, "components[1].files[0].name"},
		"file named by its source": {`components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: "ONE.TXT|One Long.txt" }
      - { source: one.txt, id: Two }
`, "components[0].files[1].source"},
		"directory and file": {`        children:
          - id: DATADIR
            name: "DATA~1|Data Files"
components:
  - directory: APPDIR
    files:
      - { source: one.txt, id: One, name: "DATA~1|Data One" }
`, "components[0].files[0].name"},
		"directories": {`        children:
          - id: DATADIR
            name: "DATA~1|Data Files"
          - id: LOGDIR
            name: "DATA~1|Log Files"
components:
  - directory: DATADIR
    files:
      - { source: one.txt }
  - directory: LOGDIR
    files:
      - { source: two.txt }
`, "directories[0].children[0].children[1].name"},
		"shorthand directories": {`components:
  - directory: APPDIR/DATA~1|Data One
    files:
      - { source: one.txt }
  - directory: APPDIR/DATA~1|Data Two
    files:
      - { source: two.txt }
`, "components[1].directory"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			res, err := builder.Build(shortNameConfig(t, tc.sections), builder.Options{BuildTime: buildTime})
			if err == nil {
				t.Fatal("build succeeded with a duplicate short name")
			}

			for _, d := range res.Diagnostics {
				if d.Path == tc.path && strings.Contains(d.Message, "FILE-07") {
					return
				}
			}

			t.Errorf("no FILE-07 error at %s: %v", tc.path, res.Diagnostics)
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

// TestShortcutKeepsItsOwnShortNames checks that the short-name check leaves
// shortcuts out: a shortcut is written as name.lnk, so its literal short
// name may equal a folder's.
func TestShortcutKeepsItsOwnShortNames(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, shortNameConfig(t, `        children:
          - id: DATADIR
            name: "ACME~1|Acme Tools"
components:
  - directory: DATADIR
    files:
      - source: one.txt
        shortcuts:
          - { name: "ACME~1|Acme App", directory: APPDIR }
`))

	if got := cell(t, res.Database, "Shortcut", "Name", func([]any) bool { return true }); got != "ACME~1|Acme App" {
		t.Errorf("Shortcut.Name = %v", got)
	}
}

// TestShortNamesAcrossKinds checks that a generated file name avoids a
// directory's literal short name, and that two sibling directories with the
// same name are no short-name conflict.
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
            name: "`+generated+`|Data Files"
          - id: DATADIR2
            name: "`+generated+`|Data Files"
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
}
