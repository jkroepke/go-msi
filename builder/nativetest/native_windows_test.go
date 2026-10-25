//go:build windows

package nativetest_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/jkroepke/go-msi/builder"
	"github.com/jkroepke/go-msi/config"
	"github.com/jkroepke/go-msi/internal/testpe"
)

var (
	msiDLL                      = syscall.NewLazyDLL("msi.dll")
	pMsiOpenDatabaseW           = msiDLL.NewProc("MsiOpenDatabaseW")
	pMsiDatabaseOpenViewW       = msiDLL.NewProc("MsiDatabaseOpenViewW")
	pMsiViewExecute             = msiDLL.NewProc("MsiViewExecute")
	pMsiViewFetch               = msiDLL.NewProc("MsiViewFetch")
	pMsiViewClose               = msiDLL.NewProc("MsiViewClose")
	pMsiCloseHandle             = msiDLL.NewProc("MsiCloseHandle")
	pMsiRecordGetStringW        = msiDLL.NewProc("MsiRecordGetStringW")
	pMsiRecordGetFieldCount     = msiDLL.NewProc("MsiRecordGetFieldCount")
	pMsiGetSummaryInformationW  = msiDLL.NewProc("MsiGetSummaryInformationW")
	pMsiSummaryInfoGetPropertyW = msiDLL.NewProc("MsiSummaryInfoGetPropertyW")
)

const (
	errorNoMoreItems = 259
	pidRevision      = 9
	pidTemplate      = 7
)

func call(name string, p *syscall.LazyProc, args ...uintptr) error {
	r, _, _ := p.Call(args...)
	if r != 0 {
		return fmt.Errorf("%s: error %d", name, r)
	}

	return nil
}

func wstr(t *testing.T, s string) uintptr {
	t.Helper()

	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		t.Fatal(err)
	}

	return uintptr(unsafe.Pointer(p))
}

func closeHandle(h uint32) { _, _, _ = pMsiCloseHandle.Call(uintptr(h)) }

func openDB(t *testing.T, path string) uint32 {
	t.Helper()

	var h uint32
	if err := call("MsiOpenDatabaseW", pMsiOpenDatabaseW, wstr(t, path), 0 /* MSIDBOPEN_READONLY */, uintptr(unsafe.Pointer(&h))); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { closeHandle(h) })

	return h
}

func recordString(r uint32, field uintptr) string {
	buf := make([]uint16, 4096)
	n := uint32(len(buf))
	_, _, _ = pMsiRecordGetStringW.Call(uintptr(r), field, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))

	return syscall.UTF16ToString(buf[:n])
}

// query runs a SELECT and returns every row as strings.
func query(t *testing.T, db uint32, sql string) [][]string {
	t.Helper()

	var view uint32
	if err := call("MsiDatabaseOpenViewW", pMsiDatabaseOpenViewW, uintptr(db), wstr(t, sql), uintptr(unsafe.Pointer(&view))); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}

	defer closeHandle(view)

	if err := call("MsiViewExecute", pMsiViewExecute, uintptr(view), 0); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}

	var rows [][]string

	for {
		var rec uint32

		r, _, _ := pMsiViewFetch.Call(uintptr(view), uintptr(unsafe.Pointer(&rec)))
		if r == errorNoMoreItems {
			break
		}

		if r != 0 {
			t.Fatalf("%s: MsiViewFetch error %d", sql, r)
		}

		n, _, _ := pMsiRecordGetFieldCount.Call(uintptr(rec))
		row := make([]string, n)

		for i := range row {
			row[i] = recordString(rec, uintptr(i+1))
		}

		closeHandle(rec)
		rows = append(rows, row)
	}

	_, _, _ = pMsiViewClose.Call(uintptr(view))

	return rows
}

func summaryString(t *testing.T, db uint32, pid uintptr) string {
	t.Helper()

	var h uint32
	if err := call("MsiGetSummaryInformationW", pMsiGetSummaryInformationW, uintptr(db), 0, 0, uintptr(unsafe.Pointer(&h))); err != nil {
		t.Fatal(err)
	}

	defer closeHandle(h)

	var (
		typ  uint32
		ival int32
		ft   [2]uint32
		buf  = make([]uint16, 1024)
		size = uint32(len(buf))
	)

	if err := call("MsiSummaryInfoGetPropertyW", pMsiSummaryInfoGetPropertyW, uintptr(h), pid, uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&ival)), uintptr(unsafe.Pointer(&ft)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); err != nil {
		t.Fatal(err)
	}

	return syscall.UTF16ToString(buf[:size])
}

