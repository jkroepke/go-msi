//go:build windows

package nativetest_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

var (
	ole32                     = syscall.NewLazyDLL("ole32.dll")                                                     //nolint:gochecknoglobals // lazy DLL handle
	propsys                   = syscall.NewLazyDLL("propsys.dll")                                                   //nolint:gochecknoglobals // lazy DLL handle
	pCoInitializeEx           = ole32.NewProc("CoInitializeEx")                                                     //nolint:gochecknoglobals // lazy procedure
	pCoUninitialize           = ole32.NewProc("CoUninitialize")                                                     //nolint:gochecknoglobals // lazy procedure
	pCoCreateInstance         = ole32.NewProc("CoCreateInstance")                                                   //nolint:gochecknoglobals // lazy procedure
	pCoTaskMemFree            = ole32.NewProc("CoTaskMemFree")                                                      //nolint:gochecknoglobals // lazy procedure
	pPropVariantClear         = ole32.NewProc("PropVariantClear")                                                   //nolint:gochecknoglobals // lazy procedure
	pPSGetPropertyKeyFromName = propsys.NewProc("PSGetPropertyKeyFromName")                                         //nolint:gochecknoglobals // lazy procedure
	pPropVariantToStringAlloc = propsys.NewProc("PropVariantToStringAlloc")                                         //nolint:gochecknoglobals // lazy procedure
	clsidShellLink            = comGUID(0x00021401, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)                             //nolint:gochecknoglobals // COM class
	iidIShellLinkW            = comGUID(0x000214F9, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)                             //nolint:gochecknoglobals // COM interface
	iidIPersistFile           = comGUID(0x0000010B, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)                             //nolint:gochecknoglobals // COM interface
	iidIPropertyStore         = comGUID(0x886D8EEB, 0x8CF2, 0x4446, 0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99) //nolint:gochecknoglobals // COM interface
)

// Vtable slots and constants of the COM calls below.
const (
	slotQueryInterface = 0
	slotRelease        = 2
	slotPersistLoad    = 5 // IPersistFile::Load
	slotStoreGetValue  = 5 // IPropertyStore::GetValue

	coinitApartmentThreaded = 2
	clsctxInprocServer      = 1
	stgmRead                = 0
)

type comGUIDValue struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

func comGUID(d1 uint32, d2, d3 uint16, b0, b1, b2, b3, b4, b5, b6, b7 byte) comGUIDValue {
	return comGUIDValue{d1, d2, d3, [8]byte{b0, b1, b2, b3, b4, b5, b6, b7}}
}

type propertyKey struct {
	fmtid comGUIDValue
	pid   uint32
}

// propVariant is PROPVARIANT: the type, three reserved words and a union of
// 8 bytes (32-bit) or 16 bytes (64-bit).
type propVariant struct {
	vt       uint16
	reserved [3]uint16
	value    [2]uintptr
}

// comCall calls the method in slot of a COM object.
func comCall(obj unsafe.Pointer, slot int, args ...uintptr) uintptr {
	vtbl := *(**[8]uintptr)(obj)
	r, _, _ := syscall.SyscallN(vtbl[slot], append([]uintptr{uintptr(obj)}, args...)...)

	return r
}

// lnkProperty is one value read from a .lnk property store: its VARTYPE and
// its text from PropVariantToStringAlloc.
type lnkProperty struct {
	vt   uint16
	text string
}

func (p lnkProperty) String() string { return fmt.Sprintf("vt %d %q", p.vt, p.text) }

// lnkProperties opens a .lnk with the shell's own ShellLink object and reads
// the named properties from its property store. A name that the property
// system does not know is left out.
func lnkProperties(t *testing.T, path string, names ...string) map[string]lnkProperty {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if hr, _, _ := pCoInitializeEx.Call(0, coinitApartmentThreaded); int32(hr) < 0 {
		t.Fatalf("CoInitializeEx: 0x%08X", uint32(hr))
	}
	defer pCoUninitialize.Call()

	var link, file, store unsafe.Pointer

	hr, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if int32(hr) < 0 {
		t.Fatalf("CoCreateInstance(ShellLink): 0x%08X", uint32(hr))
	}
	defer comCall(link, slotRelease)

	if hr := comCall(link, slotQueryInterface, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&file))); int32(hr) < 0 {
		t.Fatalf("IPersistFile: 0x%08X", uint32(hr))
	}
	defer comCall(file, slotRelease)

	if hr := comCall(file, slotPersistLoad, uintptr(unsafe.Pointer(wstr(t, path))), stgmRead); int32(hr) < 0 {
		t.Fatalf("loading %s: 0x%08X", path, uint32(hr))
	}

	if hr := comCall(link, slotQueryInterface, uintptr(unsafe.Pointer(&iidIPropertyStore)), uintptr(unsafe.Pointer(&store))); int32(hr) < 0 {
		t.Fatalf("IPropertyStore: 0x%08X", uint32(hr))
	}
	defer comCall(store, slotRelease)

	got := make(map[string]lnkProperty)

	for _, name := range names {
		var key propertyKey
		if hr, _, _ := pPSGetPropertyKeyFromName.Call(uintptr(unsafe.Pointer(wstr(t, name))), uintptr(unsafe.Pointer(&key))); int32(hr) < 0 {
			continue
		}

		var value propVariant
		if hr := comCall(store, slotStoreGetValue, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))); int32(hr) < 0 {
			t.Fatalf("%s: GetValue(%s): 0x%08X", path, name, uint32(hr))
		}

		var text *uint16

		prop := lnkProperty{vt: value.vt}
		if hr, _, _ := pPropVariantToStringAlloc.Call(uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&text))); int32(hr) >= 0 && text != nil {
			prop.text = utf16PtrString(text)
			pCoTaskMemFree.Call(uintptr(unsafe.Pointer(text)))
		}

		pPropVariantClear.Call(uintptr(unsafe.Pointer(&value)))

		got[name] = prop
	}

	return got
}

