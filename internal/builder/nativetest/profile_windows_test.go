//go:build windows

package nativetest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/ice"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// profileConfig installs a component to the roaming profile, and in a
// per-user package one to the install dir in the local profile. ${KEYPATH}
// pins the old file key paths; empty, the builder writes HKCU values
// (INST-04).
const profileConfig = `schema: msi-builder/v1
product: { name: MsiBuilderProfile, manufacturer: msi-builder tests, version: "${VERSION}", upgradeCode: 5b8e2c71-4d3a-4f9e-a1c6-7e0d2b9f4a58 }
package: { platform: x64, scope: ${SCOPE} }
components:
  - id: Data
    directory: AppDataFolder/MsiBuilderProfile/data
    ${DATAKEY}
    files: [{ source: tool.exe }, { source: readme.txt }]
${APP}${EXTRA}`

const profileApp = `  - id: App
    ${APPKEY}
    files: [{ source: app.exe }]
`

// buildProfile builds the profile package; pinned keeps the file key paths.
func buildProfile(t *testing.T, dir, scope, version string, pinned bool) (string, *builder.Result) {
	t.Helper()

	return buildProfileExtra(t, dir, scope, version, pinned, "")
}

// buildProfileExtra is buildProfile with more components.
func buildProfileExtra(t *testing.T, dir, scope, version string, pinned bool, extra string) (string, *builder.Result) {
	t.Helper()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	for name, data := range map[string][]byte{"tool.exe": exe, "app.exe": exe, "readme.txt": []byte("profile\r\n")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	dataKey, appKey := "", ""
	if pinned {
		dataKey, appKey = "keyPath: tool.exe", "keyPath: app.exe"
	}

	// The app component goes in first: one replacer pass would leave the
	// ${APPKEY} it brings along.
	app := ""
	if scope == "perUser" {
		app = strings.ReplaceAll(profileApp, "${APPKEY}", appKey)
	}

	doc := strings.NewReplacer("${APP}", app, "${VERSION}", version, "${SCOPE}", scope, "${DATAKEY}", dataKey, "${EXTRA}", extra).Replace(profileConfig)

	cfg, err := config.Parse([]byte(doc), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})
	if err != nil {
		t.Fatalf("%v\n%v", err, res.Diagnostics)
	}

	path := filepath.Join(dir, fmt.Sprintf("profile-%s-%s-%t.msi", scope, version, pinned))
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, res
}

// profileValues returns the names of the generated HKCU values, by
// component.
func profileValues(t *testing.T, res *builder.Result) map[string]string {
	t.Helper()

	values := make(map[string]string)

	for _, row := range res.Database.Table("Registry").Rows {
		if row[2] == `Software\[Manufacturer]\[ProductName]` {
			values[fmt.Sprint(row[5])] = fmt.Sprint(row[3])
		}
	}

	return values
}

const profileKey = `HKCU\Software\msi-builder tests\MsiBuilderProfile`

func profileFiles(scope string) []string {
	data := filepath.Join(os.Getenv("APPDATA"), "MsiBuilderProfile", "data")
	files := []string{filepath.Join(data, "tool.exe"), filepath.Join(data, "readme.txt")}

	if scope == "perUser" {
		files = append(files, filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "MsiBuilderProfile", "app.exe"))
	}

	return files
}

// checkProfile reports files and generated values that are not where want
// says.
func checkProfile(t *testing.T, when, scope string, values map[string]string, want bool) {
	t.Helper()

	for _, file := range profileFiles(scope) {
		if exists(file) != want {
			t.Errorf("%s: %s exists = %v, want %v", when, file, !want, want)
		}
	}

	for comp, name := range values {
		if regHas(t, "64", profileKey, name) != want {
			t.Errorf("%s: HKCU value %s of %s exists = %v, want %v", when, name, comp, !want, want)
		}
	}
}

