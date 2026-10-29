//go:build windows

package nativetest_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/testpe"
)

// The cases in this file settle claims the spec audit of 2026-10-29 found
// unverified although the builder relies on them (MSISPEC Appendix C).

var pRtlGetVersion = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion") //nolint:gochecknoglobals // lazy procedure

// osVersionInfoEx is RTL_OSVERSIONINFOEXW.
type osVersionInfoEx struct {
	size, major, minor, build, platform uint32
	csd                                 [128]uint16
	servicePackMajor, servicePackMinor  uint16
	suiteMask                           uint16
	productType, reserved               byte
}

const osPropertiesConfig = `schema: msi-builder/v1
product: { name: MsiBuilderOSProperties, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 7c4e2a90-5d1b-4f3e-8a6c-0b9d2e4f6a17 }
package: { platform: x64 }
installDir: { name: MsiBuilderOSProperties }
properties:
  OS_BUILD:
    search: { registry: { root: HKLM, key: 'SOFTWARE\Microsoft\Windows NT\CurrentVersion', name: CurrentBuildNumber } }
components:
  - files: [{ source: app.exe }]
customActions:
  - id: SetBuild22000
    setProperty: { name: BUILD_GE_22000, value: '1' }
    schedule: [{ sequence: execute, after: AppSearch, condition: OS_BUILD >= 22000 }]
  - id: SetBuild99999
    setProperty: { name: BUILD_GE_99999, value: '1' }
    schedule: [{ sequence: execute, after: AppSearch, condition: OS_BUILD >= 99999 }]
`

// TestMsiexecOSProperties records the operating-system properties the engine
// sets on this machine next to the version that RtlGetVersion reports, which
// no compatibility shim changes (PROP-02). VersionNT and WindowsBuild stop
// at Windows 8.1 (603 and 9600), so the builder's file-search warning points
// at the CurrentBuildNumber registry value instead, which this package
// searches and compares with integers in a condition.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecOSProperties(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	info := osVersionInfoEx{size: uint32(unsafe.Sizeof(osVersionInfoEx{}))}
	if r, _, _ := pRtlGetVersion.Call(uintptr(unsafe.Pointer(&info))); r != 0 {
		t.Fatalf("RtlGetVersion: status %#x", r)
	}

	path := buildConfig(t, t.TempDir(), "osprops", osPropertiesConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})
	log := filepath.Join(filepath.Dir(path), "install.log")

	t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run() })

	msiexecQuiet(t, log, "/i", path)

	props := serverProperties(readLog(log))

	t.Logf("PROP-02: RtlGetVersion %d.%d.%d, product type %d, service pack %d.%d",
		info.major, info.minor, info.build, info.productType, info.servicePackMajor, info.servicePackMinor)

	for _, name := range []string{
		"VersionNT", "VersionNT64", "WindowsBuild", "ServicePackLevel", "MsiNTProductType", "Msix64", "Intel", "VersionMsi", "VersionDatabase",
		"OS_BUILD", "BUILD_GE_22000", "BUILD_GE_99999",
	} {
		t.Logf("PROP-02: %s = %q", name, props[name])
	}

	ge22000 := ""
	if info.build >= 22000 {
		ge22000 = "1"
	}

	for name, want := range map[string]string{
		"VersionNT":        "603",
		"WindowsBuild":     "9600",
		"MsiNTProductType": strconv.Itoa(int(info.productType)),
		"OS_BUILD":         strconv.Itoa(int(info.build)),
		"BUILD_GE_22000":   ge22000,
		"BUILD_GE_99999":   "",
	} {
		if got := props[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path)
}

// The folder SEARCH-02 copies ucrtbase.dll to.
const search02Root = `C:\MsiBuilderSearch02`

