//go:build windows

package nativetest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/ice"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/testpe"
)

// contextActionsConfig is a dual package with three deferred run actions
// that echo a tag and the account they run as: one without impersonation
// that runs only per machine, one that runs only per user, and one in both
// contexts (CA-09). They have no condition, so they run on install, repair
// and uninstall.
const contextActionsConfig = `schema: msi-builder/v1
product: { name: MsiBuilderContextActions, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 5b8e2d14-6c3a-4f97-a1d0-7e4c9b2f8a63 }
package: { platform: x64, scope: ${SCOPE} }
components:
  - files: [{ source: app.exe }]
customActions:
  - id: CA09Machine
    run: { command: '"[System64Folder]cmd.exe" /c echo ca09-machine & whoami' }
    execute: deferred
    impersonate: false
    scope: perMachine
    return: check
    schedule: [{ sequence: execute, after: InstallFiles }]
  - id: CA09User
    run: { command: '"[System64Folder]cmd.exe" /c echo ca09-user & whoami' }
    execute: deferred
    scope: perUser
    return: check
    schedule: [{ sequence: execute, after: InstallFiles }]
  - id: CA09Both
    run: { command: '"[System64Folder]cmd.exe" /c echo ca09-both & whoami' }
    execute: deferred
    return: check
    schedule: [{ sequence: execute, after: InstallFiles }]
`

// contextActions is the account each action of contextActionsConfig ran
// as, lower case, or "" when it did not run.
type contextActions struct {
	machine, user, both string
}

// ranAs returns the account a CA-09 action ran as: the BuilderRun line
// after its tag, or "" when the log has no tag.
func ranAs(log, tag string) string {
	lines := strings.Split(log, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "BuilderRun: "+tag) || i+1 == len(lines) {
			continue
		}

		_, account, ok := strings.Cut(lines[i+1], "BuilderRun: ")
		if !ok {
			return "?"
		}

		return strings.ToLower(strings.TrimSpace(account))
	}

	return ""
}

func readContextActions(log string) contextActions {
	return contextActions{machine: ranAs(log, "ca09-machine"), user: ranAs(log, "ca09-user"), both: ranAs(log, "ca09-both")}
}

