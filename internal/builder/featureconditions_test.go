package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestFeatureConditions writes one Condition row per entry of
// features[].conditions, with the level of default or level.
func TestFeatureConditions(t *testing.T) {
	t.Parallel()

	cfg, err := config.Parse([]byte(`schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
features:
  - id: Main
    components: [{ files: [{ source: a.txt }] }]
  - id: Assoc
    level: 3
    conditions:
      - { condition: ASSOC = 1, default: on }
      - { condition: ASSOC <> 1, default: disabled }
      - { condition: VersionNT64, level: 5 }
    components: [{ files: [{ source: b.txt }] }]
`), config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.BaseDir = t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(cfg.BaseDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	database, err := msidb.Unmarshal(mustBuild(t, cfg).Package)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, row := range rows(t, database, "Condition") {
		got = append(got, fmt.Sprint(row...))
	}

	slices.Sort(got)

	want := []string{"Assoc0ASSOC <> 1", "Assoc1ASSOC = 1", "Assoc5VersionNT64"}
	if !slices.Equal(got, want) {
		t.Errorf("Condition rows %q, want %q", got, want)
	}

	if level := cell(t, database, "Feature", "Level", func(r []any) bool { return r[0] == "Assoc" }); fmt.Sprint(level) != "3" {
		t.Errorf("Assoc level %v, want 3", level)
	}
}