// TestMsiexecSearchesOSFileVersion compares a version-bounded search for
// ucrtbase.dll in System64Folder with the same search for a copy of the
// file in a private folder (SEARCH-02). The engine is reported to see
// compatibility-shimmed versions of the system's own files.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecSearchesOSFileVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	system := filepath.Join(os.Getenv("SystemRoot"), "System32", "ucrtbase.dll")

	//nolint:gosec // the path is SystemRoot's ucrtbase.dll
	out, err := exec.CommandContext(t.Context(), "powershell.exe", "-NoProfile", "-Command",
		fmt.Sprintf(`(Get-Item '%s').VersionInfo.FileVersionRaw.ToString()`, system)).Output()
	if err != nil {
		t.Fatalf("file version of %s: %v", system, err)
	}

	version := strings.TrimSpace(string(out))

	fields := strings.Split(version, ".")
	if len(fields) != 4 {
		t.Fatalf("file version %q has no four fields", version)
	}

	last, err := strconv.Atoi(fields[3])
	if err != nil {
		t.Fatal(err)
	}

	above := strings.Join(append(fields[:3:3], strconv.Itoa(last+1)), ".")

	cleanup := func() { _ = os.RemoveAll(search02Root) }

	cleanup()
	t.Cleanup(cleanup)

	data, err := os.ReadFile(system)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(search02Root, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(search02Root, "ucrtbase.dll"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	yaml := fmt.Sprintf(`schema: msi-builder/v1
product: { name: MsiBuilderSearch02, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 0b7d3a51-6e2c-4f19-8a4d-2c5e7f9b1d63 }
package: { platform: x64 }
installDir: { name: MsiBuilderSearch02 }
components:
  - files: [{ source: app.exe }]
properties:
  SYS_PLAIN: { value: none, search: { file: { name: ucrtbase.dll, path: '[System64Folder]' } } }
  SYS_EQUAL: { value: none, search: { file: { name: ucrtbase.dll, path: '[System64Folder]', minVersion: %[1]s } } }
  SYS_ABOVE: { value: none, search: { file: { name: ucrtbase.dll, path: '[System64Folder]', minVersion: %[2]s } } }
  SYS_LOW: { value: none, search: { file: { name: ucrtbase.dll, path: '[System64Folder]', minVersion: 6.0.0.0 } } }
  PRIV_EQUAL: { value: none, search: { file: { name: ucrtbase.dll, path: '%[3]s', minVersion: %[1]s } } }
  PRIV_ABOVE: { value: none, search: { file: { name: ucrtbase.dll, path: '%[3]s', minVersion: %[2]s } } }
`, version, above, search02Root)

	path := buildConfig(t, t.TempDir(), "search02", yaml, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})
	log := filepath.Join(filepath.Dir(path), "install.log")

	t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run() })

	msiexecQuiet(t, log, "/i", path)

	props := serverProperties(readLog(log))

	t.Logf("SEARCH-02: ucrtbase.dll file version %s (minVersion %s and %s)", version, version, above)

	for _, name := range []string{"SYS_PLAIN", "SYS_EQUAL", "SYS_ABOVE", "SYS_LOW", "PRIV_EQUAL", "PRIV_ABOVE"} {
		t.Logf("SEARCH-02: %s = %q", name, props[name])
	}

	// The private copy is the control: its search must follow the file's
	// real version.
	if !strings.EqualFold(props["PRIV_EQUAL"], filepath.Join(search02Root, "ucrtbase.dll")) || props["PRIV_ABOVE"] != "none" {
		t.Errorf("private copy: PRIV_EQUAL = %q, PRIV_ABOVE = %q", props["PRIV_EQUAL"], props["PRIV_ABOVE"])
	}

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path)
}

const fileHashConfig = `schema: msi-builder/v1
product: { name: MsiBuilderFileHash, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 9d2e4c6a-1b3f-4a5d-8e7c-6f0a2b4d9e15 }
package: { platform: x64 }
installDir: { name: MsiBuilderFileHash }
components:
  - files: [{ source: app.exe }]
  - files: [{ source: plain.txt }]
`

// TestMsiexecUsesMsiFileHash installs an unversioned file over an identical
// copy whose creation and modification times are equal, so that the engine
// treats the copy as unmodified (FILE-04). With the builder's MsiFileHash
// row the hashes match and the file is not copied; with a wrong hash, and
// with the same rows in a table named FileHash, the copy is replaced.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecUsesMsiFileHash(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	plain := []byte("an unversioned file\r\n")
	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderFileHash")
	target := filepath.Join(installDir, "plain.txt")
	stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	built := buildConfig(t, t.TempDir(), "filehash", fileHashConfig, map[string][]byte{
		"app.exe":   testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
		"plain.txt": plain,
	})

	for _, tc := range []struct {
		name   string
		edit   func(t *testing.T, db *msidb.Database)
		copied bool
	}{
		{"correct MsiFileHash", func(*testing.T, *msidb.Database) {}, false},
		{"wrong MsiFileHash", wrongFileHash, true},
		{"rows in a FileHash table", renameFileHash, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(installDir); err == nil {
				t.Fatalf("%s exists before the installation; remove what an earlier run left", installDir)
			}

			path := editPackage(t, built, tc.edit)

			t.Cleanup(func() {
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
				_ = os.RemoveAll(installDir)
			})

			if err := os.MkdirAll(installDir, 0o755); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(target, plain, 0o600); err != nil {
				t.Fatal(err)
			}

			setFileTimes(t, target, stamp)

			log := filepath.Join(filepath.Dir(path), "install.log")
			msiexecQuiet(t, log, "/i", path)

			for line := range strings.Lines(readLog(log)) {
				if strings.Contains(line, "plain.txt") && strings.Contains(line, "Overwrite") {
					t.Logf("FILE-04 %s: %s", tc.name, strings.TrimSpace(line))
				}
			}

			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}

			if copied := !info.ModTime().Equal(stamp); copied != tc.copied {
				t.Errorf("plain.txt copied = %v (modified %v), want %v\n%s", copied, info.ModTime(), tc.copied, readLog(log))
			}

			msiexecQuiet(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path)
		})
	}
}