// buildContextActions builds the CA-09 package for a scope. unconditioned
// drops the ALLUSERS=1 the builder adds to the perMachine action and its
// data action, which the validator would refuse, to see what Windows does
// with an elevated action in a per-user install.
func buildContextActions(t *testing.T, dir, scope string, unconditioned bool) string {
	t.Helper()

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	doc := strings.ReplaceAll(contextActionsConfig, "${SCOPE}", scope)
	path, res := buildConfigResult(t, dir, "ca09-"+scope, doc, map[string][]byte{"app.exe": app})

	if !unconditioned {
		return path
	}

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	changed := 0

	for _, row := range database.Table("InstallExecuteSequence").Rows {
		if row[0] == "CA09Machine" || row[0] == "CA09Machine.Data" {
			if row[1] != "ALLUSERS=1" {
				t.Fatalf("%v: condition %v, want ALLUSERS=1", row[0], row[1])
			}

			row[1] = nil
			changed++
		}
	}

	if changed != 2 {
		t.Fatalf("changed %d sequence rows, want 2", changed)
	}

	data, err := database.Marshal(msidb.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestMsiexecContextActions installs the CA-09 package silently in both
// contexts as the administrator, repairs and uninstalls it: the perMachine
// action runs as LocalSystem per machine only, the perUser action per user
// only, and the unscoped one in both. The per-user installs run elevated,
// so they also show that the condition keeps the elevated action out of an
// administrator's per-user install. An unconditioned copy shows what that
// condition prevents.
func TestMsiexecContextActions(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	for _, tc := range []struct { //nolint:paralleltest // msiexec runs one installation at a time
		name, scope, context string
		args                 []string
	}{
		{"perUserOrMachine", "perUserOrMachine", "user", nil},
		{"perUserOrMachine to machine", "perUserOrMachine", "machine", []string{`MSIINSTALLPERUSER=""`}},
		{"perMachineOrUser", "perMachineOrUser", "machine", nil},
		{"perMachineOrUser to user", "perMachineOrUser", "user", []string{"MSIINSTALLPERUSER=1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := buildContextActions(t, dir, tc.scope, false)
			t.Cleanup(func() { _, _ = runMsiexec([]string{"/x", path, "/qn"}, nil) })

			for _, step := range []struct {
				name string
				args []string
			}{
				{"install", append([]string{"/i", path}, tc.args...)},
				{"repair", []string{"/i", path, "REINSTALL=ALL", "REINSTALLMODE=amus"}},
				{"uninstall", []string{"/x", path}},
			} {
				log := filepath.Join(dir, step.name+".log")
				msiexecQuiet(t, log, step.args...)

				text := readLog(log)
				got := readContextActions(text)
				t.Logf("CA-09 %s %s: ALLUSERS %q, ran as %+v", tc.name, step.name, serverProperties(text)["ALLUSERS"], got)

				if got.both == "" || got.both == "?" {
					t.Errorf("%s: the unscoped action did not run (%q)", step.name, got.both)
				}

				switch tc.context {
				case "machine":
					if got.machine != `nt authority\system` || got.user != "" {
						t.Errorf("%s per machine: perMachine action as %q, perUser action as %q; want LocalSystem and not run",
							step.name, got.machine, got.user)
					}
				default:
					if got.machine != "" || got.user == "" || got.user == "?" {
						t.Errorf("%s per user: perMachine action as %q, perUser action as %q; want not run and run",
							step.name, got.machine, got.user)
					}
				}
			}
		})
	}

	t.Run("unconditioned per user", func(t *testing.T) {
		dir := t.TempDir()
		path := buildContextActions(t, dir, "perUserOrMachine", true)
		t.Cleanup(func() { _, _ = runMsiexec([]string{"/x", path, "/qn"}, nil) })

		log := filepath.Join(dir, "install.log")
		msiexecQuiet(t, log, "/i", path)

		text := readLog(log)
		t.Logf("CA-09 unconditioned elevated action, administrator's per-user install: ALLUSERS %q, ran as %+v",
			serverProperties(text)["ALLUSERS"], readContextActions(text))
	})
}

// TestNonAdminContextActions installs the CA-09 package per user as a
// standard user: the install needs no elevation, the perMachine action
// does not run and the others run as the user. The unconditioned copy
// shows what an elevated action does in a standard user's per-user
// install.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestNonAdminContextActions(t *testing.T) {
	user := standardUser(t)

	for _, tc := range []struct {
		name, scope   string
		unconditioned bool
		args          []string
	}{
		{"perUserOrMachine", "perUserOrMachine", false, nil},
		{"perMachineOrUser to user", "perMachineOrUser", false, []string{"MSIINSTALLPERUSER=1"}},
		{"unconditioned", "perUserOrMachine", true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := nonAdminDir(t, "ca09-"+strings.ReplaceAll(tc.name, " ", "-"))
			path := buildContextActions(t, dir, tc.scope, tc.unconditioned)

			t.Cleanup(func() { _ = user.msiexec(t, filepath.Join(dir, "cleanup.log"), "/x", path) })

			log := filepath.Join(dir, "install.log")
			code := user.msiexec(t, log, append([]string{"/i", path}, tc.args...)...)
			text := readLog(log)
			got := readContextActions(text)

			t.Logf("CA-09 %s as %s: exit %d, ALLUSERS %q, ran as %+v", tc.name, user.name, code, serverProperties(text)["ALLUSERS"], got)

			if tc.unconditioned {
				return
			}

			if code != 0 {
				t.Fatalf("msiexec /i returned %d\n%s", code, text)
			}

			account := strings.ToLower(user.name)
			if got.machine != "" || !strings.HasSuffix(got.user, `\`+account) || !strings.HasSuffix(got.both, `\`+account) {
				t.Errorf("ran as %+v; want the perMachine action not run and the others as %s", got, user.name)
			}

			if code := user.msiexec(t, filepath.Join(dir, "uninstall.log"), "/x", path); code != 0 {
				t.Errorf("msiexec /x returned %d", code)
			}
		})
	}
}

// TestICEContextActions runs the full ICE suite, ICE105 included, against
// the CA-09 package of both dual scopes and logs what ICE105 reports about
// the elevated action that runs only per machine. The validator allows it
// on the strength of this test.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEContextActions(t *testing.T) {
	cub := iceCube(t)

	for _, scope := range []string{"perUserOrMachine", "perMachineOrUser"} {
		for _, unconditioned := range []bool{false, true} {
			path := buildContextActions(t, t.TempDir(), scope, unconditioned)

			findings, err := ice.Run(path, cub)
			if err != nil {
				t.Fatal(err)
			}

			for _, finding := range findings {
				if finding.ICE != "ICE105" || finding.Type == ice.Info {
					continue
				}

				t.Logf("CA-09 %s unconditioned=%v: %s", scope, unconditioned, finding)

				if !unconditioned && (finding.Type == ice.Error || finding.Type == ice.Failure) {
					t.Errorf("%s: ICE105 error for a perMachine action: %s", scope, finding)
				}
			}

			if !unconditioned {
				requireCleanICE(t, path, cub)
			}
		}
	}
}
