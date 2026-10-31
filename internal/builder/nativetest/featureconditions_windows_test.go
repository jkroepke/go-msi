package nativetest_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

// featureConditionsConfig has a base feature and three features whose
// level comes from Condition rows: Assoc is level 3 (not installed at the
// default INSTALLLEVEL 1), turned on by ASSOC=1 and disabled otherwise, as
// a real installer's file associations; Both has two true conditions; Soft
// is level 3 and turned on by SOFT=1, without a disabled row; Extra is
// disabled (level 0) and turned on by EXTRA=1.
const featureConditionsConfig = `schema: msi-builder/v1
product: { name: MsiBuilderFeatureConditions, manufacturer: msi-builder tests, version: %s, upgradeCode: 2b7e4c19-5d3a-4f60-8e1b-9c0a7d6f3e52 }
package: { platform: x64 }
installDir: { name: MsiBuilderFeatureConditions }
properties:
  ASSOC: { secure: true }
  SOFT: { secure: true }
  EXTRA: { secure: true }
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
  - id: Soft
    level: 3
    conditions:
      - { condition: SOFT = 1, default: on }
    components: [{ files: [{ source: soft.txt }] }]
  - id: Extra
    default: disabled
    conditions:
      - { condition: EXTRA = 1, default: on }
    components: [{ files: [{ source: extra.txt }] }]
`

// featureConditionStep is one msiexec run of TestMsiexecFeatureConditions
// and the payload files expected afterwards. The step's package is expected
// registered afterwards when files is not empty, and no package otherwise.
// deleteFile is removed from the install folder before the run.
type featureConditionStep struct {
	name       string
	v2         bool
	args       []string
	files      []string
	deleteFile string
}

// TestMsiexecFeatureConditions installs, changes, upgrades and removes a
// package with Condition rows (INST-05, INST-06). The builder ANDs NOT
// Installed onto most rows: without it, a feature that its condition
// disabled at the uninstall kept its files after the product was gone. A
// row that enables a disabled feature gets (Installed AND (REMOVE OR
// REINSTALL)) OR instead: with NOT Installed the feature fell back to level
// 0 at the uninstall, and msiexec /x left it and the product installed.
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
		"soft.txt":  []byte("soft"),
		"extra.txt": []byte("extra"),
	}
	v1, v1Res := buildConfigResult(t, mkdir(t, dir, "v1"), "v1", fmt.Sprintf(featureConditionsConfig, "1.0.0"), inputs)
	v2, v2Res := buildConfigResult(t, mkdir(t, dir, "v2"), "v2", fmt.Sprintf(featureConditionsConfig, "1.1.0"), inputs)
	packages := []string{v1, v2}
	codes := []string{v1Res.ProductCode, v2Res.ProductCode}
	folder := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderFeatureConditions")

	cleanup := func() {
		for _, pkg := range packages {
			_ = exec.CommandContext(context.Background(), "msiexec", "/x", pkg, "/qn", "/norestart").Run()
		}

		_ = os.RemoveAll(folder)
	}

	app, assoc, both, soft, extra := "app.exe", "assoc.txt", "both.txt", "soft.txt", "extra.txt"

	for _, sequence := range [][]featureConditionStep{
		{
			// Both: of the true rows at levels 0 and 2, level 2 wins.
			{"install", false, nil, []string{app}, ""},
			{"uninstall", false, []string{"/x"}, nil, ""},
		},
		{
			{"install ASSOC=1", false, []string{"ASSOC=1"}, []string{app, assoc}, ""},
			{"repair /fa", false, []string{"/fa"}, []string{app, assoc}, ""},
			{"uninstall without ASSOC", false, []string{"/x"}, nil, ""},
		},
		{
			{"install", false, nil, []string{app}, ""},
			{"add with ADDLOCAL=Assoc", false, []string{"ADDLOCAL=Assoc"}, []string{app, assoc}, ""},
			{"uninstall", false, []string{"/x"}, nil, ""},
		},
		{
			// The rows are evaluated with ADDLOCAL too: Assoc stays out,
			// so nothing is installed and msiexec still returns 0.
			{"install ASSOC=0 ADDLOCAL=Assoc", false, []string{"ASSOC=0", "ADDLOCAL=Assoc"}, nil, ""},
		},
		{
			{"install ASSOC=1 ADDLOCAL=ALL", false, []string{"ASSOC=1", "ADDLOCAL=ALL"}, []string{app, assoc, both, soft}, ""},
			{"uninstall", false, []string{"/x"}, nil, ""},
		},
		{
			// The disabled row drops Assoc from the upgrade unless ASSOC
			// is passed again; Soft, installed by v1, is kept.
			{"install v1 ASSOC=1 SOFT=1", false, []string{"ASSOC=1", "SOFT=1"}, []string{app, assoc, soft}, ""},
			{"upgrade to v2", true, nil, []string{app, soft}, ""},
			{"uninstall v2", true, []string{"/x"}, nil, ""},
		},
		{
			// Soft, which v1 had available but not installed, is requested
			// absent over its true row (presumably MigrateFeatureStates);
			// Assoc, which v1 had disabled, follows its row.
			{"install v1", false, nil, []string{app}, ""},
			{"upgrade to v2 ASSOC=1 SOFT=1", true, []string{"ASSOC=1", "SOFT=1"}, []string{app, assoc}, ""},
			{"uninstall v2", true, []string{"/x"}, nil, ""},
		},
		{
			// INST-06: Extra is disabled unless EXTRA=1; the uninstall and
			// the repair without the property still reach it.
			{"install EXTRA=1", false, []string{"EXTRA=1"}, []string{app, extra}, ""},
			{"repair /fa without EXTRA", false, []string{"/fa"}, []string{app, extra}, extra},
			{"uninstall without EXTRA", false, []string{"/x"}, nil, ""},
		},
		{
			// INST-06: REMOVE=Extra removes only Extra.
			{"install EXTRA=1", false, []string{"EXTRA=1"}, []string{app, extra}, ""},
			{"remove with REMOVE=Extra", false, []string{"REMOVE=Extra"}, []string{app}, ""},
			{"uninstall", false, []string{"/x"}, nil, ""},
		},
		{
			// INST-06: without EXTRA, a repair does not add Extra, and
			// ADDLOCAL in a later run stays behind the condition.
			{"install", false, nil, []string{app}, ""},
			{"repair /fa", false, []string{"/fa"}, []string{app}, ""},
			{"add with ADDLOCAL=Extra", false, []string{"ADDLOCAL=Extra"}, []string{app}, ""},
			{"add with EXTRA=1 ADDLOCAL=Extra", false, []string{"EXTRA=1", "ADDLOCAL=Extra"}, []string{app, extra}, ""},
			{"uninstall", false, []string{"/x"}, nil, ""},
		},
		{
			// INST-06: the old product's removal during an upgrade reaches
			// Extra too; the new product follows its row.
			{"install v1 EXTRA=1", false, []string{"EXTRA=1"}, []string{app, extra}, ""},
			{"upgrade to v2 without EXTRA", true, nil, []string{app}, ""},
			{"uninstall v2", true, []string{"/x"}, nil, ""},
		},
		{
			// INST-06: a feature the old product had disabled follows its
			// row at the upgrade.
			{"install v1", false, nil, []string{app}, ""},
			{"upgrade to v2 EXTRA=1", true, []string{"EXTRA=1"}, []string{app, extra}, ""},
			{"uninstall v2", true, []string{"/x"}, nil, ""},
		},
	} {
		cleanup()

		for _, step := range sequence {
			featureConditionRun(t, dir, folder, packages, codes, step)
		}
	}

	cleanup()
}

