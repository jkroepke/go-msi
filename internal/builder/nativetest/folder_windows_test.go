//go:build windows

package nativetest_test

import (
	"bytes"
	"context"
	"fmt"
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

// folderTestConfig installs the folder dist next to the YAML file into
// Program Files\MsiBuilderFolder; %s is the version and %s removeOld.
const folderTestConfig = `schema: msi-builder/v1
product: { name: MsiBuilderFolder, manufacturer: msi-builder tests, version: %s, upgradeCode: 0b7d4c2e-5a8f-4e1b-9c3d-7f2a6e8b1d45 }
package: { platform: x64 }
upgrade: { removeOld: %s }
components:
  - files:
      - source: dist/
        exclude: '*.pdb'
`

// folderVersion is the content of one version of the folder source.
type folderVersion struct {
	files   map[string][]byte
	version string
}

// buildFolder writes the files of v below dir/dist and builds the package.
func buildFolder(t *testing.T, dir string, pkg folderVersion, removeOld string) string {
	t.Helper()

	for name, data := range pkg.files {
		path := filepath.Join(dir, "dist", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse(fmt.Appendf(nil, folderTestConfig, pkg.version, removeOld), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build %s: %v\n%s", pkg.version, err, res.Diagnostics)
	}

	path := filepath.Join(dir, "folder.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestMsiexecUpgradesFolder installs a folder source and upgrades it to
// a version in which files were removed, added, changed and renamed in case
// only, with an early and a late removal of the old product (FILE-08). The
// component GUIDs come from the install paths, so a file that stays at its
// path keeps its component and the old product's removal only takes the
// files the new version dropped.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecUpgradesFolder(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	first := folderVersion{version: "1.0.0", files: map[string][]byte{
		"app.exe":       testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
		"app.pdb":       []byte("symbols"),
		"readme.txt":    []byte("readme first"),
		"Notes.txt":     []byte("notes first"),
		"docs/old.txt":  []byte("old"),
		"keep/same.txt": []byte("same"),
	}}
	second := folderVersion{version: "2.0.0", files: map[string][]byte{
		"app.exe":       testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1033}}),
		"readme.txt":    []byte("readme second"),
		"NOTES.txt":     []byte("notes second"),
		"lib/new.txt":   []byte("new"),
		"keep/same.txt": []byte("same"),
	}}

	installDir := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderFolder")

	for _, removeOld := range []string{"afterInstallInitialize", "afterInstallExecute"} {
		t.Run(removeOld, func(t *testing.T) {
			old := buildFolder(t, t.TempDir(), first, removeOld)
			upgrade := buildFolder(t, t.TempDir(), second, removeOld)
			logs := t.TempDir()

			// The premise: a file that stays at its install path keeps its
			// component, whatever the case of its name.
			before, after := componentGUIDs(t, old), componentGUIDs(t, upgrade)
			for _, name := range []string{"app.exe", "readme.txt", "notes.txt", "same.txt"} {
				if before[name] == "" || before[name] != after[name] {
					t.Fatalf("%s has ComponentId %q in v1 and %q in v2", name, before[name], after[name])
				}
			}

			t.Cleanup(func() {
				ctx := context.Background()
				_ = exec.CommandContext(ctx, "msiexec", "/x", upgrade, "/qn").Run()
				_ = exec.CommandContext(ctx, "msiexec", "/x", old, "/qn").Run()
				_ = os.RemoveAll(installDir)
			})

			if code := msiexecCode(t, filepath.Join(logs, "v1.log"), "/i", old); code != 0 {
				t.Fatalf("msiexec /i v1 returned %d\n%s", code, readLog(filepath.Join(logs, "v1.log")))
			}

			folderState(t, "v1", installDir, map[string][]byte{
				"app.exe": first.files["app.exe"], "readme.txt": first.files["readme.txt"], "notes.txt": first.files["Notes.txt"],
				"docs/old.txt": first.files["docs/old.txt"], "keep/same.txt": first.files["keep/same.txt"],
				"app.pdb": nil,
			})

			if code := msiexecCode(t, filepath.Join(logs, "v2.log"), "/i", upgrade); code != 0 {
				t.Fatalf("msiexec /i v2 returned %d\n%s", code, readLog(filepath.Join(logs, "v2.log")))
			}

			folderState(t, "upgrade", installDir, map[string][]byte{
				"app.exe": second.files["app.exe"], "readme.txt": second.files["readme.txt"], "notes.txt": second.files["NOTES.txt"],
				"lib/new.txt": second.files["lib/new.txt"], "keep/same.txt": second.files["keep/same.txt"],
				"docs/old.txt": nil, "docs": nil,
			})

			if code := msiexecCode(t, filepath.Join(logs, "x.log"), "/x", upgrade); code != 0 {
				t.Fatalf("msiexec /x v2 returned %d", code)
			}

			if _, err := os.Stat(installDir); err == nil {
				t.Errorf("%s is still there after the uninstall", installDir)
			}

			t.Logf("FILE-08 %s: uninstall removed %s: %v", removeOld, installDir, !exists(installDir))
		})
	}
}

// folderState checks the files below dir: a nil entry must be absent, any
// other must hold exactly that content. Windows matches the names without
// case, so notes.txt finds Notes.txt and NOTES.txt.
func folderState(t *testing.T, step, dir string, want map[string][]byte) {
	t.Helper()

	for name, data := range want {
		path := filepath.Join(dir, filepath.FromSlash(name))

		got, err := os.ReadFile(path)

		switch {
		case data == nil:
			t.Logf("FILE-08 %s: %s present=%v", step, name, exists(path))

			if exists(path) {
				t.Errorf("%s: %s is still there", step, name)
			}
		case err != nil:
			t.Logf("FILE-08 %s: %s present=false", step, name)
			t.Errorf("%s: %v", step, err)
		default:
			t.Logf("FILE-08 %s: %s present=true, expected content=%v", step, name, bytes.Equal(got, data))

			if !bytes.Equal(got, data) {
				t.Errorf("%s: %s holds %d bytes, not the expected %d", step, name, len(got), len(data))
			}
		}
	}
}

// componentGUIDs returns the ComponentId of each file of a package by its
// lower-case long name.
func componentGUIDs(t *testing.T, path string) map[string]string {
	t.Helper()

	db := openDB(t, path)
	defer closeHandle(db)

	guids := make(map[string]string)

	for _, row := range query(t, db, "SELECT `File`.`FileName`, `Component`.`ComponentId` FROM `File`, `Component` WHERE `File`.`Component_` = `Component`.`Component`") {
		name := row[0]
		if i := strings.IndexByte(name, '|'); i >= 0 {
			name = name[i+1:]
		}

		guids[strings.ToLower(name)] = row[1]
	}

	return guids
}

func exists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
