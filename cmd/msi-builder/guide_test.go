package main

import (
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// guidePage is the user guide whose examples TestGuideExamples builds.
const guidePage = "../../docs/YAML Schema.md"

// yamlBlock matches a fenced YAML code block of the guide.
var yamlBlock = regexp.MustCompile("(?ms)^```yaml\n(.*?)^```")

// TestGuideExamples builds every YAML example of the user guide without a
// warning. The first example is a whole file; every later one is a part of
// a file, whose top-level keys replace those of the first. A part with
// components or features replaces both, so its files are not installed
// twice. Each file a source or license names is created: a versioned
// program for .exe and .dll, text otherwise, and a folder source gets a
// program, a .pdb and a text file.
func TestGuideExamples(t *testing.T) {
	t.Parallel()

	page, err := os.ReadFile(guidePage)
	if err != nil {
		t.Fatal(err)
	}

	blocks := yamlBlock.FindAllSubmatch(page, -1)
	if len(blocks) < 2 {
		t.Fatalf("%s has %d YAML examples", guidePage, len(blocks))
	}

	var base map[string]any
	if err := yaml.Unmarshal(blocks[0][1], &base); err != nil {
		t.Fatal(err)
	}

	for i, block := range blocks {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			buildGuideExample(t, base, block[1])
		})
	}
}

func buildGuideExample(t *testing.T, base map[string]any, example []byte) {
	t.Helper()

	var part map[string]any
	if err := yaml.Unmarshal(example, &part); err != nil {
		t.Fatalf("%v\n%s", err, example)
	}

	doc := maps.Clone(base)

	if part["components"] != nil || part["features"] != nil {
		delete(doc, "components")
	}

	maps.Copy(doc, part)

	src, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeGuideFile(t, dir, "msi.yaml", src)

	for _, name := range guideInputs(doc) {
		if strings.HasSuffix(name, "/") {
			for _, file := range []string{"app.exe", "app.pdb", "lib/readme.txt"} {
				writeGuideFile(t, dir, path.Join(name, file), guideInput(file))
			}

			continue
		}

		writeGuideFile(t, dir, name, guideInput(name))
	}

	cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{Vars: map[string]string{"VERSION": "1.4.0"}})
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		t.Fatalf("%v\n%v\n%s", err, res.Diagnostics, src)
	}

	if len(res.Diagnostics) > 0 {
		t.Errorf("%v\n%s", res.Diagnostics, src)
	}
}

// guideInputs returns the values of every source and license key in value.
func guideInputs(value any) []string {
	var names []string

	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if name, ok := item.(string); ok && (key == "source" || key == "license") {
				names = append(names, name)

				continue
			}

			names = append(names, guideInputs(item)...)
		}
	case []any:
		for _, item := range value {
			names = append(names, guideInputs(item)...)
		}
	}

	slices.Sort(names)

	return slices.Compact(names)
}

func guideInput(name string) []byte {
	switch path.Ext(name) {
	case ".exe", ".dll":
		return testpe.Build(testpe.Options{FileVersion: "1.4.0.0", Languages: []int{1033}})
	default:
		return []byte("Example content\n")
	}
}

func writeGuideFile(t *testing.T, dir, name string, data []byte) {
	t.Helper()

	file := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
