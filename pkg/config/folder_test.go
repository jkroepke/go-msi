package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jkroepke/msi-builder/pkg/config"
)

const folderHeader = `schema: msi-builder/v1
product: { name: Sample, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
`

func sampleFS() fstest.MapFS {
	return fstest.MapFS{
		"dist/app.exe":            {Data: []byte("MZ")},
		"dist/readme.txt":         {Data: []byte("hello")},
		"dist/conf/app.yaml":      {Data: []byte("a: 1")},
		"dist/debug/app.pdb":      {Data: []byte("pdb")},
		"dist/lib/a.DLL":          {Data: []byte("MZ")},
		"dist/lib/sub/b.dll":      {Data: []byte("MZ")},
		"dist/lib/sub/b.pdb":      {Data: []byte("pdb")},
		"dist/logs/today.log":     {Data: []byte("log")},
		"dist/logs/link":          {Data: []byte("today.log"), Mode: fs.ModeSymlink},
		"dist/cache/bad|name.txt": {Data: []byte("x")},
	}
}

// expand parses a config with the header and the given section,
// reads from fsys and returns it with the target paths it adds below the
// install dir, sorted.
func expand(t *testing.T, fsys fs.FS, section string) (*config.Config, config.Diagnostics, []string) {
	t.Helper()

	cfg, err := config.Parse([]byte(folderHeader+section), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.FS = fsys

	if diags := config.Validate(cfg); diags.Err() != nil {
		return cfg, diags, nil
	}

	diags := cfg.ExpandFolders()

	return cfg, diags, targets(cfg)
}

// targets lists the install paths of the config's files below the install
// dir.
func targets(cfg *config.Config) []string {
	paths := make(map[string]string)

	var walk func([]*config.Directory, string)

	walk = func(dirs []*config.Directory, prefix string) {
		for _, dir := range dirs {
			dirPath := prefix + dir.Name + "/"
			if dir.ID == cfg.InstallDir.ID {
				dirPath = ""
			}

			paths[dir.ID] = dirPath
			walk(dir.Children, dirPath)
		}
	}

	walk(cfg.Directories, "")

	var out []string

	for _, comp := range cfg.Components {
		for _, file := range comp.Files {
			if strings.HasSuffix(file.Source, "/") {
				continue // a folder that failed to expand
			}

			out = append(out, paths[comp.Directory]+file.Name)
		}
	}

	slices.Sort(out)

	return out
}

func TestFolderPatterns(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, section string
		want          []string
	}{
		{
			"exclude names in every folder and whole folders",
			"components: [{ files: [{ source: dist/, exclude: ['*.pdb', logs/, cache] }] }]",
			[]string{"app.exe", "conf/app.yaml", "lib/a.DLL", "lib/sub/b.dll", "readme.txt"},
		},
		{
			"include ignores case and keeps folders",
			"components: [{ files: [{ source: dist/, include: '*.dll' }] }]",
			[]string{"lib/a.DLL", "lib/sub/b.dll"},
		},
		{
			"anchored include with **",
			"components: [{ files: [{ source: dist/, include: 'lib/**/*.dll', exclude: 'lib/sub/' }] }]",
			[]string{"lib/a.DLL"},
		},
		{
			"include a folder",
			"components: [{ files: [{ source: dist/, include: [conf, readme.txt] }] }]",
			[]string{"conf/app.yaml", "readme.txt"},
		},
		{
			"alternatives in any folder",
			"components: [{ files: [{ source: dist/, include: '{readme,app}.*', exclude: app.exe }] }]",
			[]string{"conf/app.yaml", "debug/app.pdb", "readme.txt"},
		},
		{
			"** in the middle matches no folder too",
			"components: [{ files: [{ source: dist/, include: 'lib/**/a.dll' }] }]",
			[]string{"lib/a.DLL"},
		},
		{
			"** in the middle matches one folder",
			"components: [{ files: [{ source: dist/, include: 'lib/**/b.dll' }] }]",
			[]string{"lib/sub/b.dll"},
		},
		{
			"into a folder path",
			"components: [{ directory: etc/app, files: [{ source: dist/conf/ }] }]",
			[]string{"etc/app/app.yaml"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, diags, got := expand(t, sampleFS(), tc.section)
			if diags.Err() != nil {
				t.Fatalf("%v\n%s", diags.Err(), diags)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("files = %q, want %q", got, tc.want)
			}

			if diags := config.Validate(cfg); diags.Err() != nil {
				t.Errorf("validate after expanding: %v\n%s", diags.Err(), diags)
			}
		})
	}
}

func TestFolderErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, section, path, want string
	}{
		{"special file", "components: [{ files: [{ source: dist/logs/ }] }]", "components[0].files[0].source", "not a regular file"},
		{"bad name", "components: [{ files: [{ source: dist/, exclude: [logs] }] }]", "components[0].files[0].source", `"bad|name.txt" is not a valid Windows file name`},
		{"no match", "components: [{ files: [{ source: dist/, include: '*.msi' }] }]", "components[0].files[0].source", "matches the include and exclude patterns"},
		{"missing folder", "components: [{ files: [{ source: nope/ }] }]", "components[0].files[0].source", "nope"},
		{"backslash", `components: [{ files: [{ source: dist/, exclude: ['logs\*'] }] }]`, "components[0].files[0].exclude[0]", `use "/"`},
		{"leading slash", "components: [{ files: [{ source: dist/, include: [/conf] }] }]", "components[0].files[0].include[0]", "leading /"},
		{"dot dot", "components: [{ files: [{ source: dist/, include: [../x] }] }]", "components[0].files[0].include[0]", `".."`},
		{"alternatives", "components: [{ files: [{ source: dist/, include: ['{a'] }] }]", "components[0].files[0].include[0]", "bad pattern"},
		{"bad syntax", "components: [{ files: [{ source: dist/, include: ['[a'] }] }]", "components[0].files[0].include[0]", "bad pattern"},
		{"patterns on a file", "components: [{ files: [{ source: dist/app.exe, exclude: x }] }]", "components[0].files[0]", "need a folder source"},
		{"empty folder", "components: [{ files: [{ source: / }] }]", "components[0].files[0].source", "empty"},
		{"second file", "components: [{ files: [{ source: dist/conf/ }, { source: dist/app.exe }] }]", "components[0].files", "only file"},
		{"pinned guid", "components: [{ guid: 3f2504e0-4f89-41d3-9a0c-0305e82c3302, files: [{ source: dist/conf/ }] }]", "components[0].guid", "not allowed"},
		{"registry", "components: [{ registry: [{ root: HKLM, key: Software/X }], files: [{ source: dist/conf/ }] }]", "components[0].registry", "not allowed"},
		{"file name", "components: [{ files: [{ source: dist/conf/, name: x }] }]", "components[0].files[0].name", "not allowed"},
		{"version", "components: [{ files: [{ source: dist/conf/, version: 1.0.0 }] }]", "components[0].files[0].version", "not allowed"},
		{"shortcut", "components: [{ files: [{ source: dist/conf/, shortcuts: [{ name: X }] }] }]", "components[0].files[0].shortcuts", "not allowed"},
		{"unknown directory", "components: [{ directory: NOPE, files: [{ source: dist/ }] }]", "components[0].directory", `"NOPE" is not defined`},
		{
			"short names", "package: { platform: x64, longFileNames: false }\ncomponents: [{ files: [{ source: dist/conf/ }] }]",
			"components[0].files", "longFileNames is false",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A section with its own package mapping replaces the header's.
			text := folderHeader + tc.section
			if strings.HasPrefix(tc.section, "package:") {
				text = strings.Replace(folderHeader, "package: { platform: x64 }\n", "", 1) + tc.section
			}

			cfg, err := config.Parse([]byte(text), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.FS = sampleFS()

			diags := config.Validate(cfg)
			if diags.Err() == nil {
				diags = cfg.ExpandFolders()
			}

			for _, d := range diags {
				if d.Severity == config.SeverityError && d.Path == tc.path && strings.Contains(d.Message, tc.want) {
					return
				}
			}

			t.Errorf("want an error at %s mentioning %q, got\n%s", tc.path, tc.want, diags)
		})
	}
}

