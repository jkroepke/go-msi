//go:build windows

package nativetest_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// nativeArchConfig is an x64 package that searches the machine's
// PROCESSOR_ARCHITECTURE in the registry and refuses Arm64 Windows unless
// ALLOW_ARM64 is set. ENV_ARCH records the engine's own environment variable
// for comparison.
const nativeArchConfig = `schema: msi-builder/v1
product: { name: MsiBuilderNativeArch, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 5e2b8c41-9d7a-4f06-b3e1-6a0c9d2f4b85 }
package: { platform: x64 }
installDir: { name: MsiBuilderNativeArch }
properties:
  NATIVE_ARCH:
    search: { registry: { root: HKLM, key: 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment', name: PROCESSOR_ARCHITECTURE } }
components:
  - files: [{ source: app.exe }]
customActions:
  - id: SetEnvArch
    setProperty: { name: ENV_ARCH, value: '[%PROCESSOR_ARCHITECTURE]' }
    schedule: [{ sequence: execute, after: AppSearch }]
launchConditions:
  - condition: Installed OR ALLOW_ARM64 = "1" OR NATIVE_ARCH <> "ARM64"
    message: This x64 package does not install on Arm64 Windows.
`

// TestMsiexecNativeArch installs an x64 package that tells Arm64 Windows
// from x64 Windows through a registry search (PROP-03). On Arm64 the x64
// package is otherwise accepted and runs under emulation, so the native
// architecture has to come from the machine, not from Msix64 or the
// package's platform.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecNativeArch(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	dir := t.TempDir()
	path := buildConfig(t, dir, "nativearch", nativeArchConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run() })

	want := "AMD64"
	if runtime.GOARCH == "arm64" {
		want = "ARM64"
	}

	// With ALLOW_ARM64 the package installs everywhere and its log shows
	// what the engine saw.
	allowed := filepath.Join(dir, "allowed.log")
	msiexecQuiet(t, allowed, "/i", path, "ALLOW_ARM64=1")

	props := serverProperties(readLog(allowed))
	for _, name := range []string{"NATIVE_ARCH", "ENV_ARCH", "Msix64", "VersionNT64", "Intel"} {
		t.Logf("PROP-03 (%s runner): %s = %q", runtime.GOARCH, name, props[name])
	}

	if got := props["NATIVE_ARCH"]; got != want {
		t.Errorf("NATIVE_ARCH = %q, want %q", got, want)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

	// Without it the launch condition refuses Arm64 and lets x64 through.
	plain := filepath.Join(dir, "plain.log")
	code := msiexecCode(t, plain, "/i", path)
	log := readLog(plain)

	if runtime.GOARCH != "arm64" {
		if code != 0 {
			t.Fatalf("msiexec /i on x64 returned %d\n%s", code, log)
		}

		msiexecQuiet(t, filepath.Join(dir, "uninstall2.log"), "/x", path)

		return
	}

	if code != 1603 {
		t.Errorf("msiexec /i on Arm64 returned %d, want 1603 from the launch condition", code)
	}

	if !strings.Contains(log, "This x64 package does not install on Arm64 Windows.") {
		t.Errorf("the log does not show the launch condition message\n%s", log)
	}
}
