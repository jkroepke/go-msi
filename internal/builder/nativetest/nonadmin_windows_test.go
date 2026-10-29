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

// The non-administrator tests install packages as a standard user that the
// CI step "go test (non-admin)" creates on the disposable runner. They read
// the account from MSI_BUILDER_NONADMIN_USER and MSI_BUILDER_NONADMIN_PASSWORD
// and skip without them; never run them on a machine you keep, since
// TestNonAdminSecureProperties turns on AlwaysInstallElevated for its run.

var pCreateProcessWithLogonW = syscall.NewLazyDLL("advapi32.dll").NewProc("CreateProcessWithLogonW") //nolint:gochecknoglobals // lazy procedure

const (
	logonWithProfile = 1
	// nonAdminRoot holds the packages and logs, readable and writable by
	// the standard user.
	nonAdminRoot = `C:\MsiBuilderNonAdmin`
	// nonAdminTimeout bounds one msiexec run in milliseconds.
	nonAdminTimeout = 5 * 60 * 1000
)

// nonAdminUser is the standard user the tests run msiexec as.
type nonAdminUser struct {
	name, password string
}

// standardUser returns the test account and prepares nonAdminRoot, or skips.
func standardUser(t *testing.T) nonAdminUser {
	t.Helper()

	if testing.Short() {
		t.Skip("installing needs msiexec time")
	}

	user := nonAdminUser{name: os.Getenv("MSI_BUILDER_NONADMIN_USER"), password: os.Getenv("MSI_BUILDER_NONADMIN_PASSWORD")}
	if user.name == "" || user.password == "" {
		t.Skip("MSI_BUILDER_NONADMIN_USER and MSI_BUILDER_NONADMIN_PASSWORD are not set")
	}

	if err := os.MkdirAll(nonAdminRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	// BUILTIN\Users may modify the folder, so the user can write its logs.
	if out, err := exec.CommandContext(t.Context(), "icacls", nonAdminRoot, "/grant", "*S-1-5-32-545:(OI)(CI)M").CombinedOutput(); err != nil {
		t.Fatalf("icacls: %v\n%s", err, out)
	}

	return user
}

// run starts command line as the user with its profile loaded and returns
// the exit code.
func (u nonAdminUser) run(t *testing.T, cmdline string) int {
	t.Helper()

	var (
		startup syscall.StartupInfo
		process syscall.ProcessInformation
	)

	startup.Cb = uint32(unsafe.Sizeof(startup))
	line := wstr(t, cmdline)

	r, _, err := pCreateProcessWithLogonW.Call(
		uintptr(unsafe.Pointer(wstr(t, u.name))), uintptr(unsafe.Pointer(wstr(t, "."))), uintptr(unsafe.Pointer(wstr(t, u.password))),
		logonWithProfile, 0, uintptr(unsafe.Pointer(line)), 0, 0, uintptr(unsafe.Pointer(wstr(t, nonAdminRoot))),
		uintptr(unsafe.Pointer(&startup)), uintptr(unsafe.Pointer(&process)))
	if r == 0 {
		t.Fatalf("CreateProcessWithLogonW %s: %v", cmdline, err)
	}

	defer func() {
		_ = syscall.CloseHandle(process.Thread)
		_ = syscall.CloseHandle(process.Process)
	}()

	if event, err := syscall.WaitForSingleObject(process.Process, nonAdminTimeout); err != nil || event != syscall.WAIT_OBJECT_0 {
		_ = syscall.TerminateProcess(process.Process, 1)

		t.Fatalf("%s did not end within %d ms (%d, %v)", cmdline, nonAdminTimeout, event, err)
	}

	var code uint32
	if err := syscall.GetExitCodeProcess(process.Process, &code); err != nil {
		t.Fatal(err)
	}

	return int(code)
}

// msiexec runs msiexec silently as the user with a verbose log.
func (u nonAdminUser) msiexec(t *testing.T, log string, args ...string) int {
	t.Helper()

	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsRune(arg, ' ') && !strings.Contains(arg, "=") {
			arg = `"` + arg + `"`
		}

		quoted = append(quoted, arg)
	}

	return u.run(t, fmt.Sprintf(`msiexec.exe %s /qn /norestart /l*v "%s"`, strings.Join(quoted, " "), log))
}

