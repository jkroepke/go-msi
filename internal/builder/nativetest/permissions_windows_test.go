//go:build windows

package nativetest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// sec02Root is the ProgramData folder of SEC-02.
const sec02Root = `MsiBuilderSec02`

// sec02SDDLConfig secures a file, a new folder, a folder that exists before
// the install and a registry key with SDDL strings that have no P (protected)
// flag.
const sec02SDDLConfig = `schema: msi-builder/v1
product: { name: MsiBuilderSec02, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 3a8f1c52-9e0d-4b7a-a6c3-2d5e8f1b7c94 }
package: { platform: x64 }
installDir: { name: MsiBuilderSec02 }
components:
  - files:
      - source: app.exe
        permissions: { sddl: 'D:(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;NS)' }
  - id: NewFolder
    directory: CommonAppDataFolder/MsiBuilderSec02/new
    createFolder: true
    folderPermissions: { sddl: 'D:(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;NS)' }
  - id: OldFolder
    directory: CommonAppDataFolder/MsiBuilderSec02/existing
    createFolder: true
    folderPermissions: { sddl: 'D:(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;NS)' }
  - id: Key
    registry:
      - root: HKLM
        key: 'SOFTWARE\MsiBuilderTest\Sec02'
        name: Value
        value: x
        permissions: { sddl: 'D:(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;NS)' }
`

// objectSDDL returns the SDDL of a file, folder or registry key.
func objectSDDL(t *testing.T, path string) string {
	t.Helper()

	out, err := powershell("(Get-Acl -Path '" + path + "').Sddl")
	if err != nil {
		return "error: " + out
	}

	return out
}

// TestMsiexecSDDLWithoutProtection records what MsiLockPermissionsEx does with
// an SDDL string without the P flag (SEC-02): whether the object keeps the
// ACEs it inherits from its parent, and whether a folder that existed before
// the install keeps its own explicit ACEs.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecSDDLWithoutProtection(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	root := filepath.Join(os.Getenv("ProgramData"), sec02Root)
	existing := filepath.Join(root, "existing")

	if err := os.MkdirAll(existing, 0o750); err != nil {
		t.Fatal(err)
	}

	// An explicit ACE for Guests that the package does not author.
	if out, err := exec.CommandContext(t.Context(), "icacls", existing, "/grant", "*S-1-5-32-546:(OI)(CI)R").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v\n%s", err, out)
	}

	path := buildConfig(t, t.TempDir(), "sec02", sec02SDDLConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(root)
		_ = exec.CommandContext(context.Background(), "reg", "delete", `HKLM\SOFTWARE\MsiBuilderTest\Sec02`, "/f").Run()
	})

	t.Logf("SEC-02: existing folder before install: %s", objectSDDL(t, existing))

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "install.log"), "/i", path)

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderSec02")
	objects := []struct{ name, path string }{
		{"install folder (parent of the file)", installDir},
		{"file", filepath.Join(installDir, "app.exe")},
		{"ProgramData folder (parent of the folders)", root},
		{"new folder", filepath.Join(root, "new")},
		{"existing folder", existing},
		{"registry key", `HKLM:\SOFTWARE\MsiBuilderTest\Sec02`},
	}

	for _, o := range objects {
		t.Logf("SEC-02: after install, %s: %s", o.name, objectSDDL(t, o.path))
	}

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "repair.log"), "/fa", path)

	for _, o := range objects[1:] {
		t.Logf("SEC-02: after repair /fa, %s: %s", o.name, objectSDDL(t, o.path))
	}

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path)

	t.Logf("SEC-02: after uninstall, existing folder: %s", objectSDDL(t, existing))
}
