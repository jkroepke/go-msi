//go:build windows

package nativetest_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

const removeFilesConfig = `schema: msi-builder/v1
product: { name: MsiBuilderRemoveFiles, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 2d6f1b8a-5c3e-4f70-9a1d-7e4b2c9f0a65 }
package: { platform: x64 }
directories:
  - id: ProgramFiles64Folder
    children:
      - id: INSTALLDIR
        name: MsiBuilderRemoveFiles
        children: [{ id: LOGS, name: Logs }]
  - id: ProgramMenuFolder
    children: [{ id: MENUDIR, name: MsiBuilderRemoveFiles, children: [{ id: MENUSUB, name: Tools }] }]
components:
  - id: App
    files: [{ source: app.exe }]
    removeFiles:
      - { name: stale-config.txt, on: install }
      - { name: '*.log', directory: LOGS }
      - { directory: LOGS }
  - id: Menu
    directory: MENUSUB
    createFolder: true
    registry: [{ root: HKCU, key: 'SOFTWARE\MsiBuilderTest\RemoveFiles', name: menu, type: dword, value: 1 }]
`

// buildRemoveFiles writes the RemoveFile test package.
func buildRemoveFiles(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	if err := os.WriteFile(filepath.Join(dir, "app.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(removeFilesConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "removefiles.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func writeFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestMsiexecRemovesFiles installs over files that exist beforehand and
// uninstalls with files the application wrote: an install-mode row deletes
// its file and keeps the others, an uninstall-mode * row deletes the
// matching files of its folder only, a folder row then removes the emptied
// folder, and the folders of a component in the Start menu go too.
func TestMsiexecRemovesFiles(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path := buildRemoveFiles(t)
	install := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderRemoveFiles")
	menu := filepath.Join(os.Getenv("ProgramData"), `Microsoft\Windows\Start Menu\Programs\MsiBuilderRemoveFiles`)

	cleanup := func() {
		ctx := context.Background()
		_ = exec.CommandContext(ctx, "msiexec", "/x", path, "/qn").Run()
		_ = os.RemoveAll(install)
		_ = os.RemoveAll(menu)
		_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKCU\SOFTWARE\MsiBuilderTest\RemoveFiles`, "/f").Run()
	}

	cleanup()
	t.Cleanup(cleanup)

	// MENUDIR exists before the install, so the engine did not create it
	// and only the generated RemoveFile row can remove it.
	if err := os.MkdirAll(menu, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(install, "stale-config.txt"))
	writeFile(t, filepath.Join(install, "keep.txt"))

	log := filepath.Join(filepath.Dir(path), "install.log")
	if code := msiexecCode(t, log, "/i", path); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
	}

	if exists(filepath.Join(install, "stale-config.txt")) {
		t.Error("stale-config.txt is still there after the install")
	}

	if !exists(filepath.Join(install, "keep.txt")) {
		t.Error("keep.txt was removed by the install")
	}

	if !exists(filepath.Join(menu, "Tools")) {
		t.Error("the Start menu folder was not created")
	}

	writeFile(t, filepath.Join(install, "Logs", "app.log"))
	writeFile(t, filepath.Join(install, "Logs", "service.log"))
	writeFile(t, filepath.Join(install, "app.log"))

	log = filepath.Join(filepath.Dir(path), "uninstall.log")
	if code := msiexecCode(t, log, "/x", path); code != 0 {
		t.Fatalf("msiexec /x returned %d\n%s", code, readLog(log))
	}

	if exists(filepath.Join(install, "Logs")) {
		t.Error("the Logs folder is still there after the uninstall")
	}

	// The pattern matches in its own folder only.
	if !exists(filepath.Join(install, "app.log")) {
		t.Error("app.log next to the Logs folder was removed")
	}

	// RemoveFolders removes Tools, the CreateFolder of Menu; the generated
	// RemoveFile row removes the pre-created folder above it.
	if exists(menu) {
		t.Error("the Start menu folder is still there after the uninstall")
	}
}

// TestICERemoveFiles runs the full ICE suite against the RemoveFile test
// package.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICERemoveFiles(t *testing.T) {
	cub := iceCube(t)
	requireCleanICE(t, buildRemoveFiles(t), cub)
}
