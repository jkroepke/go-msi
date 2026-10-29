//go:build windows

package nativetest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/pkg/config"
)

const rebootConfig = `schema: msi-builder/v1
product: { name: MsiBuilderReboot, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 0c7a4e2b-9d15-4f3a-8b6e-5a2d1c9f7e43 }
package: { platform: x64 }
components:
  - files: [{ source: a.txt }]
scheduleReboot: { when: install }
`

// buildReboot writes the ScheduleReboot test package.
func buildReboot(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(rebootConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "reboot.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestMsiexecScheduleReboot installs a package with scheduleReboot under
// /norestart: msiexec returns 3010 (ERROR_SUCCESS_REBOOT_REQUIRED) without
// restarting, and the uninstall, which the when excludes, returns 0.
func TestMsiexecScheduleReboot(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path := buildReboot(t)

	t.Cleanup(func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn", "/norestart").Run()
	})

	log := filepath.Join(filepath.Dir(path), "install.log")
	if code := msiexecCode(t, log, "/i", path); code != 3010 {
		t.Fatalf("msiexec /i returned %d, want 3010\n%s", code, readLog(log))
	}

	if text := readLog(log); !strings.Contains(text, "ScheduleReboot") {
		t.Error("the install log does not mention ScheduleReboot")
	}

	log = filepath.Join(filepath.Dir(path), "uninstall.log")
	if code := msiexecCode(t, log, "/x", path); code != 0 {
		t.Fatalf("msiexec /x returned %d, want 0\n%s", code, readLog(log))
	}
}
