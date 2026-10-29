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
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// shortcutTargetsConfig has a folder, a program and a URL shortcut in the
// Start menu, in the default feature, and a desktop shortcut to the
// program's file in a feature that is off by default.
const shortcutTargetsConfig = `schema: msi-builder/v1
product: { name: MsiBuilderShortcutTargets, manufacturer: msi-builder tests, version: 1.2.3, upgradeCode: 6a0c2f4e-8b1d-4e7a-9c35-1f2e3d4c5b6a }
package: { platform: x64 }
icons: { app.ico: app.ico }
components:
  - id: App
    files: [{ source: app.exe }]
  - id: Menu
    shortcuts:
      - { name: Configuration, directory: ProgramMenuFolder/MsiBuilderShortcutTargets, folder: INSTALLDIR/config, description: Open the configuration folder }
      - { name: Uninstall, directory: ProgramMenuFolder/MsiBuilderShortcutTargets, program: "[SystemFolder]msiexec.exe", arguments: "/x [ProductCode]" }
      - { name: Documentation, directory: ProgramMenuFolder/MsiBuilderShortcutTargets, url: "https://example.com/docs?v=[ProductVersion]", iconFile: app.exe }
  - id: Desktop
    shortcuts:
      - { name: MsiBuilderShortcutTargets, directory: DesktopFolder, file: app.exe, icon: app.ico }
features:
  - { id: Main, components: [App, Menu] }
  - { id: DesktopShortcut, default: off, components: [Desktop] }
`

// buildShortcutTargets writes the shortcut target test package.
func buildShortcutTargets(t *testing.T) (string, *builder.Result) {
	t.Helper()

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

	cfg, err := config.Parse([]byte(shortcutTargetsConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "shortcuttargets.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, res
}

// shell runs a PowerShell script and returns its trimmed output.
func shell(t *testing.T, script string) string {
	t.Helper()

	out, err := powershell(script)
	if err != nil {
		t.Fatalf("powershell %s: %v\n%s", script, err, out)
	}

	return out
}

// lnkCommand reads the target and the arguments of a .lnk through the
// shell's own parser.
func lnkCommand(t *testing.T, path string) string {
	t.Helper()

	return shell(t, "$s = (New-Object -ComObject WScript.Shell).CreateShortcut('"+path+"'); "+`"$($s.TargetPath)|$($s.Arguments)|$($s.WorkingDirectory)"`)
}

// urlTarget reads the address of a .url file as Explorer does: through the
// shell's property handler for Internet shortcuts (System.Link.TargetUrl).
func urlTarget(t *testing.T, path string) string {
	t.Helper()

	return shell(t, "$f = (New-Object -ComObject Shell.Application).NameSpace('"+filepath.Dir(path)+"').ParseName('"+filepath.Base(path)+"'); "+
		"$f.ExtendedProperty('System.Link.TargetUrl')")
}

func present(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// TestMsiexecShortcutTargets installs folder, program, URL and file
// shortcuts that are listed under components, adds and removes the feature
// of the desktop shortcut, and uninstalls after the shell has written to
// the .url file (SHORTCUT-03, SHORTCUT-04).
func TestMsiexecShortcutTargets(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path, res := buildShortcutTargets(t)
	log := filepath.Join(filepath.Dir(path), "install.log")
	install := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderShortcutTargets")
	menu := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\MsiBuilderShortcutTargets`)
	desktop := filepath.Join(os.Getenv("PUBLIC"), "Desktop", "MsiBuilderShortcutTargets.lnk")
	url := filepath.Join(menu, "Documentation.url")

	cleanup := func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(install)
		_ = os.RemoveAll(menu)
		_ = os.Remove(desktop)
	}

	cleanup()
	t.Cleanup(cleanup)

	msiexecQuiet(t, log, "/i", path)

	data, err := os.ReadFile(url)
	if err != nil {
		t.Fatalf("the .url file: %v\n%s", err, readLog(log))
	}

	t.Logf("SHORTCUT-03: %s holds %q", url, data)

	if got, want := urlTarget(t, url), "https://example.com/docs?v=1.2.3"; got != want {
		t.Errorf("System.Link.TargetUrl %q, want %q", got, want)
	}

	for _, entry := range []string{"IconFile=" + filepath.Join(install, "app.exe"), "IconIndex=0"} {
		if !strings.Contains(strings.ToLower(string(data)), strings.ToLower(entry)) {
			t.Errorf("the .url has no %s", entry)
		}
	}

	configDir := filepath.Join(install, "config")
	if !present(configDir) {
		t.Errorf("%s was not created", configDir)
	}

	if got := lnkCommand(t, filepath.Join(menu, "Configuration.lnk")); !strings.EqualFold(got, configDir+"||") {
		t.Errorf("folder shortcut target|arguments|working folder %q, want %q", got, configDir+"||")
	}

	// SystemFolder is the 32-bit system folder on 64-bit Windows, also in
	// an x64 package.
	uninstall := filepath.Join(os.Getenv("SystemRoot"), "SysWOW64", "msiexec.exe") + "|/x " + res.ProductCode + "|"
	if got := lnkCommand(t, filepath.Join(menu, "Uninstall.lnk")); !strings.EqualFold(got, uninstall) {
		t.Errorf("program shortcut target|arguments|working folder %q, want %q", got, uninstall)
	}

	t.Logf("SHORTCUT-04: Configuration.lnk %q, Uninstall.lnk %q", lnkCommand(t, filepath.Join(menu, "Configuration.lnk")),
		lnkCommand(t, filepath.Join(menu, "Uninstall.lnk")))

	if present(desktop) {
		t.Errorf("%s exists although its feature is off by default", desktop)
	}

	msiexecQuiet(t, log, "/i", path, "ADDLOCAL=DesktopShortcut")

	app := filepath.Join(install, "app.exe") + "||" + install + `\`
	if got := lnkCommand(t, desktop); !strings.EqualFold(got, app) {
		t.Errorf("desktop shortcut target|arguments|working folder %q, want %q", got, app)
	}

	msiexecQuiet(t, log, "/i", path, "REMOVE=DesktopShortcut")

	if present(desktop) {
		t.Errorf("%s is still there after REMOVE=DesktopShortcut", desktop)
	}

	if !present(url) || !present(filepath.Join(install, "app.exe")) {
		t.Error("removing DesktopShortcut removed the other features' files")
	}

	// The shell adds its own section when a user changes a .url, which
	// RemoveIniValues does not remove; the RemoveFile row still does.
	shellSection := "[{000214A0-0000-0000-C000-000000000046}]\r\nProp3=19,11\r\n"
	if err := os.WriteFile(url, append(data, shellSection...), 0o600); err != nil {
		t.Fatal(err)
	}

	msiexecQuiet(t, log, "/x", path)

	for _, left := range []string{url, menu, configDir, install, desktop} {
		if present(left) {
			t.Errorf("%s is still there after the uninstall", left)
		}
	}
}

// TestICEShortcutTargets runs the full ICE suite against component
// shortcuts of every target kind, in their own feature.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEShortcutTargets(t *testing.T) {
	cub := iceCube(t)
	path, _ := buildShortcutTargets(t)

	requireCleanICE(t, path, cub)
}
