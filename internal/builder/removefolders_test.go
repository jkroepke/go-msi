package builder_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestRemoveFolders writes one anchor RemoveFile row per removeFolders
// entry, lists the entries with their conditions in BuilderRemoveFolders,
// schedules the action after CostFinalize, embeds the DLL and makes the
// public properties secure.
func TestRemoveFolders(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files:
      - source: app.exe
    removeFolders:
      - property: INSTALLDIR
      - { property: DATA_DIR, on: both, condition: DELETE_DATA="1" }
      - { property: OLD_DIR, on: install }
properties:
  DATA_DIR: { value: 'C:\Data\' }
  OLD_DIR: { value: 'C:\Old\' }
`)
	database := mustBuild(t, cfg).Database

	modes := make(map[string]string)

	for _, row := range rows(t, database, "RemoveFile") {
		key, ok := row[0].(string)
		if !ok || !strings.HasPrefix(key, "BuilderTree") {
			continue
		}

		if row[2] != "*" || row[3] != row[0] {
			t.Errorf("anchor row %v: want FileName * and DirProperty = FileKey", row)
		}

		modes[fmt.Sprint(row[4])] = key
	}

	if len(modes) != 3 || modes["1"] == "" || modes["2"] == "" || modes["3"] == "" {
		t.Errorf("anchor rows by mode %v, want 1, 2 and 3", modes)
	}

	list := fmt.Sprint(cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "BuilderRemoveFolders" }))
	lines := strings.Split(list, "\n")

	if len(lines) != 3 {
		t.Fatalf("BuilderRemoveFolders %q, want 3 lines", list)
	}

	comp := fmt.Sprint(cell(t, database, "Component", "Component", func(r []any) bool { return r[2] == "INSTALLDIR" }))

	for i, want := range []string{
		"$" + comp + "=2 AND NOT UPGRADINGPRODUCTCODE\x1f" + comp + "\x1f2\x1f" + modes["2"] + "\x1fINSTALLDIR",
		"($" + comp + "=3 OR ($" + comp + "=2 AND NOT UPGRADINGPRODUCTCODE)) AND (DELETE_DATA=\"1\")\x1f" + comp + "\x1f3\x1f" + modes["3"] + "\x1fDATA_DIR",
		"$" + comp + "=3\x1f" + comp + "\x1f1\x1f" + modes["1"] + "\x1fOLD_DIR",
	} {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}

	action := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == "BuilderRemoveFolders" })
	if fmt.Sprint(action[1]) != "1" || action[2] != "BuilderRun" || action[3] != "BuilderRemoveFolders" {
		t.Errorf("CustomAction row %v", action)
	}

	seq := func(name string) int {
		n, ok := cell(t, database, "InstallExecuteSequence", "Sequence", func(r []any) bool { return r[0] == name }).(int)
		if !ok {
			t.Fatalf("%s has no sequence number", name)
		}

		return n
	}

	if at := seq("BuilderRemoveFolders"); at <= seq("CostFinalize") || at >= seq("InstallValidate") {
		t.Errorf("BuilderRemoveFolders at %d, want between CostFinalize %d and InstallValidate %d", at, seq("CostFinalize"), seq("InstallValidate"))
	}

	rowWhere(t, database, "Binary", func(r []any) bool { return r[0] == "BuilderRun" })

	secure := fmt.Sprint(cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "SecureCustomProperties" }))
	for _, want := range []string{"DATA_DIR", "DELETE_DATA", "OLD_DIR", "INSTALLDIR"} {
		if !strings.Contains(";"+secure+";", ";"+want+";") {
			t.Errorf("SecureCustomProperties %q lacks %s", secure, want)
		}
	}
}
