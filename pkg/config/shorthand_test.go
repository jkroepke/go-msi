package config_test

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/pkg/config"
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

	docs := map[string]string{
		"short": short + `features:
  - id: Main
    components:
      - files: [{ source: svc.exe }]
        services: [{ name: svc, arguments: [-a, { value: -b, condition: B }] }]
      - { directory: data/logs, createFolder: true }
`,
		"per user":    short + "  scope: perUser\ncomponents:\n  - files: [{ source: hello.exe }]\n",
		"upgrade off": short + "upgrade: false\ncomponents:\n  - files: [{ source: hello.exe }]\n",
	}

	for _, path := range []string{"../../examples/minimal/msi.yaml", "../../examples/windows_exporter/msi.yaml"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		docs[path] = string(data)
	}

	vars := config.Options{Vars: map[string]string{"VERSION": "1.0.0"}}

	for name, doc := range docs {
		once, err := config.Parse([]byte(doc), vars)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		twice, err := config.Parse([]byte(doc), vars)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		config.ApplyDefaults(twice)

		if !reflect.DeepEqual(once, twice) {
			t.Errorf("%s: a second ApplyDefaults changed the config", name)
		}
	}
}

func TestShorthandGoReferences(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(short+"components:\n  - { id: Foo, files: [{ source: foo.exe }] }\n"), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	// A Config changed in Go refers to components by id, with or without
	// ComponentRef.
	cfg.Features = []*config.Feature{{ID: "A", Components: []*config.Component{{ID: "Foo"}}},
		{ID: "B", Components: []*config.Component{config.ComponentRef("Foo")}}}

	if err := config.Validate(cfg).Err(); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Components) != 1 {
		t.Errorf("components = %d, want 1", len(cfg.Components))
	}
}

func TestShorthandArgumentErrors(t *testing.T) {
	t.Parallel()

	svc := short + "components:\n  - files: [{ source: svc.exe }]\n    services: [{ name: svc, arguments: [%s] }]\n"

	for arg, want := range map[string]string{
		"{ value: x, condition: 'A ==' }": "components[0].services[0].arguments[0].condition",
		"{ condition: A }":                "components[0].services[0].arguments[0].value",
	} {
		ds := validate(t, fmt.Sprintf(svc, arg))
		if !find(ds, config.SeverityError, want, "") {
			t.Errorf("%s: no error at %s: %v", arg, want, ds)
		}
	}
}

func TestShorthandInstallDirErrors(t *testing.T) {
	t.Parallel()

	comp := "components:\n  - files: [{ source: hello.exe }]\n"

	for doc, want := range map[string]string{
		short + "installDir: { id: appdir }\n" + comp:  "installDir.id",
		short + "installDir: { parent: Foo }\n" + comp: "installDir.parent",
	} {
		diags := validate(t, doc)
		if !find(diags, config.SeverityError, want, "") {
			t.Errorf("no error at %s: %v", want, diags)
		}

		if errs := len(diags) - len(diags.Warnings()); errs != 1 {
			t.Errorf("%s: want only the installDir error, got %v", want, diags)
		}
	}

	// A per-user component in the profile gets the ICE38 warning.
	ds := validate(t, short+"  scope: perUser\n"+comp)
	if !find(ds, config.SeverityWarning, "components[0].directory", "ICE38") {
		t.Errorf("no ICE38 warning: %v", ds)
	}
}

func TestShorthandUnusedInstallDir(t *testing.T) {
	t.Parallel()

	// An explicit tree without the install dir's id gets no extra folder.
	doc := strings.Replace(base, "INSTALLDIR", "APPDIR", 1) + strings.Replace(appComponent, "INSTALLDIR", "APPDIR", 1)
	cfg := parseValid(t, doc)

	if dirPath(cfg.Directories, config.DefaultInstallDirID) != nil {
		t.Errorf("unused install dir added: %+v", cfg.Directories[0].Children)
	}

	// A case typo of a known id is reported, not turned into a folder.
	ds := validate(t, base+"components:\n  - { id: App, directory: installdir, createFolder: true }\n")
	if !find(ds, config.SeverityError, "components[0].directory", "is not defined") {
		t.Errorf("case typo not reported: %v", ds)
	}
}

