//go:build windows

package nativetest_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

var pMapFileAndCheckSumW = syscall.NewLazyDLL("imagehlp.dll").NewProc("MapFileAndCheckSumW") //nolint:gochecknoglobals // lazy procedure

// buildConfig writes inputs next to the configuration src and builds it into
// dir/name.msi.
func buildConfig(t *testing.T, dir, name, src string, inputs map[string][]byte) string {
	t.Helper()

	for file, data := range inputs {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(src), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, res.Diagnostics)
	}

	path := filepath.Join(dir, name+".msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

const downgradeConfig = `schema: msi-builder/v1
product: { name: MsiBuilderDowngrade, manufacturer: msi-builder tests, version: %s, upgradeCode: 3c1d7e52-8a40-4f6b-9b2e-5d7a1c0e9f34 }
package: { platform: x64 }
installDir: { name: MsiBuilderDowngrade }
upgrade: { removeOld: %s }
components:
  - files: [{ source: app.exe }]
  - files: [{ source: lib.dll%s }]
`

// TestMsiexecDowngradedFile upgrades a product whose new version ships an
// older lib.dll (UPG-07). The new component is disallowed against the
// installed file with the higher version; an early removal then deletes it,
// a late one keeps the old file, and an authored higher File.Version
// installs the new one.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecDowngradedFile(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	libV1 := testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1033}, Padding: 1})
	libV2 := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}, Padding: 2})
	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderDowngrade")
	installed := filepath.Join(installDir, "lib.dll")

	for _, tc := range []struct {
		name, removeOld, version string
		want                     []byte
	}{
		{"early removal deletes the file", config.DefaultRemoveOld, "", nil},
		{"late removal keeps the old file", config.PreserveRemoveOld, "", libV1},
		{"authored version installs the new file", config.DefaultRemoveOld, ", version: 2.0.0.1", libV2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(installDir); err == nil {
				t.Fatalf("%s exists before the installation; remove what an earlier run left", installDir)
			}

			dir := t.TempDir()
			v1 := buildConfig(t, mkdir(t, dir, "v1"), "v1", fmt.Sprintf(downgradeConfig, "1.0.0", tc.removeOld, ""), map[string][]byte{"app.exe": app, "lib.dll": libV1})
			v2 := buildConfig(t, mkdir(t, dir, "v2"), "v2", fmt.Sprintf(downgradeConfig, "1.1.0", tc.removeOld, tc.version), map[string][]byte{"app.exe": app, "lib.dll": libV2})

			t.Cleanup(func() {
				for _, path := range []string{v2, v1} {
					_, _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").CombinedOutput()
				}

				_ = os.RemoveAll(installDir)
			})

			msiexecQuiet(t, filepath.Join(dir, "v1.log"), "/i", v1)

			log := filepath.Join(dir, "v2.log")
			msiexecQuiet(t, log, "/i", v2)

			data, err := os.ReadFile(installed)

			switch {
			case tc.want == nil && err == nil:
				t.Errorf("lib.dll is still installed (%d bytes); the engine was expected to delete it", len(data))
			case tc.want != nil && err != nil:
				t.Errorf("lib.dll after the upgrade: %v\n%s", err, readLog(log))
			case tc.want != nil && !bytes.Equal(data, tc.want):
				t.Errorf("lib.dll holds other bytes than expected (%d bytes)", len(data))
			}

			// A repair restores the deleted file.
			msiexecQuiet(t, filepath.Join(dir, "repair.log"), "/fa", v2)

			if _, err := os.Stat(installed); err != nil {
				t.Errorf("lib.dll after the repair: %v", err)
			}

			msiexecQuiet(t, filepath.Join(dir, "x.log"), "/x", v2)
		})
	}
}

func mkdir(t *testing.T, parent, name string) string {
	t.Helper()

	dir := filepath.Join(parent, name)
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}

	return dir
}

const checksumConfig = `schema: msi-builder/v1
product: { name: MsiBuilderChecksum, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 9a4f1e27-6c3b-4d08-b5e9-7f2a0c8d1b63 }
package: { platform: x64 }
installDir: { name: MsiBuilderChecksum }
components:
  - files: [{ source: app.exe, checksum: true }]
`

// withPEChecksum returns data with its PE checksum set to the value
// imagehlp computes, or to a wrong one.
func withPEChecksum(t *testing.T, data []byte, correct bool) []byte {
	t.Helper()

	off := int(binary.LittleEndian.Uint32(data[0x3C:])) + 24 + 64 // e_lfanew, file header, CheckSum
	out := bytes.Clone(data)

	if !correct {
		binary.LittleEndian.PutUint32(out[off:], 0x12345678)

		return out
	}

	binary.LittleEndian.PutUint32(out[off:], 0)

	path := filepath.Join(t.TempDir(), "checksum.exe")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}

	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}

	var header, sum uint32
	if rc, _, _ := pMapFileAndCheckSumW.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&header)), uintptr(unsafe.Pointer(&sum))); rc != 0 {
		t.Fatalf("MapFileAndCheckSumW returned %d", rc)
	}

	binary.LittleEndian.PutUint32(out[off:], sum)

	return out
}

