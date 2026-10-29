package builder_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/cab"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// goConfig is the minimal example written in Go instead of YAML.
func goConfig() *config.Config {
	return &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "Hello", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package: config.Package{Platform: "x64"},
		Upgrade: &config.Upgrade{},
		Directories: []*config.Directory{{
			ID:       "ProgramFiles64Folder",
			Children: []*config.Directory{{ID: "INSTALLDIR", Name: "Hello", Configurable: true}},
		}},
		Features: []*config.Feature{{ID: "Main", Title: "Hello"}},
	}
}

// cabinetFiles returns the payload of every cabinet entry by name.
func cabinetFiles(t *testing.T, res *builder.Result) map[string][]byte {
	t.Helper()

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	data, ok := database.Stream("Hello.cab")
	if !ok {
		t.Fatal("cabinet stream missing")
	}

	entries, err := cab.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	out := make(map[string][]byte, len(entries))
	for _, e := range entries {
		out[e.Name] = e.Data
	}

	return out
}

func TestBuildFromConfigFS(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.2.3.4"})

	cfg := goConfig()
	cfg.FS = fstest.MapFS{"bin/hello.exe": {Data: exe}}
	cfg.Components = []*config.Component{{ID: "Hello", Directory: "INSTALLDIR", Files: []*config.File{{Source: "bin/hello.exe"}}}}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("Hello")}

	res := mustBuild(t, cfg)

	if v := cell(t, res.Database, "File", "Version", func(r []any) bool { return r[0] == "hello.exe" }); v != "1.2.3.4" {
		t.Errorf("File.Version %v, want 1.2.3.4 read from the PE in cfg.FS", v)
	}

	files := cabinetFiles(t, res)
	if len(files) != 1 || !bytes.Equal(files["hello.exe"], exe) {
		t.Errorf("cabinet holds %d files, want hello.exe from cfg.FS", len(files))
	}
}

