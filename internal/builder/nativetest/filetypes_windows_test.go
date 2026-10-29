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

const fileTypesConfig = `schema: msi-builder/v1
product: { name: MsiBuilderFileTypes, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 6b0e3c9d-2f4a-4e81-b7c5-1d9a8e3f5b20 }
package: { platform: x64 }
components:
  - id: App
    files:
      - source: app.exe
        fileTypes:
          - progId: MsiBuilderTest.Document
            description: msi-builder test document
            iconIndex: 0
            extensions:
              - { extension: msibtdoc, contentType: application/x-msibt }
              - extension: msibtkeep
            verbs:
              - { id: open, text: '&Open' }
              - { id: print, arguments: '/p "%1"' }
          - progId: MsiBuilderTest.Other
            extensions: [{ extension: msibtoth, openWithOnly: true }]
`

// buildFileTypes writes the file type test package.
func buildFileTypes(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	if err := os.WriteFile(filepath.Join(dir, "app.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Parse([]byte(fileTypesConfig), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, "filetypes.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// regDefault returns the REG_SZ default value of a key in the 64-bit view,
// or "<absent>". The value's name is localised, so the line is found by its
// type.
func regDefault(t *testing.T, key string) string {
	t.Helper()

	for line := range strings.Lines(reg(t, "query", key, "/ve", "/reg:64")) {
		if _, data, ok := strings.Cut(line, "REG_SZ"); ok {
			return strings.TrimSpace(data)
		}
	}

	return "<absent>"
}

// regNamed returns the REG_SZ value of a key in the 64-bit view whose name
// may hold spaces, or "<absent>".
func regNamed(t *testing.T, key, name string) string {
	t.Helper()

	for line := range strings.Lines(reg(t, "query", key, "/v", name, "/reg:64")) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), name)
		if !ok {
			continue
		}

		if _, data, ok := strings.Cut(rest, "REG_SZ"); ok {
			return strings.TrimSpace(data)
		}
	}

	return "<absent>"
}

