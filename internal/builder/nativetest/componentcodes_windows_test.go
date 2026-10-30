package nativetest_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// componentCodesConfig is one version of the UPG-11 product. v1 stands for a
// package another tool built: every component code is pinned. The three
// guid lines are the codes of v2, empty for generated ones; menu is the
// component of the Start menu shortcut, or "" when v2 puts the shortcut on
// the file instead.
const componentCodesConfig = `schema: msi-builder/v1
product: { name: MsiBuilderComponentCodes, manufacturer: msi-builder tests, version: %s, upgradeCode: 9c4e2a71-5b3d-4f60-8e1a-2d7b6c9f0e43 }
package: { platform: x64 }
installDir: { name: MsiBuilderComponentCodes }
upgrade: { removeOld: %s }
components:
  - id: App
    %s
    files:
      - source: app.exe
        %s
      - source: config.yaml
    environment: [{ name: MSIBUILDER_COMPONENT_CODES, value: '[INSTALLDIR]' }]
  - id: Settings
    %s
    registry: [{ root: HKLM, key: 'SOFTWARE\MsiBuilderComponentCodes', name: Level, value: '1' }]
%s`

// Component codes of v1.
const (
	codeApp      = "guid: 1f0c7d52-8a4e-4b39-9d61-3e5a2c8b7f10"
	codeSettings = "guid: 6a2d9e14-3c7b-4f85-a0d2-5b8e1f4c9a27"
	codeMenu     = "guid: c83b5f06-2e9a-4d17-b4c8-7f1a6e3d2b59"
	menuOnFile   = "shortcuts: [{ name: MsiBuilderComponentCodes }]"
)

func menuComponent(guid string) string {
	return fmt.Sprintf(`  - id: Menu
    %s
    shortcuts: [{ name: MsiBuilderComponentCodes, file: app.exe }]
`, guid)
}

// TestMsiexecChangedComponentCodes upgrades a product with pinned component
// codes by a v2 whose codes changed, as a port from another tool with
// generated codes does (UPG-11). With a late removal the old product's
// uninstall is expected to delete every resource whose code changed, since
// no other product holds that code; with an early removal or with the old
// codes pinned in v2 everything stays.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecChangedComponentCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	inputs := map[string][]byte{
		"app.exe":     testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
		"config.yaml": []byte("level: 1\n"),
	}
	folder := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderComponentCodes")
	link := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\MsiBuilderComponentCodes.lnk`)

	const (
		settingsKey = `HKLM\SOFTWARE\MsiBuilderComponentCodes`
		envKey      = `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	)

	for _, tc := range []struct {
		name, removeOld     string
		app, settings, menu string
		fileShortcut        string
		filesKept, linkKept bool
	}{
		{"late removal, new codes", "afterInstallExecute", "", "", menuComponent(""), "", false, false},
		{"late removal, pinned codes", "afterInstallExecute", codeApp, codeSettings, menuComponent(codeMenu), "", true, true},
		{"early removal, new codes", "afterInstallInitialize", "", "", menuComponent(""), "", true, true},
		{"late removal, pinned codes, shortcut on the file", "afterInstallExecute", codeApp, codeSettings, "", menuOnFile, true, false},
	} {
		t.Run("UPG-11 "+tc.name, func(t *testing.T) {
			if _, err := os.Stat(folder); err == nil {
				t.Fatalf("%s exists before the installation; remove what an earlier run left", folder)
			}

			dir := t.TempDir()
			older, oldRes := buildConfigResult(t, mkdir(t, dir, "v1"), "v1", fmt.Sprintf(componentCodesConfig,
				"1.0.0", tc.removeOld, codeApp, "", codeSettings, menuComponent(codeMenu)), inputs)
			newer, newRes := buildConfigResult(t, mkdir(t, dir, "v2"), "v2", fmt.Sprintf(componentCodesConfig,
				"1.1.0", tc.removeOld, tc.app, tc.fileShortcut, tc.settings, tc.menu), inputs)

			t.Cleanup(func() {
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", newer, "/qn", "/norestart").Run()
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", older, "/qn", "/norestart").Run()
				_ = os.RemoveAll(folder)
				_ = os.Remove(link)
			})

			msiexecQuiet(t, filepath.Join(dir, "v1.log"), "/i", older)
			msiexecQuiet(t, filepath.Join(dir, "v2.log"), "/i", newer)

			got := map[string]bool{
				"app.exe":     present(filepath.Join(folder, "app.exe")),
				"config.yaml": present(filepath.Join(folder, "config.yaml")),
				"registry":    regHas(t, "64", settingsKey, "Level"),
				"environment": regHas(t, "64", envKey, "MSIBUILDER_COMPONENT_CODES"),
				"shortcut":    present(link),
			}
			t.Logf("after the upgrade: %v; v1 registered %t", got, productInstalled(t, oldRes.ProductCode))

			if productInstalled(t, oldRes.ProductCode) {
				t.Error("v1 is still registered after the upgrade")
			}

			for _, name := range []string{"app.exe", "config.yaml", "registry", "environment"} {
				if got[name] != tc.filesKept {
					t.Errorf("%s present: %t, want %t", name, got[name], tc.filesKept)
				}
			}

			if got["shortcut"] != tc.linkKept {
				t.Errorf("shortcut present: %t, want %t", got["shortcut"], tc.linkKept)
			}

			msiexecQuiet(t, filepath.Join(dir, "x2.log"), "/x", newer)

			if productInstalled(t, newRes.ProductCode) {
				t.Error("v2 is still registered after its uninstall")
			}

			if present(folder) || present(link) || regHas(t, "64", settingsKey, "") {
				t.Errorf("v2's uninstall left %s, %s or %s behind", folder, link, settingsKey)
			}
		})
	}
}
