//go:build windows

package pe //nolint:testpackage // the cases reuse the unexported image builders

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

var (
	msiDLL             = syscall.NewLazyDLL("msi.dll")        //nolint:gochecknoglobals // one DLL table for the file
	pMsiGetFileVersion = msiDLL.NewProc("MsiGetFileVersionW") //nolint:gochecknoglobals // one DLL table for the file
)

// msiFileVersion returns what MsiGetFileVersionW reports for path: the
// version and language strings, and ok false when it reports no version.
func msiFileVersion(t *testing.T, path string) (string, string, bool) {
	t.Helper()

	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}

	const verSize, langSize = 256, 1024

	ver := make([]uint16, verSize)
	lang := make([]uint16, langSize)
	verLen, langLen := uint32(verSize), uint32(langSize)

	ret, _, _ := pMsiGetFileVersion.Call(
		uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(&ver[0])), uintptr(unsafe.Pointer(&verLen)),
		uintptr(unsafe.Pointer(&lang[0])), uintptr(unsafe.Pointer(&langLen)),
	)

	switch syscall.Errno(ret) { //nolint:exhaustive // the other codes fail the test
	case 0:
		return syscall.UTF16ToString(ver), syscall.UTF16ToString(lang), true
	case syscall.Errno(1006): // ERROR_FILE_INVALID: no version resource
		return "", "", false
	default:
		t.Fatalf("MsiGetFileVersionW: %v", syscall.Errno(ret))

		return "", "", false
	}
}

// TestMsiGetFileVersionAgrees parses well-formed and corrupt images with
// this package and with MsiGetFileVersionW, which fills File.Version and
// File.Language for Windows toolchains, and checks that both report the
// same version and languages, or both none (FILE-14).
func TestMsiGetFileVersionAgrees(t *testing.T) {
	t.Parallel()

	shrinkTranslation := func(t *testing.T) []byte {
		t.Helper()

		data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033, 1031}})

		binary.LittleEndian.PutUint16(data[blockOffset(t, data, "Translation"):], 36)

		return data
	}

	versionHeader := func(field int, value uint16) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			t.Helper()

			data := testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}})
			binary.LittleEndian.PutUint16(data[blockOffset(t, data, "VS_VERSION_INFO")+field:], value)

			return data
		}
	}

	patch := func(off int, value uint32) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			t.Helper()

			return corruptImage(t, off, value)
		}
	}

	cases := map[string]func(*testing.T) []byte{
		"well formed": func(*testing.T) []byte {
			return testpe.Build(testpe.Options{FileVersion: "1.2.3.4", ProductVersion: "1.2.0.0", Languages: []int{1033, 1031}})
		},
		"no translations":               func(*testing.T) []byte { return testpe.Build(testpe.Options{FileVersion: "0.32.0.5"}) },
		"no version resource":           func(*testing.T) []byte { return testpe.Build(testpe.Options{}) },
		"resource section raw size 0":   patch(testRsrcSection+16, 0),
		"translation past its Var":      shrinkTranslation,
		"NumberOfRvaAndSizes 0":         patch(testResourceDir-20, 0),
		"NumberOfRvaAndSizes 2":         patch(testResourceDir-20, 2),
		"wValueLength 65535":            versionHeader(2, 0xFFFF),
		"root wType text":               versionHeader(4, 1),
		"resource directory size huge":  patch(testResourceDir+4, 0x7FFFFFFF),
		"resource directory size wraps": patch(testResourceDir+4, 0xFFFFF000),
	}

	dir := t.TempDir()

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := build(t)

			path := filepath.Join(dir, filepath.Base(t.Name())+".exe")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}

			wantVer, wantLang, wantOK := msiFileVersion(t, path)

			info, err := Parse(bytes.NewReader(data))
			if err != nil && !errors.Is(err, ErrNotPE) {
				t.Logf("Parse: %v", err)
			}

			gotOK := err == nil && info.HasVersion

			t.Logf("Windows: ok=%v version %q language %q", wantOK, wantVer, wantLang)

			if gotOK != wantOK {
				t.Fatalf("Parse reports a version: %v, MsiGetFileVersionW: %v (%+v)", gotOK, wantOK, info)
			}

			if gotOK && (info.FileVersion != wantVer || info.Language() != wantLang) {
				t.Errorf("Parse: %q %q, MsiGetFileVersionW: %q %q", info.FileVersion, info.Language(), wantVer, wantLang)
			}
		})
	}
}