// buildExample builds examples/<name> with synthetic inputs and writes the
// package to a temporary file.
func buildExample(t *testing.T, name string, vars map[string]string, inputs map[string][]byte) (string, *builder.Result) {
	t.Helper()

	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join("..", "..", "examples", name, "msi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "msi.yaml"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	for n, data := range inputs {
		if err := os.WriteFile(filepath.Join(dir, n), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{Vars: vars})
	if err != nil {
		t.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})
	if err != nil {
		t.Fatalf("%v\n%v", err, res.Diagnostics)
	}

	path := filepath.Join(dir, name+".msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, res
}

func minimalInputs() map[string][]byte {
	return map[string][]byte{"hello.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})}
}

// TestMsiDLLOpensPackages opens both examples with msi.dll, reads every
// table and the summary information.
func TestMsiDLLOpensPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("native tests need msi.dll time")
	}

	icon := append([]byte{0, 0, 1, 0, 1, 0}, bytes.Repeat([]byte{0xAB}, 64)...)

	for _, tc := range []struct {
		name   string
		vars   map[string]string
		inputs map[string][]byte
	}{
		{"minimal", nil, minimalInputs()},
		{"windows_exporter", map[string]string{"VERSION": "0.32.0.5"}, map[string][]byte{
			"windows_exporter.exe": testpe.Build(testpe.Options{FileVersion: "0.32.0.5", Languages: []int{1033}}),
			"icon.ico":             icon,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, res := buildExample(t, tc.name, tc.vars, tc.inputs)
			db := openDB(t, path)

			if got := summaryString(t, db, pidRevision); got != res.PackageCode {
				t.Errorf("summary package code %q, want %q", got, res.PackageCode)
			}

			if got := summaryString(t, db, pidTemplate); !strings.HasPrefix(got, "x64;") {
				t.Errorf("template %q", got)
			}

			tables := query(t, db, "SELECT `Name` FROM `_Tables`")
			if len(tables) == 0 {
				t.Fatal("no tables")
			}

			for _, row := range tables {
				name := row[0]
				rows := query(t, db, "SELECT * FROM `"+name+"`")

				want := res.Database.Table(name)
				if want == nil {
					t.Errorf("msi.dll lists table %s that the builder did not write", name)

					continue
				}

				if len(rows) != len(want.Rows) {
					t.Errorf("%s: msi.dll reads %d rows, builder wrote %d", name, len(rows), len(want.Rows))
				}
			}

			props := query(t, db, "SELECT `Value` FROM `Property` WHERE `Property` = 'ProductCode'")
			if len(props) != 1 || props[0][0] != res.ProductCode {
				t.Errorf("ProductCode %v, want %s", props, res.ProductCode)
			}
		})
	}
}

// TestMsiexecInstallsMinimal installs and removes examples/minimal silently.
// It needs an elevated process; without one it is skipped.
func TestMsiexecInstallsMinimal(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path, _ := buildExample(t, "minimal", nil, minimalInputs())
	log := filepath.Join(filepath.Dir(path), "install.log")

	out, err := exec.Command("msiexec", "/i", path, "/qn", "/l*v", log).CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1925 {
			t.Skip("not elevated: msiexec returned 1925")
		}

		t.Fatalf("msiexec /i: %v\n%s\n%s", err, out, readLog(log))
	}

	installed := filepath.Join(os.Getenv("ProgramFiles"), "Hello", "hello.exe")

	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("after install: %v\n%s", err, readLog(log))
	}

	if !bytes.Equal(data, minimalInputs()["hello.exe"]) {
		t.Error("installed hello.exe differs from the source")
	}

	out, err = exec.Command("msiexec", "/x", path, "/qn", "/l*v", log).CombinedOutput()
	if err != nil {
		t.Fatalf("msiexec /x: %v\n%s\n%s", err, out, readLog(log))
	}

	if _, err := os.Stat(installed); err == nil {
		t.Error("hello.exe is still installed after the uninstall")
	}
}

// readLog decodes the UTF-16 log msiexec writes.
func readLog(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		u := make([]uint16, 0, len(data)/2)
		for i := 2; i+1 < len(data); i += 2 {
			u = append(u, uint16(data[i])|uint16(data[i+1])<<8)
		}

		return syscall.UTF16ToString(u)
	}

	return string(data)
}