func TestBuildAddTree(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0"})
	fsys := fstest.MapFS{
		"dist/hello.exe":           {Data: exe},
		"dist/conf/config.yaml":    {Data: []byte("a: 1\n")},
		"dist/docs/readme.txt":     {Data: []byte("docs\n")},
		"dist/docs/api/readme.txt": {Data: []byte("api docs\n")},
	}

	cfg := goConfig()

	comps, err := cfg.AddTree(fsys, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"})
	if err != nil {
		t.Fatal(err)
	}

	res := mustBuild(t, cfg)

	if n := len(rows(t, res.Database, "File")); n != 4 {
		t.Errorf("%d File rows, want 4", n)
	}

	if n := len(rows(t, res.Database, "Component")); n != len(comps) {
		t.Errorf("%d Component rows, want %d", n, len(comps))
	}

	// INSTALLDIR plus conf, docs and docs/api below it.
	dirs := map[string]bool{}
	for _, r := range rows(t, res.Database, "Directory") {
		dirs[r[2].(string)] = true //nolint:forcetypeassert // DefaultDir is a string column
	}

	for _, want := range []string{"conf", "docs", "api"} {
		found := false

		for d := range dirs {
			if d == want || strings.HasSuffix(d, "|"+want) {
				found = true
			}
		}

		if !found {
			t.Errorf("no Directory row named %s in %v", want, dirs)
		}
	}

	files := cabinetFiles(t, res)
	if len(files) != 4 {
		t.Errorf("cabinet holds %d files, want 4", len(files))
	}

	again := goConfig()
	if _, err := again.AddTree(fsys, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
		t.Fatal(err)
	}

	if second := mustBuild(t, again); !bytes.Equal(res.Package, second.Package) {
		t.Error("the same tree built twice gives different packages")
	}
}

func TestBuildConfigFSPaths(t *testing.T) {
	t.Parallel()

	icon := append([]byte{0, 0, 1, 0, 1, 0}, make([]byte, 64)...)
	own := []byte("from File.FS")

	cfg := goConfig()
	cfg.FS = fstest.MapFS{
		"bin/a.txt":      {Data: []byte("a")},
		"bin/b.txt":      {Data: []byte("b")},
		"bin/c.txt":      {Data: []byte("c")},
		"bin/own.txt":    {Data: []byte("from Config.FS")},
		"res/app.ico":    {Data: icon},
		"res/helper.bin": {Data: []byte("helper")},
		"res/blob.bin":   {Data: []byte("blob")},
	}
	cfg.Icons = map[string]string{"app.ico": `res\app.ico`}
	cfg.Binaries = map[string]string{"helper": "res/helper.bin"}
	cfg.Tables = []*config.Table{{
		Name:    "Blobs",
		Columns: []config.Column{{Name: "Key", Type: "s72", Key: true}, {Name: "Data", Type: "V0"}},
		Rows:    [][]any{{"one", "res/blob.bin"}},
	}}
	cfg.Components = []*config.Component{{ID: "Hello", Directory: "INSTALLDIR", Files: []*config.File{
		{Source: "./bin/a.txt"},
		{Source: "bin//./b.txt"},
		{Source: `bin\c.txt`},
		{Source: "bin/own.txt", FS: fstest.MapFS{"bin/own.txt": {Data: own}}},
	}}}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("Hello")}

	res := mustBuild(t, cfg)

	files := cabinetFiles(t, res)
	for name, want := range map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c", "own.txt": string(own)} {
		if got := string(files[name]); got != want {
			t.Errorf("%s holds %q, want %q", name, got, want)
		}
	}

	data := cell(t, res.Database, "Icon", "Data", func(r []any) bool { return r[0] == "app.ico" })
	if got, ok := data.([]byte); !ok || !bytes.Equal(got, icon) {
		t.Errorf("Icon.Data = %v, want the icon from cfg.FS", data)
	}

	data = cell(t, res.Database, "Binary", "Data", func(r []any) bool { return r[0] == "helper" })
	if got, ok := data.([]byte); !ok || string(got) != "helper" {
		t.Errorf("Binary.Data = %v, want the binary from cfg.FS", data)
	}

	data = cell(t, res.Database, "Blobs", "Data", func(r []any) bool { return r[0] == "one" })
	if got, ok := data.([]byte); !ok || string(got) != "blob" {
		t.Errorf("Blobs.Data = %v, want the binary cell from cfg.FS", data)
	}
}

func TestBuildBaseDirBackslash(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "bin", "app.txt"), []byte("app"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := goConfig()
	cfg.BaseDir = dir
	cfg.Components = []*config.Component{{ID: "Hello", Directory: "INSTALLDIR", Files: []*config.File{{Source: `bin\app.txt`}}}}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("Hello")}

	if got := string(cabinetFiles(t, mustBuild(t, cfg))["app.txt"]); got != "app" {
		t.Errorf("app.txt holds %q, want bin/app.txt below BaseDir", got)
	}
}

func TestBuildAbsoluteSourceIgnoresFS(t *testing.T) {
	t.Parallel()

	abs := filepath.Join(t.TempDir(), "disk.txt")
	if err := os.WriteFile(abs, []byte("from disk"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := goConfig()
	cfg.FS = fstest.MapFS{}
	cfg.Components = []*config.Component{{ID: "Hello", Directory: "INSTALLDIR", Files: []*config.File{
		{Source: abs, FS: fstest.MapFS{}},
	}}}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("Hello")}

	if got := string(cabinetFiles(t, mustBuild(t, cfg))["disk.txt"]); got != "from disk" {
		t.Errorf("disk.txt holds %q, want the file read from the OS", got)
	}
}

// streamFS hides io.ReaderAt and io.Seeker from the files of an fs.FS, like a
// file system that can only be read in order.
type streamFS struct{ fs.FS }

type streamFile struct{ fs.File }

func (s streamFS) Open(name string) (fs.File, error) {
	file, err := s.FS.Open(name)
	if err != nil {
		return nil, err
	}

	return streamFile{file}, nil
}

func TestBuildFromStreamOnlyFS(t *testing.T) {
	t.Parallel()

	cfg := goConfig()
	cfg.FS = streamFS{fstest.MapFS{"bin/hello.txt": {Data: []byte("hello")}}}
	cfg.Components = []*config.Component{{ID: "Hello", Directory: "INSTALLDIR", Files: []*config.File{{Source: "bin/hello.txt"}}}}
	cfg.Features[0].Components = []*config.Component{config.ComponentRef("Hello")}

	res, err := builder.Build(cfg, builder.Options{})
	if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "io.ReaderAt") {
		t.Errorf("err = %v, want a diagnostic about io.ReaderAt: %v", err, res.Diagnostics)
	}
}

