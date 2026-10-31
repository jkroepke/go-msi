package nativetest_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// upgradeFloorConfig is one version of the UPG-13 product: the version, the
// upgrade mapping and extra top-level keys.
const upgradeFloorConfig = `schema: msi-builder/v1
product: { name: MsiBuilderUpgradeFloor, manufacturer: msi-builder tests, version: %s, upgradeCode: 8c2f6a41-3d7e-4b95-a0c8-1e5f9b7d3a62 }
package: { platform: x64 }
installDir: { name: MsiBuilderUpgradeFloor }
upgrade: %s
components:
  - files: [{ source: app.exe }]
%s`

// blockBelowFloor detects a version below the floor of 1.0.0 with a
// detect-only row of the product's own upgrade code and refuses to install
// next to it.
const blockBelowFloor = `upgrade: { minVersion: 1.0.0, related: [{ upgradeCode: 8c2f6a41-3d7e-4b95-a0c8-1e5f9b7d3a62, maxVersion: 1.0.0, onlyDetect: true, property: BELOWFLOORFOUND }] }`

// TestMsiexecUpgradeFloor upgrades an installed version with a v2 whose
// upgrade.minVersion is 1.0.0 (UPG-13). A version at the floor is expected
// to be removed when the floor is inclusive; one below it, or at an
// exclusive floor, to stay registered next to v2, sharing app.exe, which
// the uninstall of v2 then leaves for it. With a detect-only row below the
// floor and a launch condition, v2 is expected to refuse to install.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecUpgradeFloor(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	folder := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderUpgradeFloor")

	for _, tc := range []struct {
		name, oldVersion, upgrade, extra string
		// code is the exit code of the v2 install; oldStays tells
		// whether v1 is still registered afterwards.
		code     int
		oldStays bool
	}{
		{"below the floor", "0.9.0", "{ minVersion: 1.0.0 }", "", 0, true},
		{"at an inclusive floor", "1.0.0", "{ minVersion: 1.0.0 }", "", 0, false},
		{"at an exclusive floor", "1.0.0", "{ minVersion: 1.0.0, includeMin: false }", "", 0, true},
		{"above the floor", "1.1.0", "{ minVersion: 1.0.0 }", "", 0, false},
		{
			"below the floor, blocked", "0.9.0", strings.TrimPrefix(blockBelowFloor, "upgrade: "),
			"launchConditions: [{ condition: NOT BELOWFLOORFOUND, message: Remove the old version first. }]\n", 1603, true,
		},
	} {
		t.Run("UPG-13 "+tc.name, func(t *testing.T) {
			if _, err := os.Stat(folder); err == nil {
				t.Fatalf("%s exists before the installation; remove what an earlier run left", folder)
			}

			dir := t.TempDir()
			inputs := map[string][]byte{"app.exe": app}
			older, oldRes := buildConfigResult(t, mkdir(t, dir, "v1"), "v1",
				fmt.Sprintf(upgradeFloorConfig, tc.oldVersion, "{}", ""), inputs)
			newer, newRes := buildConfigResult(t, mkdir(t, dir, "v2"), "v2",
				fmt.Sprintf(upgradeFloorConfig, "1.2.0", tc.upgrade, tc.extra), inputs)

			t.Cleanup(func() {
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", newer, "/qn", "/norestart").Run()
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", older, "/qn", "/norestart").Run()
				_ = os.RemoveAll(folder)
			})

			msiexecQuiet(t, filepath.Join(dir, "v1.log"), "/i", older)

			log := filepath.Join(dir, "v2.log")
			code := msiexecCode(t, log, "/i", newer)

			for line := range strings.Lines(readLog(log)) {
				if strings.Contains(line, "FindRelatedProducts") || strings.Contains(line, "RemoveExistingProducts") ||
					strings.Contains(line, "BELOWFLOORFOUND") || strings.Contains(line, "Product: MsiBuilderUpgradeFloor") {
					t.Logf("%s", strings.TrimSpace(line))
				}
			}

			oldInstalled := productInstalled(t, oldRes.ProductCode)
			newInstalled := productInstalled(t, newRes.ProductCode)
			t.Logf("v2 exit %d, v1 %s registered %t, v2 registered %t", code, tc.oldVersion, oldInstalled, newInstalled)

			if code != tc.code {
				t.Errorf("msiexec /i v2 returned %d, want %d\n%s", code, tc.code, readLog(log))
			}

			if oldInstalled != tc.oldStays {
				t.Errorf("v1 registered: %t, want %t", oldInstalled, tc.oldStays)
			}

			if newInstalled != (tc.code == 0) {
				t.Errorf("v2 registered: %t, want %t", newInstalled, tc.code == 0)
			}

			if newInstalled {
				msiexecQuiet(t, filepath.Join(dir, "x2.log"), "/x", newer)
			}

			_, statErr := os.Stat(filepath.Join(folder, "app.exe"))
			t.Logf("app.exe after v2 is gone: present %t", statErr == nil)

			if oldInstalled && statErr != nil {
				t.Errorf("app.exe is gone although v1 is still installed: %v", statErr)
			}

			if oldInstalled {
				msiexecQuiet(t, filepath.Join(dir, "x1.log"), "/x", older)
			}

			if _, err := os.Stat(folder); err == nil {
				t.Errorf("%s is still there after both versions were removed", folder)
			}
		})
	}
}