// TestMsiexecProfileKeyPath installs components in the user profile whose
// key path is the HKCU value the builder writes (INST-04): a per-user
// package upgraded from a version with file key paths, then repaired after
// its files were deleted, and dual and per-machine packages in both
// contexts. The uninstall removes the files, the folders and the values.
func TestMsiexecProfileKeyPath(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	// The early removal removes the file-keyed product first. With the
	// late one, an HKCU key path installed while the old product still held
	// the same ComponentIds lost the files (CI runs 40966711286 and
	// 40967057615), so the builder keeps the file key paths there. The last
	// case goes the other way: from the HKCU key paths of an early-removal
	// version to the file key paths a late removal keeps. Its uninstall left
	// version 1's values behind (CI run 40974297757) until version 2 wrote
	// the same values as its own rows.
	early, late := "upgrade: { removeOld: afterInstallInitialize }\n", "upgrade: { removeOld: afterInstallExecute }\n"

	for _, tc := range []struct {
		name, oldUpgrade, newUpgrade string
		oldPinned                    bool
	}{
		{"file to HKCU, early removal", early, early, true},
		{"file to file, late removal", late, late, true},
		{"HKCU to file, late removal", early, late, false},
	} {
		t.Run("perUser upgrade and repair "+tc.name, func(t *testing.T) {
			profileUpgrade(t, tc.oldUpgrade, tc.newUpgrade, tc.oldPinned)
		})
	}

	for _, tc := range []struct { //nolint:paralleltest // msiexec runs one installation at a time
		name, scope, context string
		args                 []string
	}{
		{"perMachine", "perMachine", "machine", nil},
		{"perUserOrMachine per user", "perUserOrMachine", "user", nil},
		{"perUserOrMachine per machine", "perUserOrMachine", "machine", []string{`MSIINSTALLPERUSER=""`}},
		{"perMachineOrUser per machine", "perMachineOrUser", "machine", nil},
		{"perMachineOrUser per user", "perMachineOrUser", "user", []string{"MSIINSTALLPERUSER=1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path, res := buildProfile(t, dir, tc.scope, "1.0.0", false)
			values := profileValues(t, res)

			t.Cleanup(func() { _, _ = runMsiexec([]string{"/x", path, "/qn"}, nil) })

			if len(values) != 1 {
				t.Fatalf("generated values %v", values)
			}

			msiexecQuiet(t, filepath.Join(dir, "install.log"), append([]string{"/i", path}, tc.args...)...)
			checkProfile(t, "after the install", tc.scope, values, true)

			registered := where(regHas(t, "64", `HKCU\Software\Microsoft\Installer\Products\`+squash(res.ProductCode), ""),
				regHas(t, "64", `HKLM\SOFTWARE\Classes\Installer\Products\`+squash(res.ProductCode), ""))
			if registered != tc.context {
				t.Errorf("registered %s, want %s", registered, tc.context)
			}

			msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
			checkProfile(t, "after the uninstall", tc.scope, values, false)

			if folder := filepath.Join(os.Getenv("APPDATA"), "MsiBuilderProfile"); exists(folder) {
				t.Errorf("%s is still there after the uninstall", folder)
			}
		})
	}
}

// profileUpgrade installs version 1 of the per-user package, with file key
// paths when oldPinned is set, upgrades it to version 2 as the builder
// makes it by default, which writes the HKCU values also when a late
// removal keeps the file key paths, deletes the files, repairs and
// uninstalls it. A full repair restores missing files whatever the key
// path is.
func profileUpgrade(t *testing.T, oldUpgrade, newUpgrade string, oldPinned bool) {
	t.Helper()

	dir := t.TempDir()
	old, oldRes := buildProfileExtra(t, dir, "perUser", "1.0.0", oldPinned, oldUpgrade)
	path, res := buildProfileExtra(t, dir, "perUser", "2.0.0", false, newUpgrade)
	values := profileValues(t, res)
	oldValues := profileValues(t, oldRes)

	t.Cleanup(func() {
		_, _ = runMsiexec([]string{"/x", path, "/qn"}, nil)
		_, _ = runMsiexec([]string{"/x", old, "/qn"}, nil)
	})

	wantOld := 2
	if oldPinned {
		wantOld = 0
	}

	if len(values) != 2 || len(oldValues) != wantOld {
		t.Fatalf("generated values %v, old %v", values, oldValues)
	}

	msiexecQuiet(t, filepath.Join(dir, "install-1.log"), "/i", old)
	checkProfile(t, "after the first install", "perUser", oldValues, true)

	msiexecQuiet(t, filepath.Join(dir, "upgrade.log"), "/i", path)
	checkProfile(t, "after the upgrade", "perUser", values, true)
	logOldValues(t, "after the upgrade", oldValues)

	for _, file := range profileFiles("perUser") {
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
	}

	msiexecQuiet(t, filepath.Join(dir, "repair.log"), "/f", path)
	checkProfile(t, "after the repair", "perUser", values, true)

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	checkProfile(t, "after the uninstall", "perUser", values, false)

	// Version 2 owns the values version 1 wrote, under the same names, so
	// its uninstall removes them (checked above).
	for comp, name := range oldValues {
		if values[comp] != name {
			t.Errorf("version 1 names the HKCU value of %s %s, version 2 %s", comp, name, values[comp])
		}
	}

	for _, folder := range []string{
		filepath.Join(os.Getenv("APPDATA"), "MsiBuilderProfile"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "MsiBuilderProfile"),
	} {
		if exists(folder) {
			t.Errorf("%s is still there after the uninstall", folder)
		}
	}
}

// shellFolderFiles installs files to the desktop and a Start menu folder,
// which are the all-users folders in a per-machine install.
const shellFolderFiles = `  - { id: Desk, directory: DesktopFolder, files: [{ source: readme.txt, name: MsiBuilderProfile.txt }] }
  - { id: Menu, directory: ProgramMenuFolder/MsiBuilderProfile, files: [{ source: readme.txt, name: about.txt }] }
`

// TestICEProfileKeyPath runs the ICE suite on the profile packages of every
// scope, with files on the desktop and in the Start menu in the per-user
// and per-machine ones: ICE38 and ICE64 report nothing and no ICE reports
// an error. The file-keyed variant shows that ICE38 finds the old layout,
// and so do the late-removal variants, whose components keep their key
// files next to the HKCU value.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEProfileKeyPath(t *testing.T) {
	cub := iceCube(t)

	for _, tc := range []struct {
		scope        string
		pinned, late bool
	}{
		{"perUser", false, false}, {"perMachine", false, false}, {"perUserOrMachine", false, false},
		{"perMachineOrUser", false, false}, {"perUser", true, false}, {"perUser", false, true}, {"perMachine", false, true},
	} {
		t.Run(fmt.Sprintf("%s pinned=%t late=%t", tc.scope, tc.pinned, tc.late), func(t *testing.T) {
			extra := ""
			if tc.scope == "perUser" || tc.scope == "perMachine" {
				extra = shellFolderFiles
			}

			if tc.late {
				extra += "upgrade: { removeOld: afterInstallExecute }\n"
			}

			path, _ := buildProfileExtra(t, t.TempDir(), tc.scope, "1.0.0", tc.pinned, extra)

			findings, err := ice.Run(path, cub)
			if err != nil {
				t.Fatal(err)
			}

			var ice38 int

			for _, finding := range findings {
				switch {
				case finding.Type == ice.Info:
				case finding.ICE == "ICE38":
					ice38++

					t.Log(finding)
				case finding.ICE == "ICE64" || finding.Type == ice.Error || finding.Type == ice.Failure:
					t.Error(finding)
				default:
					t.Log(finding)
				}
			}

			if (ice38 > 0) != (tc.pinned || tc.late) {
				t.Errorf("%d ICE38 findings, want some = %v", ice38, tc.pinned || tc.late)
			}
		})
	}
}

// TestICEProfileHKMUKeyPath checks that ICE38 accepts a component in the
// profile whose own key path is an HKMU value (root -1) in a per-user
// package, which keeps it as key path (dual packages: see
// TestProfileKeyPathMachineRegistry).
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEProfileHKMUKeyPath(t *testing.T) {
	cub := iceCube(t)

	const extra = `  - id: Settings
    directory: AppDataFolder/MsiBuilderProfile/settings
    registry: [{ root: HKMU, key: Software\MsiBuilderProfile, name: x, type: dword, value: 1 }]
`

	for _, scope := range []string{"perUser"} {
		t.Run(scope, func(t *testing.T) {
			path, _ := buildProfileExtra(t, t.TempDir(), scope, "1.0.0", false, extra)

			findings, err := ice.Run(path, cub)
			if err != nil {
				t.Fatal(err)
			}

			for _, finding := range findings {
				switch {
				case finding.Type == ice.Info:
				case finding.ICE == "ICE38" || finding.Type == ice.Error || finding.Type == ice.Failure:
					t.Error(finding)
				default:
					t.Log(finding)
				}
			}
		})
	}
}

// logOldValues logs which HKCU values of the first version are still there.
func logOldValues(t *testing.T, when string, values map[string]string) {
	t.Helper()

	for comp, name := range values {
		t.Logf("INST-04 %s: HKCU value %s of %s from version 1 present = %v", when, name, comp, regHas(t, "64", profileKey, name))
	}
}
