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

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// removeFoldersConfig is one version of the FILE-11 product. DATA_DIR comes
// from the command line of the first install and is remembered in the
// registry, which a search reads back, the way installers that keep data
// outside the install dir find it again at uninstall. OLD_DIR is removed on
// install. GUARD_WIN (inside the Windows folder) and GUARD_STD (a folder
// that holds TemplateFolder, which StdInside points into it) must be
// refused, and so must GUARD_LINK, a folder reached through a junction,
// and GUARD_PROFILE, a folder in the Public profile; OWN_DIR, a folder in
// the installing account's own profile, must not be. NESTED_DIR, a folder
// inside DATA_DIR listed before it, is moved first and must come back
// inside DATA_DIR on a rollback. FailRemove fails a
// removal after the folders were moved aside when FAIL_REMOVE is set, so
// the installation rolls back; it first writes MARKER when the read-only
// and the hidden file are gone from DATA_DIR at that point, so the test
// knows that the rollback put them back. Swap replaces the folder
// DATA_DIR\sub with a junction to BAIT_DIR, which holds files of the same
// names, after the immediate action checked DATA_DIR and before the
// deferred one moves it, the way a user who can write there could; the
// deletion must remove the junction, not what it points to.
const removeFoldersConfig = `schema: msi-builder/v1
product: { name: MsiBuilderRemoveFolders, manufacturer: msi-builder tests, version: %s, upgradeCode: 953f4393-d6f8-4869-9f33-427757ebacb1 }
package: { platform: x64 }
installDir: { name: MsiBuilderRemoveFolders, remember: true }
properties:
  DATA_DIR:
    search: { registry: { root: HKLM, key: SOFTWARE\MsiBuilderRemoveFolders, name: DataDir, type: directory } }
  OLD_DIR: {}
  NESTED_DIR: {}
  GUARD_WIN: {}
  GUARD_STD: {}
  GUARD_LINK: {}
  GUARD_PROFILE: {}
  OWN_DIR: {}
  MARKER: { secure: true }
  FAIL_REMOVE: { secure: true }
  SWAP: { secure: true }
  BAIT_DIR: { secure: true }
components:
  - files: [{ source: app.exe }]
    registry: [{ root: HKLM, key: SOFTWARE\MsiBuilderRemoveFolders, name: DataDir, value: '[DATA_DIR]' }]
    removeFolders:
      - property: INSTALLDIR
      - property: NESTED_DIR
      - property: DATA_DIR
      - { property: OLD_DIR, on: install }
      - property: GUARD_WIN
      - property: GUARD_STD
      - property: GUARD_LINK
      - property: GUARD_PROFILE
      - property: OWN_DIR
customActions:
  - id: StdInside
    setProperty: { name: TemplateFolder, value: '[GUARD_STD]Inner\' }
    schedule: [{ sequence: execute, after: AppSearch, condition: GUARD_STD }]   # before BuilderRemoveFolders, which runs right after RemoveFiles
  - id: FailRemove
    run: { command: '"[System64Folder]cmd.exe" /c (if not exist "[DATA_DIR]a.txt" if not exist "[DATA_DIR]nested\deeper\d.txt" echo removed>"[MARKER]") & exit 1' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, before: InstallFinalize, condition: REMOVE~="ALL" AND FAIL_REMOVE="1" }]
  - id: Swap
    run: { command: '"[System64Folder]cmd.exe" /c rmdir /s /q "[DATA_DIR]sub" && mklink /J "[DATA_DIR]sub" "[BAIT_DIR]"' }   # BAIT_DIR without a trailing backslash
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, before: RemoveFiles, condition: REMOVE~="ALL" AND SWAP="1" }]
`