// TestMsiexecVerifiesChecksum installs a file with checksum: true (FILE-09).
// The builder sets MSICHECKCRCS, so a wrong PE checksum fails the install
// with 1603 and a correct one installs. Without MSICHECKCRCS the engine
// installed the wrong checksum.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecVerifiesChecksum(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	base := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}, Checksum: 1})
	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderChecksum")

	for _, tc := range []struct {
		name    string
		correct bool
		want    int
	}{
		{"wrong checksum fails", false, 1603},
		{"correct checksum installs", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := buildConfig(t, dir, "checksum", checksumConfig, map[string][]byte{"app.exe": withPEChecksum(t, base, tc.correct)})

			t.Cleanup(func() {
				_, _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").CombinedOutput()
				_ = os.RemoveAll(installDir)
			})

			log := filepath.Join(dir, "install.log")
			if got := msiexecCode(t, log, "/i", path); got != tc.want {
				t.Fatalf("install exit %d, want %d\n%s", got, tc.want, readLog(log))
			}

			_, err := os.Stat(filepath.Join(installDir, "app.exe"))
			if installedOK := err == nil; installedOK != (tc.want == 0) {
				t.Errorf("app.exe installed = %v after exit %d", installedOK, tc.want)
			}
		})
	}
}

const dotNamesConfig = `schema: msi-builder/v1
product: { name: MsiBuilderDotNames, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 6e2a9c41-0b7d-4e85-a1f3-2c9d8b7e5a16 }
package: { platform: x64 }
installDir: { name: MsiBuilderDotNames }
components:
  - files: [{ source: app.exe }, { source: .gitignore }, { source: .a.b.c }]
  - directory: .git
    files: [{ source: config }]
  - directory: .config/.x
    files: [{ source: y.txt }]
`

// TestMsiexecInstallsDotNames installs folders and files whose names start
// with a dot (FILE-10): the generated short names pass ICE03 and the long
// names install as written.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecInstallsDotNames(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	dir := t.TempDir()
	path := buildConfig(t, dir, "dotnames", dotNamesConfig, map[string][]byte{
		"app.exe":    testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
		".gitignore": []byte("*.log\n"),
		".a.b.c":     []byte("abc\n"),
		"config":     []byte("[core]\n"),
		"y.txt":      []byte("y\n"),
	})

	if cub := os.Getenv("MSI_BUILDER_ICE_CUB"); cub != "" {
		requireCleanICE(t, path, cub)
	}

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderDotNames")
	if _, err := os.Stat(installDir); err == nil {
		t.Fatalf("%s exists before the installation; remove what an earlier run left", installDir)
	}

	installed := true

	t.Cleanup(func() {
		if installed {
			_, _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").CombinedOutput()
		}

		_ = os.RemoveAll(installDir)
	})

	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	for _, name := range []string{".gitignore", ".a.b.c", `.git\config`, `.config\.x\y.txt`} {
		if _, err := os.Stat(filepath.Join(installDir, name)); err != nil {
			t.Errorf("after install: %v", err)
		}
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

	installed = false

	if _, err := os.Stat(installDir); err == nil {
		t.Errorf("%s is left after uninstall", installDir)
	}
}

const disabledServiceConfig = `schema: msi-builder/v1
product: { name: MsiBuilderDisabledSvc, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 4b8e2f19-7a3c-4d56-9e01-c2a7f5b8d934 }
package: { platform: x64 }
installDir: { name: MsiBuilderDisabledSvc }
components:
  - files: [{ source: svc.exe }]
    services:
      - { name: MsiBuilderDisabledSvc, start: disabled }
`

// TestMsiexecInstallsDisabledService installs a service with start:
// disabled and the default control settings (SVC-05). StartServices fails
// a disabled service with 1920, so startOnInstall defaults to false there
// and the install leaves the service registered and stopped.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecInstallsDisabledService(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	const name = "MsiBuilderDisabledSvc"

	dir := t.TempDir()
	path := buildConfig(t, dir, "disabled", disabledServiceConfig, map[string][]byte{"svc.exe": testService(t)})
	installDir := filepath.Join(os.Getenv("ProgramFiles"), name)

	t.Cleanup(func() {
		_, _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").CombinedOutput()
		_, _ = exec.CommandContext(context.Background(), "sc", "delete", name).CombinedOutput()
		_ = os.RemoveAll(installDir)
	})

	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	if state := serviceState(t, name); !strings.Contains(state, "STOPPED") {
		t.Errorf("service state %q, want STOPPED", state)
	}

	out, err := exec.CommandContext(t.Context(), "sc", "qc", name).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "DISABLED") {
		t.Errorf("sc qc %s: %v\n%s", name, err, out)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

	if state := serviceState(t, name); state != "" {
		t.Errorf("service still registered after uninstall: %q", state)
	}
}
