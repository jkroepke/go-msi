package builder_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestRemoveFiles writes one RemoveFile row per removeFiles entry, with a
// generated short name for a long pattern, and RemoveFile rows for the
// folders a component creates in the user profile.
func TestRemoveFiles(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files:
      - source: app.exe
    removeFiles:
      - name: '*.log'
      - { name: acme-server-*.log, on: both }
      - { id: StaleConfig, name: stale.txt, on: install }
      - {}
  - directory: ProgramMenuFolder/Acme/Tools
    registry: [{ root: HKCU, key: Software\Acme, name: tools, type: dword, value: 1 }]
`)
	database := mustBuild(t, cfg).Database

	byName := make(map[any][]any)

	for _, row := range rows(t, database, "RemoveFile") {
		byName[row[2]] = row
	}

	if row := byName["*.log"]; row == nil || fmt.Sprint(row[4]) != "2" {
		t.Errorf("*.log row %v", row)
	}

	var long []any

	for name, row := range byName {
		if s, ok := name.(string); ok && strings.HasSuffix(s, "|acme-server-*.log") {
			long = row
		}
	}

	if long == nil || fmt.Sprint(long[4]) != "3" {
		t.Errorf("long pattern row %v in %v", long, byName)
	}

	if row := byName["stale.txt"]; row == nil || row[0] != "StaleConfig" || fmt.Sprint(row[4]) != "1" {
		t.Errorf("stale.txt row %v", row)
	}

	folders := 0

	for _, row := range rows(t, database, "RemoveFile") {
		if row[2] == nil && fmt.Sprint(row[4]) == "2" {
			folders++
		}
	}

	// The empty entry, plus Acme and Tools below ProgramMenuFolder.
	if folders != 3 {
		t.Errorf("%d folder rows, want 3: %v", folders, byName)
	}
}