func TestFolderMatchesAddTree(t *testing.T) {
	t.Parallel()

	cfg, diags, _ := expand(t, sampleFS(), "components: [{ files: [{ source: dist/conf/ }] }]")
	if diags.Err() != nil {
		t.Fatal(diags)
	}

	tree := &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "Sample", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package: config.Package{Platform: "x64"},
	}
	config.ApplyDefaults(tree)

	comps, err := tree.AddTree(sampleFS(), "dist/conf", tree.InstallDir.ID, config.TreeOptions{Prefix: "conf"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Components[0].ID != comps[0].ID || cfg.Components[0].Files[0].ID != comps[0].Files[0].ID {
		t.Errorf("folder ids %s/%s differ from AddTree's %s/%s", cfg.Components[0].ID, cfg.Components[0].Files[0].ID, comps[0].ID, comps[0].Files[0].ID)
	}

	if got := cfg.Components[0].Path(0); got != "components[0]{app.yaml}" {
		t.Errorf("path = %s, want components[0]{app.yaml}", got)
	}

	// A second call adds nothing.
	if diags := cfg.ExpandFolders(); diags.Err() != nil || len(cfg.Components) != 1 {
		t.Errorf("second ExpandFolders: %v, %d components", diags.Err(), len(cfg.Components))
	}
}

func TestFolderDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	for _, name := range []string{"dist/bin/app.txt", "dist/readme.txt"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	yamlDir := filepath.Join(root, "packaging")
	if err := os.Mkdir(yamlDir, 0o750); err != nil {
		t.Fatal(err)
	}

	ids := make([][]string, 0, 2)

	// A source outside the YAML file's folder, and an absolute one.
	for _, src := range []string{"../dist", filepath.Join(root, "dist")} {
		yamlPath := filepath.Join(yamlDir, "msi.yaml")
		if err := os.WriteFile(yamlPath, []byte(folderHeader+"components: [{ files: [{ source: '"+src+"/', allowUnversionedExecutable: true }] }]\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(yamlPath, config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		if diags := cfg.ExpandFolders(); diags.Err() != nil {
			t.Fatalf("%s: %s", src, diags)
		}

		if got := targets(cfg); !slices.Equal(got, []string{"bin/app.txt", "readme.txt"}) {
			t.Errorf("%s: files = %q", src, got)
		}

		comps := make([]string, 0, len(cfg.Components))

		for _, comp := range cfg.Components {
			if !comp.Files[0].AllowUnversionedExecutable {
				t.Errorf("%s: allowUnversionedExecutable not set on %s", src, comp.Files[0].Source)
			}

			f, err := comp.Files[0].FS.Open(comp.Files[0].Source)
			if err != nil {
				t.Errorf("%s: %v", src, err)
			} else {
				_ = f.Close()
			}

			comps = append(comps, comp.ID)
		}

		ids = append(ids, comps)
	}

	// The ids come from the source as written.
	if slices.Equal(ids[0], ids[1]) {
		t.Error("a relative and an absolute source gave the same ids")
	}
}

func TestExpandFoldersWithoutValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, section, want string }{
		{"unknown directory", "components: [{ directory: NOPE, files: [{ source: dist/conf/ }] }]", `directory "NOPE" is not defined`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Parse([]byte(folderHeader+tc.section), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.FS = sampleFS()

			diags := cfg.ExpandFolders()
			if len(diags) != 1 || diags[0].Path != "components[0].files[0].source" || diags[0].Message != tc.want {
				t.Errorf("diagnostics %s, want components[0].files[0].source: %s", diags, tc.want)
			}
		})
	}
}

func TestExpandFoldersRetriesFailedEntries(t *testing.T) {
	t.Parallel()

	fsys := sampleFS()

	cfg, diags, got := expand(t, fsys, "components: [{ files: [{ source: dist/missing/ }] }, { bitness: '32', files: [{ source: dist/conf/ }] }]")
	if len(diags) != 1 || diags[0].Path != "components[0].files[0].source" || !slices.Equal(got, []string{"app.yaml"}) {
		t.Fatalf("diagnostics %s and files %q, want components[0] to fail and app.yaml from components[1]", diags, got)
	}

	if cfg.Components[1].Bitness != "32" || cfg.Components[1].Path(1) != "components[1]{app.yaml}" {
		t.Errorf("bitness %q at %s, want 32 from components[1]", cfg.Components[1].Bitness, cfg.Components[1].Path(1))
	}

	// Once the folder exists, the next call expands only the failed folder.
	fsys["dist/missing/late.txt"] = &fstest.MapFile{Data: []byte("late")}

	if diags := cfg.ExpandFolders(); diags.Err() != nil {
		t.Fatal(diags)
	}

	if got := targets(cfg); !slices.Equal(got, []string{"app.yaml", "late.txt"}) {
		t.Errorf("files after the retry = %q", got)
	}
}

