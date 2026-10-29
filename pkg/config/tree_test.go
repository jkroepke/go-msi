package config_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jkroepke/msi-builder/pkg/config"
)

func treeConfig() *config.Config {
	return &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "Tree", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package: config.Package{Platform: "x64"},
		Directories: []*config.Directory{{
			ID:       "ProgramFiles64Folder",
			Children: []*config.Directory{{ID: "INSTALLDIR", Name: "Tree"}},
		}},
		Features: []*config.Feature{{ID: "Main", Title: "Tree"}},
	}
}

func treeFS() fstest.MapFS {
	return fstest.MapFS{
		"dist/readme.txt":          {Data: []byte("hello")},
		"dist/conf/app.yaml":       {Data: []byte("a: 1")},
		"dist/conf/extra/b.yaml":   {Data: []byte("b: 2")},
		"dist/data/readme.txt":     {Data: []byte("same name, other folder")},
		"outside/not-included.txt": {Data: []byte("x")},
	}
}

func TestAddTree(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()

	comps, err := cfg.AddTree(treeFS(), "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"})
	if err != nil {
		t.Fatal(err)
	}

	if len(comps) != 4 || len(cfg.Components) != 4 || len(cfg.Features[0].Components) != 4 {
		t.Fatalf("got %d components, %d in config, %d in feature; want 4", len(comps), len(cfg.Components), len(cfg.Features[0].Components))
	}

	install := cfg.Directories[0].Children[0]

	var names []string
	for _, d := range install.Children {
		names = append(names, d.Name)

		for _, sub := range d.Children {
			names = append(names, d.Name+"/"+sub.Name)
		}
	}

	if got := strings.Join(names, ","); got != "conf,conf/extra,data" {
		t.Errorf("directories = %s, want conf,conf/extra,data", got)
	}

	for _, c := range comps {
		if len(c.Files) != 1 || c.Files[0].FS == nil || !strings.HasPrefix(c.Files[0].Source, "dist/") {
			t.Errorf("component %s: files %+v", c.ID, c.Files)
		}
	}

	if diags := config.Validate(cfg); diags.Err() != nil {
		t.Fatalf("validate: %v\n%s", diags.Err(), diags)
	}

	again := treeConfig()
	if _, err := again.AddTree(treeFS(), "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
		t.Fatal(err)
	}

	for i := range comps {
		if comps[i].ID != again.Components[i].ID || comps[i].Files[0].ID != again.Components[i].Files[0].ID {
			t.Errorf("ids differ between runs: %s/%s", comps[i].ID, again.Components[i].ID)
		}
	}
}

func TestAddTreeTwice(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()
	fsys := treeFS()

	if _, err := cfg.AddTree(fsys, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
		t.Fatal(err)
	}

	cfg.Directories[0].Children = append(cfg.Directories[0].Children, &config.Directory{ID: "OTHERDIR", Name: "Other"})

	if _, err := cfg.AddTree(fsys, "dist", "OTHERDIR", config.TreeOptions{Feature: "Main"}); err != nil {
		t.Fatal(err)
	}

	if diags := config.Validate(cfg); diags.Err() != nil {
		t.Fatalf("validate: %v\n%s", diags.Err(), diags)
	}
}

func TestAddTreeTwoRootsOneDirectory(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()
	fsys := fstest.MapFS{
		"a/conf/a.yaml": {Data: []byte("a")},
		"b/conf/b.yaml": {Data: []byte("b")},
		"b/x/y/z.txt":   {Data: []byte("z")},
	}

	for _, root := range []string{"a", "b"} {
		if _, err := cfg.AddTree(fsys, root, "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
			t.Fatal(err)
		}
	}

	if diags := config.Validate(cfg); diags.Err() != nil {
		t.Fatalf("validate: %v\n%s", diags.Err(), diags)
	}

	install := cfg.Directories[0].Children[0]
	if len(install.Children) != 2 || install.Children[0].Name != "conf" || len(install.Children[0].Children) != 0 {
		t.Errorf("want conf reused and x added below INSTALLDIR, got %+v", install.Children)
	}

	if x := install.Children[1]; x.Name != "x" || len(x.Children) != 1 || x.Children[0].Name != "y" {
		t.Errorf("want x/y for a folder without files of its own, got %+v", x)
	}
}