func TestProductCodeFollowsConfigFSInputs(t *testing.T) {
	t.Parallel()

	codes := map[string]bool{}

	for _, content := range []string{"one", "two"} {
		cfg := goConfig()
		cfg.FS = fstest.MapFS{"bin/hello.txt": {Data: []byte("hello")}, "sub/helper.bin": {Data: []byte(content)}}
		cfg.Binaries = map[string]string{"helper": `sub\helper.bin`}
		cfg.Components = []*config.Component{{ID: "Hello", Directory: "INSTALLDIR", Files: []*config.File{{Source: "bin/hello.txt"}}}}
		cfg.Features[0].Components = []*config.Component{config.ComponentRef("Hello")}

		codes[mustBuild(t, cfg).ProductCode] = true
	}

	if len(codes) != 2 {
		t.Error("a binary from cfg.FS with other content gives the same ProductCode")
	}
}

// folderYAML installs the folder dist below the install dir.
const folderYAML = `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files:
      - source: dist/
        exclude: '*.pdb'
`

func folderConfig(t *testing.T, fsys fs.FS) *config.Config {
	t.Helper()

	cfg, err := config.Parse([]byte(folderYAML), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.FS = fsys

	return cfg
}

func TestBuildFolder(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.2.3.4"})
	fsys := fstest.MapFS{
		"dist/app.exe":       {Data: exe},
		"dist/app.pdb":       {Data: []byte("symbols")},
		"dist/conf/app.yaml": {Data: []byte("a: 1")},
	}

	cfg := folderConfig(t, fsys)

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("%v\n%s", err, res.Diagnostics)
	}

	files := cabinetFiles(t, res)
	if len(res.Files) != 2 || len(files) != 2 {
		t.Fatalf("got %d files, %d in the cabinet; want app.exe and app.yaml", len(res.Files), len(files))
	}

	// Building the same config again adds nothing, and a fresh parse of
	// the same input gives the same package.
	again, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("%v\n%s", err, again.Diagnostics)
	}

	fresh, err := builder.Build(folderConfig(t, fsys), builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("%v\n%s", err, fresh.Diagnostics)
	}

	if !bytes.Equal(res.Package, again.Package) || !bytes.Equal(res.Package, fresh.Package) {
		t.Error("rebuilding the folder changed the package")
	}

	// A new file is new content, so it gets a new ProductCode.
	fsys["dist/new.txt"] = &fstest.MapFile{Data: []byte("new")}

	added, err := builder.Build(folderConfig(t, fsys), builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("%v\n%s", err, added.Diagnostics)
	}

	if added.ProductCode == res.ProductCode || len(added.Files) != 3 {
		t.Errorf("after adding a file: ProductCode %s (was %s), %d files", added.ProductCode, res.ProductCode, len(added.Files))
	}
}

