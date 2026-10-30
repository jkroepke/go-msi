//go:build windows

package nativetest_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// The folder, registry key and component the locator test searches.
const (
	locatorRoot = `C:\MsiBuilderLocator`
	locatorKey  = `HKLM\SOFTWARE\MsiBuilderTest\Locator`
	locatorGUID = "7B1E4D3F-6C2A-4F8B-8D4E-3A9F2C5B8D71"
)

const locatorBaseConfig = `schema: msi-builder/v1
product: { name: MsiBuilderLocatorBase, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a80 }
package: { platform: x64 }
installDir: { name: MsiBuilderLocatorBase }
components:
  - guid: ` + locatorGUID + `
    files: [{ source: base.exe }]
`

// locatorConfig returns the configuration of the search package; size is
// the size of app.exe.
func locatorConfig(size int) string {
	return fmt.Sprintf(`schema: msi-builder/v1
product: { name: MsiBuilderLocator, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 6f7a8b9c-0d1e-4f2a-9b3c-4d5e6f7a8b91 }
package: { platform: x64 }
installDir: { name: MsiBuilderLocator }
components:
  - files: [{ source: app.exe }]
properties:
  D_ABS: { search: { directory: { path: '%[1]s' } } }
  D_PROP: { search: { directory: { path: '[WindowsFolder]System32' } } }
  D_UNDER_REG: { search: { directory: { path: sub, under: { registry: { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\Locator', name: Dir } } } } }
  D_DEPTH: { search: { directory: { path: '%[1]s\b', depth: 3 } } }
  D_DEPTH_MISS: { value: none, search: { directory: { path: '%[1]s\b', depth: 1 } } }
  D_PF64: { search: { directory: { path: '[ProgramFiles64Folder]MsiBuilderLocatorBase' } } }
  D_MISSING: { value: none, search: { directory: { path: 'C:\MsiBuilderNoSuchFolder' } } }
  F_ABS: { search: { file: { name: app.exe, path: '%[1]s\sub' } } }
  F_DEPTH: { search: { file: { name: app.exe, path: '%[1]s', depth: 1 } } }
  F_DEPTH_MISS: { value: none, search: { file: { name: app.exe, path: '%[1]s' } } }
  F_LONG: { search: { file: { name: long file name.txt, path: '%[1]s' } } }
  F_LONG_EXT: { search: { file: { name: settings.json, path: '%[1]s' } } }
  F_LONG_MISS: { value: none, search: { file: { name: long file other.txt, path: '%[1]s' } } }
  F_MIN_EQUAL: { search: { file: { name: app.exe, path: '%[1]s\sub', minVersion: 1.2.3.4 } } }
  F_MIN_EQUAL_1033: { search: { file: { name: app.exe, path: '%[1]s\sub', minVersion: 1.2.3.4, languages: [1033] } } }
  F_MIN_EQUAL_1031: { value: none, search: { file: { name: app.exe, path: '%[1]s\sub', minVersion: 1.2.3.4, languages: [1031] } } }
  F_MIN_ABOVE: { value: none, search: { file: { name: app.exe, path: '%[1]s\sub', minVersion: 1.2.3.5 } } }
  F_MAX_EQUAL: { search: { file: { name: app.exe, path: '%[1]s\sub', maxVersion: 1.2.3.4 } } }
  F_MAX_BELOW: { value: none, search: { file: { name: app.exe, path: '%[1]s\sub', maxVersion: 1.2.3.3 } } }
  F_SIZE: { search: { file: { name: app.exe, path: '%[1]s\sub', minSize: %[2]d, maxSize: %[2]d } } }
  F_SIZE_BELOW: { value: none, search: { file: { name: app.exe, path: '%[1]s\sub', maxSize: %[3]d } } }
  F_DATE: { search: { file: { name: app.exe, path: '%[1]s\sub', minDate: 2000-01-01, maxDate: 2043-12-31 } } }
  F_DATE_TODAY: { search: { file: { name: app.exe, path: '%[1]s\sub', maxDate: %[5]s } } }
  F_DATE_FUTURE: { value: none, search: { file: { name: app.exe, path: '%[1]s\sub', minDate: 2043-01-01 } } }
  F_UNVERSIONED: { value: none, search: { file: { name: plain.txt, path: '%[1]s', minVersion: 0.0.0.1 } } }
  F_UNDER_REG: { search: { file: { name: app.exe, path: sub, under: { registry: { root: HKLM, key: 'SOFTWARE\MsiBuilderTest\Locator', name: Dir } } } } }
  F_UNDER_COMPONENT: { search: { file: { name: base.exe, under: { component: { guid: %[4]s } } } } }
  C_FOLDER: { search: { component: { guid: %[4]s } } }
  C_MISSING: { value: none, search: { component: { guid: 00000000-1111-2222-3333-444444444444 } } }
  L_FIRST_HIT:
    value: none
    search:
      - directory: { path: 'C:\MsiBuilderNoSuchFolder' }
      - directory: { path: '%[1]s\sub' }
      - directory: { path: '%[1]s' }
  L_NONE: { value: none, search: [{ directory: { path: 'C:\MsiBuilderNoSuchFolder' } }, { component: { guid: 00000000-1111-2222-3333-444444444444 } }] }
  L_CMD: { search: [{ directory: { path: '%[1]s' } }, { directory: { path: '%[1]s\sub' } }] }
  OS_PLAIN: { value: none, search: { file: { name: kernel32.dll, path: '[System64Folder]' } } }
  OS_WINDOWS: { value: none, search: { file: { name: kernel32.dll, path: '[WindowsFolder]System32' } } }
  OS_MIN: { value: none, search: { file: { name: kernel32.dll, path: '[WindowsFolder]System32', minVersion: 6.0 } } }
`, locatorRoot, size, size-1, locatorGUID, time.Now().Format(time.DateOnly))
}

