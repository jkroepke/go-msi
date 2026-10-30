package builder_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jkroepke/msi-builder/internal/cfb"
	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestProgressText writes an ActionText row, without a template, for each
// custom action with a progress text, and lets a language transform
// replace it (CA-08).
func TestProgressText(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: a.txt }]
customActions:
  - id: Prepare
    setProperty: { name: A, value: "1" }
    progress: Preparing [ProductName]
    schedule: [{ sequence: execute, after: CostFinalize }]
  - id: Quiet
    setProperty: { name: B, value: "1" }
    schedule: [{ sequence: execute, after: CostFinalize }]
ui:
  enabled: true
  languages:
    de: { progress: { Prepare: Vorbereitung } }
    fr: {}
`), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.BaseDir, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}

	res := mustBuild(t, cfg)

	database, err := msidb.Unmarshal(res.Package)
	if err != nil {
		t.Fatal(err)
	}

	table := database.Table("ActionText")
	if table == nil || len(table.Rows) != 1 {
		t.Fatalf("ActionText %v, want one row", table)
	}

	if row := table.Rows[0]; row[0] != "Prepare" || row[1] != "Preparing [ProductName]" || row[2] != nil {
		t.Errorf("ActionText row %v", row)
	}

	actionText, err := msidb.EncodeStreamName("ActionText", true)
	if err != nil {
		t.Fatal(err)
	}

	for _, storage := range res.Database.Storages {
		changes := slices.ContainsFunc(storage.Streams, func(s cfb.Source) bool { return s.Name == actionText })
		if want := storage.Name == "1031"; changes != want {
			t.Errorf("transform %s changes ActionText: %t, want %t", storage.Name, changes, want)
		}
	}
}