func TestBuildFolderDiagnostics(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"dist/tool.exe":     {Data: testpe.Build(testpe.Options{})},
		"dist/conf/a.yaml":  {Data: []byte("a")},
		"other/conf/a.yaml": {Data: []byte("a")},
	}

	res, err := builder.Build(folderConfig(t, fsys), builder.Options{BuildTime: buildTime})
	if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "components[0]{tool.exe}.files[0]") {
		t.Errorf("want the unversioned executable reported at components[0]{tool.exe}, got %v\n%s", err, res.Diagnostics)
	}

	// The same install path from a component and a folder is an error.
	cfg := folderConfig(t, fsys)
	cfg.Components[0].Files[0].Exclude = append(cfg.Components[0].Files[0].Exclude, "tool.exe")
	cfg.Components = append(cfg.Components, &config.Component{ID: "Other", Directory: "conf", Files: []*config.File{{Source: "other/conf/a.yaml"}}})
	cfg.Features[0].Components = append(cfg.Features[0].Components, config.ComponentRef("Other"))

	res, err = builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), "components[1].files[0]: a.yaml installs the same file as components[0]{conf/a.yaml}.files[0]") {
		t.Errorf("want a duplicate install path error, got %v\n%s", err, res.Diagnostics)
	}
}

func TestBuildFolderFromDisk(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := map[string][]byte{
		"dist/app.exe":       testpe.Build(testpe.Options{FileVersion: "1.2.3.4"}),
		"dist/conf/app.yaml": []byte("a: 1"),
	}

	for name, data := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	yamlDir := filepath.Join(root, "packaging")
	if err := os.Mkdir(yamlDir, 0o750); err != nil {
		t.Fatal(err)
	}

	yamlPath := filepath.Join(yamlDir, "msi.yaml")
	if err := os.WriteFile(yamlPath, []byte(strings.Replace(folderYAML, "source: dist/", "source: ../dist/", 1)), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(yamlPath, config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("%v\n%s", err, res.Diagnostics)
	}

	payload := cabinetFiles(t, res)
	if len(payload) != 2 {
		t.Fatalf("cabinet holds %d files, want 2", len(payload))
	}

	for _, f := range res.Files {
		if !bytes.Equal(payload[f.Key], files["dist/"+f.Source]) {
			t.Errorf("%s: cabinet content differs from dist/%s", f.Key, f.Source)
		}
	}
}

// folderGUIDs builds the folder source from fsys and returns each
// file's ComponentId by its lower-case long name.
func folderGUIDs(t *testing.T, fsys fs.FS, source string) map[string]any {
	t.Helper()

	cfg := folderConfig(t, fsys)
	cfg.Components[0].Files[0].Source = source + "/"

	res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
	if err != nil {
		t.Fatalf("%v\n%s", err, res.Diagnostics)
	}

	codes := make(map[any]any)
	for _, row := range rows(t, res.Database, "Component") {
		codes[row[0]] = row[1]
	}

	guids := make(map[string]any)

	for _, row := range rows(t, res.Database, "File") {
		name := fmt.Sprint(row[2])
		if i := strings.IndexByte(name, '|'); i >= 0 {
			name = name[i+1:]
		}

		guids[strings.ToLower(name)] = codes[row[1]]
	}

	for _, f := range res.Files {
		if !strings.HasPrefix(f.Source, source+"/") {
			t.Errorf("source %s with Config.FS, want it below %s/", f.Source, source)
		}
	}

	return guids
}

func TestFolderGUIDsFollowInstallPaths(t *testing.T) {
	t.Parallel()

	// The same install paths from a moved source folder whose names
	// changed case: the components keep their codes.
	before := folderGUIDs(t, fstest.MapFS{
		"dist/readme.txt":     {Data: []byte("v1")},
		"dist/conf/Notes.txt": {Data: []byte("v1")},
	}, "dist")
	after := folderGUIDs(t, fstest.MapFS{
		"build/dist/README.txt":     {Data: []byte("v2")},
		"build/dist/Conf/NOTES.txt": {Data: []byte("v2")},
		"build/dist/new.txt":        {Data: []byte("new")},
	}, "build/dist")

	for _, name := range []string{"readme.txt", "notes.txt"} {
		if before[name] == nil || before[name] != after[name] {
			t.Errorf("%s: ComponentId %v before the move, %v after", name, before[name], after[name])
		}
	}

	if after["new.txt"] == nil || after["new.txt"] == after["readme.txt"] {
		t.Errorf("new.txt has ComponentId %v", after["new.txt"])
	}
}