func TestShorthandStrictDecoding(t *testing.T) {
	t.Parallel()

	for _, doc := range []string{
		short + "features:\n  - id: F\n    components:\n      - bogus: 1\n",
		short + "upgrade: { bogus: 1 }\n",
		short + "upgrade: maybe\n",
		short + "installDir: { bogus: 1 }\n",
		short + "components:\n  - services: [{ name: s, arguments: [{ bogus: 1 }] }]\n",
	} {
		_, err := config.Parse([]byte(doc), config.Options{})
		if err == nil {
			t.Errorf("accepted:\n%s", doc)

			continue
		}

		if strings.Contains(err.Error(), "config.raw") {
			t.Errorf("internal type name in %v", err)
		}
	}
}

func TestShorthandReviewEdgeCases(t *testing.T) {
	t.Parallel()

	tree := strings.Replace(short, "package:\n  platform: x64\n", "package:\n  platform: x64\n"+
		"directories:\n  - id: ProgramFiles64Folder\n    children: [{ id: OTHER, name: Other }]\n", 1)

	// A path through the lazily added install dir starts in it.
	cfg := parseValid(t, tree+"components:\n  - { directory: INSTALLDIR/bin, createFolder: true }\n")
	if got := dirPath(cfg.Directories, cfg.Components[0].Directory); !reflect.DeepEqual(got,
		[]string{"ProgramFiles64Folder", "INSTALLDIR", "INSTALLDIR_bin"}) {
		t.Errorf("lazy install dir path = %v", got)
	}

	// {id: X} in a feature is an empty component, not a reference.
	cfg = parseValid(t, short+"features: [{ id: F, components: [{ id: Empty }] }]\n")
	if len(cfg.Components) != 1 || cfg.Components[0].ID != "Empty" {
		t.Errorf("components = %+v", cfg.Components)
	}

	// Null entries are errors, not panics.
	for _, doc := range []string{
		short + "features: [{ id: F, components: [~] }]\n",
		short + "components: [~]\n",
	} {
		if _, err := config.Parse([]byte(doc), config.Options{}); err == nil || !strings.Contains(err.Error(), "empty entry") {
			t.Errorf("null entry: %v", err)
		}
	}

	// A typo in a path's first segment is reported.
	diags := validate(t, short+"components:\n  - { directory: CommonAppdataFolder/Hello/logs, createFolder: true }\n")
	if !find(diags, config.SeverityError, "components[0].directory", "is not defined") {
		t.Errorf("first-segment typo not reported: %v", diags)
	}
}

func TestShorthandProfileKeyPath(t *testing.T) {
	t.Parallel()

	reg := "registry: [{ root: HKCU, key: Software\\Acme, name: installed, type: dword, value: 1 }]"

	for comp, ice38 := range map[string]bool{
		"{ directory: ProgramMenuFolder/Acme, " + reg + " }":                             false,
		"{ directory: ProgramMenuFolder/Acme, files: [{ source: a.txt }], " + reg + " }": true,
	} {
		diags := validate(t, short+"components:\n  - "+comp+"\n")
		if find(diags, config.SeverityWarning, "components[0].directory", "ICE38") != ice38 {
			t.Errorf("%s: ICE38 warning = %v, want %v: %v", comp, !ice38, ice38, diags)
		}

		if !find(diags, config.SeverityWarning, "components[0].directory", "ICE64") {
			t.Errorf("%s: no ICE64 warning: %v", comp, diags)
		}
	}
}