// buildLocatorPackage writes a package from yaml with the given input files.
func buildLocatorPackage(t *testing.T, dir, name, yaml string, inputs map[string][]byte) string {
	t.Helper()

	for file, data := range inputs {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(yaml), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// locatorApp is app.exe, the file the file searches look for.
func locatorApp() []byte {
	return testpe.Build(testpe.Options{FileVersion: "1.2.3.4", Languages: []int{1033}})
}

// buildLocators writes the search package.
func buildLocators(t *testing.T) string {
	t.Helper()

	app := locatorApp()

	return buildLocatorPackage(t, t.TempDir(), "locator.msi", locatorConfig(len(app)), map[string][]byte{"app.exe": app})
}

// TestMsiexecSearchesFiles installs a package whose properties search
// folders, files and an installed component (SEARCH-05): full and relative
// paths, depth, version, size, date and language bounds, under searches
// and several searches into one property.
func TestMsiexecSearchesFiles(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	if testing.Short() {
		t.Skip("installing needs msiexec time and elevation")
	}

	ctx := context.Background()
	work := t.TempDir()
	app := locatorApp()

	cleanup := func() {
		_ = os.RemoveAll(locatorRoot)
		// The parent too: a key left below HKLM\SOFTWARE\MsiBuilderTest
		// changes what other tests' installs create.
		_ = exec.CommandContext(ctx, "reg.exe", "delete", `HKLM\SOFTWARE\MsiBuilderTest`, "/f", "/reg:64").Run()
	}

	cleanup()
	t.Cleanup(cleanup)

	for name, data := range map[string][]byte{
		`sub\app.exe`:         app,
		`plain.txt`:           []byte("x"),
		`long file name.txt`:  []byte("long"),
		`settings.json`:       []byte("{}"),
		`deep\a\b\marker.txt`: []byte("deep"),
	} {
		path := filepath.Join(locatorRoot, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := exec.CommandContext(t.Context(), "reg.exe", "add", locatorKey, "/f", "/reg:64", "/v", "Dir", "/d", locatorRoot).CombinedOutput(); err != nil {
		t.Fatalf("reg add: %v\n%s", err, out)
	}

	// The component the component searches find belongs to another product.
	baseDir := filepath.Join(work, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}

	base := buildLocatorPackage(t, baseDir, "base.msi", locatorBaseConfig, map[string][]byte{
		"base.exe": testpe.Build(testpe.Options{FileVersion: "3.0.0.0", Languages: []int{1033}}),
	})

	t.Cleanup(func() { _ = exec.CommandContext(ctx, "msiexec", "/x", base, "/qn").Run() })

	if code := msiexecCode(t, filepath.Join(baseDir, "install.log"), "/i", base); code != 0 {
		t.Fatalf("msiexec /i base.msi returned %d", code)
	}

	path := buildLocators(t)
	log := filepath.Join(filepath.Dir(path), "install.log")

	t.Cleanup(func() { _ = exec.CommandContext(ctx, "msiexec", "/x", path, "/qn").Run() })

	if code := msiexecCode(t, log, "/i", path, "L_CMD=cmd"); code != 0 {
		t.Fatalf("msiexec /i returned %d\n%s", code, readLog(log))
	}

	props := serverProperties(readLog(log))
	baseFolder := filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderLocatorBase") + `\`
	sub := locatorRoot + `\sub\`

	for name, want := range map[string]string{
		"D_ABS":             locatorRoot + `\`,
		"D_PROP":            filepath.Join(os.Getenv("SystemRoot"), "System32") + `\`,
		"D_UNDER_REG":       sub,
		"D_DEPTH":           locatorRoot + `\deep\a\b\`,
		"D_DEPTH_MISS":      "none",
		"D_PF64":            baseFolder,
		"D_MISSING":         "none",
		"F_ABS":             sub + "app.exe",
		"F_DEPTH":           sub + "app.exe",
		"F_DEPTH_MISS":      "none",
		"F_LONG":            locatorRoot + `\long file name.txt`,
		"F_LONG_EXT":        locatorRoot + `\settings.json`,
		"F_LONG_MISS":       "none",
		"F_MIN_EQUAL":       sub + "app.exe",
		"F_MIN_EQUAL_1033":  sub + "app.exe",
		"F_MIN_EQUAL_1031":  "none",
		"F_MIN_ABOVE":       "none",
		"F_MAX_EQUAL":       sub + "app.exe",
		"F_MAX_BELOW":       "none",
		"F_SIZE":            sub + "app.exe",
		"F_SIZE_BELOW":      "none",
		"F_DATE":            sub + "app.exe",
		"F_DATE_TODAY":      sub + "app.exe",
		"F_DATE_FUTURE":     "none",
		"F_UNVERSIONED":     "none",
		"F_UNDER_REG":       sub + "app.exe",
		"F_UNDER_COMPONENT": baseFolder + "base.exe",
		"C_FOLDER":          baseFolder,
		"C_MISSING":         "none",
		"L_FIRST_HIT":       sub,
		"L_NONE":            "none",
		"L_CMD":             "cmd",
	} {
		if got := props[name]; !strings.EqualFold(got, want) {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	// Windows' own files: recorded, not asserted (SEARCH-05).
	for _, name := range []string{"OS_PLAIN", "OS_WINDOWS", "OS_MIN"} {
		t.Logf("SEARCH-05 %s = %q", name, props[name])
	}

	if code := msiexecCode(t, filepath.Join(filepath.Dir(path), "uninstall.log"), "/x", path); code != 0 {
		t.Errorf("msiexec /x returned %d", code)
	}
}

// TestICELocators runs the full ICE suite against the locator test package.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICELocators(t *testing.T) {
	cub := iceCube(t)
	requireCleanICE(t, buildLocators(t), cub)
}