// policy runs reg.exe on the user's Installer policy key. A standard user
// may not write its own HKCU\SOFTWARE\Policies, so the administrator does,
// through HKU\<SID>; when the user's hive is not loaded, it loads
// NTUSER.DAT for the change. A logon with profile first creates the profile.
func (u nonAdminUser) policy(t *testing.T, action string, args ...string) error {
	t.Helper()

	if code := u.run(t, "cmd.exe /c exit 0"); code != 0 {
		return fmt.Errorf("logon of %s returned %d", u.name, code)
	}

	sid, _, _, err := syscall.LookupSID("", u.name)
	if err != nil {
		return fmt.Errorf("security identifier of %s: %w", u.name, err)
	}

	hive, err := sid.String()
	if err != nil {
		return fmt.Errorf("security identifier of %s: %w", u.name, err)
	}

	hive = `HKU\` + hive
	ctx := context.Background()
	key := hive + `\SOFTWARE\Policies\Microsoft\Windows\Installer`

	if exec.CommandContext(ctx, "reg.exe", "query", hive).Run() != nil {
		profile := filepath.Join(os.Getenv("SystemDrive")+`\`, "Users", u.name, "NTUSER.DAT")
		//nolint:gosec // the hive and profile path come from the test account
		if out, err := exec.CommandContext(ctx, "reg.exe", "load", hive, profile).CombinedOutput(); err != nil {
			return fmt.Errorf("reg load %s: %w\n%s", profile, err, out)
		}

		defer func() { _ = exec.CommandContext(ctx, "reg.exe", "unload", hive).Run() }()
	}

	//nolint:gosec // the test passes fixed reg.exe arguments
	if out, err := exec.CommandContext(ctx, "reg.exe", append([]string{action, key}, args...)...).CombinedOutput(); err != nil {
		return fmt.Errorf("reg %s %s: %w\n%s", action, key, err, out)
	}

	return nil
}

// nonAdminDir returns a fresh folder under nonAdminRoot for one test.
func nonAdminDir(t *testing.T, name string) string {
	t.Helper()

	dir := filepath.Join(nonAdminRoot, name)
	_ = os.RemoveAll(dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	return dir
}

const nonAdminScopeConfig = `schema: msi-builder/v1
product: { name: MsiBuilderNonAdmin, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 8e1c5a7d-3f2b-4d6e-9a0c-1b4d7e2f5a38 }
package: { platform: x64, scope: %s }
components:
  - files: [{ source: app.exe }]
`

// TestNonAdminInstallScopes installs packages of each scope silently as a
// standard user (INST-02 on the CI runner): perUser and perUserOrMachine
// install into the user's profile without elevation, and perMachine and
// perMachineOrUser fail with 1603 after error 1925, as on SERVER2025.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestNonAdminInstallScopes(t *testing.T) {
	user := standardUser(t)
	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	for _, tc := range []struct {
		scope string
		code  int
	}{
		{"perUser", 0},
		{"perUserOrMachine", 0},
		{"perMachineOrUser", 1603},
		{"perMachine", 1603},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			dir := nonAdminDir(t, "scope-"+tc.scope)
			path := buildConfig(t, dir, "scope", fmt.Sprintf(nonAdminScopeConfig, tc.scope), map[string][]byte{"app.exe": app})
			log := filepath.Join(dir, "install.log")

			t.Cleanup(func() { _ = user.msiexec(t, filepath.Join(dir, "cleanup.log"), "/x", path) })

			code := user.msiexec(t, log, "/i", path)
			props := serverProperties(readLog(log))

			t.Logf("INST-02 %s as %s: exit %d, INSTALLDIR %q, ALLUSERS %q, MSIINSTALLPERUSER %q, Privileged %q",
				tc.scope, user.name, code, props["INSTALLDIR"], props["ALLUSERS"], props["MSIINSTALLPERUSER"], props["Privileged"])

			if code != tc.code {
				t.Fatalf("msiexec /i returned %d, want %d\n%s", code, tc.code, readLog(log))
			}

			if code != 0 {
				if !strings.Contains(readLog(log), "Error 1925.") {
					t.Errorf("the log has no error 1925\n%s", readLog(log))
				}

				return
			}

			installed := filepath.Join(props["INSTALLDIR"], "app.exe")
			if !strings.Contains(strings.ToLower(installed), strings.ToLower(`\Users\`+user.name+`\`)) {
				t.Errorf("installed to %s, not into the profile of %s", installed, user.name)
			}

			if _, err := os.Stat(installed); err != nil {
				t.Errorf("app.exe after the install: %v", err)
			}

			if code := user.msiexec(t, filepath.Join(dir, "uninstall.log"), "/x", path); code != 0 {
				t.Errorf("msiexec /x returned %d", code)
			}

			if _, err := os.Stat(installed); err == nil {
				t.Errorf("%s is still there after the uninstall", installed)
			}
		})
	}
}

const secureConfig = `schema: msi-builder/v1
product: { name: MsiBuilderSecure, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 1d5f9b3e-7a2c-4e8d-b6f0-3c7e1a5d9b42 }
package: { platform: x64 }
installDir: { name: MsiBuilderSecure }
properties:
  PUB_SECURE: { value: default }
  PUB_PLAIN: { value: default, secure: false }
components:
  - files: [{ source: app.exe }]
customActions:
  - id: EchoProps
    run:
      command: '"[%ComSpec]" /c echo PROP-01 SECURE=[PUB_SECURE] PLAIN=[PUB_PLAIN] Privileged=[Privileged]'
    execute: deferred
    impersonate: false
    return: check
    schedule:
      - { sequence: execute, after: InstallFiles, condition: NOT REMOVE~="ALL" }
`

// TestNonAdminSecureProperties installs a per-machine package with
// AlwaysInstallElevated as a standard user, the elevated case in which only
// secure public properties reach the execute sequence (PROP-01), and the
// same package as the administrator for control. A deferred run action
// echoes both properties into the log.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestNonAdminSecureProperties(t *testing.T) {
	user := standardUser(t)
	dir := nonAdminDir(t, "secure")
	path := buildConfig(t, dir, "secure", secureConfig, map[string][]byte{
		"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
	})

	const policy = `HKLM\SOFTWARE\Policies\Microsoft\Windows\Installer`

	ctx := context.Background()

	if out, err := exec.CommandContext(ctx, "reg.exe", "add", policy, "/v", "AlwaysInstallElevated", "/t", "REG_DWORD", "/d", "1", "/f").CombinedOutput(); err != nil {
		t.Fatalf("reg add: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		_ = user.policy(t, "delete", "/v", "AlwaysInstallElevated", "/f")
		_ = exec.CommandContext(ctx, "reg.exe", "delete", policy, "/v", "AlwaysInstallElevated", "/f").Run()
		_ = exec.CommandContext(ctx, "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderSecure"))
	})

	if err := user.policy(t, "add", "/v", "AlwaysInstallElevated", "/t", "REG_DWORD", "/d", "1", "/f"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		admin bool
		want  string
	}{
		{"administrator", true, "SECURE=cmd PLAIN=cmd"},
		{"standard user elevated by policy", false, "SECURE=cmd PLAIN=default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-")+".log")
			args := []string{"/i", path, "PUB_SECURE=cmd", "PUB_PLAIN=cmd"}

			var code int
			if tc.admin {
				code = msiexecCode(t, log, args...)
			} else {
				code = user.msiexec(t, log, args...)
			}

			var echoed string

			for line := range strings.Lines(readLog(log)) {
				if _, rest, ok := strings.Cut(line, "BuilderRun: PROP-01 "); ok {
					echoed = strings.TrimSpace(rest)
				}
			}

			t.Logf("PROP-01 %s: exit %d, %s", tc.name, code, echoed)

			if code != 0 {
				t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
			}

			if !strings.HasPrefix(echoed, tc.want) {
				t.Errorf("the deferred action saw %q, want %q", echoed, tc.want)
			}

			if code := msiexecCode(t, filepath.Join(dir, "x.log"), "/x", path); code != 0 {
				t.Errorf("msiexec /x returned %d", code)
			}
		})
	}
}
