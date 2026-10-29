//go:build windows

package nativetest_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// serviceConfig is a package with one service, MsiBuilderCtlSvc, whose
// control settings are the format argument.
const serviceConfig = `schema: msi-builder/v1
product: { name: MsiBuilderCtlSvc, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 9d2c4e61-3b7a-4f08-a5c1-7e9b0d3f2a48 }
package: { platform: x64 }
installDir: { name: MsiBuilderCtlSvc }
components:
  - files: [{ source: svc.exe }]
    services:
      - { name: MsiBuilderCtlSvc, start: demand, control: %s }
`

const ctlService = "MsiBuilderCtlSvc"

// createService registers a service with sc.exe and removes it when the
// test ends.
func createService(t *testing.T, name, binPath string, start bool) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = exec.CommandContext(context.Background(), "sc", "stop", name).CombinedOutput()
		_, _ = exec.CommandContext(context.Background(), "sc", "delete", name).CombinedOutput()
	})

	//nolint:gosec // the arguments are test-controlled names and paths
	out, err := exec.CommandContext(t.Context(), "sc", "create", name, "binPath=", binPath, "start=", "demand", "DisplayName=", name+" stale").CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Access is denied") {
			t.Skip("not elevated: sc create was denied")
		}

		t.Fatalf("sc create %s: %v\n%s", name, err, out)
	}

	if start {
		if out, err := exec.CommandContext(t.Context(), "sc", "start", name).CombinedOutput(); err != nil {
			t.Fatalf("sc start %s: %v\n%s", name, err, out)
		}
	}
}

// serviceBinary returns the BINARY_PATH_NAME of sc qc.
func serviceBinary(t *testing.T, name string) string {
	t.Helper()

	out, err := exec.CommandContext(t.Context(), "sc", "qc", name).CombinedOutput()
	if err != nil {
		t.Fatalf("sc qc %s: %v\n%s", name, err, out)
	}

	for line := range strings.SplitSeq(string(out), "\n") {
		if _, value, ok := strings.Cut(line, "BINARY_PATH_NAME"); ok {
			_, value, _ = strings.Cut(value, ":")

			return strings.TrimSpace(value)
		}
	}

	t.Fatalf("sc qc %s has no BINARY_PATH_NAME line:\n%s", name, out)

	return ""
}

// uninstallOnCleanup removes the package, the service and the install
// folder when the test ends.
func uninstallOnCleanup(t *testing.T, path, service, installDir string) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").CombinedOutput()
		_, _ = exec.CommandContext(context.Background(), "sc", "stop", service).CombinedOutput()
		_, _ = exec.CommandContext(context.Background(), "sc", "delete", service).CombinedOutput()
		_ = os.RemoveAll(installDir)
	})
}

// TestMsiexecDeletesServiceOnInstall installs a service whose name is
// already registered by something else (SVC-06). Without deleteOnInstall
// InstallServices leaves the registered service alone, so the package's
// service runs the old binary; with it, DeleteServices removes the old
// registration first and InstallServices creates the package's own.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecDeletesServiceOnInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	installDir := filepath.Join(os.Getenv("ProgramFiles"), ctlService)
	stale := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	svc := testService(t)

	t.Run("without deleteOnInstall", func(t *testing.T) {
		dir := t.TempDir()
		path := buildConfig(t, dir, "keep", fmt.Sprintf(serviceConfig, "{ startOnInstall: false }"), map[string][]byte{"svc.exe": svc})

		createService(t, ctlService, stale, false)
		uninstallOnCleanup(t, path, ctlService, installDir)

		code := msiexecCode(t, filepath.Join(dir, "install.log"), "/i", path)
		t.Logf("install exit code %d", code)

		if code != 0 {
			t.Fatalf("install failed with %d\n%s", code, readLog(filepath.Join(dir, "install.log")))
		}

		if got := serviceBinary(t, ctlService); !strings.Contains(strings.ToLower(got), "notepad.exe") {
			t.Errorf("binary path %q, want the stale registration left alone", got)
		}

		msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

		if state := serviceState(t, ctlService); state != "" {
			t.Errorf("service still registered after uninstall: %q", state)
		}
	})

	t.Run("with deleteOnInstall", func(t *testing.T) {
		dir := t.TempDir()
		path := buildConfig(t, dir, "replace", fmt.Sprintf(serviceConfig, "{ deleteOnInstall: true }"), map[string][]byte{"svc.exe": svc})

		createService(t, ctlService, stale, false)
		uninstallOnCleanup(t, path, ctlService, installDir)

		msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

		want := strings.ToLower(filepath.Join(installDir, "svc.exe"))
		if got := serviceBinary(t, ctlService); !strings.Contains(strings.ToLower(got), want) {
			t.Errorf("binary path %q, want %q", got, want)
		}

		if state := serviceState(t, ctlService); !strings.Contains(state, "RUNNING") {
			t.Errorf("service state %q, want RUNNING", state)
		}

		// A repair reinstalls the component, so event 8 deletes the
		// running service and InstallServices creates it again.
		msiexecQuiet(t, filepath.Join(dir, "repair.log"), "/fa", path)

		if state := serviceState(t, ctlService); !strings.Contains(state, "RUNNING") {
			t.Errorf("service state after repair %q, want RUNNING", state)
		}

		msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

		if state := serviceState(t, ctlService); state != "" {
			t.Errorf("service still registered after uninstall: %q", state)
		}
	})
}

