package nativetest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// exeProbeSource is the program the exe actions of TestMsiexecEXEActions
// run: it appends "<tag>|<its own path>|<user>|<working folder>" to the file
// in its first argument and exits with the code in its third.
const exeProbeSource = `package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

func main() {
	code, _ := strconv.Atoi(os.Args[3])
	self, _ := os.Executable()
	who := "?"
	if u, err := user.Current(); err == nil {
		who = u.Username
	}
	cwd, _ := os.Getwd()
	_ = os.MkdirAll(filepath.Dir(os.Args[1]), 0o755)
	f, err := os.OpenFile(os.Args[1], os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err == nil {
		fmt.Fprintf(f, "%s|%s|%s|%s\n", os.Args[2], self, who, cwd)
		f.Close()
	}
	os.Exit(code)
}
`

// exeProbeConfig runs the probe from the Binary table in every way CA-05
// looks at. MARKER is given on the command line.
const exeProbeConfig = `schema: msi-builder/v1
product: { name: MsiBuilder EXE Probe, manufacturer: MsiBuilderTest, version: 1.0.0, upgradeCode: 4b8e2c6a-7d13-4f0e-9a52-c1e6b3d8f27a }
package: { platform: x64 }
binaries: { ExeProbe: probe.exe }
components:
  - files: [{ source: readme.txt }]
customActions:
  # Before InstallFiles: nothing of the product is on disk yet.
  - id: ExeBeforeFiles
    exe: { binary: ExeProbe, arguments: '"[MARKER]" deferred-system 0' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, before: InstallFiles, when: install }]
  - id: ExeImmediate
    exe: { binary: ExeProbe, arguments: '"[MARKER]" immediate 0' }
    return: check
    schedule: [{ sequence: execute, after: CostFinalize, condition: NOT Installed }]
  - id: ExeDeferredUser
    exe: { binary: ExeProbe, arguments: '"[MARKER]" deferred-user 0' }
    execute: deferred
    return: check
    schedule: [{ sequence: execute, after: InstallFiles, when: install }]
  - id: ExeIgnored
    exe: { binary: ExeProbe, arguments: '"[MARKER]" ignored-5 5' }
    execute: deferred
    impersonate: false
    return: ignore
    schedule: [{ sequence: execute, after: InstallFiles, when: install }]
  - id: ExeReboot
    exe: { binary: ExeProbe, arguments: '"[MARKER]" exit-3010 3010' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, after: InstallFiles, condition: NOT Installed AND EXITCODE="3010" }]
  - id: ExeFails
    exe: { binary: ExeProbe, arguments: '"[MARKER]" exit-7 7' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, after: InstallFiles, condition: NOT Installed AND EXITCODE="7" }]
  # After RemoveFiles: the product's files are gone, the probe is not.
  - id: ExeAfterRemove
    exe: { binary: ExeProbe, arguments: '"[MARKER]" uninstall 0' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, after: RemoveFiles, when: uninstall }]
`

// exeProbe builds exeProbeSource for x64.
func exeProbe(t *testing.T) []byte {
	t.Helper()

	dir := t.TempDir()

	for name, data := range map[string]string{"main.go": exeProbeSource, "go.mod": "module exeprobe\n\ngo 1.22\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	exe := filepath.Join(dir, "probe.exe")

	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", exe, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off", "GOARCH=amd64") //nolint:wsl_v5 // configures cmd

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the probe: %v\n%s", err, out)
	}

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// probeLine is one line the probe wrote.
type probeLine struct {
	tag, self, user, cwd string
}

func readProbe(t *testing.T, marker string) map[string]probeLine {
	t.Helper()

	data, err := os.ReadFile(marker)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	lines := make(map[string]probeLine)

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.Split(strings.TrimSpace(line), "|")
		if len(parts) == 4 {
			lines[parts[0]] = probeLine{parts[0], parts[1], parts[2], parts[3]}
		}
	}

	return lines
}

// TestMsiexecEXEActions installs exe actions (CA-05): a program stored in
// the Binary table, run deferred as LocalSystem before InstallFiles and
// after RemoveFiles, deferred as the user, immediate, with its exit code
// ignored, failing, and returning 3010. Each run records where the engine
// put the program, which must be gone afterwards.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecEXEActions(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	dir := t.TempDir()

	for name, data := range map[string][]byte{"probe.exe": exeProbe(t), "readme.txt": []byte("probe")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(exeProbeConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "exe-probe.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	// SYSTEM writes the marker too, so it lives outside the user profile.
	markerDir := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderExeProbe")
	marker := filepath.Join(markerDir, "marker.txt")

	_ = os.RemoveAll(markerDir)

	t.Cleanup(func() {
		_ = exec.Command("msiexec", "/x", path, "/qn", "/norestart").Run()
		_ = os.RemoveAll(markerDir)
	})

	installLog := filepath.Join(dir, "install.log")
	if code := msiexecCode(t, installLog, "/i", path, "MARKER="+marker); code != 0 {
		t.Fatalf("install: exit %d\n%s", code, readLog(installLog))
	}

	got := readProbe(t, marker)
	t.Logf("install: %+v", got)

	for tag, system := range map[string]bool{"deferred-system": true, "immediate": false, "deferred-user": false, "ignored-5": true} {
		line, ok := got[tag]
		if !ok {
			t.Errorf("%s did not run", tag)

			continue
		}

		if isSystem := strings.EqualFold(line.user, `NT AUTHORITY\SYSTEM`); isSystem != system {
			t.Errorf("%s ran as %s", tag, line.user)
		}

		if _, err := os.Stat(line.self); err == nil {
			t.Errorf("%s: the engine left %s behind", tag, line.self)
		}
	}

	uninstallLog := filepath.Join(dir, "uninstall.log")
	if code := msiexecCode(t, uninstallLog, "/x", path, "MARKER="+marker); code != 0 {
		t.Fatalf("uninstall: exit %d\n%s", code, readLog(uninstallLog))
	}

	if line, ok := readProbe(t, marker)["uninstall"]; !ok {
		t.Error("the action after RemoveFiles did not run on uninstall")
	} else if _, err := os.Stat(line.self); err == nil {
		t.Errorf("uninstall: the engine left %s behind", line.self)
	}

	for _, tc := range []struct {
		exit string
		want int
	}{{"7", 1603}, {"3010", 1603}} {
		log := filepath.Join(dir, "exit-"+tc.exit+".log")
		code := msiexecCode(t, log, "/i", path, "MARKER="+marker, "EXITCODE="+tc.exit)
		t.Logf("exit code %s: msiexec returned %d", tc.exit, code)

		if code != tc.want {
			t.Errorf("an exe action exiting with %s: msiexec returned %d, want %d\n%s", tc.exit, code, tc.want, readLog(log))
		}

		for _, line := range strings.Split(readLog(log), "\n") {
			if strings.Contains(line, "ExeFails") || strings.Contains(line, "ExeReboot") {
				t.Logf("log: %s", strings.TrimSpace(line))
			}
		}
	}

	if t.Failed() {
		t.Logf("install log:\n%s", readLog(installLog))
	}
}
