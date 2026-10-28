//go:build windows

package nativetest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/msidb"
	"github.com/jkroepke/msi-builder/pkg/summary"
)

// textTemplate puts the same text into every kind of string column the
// fuzz tests reach: names, a directory, a long file name, a registry key
// and value, a property, a condition and the summary.
const textTemplate = `schema: msi-builder/v1
product:
  name: '%[1]s'
  manufacturer: '%[1]s'
  version: 1.0.0
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
  description: '%[1]s'
package:
  platform: x64
  codepage: %[2]d
upgrade: {}
directories:
  - id: ProgramFiles64Folder
    children:
      - id: INSTALLDIR
        name: '%[1]s'
features:
  - id: Main
    title: '%[1]s'
    components: [Main, Reg]
components:
  - id: Main
    directory: INSTALLDIR
    files:
      - source: app.exe
      - source: data.bin
        id: Payload
        name: '%[1]s.txt'
  - id: Reg
    directory: INSTALLDIR
    registry:
      - root: HKLM
        key: 'Software\%[1]s'
        name: '%[1]s'
        value: '%[1]s'
properties:
  TEXTPROP: '%[1]s'
launchConditions:
  - condition: 'TEXTPROP = "%[1]s"'
    message: '%[1]s'
`

// TestMsiDLLReadsText builds packages with non-ASCII text in each database
// code page and checks that msi.dll returns every string cell as written.
//
// One package at a time: parallel msi.dll views failed once with
// ERROR_INVALID_HANDLE on windows-latest.
//
//nolint:paralleltest // see above
func TestMsiDLLReadsText(t *testing.T) {
	if testing.Short() {
		t.Skip("native tests need msi.dll time")
	}

	for _, tc := range []struct {
		text     string
		codepage int
	}{
		{"Grüße Größe", 1252},
		{"Привет мир", 1251},
		{"日本語のテキスト", 932},
		{"简体中文", 936},
		{"Grüße Привет 日本 😀", 65001},
		{"Hello", 0},
	} {
		t.Run(strconv.Itoa(tc.codepage), func(t *testing.T) {
			path, res := buildText(t, tc.text, tc.codepage)
			database := openDB(t, path)

			for _, tbl := range res.Database.Tables {
				compareTable(t, database, tbl)
			}

			// The summary keeps its own code page, with '?' for what it
			// cannot hold; msi.dll must return what the builder stored.
			for _, pid := range []int{summary.PIDTitle, summary.PIDSubject, summary.PIDAuthor, summary.PIDComments} {
				if got, want := summaryString(t, database, uintptr(pid)), summary.Get(res.Database.Summary, pid); got != want {
					t.Errorf("summary PID %d %q, want %q", pid, got, want)
				}
			}

			props := query(t, database, "SELECT `Value` FROM `Property` WHERE `Property` = 'TEXTPROP'")
			if len(props) != 1 || props[0][0] != tc.text {
				t.Errorf("TEXTPROP %q, want %q", props, tc.text)
			}
		})
	}
}

func buildText(t *testing.T, text string, codepage int) (string, *builder.Result) {
	t.Helper()

	dir := t.TempDir()

	files := map[string][]byte{
		"msi.yaml": fmt.Appendf(nil, textTemplate, text, codepage),
		"app.exe":  testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}}),
		"data.bin": []byte("payload"),
	}

	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})
	if err != nil {
		t.Fatalf("%v\n%v", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "text.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, res
}

// compareTable compares the rows msi.dll returns with the rows the builder
// wrote, as sorted lists of rendered rows. Binary columns are left out.
func compareTable(t *testing.T, database uint32, tbl *msidb.Table) {
	t.Helper()

	var names []string

	var cols []int

	for i, col := range tbl.Columns {
		if col.Type.Kind != msidb.KindBinary {
			names = append(names, "`"+col.Name+"`")
			cols = append(cols, i)
		}
	}

	want := make([]string, 0, len(tbl.Rows))

	for _, row := range tbl.Rows {
		cells := make([]string, len(cols))

		for j, i := range cols {
			if row[i] != nil {
				cells[j] = fmt.Sprint(row[i])
			}
		}

		want = append(want, strings.Join(cells, "\x00"))
	}

	rows := query(t, database, "SELECT "+strings.Join(names, ", ")+" FROM `"+tbl.Name+"`")
	got := make([]string, 0, len(rows))

	for _, row := range rows {
		got = append(got, strings.Join(row, "\x00"))
	}

	slices.Sort(want)
	slices.Sort(got)

	if !slices.Equal(got, want) {
		t.Errorf("table %s: msi.dll reads\n%q\nbuilder wrote\n%q", tbl.Name, got, want)
	}
}
