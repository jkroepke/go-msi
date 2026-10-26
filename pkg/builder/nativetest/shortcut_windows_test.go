//go:build windows

package nativetest_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/internal/exampletest"
	"github.com/jkroepke/go-msi/internal/testpe"
	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/config"
)

func desktopInputs() map[string][]byte { return exampletest.Examples()["desktop"].Inputs }

// lnkTarget reads a shortcut through the shell's own parser.
func lnkTarget(t *testing.T, path string) string {
	t.Helper()

	script := "$s = (New-Object -ComObject WScript.Shell).CreateShortcut('" + path + "'); " +
		`"$($s.TargetPath)|$($s.WorkingDirectory)|$($s.Description)"`

	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).CombinedOutput()
	if err != nil {
		t.Fatalf("reading %s: %v\n%s", path, err, out)
	}

	return strings.TrimSpace(string(out))
}

// TestMsiexecInstallsShortcuts installs the desktop example, checks its
// Start menu and desktop shortcuts and that the uninstall removes them
// (SHORTCUT-01).
func TestMsiexecInstallsShortcuts(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path, _ := buildExample(t, "desktop", map[string]string{"VERSION": "2.0.0"}, desktopInputs())
	log := filepath.Join(filepath.Dir(path), "install.log")

	out, err := exec.Command("msiexec", "/i", path, "/qn", "/l*v", log).CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1925 {
			t.Skip("not elevated: msiexec returned 1925")
		}

		t.Fatalf("msiexec /i: %v\n%s\n%s", err, out, readLog(log))
	}

	t.Cleanup(func() {
		// Leave no product behind when a check below fails.
		_ = exec.Command("msiexec", "/x", path, "/qn").Run()
	})

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "Example Viewer")
	links := []string{
		filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs`, "Example Viewer.lnk"),
		filepath.Join(os.Getenv("PUBLIC"), "Desktop", "Example Viewer.lnk"),
	}

	for i, link := range links {
		got := lnkTarget(t, link)
		want := filepath.Join(installDir, "viewer.exe") + "|" + installDir + `\|`

		if i == 0 {
			want += "View example files"
		}

		if !strings.EqualFold(got, want) {
			t.Errorf("%s: target|working folder|description %q, want %q", link, got, want)
		}
	}

	out, err = exec.Command("msiexec", "/x", path, "/qn", "/l*v", log).CombinedOutput()
	if err != nil {
		t.Fatalf("msiexec /x: %v\n%s\n%s", err, out, readLog(log))
	}

	for _, link := range links {
		if _, err := os.Stat(link); err == nil {
			t.Errorf("%s is still there after the uninstall", link)
		}
	}
}

// TestICEShortcuts runs the full ICE suite against shortcuts in a Start menu
// subfolder, which need RemoveFile rows (ICE64), and on the desktop, with an
// icon. A per-user package is left out: its install dir lies in the user
// profile, and the file components there still fail ICE38 (see the
// validator's warning), whatever the shortcuts do.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEShortcuts(t *testing.T) {
	cub := iceCube(t)

	for _, scope := range []string{"perMachine"} {
		t.Run(scope, func(t *testing.T) {
			dir := t.TempDir()
			inputs := map[string][]byte{
				"app.exe": testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
				"app.ico": append([]byte{0, 0, 1, 0, 1, 0}, make([]byte, 64)...),
			}

			for name, data := range inputs {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := config.Parse([]byte(`schema: go-msi/v1
product: { name: Acme App, manufacturer: Acme, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3303 }
package: { platform: x64, scope: `+scope+` }
icons: { app.ico: app.ico }
components:
  - files:
      - source: app.exe
        shortcuts:
          - { directory: ProgramMenuFolder/Acme/Tools, icon: app.ico, arguments: --tools }
          - { directory: ProgramMenuFolder/Acme }
          - { directory: DesktopFolder }
`), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.BaseDir = dir

			res, err := builder.Build(cfg, builder.Options{})
			if err != nil {
				t.Fatalf("build: %v\n%s", err, res.Diagnostics)
			}

			path := filepath.Join(dir, "shortcuts.msi")
			if err := os.WriteFile(path, res.Package, 0o600); err != nil {
				t.Fatal(err)
			}

			requireCleanICE(t, path, cub)
		})
	}
}
