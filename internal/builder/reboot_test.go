package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestScheduleReboot places ScheduleReboot after InstallFinalize with its
// when condition, and lets a custom action use it as an anchor.
func TestScheduleReboot(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: a.txt }]
scheduleReboot: { when: install, condition: 'NOT NO_REBOOT' }
customActions:
  - id: AfterReboot
    setProperty: { name: A, value: "1" }
    schedule: [{ sequence: execute, after: ScheduleReboot }]
`), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.BaseDir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := msidb.Unmarshal(mustBuild(t, cfg).Package)
	if err != nil {
		t.Fatal(err)
	}

	row := rowWhere(t, database, "InstallExecuteSequence", func(r []any) bool { return r[0] == "ScheduleReboot" })
	if row == nil || fmt.Sprint(row[2]) != "6601" || row[1] != "(NOT Installed AND NOT OLDERVERSIONBEINGUPGRADED) AND (NOT NO_REBOOT)" {
		t.Errorf("ScheduleReboot row %v", row)
	}

	if got := cell(t, database, "InstallExecuteSequence", "Sequence", func(r []any) bool { return r[0] == "AfterReboot" }); fmt.Sprint(got) != "6602" {
		t.Errorf("AfterReboot is at %v, want 6602", got)
	}
}
