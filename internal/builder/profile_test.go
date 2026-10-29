package builder_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/msidb"
)

// profileKey returns the Registry row that is the key path of component
// compID, failing unless the key path is a registry value.
func profileKey(t *testing.T, database *msidb.Database, compID string) ([]any, []any) {
	t.Helper()

	comp := rowWhere(t, database, "Component", func(r []any) bool { return r[0] == compID })

	if attrs, ok := comp[3].(int); !ok || attrs&4 == 0 {
		t.Fatalf("component %s: attributes %v have no registry key path bit: %v", compID, comp[3], comp)
	}

	return comp, rowWhere(t, database, "Registry", func(r []any) bool { return r[0] == comp[5] })
}

// A component in the user profile without a keyPath gets an HKCU value as
// its key path (ICE38), in every scope, and keeps the ComponentId its key
// file gives it (INST-04).
func TestProfileComponentGetsHKCUKeyPath(t *testing.T) {
	t.Parallel()

	for _, scope := range []string{"perUser", "perMachine", "perUserOrMachine", "perMachineOrUser"} {
		res := mustBuild(t, scopedShortcutConfig(t, scope, `
  - { id: Data, directory: LocalAppDataFolder/Acme, files: [{ source: tool.exe }] }
  - { id: Pinned, directory: LocalAppDataFolder/Acme/bin, keyPath: app.exe, files: [{ source: app.exe }] }
`))

		comp, reg := profileKey(t, res.Database, "Data")
		if reg[1] != 1 || reg[2] != `Software\[Manufacturer]\[ProductName]` || reg[4] != "#1" || reg[5] != "Data" ||
			!strings.HasPrefix(fmt.Sprint(reg[3]), "cmp") {
			t.Errorf("%s: key path row %v", scope, reg)
		}

		// The same component keyed on its file has the same code.
		keyed := mustBuild(t, scopedShortcutConfig(t, scope, `
  - { id: Data, directory: LocalAppDataFolder/Acme, keyPath: tool.exe, files: [{ source: tool.exe }] }
`))
		fileKeyed := rowWhere(t, keyed.Database, "Component", func(r []any) bool { return r[0] == "Data" })

		if fileKeyed[5] != "tool.exe" || fileKeyed[1] != comp[1] {
			t.Errorf("%s: file-keyed component %v, generated %v: the ComponentId changed", scope, fileKeyed, comp)
		}

		// An explicit keyPath stays, and the validator warns about it.
		pinned := rowWhere(t, res.Database, "Component", func(r []any) bool { return r[0] == "Pinned" })
		if pinned[5] != "app.exe" || !strings.Contains(fmt.Sprint(res.Diagnostics), "components[1].keyPath") {
			t.Errorf("%s: pinned component %v, diagnostics %s", scope, pinned, res.Diagnostics)
		}

		if strings.Contains(fmt.Sprint(res.Diagnostics), "components[0]") {
			t.Errorf("%s: %s", scope, res.Diagnostics)
		}
	}
}

// A per-user install dir is in the profile: its components get the value;
// a dual package's install dir is under Program Files and keeps the file.
func TestProfileKeyPathInstallDir(t *testing.T) {
	t.Parallel()

	for scope, generated := range map[string]bool{"perUser": true, "perUserOrMachine": false, "perMachine": false} {
		res := mustBuild(t, scopedShortcutConfig(t, scope, "  - { id: App, files: [{ source: app.exe }] }\n"))

		comp := rowWhere(t, res.Database, "Component", func(r []any) bool { return r[0] == "App" })
		if (comp[5] != "app.exe") != generated {
			t.Errorf("%s: key path %v, generated %v", scope, comp[5], generated)
		}

		if generated {
			profileKey(t, res.Database, "App")
		}
	}
}

// A component keyed on its folder keeps its CreateFolder row; one keyed on
// an HKCU value of its own keeps that value; an HKLM or HKMU value is
// replaced in a per-machine package.
func TestProfileKeyPathOtherKeys(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, scopedShortcutConfig(t, "perMachine", `
  - { id: Folder, directory: AppDataFolder/Acme/logs, createFolder: true }
  - { id: Empty, directory: AppDataFolder/Acme/cache }
  - id: Own
    directory: AppDataFolder/Acme
    registry: [{ id: OwnValue, root: HKCU, key: Software\Acme, name: x, type: dword, value: 1 }]
  - id: Machine
    directory: AppDataFolder/Acme/m
    registry: [{ id: MachineValue, root: HKLM, key: Software\Acme, name: y, type: dword, value: 1 }]
  - id: Mixed
    directory: AppDataFolder/Acme/u
    registry: [{ id: MixedValue, root: HKMU, key: Software\Acme, name: z, type: dword, value: 1 }]
  - { id: App, directory: INSTALLDIR, files: [{ source: app.exe }] }
`))

	for _, compID := range []string{"Folder", "Empty", "Machine", "Mixed"} {
		profileKey(t, res.Database, compID)
	}

	for _, compID := range []string{"Folder", "Empty"} {
		rowWhere(t, res.Database, "CreateFolder", func(r []any) bool { return r[1] == compID })
	}

	for compID, key := range map[string]string{"Own": "OwnValue"} {
		if comp, _ := profileKey(t, res.Database, compID); comp[5] != key {
			t.Errorf("%s: key path %v, want its own value %s", compID, comp[5], key)
		}
	}

	// An HKMU value is HKCU in a per-user package and stays the key path.
	perUser := mustBuild(t, scopedShortcutConfig(t, "perUser", `
  - id: Mixed
    directory: AppDataFolder/Acme/u
    registry: [{ id: MixedValue, root: HKMU, key: Software\Acme, name: z, type: dword, value: 1 }]
`))
	if comp, _ := profileKey(t, perUser.Database, "Mixed"); comp[5] != "MixedValue" {
		t.Errorf("per-user HKMU: key path %v", comp[5])
	}

	if strings.Contains(fmt.Sprint(res.Diagnostics), "ICE38") {
		t.Errorf("ICE38 warning: %s", res.Diagnostics)
	}
}

// Every component of a folder source in the profile gets its own value.
func TestProfileKeyPathFolderSource(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, scopedShortcutConfig(t, "perUser", "  - { id: Dist, files: [{ source: ./ }] }\n"))

	names := make(map[any]bool)

	for _, comp := range rows(t, res.Database, "Component") {
		_, reg := profileKey(t, res.Database, fmt.Sprint(comp[0]))
		names[reg[3]] = true
	}

	if n := len(rows(t, res.Database, "Component")); n < 3 || len(names) != n {
		t.Errorf("%d components, %d value names", n, len(names))
	}
}
