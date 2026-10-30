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

// featureConditionsConfig has a base feature and two features whose level
// comes from Condition rows: Assoc is level 3 (not installed at the
// default INSTALLLEVEL 1), turned on by ASSOC=1 and disabled otherwise, as
// a real installer's file associations; Both has two true conditions.
const featureConditionsConfig = `schema: msi-builder/v1
product: { name: MsiBuilderFeatureConditions, manufacturer: msi-builder tests, version: %s, upgradeCode: 2b7e4c19-5d3a-4f60-8e1b-9c0a7d6f3e52 }
package: { platform: x64 }
installDir: { name: MsiBuilderFeatureConditions }
properties:
  ASSOC: { secure: true }
features:
  - id: Base
    components: [{ files: [{ source: app.exe }] }]
  - id: Assoc
    level: 3
    conditions:
      - { condition: ASSOC = 1, default: on }
      - { condition: ASSOC <> 1, default: disabled }
    components: [{ files: [{ source: assoc.txt }] }]
  - id: Both
    conditions:
      - { condition: "1", default: disabled }
      - { condition: "1", level: 2 }
    components: [{ files: [{ source: both.txt }] }]
`

// TestMsiexecFeatureConditions probes Condition rows (INST-05).
//
//nolint:paralleltest // msiexec runs one installation at a time
func TestMsiexecFeatureConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	dir := t.TempDir()
	inputs := map[string][]byte{
		"app.exe":   testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}}),
		"assoc.txt": []byte("assoc"),
		"both.txt":  []byte("both"),
	}
	older, oldRes := buildConfigResult(t, mkdir(t, dir, "v1"), "v1", fmt.Sprintf(featureConditionsConfig, "1.0.0"), inputs)
	newer, newRes := buildConfigResult(t, mkdir(t, dir, "v2"), "v2", fmt.Sprintf(featureConditionsConfig, "1.1.0"), inputs)
	folder := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderFeatureConditions")

	cleanup := func() {
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", newer, "/qn", "/norestart").Run()
		_ = exec.CommandContext(context.Background(), "msiexec", "/x", older, "/qn", "/norestart").Run()
		_ = os.RemoveAll(folder)
	}

	cleanup()
	t.Cleanup(cleanup)

	var observed []string

	step := func(name string, args ...string) {
		log := filepath.Join(dir, fmt.Sprintf("%02d.log", len(observed)))
		code := msiexecCode(t, log, args...)

		var states []string

		for line := range strings.Lines(readLog(log)) {
			if strings.Contains(line, "MSI (s)") && strings.Contains(line, "Feature: ") {
				states = append(states, strings.TrimSpace(line[strings.Index(line, "Feature: "):]))
			}
		}

		files := ""

		for _, file := range []string{"app.exe", "assoc.txt", "both.txt"} {
			if present(filepath.Join(folder, file)) {
				files += file + " "
			}
		}

		observed = append(observed, fmt.Sprintf("%s: exit %d, v1 registered %t, v2 registered %t, files [%s]\n    %s",
			name, code, productInstalled(t, oldRes.ProductCode), productInstalled(t, newRes.ProductCode),
			strings.TrimSpace(files), strings.Join(states, "\n    ")))
	}

	step("install ASSOC=1", "/i", older, "ASSOC=1")
	step("uninstall without ASSOC", "/x", older)
	cleanup()
	step("install no ASSOC", "/i", older)
	step("modify ADDLOCAL=Assoc", "/i", older, "ADDLOCAL=Assoc")
	step("uninstall", "/x", older)
	cleanup()
	step("install ASSOC=0 ADDLOCAL=Assoc", "/i", older, "ASSOC=0", "ADDLOCAL=Assoc")
	step("uninstall", "/x", older)
	cleanup()
	step("install ASSOC=1 ADDLOCAL=ALL", "/i", older, "ASSOC=1", "ADDLOCAL=ALL")
	step("uninstall", "/x", older)
	cleanup()
	step("install v1 ASSOC=1", "/i", older, "ASSOC=1")
	step("upgrade to v2 without ASSOC", "/i", newer)
	step("uninstall v2", "/x", newer)
	cleanup()
	step("install v1 ASSOC=1", "/i", older, "ASSOC=1")
	step("upgrade to v2 ASSOC=1", "/i", newer, "ASSOC=1")
	step("uninstall v2", "/x", newer)

	t.Errorf("INST-05 probe:\n%s", strings.Join(observed, "\n"))
}