// buildRemoveFolders writes one version of the FILE-11 package.
func buildRemoveFolders(t *testing.T, version string) (string, string) {
	t.Helper()

	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	if err := os.WriteFile(filepath.Join(dir, "app.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse(fmt.Appendf(nil, removeFoldersConfig, version), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "removefolders-"+version+".msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, res.ProductCode
}

// removeFoldersTree holds the folders the test fills and checks.
type removeFoldersTree struct {
	install, data, outside, bait, old, own, guardWin, guardStd, guardLink, guardProfile string
}

func newRemoveFoldersTree() removeFoldersTree {
	data := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderRemoveFolders")

	return removeFoldersTree{
		install:  filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderRemoveFolders"),
		data:     filepath.Join(data, "Data"),
		outside:  filepath.Join(data, "Outside"),
		bait:     filepath.Join(data, "Bait"),
		own:      filepath.Join(os.Getenv("LOCALAPPDATA"), "MsiBuilderRemoveFolders"),
		old:      filepath.Join(data, "Old"),
		guardWin: filepath.Join(os.Getenv("SystemRoot"), "MsiBuilderRemoveFolders"),
		guardStd: filepath.Join(data, "Std"),
		// Link is a junction to Outside; the guard sees the path through it.
		guardLink:    filepath.Join(data, "Link", "inner"),
		guardProfile: filepath.Join(os.Getenv("PUBLIC"), "MsiBuilderRemoveFolders"),
	}
}

// dataFiles are the files and folders the application "wrote" into
// DATA_DIR: nested folders, a read-only and a hidden file, an empty folder.
// Swap replaces sub with a junction before the final removal.
var dataFiles = []string{`a.txt`, `sub\b.txt`, `sub\deeper\c.txt`, `nested\deeper\d.txt`, `empty`} //nolint:gochecknoglobals // test table

// baitFiles are the files behind the junction that Swap puts in place of
// sub, named like the files RemoveFiles deletes there.
var baitFiles = []string{`b.txt`, `deeper\c.txt`} //nolint:gochecknoglobals // test table

// fill creates the data tree, a junction in it that points outside, and the
// guarded folders.
func (tree removeFoldersTree) fill(t *testing.T) {
	t.Helper()

	for _, name := range dataFiles {
		if name == "empty" {
			if err := os.MkdirAll(filepath.Join(tree.data, name), 0o755); err != nil {
				t.Fatal(err)
			}

			continue
		}

		writeFile(t, filepath.Join(tree.data, name))
	}

	if err := os.Chmod(filepath.Join(tree.data, `a.txt`), 0o400); err != nil {
		t.Fatal(err)
	}

	hidden, err := syscall.UTF16PtrFromString(filepath.Join(tree.data, `nested\deeper\d.txt`))
	if err != nil {
		t.Fatal(err)
	}

	if err := syscall.SetFileAttributes(hidden, syscall.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(tree.outside, "keep.txt"))

	for _, name := range baitFiles {
		writeFile(t, filepath.Join(tree.bait, name))
	}

	//nolint:gosec // test-controlled paths
	if out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", filepath.Join(tree.data, "link"), tree.outside).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v\n%s", err, out)
	}

	writeFile(t, filepath.Join(tree.outside, `inner\k.txt`))

	//nolint:gosec // test-controlled paths
	if out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", filepath.Dir(tree.guardLink), tree.outside).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v\n%s", err, out)
	}

	writeFile(t, filepath.Join(tree.old, `x\y.txt`))
	writeFile(t, filepath.Join(tree.guardWin, "g.txt"))
	writeFile(t, filepath.Join(tree.guardStd, `Inner\s.txt`))
	writeFile(t, filepath.Join(tree.guardProfile, "p.txt"))
	writeFile(t, filepath.Join(tree.own, `Own\o.txt`))
}

