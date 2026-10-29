//go:build windows

package nativetest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// sec02Root is the ProgramData folder of SEC-02.
const sec02Root = `MsiBuilderSec02`

// sec02SDDLConfig secures a file, a new folder, a folder that exists before
// the install and a registry key with SDDL strings that have no P (protected)
// flag.
const sec02SDDLConfig = `schema: msi-builder/v1
product: { name: MsiBuilderSec02, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 3a8f1c52-9e0d-4b7a-a6c3-2d5e8f1b7c94 }
package: { platform: x64 }
installDir: { name: MsiBuilderSec02 }
components:
  - files:
      - source: app.exe
        permissions: { sddl: 'D:(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;NS)' }
  - id: NewFolder
    directory: CommonAppDataFolder/MsiBuilderSec02/new
    createFolder: true
    folderPermissions: { sddl: 'D:(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;NS)' }
  - id: OldFolder
    directory: CommonAppDataFolder/MsiBuilderSec02/existing
    createFolder: true
    folderPermissions: { sddl: 'D:(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;NS)' }
  - id: Key
    registry:
      - root: HKLM
        key: 'SOFTWARE\MsiBuilderTest\Sec02'
        name: Value
        value: x
        permissions: { sddl: 'D:(A;CI;KA;;;SY)(A;CI;KA;;;BA)(A;CI;KR;;;NS)' }
`

// grantGuests gives a folder an explicit ACE for Guests that no package
// authors.
func grantGuests(t *testing.T, folder string) {
	t.Helper()

	//nolint:gosec // the folder is a test-controlled path
	if out, err := exec.CommandContext(t.Context(), "icacls", folder, "/grant", "*S-1-5-32-546:(OI)(CI)R").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v\n%s", err, out)
	}
}

// objectSDDL returns the SDDL of a file, folder or registry key.
func objectSDDL(t *testing.T, path string) string {
	t.Helper()

	out, err := powershell("(Get-Acl -Path '" + path + "').Sddl")
	if err != nil {
		return "error: " + out
	}

	return out
}

// TestMsiexecSDDLWithoutProtection checks what MsiLockPermissionsEx does with
// an SDDL string without the P (protected) flag (SEC-02): files and folders
// still get a protected DACL with the authored ACEs only, so they drop what
// they inherit, and a folder that existed before the install loses its own
// explicit ACEs; a registry key reads back unprotected, with the authored
// ACEs marked inherited and none of its parent's. An SDDL replaces the DACL
// whatever its flags, which is why grants exist.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecSDDLWithoutProtection(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	root := filepath.Join(os.Getenv("ProgramData"), sec02Root)
	existing := filepath.Join(root, "existing")

	if err := os.MkdirAll(existing, 0o750); err != nil {
		t.Fatal(err)
	}

	grantGuests(t, existing)

	path := buildConfig(t, t.TempDir(), "sec02", sec02SDDLConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(root)
		_ = exec.CommandContext(context.Background(), "reg", "delete", `HKLM\SOFTWARE\MsiBuilderTest\Sec02`, "/f").Run()
	})

	t.Logf("SEC-02: existing folder before install: %s", objectSDDL(t, existing))

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "install.log"), "/i", path)

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderSec02")
	for _, object := range []struct{ name, path string }{
		{"file", filepath.Join(installDir, "app.exe")},
		{"new folder", filepath.Join(root, "new")},
		{"existing folder", existing},
	} {
		got := objectSDDL(t, object.path)
		t.Logf("SEC-02: after install, %s: %s", object.name, got)

		if !strings.Contains(got, "D:P(") || strings.Contains(got, "ID;") || strings.Contains(got, ";;;BG)") || !strings.Contains(got, ";;;NS)") {
			t.Errorf("%s: %s, want a protected DACL with the authored ACEs only", object.name, got)
		}
	}

	key := objectSDDL(t, `HKLM:\SOFTWARE\MsiBuilderTest\Sec02`)
	t.Logf("SEC-02: after install, registry key: %s", key)

	if strings.Contains(key, "D:P") || strings.Count(key, "(") != 3 ||
		!strings.Contains(key, "(A;CIID;KA;;;SY)") || !strings.Contains(key, "(A;CIID;KA;;;BA)") || !strings.Contains(key, "(A;CIID;KR;;;NS)") {
		t.Errorf("registry key: %s, want the authored ACEs, marked inherited, and no others", key)
	}

	msiexecQuiet(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path)
}

