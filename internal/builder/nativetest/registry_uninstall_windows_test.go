//go:build windows

package nativetest_test

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// removeOnUninstallConfig removes keys only when its components are
// removed (REG-08): a key the program creates at run time in both views,
// a key that exists before the install, a shell verb under HKCR, a key
// removed on install and uninstall, and a key of a 32-bit component. The
// placeholders are the version and upgrade.removeOld.
const removeOnUninstallConfig = `schema: msi-builder/v1
product: { name: MsiBuilderUninstallKey, manufacturer: msi-builder tests, version: %s, upgradeCode: 0b7e3a95-6c2d-4f18-a4e9-71d5c3b80f26 }
package: { platform: x64 }
upgrade: { removeOld: %s }
directories:
  - id: ProgramFiles64Folder
    children: [{ id: INSTALLDIR, name: MsiBuilderUninstallKey }]
  - id: ProgramFilesFolder
    children: [{ id: DIR32, name: MsiBuilderUninstallKey }]
components:
  - id: App
    files: [{ source: app.exe }]
    removeRegistry:
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderUninstallKeyTest\Tree', on: uninstall }
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderUninstallKeyTest\Existing', on: uninstall }
      - { root: HKCR, key: 'Directory\shell\MsiBuilderTestVerb', on: uninstall }
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderUninstallKeyTest\Both', on: both }
  - id: Old32
    directory: DIR32
    bitness: "32"
    removeRegistry:
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderUninstallKeyTest\Only32', on: uninstall }
`

// buildRemoveOnUninstall writes the REG-08 package of one version.
func buildRemoveOnUninstall(t *testing.T, dir, version, removeOld string) string {
	t.Helper()

	exe := testpe.Build(testpe.Options{FileVersion: version + ".0", Languages: []int{1033}})

	return buildConfig(t, dir, "uninstallkey-"+version, fmt.Sprintf(removeOnUninstallConfig, version, removeOld),
		map[string][]byte{"app.exe": exe})
}

// regKey is a registry key in one view.
type regKey struct{ view, key string }

// TestMsiexecRemovesKeysOnUninstall installs, repairs, upgrades and removes
// a package whose removeRegistry entries act at uninstall (REG-08): a
// Registry row named - with a Null value. Microsoft documents that it
// deletes the key with its subkeys when the component is removed; the test
// checks that the install neither creates nor deletes the key, what a
// repair and the old product's removal in a major upgrade do, and that the
// uninstall stays in the component's registry view.
func TestMsiexecRemovesKeysOnUninstall(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	dir := t.TempDir()
	first := buildRemoveOnUninstall(t, dir, "1.0.0", "afterInstallInitialize")
	early := buildRemoveOnUninstall(t, dir, "1.0.1", "afterInstallInitialize")
	late := buildRemoveOnUninstall(t, dir, "1.0.2", "afterInstallExecute")

	base := `HKLM\SOFTWARE\MsiBuilderUninstallKeyTest`
	verb := `HKLM\SOFTWARE\Classes\Directory\shell\MsiBuilderTestVerb`

	cleanup := func() {
		ctx := context.Background()
		for _, pkg := range []string{late, early, first} {
			_ = exec.CommandContext(ctx, "msiexec", "/x", pkg, "/qn").Run()
		}

		for _, view := range []string{"/reg:64", "/reg:32"} {
			_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\MsiBuilderUninstallKeyTest`, "/f", view).Run()
			_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\Classes\Directory\shell\MsiBuilderTestVerb`, "/f", view).Run()
		}
	}

	cleanup()
	t.Cleanup(cleanup)

	tree64, tree32 := regKey{"64", base + `\Tree`}, regKey{"32", base + `\Tree`}
	existing, both := regKey{"64", base + `\Existing`}, regKey{"64", base + `\Both`}
	verb64 := regKey{"64", verb}
	only32, only32in64 := regKey{"32", base + `\Only32`}, regKey{"64", base + `\Only32`}

	// What the program would create while it runs.
	seed := func() {
		regSet(t, "64", tree64.key+`\Sub`, "Value", "runtime")
		regSet(t, "32", tree32.key, "Value", "runtime")
		regSet(t, "64", verb+`\command`, "Value", `"C:\app.exe" "%1"`)
		regSet(t, "32", only32.key, "Value", "runtime")
		regSet(t, "64", only32in64.key, "Value", "runtime")
		regSet(t, "64", both.key, "Value", "runtime")
	}

	has := func(k regKey) bool { return regHas(t, k.view, k.key, "") }
	report := func(step string) {
		for _, k := range []regKey{tree64, tree32, existing, both, verb64, only32, only32in64} {
			t.Logf("REG-08 %s: %s (view %s) present=%v", step, k.key, k.view, has(k))
		}
	}
	expect := func(step string, present, absent []regKey) {
		t.Helper()
		report(step)

		for _, k := range present {
			if !has(k) {
				t.Errorf("%s: %s (view %s) is gone", step, k.key, k.view)
			}
		}

		for _, k := range absent {
			if has(k) {
				t.Errorf("%s: %s (view %s) is still there", step, k.key, k.view)
			}
		}
	}

	// A key that exists before the install, with a subkey.
	regSet(t, "64", existing.key+`\Sub`, "Value", "old")
	regSet(t, "64", both.key, "Value", "old")

	if code := msiexecCode(t, filepath.Join(dir, "install.log"), "/i", first); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(filepath.Join(dir, "install.log")))
	}

	// The install creates none of the keys and deletes only the both key.
	expect("install", []regKey{existing, {"64", existing.key + `\Sub`}}, []regKey{tree64, verb64, both, only32})

	if got := regData(t, existing.key+`\Sub`, "Value"); got != "old" {
		t.Errorf(`Existing\Sub\Value is %q after the install, want old`, got)
	}

	seed()

	if code := msiexecCode(t, filepath.Join(dir, "repair.log"), "/fomus", first); code != 0 {
		t.Fatalf("msiexec /fomus returned %d", code)
	}

	expect("repair", []regKey{tree64, tree32, existing, verb64, only32}, []regKey{both})

	// The old product's removal in an early major upgrade removes its
	// components, so their keys go although the new version has the
	// same entries.
	seed()

	if code := msiexecCode(t, filepath.Join(dir, "upgrade-early.log"), "/i", early); code != 0 {
		t.Fatalf("msiexec /i early returned %d", code)
	}

	expect("early upgrade", []regKey{tree32, only32in64}, []regKey{tree64, existing, verb64, only32})

	// In a late major upgrade the new version keeps the components
	// installed while the old product goes.
	seed()

	if code := msiexecCode(t, filepath.Join(dir, "upgrade-late.log"), "/i", late); code != 0 {
		t.Fatalf("msiexec /i late returned %d", code)
	}

	expect("late upgrade", []regKey{tree64, tree32, verb64, only32, only32in64}, nil)

	seed()

	if code := msiexecCode(t, filepath.Join(dir, "uninstall.log"), "/x", late); code != 0 {
		t.Fatalf("msiexec /x returned %d", code)
	}

	// Each component deletes its keys with their subkeys, in its own view.
	expect("uninstall", []regKey{tree32, only32in64}, []regKey{tree64, {"64", tree64.key + `\Sub`}, verb64, both, only32})
}

// TestICERemoveOnUninstall runs the full ICE suite against the REG-08
// package.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICERemoveOnUninstall(t *testing.T) {
	cub := iceCube(t)
	requireCleanICE(t, buildRemoveOnUninstall(t, t.TempDir(), "1.0.0", "afterInstallInitialize"), cub)
}
