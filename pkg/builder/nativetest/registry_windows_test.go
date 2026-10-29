//go:build windows

package nativetest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// registryTestKey is the key below HKLM and HKCU the registry tests use.
const registryTestKey = `SOFTWARE\MsiBuilderTest\RemoveRegistry`

const removeRegistryConfig = `schema: msi-builder/v1
product: { name: MsiBuilderRemoveRegistry, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 6c0d9f4e-2a71-4b8e-9d3c-5f1a7e2b8c41 }
package: { platform: x64 }
directories:
  - id: ProgramFiles64Folder
    children: [{ id: INSTALLDIR, name: MsiBuilderRemoveRegistry }]
  - id: ProgramFilesFolder
    children: [{ id: DIR32, name: MsiBuilderRemoveRegistry }]
components:
  - id: App
    files: [{ source: app.exe }]
    removeRegistry:
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Old' }
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Only64' }
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Keep', name: Legacy }
      - { root: HKMU, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Mu' }
      - { root: HKCU, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\User' }
  - id: Reset
    registry:
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Reset', name: Value, value: new }
    removeRegistry:
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Reset' }
  - id: Old32
    directory: DIR32
    bitness: "32"
    removeRegistry:
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Old' }
      - { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\RemoveRegistry\Only32' }
`

// buildRemoveRegistry writes the RemoveRegistry test package.
func buildRemoveRegistry(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	if err := os.WriteFile(filepath.Join(dir, "app.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(removeRegistryConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "removeregistry.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// reg runs reg.exe with the arguments and returns its output; failures are
// part of the output, because a query of a missing key fails.
func reg(t *testing.T, args ...string) string {
	t.Helper()

	out, _ := exec.CommandContext(t.Context(), "reg.exe", args...).CombinedOutput()

	return string(out)
}

// regSet writes a REG_SZ value in a registry view (64 or 32).
func regSet(t *testing.T, view, key, name, value string) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "reg.exe", "add", key, "/v", name, "/d", value, "/f", "/reg:"+view).CombinedOutput()
	if err != nil {
		t.Fatalf("reg add %s: %v\n%s", key, err, out)
	}
}

// regHas reports whether a key, or a value of it when name is set, exists
// in a registry view.
func regHas(t *testing.T, view, key, name string) bool {
	t.Helper()

	args := []string{"query", key, "/reg:" + view}
	if name != "" {
		args = append(args, "/v", name)
	}

	return exec.CommandContext(t.Context(), "reg.exe", args...).Run() == nil
}

// regData returns the data of a REG_SZ value in the 64-bit view, or
// "<absent>".
func regData(t *testing.T, key, name string) string {
	t.Helper()

	for line := range strings.Lines(reg(t, "query", key, "/v", name, "/reg:64")) {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[2:], " ")
		}
	}

	return "<absent>"
}

// TestMsiexecRemovesRegistry installs a package with removeRegistry
// entries over keys and values that exist beforehand (REG-02): whole keys
// with subkeys in both registry views, one value, an HKMU and an HKCU key,
// and a key that the same package writes again.
func TestMsiexecRemovesRegistry(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path := buildRemoveRegistry(t)
	log := filepath.Join(filepath.Dir(path), "install.log")
	hklm, hkcu := `HKLM\`+registryTestKey, `HKCU\`+registryTestKey

	cleanup := func() {
		ctx := context.Background()
		_ = exec.CommandContext(ctx, "msiexec", "/x", path, "/qn").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\MsiBuilderTest`, "/f", "/reg:64").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\MsiBuilderTest`, "/f", "/reg:32").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKCU\SOFTWARE\MsiBuilderTest`, "/f").Run()
	}

	cleanup()
	t.Cleanup(cleanup)

	seed := func() {
		for _, view := range []string{"64", "32"} {
			regSet(t, view, hklm+`\Old`, "Value", "old")
			regSet(t, view, hklm+`\Old\Sub`, "Value", "old")
			regSet(t, view, hklm+`\Only64`, "Value", "old")
			regSet(t, view, hklm+`\Only32`, "Value", "old")
		}

		regSet(t, "64", hklm+`\Keep`, "Legacy", "old")
		regSet(t, "64", hklm+`\Keep`, "Other", "kept")
		regSet(t, "64", hklm+`\Mu`, "Value", "old")
		regSet(t, "64", hkcu+`\User`, "Value", "old")
		regSet(t, "64", hklm+`\Reset`, "Stale", "old")
		regSet(t, "64", hklm+`\Reset`, "Value", "old")
	}

	seed()

	if code := msiexecCode(t, log, "/i", path); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
	}

	for _, gone := range []struct{ view, key, name string }{
		{"64", hklm + `\Old`, ""},
		{"32", hklm + `\Old`, ""},
		{"64", hklm + `\Keep`, "Legacy"},
		{"64", hklm + `\Mu`, ""},
		{"64", hkcu + `\User`, ""},
		{"64", hklm + `\Reset`, "Stale"},
		{"64", hklm + `\Only64`, ""},
		{"32", hklm + `\Only32`, ""},
	} {
		if regHas(t, gone.view, gone.key, gone.name) {
			t.Errorf("%s %s (view %s) is still there after the install", gone.key, gone.name, gone.view)
		}
	}

	// Each component removes only in its own view.
	if !regHas(t, "32", hklm+`\Only64`, "") {
		t.Errorf(`the 64-bit component also removed Only64 from the 32-bit view`)
	}

	if !regHas(t, "64", hklm+`\Only32`, "") {
		t.Errorf(`the 32-bit component also removed Only32 from the 64-bit view`)
	}

	if got := regData(t, hklm+`\Keep`, "Other"); got != "kept" {
		t.Errorf(`Keep\Other is %q after the install, want kept`, got)
	}

	if got := regData(t, hklm+`\Reset`, "Value"); got != "new" {
		t.Errorf(`Reset\Value is %q after the install, want new`, got)
	}

	// Repair and uninstall: what happens to keys that exist again.
	seed()

	repairLog := filepath.Join(filepath.Dir(path), "repair.log")
	code := msiexecCode(t, repairLog, "/fomus", path)
	t.Logf("REG-02 repair /fomus: exit %d, Old(64) present=%v, Old(32) present=%v, Keep\\Legacy present=%v, User present=%v, Reset\\Stale present=%v, Reset\\Value=%q",
		code, regHas(t, "64", hklm+`\Old`, ""), regHas(t, "32", hklm+`\Old`, ""), regHas(t, "64", hklm+`\Keep`, "Legacy"),
		regHas(t, "64", hkcu+`\User`, ""), regHas(t, "64", hklm+`\Reset`, "Stale"), regData(t, hklm+`\Reset`, "Value"))

	seed()

	uninstallLog := filepath.Join(filepath.Dir(path), "uninstall.log")
	code = msiexecCode(t, uninstallLog, "/x", path)
	t.Logf("REG-02 uninstall: exit %d, Old(64) present=%v, Keep\\Legacy present=%v, Keep\\Other=%q, Reset\\Stale present=%v, Reset\\Value=%q",
		code, regHas(t, "64", hklm+`\Old`, ""), regHas(t, "64", hklm+`\Keep`, "Legacy"), regData(t, hklm+`\Keep`, "Other"),
		regHas(t, "64", hklm+`\Reset`, "Stale"), regData(t, hklm+`\Reset`, "Value"))
}

// TestICERemoveRegistry runs the full ICE suite against the RemoveRegistry
// test package.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICERemoveRegistry(t *testing.T) {
	cub := iceCube(t)
	requireCleanICE(t, buildRemoveRegistry(t), cub)
}