// grantConfig grants rights on a file to an account from a property, on a
// new folder and a folder that exists before the install, and on a service.
// FAILNOW=1 fails the install after the grants, so the rollback runs.
const grantConfig = `schema: msi-builder/v1
product: { name: MsiBuilderGrant, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 5b2e7d14-8c3f-4a96-b1d0-6e9f2a4c8b37 }
package: { platform: x64, compression: none }
installDir: { name: MsiBuilderGrant }
components:
  - files:
      - source: app.exe
        permissions:
          grant: [{ account: '[SVC_ACCOUNT]', rights: readExecute }]
    services:
      - name: MsiBuilderGrantSvc
        start: demand
        permissions:
          grant: [{ account: BU, rights: [start, stop] }]
  - id: NewFolder
    directory: CommonAppDataFolder/MsiBuilderGrant/data
    createFolder: true
    folderPermissions:
      grant: [{ account: '[SVC_ACCOUNT]', rights: modify }]
  - id: OldFolder
    directory: CommonAppDataFolder/MsiBuilderGrant/existing
    createFolder: true
    folderPermissions:
      grant:
        - { account: 'NT AUTHORITY\LocalService', rights: full }
        - { account: S-1-5-32-546, rights: readExecute }
customActions:
  - id: FailNow
    run: { command: '"[System64Folder]cmd.exe" /c exit 1' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, before: InstallFinalize, condition: FAILNOW=1 }]
`