func TestAddTreeReusesDirectoryOfOtherCase(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()
	install := cfg.Directories[0].Children[0]
	install.Children = []*config.Directory{{ID: "CONFDIR", Name: "Conf"}}

	comps, err := cfg.AddTree(fstest.MapFS{"dist/conf/app.yaml": {Data: []byte("a")}}, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"})
	if err != nil {
		t.Fatal(err)
	}

	if len(install.Children) != 1 || comps[0].Directory != "CONFDIR" {
		t.Errorf("want the file in CONFDIR, got directory %s and children %+v", comps[0].Directory, install.Children)
	}
}

func TestValidateSameTargetPath(t *testing.T) {
	t.Parallel()

	// Two directory ids for INSTALLDIR\conf: a file in both is one file with
	// one automatic GUID, even when one of them is configurable.
	cfg := treeConfig()
	install := cfg.Directories[0].Children[0]
	install.Children = []*config.Directory{{ID: "CONFA", Name: "conf"}, {ID: "CONFB", Name: "Conf", Configurable: true}}
	cfg.Components = []*config.Component{
		{ID: "A", Directory: "CONFA", Files: []*config.File{{Source: "a/app.yaml"}}},
		{ID: "B", Directory: "CONFB", Files: []*config.File{{Source: "b/app.yaml", ID: "app2"}}},
	}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("A"), config.ComponentRef("B")}

	diags := config.Validate(cfg)
	if diags.Err() == nil || !strings.Contains(fmt.Sprint(diags), "installs the same file") {
		t.Errorf("want an error for one target path from two directories, got %v\n%s", diags.Err(), diags)
	}
}

func TestValidateExclusiveComponentsSamePath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		condB     string
		guidB     string
		wantError bool
	}{
		{"exclusive", "NOT VersionNT64", "3f2504e0-4f89-41d3-9a0c-0305e82c3402", false},
		{"unconditional", "", "3f2504e0-4f89-41d3-9a0c-0305e82c3402", true},
		{"automatic GUID", "NOT VersionNT64", config.Auto, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := treeConfig()
			cfg.Directories[0].Children = append(cfg.Directories[0].Children, &config.Directory{ID: "OTHER", Name: "Tree"})
			cfg.Components = []*config.Component{
				{ID: "A", Directory: "INSTALLDIR", Condition: "VersionNT64", GUID: "3f2504e0-4f89-41d3-9a0c-0305e82c3401", Files: []*config.File{{Source: "x64/a.txt"}}},
				{ID: "B", Directory: "OTHER", Condition: tc.condB, GUID: tc.guidB, Files: []*config.File{{Source: "x86/a.txt", ID: "a2"}}},
			}
			cfg.Features[0].Components = []*config.Component{config.ComponentRef("A"), config.ComponentRef("B")}

			diags := config.Validate(cfg)
			if (diags.Err() != nil) != tc.wantError || !strings.Contains(fmt.Sprint(diags), "installs the same file") {
				t.Errorf("err = %v, want error = %v and a diagnostic\n%s", diags.Err(), tc.wantError, diags)
			}
		})
	}
}

func TestValidateExclusiveNeedsEveryPair(t *testing.T) {
	t.Parallel()

	// C and D share a directory id, so they collide although each is
	// exclusive with B.
	cfg := treeConfig()
	cfg.Directories[0].Children = append(cfg.Directories[0].Children, &config.Directory{ID: "OTHER", Name: "tree"})
	cfg.Components = []*config.Component{
		{ID: "B", Directory: "INSTALLDIR", Condition: "X=1", GUID: "3f2504e0-4f89-41d3-9a0c-0305e82c3401", Files: []*config.File{{Source: "1/a.txt", ID: "a1"}}},
		{ID: "C", Directory: "OTHER", Condition: "X=2", GUID: "3f2504e0-4f89-41d3-9a0c-0305e82c3402", Files: []*config.File{{Source: "2/a.txt", ID: "a2"}}},
		{ID: "D", Directory: "OTHER", Condition: "X=3", GUID: "3f2504e0-4f89-41d3-9a0c-0305e82c3403", Files: []*config.File{{Source: "3/a.txt", ID: "a3"}}},
	}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("B"), config.ComponentRef("C"), config.ComponentRef("D")}

	if err := config.Validate(cfg).Err(); err == nil {
		t.Error("two components in one directory id install one file without an error")
	}
}

