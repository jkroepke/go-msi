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
// to a folder and a file without a DACL (a NULL DACL, which gives everyone
// full access) in that folder, when the install succeeds, when it is rolled
// back and on uninstall (SEC-03). Microsoft documents that such a child gets
// a DACL with only the inherited ACE when the parent gains an inheritable
// ACE, and an empty DACL, which gives no one access, when the ACE is removed
// again. On Windows Server 2025 both keep their NULL DACL throughout, so the
// grant needs no protection for them.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecGrantNullDACLChild(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	root := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderSec03")
	existing := filepath.Join(root, "existing")

	// Children without a DACL, a folder and a file. SetFileSecurityW drops
	// an AI flag (D:AINO_ACCESS_CONTROL reads back as D:NO_ACCESS_CONTROL,
	// SEC-03), so a NULL DACL marked auto-inherited is not covered.
	children := []struct{ name, sddl string }{
		{"open", "D:NO_ACCESS_CONTROL"},
		{"open.txt", "D:NO_ACCESS_CONTROL"},
	}

	if err := os.MkdirAll(filepath.Join(existing, "open"), 0o750); err != nil {
		t.Fatal(err)
	}

	// The test's own objects also keep the folder when the package is
	// removed.
	if err := os.WriteFile(filepath.Join(existing, "open.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	reset := func() {
		for _, child := range children {
			setSecurity(t, filepath.Join(existing, child.name), child.sddl)
		}
	}

	reset()

	dir := t.TempDir()
	path := buildConfig(t, dir, "sec03", nullDACLConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		// The test's objects' owner, the elevated test, may always write
		// the DACL, also over an empty one.
		reset()

		_ = os.RemoveAll(root)
	})

	t.Logf("SEC-03: parent before: %s", fileDACL(t, existing))

	before := map[string]string{}
	for _, child := range children {
		before[child.name] = fileDACL(t, filepath.Join(existing, child.name))
		t.Logf("SEC-03: %s before: %s", child.name, before[child.name])

		if !strings.Contains(before[child.name], "NO_ACCESS_CONTROL") {
			t.Fatalf("%s before the install: %s, want no DACL", child.name, before[child.name])
		}
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

	requireNullDACLs := func(when string) {
		t.Helper()

		for _, child := range children {
			got := fileDACL(t, filepath.Join(existing, child.name))
			t.Logf("SEC-03: %s %s: %s", child.name, when, got)

			if got != before[child.name] {
				t.Errorf("%s %s: %s, want %s as before", child.name, when, got, before[child.name])
			}
		}
	}

	requireNullDACLs("after rollback")

	// Install: the grant reaches the folder; the children gave everyone
	// full access already, so they need nothing of it.
	reset()
	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	t.Logf("SEC-03: parent after install: %s", fileDACL(t, existing))
	requireNullDACLs("after install")

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	requireNullDACLs("after uninstall")
}