func (tree removeFoldersTree) cleanup(packages ...string) {
	ctx := context.Background()

	for _, pkg := range packages {
		_ = exec.CommandContext(ctx, "msiexec", "/x", pkg, "/qn").Run()
	}

	_ = os.Remove(filepath.Join(tree.data, "link"))
	_ = os.Remove(filepath.Join(tree.data, "sub")) // only removes the junction Swap leaves
	_ = os.Remove(filepath.Dir(tree.guardLink))
	_ = os.Chmod(filepath.Join(tree.data, `a.txt`), 0o600)
	_ = os.RemoveAll(filepath.Dir(tree.data))
	_ = os.RemoveAll(tree.install)
	_ = os.RemoveAll(tree.guardWin)
	_ = os.RemoveAll(tree.guardProfile)
	_ = os.RemoveAll(tree.own)
	_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\MsiBuilderRemoveFolders`, "/f", "/reg:64").Run()
}

// requireData fails unless every data file is in place.
func (tree removeFoldersTree) requireData(t *testing.T, when string) {
	t.Helper()

	for _, name := range append([]string{`link`}, dataFiles...) {
		if _, err := os.Lstat(filepath.Join(tree.data, name)); err != nil {
			t.Errorf("%s: %s is gone: %v", when, name, err)
		}
	}

	if !exists(filepath.Join(tree.install, `runtime\r.txt`)) {
		t.Errorf("%s: the runtime file in the install dir is gone", when)
	}

	if !exists(filepath.Join(tree.own, `Own\o.txt`)) {
		t.Errorf("%s: the file in OWN_DIR is gone", when)
	}
}

// requireGuarded fails when a refused folder or the junction's target lost
// a file.
func (tree removeFoldersTree) requireGuarded(t *testing.T, when string) {
	t.Helper()

	paths := make([]string, 0, 5+len(baitFiles))
	paths = append(paths,
		filepath.Join(tree.outside, "keep.txt"), filepath.Join(tree.outside, `inner\k.txt`),
		filepath.Join(tree.guardWin, "g.txt"), filepath.Join(tree.guardStd, `Inner\s.txt`),
		filepath.Join(tree.guardProfile, "p.txt"),
	)

	for _, name := range baitFiles {
		paths = append(paths, filepath.Join(tree.bait, name))
	}

	for _, path := range paths {
		if !exists(path) {
			t.Errorf("%s: %s is gone", when, path)
		}
	}
}

// TestMsiexecRemoveFolders checks removeFolders (FILE-11): the install
// removes OLD_DIR's tree, a major upgrade keeps the data, a removal that
// fails after the folders were moved aside restores every file, and the real removal
// deletes the install dir and DATA_DIR with their subfolders, read-only and
// hidden files, but not what a junction points to, nor the refused folders,
// nor the files behind a junction swapped in after the listing.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecRemoveFolders(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	oldPkg, code1 := buildRemoveFolders(t, "1.0.0")
	newPkg, code2 := buildRemoveFolders(t, "1.0.1")
	tree := newRemoveFoldersTree()
	logs := t.TempDir()

	tree.cleanup(oldPkg, newPkg)
	t.Cleanup(func() { tree.cleanup(oldPkg, newPkg) })
	tree.fill(t)

	guards := []string{
		"GUARD_WIN=" + tree.guardWin + `\`, "GUARD_STD=" + tree.guardStd + `\`, "GUARD_LINK=" + tree.guardLink + `\`,
		"GUARD_PROFILE=" + tree.guardProfile + `\`, "OWN_DIR=" + tree.own + `\`,
		// A folder inside DATA_DIR, listed before it: moved first, so the
		// rollback must put DATA_DIR back before it.
		"NESTED_DIR=" + filepath.Join(tree.data, "nested") + `\`,
	}

	log := filepath.Join(logs, "install.log")
	if code := msiexecCode(t, log, "/i", oldPkg, "DATA_DIR="+tree.data+`\`, "OLD_DIR="+tree.old+`\`); code != 0 {
		t.Fatalf("msiexec /i v1 returned %d\n%s", code, readLog(log))
	}

	if exists(tree.old) {
		t.Errorf("OLD_DIR is still there after the install\n%s", grepLog(readLog(log), "BuilderRemoveFolders: "))
	}

	writeFile(t, filepath.Join(tree.install, `runtime\r.txt`))
	tree.requireData(t, "after the install")

	log = filepath.Join(logs, "upgrade.log")
	if code := msiexecCode(t, log, "/i", newPkg); code != 0 {
		t.Fatalf("msiexec /i v2 returned %d\n%s", code, readLog(log))
	}

	if productInstalled(t, code1) || !productInstalled(t, code2) {
		t.Fatalf("after the upgrade: v1 installed %t, v2 installed %t", productInstalled(t, code1), productInstalled(t, code2))
	}

	tree.requireData(t, "after the major upgrade")

	marker := filepath.Join(filepath.Dir(tree.data), "removed.txt")
	log = filepath.Join(logs, "rollback.log")

	if code := msiexecCode(t, log, append([]string{"/x", newPkg, "FAIL_REMOVE=1", "MARKER=" + marker}, guards...)...); code != 1603 {
		t.Fatalf("msiexec /x FAIL_REMOVE=1 returned %d, want 1603\n%s", code, readLog(log))
	}

	if !productInstalled(t, code2) {
		t.Fatal("v2 is not installed after the rolled-back removal")
	}

	text := readLog(log)
	t.Logf("rolled-back removal:\n%s", grepLog(text, "BuilderRemoveFolders: "))

	if !strings.Contains(text, "BuilderRemoveFolders: to remove: "+tree.data+`\`) {
		t.Error("the log does not show the removal of DATA_DIR")
	}

	if strings.Contains(text, "cannot be restored") {
		t.Error("the rollback left a folder in the backup folder")
	}

	// Without the marker the files may never have been deleted, and the
	// check below would not show a rollback.
	if !exists(marker) {
		t.Error("DATA_DIR still held the read-only and the hidden file when FailRemove ran")
	}

	tree.requireData(t, "after the rolled-back removal")
	tree.requireGuarded(t, "after the rolled-back removal")

	log = filepath.Join(logs, "uninstall.log")
	if code := msiexecCode(t, log, append([]string{"/x", newPkg, "SWAP=1", "BAIT_DIR=" + tree.bait}, guards...)...); code != 0 {
		t.Fatalf("msiexec /x returned %d\n%s", code, readLog(log))
	}

	text = readLog(log)
	t.Logf("removal:\n%s", grepLog(text, "BuilderRemoveFolders: "))

	for _, want := range []string{
		"refused, inside the Windows folder", "refused, it is or holds the standard folder TemplateFolder",
		"refused, reached through a junction", "refused, inside the Public or Default profile",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the removal log lacks %q", want)
		}
	}

	if exists(tree.install) {
		t.Error("the install dir is still there after the removal")
	}

	if exists(tree.own) {
		t.Error("OWN_DIR, in the installing account's profile, is still there after the removal")
	}

	// Swap made sub a junction after the check; the bait files behind it
	// must stay (requireGuarded), and so must the target of the junction
	// link, which goes as a link with DATA_DIR.
	if _, err := os.Lstat(tree.data); err == nil {
		t.Error("DATA_DIR is still there after the removal")
	}

	for _, name := range []string{"BuilderRemoveFolders: to remove: " + tree.data + `\`, "BuilderRemoveFolders: deleted: " + tree.data + `\`} {
		if !strings.Contains(text, name) {
			t.Errorf("the removal log lacks %q", name)
		}
	}

	if matches, _ := filepath.Glob(filepath.Join(filepath.VolumeName(tree.data)+`\`, "BuilderRemoveFolders.*")); len(matches) != 0 {
		t.Errorf("backup folders left behind: %v", matches)
	}

	tree.requireGuarded(t, "after the removal")
}

// TestICERemoveFolders runs the full ICE suite against the FILE-11 package.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICERemoveFolders(t *testing.T) {
	cub := iceCube(t)
	path, _ := buildRemoveFolders(t, "1.0.0")
	requireCleanICE(t, path, cub)
}
