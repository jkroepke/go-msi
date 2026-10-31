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
	"time"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// grantUserConfig is a per-user package that grants rights on a folder in
// the user's local AppData to an account from a property (SEC-05).
const grantUserConfig = `schema: msi-builder/v1
product: { name: MsiBuilderGrantUser, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 9d4a6e21-5b7c-4f38-8e1a-3c6b0d2f7a95 }
package: { platform: x64, scope: perUser, compression: none }
installDir: { name: MsiBuilderGrantUser }
properties:
  SVC_ACCOUNT: { secure: true }
components:
  - files: [{ source: app.exe }]
  - id: UserData
    directory: LocalAppDataFolder/MsiBuilderGrantUser
    createFolder: true
    folderPermissions:
      grant: [{ account: '[SVC_ACCOUNT]', rights: modify }]
`

// userAccountsKey is where a per-user install remembers its grant accounts,
// below the user's hive (spec 17.7).
const userAccountsKey = `Software\msi-builder\GrantAccounts\`

// TestMsiexecGrantsPerUser installs grantUserConfig as the administrator
// (SEC-05): the account from the property is remembered in the user's hive,
// not in HKLM, a repair without the property grants to it, and the removal
// forgets it.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecGrantsPerUser(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time")
	}

	dir := t.TempDir()
	path, res := buildConfigResult(t, dir, "grant-user", grantUserConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})
	data := filepath.Join(os.Getenv("LOCALAPPDATA"), "MsiBuilderGrantUser")
	userKey := `HKCU\` + userAccountsKey + res.ProductCode
	machineKey := `HKLM\SOFTWARE\msi-builder\GrantAccounts\` + res.ProductCode

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(data)
	})

	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path, `SVC_ACCOUNT=BUILTIN\Guests`)

	sddl := objectSDDL(t, data)
	t.Logf("SEC-05: per-user folder after the install: %s", sddl)

	if !strings.Contains(sddl, "(A;OICI;0x1301bf;;;BG)") {
		t.Errorf("the folder has no modify ACE for Guests: %s", sddl)
	}

	if got := regQuery(t, userKey); !strings.Contains(got, "[SVC_ACCOUNT]") || !strings.Contains(got, `BUILTIN\Guests`) {
		t.Errorf("after the install, %s does not remember the account:\n%s\n%s", userKey, got,
			grepLog(readLog(filepath.Join(dir, "install.log")), "BuilderGrant"))
	}

	if got := regQuery(t, machineKey); got != "" {
		t.Errorf("a per-user install remembered the account in HKLM:\n%s", got)
	}

	msiexecQuiet(t, filepath.Join(dir, "repair.log"), "/fa", path)

	if log := readLog(filepath.Join(dir, "repair.log")); !strings.Contains(log, "granting to the remembered account") {
		t.Errorf("repair: no grant to the remembered account\n%s", grepLog(log, "BuilderGrant"))
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

	if got := regQuery(t, userKey); got != "" {
		t.Errorf("after the uninstall, the account is still remembered:\n%s", got)
	}
}

// TestNonAdminGrantsPerUser installs grantUserConfig as a standard user
// (SEC-05): the commit action, which runs as that user, remembers the
// account in the user's hive, so a repair without the property grants to it,
// and the removal forgets it.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestNonAdminGrantsPerUser(t *testing.T) {
	user := standardUser(t)
	dir := nonAdminDir(t, "grant-user")
	path, res := buildConfigResult(t, dir, "grant-user", grantUserConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})
	query := `reg.exe query HKCU\` + userAccountsKey + res.ProductCode + ` /v [SVC_ACCOUNT]`

	defer func() { _ = user.msiexec(t, filepath.Join(dir, "cleanup.log"), "/x", path) }()

	install := filepath.Join(dir, "install.log")
	if code := user.msiexec(t, install, "/i", path, `SVC_ACCOUNT=BUILTIN\Guests`); code != 0 {
		t.Fatalf("install returned %d\n%s", code, readLog(install))
	}

	log := grepLog(readLog(install), "BuilderGrant")
	t.Logf("SEC-05 as %s, install:\n%s", user.name, log)

	if !strings.Contains(log, "BuilderGrant: granted") || strings.Contains(log, "cannot remember") {
		t.Errorf("install: the grant or the remembered account failed\n%s", log)
	}

	if code := user.run(t, query); code != 0 {
		t.Errorf("after the install, %s returned %d, want the remembered account", query, code)
	}

	repair := filepath.Join(dir, "repair.log")
	if code := user.msiexec(t, repair, "/fa", path); code != 0 {
		t.Fatalf("repair returned %d\n%s", code, readLog(repair))
	}

	if log := grepLog(readLog(repair), "BuilderGrant"); !strings.Contains(log, "granting to the remembered account") {
		t.Errorf("repair: no grant to the remembered account\n%s", log)
	}

	uninstall := filepath.Join(dir, "uninstall.log")
	if code := user.msiexec(t, uninstall, "/x", path); code != 0 {
		t.Fatalf("uninstall returned %d\n%s", code, readLog(uninstall))
	}

	if code := user.run(t, query); code == 0 {
		t.Errorf("after the uninstall, %s still finds the account", query)
	}
}

// repairWithPlantedAccount plants BUILTIN\Users as the remembered account
// of a per-machine product in the administrator's own hive, where a
// per-user install would keep it, and repairs the product with ALLUSERS=""
// and no account (SEC-05): the DLL follows the context the product is
// registered in, so the folder gets no grant for Users and the account
// remembered in HKLM is used. The log shows whether the engine kept
// ALLUSERS empty, so that the per-user marker reached the DLL.
func repairWithPlantedAccount(t *testing.T, dir, path, productCode, folder string) {
	t.Helper()

	planted := `HKCU\` + userAccountsKey + productCode
	if out, err := exec.CommandContext(t.Context(), "reg", "add", planted, "/v", "[SVC_ACCOUNT]", "/d", `BUILTIN\Users`, "/f").CombinedOutput(); err != nil {
		t.Fatalf("reg add %s: %v\n%s", planted, err, out)
	}

	defer func() { _ = exec.CommandContext(context.Background(), "reg", "delete", planted, "/f").Run() }()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	log := filepath.Join(dir, "repair-allusers.log")
	cmd := exec.CommandContext(ctx, "msiexec")
	// ALLUSERS="" needs the quotes as msiexec reads them, which the
	// argument escaping of os/exec would change.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`msiexec /i %s REINSTALL=ALL REINSTALLMODE=amus ALLUSERS="" /qn /norestart /l*v %s`,
		syscall.EscapeArg(path), syscall.EscapeArg(log))}
	err := cmd.Run()

	text := readLog(log)
	t.Logf("SEC-05: repair with ALLUSERS=\"\" of a per-machine product: %v\n%s", err, grepLog(text, "BuilderGrant", "ALLUSERS"))

	if err != nil {
		t.Fatalf("the repair with ALLUSERS=\"\" failed: %v", err)
	}

	// The account remembered in HKLM is still used.
	if !strings.Contains(text, "granting to the remembered account") {
		t.Errorf("the repair did not grant to the account remembered in HKLM\n%s", grepLog(text, "BuilderGrant:"))
	}

	if sddl := objectSDDL(t, folder); strings.Contains(sddl, "(A;OICI;0x1301bf;;;BU)") {
		t.Errorf("the repair granted to the account planted in the user's hive: %s", sddl)
	}

	if strings.Contains(text, `to: BUILTIN\Users`) {
		t.Errorf("the repair log grants to the planted account\n%s", grepLog(text, "BuilderGrant:"))
	}
}

// regQuery returns what reg query lists for a key and its subkeys, or ""
// when the key does not exist.
func regQuery(t *testing.T, key string) string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "reg", "query", key, "/s").CombinedOutput()
	if err != nil {
		return ""
	}

	return string(out)
}