// TestMsiexecFileTypes installs a package with file types: the per-machine
// HKCR rows land in HKLM\Software\Classes, cmd's assoc and ftype resolve
// the extension and the open command to the installed file, each ProgID is
// an empty REG_SZ in its extensions' OpenWithProgids (REG-04), an
// openWithOnly extension gets no default, and the run DLL notifies the
// shell after InstallFinalize (REG-05). Uninstall removes the ProgIDs and
// their OpenWithProgids entries but keeps another program's entry, leaves
// the package's own extension defaults and Content Type in place (the
// permanent component, REG-05), and leaves a default that another program
// set after the install alone.
func TestMsiexecFileTypes(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	path := buildFileTypes(t)
	exe := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderFileTypes", "app.exe")
	classes := `HKLM\SOFTWARE\Classes\`

	cleanup := func() {
		ctx := context.Background()
		_ = exec.CommandContext(ctx, "msiexec", "/x", path, "/qn").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", classes+".msibtdoc", "/f", "/reg:64").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", classes+"MsiBuilderTest.Document", "/f", "/reg:64").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", classes+".msibtoth", "/f", "/reg:64").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", classes+".msibtkeep", "/f", "/reg:64").Run()
		_ = exec.CommandContext(ctx, "reg.exe", "delete", classes+"MsiBuilderTest.Other", "/f", "/reg:64").Run()
	}

	cleanup()
	t.Cleanup(cleanup)

	// Another program already lists itself under .msibtoth.
	regSet(t, "64", classes+`.msibtoth\OpenWithProgids`, "MsiBuilderTest.Foreign", "")

	log := filepath.Join(filepath.Dir(path), "install.log")
	if code := msiexecCode(t, log, "/i", path); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
	}

	for key, want := range map[string]string{
		".msibtdoc":                                   "MsiBuilderTest.Document",
		"MsiBuilderTest.Document":                     "msi-builder test document",
		`MsiBuilderTest.Document\DefaultIcon`:         `"` + exe + `",0`,
		`MsiBuilderTest.Document\shell\open`:          "&Open",
		`MsiBuilderTest.Document\shell\open\command`:  `"` + exe + `" "%1"`,
		`MsiBuilderTest.Document\shell\print\command`: `"` + exe + `" /p "%1"`,
		`MsiBuilderTest.Other\shell\open\command`:     `"` + exe + `" "%1"`,
	} {
		if got := regDefault(t, classes+key); !strings.EqualFold(got, want) {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	if got := regNamed(t, classes+".msibtdoc", "Content Type"); got != "application/x-msibt" {
		t.Errorf("Content Type = %q", got)
	}

	// A Registry row with a Name and a Null Value (REG-04).
	for key, name := range map[string]string{
		`.msibtdoc\OpenWithProgids`: "MsiBuilderTest.Document",
		`.msibtoth\OpenWithProgids`: "MsiBuilderTest.Other",
	} {
		if got := regNamed(t, classes+key, name); got != "" {
			t.Errorf("%s %s = %q, want an empty REG_SZ", key, name, got)
		}
	}

	// A key without a default lists no REG_SZ line without /ve; with /ve,
	// reg prints a localized "(value not set)".
	if text := reg(t, "query", classes+".msibtoth", "/reg:64"); strings.Contains(text, "REG_SZ") {
		t.Errorf(".msibtoth has a value, want none for openWithOnly:\n%s", text)
	}

	// The action starts after InstallFinalize and logs its session.
	notified := func(log string) {
		t.Helper()

		text := readLog(log)
		finalize := strings.Index(text, ": InstallFinalize.")
		action := strings.Index(text, ": BuilderAssocChanged.")

		if finalize < 0 || action < finalize {
			t.Errorf("%s: BuilderAssocChanged starts at %d, InstallFinalize at %d", log, action, finalize)
		}

		for line := range strings.Lines(text) {
			if strings.Contains(line, "BuilderRun: SHChangeNotify(SHCNE_ASSOCCHANGED) in session") {
				t.Logf("%s: %s", filepath.Base(log), strings.TrimSpace(line))

				return
			}
		}

		t.Errorf("%s has no SHChangeNotify line", log)
	}

	notified(log)

	// cmd reads HKEY_CLASSES_ROOT, the merged view Explorer uses.
	out, _ := exec.CommandContext(t.Context(), "cmd.exe", "/c", "assoc .msibtdoc && ftype MsiBuilderTest.Document").CombinedOutput()
	if text := string(out); !strings.Contains(text, ".msibtdoc=MsiBuilderTest.Document") ||
		!strings.Contains(strings.ToLower(text), strings.ToLower(`MsiBuilderTest.Document="`+exe+`" "%1"`)) {
		t.Errorf("assoc/ftype:\n%s", text)
	}

	// Another program takes .msibtdoc over after the install.
	if out, err := exec.CommandContext(t.Context(), "reg.exe", "add", classes+".msibtdoc", "/ve", "/d", "MsiBuilderTest.Foreign", "/f", "/reg:64").CombinedOutput(); err != nil {
		t.Fatalf("reg add: %v\n%s", err, out)
	}

	log = filepath.Join(filepath.Dir(path), "uninstall.log")
	if code := msiexecCode(t, log, "/x", path); code != 0 {
		t.Fatalf("msiexec /x returned %d\n%s", code, readLog(log))
	}

	notified(log)

	for _, key := range []string{"MsiBuilderTest.Document", "MsiBuilderTest.Other", `.msibtdoc\OpenWithProgids`, `.msibtkeep\OpenWithProgids`} {
		if regHas(t, "64", classes+key, "") {
			t.Errorf("%s is still there after the uninstall", key)
		}
	}

	// The extension defaults stay: the one another program set, and the
	// package's own, which now names a ProgID that is gone.
	for key, want := range map[string]string{".msibtdoc": "MsiBuilderTest.Foreign", ".msibtkeep": "MsiBuilderTest.Document"} {
		if got := regDefault(t, classes+key); got != want {
			t.Errorf("%s = %q after the uninstall, want %q", key, got, want)
		}
	}

	if got := regNamed(t, classes+".msibtdoc", "Content Type"); got != "application/x-msibt" {
		t.Errorf("Content Type = %q after the uninstall, want it kept", got)
	}

	out, _ = exec.CommandContext(t.Context(), "cmd.exe", "/c", "ftype MsiBuilderTest.Document").CombinedOutput()
	t.Logf("ftype after the uninstall: %s", strings.TrimSpace(string(out)))

	if regHas(t, "64", classes+`.msibtoth\OpenWithProgids`, "MsiBuilderTest.Other") ||
		!regHas(t, "64", classes+`.msibtoth\OpenWithProgids`, "MsiBuilderTest.Foreign") {
		t.Errorf("uninstall left .msibtoth\\OpenWithProgids as:\n%s", reg(t, "query", classes+`.msibtoth\OpenWithProgids`, "/reg:64"))
	}
}

// TestICEFileTypes runs the full ICE suite against the file type test
// package.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEFileTypes(t *testing.T) {
	cub := iceCube(t)
	requireCleanICE(t, buildFileTypes(t), cub)
}
