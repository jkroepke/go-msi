package builder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
	"github.com/jkroepke/msi-builder/pkg/msidb"
)

// buildYAML builds a package from src next to a versioned hello.exe and
// decodes its database.
func buildYAML(t *testing.T, src string) *msidb.Database {
	t.Helper()

	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "msi.yaml"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(dir, "hello.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	database, err := msidb.Unmarshal(mustBuild(t, cfg).Package)
	if err != nil {
		t.Fatal(err)
	}

	return database
}

func TestRemoveRegistry(t *testing.T) {
	t.Parallel()

	database := buildYAML(t, `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: hello.exe }]
    removeRegistry:
      - { id: OldHandler, root: HKCU, key: 'Software\Classes\hello' }
      - { root: HKLM, key: 'Software\Example\Hello', name: Legacy }
  - id: Cleanup
    removeRegistry:
      - { root: HKMU, key: 'Software\Example\Old' }
`)

	if got := len(rows(t, database, "RemoveRegistry")); got != 3 {
		t.Fatalf("%d RemoveRegistry rows, want 3", got)
	}

	byID := func(id string) func([]any) bool { return func(r []any) bool { return r[0] == id } }
	if got := cell(t, database, "RemoveRegistry", "Name", byID("OldHandler")); got != "-" {
		t.Errorf("whole-key removal has name %v, want -", got)
	}

	if got := cell(t, database, "RemoveRegistry", "Root", byID("OldHandler")); got != 1 {
		t.Errorf("HKCU root %v, want 1", got)
	}

	isLegacy := func(r []any) bool { return r[3] == "Legacy" }
	if got := cell(t, database, "RemoveRegistry", "Root", isLegacy); got != 2 {
		t.Errorf("HKLM root %v, want 2", got)
	}

	isOld := func(r []any) bool { return r[2] == `Software\Example\Old` }
	if got := cell(t, database, "RemoveRegistry", "Root", isOld); got != -1 {
		t.Errorf("HKMU root %v, want -1", got)
	}

	if got := cell(t, database, "RemoveRegistry", "Component_", isOld); got != "Cleanup" {
		t.Errorf("component %v, want Cleanup", got)
	}

	// A component with only removals has its folder as key path.
	if got := cell(t, database, "Component", "KeyPath", func(r []any) bool { return r[0] == "Cleanup" }); got != nil {
		t.Errorf("Cleanup key path %v, want null", got)
	}

	seq := func(action string) any {
		return cell(t, database, "InstallExecuteSequence", "Sequence", func(r []any) bool { return r[0] == action })
	}

	remove, removeOK := seq("RemoveRegistryValues").(int)
	write, writeOK := seq("WriteRegistryValues").(int)

	if !removeOK || !writeOK || remove >= write {
		t.Errorf("RemoveRegistryValues at %v is not before WriteRegistryValues at %v", remove, write)
	}
}
