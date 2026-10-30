package builder_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestRemoveFolders lists the removeFolders entries with their conditions
// in BuilderRemoveFolders, schedules the immediate action and its rollback,
// deferred and commit actions right after RemoveFiles, embeds the DLL and
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

	list := fmt.Sprint(cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "BuilderRemoveFolders" }))
	lines := strings.Split(list, "\n")

	if len(lines) != 3 {
		t.Fatalf("BuilderRemoveFolders %q, want 3 lines", list)
	}

	comp := fmt.Sprint(cell(t, database, "Component", "Component", func(r []any) bool { return r[2] == "INSTALLDIR" }))
	removed := "($" + comp + "=2 AND NOT UPGRADINGPRODUCTCODE)"

	for i, want := range []string{
		// A folder of the package goes only with the whole product.
		"($" + comp + `=2 AND REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE)` + "\x1fINSTALLDIR",
		"(" + removed + " OR $" + comp + "=3) AND (DELETE_DATA=\"1\")\x1fDATA_DIR",
		"$" + comp + "=3\x1fOLD_DIR",
	} {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}

	seq := func(name string) int {
		n, ok := cell(t, database, "InstallExecuteSequence", "Sequence", func(r []any) bool { return r[0] == name }).(int)
		if !ok {
			t.Fatalf("%s has no sequence number", name)
		}

		return n
	}

	last := seq("RemoveFiles")

	for _, action := range []struct {
		name, typ, cond string
	}{
		{"BuilderRemoveFolders", "1", ""},
		{"BuilderRemoveFoldersUndo", "11521", "BuilderRemoveFoldersUndo"},
		{"BuilderRemoveFoldersExec", "11265", "BuilderRemoveFoldersExec"},
		{"BuilderRemoveFoldersCommit", "11777", "BuilderRemoveFoldersCommit"},
	} {
		row := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == action.name })

		target := "BuilderRemoveFoldersExec"
		if action.name == "BuilderRemoveFolders" {
			target = action.name
		}

		if fmt.Sprint(row[1]) != action.typ || row[2] != "BuilderRun" || row[3] != target {
			t.Errorf("CustomAction row %v, want type %s and target %s", row, action.typ, target)
		}

		if next := seq(action.name); next <= last || next >= seq("RemoveFolders") {
			t.Errorf("%s at %d, want after %d and before RemoveFolders %d", action.name, next, last, seq("RemoveFolders"))
		} else {
			last = next
		}

		cond := cell(t, database, "InstallExecuteSequence", "Condition", func(r []any) bool { return r[0] == action.name })
		if c, ok := cond.(string); (ok && c != action.cond) || (!ok && action.cond != "") {
			t.Errorf("%s condition %v, want %q", action.name, cond, action.cond)
		}
	}

	rowWhere(t, database, "Binary", func(r []any) bool { return r[0] == "BuilderRun" })

	hidden := fmt.Sprint(cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "MsiHiddenProperties" }))
	for _, want := range []string{"BuilderRemoveFoldersUndo", "BuilderRemoveFoldersExec", "BuilderRemoveFoldersCommit"} {
		if !strings.Contains(";"+hidden+";", ";"+want+";") {
			t.Errorf("MsiHiddenProperties %q lacks %s", hidden, want)
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