// editPackage writes the package at path again beside it after edit and
// returns the new path.
func editPackage(t *testing.T, path string, edit func(t *testing.T, db *msidb.Database)) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	database, err := msidb.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}

	edit(t, database)

	out, err := database.Marshal(msidb.Options{})
	if err != nil {
		t.Fatal(err)
	}

	edited := filepath.Join(t.TempDir(), filepath.Base(path))
	if err := os.WriteFile(edited, out, 0o600); err != nil {
		t.Fatal(err)
	}

	return edited
}

func wrongFileHash(t *testing.T, database *msidb.Database) {
	t.Helper()

	tbl := database.Table("MsiFileHash")
	if tbl == nil || len(tbl.Rows) != 1 {
		t.Fatal("the package has no single MsiFileHash row")
	}

	col := tbl.Column("HashPart1")

	part, ok := tbl.Rows[0][col].(int)
	if !ok {
		t.Fatalf("HashPart1 is %T", tbl.Rows[0][col])
	}

	tbl.Rows[0][col] = part ^ 1
}

func renameFileHash(t *testing.T, database *msidb.Database) {
	t.Helper()

	tbl := database.Table("MsiFileHash")
	if tbl == nil {
		t.Fatal("the package has no MsiFileHash table")
	}

	tbl.Name = "FileHash"

	if validation := database.Table("_Validation"); validation != nil {
		col := validation.Column("Table")
		for _, row := range validation.Rows {
			if row[col] == "MsiFileHash" {
				row[col] = "FileHash"
			}
		}
	}
}

// setFileTimes sets the creation, access and modification times of path.
func setFileTimes(t *testing.T, path string, when time.Time) {
	t.Helper()

	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}

	handle, err := syscall.CreateFile(name, syscall.FILE_WRITE_ATTRIBUTES, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = syscall.CloseHandle(handle) }()

	ft := syscall.NsecToFiletime(when.UnixNano())
	if err := syscall.SetFileTime(handle, &ft, &ft, &ft); err != nil {
		t.Fatal(err)
	}
}

const versionUpgradeConfig = `schema: msi-builder/v1
product: { name: MsiBuilderVersions, manufacturer: msi-builder tests, version: %s, upgradeCode: 4a8c2e6f-0d1b-4c3e-9f5a-7b2d4e6f8a10 }
package: { platform: %s }
installDir: { name: MsiBuilderVersions }
components:
  - files: [{ source: app.exe }]
`

