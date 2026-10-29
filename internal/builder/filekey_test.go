package builder_test

import (
	"testing"
)

// TestGeneratedFileKeys builds files whose names are no identifiers: their
// keys replace the "-", a shortcut targets the generated key, and a
// companion reference by long name is written as that key.
func TestGeneratedFileKeys(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files:
      - source: app.exe
        name: my-app.exe
        shortcuts:
          - {}
      - source: app.ico
        name: my-data.ico
        version: my-app.exe
ui: { wizard: minimal, launch: { file: my-app.exe } }
`)
	database := mustBuild(t, cfg).Database

	app := rowWhere(t, database, "File", func(r []any) bool { return r[0] == "my_app.exe" })
	if longOf(app[2]) != "my-app.exe" {
		t.Errorf("File row %v", app)
	}

	data := rowWhere(t, database, "File", func(r []any) bool { return r[0] == "my_data.ico" })
	if data[4] != "my_app.exe" {
		t.Errorf("companion File row %v, want Version my_app.exe", data)
	}

	launch := rowWhere(t, database, "CustomAction", func(r []any) bool { return r[0] == "LaunchOnFinish" })
	if launch[3] != `"[#my_app.exe]"` {
		t.Errorf("launch action %v", launch)
	}

	shortcut := rowWhere(t, database, "Shortcut", func(_ []any) bool { return true })
	if shortcut[scTarget] != "[#my_app.exe]" {
		t.Errorf("shortcut target %v", shortcut[scTarget])
	}
}

// TestKeyFileIsFirstConfiguredFile keeps the first file of the
// configuration as key path, although the files are sequenced by name: a
// service component lists its executable first.
func TestKeyFileIsFirstConfiguredFile(t *testing.T) {
	t.Parallel()

	cfg := shortcutConfig(t, `
  - files:
      - source: app.exe
        name: z-app.exe
      - source: app.ico
        name: a-config.ico
`)
	database := mustBuild(t, cfg).Database

	comp := rowWhere(t, database, "Component", func(r []any) bool { return r[5] != nil })
	if comp[5] != "z_app.exe" {
		t.Errorf("Component row %v, want key path z_app.exe", comp)
	}
}
