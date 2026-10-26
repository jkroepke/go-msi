package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jkroepke/go-msi/internal/exampletest"
)

// TestExampleTables builds every example and compares what inspect prints
// for it with testdata/tables/<name>.txt, so a change to the generated
// tables shows up as a diff of the snapshot. GO_MSI_UPDATE_SNAPSHOTS=1
// rewrites the snapshots.
func TestExampleTables(t *testing.T) {
	t.Parallel()

	examples := exampletest.Examples()

	entries, err := os.ReadDir(exampletest.Dir())
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if _, ok := examples[entry.Name()]; entry.IsDir() && !ok {
			t.Errorf("examples/%s has no entry in exampletest.Examples", entry.Name())
		}
	}

	for _, name := range slices.Sorted(maps.Keys(examples)) {
		example := examples[name]

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path, _ := exampletest.Build(t, name, example.Vars, example.Inputs)

			var got bytes.Buffer
			if err := inspect(&got, path, ""); err != nil {
				t.Fatal(err)
			}

			exampletest.CompareSnapshot(t, filepath.Join("testdata", "tables", name+".txt"), got.String())
		})
	}
}
