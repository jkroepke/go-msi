package builder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
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

func TestPropertySearch(t *testing.T) {
	t.Parallel()

	database := buildYAML(t, `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: hello.exe }]
properties:
  NETFX:
    search: { registry: { root: HKLM, key: 'SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full', name: Release } }
  OLDDIR:
    value: '[none]'
    hidden: true
    search: { registry: { root: HKCU, key: 'Software\Example', bitness: 32, type: directory } }
`)

	isNetfx := func(r []any) bool { return r[0] == "Search_NETFX" }
	if got := cell(t, database, "RegLocator", "Type", isNetfx); got != 18 {
		t.Errorf("NETFX RegLocator type %v, want 18 (raw, 64-bit)", got)
	}

	if got := cell(t, database, "RegLocator", "Name", isNetfx); got != "Release" {
		t.Errorf("NETFX RegLocator name %v", got)
	}

	isOld := func(r []any) bool { return r[0] == "Search_OLDDIR" }
	if got := cell(t, database, "RegLocator", "Type", isOld); got != 0 {
		t.Errorf("OLDDIR RegLocator type %v, want 0 (directory, 32-bit)", got)
	}

	if got := cell(t, database, "RegLocator", "Name", isOld); got != nil {
		t.Errorf("OLDDIR RegLocator name %v, want null (the default value)", got)
	}

	if got := cell(t, database, "AppSearch", "Property", func(r []any) bool { return r[1] == "Search_NETFX" }); got != "NETFX_FOUND" {
		t.Errorf("AppSearch property %v, want NETFX_FOUND", got)
	}

	if got := len(rows(t, database, "Signature")); got != 0 {
		t.Errorf("%d Signature rows, want an empty table", got)
	}

	byAction := func(name string) func([]any) bool { return func(r []any) bool { return r[0] == name } }
	if got := cell(t, database, "CustomAction", "Target", byAction("SearchDefault_OLDDIR")); got != `[\[]none[\]]` {
		t.Errorf("default target %v, want the escaped literal", got)
	}

	for _, seq := range []string{"InstallExecuteSequence", "InstallUISequence"} {
		apply := cell(t, database, seq, "Condition", byAction("SearchApply_NETFX"))
		if apply != "NETFX_FOUND AND NOT NETFX" {
			t.Errorf("%s: apply condition %v", seq, apply)
		}

		search, searchOK := cell(t, database, seq, "Sequence", byAction("AppSearch")).(int)
		applyAt, applyOK := cell(t, database, seq, "Sequence", byAction("SearchApply_OLDDIR")).(int)
		fallback, fallbackOK := cell(t, database, seq, "Sequence", byAction("SearchDefault_OLDDIR")).(int)

		if !searchOK || !applyOK || !fallbackOK || search >= applyAt || applyAt >= fallback {
			t.Errorf("%s: AppSearch %d, apply %d, default %d are not in that order", seq, search, applyAt, fallback)
		}
	}

	isProperty := func(name string) func([]any) bool { return func(r []any) bool { return r[0] == name } }
	if got := cell(t, database, "Property", "Value", isProperty("MsiHiddenProperties")); got != "OLDDIR;OLDDIR_FOUND" {
		t.Errorf("MsiHiddenProperties %v", got)
	}

	for _, r := range rows(t, database, "Property") {
		if r[0] == "OLDDIR" {
			t.Errorf("OLDDIR has a Property row %v; its default comes from the action", r)
		}
	}
}

// TestProductCodeIgnoresEmptyKeys checks that keys with an empty value do
// not change the automatic ProductCode, so that an optional key added to the
// schema keeps the code of an unchanged configuration.
func TestProductCodeIgnoresEmptyKeys(t *testing.T) {
	t.Parallel()

	const src = `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: hello.exe }]
`

	productCode := func(doc string) any {
		return cell(t, buildYAML(t, doc), "Property", "Value", func(r []any) bool { return r[0] == "ProductCode" })
	}

	plain := productCode(src)
	explicit := productCode(src + "    removeRegistry: []\n    condition: ''\n")

	if plain != explicit {
		t.Errorf("ProductCode %v with empty keys, %v without", explicit, plain)
	}

	// An explicit value, false and a mapping that pruning empties included,
	// is not the same as a missing key.
	const (
		data = "  - directory: dataDir\n"
		dirs = "directories:\n  - id: CommonAppDataFolder\n    children: [{ id: dataDir, name: Hello }]\n"
	)

	const launch = "ui: { wizard: minimal, launch: { file: hello.exe"

	for _, pair := range []struct{ without, with string }{
		{src, src + "    createFolder: true\n"},
		{src + "properties:\n  FOO: { value: x }\n", src + "properties:\n  FOO: { value: x, secure: false }\n"},
		{src + data + dirs, src + data + "    purge: true\n" + dirs},
		{src + launch + " } }\n", src + launch + ", checked: false } }\n"},
	} {
		if without, with := productCode(pair.without), productCode(pair.with); without == with {
			t.Errorf("ProductCode %v is the same for\n%s\nand\n%s", with, pair.without, pair.with)
		}
	}
}

// TestRegistryKeyAndDefaultValue gives a type key row and the key's default
// value different generated ids (round 4: aspia, qttabbar).
func TestRegistryKeyAndDefaultValue(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - registry:
      - { root: HKLM, key: Software\Acme, type: key, removeOnUninstall: true }
      - { root: HKLM, key: Software\Acme, value: x }
      - { root: HKLM, key: Software\Acme\Sub, type: key }
`)

	names := make(map[any]bool)

	for _, row := range rows(t, mustBuild(t, cfg).Database, "Registry") {
		names[row[3]] = true
	}

	if !names["*"] || !names["+"] || !names[nil] {
		t.Errorf("Registry names %v, want *, + and the default value", names)
	}
}
