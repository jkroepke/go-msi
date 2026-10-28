//go:build windows

package config_test

import (
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/pkg/config"
)

var (
	advapi32                       = syscall.NewLazyDLL("advapi32.dll")                                       //nolint:gochecknoglobals // one DLL table for the file
	pConvertStringSecurityDescToSD = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW") //nolint:gochecknoglobals // one DLL table for the file
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")                                       //nolint:gochecknoglobals // one DLL table for the file
	pLocalFree                     = kernel32.NewProc("LocalFree")                                            //nolint:gochecknoglobals // one DLL table for the file
)

const sddlRevision1 = 1

// nativeSDDL reports whether Windows parses the SDDL string.
func nativeSDDL(t *testing.T, sddl string) bool {
	t.Helper()

	ptr, err := syscall.UTF16PtrFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}

	var desc uintptr

	ret, _, _ := pConvertStringSecurityDescToSD.Call(uintptr(unsafe.Pointer(ptr)), sddlRevision1, uintptr(unsafe.Pointer(&desc)), 0)
	if ret == 0 {
		return false
	}

	_, _, _ = pLocalFree.Call(desc)

	return true
}

// FuzzCheckSDDLNative compares CheckSDDL with the Windows SDDL parser: a
// string CheckSDDL accepts must parse on Windows too, or the engine fails
// with 1943 at install time. <account> forms are resolved by the engine and
// left out.
func FuzzCheckSDDLNative(f *testing.F) {
	for _, seed := range []string{
		"D:PAI(A;;FA;;;BA)(A;;FRFX;;;BU)", "O:BAG:BAD:(A;OICI;FA;;;SY)(A;OICI;0x1200a9;;;BU)",
		"D:(A;;FA;;;S-1-5-21-1-2-3-500)", "D:(A;;KA;;;BA)S:(AU;SAFA;FA;;;WD)", "D:(A;;FA;;;BA)(D;;FW;;;BU)",
		"D:(A;;FA;;;S-1-5)", "D:(A;;0xFFFFFFFFF;;;BA)", "D:(A;;FA;;;S-1-99999999999-1)", "D:(A;;FA;;;ba)",
		"D:(A;;FA;;;BA)D:(A;;FA;;;BA)", "D:", "S:", "O:BA", "D:(OA;;FA;;;BA)", "D:(A;;FA;x;;BA)",
		"D:(A;;FA;;;BA;x)", "S:(RA;;FA;;;BA;x)", "D:(A;;FA;;;S-1-5-21-1-2-3-4-5-6-7-8-9-10-11-12-13-14)",
		"D:(A;;FA;;;S-1-5-4294967295)", "D:(A;;FA;;;S-1-281474976710655-1)", "D:(XA;;FX;;;WD)", "S:(RA;;FA;;;BA)",
		"D:(A;;FA;ab721a53-1e2f-11d0-9819-00aa0040529b;;BA)", "D:(A;;FA;;;S-1-5-000000000000000000000000000000018)",
		"S:(FL;;FA;;;BA)", "D:(A;;0x100000000;;;BA)", "D:(A;;99999999999;;;BA)",
		"D:(OA;;CR;AB721A53-1E2F-11D0-9819-00AA0040529B;;BA)", "S:(TL;;0x0;;;S-1-19-512-1024)", "S:(ML;;NW;;;LW)", "D:(A;CI CI;FA;;;BA)", "D:P(A;;FA;;;BA)", "D:NO_ACCESS_CONTROL", "D:(A;;FAFA;;;BA)",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, sddl string) {
		if strings.ContainsAny(sddl, "<>") {
			return
		}

		if _, err := config.CheckSDDL(sddl); err != nil {
			return
		}

		if strings.ContainsRune(sddl, 0) {
			t.Fatalf("CheckSDDL accepts %q, which holds a NUL", sddl)
		}

		if !nativeSDDL(t, sddl) {
			t.Fatalf("CheckSDDL accepts %q, but Windows does not parse it", sddl)
		}
	})
}
