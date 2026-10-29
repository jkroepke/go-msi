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

	shortcut := rowWhere(t, database, "Shortcut", func(_ []any) bool { return true })
	if shortcut[scTarget] != "[#my_app.exe]" {
		t.Errorf("shortcut target %v", shortcut[scTarget])
	}
}
