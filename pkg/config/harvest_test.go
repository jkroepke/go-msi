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

const harvestHeader = `schema: msi-builder/v1
product: { name: Harvest, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
`

func harvestFS() fstest.MapFS {
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

// harvest parses a config with the header and the given harvest section,
// reads from fsys and returns it with the target paths it adds below the
// install dir, sorted.
func harvest(t *testing.T, fsys fs.FS, section string) (*config.Config, config.Diagnostics, []string) {
	t.Helper()

	cfg, err := config.Parse([]byte(harvestHeader+section), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.FS = fsys

	if diags := config.Validate(cfg); diags.Err() != nil {
		return cfg, diags, nil
	}

	diags := cfg.HarvestFiles()

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
			out = append(out, paths[comp.Directory]+file.Name)
		}
	}

	slices.Sort(out)

	return out
}

func TestHarvestPatterns(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, section string
		want          []string
	}{
		{
			"exclude names in every folder and whole folders",
			"harvest: [{ source: dist, exclude: ['*.pdb', logs/, cache] }]",
			[]string{"app.exe", "conf/app.yaml", "lib/a.DLL", "lib/sub/b.dll", "readme.txt"},
		},
		{
			"include ignores case and keeps folders",
			"harvest: [{ source: dist, include: '*.dll' }]",
			[]string{"lib/a.DLL", "lib/sub/b.dll"},
		},
		{
			"anchored include with **",
			"harvest: [{ source: dist, include: 'lib/**/*.dll', exclude: 'lib/sub/' }]",
			[]string{"lib/a.DLL"},
		},
		{
			"include a folder",
			"harvest: [{ source: dist, include: [conf, readme.txt] }]",
			[]string{"conf/app.yaml", "readme.txt"},
		},
		{
			"alternatives in any folder",
			"harvest: [{ source: dist, include: '{readme,app}.*', exclude: app.exe }]",
			[]string{"conf/app.yaml", "debug/app.pdb", "readme.txt"},
		},
		{
			"** in the middle matches no folder too",
			"harvest: [{ source: dist, include: 'lib/**/b.dll' }]",
			[]string{"lib/sub/b.dll"},
		},
		{
			"into a folder path",
			"harvest: [{ source: dist/conf, directory: etc/app }]",
			[]string{"etc/app/app.yaml"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, diags, got := harvest(t, harvestFS(), tc.section)
			if diags.Err() != nil {
				t.Fatalf("%v\n%s", diags.Err(), diags)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("files = %q, want %q", got, tc.want)
			}

			if diags := config.Validate(cfg); diags.Err() != nil {
				t.Errorf("validate after harvest: %v\n%s", diags.Err(), diags)
			}
		})
	}
}

func TestHarvestErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, section, path, want string
	}{
		{"special file", "harvest: [{ source: dist/logs }]", "harvest[0]", "not a regular file"},
		{"bad name", "harvest: [{ source: dist, exclude: [logs] }]", "harvest[0]", `"bad|name.txt" is not a valid Windows file name`},
		{"no match", "harvest: [{ source: dist, include: '*.msi' }]", "harvest[0]", "matches the include and exclude patterns"},
		{"missing folder", "harvest: [{ source: nope }]", "harvest[0]", "nope"},
		{"backslash", `harvest: [{ source: dist, exclude: ['logs\*'] }]`, "harvest[0].exclude[0]", `use "/"`},
		{"leading slash", "harvest: [{ source: dist, include: [/conf] }]", "harvest[0].include[0]", "leading /"},
		{"dot dot", "harvest: [{ source: dist, include: [../x] }]", "harvest[0].include[0]", `".."`},
		{"alternatives", "harvest: [{ source: dist, include: ['{a'] }]", "harvest[0].include[0]", "bad pattern"},
		{"bad syntax", "harvest: [{ source: dist, include: ['[a'] }]", "harvest[0].include[0]", "bad pattern"},
		{"no source", "harvest: [{ exclude: x }]", "harvest[0].source", "required"},
		{"unknown directory", "harvest: [{ source: dist, directory: NOPE }]", "harvest[0].directory", `"NOPE" is not defined`},
		{"unknown feature", "harvest: [{ source: dist/conf, feature: Nope }]", "harvest[0].feature", `"Nope" is not defined`},
		{"empty entry", "harvest: [~]", "harvest[0]", "empty entry"},
		{
			"two features", "features: [{ id: A, title: A }, { id: B, title: B }]\nharvest: [{ source: dist/conf }]",
			"harvest[0].feature", "2 features",
		},
		{
			"short names", "package: { platform: x64, longFileNames: false }\nharvest: [{ source: dist/conf }]",
			"harvest[0]", "longFileNames is false",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A section with its own package mapping replaces the header's.
			text := harvestHeader + tc.section
			if strings.HasPrefix(tc.section, "package:") {
				text = strings.Replace(harvestHeader, "package: { platform: x64 }\n", "", 1) + tc.section
			}

			cfg, err := config.Parse([]byte(text), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.FS = harvestFS()

			diags := config.Validate(cfg)
			if diags.Err() == nil {
				diags = cfg.HarvestFiles()
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

func TestHarvestMatchesAddTree(t *testing.T) {
	t.Parallel()

	cfg, diags, _ := harvest(t, harvestFS(), "harvest: [{ source: dist/conf }]")
	if diags.Err() != nil {
		t.Fatal(diags)
	}

	tree := &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "Harvest", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package: config.Package{Platform: "x64"},
	}
	config.ApplyDefaults(tree)

	comps, err := tree.AddTree(harvestFS(), "dist/conf", tree.InstallDir.ID, config.TreeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Components[0].ID != comps[0].ID || cfg.Components[0].Files[0].ID != comps[0].Files[0].ID {
		t.Errorf("harvest ids %s/%s differ from AddTree's %s/%s", cfg.Components[0].ID, cfg.Components[0].Files[0].ID, comps[0].ID, comps[0].Files[0].ID)
	}

	if got := cfg.Components[0].Path(0); got != "harvest[0]{app.yaml}" {
		t.Errorf("path = %s, want harvest[0]{app.yaml}", got)
	}

	// A second call adds nothing.
	if diags := cfg.HarvestFiles(); diags.Err() != nil || len(cfg.Components) != 1 {
		t.Errorf("second HarvestFiles: %v, %d components", diags.Err(), len(cfg.Components))
	}
}

func TestHarvestDisk(t *testing.T) {
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
		if err := os.WriteFile(yamlPath, []byte(harvestHeader+"harvest: [{ source: '"+src+"', allowUnversionedExecutable: true }]\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(yamlPath, config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		if diags := cfg.HarvestFiles(); diags.Err() != nil {
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

func TestHarvestFilesWithoutValidate(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, section, want string }{
		{"unknown directory", "harvest: [{ source: dist/conf, directory: NOPE }]", `directory "NOPE" is not defined`},
		{"two features", "features: [{ id: A, title: A }, { id: B, title: B }]\nharvest: [{ source: dist/conf }]", "feature is required: the package has 2 features"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Parse([]byte(harvestHeader+tc.section), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.FS = harvestFS()

			diags := cfg.HarvestFiles()
			if len(diags) != 1 || diags[0].Path != "harvest[0]" || diags[0].Message != tc.want {
				t.Errorf("diagnostics %s, want harvest[0]: %s", diags, tc.want)
			}
		})
	}
}

func TestHarvestFilesRetriesFailedEntries(t *testing.T) {
	t.Parallel()

	fsys := harvestFS()

	cfg, diags, got := harvest(t, fsys, "harvest: [{ source: dist/missing }, { source: dist/conf, bitness: '32' }]")
	if len(diags) != 1 || diags[0].Path != "harvest[0]" || !slices.Equal(got, []string{"app.yaml"}) {
		t.Fatalf("diagnostics %s and files %q, want harvest[0] to fail and app.yaml from harvest[1]", diags, got)
	}

	if cfg.Components[0].Bitness != "32" {
		t.Errorf("bitness = %q, want 32 from the entry", cfg.Components[0].Bitness)
	}

	// Once the folder exists, the next call adds only the failed entry.
	fsys["dist/missing/late.txt"] = &fstest.MapFile{Data: []byte("late")}

	if diags := cfg.HarvestFiles(); diags.Err() != nil {
		t.Fatal(diags)
	}

	if got := targets(cfg); !slices.Equal(got, []string{"app.yaml", "late.txt"}) {
		t.Errorf("files after the retry = %q", got)
	}
}

func TestHarvestBitness(t *testing.T) {
	t.Parallel()

	_, diags, _ := harvest(t, harvestFS(), "harvest: [{ source: dist/conf, bitness: '16' }]")
	if diags.Err() == nil || diags[0].Path != "harvest[0].bitness" {
		t.Errorf("want an error at harvest[0].bitness, got %s", diags)
	}
}

func TestHarvestWarnsAboutYAMLFolder(t *testing.T) {
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

		if err := os.WriteFile(yamlPath, []byte(harvestHeader+"harvest: [{ source: '"+tc.source+"' }]\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(yamlPath, config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		diags := cfg.HarvestFiles()
		if diags.Err() != nil {
			t.Fatalf("%s: %s", tc.source, diags)
		}

		if warned := len(diags) == 1 && diags[0].Path == "harvest[0].source"; warned != tc.warn {
			t.Errorf("source %s: diagnostics %s, want a warning %v", tc.source, diags, tc.warn)
		}
	}
}

func TestHarvestWarnsWithRelativeBaseDir(t *testing.T) {
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
		cfg, err := config.Parse([]byte(harvestHeader+"harvest: [{ source: '"+src+"' }]\n"), config.Options{})
		if err != nil {
			t.Fatal(err)
		}

		cfg.BaseDir = base

		diags := cfg.HarvestFiles()
		if len(diags) != 1 || diags[0].Severity != config.SeverityWarning || diags[0].Path != "harvest[0].source" {
			t.Errorf("source %s: diagnostics %s, want one warning at harvest[0].source", src, diags)
		}
	}
}

func TestHarvest32BitFolderWarnsOnce(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		bitness string
		want    int
	}{{"", 1}, {"32", 0}} {
		section := "harvest: [{ source: dist/lib, directory: ProgramFilesFolder/Harvest"
		if tc.bitness != "" {
			section += ", bitness: '" + tc.bitness + "'"
		}

		cfg, diags, _ := harvest(t, harvestFS(), section+" }]\n")
		if diags.Err() != nil {
			t.Fatal(diags)
		}

		warnings := 0

		for _, d := range config.Validate(cfg) {
			if strings.Contains(d.Message, "ICE80") {
				warnings++

				if d.Path != "harvest[0].directory" {
					t.Errorf("ICE80 warning at %s, want harvest[0].directory", d.Path)
				}
			}
		}

		if warnings != tc.want {
			t.Errorf("bitness %q: %d ICE80 warnings, want %d", tc.bitness, warnings, tc.want)
		}
	}
}