func TestShorthandGeneratedFolderErrors(t *testing.T) {
	t.Parallel()

	comp := "components:\n  - files: [{ source: hello.exe }]\n"

	for doc, want := range map[string]string{
		strings.Replace(short, "name: Hello", "name: Hello Inc.", 1) + comp:                           "product.name",
		short + "installDir: Hello.\n" + comp:                                                         "installDir.name",
		short + "components:\n  - { directory: ../x, createFolder: true }\n":                          "components[0].directory",
		short + "features:\n  - id: F\n    components: [{ directory: 'a/b.', createFolder: true }]\n": "features[0].components[0].directory",
	} {
		diags := validate(t, doc)
		if !find(diags, config.SeverityError, want, "folder name") {
			t.Errorf("no error at %s: %v", want, diags)
		}
	}

	// A case typo of the install dir is reported while the dir is not yet in
	// the tree.
	tree := strings.Replace(short, "package:\n  platform: x64\n", "package:\n  platform: x64\n"+
		"directories:\n  - id: CommonAppDataFolder\n    children: [{ id: DATADIR, name: Data }]\n", 1)

	diags := validate(t, tree+"components:\n  - { directory: installdir/bin, createFolder: true }\n")
	if !find(diags, config.SeverityError, "components[0].directory", "is not defined") {
		t.Errorf("lazy install dir typo not reported: %v", diags)
	}
}

func TestShorthandProductNameFolder(t *testing.T) {
	t.Parallel()

	comp := "components:\n  - files: [{ source: hello.exe }]\n"

	for _, name := range []string{"TCP/IP Monitor", `'A\B'`, "'Acme: Tool'"} {
		doc := strings.Replace(short, "name: Hello", "name: "+name, 1) + "  cabinet: app.cab\n" + comp

		diags := validate(t, doc)
		if !find(diags, config.SeverityError, "product.name", "set installDir.name") {
			t.Errorf("%s: %v", name, diags)
		}
	}

	// The same component in components and in a feature, as Go code writes it.
	cfg, err := config.Parse([]byte(short), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	exe := &config.Component{ID: "Exe", Files: []*config.File{{Source: "a.exe"}}}
	cfg.Components = []*config.Component{exe}
	cfg.Features = []*config.Feature{{ID: "F", Components: []*config.Component{exe}}}

	if err := config.Validate(cfg).Err(); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Components) != 1 || exe.Path(0) != "components[0]" {
		t.Errorf("shared component hoisted again: %d components, path %s", len(cfg.Components), exe.Path(0))
	}
}

func TestShorthandGoSharedWithoutID(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(short), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	exe := &config.Component{Files: []*config.File{{Source: "hello.exe"}}}
	cfg.Components = []*config.Component{exe}
	cfg.Features = []*config.Feature{{ID: "F", Components: []*config.Component{exe}}}

	if err := config.Validate(cfg).Err(); err != nil {
		t.Fatal(err)
	}

	if got := cfg.Features[0].Components[0].ID; got != "hello.exe" {
		t.Errorf("feature refers to %q, want hello.exe", got)
	}
}

func TestShorthandInstallDirOrder(t *testing.T) {
	t.Parallel()

	tree := "directories:\n  - id: CommonAppDataFolder\n    children: [{ id: DATADIR, name: Hello }]\n"
	plugins := "  - { directory: ProgramFiles64Folder/Hello/plugins, files: [{ source: readme.txt }] }\n"
	exe := "  - files: [{ source: hello.exe }]\n"

	// The path through the install dir's folder reuses it in either order.
	for _, comps := range []string{plugins + exe, exe + plugins} {
		cfg := parseValid(t, short+tree+"components:\n"+comps)

		got := dirPath(cfg.Directories, config.DefaultInstallDirID+"_plugins")
		want := []string{"ProgramFiles64Folder", config.DefaultInstallDirID, config.DefaultInstallDirID + "_plugins"}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("plugins path %v, want %v\n%s", got, want, comps)
		}
	}

	// An explicit folder at the install dir's place is a warning.
	explicit := "directories:\n  - id: ProgramFiles64Folder\n    children: [{ id: APPDIR, name: Hello }]\n" +
		"components:\n  - { directory: APPDIR, files: [{ source: readme.txt }] }\n" + exe

	if ds := validate(t, short+explicit); !find(ds, config.SeverityWarning, "product.name", "same folder") {
		t.Errorf("no same-folder warning: %v", ds)
	}
}

