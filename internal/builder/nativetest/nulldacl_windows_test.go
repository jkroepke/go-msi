//go:build windows

package nativetest_test

import (
	"context"
	"fmt"
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
// files, on a folder that exists before the install (SEC-04).
// FAILNOW=1 fails the install after the grant, so the rollback runs.
const nullDACLConfig = `schema: msi-builder/v1
product: { name: MsiBuilderSec04, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 8d4c2a71-3f6e-4b58-9a1d-c7e05b2f9e63 }
package: { platform: x64, compression: none }
installDir: { name: MsiBuilderSec04 }
properties:
  FAILNOW: { secure: true }
components:
  - files:
      - source: app.exe
  - id: OldFolder
    directory: CommonAppDataFolder/MsiBuilderSec04/existing
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
func setSecurity(path, sddl string) error {
	sddlPtr, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		return fmt.Errorf("sddl %q: %w", sddl, err)
	}

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("path %q: %w", path, err)
	}

	var descriptor uintptr

	const sddlRevision1 = 1
	if r, _, callErr := pConvertStringSD.Call(uintptr(unsafe.Pointer(sddlPtr)), sddlRevision1, uintptr(unsafe.Pointer(&descriptor)), 0); r == 0 {
		return fmt.Errorf("convert SDDL %s: %w", sddl, callErr)
	}

	defer pLocalFree.Call(descriptor) //nolint:errcheck // frees a local allocation

	const daclSecurityInformation = 4
	if r, _, callErr := pSetFileSecurity.Call(uintptr(unsafe.Pointer(pathPtr)), daclSecurityInformation, descriptor); r == 0 {
		return fmt.Errorf("set the DACL of %s to %s: %w", path, sddl, callErr)
	}

	return nil
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
// back and on uninstall (SEC-04). Microsoft documents that such a child gets
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

	root := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderSec04")
	existing := filepath.Join(root, "existing")
	open := filepath.Join(existing, "open")
	// keep.txt inherits from the folder, so it shows that the grant
	// propagated past the children without a DACL.
	keep := filepath.Join(existing, "keep.txt")

	// Children without a DACL, a folder and a file. SetFileSecurityW drops
	// an AI flag (D:AINO_ACCESS_CONTROL reads back as D:NO_ACCESS_CONTROL,
	// SEC-04), so a NULL DACL marked auto-inherited is not covered.
	children := []string{open, filepath.Join(existing, "open.txt")}

	// reset gives the children their NULL DACL back. The test, their
	// owner, may always write the DACL, also over an empty one that a
	// failed earlier run left; a child that does not exist is skipped.
	reset := func() {
		for _, child := range children {
			if _, err := os.Stat(child); err != nil {
				continue
			}

			if err := setSecurity(child, "D:NO_ACCESS_CONTROL"); err != nil {
				t.Errorf("reset: %v", err)
			}
		}
	}

	reset()

	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(open, 0o750); err != nil {
		t.Fatal(err)
	}

	// The test's objects also keep the folder when the package is removed.
	for _, file := range []string{children[1], keep} {
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	reset()

	dir := t.TempDir()
	path := buildConfig(t, dir, "sec04", nullDACLConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()

		reset()

		_ = os.RemoveAll(root)
	})

	const guests = "(A;OICI;0x1200a9;;;BG)"

	const guestsInherited = "(A;ID;0x1200a9;;;BG)"

	t.Logf("SEC-04: parent before: %s", fileDACL(t, existing))
	t.Logf("SEC-04: keep.txt before: %s", fileDACL(t, keep))

	requireNullDACLs := func(when string) {
		t.Helper()

		for _, child := range children {
			got := fileDACL(t, child)
			t.Logf("SEC-04: %s %s: %s", filepath.Base(child), when, got)

			if got != "D:NO_ACCESS_CONTROL" {
				t.Errorf("%s %s: %s, want D:NO_ACCESS_CONTROL as before", filepath.Base(child), when, got)
			}
		}
	}

	requireNullDACLs("before")

	// Rollback: the grant ran, then FailNow failed the install.
	if code := msiexecCode(t, filepath.Join(dir, "rollback.log"), "/i", path, "FAILNOW=1"); code != 1603 {
		t.Fatalf("install with FAILNOW=1 returned %d, want 1603", code)
	}

	log := readLog(filepath.Join(dir, "rollback.log"))
	t.Logf("SEC-04: rollback log:\n%s", grepLog(log, "BuilderGrant: "))

	for _, want := range []string{"BuilderGrant: granted: " + existing, "BuilderGrant: revoked: " + existing} {
		if !strings.Contains(log, want) {
			t.Errorf("rollback log has no %q", want)
		}
	}

	if got := fileDACL(t, existing); strings.Contains(got, guests) {
		t.Errorf("parent after rollback: %s, want no %s", got, guests)
	}

	if got := fileDACL(t, keep); strings.Contains(got, guestsInherited) {
		t.Errorf("keep.txt after rollback: %s, want no %s", got, guestsInherited)
	}

	requireNullDACLs("after rollback")

	// Everyone may still write into the folder.
	if err := os.WriteFile(filepath.Join(open, "probe.txt"), []byte("x"), 0o600); err != nil {
		t.Errorf("writing into the folder without a DACL after the rollback: %v", err)
	}

	// Install: the grant reaches the folder and what inherits from it; the
	// children gave everyone full access already, so they need nothing of
	// it.
	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	if got := fileDACL(t, existing); !strings.Contains(got, guests) {
		t.Errorf("parent after install: %s, want %s", got, guests)
	}

	if got := fileDACL(t, keep); !strings.Contains(got, guestsInherited) {
		t.Errorf("keep.txt after install: %s, want %s", got, guestsInherited)
	} else {
		t.Logf("SEC-04: keep.txt after install: %s", got)
	}

	requireNullDACLs("after install")

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	requireNullDACLs("after uninstall")
}