func utf16PtrString(p *uint16) string {
	var units []uint16

	for ptr := unsafe.Pointer(p); *(*uint16)(ptr) != 0; ptr = unsafe.Add(ptr, 2) {
		units = append(units, *(*uint16)(ptr))
	}

	return syscall.UTF16ToString(units)
}

// VARTYPEs the engine may store.
const (
	vtEmpty  = 0
	vtBool   = 11
	vtUI4    = 19
	vtLPWSTR = 31
	vtCLSID  = 72
)

// TestMsiexecShortcutProperties installs shortcuts with MsiShortcutProperty
// rows and reads the property store of the installed .lnk files back
// (SHORTCUT-02): which VARTYPE the engine stores for a string, a CLSID, a
// boolean and an integer property, whether the values are formatted, and
// what an unknown property name does.
func TestMsiexecShortcutProperties(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	const toastCLSID = "{6E3F1A3C-61B6-4C1B-9A0F-2C5D8B7E4A10}"

	dir := t.TempDir()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(dir, "toast.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: MsiBuilder Toast Test, manufacturer: MsiBuilderTest, version: 1.0.0, upgradeCode: 9d6c1f3e-2b7a-4e58-a1c4-5f0e8d2b7a62 }
package: { platform: x64 }
components:
  - files:
      - source: toast.exe
        shortcuts:
          - name: MsiBuilder Toast Test
            properties:
              System.AppUserModel.ID: "[Manufacturer].Toast"
              System.AppUserModel.ToastActivatorCLSID: "`+toastCLSID+`"
              System.AppUserModel.ExcludeFromShowInNewInstall: "1"
              System.AppUserModel.StartPinOption: "1"
          - name: MsiBuilder Toast Test 2
            properties:
              System.AppUserModel.ID: Plain.Value
              System.AppUserModel.ToastActivatorCLSID: "`+strings.Trim(toastCLSID, "{}")+`"
              System.AppUserModel.PreventPinning: "true"
              System.MsiBuilderTest.Unknown: x
`), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "props.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	log := filepath.Join(dir, "install.log")
	msiexecQuiet(t, log, "/i", path)

	t.Cleanup(func() { msiexecQuietCleanup(path) })

	menu := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`)
	names := []string{
		"System.AppUserModel.ID", "System.AppUserModel.ToastActivatorCLSID", "System.AppUserModel.ExcludeFromShowInNewInstall",
		"System.AppUserModel.StartPinOption", "System.AppUserModel.PreventPinning",
	}

	first := lnkProperties(t, filepath.Join(menu, "MsiBuilder Toast Test.lnk"), names...)
	second := lnkProperties(t, filepath.Join(menu, "MsiBuilder Toast Test 2.lnk"), names...)

	t.Logf("first shortcut: %v", first)
	t.Logf("second shortcut: %v", second)

	for _, check := range []struct {
		what string
		got  lnkProperty
		vt   uint16
		text string
	}{
		{"formatted AppUserModel.ID", first["System.AppUserModel.ID"], vtLPWSTR, "MsiBuilderTest.Toast"},
		{"ToastActivatorCLSID in braces", first["System.AppUserModel.ToastActivatorCLSID"], vtCLSID, toastCLSID},
		{"ExcludeFromShowInNewInstall 1", first["System.AppUserModel.ExcludeFromShowInNewInstall"], vtBool, ""},
		{"StartPinOption 1", first["System.AppUserModel.StartPinOption"], vtUI4, "1"},
		{"plain AppUserModel.ID", second["System.AppUserModel.ID"], vtLPWSTR, "Plain.Value"},
	} {
		if check.got.vt != check.vt || check.text != "" && !strings.EqualFold(check.got.text, check.text) {
			t.Errorf("%s: read %v, want vt %d %q", check.what, check.got, check.vt, check.text)
		}
	}

	// Recorded, not asserted, until the results are in the spec.
	t.Logf("CLSID without braces: %v; PreventPinning \"true\": %v; empty: vt %d",
		second["System.AppUserModel.ToastActivatorCLSID"], second["System.AppUserModel.PreventPinning"], vtEmpty)

	installLog := readLog(log)
	for _, line := range strings.Split(installLog, "\n") {
		if strings.Contains(line, "1946") || strings.Contains(line, "ShortcutProperty") {
			t.Logf("log: %s", strings.TrimSpace(line))
		}
	}

	if !strings.Contains(installLog, "1946") {
		t.Errorf("the unknown property name did not log warning 1946")
	}
}

// msiexecQuietCleanup removes a test product, ignoring errors.
func msiexecQuietCleanup(path string) {
	_ = exec.Command("msiexec", "/x", path, "/qn").Run()
}