func TestShorthandNestedStandardFolder(t *testing.T) {
	t.Parallel()

	tree := "directories:\n  - id: TARGETDIR\n    children:\n      - id: ProgramFiles64Folder\n" +
		"        name: PFiles64\n        children: [{ id: APPDIR, name: Other }]\n"
	comps := "components:\n  - files: [{ source: hello.exe }]\n" +
		"  - { directory: ProgramFiles64Folder/x, files: [{ source: readme.txt }] }\n"

	cfg := parseValid(t, short+tree+comps)

	// Standard folders with the same placeholder name are not one folder.
	twins := "directories:\n  - id: TARGETDIR\n    children:\n" +
		"      - { id: ProgramFiles64Folder, name: PFiles }\n      - { id: ProgramFilesFolder, name: PFiles }\n"
	if ds := validate(t, short+twins+"components:\n  - files: [{ source: hello.exe }]\n"); len(ds) != 0 {
		t.Errorf("standard folders reported: %v", ds)
	}

	if len(cfg.Directories) != 1 {
		t.Fatalf("standard folder added at the top level again: %d top-level directories", len(cfg.Directories))
	}

	for _, dirID := range []string{config.DefaultInstallDirID, "ProgramFiles64Folder_x"} {
		want := []string{"TARGETDIR", "ProgramFiles64Folder", dirID}
		if got := dirPath(cfg.Directories, dirID); !reflect.DeepEqual(got, want) {
			t.Errorf("%s path %v, want %v", dirID, got, want)
		}
	}
}

func TestShorthandFeatureDefault(t *testing.T) {
	t.Parallel()

	feature := func(fields string) string {
		return short + "features:\n  - { id: F, " + fields + "components: [{ files: [{ source: hello.exe }] }] }\n"
	}

	for value, level := range map[string]int{"on": 1, "off": 2, "disabled": 0} {
		cfg := parseValid(t, feature("default: "+value+", "))
		if got := *cfg.Features[0].Level; got != level {
			t.Errorf("default %s: level %d, want %d", value, got, level)
		}
	}

	if ds := validate(t, feature("default: off, level: 3, ")); !find(ds, config.SeverityError, "features[0].default", "not both") {
		t.Errorf("default with level: %v", ds)
	}

	if ds := validate(t, feature("default: maybe, ")); !find(ds, config.SeverityError, "features[0].default", "") {
		t.Errorf("invalid default: %v", ds)
	}
}

func TestShorthandFeatureFirewall(t *testing.T) {
	t.Parallel()

	rule := "{ name: %s, program: '[#hello.exe]', protocol: tcp, port: '9182' }"
	doc := short + "features:\n  - id: Main\n    components: [{ files: [{ source: hello.exe }] }]\n" +
		"    features:\n      - id: Fw\n        default: off\n        firewall: [" + fmt.Sprintf(rule, "a") + ", " + fmt.Sprintf(rule, "b") + "]\n" +
		"      - id: Fw2\n        firewall: [" + fmt.Sprintf(rule, "c") + "]\n"

	cfg := parseValid(t, doc)

	carriers := map[string]string{}

	for _, comp := range cfg.Components {
		if feature := comp.FirewallFeature(); feature != "" {
			carriers[feature] = comp.ID

			if comp.Directory != config.DefaultInstallDirID || comp.KeyPath != config.KeyPathDirectory {
				t.Errorf("carrier %s: directory %s, keyPath %s", comp.ID, comp.Directory, comp.KeyPath)
			}
		}
	}

	if carriers["Fw"] != "Fw_Firewall" || carriers["Fw2"] != "Fw2_Firewall" {
		t.Fatalf("carriers: %v", carriers)
	}

	want := map[string]string{"a": "Fw_Firewall", "b": "Fw_Firewall", "c": "Fw2_Firewall"}
	for _, fw := range cfg.Firewall {
		if fw.Component != want[fw.Name] {
			t.Errorf("rule %s follows %q, want %s", fw.Name, fw.Component, want[fw.Name])
		}
	}

	if sub := cfg.Features[0].Features[0]; len(sub.Firewall) != 0 || len(sub.Components) != 1 || sub.Components[0].ID != "Fw_Firewall" {
		t.Errorf("feature Fw: %d rules, components %v", len(sub.Firewall), sub.Components)
	}

	// Errors in a rule name its place under the feature.
	bad := short + "components: [{ files: [{ source: hello.exe }] }]\nfeatures:\n  - id: Fw\n" +
		"    firewall:\n      - { name: a, protocol: tcp }\n      - { name: b, component: hello.exe, program: x, protocol: tcp }\n"

	diags := validate(t, bad)
	if !find(diags, config.SeverityError, "features[0].firewall[0].program", "required") {
		t.Errorf("no error at the rule's path: %v", diags)
	}

	if !find(diags, config.SeverityError, "features[0].firewall[1].component", "remove component") {
		t.Errorf("component on a feature rule accepted: %v", diags)
	}
}