// TestMsiexecUpgradeVersions installs v1 and then v2 of one upgrade code
// (UPG-08, UPG-09). Leading zeros stay in ProductVersion as written, and
// the engine is expected to compare the fields as numbers: 1.010.0 is
// newer than 1.9.0. A package of the other platform upgrades one of this
// platform like any other version.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecUpgradeVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	pf64 := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderVersions")
	pf32 := filepath.Join(os.Getenv("ProgramFiles(x86)"), "MsiBuilderVersions")

	for _, tc := range []struct {
		name, v1, p1, v2, p2 string
		// code is the exit code of the v2 install; folder is where app.exe
		// is afterwards.
		code   int
		folder string
	}{
		{"UPG-08 leading zero upgrades", "1.9.0", "x64", "1.010.0", "x64", 0, pf64},
		{"UPG-08 leading zero downgrade is refused", "1.010.0", "x64", "1.9.0", "x64", 1603, pf64},
		{"UPG-09 x86 upgrades x64", "1.0.0", "x64", "1.1.0", "x86", 0, pf32},
		{"UPG-09 x64 upgrades x86", "1.0.0", "x86", "1.1.0", "x64", 0, pf64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, dir := range []string{pf64, pf32} {
				if _, err := os.Stat(dir); err == nil {
					t.Fatalf("%s exists before the installation; remove what an earlier run left", dir)
				}
			}

			dir := t.TempDir()
			older := buildConfig(t, mkdir(t, dir, "v1"), "v1", fmt.Sprintf(versionUpgradeConfig, tc.v1, tc.p1), map[string][]byte{"app.exe": app})
			newer := buildConfig(t, mkdir(t, dir, "v2"), "v2", fmt.Sprintf(versionUpgradeConfig, tc.v2, tc.p2), map[string][]byte{"app.exe": app})

			t.Cleanup(func() {
				for _, path := range []string{newer, older} {
					_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
				}

				_ = os.RemoveAll(pf64)
				_ = os.RemoveAll(pf32)
			})

			msiexecQuiet(t, filepath.Join(dir, "v1.log"), "/i", older)

			log := filepath.Join(dir, "v2.log")
			code := msiexecCode(t, log, "/i", newer)

			for line := range strings.Lines(readLog(log)) {
				if strings.Contains(line, "FindRelatedProducts: Found application") || strings.Contains(line, "Property(S): ProductVersion") {
					t.Logf("%s: %s", tc.name, strings.TrimSpace(line))
				}
			}

			if code != tc.code {
				t.Fatalf("msiexec /i v2 returned %d, want %d\n%s", code, tc.code, readLog(log))
			}

			other := pf32
			if tc.folder == pf32 {
				other = pf64
			}

			if _, err := os.Stat(filepath.Join(tc.folder, "app.exe")); err != nil {
				t.Errorf("app.exe after the v2 install: %v", err)
			}

			if _, err := os.Stat(filepath.Join(other, "app.exe")); err == nil {
				t.Errorf("app.exe is still in %s: the other version was not removed", other)
			}

			last := newer
			if code != 0 {
				last = older
			}

			msiexecQuiet(t, filepath.Join(dir, "x.log"), "/x", last)
		})
	}
}

const firewallPortsConfig = `schema: msi-builder/v1
product: { name: MsiBuilderFirewallPorts, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 2f6a8c0e-4b1d-4e3f-a5c7-9d1b3f5a7c29 }
package: { platform: x64 }
installDir: { name: MsiBuilderFirewallPorts }
properties:
  PORTS: '80'
components:
  - id: App
    files: [{ source: app.exe }]
firewall:
  - name: MsiBuilderCA07
    component: App
    program: '[#app.exe]'
    protocol: tcp
    port: '%s'
`

// TestMsiexecFirewallPortLists installs a firewall rule whose port is a list
// with a range, written in the package and passed through a property
// (CA-07), reads it back with netsh and checks that uninstall removes it.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecFirewallPortLists(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	const ports = "80,443,8000-8010"

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	for _, tc := range []struct {
		name, port string
		args       []string
	}{
		{"in the package", ports, nil},
		{"from a property", "[PORTS]", []string{"PORTS=" + ports}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := buildConfig(t, t.TempDir(), "ports", fmt.Sprintf(firewallPortsConfig, tc.port), map[string][]byte{"app.exe": app})

			t.Cleanup(func() {
				_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
				// Only the probe's own rule, by its unique name.
				_ = exec.CommandContext(context.Background(), "netsh", "advfirewall", "firewall", "delete", "rule", "name=MsiBuilderCA07").Run()
			})

			log := filepath.Join(filepath.Dir(path), "install.log")
			msiexecQuiet(t, log, append([]string{"/i", path}, tc.args...)...)

			out, err := exec.CommandContext(t.Context(), "netsh", "advfirewall", "firewall", "show", "rule", "name=MsiBuilderCA07").CombinedOutput()
			if err != nil {
				t.Fatalf("netsh show rule: %v\n%s\n%s", err, out, readLog(log))
			}

			var local string

			for line := range strings.Lines(string(out)) {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "LocalPort:"); ok {
					local = strings.TrimSpace(rest)
				}
			}

			t.Logf("CA-07 %s: LocalPort %q", tc.name, local)

			if local != ports {
				t.Errorf("LocalPort = %q, want %q\n%s", local, ports, out)
			}

			msiexecQuiet(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path)

			out, _ = exec.CommandContext(t.Context(), "netsh", "advfirewall", "firewall", "show", "rule", "name=MsiBuilderCA07").CombinedOutput()
			if bytes.Contains(out, []byte("LocalPort")) {
				t.Errorf("the rule is still there after uninstall\n%s", out)
			}
		})
	}
}
