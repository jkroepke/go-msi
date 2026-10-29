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

// removeFailureConfig is one version of the UPG-10 product. The old version
// adds failOnRemove, an error action that fails every removal unless
// KEEPGOING is set, so that RemoveExistingProducts cannot remove it.
const removeFailureConfig = `schema: msi-builder/v1
product: { name: MsiBuilderRemoveFailure, manufacturer: msi-builder tests, version: %s, upgradeCode: 5e3b9d17-0c2a-4f8e-b6d4-7a1c9e3f5b80 }
package: { platform: x64 }
installDir: { name: MsiBuilderRemoveFailure }
upgrade: { removeOld: %s, ignoreRemoveFailure: %t }
properties:
  KEEPGOING: { secure: true }
components:
  - files: [{ source: app.exe }]
%s`

const failOnRemove = `customActions:
  - id: FailRemove
    error: The old version refuses to be removed.
    schedule: [{ sequence: execute, after: InstallInitialize, condition: REMOVE~="ALL" AND NOT KEEPGOING }]
`

// TestMsiexecIgnoreRemoveFailure upgrades a product whose removal fails
// (UPG-10). Without upgrade.ignoreRemoveFailure the upgrade is expected to
// fail with 1603 and roll back; with it, the new version installs with exit
// 0 in every removeOld window and the old product stays registered.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecIgnoreRemoveFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	folder := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderRemoveFailure")

	for _, tc := range []struct {
		removeOld string
		ignore    bool
		// code is the exit code of the v2 install; newInstalled tells
		// whether v2 is registered afterwards. v1 always stays.
		code         int
		newInstalled bool
	}{
		{"afterInstallInitialize", false, 1603, false},
		{"afterInstallValidate", true, 0, true},
		{"afterInstallInitialize", true, 0, true},
		{"afterInstallExecute", true, 0, true},
		{"afterInstallFinalize", true, 0, true},
	} {
		t.Run(fmt.Sprintf("UPG-10 %s ignore=%t", tc.removeOld, tc.ignore), func(t *testing.T) {
			if _, err := os.Stat(folder); err == nil {
				t.Fatalf("%s exists before the installation; remove what an earlier run left", folder)
			}

			dir := t.TempDir()
			inputs := map[string][]byte{"app.exe": app}
			older, oldRes := buildConfigResult(t, mkdir(t, dir, "v1"), "v1",
				fmt.Sprintf(removeFailureConfig, "1.0.0", tc.removeOld, tc.ignore, failOnRemove), inputs)
			newer, newRes := buildConfigResult(t, mkdir(t, dir, "v2"), "v2",
				fmt.Sprintf(removeFailureConfig, "1.1.0", tc.removeOld, tc.ignore, ""), inputs)

			t.Cleanup(func() {
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", newer, "/qn").Run()
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", older, "/qn", "KEEPGOING=1").Run()
				_ = os.RemoveAll(folder)
			})

			msiexecQuiet(t, filepath.Join(dir, "v1.log"), "/i", older)

			log := filepath.Join(dir, "v2.log")
			code := msiexecCode(t, log, "/i", newer)

			for line := range strings.Lines(readLog(log)) {
				if strings.Contains(line, "RemoveExistingProducts") || strings.Contains(line, "FindRelatedProducts: Found application") ||
					strings.Contains(line, "Product: MsiBuilderRemoveFailure") {
					t.Logf("%s", strings.TrimSpace(line))
				}
			}

			oldInstalled := productInstalled(t, oldRes.ProductCode)
			newInstalled := productInstalled(t, newRes.ProductCode)
			_, statErr := os.Stat(filepath.Join(folder, "app.exe"))
			t.Logf("v2 exit %d, v1 registered %t, v2 registered %t, app.exe present %t", code, oldInstalled, newInstalled, statErr == nil)

			if code != tc.code {
				t.Errorf("msiexec /i v2 returned %d, want %d\n%s", code, tc.code, readLog(log))
			}

			if !oldInstalled {
				t.Error("v1 is no longer registered although its removal failed")
			}

			if newInstalled != tc.newInstalled {
				t.Errorf("v2 registered: %t, want %t", newInstalled, tc.newInstalled)
			}

			if statErr != nil {
				t.Errorf("app.exe after the v2 install: %v", statErr)
			}

			if newInstalled {
				msiexecQuiet(t, filepath.Join(dir, "x2.log"), "/x", newer)
			}

			msiexecQuiet(t, filepath.Join(dir, "x1.log"), "/x", older, "KEEPGOING=1")

			if _, err := os.Stat(folder); err == nil {
				t.Errorf("%s is still there after both versions were removed", folder)
			}
		})
	}
}
