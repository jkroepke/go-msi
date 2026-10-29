package builder_test

import (
	"fmt"
	"testing/fstest"

	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// A package built in Go from a folder tree, without a YAML file.
func ExampleBuild() {
	cfg := &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "My App", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3301"},
		Package: config.Package{Platform: "x64"},
		Upgrade: &config.Upgrade{},
		Directories: []*config.Directory{{
			ID:       "ProgramFiles64Folder",
			Children: []*config.Directory{{ID: "INSTALLDIR", Name: "My App", Configurable: true}},
		}},
		Features: []*config.Feature{{ID: "Main", Title: "My App"}},
	}

	dist := fstest.MapFS{
		"dist/readme.txt":      {Data: []byte("hello\n")},
		"dist/conf/app.yaml":   {Data: []byte("a: 1\n")},
		"dist/conf/extra.yaml": {Data: []byte("b: 2\n")},
	}

	if _, err := cfg.AddTree(dist, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
		fmt.Println(err)

		return
	}

	res, err := builder.Build(cfg, builder.Options{})
	if err != nil {
		fmt.Println(err, res.Diagnostics)

		return
	}

	fmt.Println(len(res.Files), "files")
	// Output: 3 files
}