func TestValidateFileAtFolderPath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		roots []string
		fsys  fstest.MapFS
	}{
		{"two trees", []string{"a", "b"}, fstest.MapFS{"a/conf": {Data: []byte("x")}, "b/conf/app.yaml": {Data: []byte("a")}}},
		{"one tree", []string{"a"}, fstest.MapFS{"a/Conf": {Data: []byte("x")}, "a/conf/app.yaml": {Data: []byte("a")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := treeConfig()

			for _, root := range tc.roots {
				if _, err := cfg.AddTree(tc.fsys, root, "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
					t.Fatal(err)
				}
			}

			diags := config.Validate(cfg)
			if diags.Err() == nil || !strings.Contains(fmt.Sprint(diags), "has the same path as directory") {
				t.Errorf("want an error for a file at a folder path, got %v\n%s", diags.Err(), diags)
			}
		})
	}
}

func TestAddTreeRootNotInstalled(t *testing.T) {
	t.Parallel()

	// Only the path below root is installed, so root may use names that
	// Windows does not allow.
	comps, err := treeConfig().AddTree(fstest.MapFS{"build/aux/out./a.txt": {Data: []byte("a")}}, "build/aux/out.", "INSTALLDIR", config.TreeOptions{Feature: "Main"})
	if err != nil {
		t.Fatal(err)
	}

	if len(comps) != 1 || comps[0].Directory != "INSTALLDIR" {
		t.Errorf("components %+v", comps)
	}
}

func TestAddTreeShortNamesOnly(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()
	cfg.Package.LongFileNames = new(bool)

	if _, err := cfg.AddTree(treeFS(), "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err == nil || !strings.Contains(err.Error(), "longFileNames") {
		t.Errorf("err = %v, want longFileNames", err)
	}
}

func TestValidateNames(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"app.txt", true},
		{"COM10", true},
		{"console.txt", true},
		{"nullable.txt", true},
		{".nul", true},
		{"APP|application.txt", false},
		{"nul.txt", false},
		{"CONIN$", false},
		{"conout$.log", false},
		{"COM1", false},
		{"COM¹.log", false},
		{"lpt9.tar.gz", false},
		{"tab\there.txt", false},
		{"x\x01.txt", false},
		{"trail.", false},
		{"CON|config.txt", false},
		{"NUL.TXT|null.txt", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := treeConfig()
			cfg.Components = []*config.Component{{ID: "A", Directory: "INSTALLDIR", Files: []*config.File{{Source: "a.bin", ID: "a", Name: tc.name}}}}
			cfg.Features[0].Components = []*config.Component{config.ComponentRef("A")}

			if err := config.Validate(cfg).Err(); (err == nil) != tc.ok {
				t.Errorf("file name %q: err = %v, want ok = %v", tc.name, err, tc.ok)
			}

			dirs := treeConfig()
			dirs.Directories[0].Children[0].Children = []*config.Directory{{ID: "SUB", Name: tc.name}}
			dirs.Components = []*config.Component{{ID: "A", Directory: "SUB", Files: []*config.File{{Source: "a.bin"}}}}
			dirs.Features[0].Components = []*config.Component{config.ComponentRef("A")}

			if err := config.Validate(dirs).Err(); (err == nil) != tc.ok {
				t.Errorf("directory name %q: err = %v, want ok = %v", tc.name, err, tc.ok)
			}
		})
	}
}

func TestAddTreeErrorLeavesConfig(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()
	fsys := fstest.MapFS{"dist/a/ok.txt": {Data: []byte("a")}, "dist/b/x|y.txt": {Data: []byte("b")}}

	if _, err := cfg.AddTree(fsys, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err == nil {
		t.Fatal("no error for x|y.txt")
	}

	if n := len(cfg.Directories[0].Children[0].Children); n != 0 || len(cfg.Components) != 0 || len(cfg.Features[0].Components) != 0 {
		t.Errorf("config changed after an error: %d directories, %d components", n, len(cfg.Components))
	}
}

func TestAddTreeSameFilesTwice(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()

	for _, prefix := range []string{"one", "two"} {
		if _, err := cfg.AddTree(treeFS(), "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main", Prefix: prefix}); err != nil {
			t.Fatal(err)
		}
	}

	if diags := config.Validate(cfg); diags.Err() == nil {
		t.Error("two trees that install the same files validate")
	}
}

func TestAddTreeRootDot(t *testing.T) {
	t.Parallel()

	cfg := treeConfig()

	comps, err := cfg.AddTree(fstest.MapFS{"a.txt": {Data: []byte("a")}, "sub/b.txt": {Data: []byte("b")}}, ".", "INSTALLDIR", config.TreeOptions{Feature: "Main"})
	if err != nil {
		t.Fatal(err)
	}

	if len(comps) != 2 || comps[0].Directory != "INSTALLDIR" || comps[0].Files[0].Source != "a.txt" {
		t.Errorf("components %+v", comps)
	}
}

func TestAddTreeErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, root, dir string
		opts            config.TreeOptions
		want            string
	}{
		{"unknown directory", "dist", "NOPE", config.TreeOptions{Feature: "Main"}, "directory NOPE"},
		{"unknown feature", "dist", "INSTALLDIR", config.TreeOptions{Feature: "Nope"}, `feature "Nope"`},
		{"bad prefix", "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main", Prefix: "1x"}, "prefix"},
		{"bad root", "../dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "not a valid fs path"},
		{"missing root", "nope", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "nope"},
		{"long prefix", "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main", Prefix: "p" + strings.Repeat("x", config.MaxTreePrefixLength)}, "at most 50"},
		{"root is a file", "dist/readme.txt", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "not a directory"},
		{"symlink", "links", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "not a regular file"},
		{"pipe in file name", "bad", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, `"x|y.txt" is not a valid Windows file name`},
		{"colon in folder name", "colon", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, `"a:b" is not a valid Windows file name`},
		{"trailing dot", "dot", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, `"trail." is not a valid Windows file name`},
		{"device name", "device", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, `"nul.txt" is not a valid Windows file name`},
		{"superscript device name", "super", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, `"LPT².log" is not a valid Windows file name`},
		{"drive root", "C:", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "contains a colon"},
		{"backslash in root", `my\dist`, "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "not a valid fs path"},
		{"empty root", "", "INSTALLDIR", config.TreeOptions{Feature: "Main"}, "root is empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fsys := treeFS()
			fsys["links/l.txt"] = &fstest.MapFile{Data: []byte("readme.txt"), Mode: fs.ModeSymlink}
			fsys["bad/x|y.txt"] = &fstest.MapFile{Data: []byte("x")}
			fsys["colon/a:b/c.txt"] = &fstest.MapFile{Data: []byte("c")}
			fsys["dot/trail./x.txt"] = &fstest.MapFile{Data: []byte("x")}
			fsys["device/nul.txt"] = &fstest.MapFile{Data: []byte("x")}
			fsys["super/LPT².log"] = &fstest.MapFile{Data: []byte("x")}
			fsys["C:/Windows/win.ini"] = &fstest.MapFile{Data: []byte("x")}

			_, err := treeConfig().AddTree(fsys, tc.root, tc.dir, tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestAddTreeIntoDefaultFeature(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "Hello", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package: config.Package{Platform: "x64"},
	}
	config.ApplyDefaults(cfg)

	if _, err := cfg.AddTree(treeFS(), "dist", cfg.InstallDir.ID, config.TreeOptions{}); err != nil {
		t.Fatal(err)
	}

	if diags := config.Validate(cfg); diags.Err() != nil {
		t.Fatalf("validate: %v\n%s", diags.Err(), diags)
	}

	if len(cfg.Features) != 1 || cfg.Features[0].ID != config.DefaultFeatureID || len(cfg.Features[0].Components) != 4 {
		t.Errorf("features %+v, want the default feature with 4 components", cfg.Features)
	}
}

func TestAddTreeEmptyFeature(t *testing.T) {
	t.Parallel()

	// One feature, here the default one after ApplyDefaults: an empty
	// Feature means it.
	cfg := &config.Config{
		Schema:     config.SchemaV1,
		Product:    config.Product{Name: "Hello", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package:    config.Package{Platform: "x64"},
		Components: []*config.Component{{Files: []*config.File{{Source: "own.txt"}}}},
	}
	config.ApplyDefaults(cfg)

	if _, err := cfg.AddTree(treeFS(), "dist", cfg.InstallDir.ID, config.TreeOptions{}); err != nil {
		t.Fatal(err)
	}

	if len(cfg.Features) != 1 || len(cfg.Features[0].Components) != 5 {
		t.Errorf("features %+v, want one feature with 5 components", cfg.Features)
	}

	// Two features: the caller has to choose.
	two := treeConfig()
	two.Features[0].Features = []*config.Feature{{ID: "Sub", Title: "Sub"}}

	if _, err := two.AddTree(treeFS(), "dist", "INSTALLDIR", config.TreeOptions{}); err == nil || !strings.Contains(err.Error(), "2 features") {
		t.Errorf("err = %v, want a hint to set Feature", err)
	}
}