// TestMsiexecGrants installs grantConfig (SEC-02): each grant adds one
// explicit ACE, the inherited ACEs and the explicit ACE the test put on the
// existing folder stay, a repair adds no second ACE, the rollback of a
// failed install takes the rights away again, the rollback of a failed
// repair keeps the rights the install granted, an empty account fails the
// install, and uninstalling leaves the ACE on the folder that existed
// before.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecGrants(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	root := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderGrant")
	existing := filepath.Join(root, "existing")
	data := filepath.Join(root, "data")
	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderGrant")
	app := filepath.Join(installDir, "app.exe")

	if err := os.MkdirAll(existing, 0o750); err != nil {
		t.Fatal(err)
	}

	// A file of its own keeps the folder when the package is removed.
	if err := os.WriteFile(filepath.Join(existing, "keep.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	grantGuests(t, existing)

	dir := t.TempDir()
	path := buildConfig(t, dir, "grant", grantConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(root)
	})

	before := objectSDDL(t, existing)
	t.Logf("SEC-02: existing folder before: %s", before)

	// A name without spaces, which LookupAccountNameW resolves (on an
	// English system), keeps msiexec's command line simple.
	account := `SVC_ACCOUNT=BUILTIN\Guests`

	// Rollback: the grants ran, then FailNow failed the install.
	if code := msiexecCode(t, filepath.Join(dir, "rollback.log"), "/i", path, account, "FAILNOW=1"); code != 1603 {
		t.Fatalf("install with FAILNOW=1 returned %d, want 1603", code)
	}

	log := readLog(filepath.Join(dir, "rollback.log"))
	for _, want := range []string{"BuilderGrant: granted", "BuilderGrant: revoked"} {
		if !strings.Contains(log, want) {
			t.Errorf("rollback log has no %q", want)
		}
	}

	if got := objectSDDL(t, existing); got != before {
		t.Errorf("existing folder after rollback:\n%s\nwant\n%s", got, before)
	}

	requireNoGrantRecord(t, "after the rollback")

	// An empty account fails the install.
	if code := msiexecCode(t, filepath.Join(dir, "empty.log"), "/i", path); code != 1603 {
		t.Errorf("install without SVC_ACCOUNT returned %d, want 1603", code)
	}

	if log := readLog(filepath.Join(dir, "empty.log")); !strings.Contains(log, "no account to grant rights to") {
		t.Error("empty account: no BuilderGrant message in the log")
	}

	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path, account)

	requireNoGrantRecord(t, "after the install")

	installed := map[string]string{}
	for name, object := range map[string]string{"file": app, "data folder": data, "existing folder": existing} {
		installed[name] = objectSDDL(t, object)
		t.Logf("SEC-02: grant, %s: %s", name, installed[name])
	}

	installed["service"] = serviceSDDL(t, "MsiBuilderGrantSvc")
	t.Logf("SEC-02: grant, service: %s", installed["service"])

	for name, want := range map[string][]string{
		"file":            {"(A;;0x1200a9;;;BG)", ";ID;"},
		"data folder":     {"(A;OICI;0x1301bf;;;BG)", ";ID;"},
		"existing folder": {"(A;OICI;FA;;;LS)", "(A;OICI;0x1200a9;;;BG)", ";ID;"},
		"service":         {"(A;;RPWP;;;BU)"},
	} {
		for _, part := range want {
			if !strings.Contains(installed[name], part) {
				t.Errorf("%s: %s has no %s", name, installed[name], part)
			}
		}
	}

	// A repair grants again and finds the rights there.
	msiexecQuiet(t, filepath.Join(dir, "repair.log"), "/fa", path, account)

	if log := readLog(filepath.Join(dir, "repair.log")); !strings.Contains(log, "already granted") {
		t.Error("repair: no grant ran")
	}

	for name, object := range map[string]string{"file": app, "data folder": data, "existing folder": existing} {
		if got := objectSDDL(t, object); got != installed[name] {
			t.Errorf("%s after repair:\n%s\nwant\n%s", name, got, installed[name])
		}
	}

	// A repair that fails after the grants keeps the rights that were there.
	if code := msiexecCode(t, filepath.Join(dir, "repair-rollback.log"), "/fa", path, account, "FAILNOW=1"); code != 1603 {
		t.Fatalf("repair with FAILNOW=1 returned %d, want 1603", code)
	}

	for name, object := range map[string]string{"data folder": data, "existing folder": existing} {
		if got := objectSDDL(t, object); got != installed[name] {
			t.Errorf("%s after a failed repair:\n%s\nwant\n%s", name, got, installed[name])
		}
	}

	if got := serviceSDDL(t, "MsiBuilderGrantSvc"); got != installed["service"] {
		t.Errorf("service after a failed repair: %s, want %s", got, installed["service"])
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

	// Uninstalling leaves the ACE on the folder that outlives the package.
	if got := objectSDDL(t, existing); got != installed["existing folder"] {
		t.Errorf("existing folder after uninstall:\n%s\nwant\n%s", got, installed["existing folder"])
	}
}

// requireNoGrantRecord fails when a product's rollback record of grants
// is still in LocalSystem's hive, where the deferred action, which runs as
// LocalSystem, keeps it until the commit or rollback action deletes it.
func requireNoGrantRecord(t *testing.T, when string) {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "reg", "query", `HKU\S-1-5-18\Software\msi-builder\GrantRollback`).CombinedOutput()
	if err != nil {
		return // no record of any product
	}

	if strings.Contains(string(out), "GrantRollback\\{") {
		t.Errorf("%s, a grant record is left:\n%s", when, out)
	}
}

// serviceSDDL returns the security descriptor of a service.
func serviceSDDL(t *testing.T, name string) string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "sc.exe", "sdshow", name).CombinedOutput()
	if err != nil {
		t.Fatalf("sc sdshow %s: %v\n%s", name, err, out)
	}

	return strings.TrimSpace(string(out))
}
