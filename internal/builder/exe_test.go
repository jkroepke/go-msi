package builder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// exeConfig parses a package whose binaries section maps setup.exe to the
// given data and that has the given customActions section.
func exeConfig(t *testing.T, data []byte, actions string) *config.Config {
	t.Helper()

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
binaries: { Setup: setup.exe }
components:
  - files: [{ source: a.txt }]
customActions:
`+actions), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()

	for name, content := range map[string][]byte{"a.txt": []byte("a"), "setup.exe": data} {
		if err := os.WriteFile(filepath.Join(cfg.BaseDir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return cfg
}

// TestEXEActions checks the type-2 rows of exe actions: the Binary key as
// Source, the arguments as Target and the options on top.
func TestEXEActions(t *testing.T) {
	t.Parallel()

	cfg := exeConfig(t, testpe.Build(testpe.Options{}), `
  - id: Prereq
    exe: { binary: Setup, arguments: '/silent /install /log "[TempFolder]setup.log"' }
    execute: deferred
    impersonate: false
    return: check
    schedule: [{ sequence: execute, before: InstallFiles, when: install }]
  - id: Probe
    exe: { binary: Setup }
    return: ignore
    schedule: [{ sequence: execute, after: CostFinalize }]
`)
	res := mustBuild(t, cfg)

	for name, want := range map[string][]any{
		"Prereq": {2 | 0x400 | 0x800, "Setup", `/silent /install /log "[TempFolder]setup.log"`},
		"Probe":  {2 | 0x40, "Setup", nil},
	} {
		row := rowWhere(t, res.Database, "CustomAction", func(r []any) bool { return r[0] == name })
		if row[1] != want[0] || row[2] != want[1] || row[3] != want[2] {
			t.Errorf("%s: CustomAction row %v, want type, source, target %v", name, row, want)
		}
	}

	if len(res.Diagnostics) > 0 {
		t.Errorf("diagnostics for a valid program: %s", res.Diagnostics)
	}
}

// TestEXEActionOptions checks the option bits on top of type 2.
func TestEXEActionOptions(t *testing.T) {
	t.Parallel()

	cfg := exeConfig(t, testpe.Build(testpe.Options{}), `
  - { id: Hidden, exe: { binary: Setup, arguments: --token }, execute: deferred, return: check, hideTarget: true,
      schedule: [{ sequence: execute, after: InstallFiles }] }
  - { id: Wait, exe: { binary: Setup }, return: asyncWait, once: firstSequence, schedule: [{ sequence: execute, after: CostFinalize }] }
  - { id: NoWait, exe: { binary: Setup }, return: asyncNoWait, schedule: [{ sequence: execute, after: InstallFinalize }] }
`)
	res := mustBuild(t, cfg)

	for name, want := range map[string]int{"Hidden": 2 | 0x400 | 0x2000, "Wait": 2 | 0x80 | 0x100, "NoWait": 2 | 0xC0} {
		if got := cell(t, res.Database, "CustomAction", "Type", func(r []any) bool { return r[0] == name }); got != want {
			t.Errorf("%s: Type %v, want %d", name, got, want)
		}
	}
}

func TestEXEActionValidation(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{})

	for _, tc := range []struct{ action, path, message string }{
		{`{ id: X, exe: { binary: Other }, return: check, schedule: [{ sequence: execute, after: CostFinalize }] }`, "exe.binary", `binary "Other" is not defined`},
		{`{ id: X, exe: { binary: Setup }, schedule: [{ sequence: execute, after: CostFinalize }] }`, ".return", "required"},
		{`{ id: X, exe: { binary: Setup }, dll: { binary: Setup, entry: E }, return: check, schedule: [{ sequence: execute, after: CostFinalize }] }`, "customActions[0]", "exactly one of"},
		{`{ id: X, exe: { binary: Setup }, impersonate: false, return: check, schedule: [{ sequence: execute, after: CostFinalize }] }`, ".impersonate", "needs a deferred"},
	} {
		_, err := builder.Build(exeConfig(t, exe, "  - "+tc.action+"\n"), builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(err.Error(), tc.path) || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%s: %v", tc.action, err)
		}
	}
}

// TestEXEActionBinaryErrors checks the errors for binaries the engine
// cannot start as a program.
func TestEXEActionBinaryErrors(t *testing.T) {
	t.Parallel()

	dll := testpe.Build(testpe.Options{})
	// COFF Characteristics: PE offset + signature + 18 bytes.
	peOff := int(dll[0x3C]) | int(dll[0x3D])<<8
	dll[peOff+4+18+1] |= 0x20

	for name, tc := range map[string]struct {
		data    []byte
		message string
	}{
		"msi":  {append([]byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"), make([]byte, 504)...), "Windows Installer package"},
		"text": {[]byte("echo hi"), "no PE header"},
		"dll":  {dll, "is a DLL"},
	} {
		cfg := exeConfig(t, tc.data, `  - { id: X, exe: { binary: Setup }, return: check, schedule: [{ sequence: execute, after: CostFinalize }] }
`)

		_, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
		if err == nil || !strings.Contains(err.Error(), "customActions[0].exe.binary") || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