func TestFolderBitness(t *testing.T) {
	t.Parallel()

	_, diags, _ := expand(t, sampleFS(), "components: [{ bitness: '16', files: [{ source: dist/conf/ }] }]")
	if diags.Err() == nil || diags[0].Path != "components[0].bitness" {
		t.Errorf("want an error at components[0].bitness, got %s", diags)
	}
}

func TestFolderWarnsAboutYAMLFolder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		source string
		warn   bool
	}{{".", true}, {"sub/..", true}, {"out", false}} {
		yamlPath := filepath.Join(dir, "msi.yaml")
		if err := os.MkdirAll(filepath.Join(dir, "out"), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, "out", "a.txt"), []byte("a"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(yamlPath, []byte(folderHeader+"components: [{ files: [{ source: '"+tc.source+"/' }] }]\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(yamlPath, config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		diags := cfg.ExpandFolders()
		if diags.Err() != nil {
			t.Fatalf("%s: %s", tc.source, diags)
		}

		if warned := len(diags) == 1 && diags[0].Path == "components[0].files[0].source"; warned != tc.warn {
			t.Errorf("source %s: diagnostics %s, want a warning %v", tc.source, diags, tc.warn)
		}
	}
}

func TestFolderWarnsWithRelativeBaseDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	yamlDir := filepath.Join(root, "packaging")

	if err := os.Mkdir(yamlDir, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(yamlDir, "msi.yaml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Load sets BaseDir as the YAML path was given, often relative.
	base, err := filepath.Rel(cwd, yamlDir)
	if err != nil {
		t.Skip("the temporary folder has no relative path from here")
	}

	for _, src := range []string{"..", root} {
		cfg, err := config.Parse([]byte(folderHeader+"components: [{ files: [{ source: '"+src+"/' }] }]\n"), config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		cfg.BaseDir = base

		diags := cfg.ExpandFolders()
		if len(diags) != 1 || diags[0].Severity != config.SeverityWarning || diags[0].Path != "components[0].files[0].source" {
			t.Errorf("source %s: diagnostics %s, want one warning at components[0].files[0].source", src, diags)
		}
	}
}

func TestFolder32BitFolderWarnsOnce(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		bitness string
		want    int
	}{{"", 1}, {"32", 0}} {
		section := "components: [{ directory: ProgramFilesFolder/Sample"
		if tc.bitness != "" {
			section += ", bitness: '" + tc.bitness + "'"
		}

		cfg, diags, _ := expand(t, sampleFS(), section+", files: [{ source: dist/lib/ }] }]\n")
		if diags.Err() != nil {
			t.Fatal(diags)
		}

		warnings := 0

		for _, d := range config.Validate(cfg) {
			if strings.Contains(d.Message, "ICE80") {
				warnings++

				if d.Path != "components[0].directory" {
					t.Errorf("ICE80 warning at %s, want components[0].directory", d.Path)
				}
			}
		}

		if warnings != tc.want {
			t.Errorf("bitness %q: %d ICE80 warnings, want %d", tc.bitness, warnings, tc.want)
		}
	}
}

func TestFolderTakesTheComponentsPlace(t *testing.T) {
	t.Parallel()

	cfg, diags, _ := expand(t, sampleFS(), `features:
  - id: Main
    title: Main
    components:
      - { id: First, files: [{ source: dist/app.exe }] }
      - id: Conf
        condition: INSTALLCONF
        permanent: true
        bitness: '32'
        files: [{ source: dist/conf/, readOnly: true, vital: false, allowUnversionedExecutable: true }]
      - { id: Last, files: [{ source: dist/readme.txt }] }
`)
	if diags.Err() != nil {
		t.Fatal(diags)
	}

	ids := make([]string, 0, len(cfg.Components))
	paths := make([]string, 0, len(cfg.Components))

	for i, comp := range cfg.Components {
		ids = append(ids, comp.ID)
		paths = append(paths, comp.Path(i))
	}

	wantPaths := []string{"features[0].components[0]", "features[0].components[1]{app.yaml}", "features[0].components[2]"}
	if !slices.Equal(paths, wantPaths) {
		t.Errorf("paths = %q, want %q", paths, wantPaths)
	}

	refs := make([]string, 0, len(cfg.Features[0].Components))

	for _, ref := range cfg.Features[0].Components {
		refs = append(refs, ref.ID)
	}

	if !slices.Equal(refs, ids) || !strings.HasPrefix(ids[1], "Conf_c") {
		t.Errorf("feature components = %q, want %q with a Conf_c id in the middle", refs, ids)
	}

	conf := cfg.Components[1]
	file := conf.Files[0]

	if conf.Condition != "INSTALLCONF" || !conf.Permanent || conf.Bitness != "32" || conf.GUID != config.Auto {
		t.Errorf("component settings not copied: %+v", conf)
	}

	if !file.ReadOnly || file.Vital == nil || *file.Vital || !file.AllowUnversionedExecutable || file.Name != "app.yaml" {
		t.Errorf("file settings not copied: %+v", file)
	}

	if diags := config.Validate(cfg); diags.Err() != nil {
		t.Errorf("validate after expanding: %s", diags)
	}
}

// bothPasses validates a config, expands its folders and validates it
// again, as the builder does, and returns every diagnostic.
func bothPasses(t *testing.T, text string) (config.Diagnostics, config.Diagnostics) {
	t.Helper()

	cfg, err := config.Parse([]byte(text), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.FS = sampleFS()

	first := config.Validate(cfg)
	if expanded := cfg.ExpandFolders(); expanded.Err() != nil {
		t.Fatal(expanded)
	}

	return first, config.Validate(cfg)
}

func TestFolderReportsOncePerComponent(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, text, path, want string
	}{
		{
			"firewall rule on the folder",
			folderHeader + "components: [{ id: App, files: [{ source: dist/conf/ }] }]\n" +
				"firewall: [{ name: r, component: App, program: '[#x]', port: '80' }]\n",
			"firewall[0].component", "folder source",
		},
		{
			"preserve with an early removal",
			folderHeader + "upgrade: { removeOld: afterInstallValidate }\n" +
				"components: [{ preserve: true, files: [{ source: dist/lib/ }] }]\n",
			"components[0].preserve", "needs removeOld afterInstallExecute",
		},
		{
			"user profile",
			folderHeader + "components: [{ directory: AppDataFolder/Sample, files: [{ source: dist/lib/ }] }]\n",
			"components[0].directory", "user profile",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			first, second := bothPasses(t, tc.text)

			for pass, diags := range []config.Diagnostics{first, second} {
				count := 0

				for _, d := range diags {
					if strings.Contains(d.Message, tc.want) {
						count++

						if d.Path != tc.path {
							t.Errorf("pass %d: %q at %s, want %s", pass+1, tc.want, d.Path, tc.path)
						}
					}
				}

				if count != 1 {
					t.Errorf("pass %d: %d diagnostics mentioning %q, want 1\n%s", pass+1, count, tc.want, diags)
				}
			}
		})
	}
}

func TestFolderOnDiskNamesTheFolder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ source, want string }{
		{"nothere/", "folder nothere/: "},
		{"file.txt/", "file.txt/ is not a folder"},
	} {
		cfg, err := config.Parse([]byte(folderHeader+"components: [{ files: [{ source: '"+tc.source+"' }] }]\n"), config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		cfg.BaseDir = dir

		diags := cfg.ExpandFolders()
		if len(diags) != 1 || !strings.HasPrefix(diags[0].Message, tc.want) {
			t.Errorf("%s: diagnostics %s, want one starting with %q", tc.source, diags, tc.want)
		}
	}
}

func TestFolderChecksMovedComponents(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(folderHeader+"components: [{ files: [{ source: dist/conf/ }] }]\n"), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.FS = sampleFS()

	if diags := cfg.ExpandFolders(); diags.Err() != nil {
		t.Fatal(diags)
	}

	// A generated component that Go code moves to a 32-bit folder is
	// checked like any other.
	cfg.Directories = append(cfg.Directories, &config.Directory{ID: "ProgramFilesFolder", Children: []*config.Directory{{ID: "OLD", Name: "Old"}}})
	cfg.Components[0].Directory = "OLD"

	for _, d := range config.Validate(cfg) {
		if strings.Contains(d.Message, "ICE80") {
			return
		}
	}

	t.Error("no ICE80 warning for a generated component moved to ProgramFilesFolder")
}
