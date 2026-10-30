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

// removeFoldersSidConfig is the FILE-12 product: ForgeSid sets UserSID to
// the Guests group before the folder removal's data is built, the way a
// standard user who controls the custom action server could set it to an
// administrator's SID. The deferred side reads UserSID to decide whether
// the account that started the session is an administrator; it must get
// the engine's value, not the forged one.
const removeFoldersSidConfig = `schema: msi-builder/v1
product: { name: MsiBuilderRemoveFoldersSid, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 0c6a0a39-6f0e-4f5c-9a0e-5b8d2b6f4e11 }
package: { platform: x64 }
properties:
  DATA_DIR: { secure: true }
  FORGE_SID: { secure: true }
components:
  - files: [{ source: app.exe }]
    removeFolders:
      - property: DATA_DIR
customActions:
  - id: ForgeSid
    setProperty: { name: UserSID, value: S-1-5-32-546 }
    schedule: [{ sequence: execute, after: AppSearch, condition: FORGE_SID="1" }]
`

// TestMsiexecRemoveFoldersUserSID checks FILE-12: an uninstall whose
// UserSID an earlier action set to a group without administrator rights
// still removes DATA_DIR, so the deferred side reads the engine's own
// value. If it read the forged one, the removal would log "nothing
// removed", and a standard user could forge an administrator's SID the
// same way.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecRemoveFoldersUserSID(t *testing.T) {
	pkg := buildRemoveFoldersProbe(t, removeFoldersSidConfig, "removefolders-sid.msi")

	data := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderRemoveFoldersSid")
	logs := t.TempDir()

	cleanup := func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", pkg, "/qn").Run()
		_ = os.RemoveAll(data)
	}
	cleanup()
	t.Cleanup(cleanup)
	writeFile(t, filepath.Join(data, `sub\file.txt`))

	log := filepath.Join(logs, "install.log")
	if code := msiexecCode(t, log, "/i", pkg); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
	}

	log = filepath.Join(logs, "uninstall.log")
	if code := msiexecCode(t, log, "/x", pkg, "FORGE_SID=1", "DATA_DIR="+data+`\`); code != 0 {
		t.Fatalf("msiexec /x returned %d\n%s", code, readLog(log))
	}

	text := readLog(log)
	t.Logf("removal:\n%s", grepLog(text, "BuilderRemoveFolders: ", "UserSID"))

	if !strings.Contains(text, "S-1-5-32-546") {
		t.Error("the log does not show ForgeSid setting UserSID; the probe proves nothing")
	}

	if strings.Contains(text, "nothing removed") {
		t.Error("the deferred side read the forged UserSID: it cannot tell an administrator from a standard user who sets UserSID")
	}

	if exists(data) {
		t.Error("DATA_DIR is still there after the removal")
	}
}

// buildRemoveFoldersProbe builds a FILE-12 or FILE-13 package from text with
// an app.exe next to it.
func buildRemoveFoldersProbe(t *testing.T, text, name string) string {
	t.Helper()

	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	if err := os.WriteFile(filepath.Join(dir, "app.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(text), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	pkg := filepath.Join(dir, name)
	if err := os.WriteFile(pkg, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return pkg
}

// removeFoldersLineConfig is the FILE-13 product: Inject sets LINE_DIR,
// the second entry, to a value with a line break followed by an f line
// that names BAIT_DIR, the way a property an earlier action or a command
// line set could try to add a folder to the data the engine formats.
const removeFoldersLineConfig = `schema: msi-builder/v1
product: { name: MsiBuilderRemoveFoldersLine, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 5d0b8f0e-3c41-4b6a-9f57-2f7b0c9e1d23 }
package: { platform: x64 }
properties:
  DATA_DIR: { secure: true }
  BAIT_DIR: { secure: true }
  INJECT: { secure: true }
components:
  - files: [{ source: app.exe }]
    removeFolders:
      - property: DATA_DIR
      - property: LINE_DIR
customActions:
  - id: Inject
    setProperty: { name: LINE_DIR, value: "[DATA_DIR]x\nfBAIT|[BAIT_DIR]" }
    schedule: [{ sequence: execute, after: AppSearch, condition: INJECT="1" }]
`

// TestMsiexecRemoveFoldersLineBreak checks FILE-13: an uninstall whose
// LINE_DIR holds a line break and a forged entry line removes nothing,
// neither DATA_DIR nor the folder the forged line names, and logs that
// the data has more lines than entries. Without the count, the deferred
// action would read the forged line as an entry of its own.
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecRemoveFoldersLineBreak(t *testing.T) {
	pkg := buildRemoveFoldersProbe(t, removeFoldersLineConfig, "removefolders-line.msi")
	data := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderRemoveFoldersLine")
	bait := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderRemoveFoldersLineBait")
	logs := t.TempDir()

	cleanup := func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", pkg, "/qn").Run()
		_ = os.RemoveAll(data)
		_ = os.RemoveAll(bait)
	}
	cleanup()
	t.Cleanup(cleanup)
	writeFile(t, filepath.Join(data, `sub\file.txt`))
	writeFile(t, filepath.Join(bait, `file.txt`))

	log := filepath.Join(logs, "install.log")
	if code := msiexecCode(t, log, "/i", pkg); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
	}

	log = filepath.Join(logs, "uninstall.log")
	if code := msiexecCode(t, log, "/x", pkg, "INJECT=1", "DATA_DIR="+data+`\`, "BAIT_DIR="+bait+`\`); code != 0 {
		t.Fatalf("msiexec /x returned %d\n%s", code, readLog(log))
	}

	text := readLog(log)
	t.Logf("removal:\n%s", grepLog(text, "BuilderRemoveFolders: ", "LINE_DIR"))

	if !strings.Contains(text, "more lines than entries") {
		t.Error("the removal log lacks the line count refusal")
	}

	if !exists(bait) {
		t.Error("the folder the forged line names was removed")
	}

	if !exists(data) {
		t.Error("DATA_DIR was removed although the data was malformed")
	}
}
