package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/config"
)

// short is the smallest package: no directories, features or upgrade, and a
// component without id or directory.
const short = `schema: go-msi/v1
product:
  name: Hello
  manufacturer: Example
  version: 1.0.0
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
package:
  platform: x64
`

func parseValid(t *testing.T, doc string) *config.Config {
	t.Helper()

	cfg, err := config.Parse([]byte(doc), config.Options{})
	if err != nil {
		t.Fatalf("parse: %v\n%s", err, doc)
	}

	if err := config.Validate(cfg).Err(); err != nil {
		t.Fatalf("validate: %v\n%s", err, doc)
	}

	return cfg
}

func findDir(ds []*config.Directory, dirID string) *config.Directory {
	for _, d := range ds {
		if d.ID == dirID {
			return d
		}

		if found := findDir(d.Children, dirID); found != nil {
			return found
		}
	}

	return nil
}

// dirPath returns the directory ids from the top-level folder down to id.
func dirPath(ds []*config.Directory, dirID string) []string {
	for _, dir := range ds {
		if dir.ID == dirID {
			return []string{dir.ID}
		}

		if p := dirPath(dir.Children, dirID); p != nil {
			return append([]string{dir.ID}, p...)
		}
	}

	return nil
}

func TestShorthandMinimal(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, short+"components:\n  - files: [{ source: hello.exe }]\n")

	c := cfg.Components[0]
	if c.ID != "hello.exe" || c.Directory != config.DefaultInstallDirID {
		t.Errorf("component = %s in %s, want hello.exe in INSTALLDIR", c.ID, c.Directory)
	}

	if got := dirPath(cfg.Directories, "INSTALLDIR"); !reflect.DeepEqual(got, []string{"ProgramFiles64Folder", "INSTALLDIR"}) {
		t.Errorf("install dir path = %v", got)
	}

	if d := cfg.Directories[0].Children[0]; d.Name != "Hello" || !d.Configurable {
		t.Errorf("install dir = %+v, want a configurable Hello", d)
	}

	if len(cfg.Features) != 1 || cfg.Features[0].ID != config.DefaultFeatureID ||
		cfg.Features[0].Components[0].ID != "hello.exe" || cfg.Features[0].Directory != "INSTALLDIR" {
		t.Errorf("default feature = %+v", cfg.Features)
	}

	if cfg.Upgrade == nil || cfg.Upgrade.Disabled || !*cfg.Upgrade.AllowSameVersion {
		t.Errorf("upgrade = %+v, want the standard policy", cfg.Upgrade)
	}
}

func TestShorthandInstallDir(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, doc string
		want      []string
		wantName  string
	}{
		{"x86", strings.Replace(short, "x64", "x86", 1), []string{"ProgramFilesFolder", "INSTALLDIR"}, "Hello"},
		{"name", short + "installDir: Hello World\n", []string{"ProgramFiles64Folder", "INSTALLDIR"}, "Hello World"},
		{
			"nested", short + "installDir: { id: APPDIR, name: Example/Hello }\n",
			[]string{"ProgramFiles64Folder", "ProgramFiles64Folder_Example", "APPDIR"}, "Hello",
		},
		{
			"per user", short + "  scope: perUser\n",
			[]string{"LocalAppDataFolder", "LocalAppDataFolder_Programs", "INSTALLDIR"}, "Hello",
		},
	} {
		cfg := parseValid(t, tc.doc+"components:\n  - files: [{ source: hello.exe }]\n")

		dirID := tc.want[len(tc.want)-1]
		if got := dirPath(cfg.Directories, dirID); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: path = %v, want %v", tc.name, got, tc.want)
		}

		if cfg.Components[0].Directory != dirID {
			t.Errorf("%s: component directory = %s", tc.name, cfg.Components[0].Directory)
		}

		if d := findDir(cfg.Directories, dirID); d == nil || d.Name != tc.wantName {
			t.Errorf("%s: directory %s = %+v, want name %q", tc.name, dirID, d, tc.wantName)
		}
	}
}

func TestShorthandExplicitTreeWins(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, base+appComponent)

	if len(cfg.Directories) != 1 || len(cfg.Directories[0].Children) != 1 {
		t.Errorf("an install dir defined in directories was added again: %+v", cfg.Directories[0].Children)
	}
}

