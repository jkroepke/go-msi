//go:build windows

package nativetest_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/jkroepke/go-msi/internal/ice"
	"github.com/jkroepke/go-msi/internal/testpe"
	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/config"
	"github.com/jkroepke/go-msi/pkg/msidb"
)

// iceCube returns the path of the full ICE suite. GO_MSI_ICE_CUB names it;
// otherwise the MsiVal2 install location is tried. The file comes with the
// Windows SDK and may not be redistributed, so the test skips without it
// unless GO_MSI_ICE_REQUIRED is set (CI sets both).
func iceCube(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("ICE validation needs msi.dll time")
	}

	candidates := []string{os.Getenv("GO_MSI_ICE_CUB")}
	if pf := os.Getenv("ProgramFiles(x86)"); pf != "" {
		candidates = append(candidates, filepath.Join(pf, "MsiVal2", "darice.cub"))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if os.Getenv("GO_MSI_ICE_REQUIRED") != "" {
		t.Fatal("GO_MSI_ICE_REQUIRED is set but no darice.cub was found; set GO_MSI_ICE_CUB")
	}

	t.Skip("no darice.cub: install MsiVal2 from the Windows SDK or set GO_MSI_ICE_CUB")

	return ""
}

func windowsExporterInputs() (map[string]string, map[string][]byte) {
	icon := append([]byte{0, 0, 1, 0, 1, 0}, make([]byte, 64)...)

	return map[string]string{"VERSION": "0.32.0.5"}, map[string][]byte{
		"windows_exporter.exe": testpe.Build(testpe.Options{FileVersion: "0.32.0.5", Languages: []int{1033}}),
		"icon.ico":             icon,
	}
}

// TestICEExamples runs the full ICE suite against both examples and fails on
// any ICE error or failure. Warnings are logged.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEExamples(t *testing.T) {
	cub := iceCube(t)
	exporterVars, exporterInputs := windowsExporterInputs()

	for _, tc := range []struct {
		name   string
		vars   map[string]string
		inputs map[string][]byte
	}{
		{"minimal", nil, minimalInputs()},
		{"windows_exporter", exporterVars, exporterInputs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, _ := buildExample(t, tc.name, tc.vars, tc.inputs)
			requireCleanICE(t, path, cub)
		})
	}
}

// TestICEAddTree runs the full ICE suite against a package built in Go from
// a harvested folder tree, without any YAML. dist/a holds no file of its
// own, so the suite also sees a directory that no component installs to.
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEAddTree(t *testing.T) {
	cub := iceCube(t)

	cfg := &config.Config{
		Schema:  config.SchemaV1,
		Product: config.Product{Name: "Tree", Manufacturer: "Example", Version: "1.0.0", UpgradeCode: "3f2504e0-4f89-41d3-9a0c-0305e82c3302"},
		Package: config.Package{Platform: "x64"},
		Upgrade: &config.Upgrade{},
		Directories: []*config.Directory{{
			ID:       "ProgramFiles64Folder",
			Children: []*config.Directory{{ID: "INSTALLDIR", Name: "Tree", Configurable: true}},
		}},
		Features: []*config.Feature{{ID: "Main", Title: "Tree"}},
	}

	fsys := fstest.MapFS{
		"dist/tree.exe":            {Data: testpe.Build(testpe.Options{FileVersion: "1.0.0.0"})},
		"dist/conf/config.yaml":    {Data: []byte("a: 1\n")},
		"dist/docs/readme.txt":     {Data: []byte("docs\n")},
		"dist/docs/api/readme.txt": {Data: []byte("api docs\n")},
		"dist/a/b/c.txt":           {Data: []byte("only in a folder below a\n")},
	}

	if _, err := cfg.AddTree(fsys, "dist", "INSTALLDIR", config.TreeOptions{Feature: "Main"}); err != nil {
		t.Fatal(err)
	}

	res, err := builder.Build(cfg, builder.Options{})
	if err != nil {
		t.Fatalf("build: %v\n%s", err, res.Diagnostics)
	}

	path := filepath.Join(t.TempDir(), "tree.msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	requireCleanICE(t, path, cub)
}

// requireCleanICE runs the ICE suite on the package at path and fails on any
// ICE error or failure. Warnings are logged.
func requireCleanICE(t *testing.T, path, cub string) {
	t.Helper()

	findings, err := ice.Run(path, cub)
	if err != nil {
		t.Fatal(err)
	}

	ran := map[string]bool{}

	for _, finding := range findings {
		ran[finding.ICE] = true

		switch finding.Type {
		case ice.Error, ice.Failure:
			t.Error(finding)
		case ice.Warning:
			t.Log(finding)
		case ice.Info:
		}
	}

	// A suite that posts nothing at all did not run: every ICE posts at least
	// its "created/last modified" info line.
	if len(ran) < 20 {
		t.Errorf("only %d ICEs reported anything: %v", len(ran), findings)
	}
}

// TestICEFindsDuplicateComponentGUID checks that the runner reports a known
// defect: two components that share one ComponentId (ICE08).
//
//nolint:paralleltest // msi.dll's external UI handler is process-wide
func TestICEFindsDuplicateComponentGUID(t *testing.T) {
	cub := iceCube(t)
	vars, inputs := windowsExporterInputs()
	_, res := buildExample(t, "windows_exporter", vars, inputs)

	comp := res.Database.Table("Component")
	col := comp.Column("ComponentId")

	if len(comp.Rows) < 2 {
		t.Fatalf("windows_exporter has %d components, need two", len(comp.Rows))
	}

	comp.Rows[1][col] = comp.Rows[0][col]

	data, err := res.Database.Marshal(msidb.Options{})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "duplicate-guid.msi")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	findings, err := ice.Run(path, cub)
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range findings {
		if f.ICE == "ICE08" && f.Type == ice.Error {
			return
		}
	}

	t.Errorf("no ICE08 error for a duplicate ComponentId; findings: %v", findings)
}
