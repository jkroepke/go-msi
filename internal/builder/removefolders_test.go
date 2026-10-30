package builder_test

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestRemoveFolders builds the data of the rollback, deferred and commit
// actions with type-51 actions right after RemoveFiles, one pair per entry
// under its condition, schedules no immediate action, embeds the DLL and
// leaves the properties' security to their own settings.
func TestRemoveFolders(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files:
      - source: app.exe
    removeFolders:
      - property: INSTALLDIR
      - { property: DATA_DIR, on: both, condition: DELETE_DATA="1" }
      - { property: OLD_DIR, on: install }
installDir: { remember: true }
properties:
  DATA_DIR: { value: 'C:\Data\' }
  OLD_DIR: { value: 'C:\Old\', secure: false }
`)
	database := mustBuild(t, cfg).Database

	seq := make(map[string]int)
	conds := make(map[string]string)
	byNumber := make(map[int]string)

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if number, ok := row[2].(int); ok {
			seq[fmt.Sprint(row[0])] = number
			byNumber[number] = fmt.Sprint(row[0])
		}

		conds[fmt.Sprint(row[0])] = strings.TrimSuffix(fmt.Sprint(row[1]), "<nil>")
	}

	if _, ok := seq["BuilderRemoveFolders"]; ok {
		t.Error("the immediate BuilderRemoveFolders is scheduled; the engine builds the data")
	}

	// Run the type-51 actions the way the engine would, from the first
	// reset to the commit action, with every entry's condition true and
	// REMOVE~="ALL" and RollbackDisabled false. Nothing, above all no
	// action that runs as the user, comes between them.
	values := map[string]string{"ProductCode": "{00000000-0000-0000-0000-000000000001}"}
	format := regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_.]*)\]`)
	first := seq["BuilderRemoveFoldersResetCount"]

	if first <= seq["RemoveFiles"] {
		t.Fatalf("the setters start at %d, not after RemoveFiles at %d", first, seq["RemoveFiles"])
	}

	number := first
	for ; byNumber[number] != "BuilderRemoveFoldersUndo"; number++ {
		name := byNumber[number]
		row := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == name })

		if fmt.Sprint(row[1]) != "51" || !strings.HasPrefix(name, "BuilderRemoveFolders") {
			t.Fatalf("action %s at %d between the setters: %v", name, number, row)
		}

		if c := conds[name]; c == `REMOVE~="ALL"` || c == "RollbackDisabled" {
			continue
		}

		values[fmt.Sprint(row[2])] = format.ReplaceAllStringFunc(fmt.Sprint(row[3]), func(ref string) string {
			name := ref[1 : len(ref)-1]
			if v, ok := values[name]; ok {
				return v
			}

			return `C:\` + name + `\`
		})
	}

	for i, action := range []struct {
		name string
		typ  int
	}{
		{"BuilderRemoveFoldersUndo", 1 | 0x500 | 0x800}, // DLL, rollback, no impersonation
		{"BuilderRemoveFoldersExec", 1 | 0x400 | 0x800}, // deferred
		{"BuilderRemoveFoldersCommit", 1 | 0x600 | 0x800},
	} {
		if byNumber[number+i] != action.name {
			t.Errorf("%s at %d, want %s", byNumber[number+i], number+i, action.name)
		}

		row := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == action.name })
		if fmt.Sprint(row[1]) != strconv.Itoa(action.typ) || row[2] != "BuilderRun" || row[3] != "BuilderRemoveFoldersExec" {
			t.Errorf("CustomAction row %v, want type %d", row, action.typ)
		}

		if conds[action.name] != `BuilderRemoveFoldersCount<>":"` {
			t.Errorf("%s condition %q", action.name, conds[action.name])
		}
	}

	if seq["BuilderRemoveFoldersCommit"] >= seq["RemoveFolders"] {
		t.Errorf("the actions end at %d, after RemoveFolders %d", seq["BuilderRemoveFoldersCommit"], seq["RemoveFolders"])
	}

	// The data: op, scope, ProductCode, `:` and one `#` per line, then the
	// standard folders, the package's directories and the entries.
	product := values["ProductCode"]
	for op, property := range map[string]string{"u": "BuilderRemoveFoldersUndo", "m": "BuilderRemoveFoldersExec", "c": "BuilderRemoveFoldersCommit"} {
		data := values[property]
		head, body, _ := strings.Cut(data, "\n")
		lines := strings.Split(body, "\n")

		if want := op + "k" + product + ":" + strings.Repeat("#", len(lines)); head != want {
			t.Errorf("%s head %q, want %q", property, head, want)
		}

		var kinds string

		for _, line := range lines {
			if !strings.HasSuffix(kinds, line[:1]) {
				kinds += line[:1]
			}
		}

		if kinds != "spf" {
			t.Errorf("%s line kinds %q, want s, p, f in that order", property, kinds)
		}

		for _, want := range []string{"sWindowsFolder|C:\\WindowsFolder\\", "pC:\\INSTALLDIR\\", "fINSTALLDIR|C:\\INSTALLDIR\\", "fDATA_DIR|C:\\DATA_DIR\\", "fOLD_DIR|C:\\OLD_DIR\\"} {
			if !slices.Contains(lines, want) {
				t.Errorf("%s lacks the line %q", property, want)
			}
		}
	}

	// The entries' conditions: a folder of the package goes only with the
	// whole product.
	comp := fmt.Sprint(cell(t, database, "Component", "Component", func(r []any) bool { return r[2] == "INSTALLDIR" }))
	removed := "($" + comp + "=2 AND NOT UPGRADINGPRODUCTCODE)"
	entryConds := map[string]string{}

	for name, cond := range conds {
		if strings.HasPrefix(name, "BuilderRemoveFoldersEntry") {
			row := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == name })
			entryConds[strings.TrimPrefix(fmt.Sprint(row[3]), "[BuilderRemoveFoldersData]\n")] = cond
		}
	}

	for line, want := range map[string]string{
		"fINSTALLDIR|[INSTALLDIR]": "($" + comp + `=2 AND REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE)`,
		"fDATA_DIR|[DATA_DIR]":     "(" + removed + " OR $" + comp + "=3) AND (DELETE_DATA=\"1\")",
		"fOLD_DIR|[OLD_DIR]":       "$" + comp + "=3",
	} {
		if entryConds[line] != want {
			t.Errorf("condition of %q = %q, want %q", line, entryConds[line], want)
		}
	}

	// With rollback disabled, which skips commit actions, the deferred
	// action deletes at once; with the whole product removed the scope is a.
	for name, want := range map[string][2]string{
		"BuilderRemoveFoldersSetDelete": {"RollbackDisabled", "d[BuilderRemoveFoldersScope][ProductCode]"},
		"BuilderRemoveFoldersAll":       {`REMOVE~="ALL"`, "a"},
	} {
		row := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == name })
		if conds[name] != want[0] || !strings.HasPrefix(fmt.Sprint(row[3]), want[1]) {
			t.Errorf("%s: condition %q target %v, want %q and %q", name, conds[name], row[3], want[0], want[1])
		}
	}

	rowWhere(t, database, "Binary", func(r []any) bool { return r[0] == "BuilderRun" })

	for _, row := range rows(t, database, "CustomAction") {
		if target := fmt.Sprint(row[3]); len(target) > 255 {
			t.Errorf("%s target is %d characters (ICE03)", row[0], len(target))
		}
	}

	secure := fmt.Sprint(cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "SecureCustomProperties" }))
	// The entries leave security to the properties' own settings: DATA_DIR
	// is secure by default, OLD_DIR opted out, DELETE_DATA is undeclared.
	for name, want := range map[string]bool{"DATA_DIR": true, "OLD_DIR": false, "DELETE_DATA": false} {
		if got := strings.Contains(";"+secure+";", ";"+name+";"); got != want {
			t.Errorf("SecureCustomProperties %q: %s secure %t, want %t", secure, name, got, want)
		}
	}
}