func TestShorthandFeatureFirewallEdges(t *testing.T) {
	t.Parallel()

	main := "features:\n  - id: Main\n    components: [{ files: [{ source: hello.exe }] }]\n"
	rule := "{ name: %s, program: '[#hello.exe]', protocol: tcp }"

	// A null rule is an error, not a panic.
	for doc, path := range map[string]string{
		short + main + "    firewall: [~]\n":                                    "features[0].firewall[0]",
		short + main + "firewall: [~]\n":                                        "firewall[0]",
		short + "components: [{ files: [{ source: a.exe }] }]\nfirewall: [~]\n": "firewall[0]",
	} {
		if _, err := config.Parse([]byte(doc), config.Options{}); err == nil || !strings.Contains(err.Error(), path+": empty entry") {
			t.Errorf("%s: %v", path, err)
		}
	}

	// A rule added from Go after parsing reuses the feature's carrier.
	cfg := parseValid(t, short+main+"    firewall: ["+fmt.Sprintf(rule, "a")+"]\n")
	cfg.Features[0].Firewall = append(cfg.Features[0].Firewall, &config.FirewallRule{Name: "b", Program: "x", Protocol: "tcp"})

	if err := config.Validate(cfg).Err(); err != nil {
		t.Fatal(err)
	}

	carriers := 0

	for _, comp := range cfg.Components {
		if comp.FirewallFeature() != "" {
			carriers++
		}
	}

	if carriers != 1 || len(cfg.Firewall) != 2 || cfg.Firewall[1].Component != "Main_Firewall" {
		t.Errorf("%d carriers, %d rules", carriers, len(cfg.Firewall))
	}

	// With an explicit tree, the carrier joins the feature's directory and
	// adds no install dir.
	tree := "directories:\n  - id: ProgramFiles64Folder\n    children: [{ id: APPDIR, name: Hello, configurable: true }]\n" +
		"components: [{ id: App, directory: APPDIR, files: [{ source: hello.exe }] }]\n"

	for name, feature := range map[string]string{
		"feature directory": "features:\n  - { id: Main, directory: APPDIR, components: [App], firewall: [" + fmt.Sprintf(rule, "a") + "] }\n",
		"no directory":      "features:\n  - { id: Main, components: [App], firewall: [" + fmt.Sprintf(rule, "a") + "] }\n",
	} {
		cfg := parseValid(t, short+tree+feature)

		ds := config.Validate(cfg)
		if dirPath(cfg.Directories, config.DefaultInstallDirID) != nil || len(ds) != 0 {
			t.Errorf("%s: install dir added or diagnostics %v", name, ds)
		}

		if dir := cfg.Components[1].Directory; dir != "APPDIR" {
			t.Errorf("%s: carrier in %s, want APPDIR", name, dir)
		}
	}
}

func TestShorthandFeatureDefaultFromGo(t *testing.T) {
	t.Parallel()

	doc := short + "features:\n  - { id: F, components: [{ files: [{ source: hello.exe }] }] }\n"

	// A default set after parsing still sets the level.
	cfg := parseValid(t, doc)
	cfg.Features[0].Default = config.FeatureOff

	if err := config.Validate(cfg).Err(); err != nil || *cfg.Features[0].Level != 2 {
		t.Errorf("level %d, %v", *cfg.Features[0].Level, err)
	}

	// A level set after parsing is kept.
	cfg = parseValid(t, doc)
	cfg.Features[0].Level = new(3)

	if err := config.Validate(cfg).Err(); err != nil || *cfg.Features[0].Level != 3 {
		t.Errorf("level %d, %v", *cfg.Features[0].Level, err)
	}
}
