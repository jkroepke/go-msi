//go:build windows

package nativetest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// removeFoldersSidConfig is the FILE-12 product: ForgeSid sets UserSID to
// the Guests group before the immediate folder removal runs, the way a
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
	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	if err := os.WriteFile(filepath.Join(dir, "app.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(removeFoldersSidConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	pkg := filepath.Join(dir, "removefolders-sid.msi")
	if err := os.WriteFile(pkg, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	data := filepath.Join(os.Getenv("ProgramData"), "MsiBuilderRemoveFoldersSid")
	logs := t.TempDir()

	cleanup := func() {
		_ = msiexecCode(t, filepath.Join(logs, "cleanup.log"), "/x", pkg)
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