// featureConditionRun runs one step and compares the payload files.
func featureConditionRun(t *testing.T, dir, folder string, packages, codes []string, step featureConditionStep) {
	t.Helper()

	if step.deleteFile != "" {
		if err := os.Remove(filepath.Join(folder, step.deleteFile)); err != nil {
			t.Fatal(err)
		}
	}

	pkg := packages[0]
	if step.v2 {
		pkg = packages[1]
	}

	args := []string{"/i", pkg}
	if len(step.args) > 0 && strings.HasPrefix(step.args[0], "/") {
		args = append([]string{step.args[0], pkg}, step.args[1:]...)
	} else {
		args = append(args, step.args...)
	}

	log := filepath.Join(dir, strings.NewReplacer(" ", "_", "/", "", "=", "-").Replace(step.name)+".log")
	code := msiexecCode(t, log, args...)

	var states []string

	for line := range strings.Lines(readLog(log)) {
		if _, state, found := strings.Cut(line, "Feature: "); found && strings.Contains(line, "MSI (s)") && strings.Contains(state, "Installed:") {
			states = append(states, strings.TrimSpace(state))
		}
	}

	var files []string

	for _, file := range []string{"app.exe", "assoc.txt", "both.txt", "soft.txt", "extra.txt"} {
		if present(filepath.Join(folder, file)) {
			files = append(files, file)
		}
	}

	registered := make([]bool, 0, len(codes))
	for _, productCode := range codes {
		registered = append(registered, productInstalled(t, productCode))
	}

	t.Logf("INST-05/INST-06 %s: exit %d, files %v, v1/v2 registered %v\n%s", step.name, code, files, registered, strings.Join(states, "\n"))

	if code != 0 {
		t.Errorf("%s: msiexec returned %d\n%s", step.name, code, readLog(log))
	}

	if !slices.Equal(files, step.files) {
		t.Errorf("%s: files %v, want %v", step.name, files, step.files)
	}

	want := []bool{len(step.files) > 0 && !step.v2, len(step.files) > 0 && step.v2}
	if !slices.Equal(registered, want) {
		t.Errorf("%s: v1/v2 registered %v, want %v", step.name, registered, want)
	}
}
