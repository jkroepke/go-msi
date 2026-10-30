//go:build windows

package nativetest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

//nolint:gochecknoglobals // lazy procedures
var (
	pConvertStringSD = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	pSetFileSecurity = advapi32.NewProc("SetFileSecurityW")
	pLocalFree       = kernel32.NewProc("LocalFree")
)

// nullDACLConfig grants Guests read access, inherited by subfolders and
// files, on a folder that exists before the install (SEC-03).
// FAILNOW=1 fails the install after the grant, so the rollback runs.
const nullDACLConfig = `schema: msi-builder/v1
product: { name: MsiBuilderSec03, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 8d4c2a71-3f6e-4b58-9a1d-c7e05b2f9e63 }
package: { platform: x64, compression: none }
installDir: { name: MsiBuilderSec03 }
properties:
  FAILNOW: { secure: true }
components:
  - files:
      - source: app.exe
  - id: OldFolder
    directory: CommonAppDataFolder/MsiBuilderSec03/existing
    createFolder: true
    folderPermissions:
      grant: [{ account: S-1-5-32-546, rights: readExecute }]
customActions:
  - id: FailNow
    run: { command: '"[System64Folder]cmd.exe" /c exit 1' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, before: InstallFinalize, condition: FAILNOW=1 }]
`

// setSecurity writes the DACL and its control flags of an SDDL string to a
// file or folder with SetFileSecurityW, which stores the descriptor as it is,
// without the inheritance that SetNamedSecurityInfoW applies, so that a
// folder can have an unprotected NULL DACL ("D:NO_ACCESS_CONTROL").
func setSecurity(t *testing.T, path, sddl string) {
	t.Helper()

	sddlPtr, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}

	var descriptor uintptr

	const sddlRevision1 = 1
	if r, _, callErr := pConvertStringSD.Call(uintptr(unsafe.Pointer(sddlPtr)), sddlRevision1, uintptr(unsafe.Pointer(&descriptor)), 0); r == 0 {
		t.Fatalf("ConvertStringSecurityDescriptorToSecurityDescriptorW(%s): %v", sddl, callErr)
	}

	defer pLocalFree.Call(descriptor) //nolint:errcheck // frees a local allocation

	const daclSecurityInformation = 4
	if r, _, callErr := pSetFileSecurity.Call(uintptr(unsafe.Pointer(pathPtr)), daclSecurityInformation, descriptor); r == 0 {
		t.Fatalf("SetFileSecurityW(%s, %s): %v", path, sddl, callErr)
	}
}

// daclPart returns the D: part of an SDDL string.
func daclPart(sddl string) string {
	if i := strings.Index(sddl, "D:"); i >= 0 {
		return sddl[i:]
	}

	return sddl
}

// TestMsiexecGrantNullDACLChild checks what an inheriting folder grant does
// to a subfolder without a DACL (a NULL DACL, which gives everyone full
// access) when the install succeeds and when it is rolled back (SEC-03).
// Microsoft documents that such a child gets a DACL with only the inherited
// ACE when the parent gains an inheritable ACE, and an empty DACL, which
// gives no one access, when the ACE is removed again.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecGrantNullDACLChild(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	root := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderSec03")
	existing := filepath.Join(root, "existing")
	open := filepath.Join(existing, "open")

	if err := os.MkdirAll(open, 0o750); err != nil {
		t.Fatal(err)
	}

	// A file of its own keeps the folder when the package is removed.
	if err := os.WriteFile(filepath.Join(existing, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	setSecurity(t, open, "D:NO_ACCESS_CONTROL")

	dir := t.TempDir()
	path := buildConfig(t, dir, "sec03", nullDACLConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		// The test folder's owner, the elevated test, may always write
		// the DACL, also an empty one.
		setSecurity(t, open, "D:NO_ACCESS_CONTROL")

		_ = os.RemoveAll(root)
	})

	before := daclPart(objectSDDL(t, open))
	t.Logf("SEC-03: NULL-DACL child before: %s, parent: %s", before, objectSDDL(t, existing))

	if before != "D:NO_ACCESS_CONTROL" {
		t.Fatalf("child before the install: %s, want D:NO_ACCESS_CONTROL", before)
	}

	// Rollback: the grant ran, then FailNow failed the install.
	if code := msiexecCode(t, filepath.Join(dir, "rollback.log"), "/i", path, "FAILNOW=1"); code != 1603 {
		t.Fatalf("install with FAILNOW=1 returned %d, want 1603", code)
	}

	log := readLog(filepath.Join(dir, "rollback.log"))
	t.Logf("SEC-03: rollback log:\n%s", grepLog(log, "BuilderGrant: "))

	for _, want := range []string{"BuilderGrant: granted", "BuilderGrant: revoked"} {
		if !strings.Contains(log, want) {
			t.Errorf("rollback log has no %q", want)
		}
	}

	afterRollback := objectSDDL(t, open)
	t.Logf("SEC-03: NULL-DACL child after rollback: %s", afterRollback)

	writeErr := os.WriteFile(filepath.Join(open, "probe.txt"), []byte("x"), 0o600)
	t.Logf("SEC-03: writing a file into the child after rollback: %v", writeErr)

	if got := daclPart(afterRollback); !strings.Contains(got, "NO_ACCESS_CONTROL") {
		t.Errorf("child after rollback: %s, want no DACL (NO_ACCESS_CONTROL) as before", afterRollback)
	}

	// Install: the grant reaches the folder; the child had full access
	// for everyone, so it needs nothing of it.
	setSecurity(t, open, "D:NO_ACCESS_CONTROL")
	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	t.Logf("SEC-03: parent after install: %s", objectSDDL(t, existing))

	afterInstall := objectSDDL(t, open)
	t.Logf("SEC-03: NULL-DACL child after install: %s", afterInstall)

	if got := daclPart(afterInstall); !strings.Contains(got, "NO_ACCESS_CONTROL") {
		t.Errorf("child after install: %s, want no DACL (NO_ACCESS_CONTROL) as before", afterInstall)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	t.Logf("SEC-03: NULL-DACL child after uninstall: %s", objectSDDL(t, open))
}
