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

	// The early removal removes the file-keyed product first; the late one
	// installs the new key path while the old product still holds the
	// same ComponentIds.
	for _, removeOld := range []string{"afterInstallInitialize", "afterInstallExecute"} {
		t.Run("perUser upgrade and repair "+removeOld, func(t *testing.T) {
			profileUpgrade(t, "upgrade: { removeOld: "+removeOld+" }\n")
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

// profileUpgrade installs the per-user package with file key paths,
// upgrades it to the one with HKCU key paths, deletes the files, repairs
// and uninstalls it.
func profileUpgrade(t *testing.T, upgrade string) {
	t.Helper()

	dir := t.TempDir()
	old, oldRes := buildProfileExtra(t, dir, "perUser", "1.0.0", true, upgrade)
	path, res := buildProfileExtra(t, dir, "perUser", "2.0.0", false, upgrade)
	values := profileValues(t, res)

	t.Cleanup(func() {
		_, _ = runMsiexec([]string{"/x", path, "/qn"}, nil)
		_, _ = runMsiexec([]string{"/x", old, "/qn"}, nil)
	})

	if len(values) != 2 || len(profileValues(t, oldRes)) != 0 {
		t.Fatalf("generated values %v, old %v", values, profileValues(t, oldRes))
	}

	msiexecQuiet(t, filepath.Join(dir, "install-1.log"), "/i", old)
	checkProfile(t, "after the file-keyed install", "perUser", nil, true)

	msiexecQuiet(t, filepath.Join(dir, "upgrade.log"), "/i", path)
	checkProfile(t, "after the upgrade", "perUser", values, true)

	for _, file := range profileFiles("perUser") {
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
	}

	msiexecQuiet(t, filepath.Join(dir, "repair.log"), "/f", path)
	checkProfile(t, "after the repair", "perUser", values, true)

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	checkProfile(t, "after the uninstall", "perUser", values, false)

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
// an error. The file-keyed variant shows that ICE38 finds the old layout.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEProfileKeyPath(t *testing.T) {
	cub := iceCube(t)

	for _, tc := range []struct {
		scope  string
		pinned bool
	}{
		{"perUser", false}, {"perMachine", false}, {"perUserOrMachine", false}, {"perMachineOrUser", false}, {"perUser", true},
	} {
		t.Run(fmt.Sprintf("%s pinned=%t", tc.scope, tc.pinned), func(t *testing.T) {
			extra := ""
			if tc.scope == "perUser" || tc.scope == "perMachine" {
				extra = shellFolderFiles
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

			if (ice38 > 0) != tc.pinned {
				t.Errorf("%d ICE38 findings, want some = %v", ice38, tc.pinned)
			}
		})
	}
}

// TestICEProfileHKMUKeyPath shows how ICE38 judges a component in the
// profile whose own key path is an HKMU value (root -1) in a per-user
// package, which keeps it; a dual package gets the HKCU value instead.
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
				if finding.Type != ice.Info {
					t.Logf("PROBE %s: %v", scope, finding)
				}
			}
		})
	}
}