func TestShorthandFolderPaths(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, short+`components:
  - files: [{ source: hello.exe }]
  - directory: textfile_inputs
    createFolder: true
  - directory: CommonAppDataFolder/Hello/logs
    createFolder: true
  - directory: ./CONF
    createFolder: true
  - directory: INSTALLDIR/textfile_inputs/sub
    createFolder: true
`)

	for i, want := range [][]string{
		1: {"ProgramFiles64Folder", "INSTALLDIR", "INSTALLDIR_textfile_inputs"},
		2: {"CommonAppDataFolder", "CommonAppDataFolder_Hello", "CommonAppDataFolder_Hello_logs"},
		3: {"ProgramFiles64Folder", "INSTALLDIR", "INSTALLDIR_CONF"},
		4: {"ProgramFiles64Folder", "INSTALLDIR", "INSTALLDIR_textfile_inputs", "INSTALLDIR_textfile_inputs_sub"},
	} {
		if want == nil {
			continue
		}

		if got := dirPath(cfg.Directories, cfg.Components[i].Directory); !reflect.DeepEqual(got, want) {
			t.Errorf("components[%d]: path = %v, want %v", i, got, want)
		}
	}

	if got := cfg.Components[1].ID; got != "INSTALLDIR_textfile_inputs" {
		t.Errorf("folder component id = %s", got)
	}

	// A typo in an id is reported, not turned into a folder.
	ds := validate(t, short+"components:\n  - { directory: INSTALDIR, createFolder: true }\n")
	if !find(ds, config.SeverityError, "components[0].directory", `directory "INSTALDIR" is not defined`) {
		t.Errorf("typo not reported: %v", ds)
	}
}

func TestShorthandFeatureComponents(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, short+`features:
  - id: Main
    title: Hello
    components:
      - files: [{ source: hello.exe }]
      - Shared
    features:
      - id: Extra
        components:
          - id: Docs
            files: [{ source: readme.txt }]
          - Shared
components:
  - id: Shared
    files: [{ source: shared.dll }]
`)

	ids := make([]string, 0, len(cfg.Components))
	for _, c := range cfg.Components {
		ids = append(ids, c.ID)
	}

	if want := []string{"Shared", "hello.exe", "Docs"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("components = %v, want %v", ids, want)
	}

	main := cfg.Features[0]
	if main.Components[0].ID != "hello.exe" || main.Components[1].ID != "Shared" ||
		main.Features[0].Components[0].ID != "Docs" || len(main.Components[0].Files) != 0 {
		t.Errorf("feature entries are not references: %+v", main.Components)
	}

	// Errors in a component defined in a feature name its place in the file.
	diags := validate(t, short+"features:\n  - id: Main\n    components:\n      - files: [{ source: a.exe, version: bad }]\n")
	if !find(diags, config.SeverityError, "features[0].components[0].files[0].version", "is not auto") {
		t.Errorf("path of an inline component: %v", diags)
	}

	// An id in components refers to nothing.
	diags = validate(t, short+"features: [{ id: Main, components: [A] }]\ncomponents: [A]\n")
	if !find(diags, config.SeverityError, "components[0]", "a component here is a mapping") {
		t.Errorf("id in components accepted: %v", diags)
	}
}

func TestShorthandUpgradeOff(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, short+"upgrade: false\ncomponents:\n  - files: [{ source: hello.exe }]\n")
	if !cfg.Upgrade.Disabled {
		t.Error("upgrade: false did not disable the upgrade")
	}

	ds := validate(t, short+"upgrade: { downgrade: block }\ncomponents:\n  - files: [{ source: hello.exe }]\n")
	if !find(ds, config.SeverityError, "upgrade.downgrade", "") {
		t.Errorf("upgrade mapping not validated: %v", ds)
	}
}

func TestShorthandServiceArguments(t *testing.T) {
	t.Parallel()

	cfg := parseValid(t, short+`components:
  - files: [{ source: svc.exe }]
    services:
      - name: svc
        arguments:
          - --verbose
          - { value: '--port [PORT]', condition: PORT }
          - value: --config="[CONFIG]"
            condition: CONFIG AND CONFIG<>"none"
`)

	s := cfg.Components[0].Services[0]
	if got, want := s.Arguments.String(), "--verbose [Arg_svc_1] [Arg_svc_2]"; got != want {
		t.Errorf("arguments = %q, want %q", got, want)
	}

	if len(cfg.CustomActions) != 2 {
		t.Fatalf("custom actions = %d, want 2", len(cfg.CustomActions))
	}

	ca := cfg.CustomActions[1]
	if ca.ID != "SetArg_svc_2" || ca.SetProperty.Name != "Arg_svc_2" || ca.SetProperty.Value != `--config="[CONFIG]"` ||
		ca.Schedule[0].Before != "InstallServices" || ca.Schedule[0].Condition != `CONFIG AND CONFIG<>"none"` {
		t.Errorf("action = %+v %+v", ca, ca.Schedule)
	}

	// The short form stays one command line.
	cfg = parseValid(t, short+"components:\n  - files: [{ source: svc.exe }]\n    services: [{ name: svc, arguments: '-a -b' }]\n")
	if got := cfg.Components[0].Services[0].Arguments.String(); got != "-a -b" {
		t.Errorf("arguments = %q", got)
	}
}

func TestShorthandIdempotent(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"../examples/minimal/msi.yaml", "../examples/windows_exporter/msi.yaml"} {
		cfg := load(t, path, map[string]string{"VERSION": "1.0.0"})

		once := *cfg
		config.ApplyDefaults(cfg)

		if len(once.Directories) != len(cfg.Directories) || len(once.Components) != len(cfg.Components) ||
			len(once.Features) != len(cfg.Features) || len(once.CustomActions) != len(cfg.CustomActions) {
			t.Errorf("%s: a second ApplyDefaults changed the config", path)
		}
	}
}
