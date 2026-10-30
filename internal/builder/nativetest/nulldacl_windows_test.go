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
	pGetFileSecurity = advapi32.NewProc("GetFileSecurityW")
	pConvertSDString = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
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

// fileDACL returns the DACL of a file or folder as SDDL, read with
// GetFileSecurityW: "D:NO_ACCESS_CONTROL" for no DACL, "D:" (with control
// flags such as AI) for an empty one. Get-Acl leaves a NULL DACL out of its
// Sddl, so it cannot tell the two apart.
func fileDACL(t *testing.T, path string) string {
	t.Helper()

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}

	const daclSecurityInformation = 4

	var size uint32

	_, _, _ = pGetFileSecurity.Call(uintptr(unsafe.Pointer(pathPtr)), daclSecurityInformation, 0, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return "error: GetFileSecurityW returned no size"
	}

	buf := make([]byte, size)
	if r, _, callErr := pGetFileSecurity.Call(uintptr(unsafe.Pointer(pathPtr)), daclSecurityInformation, uintptr(unsafe.Pointer(&buf[0])), uintptr(size), uintptr(unsafe.Pointer(&size))); r == 0 {
		return "error: GetFileSecurityW: " + callErr.Error()
	}

	var (
		text   *uint16
		length uint32
	)

	const sddlRevision1 = 1
	if r, _, callErr := pConvertSDString.Call(uintptr(unsafe.Pointer(&buf[0])), sddlRevision1, daclSecurityInformation, uintptr(unsafe.Pointer(&text)), uintptr(unsafe.Pointer(&length))); r == 0 {
		return "error: ConvertSecurityDescriptorToStringSecurityDescriptorW: " + callErr.Error()
	}

	defer pLocalFree.Call(uintptr(unsafe.Pointer(text))) //nolint:errcheck // frees a local allocation

	return syscall.UTF16ToString(unsafe.Slice(text, length))
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

	before := fileDACL(t, open)
	t.Logf("SEC-03: NULL-DACL child before: %s, parent: %s", before, fileDACL(t, existing))

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

	afterRollback := fileDACL(t, open)
	t.Logf("SEC-03: NULL-DACL child after rollback: %s", afterRollback)

	writeErr := os.WriteFile(filepath.Join(open, "probe.txt"), []byte("x"), 0o600)
	t.Logf("SEC-03: writing a file into the child after rollback: %v", writeErr)

	if !strings.Contains(afterRollback, "NO_ACCESS_CONTROL") {
		t.Errorf("child after rollback: %s, want no DACL (NO_ACCESS_CONTROL) as before", afterRollback)
	}

	// Install: the grant reaches the folder; the child had full access
	// for everyone, so it needs nothing of it.
	setSecurity(t, open, "D:NO_ACCESS_CONTROL")
	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	t.Logf("SEC-03: parent after install: %s", fileDACL(t, existing))

	afterInstall := fileDACL(t, open)
	t.Logf("SEC-03: NULL-DACL child after install: %s", afterInstall)

	if !strings.Contains(afterInstall, "NO_ACCESS_CONTROL") {
		t.Errorf("child after install: %s, want no DACL (NO_ACCESS_CONTROL) as before", afterInstall)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	t.Logf("SEC-03: NULL-DACL child after uninstall: %s", fileDACL(t, open))
}
