//go:build windows

package nativetest_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/internal/testpe"
)

func desktopInputs() map[string][]byte {
	return map[string][]byte{
		"viewer.exe":  testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1033}}),
		"LICENSE.txt": []byte("Example license\n"),
	}
}

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
// (RUN-28).
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
