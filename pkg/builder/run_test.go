package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/msidb"
)

// runConfig is a one-file package with a deferred and an immediate run
// action.
const runConfig = `schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: %s }
components:
  - files: [{ source: a.txt }]
customActions:
  - id: Later
    run: { command: '"[System64Folder]cmd.exe" /c echo [ProductName]', directory: INSTALLDIR }
    execute: deferred
    impersonate: false
    return: check
    hideTarget: true
    schedule: [{ sequence: execute, after: InstallFiles }]
  - id: Now
    run: { command: '"[System64Folder]cmd.exe" /c echo now' }
    return: ignore
    schedule: [{ sequence: execute, after: InstallFinalize }]
  # Scheduled before Now after Now.Data was: its rows must not land between
  # Now.Data and Now, which share the BuilderRun property with Earlier.
  - id: Earlier
    run: { command: '"[System64Folder]cmd.exe" /c echo earlier' }
    return: ignore
    schedule: [{ sequence: execute, before: Now }]
`

func buildRun(t *testing.T, platform string) *msidb.Database {
	t.Helper()

	cfg, err := config.Parse(fmt.Appendf(nil, runConfig, platform), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.BaseDir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	database, err := msidb.Unmarshal(mustBuild(t, cfg).Package)
	if err != nil {
		t.Fatal(err)
	}

	return database
}

// TestRunActions checks the rows a run action expands to: the DLL in the
// Binary table, the type-1 action, the type-51 data action before it, and
// the data property per execution mode.
func TestRunActions(t *testing.T) {
	t.Parallel()

	database := buildRun(t, "x64")

	dll, err := cadll.Binary("x64")
	if err != nil {
		t.Fatal(err)
	}

	data, ok := cell(t, database, "Binary", "Data", func(r []any) bool { return r[0] == cadll.Key }).([]byte)
	if !ok || string(data) != string(dll) {
		t.Errorf("Binary.%s is not the x64 DLL", cadll.Key)
	}

	actions := map[string][]any{}
	for _, row := range rows(t, database, "CustomAction") {
		actions[fmt.Sprint(row[0])] = row
	}

	for name, want := range map[string]string{
		"Later":        "[11265 BuilderRun BuilderRun]",
		"Later.Data":   `[8243 Later [INSTALLDIR]|"[System64Folder]cmd.exe" /c echo [ProductName]]`,
		"Now":          "[65 BuilderRun BuilderRun]",
		"Now.Data":     `[51 BuilderRun |"[System64Folder]cmd.exe" /c echo now]`,
		"Earlier":      "[65 BuilderRun BuilderRun]",
		"Earlier.Data": `[51 BuilderRun |"[System64Folder]cmd.exe" /c echo earlier]`,
	} {
		row, found := actions[name]
		if !found {
			t.Errorf("CustomAction %s is missing", name)

			continue
		}

		if got := fmt.Sprint(row[1:4]); got != want {
			t.Errorf("CustomAction %s = %s, want %s", name, got, want)
		}
	}

	seq := map[string]int{}

	for _, row := range rows(t, database, "InstallExecuteSequence") {
		if n, ok := row[2].(int); ok {
			seq[fmt.Sprint(row[0])] = n
		}
	}

	for _, name := range []string{"Later", "Now", "Earlier"} {
		if seq[name+".Data"] != seq[name]-1 {
			t.Errorf("%s.Data at %d, %s at %d: the data action must come right before", name, seq[name+".Data"], name, seq[name])
		}
	}

	if seq["Earlier"] >= seq["Now.Data"] {
		t.Errorf("Earlier at %d, Now.Data at %d: a before-scheduled action must not split a run action from its data action", seq["Earlier"], seq["Now.Data"])
	}

	if hidden := cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "MsiHiddenProperties" }); hidden != "Later" {
		t.Errorf("MsiHiddenProperties = %v, want Later", hidden)
	}
}

// TestRunActionDLLPerPlatform writes the DLL of the package's platform.
func TestRunActionDLLPerPlatform(t *testing.T) {
	t.Parallel()

	for _, platform := range []string{"x86", "arm64"} {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()

			database := buildRun(t, platform)

			token := map[string]string{"x86": "Intel", "arm64": "Arm64"}[platform]

			dll, err := cadll.Binary(token)
			if err != nil {
				t.Fatal(err)
			}

			data, ok := cell(t, database, "Binary", "Data", func(r []any) bool { return r[0] == cadll.Key }).([]byte)
			if !ok || string(data) != string(dll) {
				t.Errorf("Binary.%s is not the %s DLL", cadll.Key, token)
			}
		})
	}
}

// TestRunActionReservedBinary rejects a binaries entry named like the DLL.
func TestRunActionReservedBinary(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse(fmt.Appendf(nil, runConfig+"binaries: { BuilderRun: a.txt }\n", "x64"), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.BaseDir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil {
		t.Fatal("built with a reserved binaries key")
	}

	if msg := fmt.Sprint(err, res.Diagnostics); !strings.Contains(msg, "binaries.BuilderRun") {
		t.Errorf("error does not name the key: %s", msg)
	}
}