const foreignConfig = `schema: msi-builder/v1
product: { name: MsiBuilderForeignSvc, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 5e1f8a27-c94d-4b3e-8d60-2a7c9f1b4e85 }
package: { platform: x64 }
installDir: { name: MsiBuilderForeignSvc }
components:
  - files: [{ source: app.exe }]
    serviceControl:
      - { name: MsiBuilderOldSvc, stopOnInstall: true, deleteOnInstall: true, stopOnUninstall: true, deleteOnUninstall: true }
      - { name: MsiBuilderOtherSvc, startOnInstall: true, stopOnUninstall: true }
      - { name: MsiBuilderMissingSvc, stopOnInstall: true, deleteOnInstall: true, stopOnUninstall: true, deleteOnUninstall: true }
`

// buildForeign builds the package that controls services it does not
// install.
func buildForeign(t *testing.T, dir string) string {
	t.Helper()

	app := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	return buildConfig(t, dir, "foreign", foreignConfig, map[string][]byte{"app.exe": app})
}

// TestMsiexecControlsForeignServices installs a package with ServiceControl
// rows for services it does not install (SVC-07): a running old service is
// stopped and deleted, a stopped one is started at install and stopped at
// uninstall, and a service that does not exist does not fail either.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecControlsForeignServices(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "svc.exe")

	if err := os.WriteFile(exe, testService(t), 0o600); err != nil {
		t.Fatal(err)
	}

	path := buildForeign(t, dir)

	createService(t, "MsiBuilderOldSvc", exe, true)
	createService(t, "MsiBuilderOtherSvc", exe, false)
	uninstallOnCleanup(t, path, "MsiBuilderMissingSvc", filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderForeignSvc"))

	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	if state := serviceState(t, "MsiBuilderOldSvc"); state != "" {
		t.Errorf("old service still registered after install: %q", state)
	}

	if state := serviceState(t, "MsiBuilderOtherSvc"); !strings.Contains(state, "RUNNING") {
		t.Errorf("other service state %q after install, want RUNNING", state)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)

	if state := serviceState(t, "MsiBuilderOtherSvc"); !strings.Contains(state, "STOPPED") {
		t.Errorf("other service state %q after uninstall, want STOPPED", state)
	}
}

const conditionalStartConfig = `schema: msi-builder/v1
product: { name: MsiBuilderCondSvc, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 2f6b8d13-a7c4-4e59-b1d0-8c3e5a9f7b26 }
package: { platform: x64 }
installDir: { name: MsiBuilderCondSvc }
components:
  - files: [{ source: svc.exe }]
    services:
      - { name: MsiBuilderCtlSvc, start: demand, control: { startCondition: START_AFTER_INSTALL="1" } }
`

// TestMsiexecStartsServiceOnCondition installs a service whose start has a
// condition (SVC-08): the start row sits in a generated component with the
// condition, so the service only starts when the property is set.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecStartsServiceOnCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderCondSvc")
	svc := testService(t)
	dir := t.TempDir()
	path := buildConfig(t, dir, "conditional", conditionalStartConfig, map[string][]byte{"svc.exe": svc})

	uninstallOnCleanup(t, path, ctlService, installDir)

	msiexecQuiet(t, filepath.Join(dir, "install.log"), "/i", path)

	if state := serviceState(t, ctlService); !strings.Contains(state, "STOPPED") {
		t.Errorf("service state %q without the property, want STOPPED", state)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall.log"), "/x", path)
	msiexecQuiet(t, filepath.Join(dir, "install-start.log"), "/i", path, "START_AFTER_INSTALL=1")

	if state := serviceState(t, ctlService); !strings.Contains(state, "RUNNING") {
		t.Errorf("service state %q with the property, want RUNNING", state)
	}

	msiexecQuiet(t, filepath.Join(dir, "uninstall-start.log"), "/x", path)

	if state := serviceState(t, ctlService); state != "" {
		t.Errorf("service still registered after uninstall: %q", state)
	}
}

// TestICEServiceControl runs the full ICE suite against the packages with
// foreign services and a conditional start.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEServiceControl(t *testing.T) {
	cub := iceCube(t)
	dir := t.TempDir()

	requireCleanICE(t, buildForeign(t, dir), cub)
	requireCleanICE(t, buildConfig(t, dir, "conditional", conditionalStartConfig, map[string][]byte{"svc.exe": testService(t)}), cub)
}
