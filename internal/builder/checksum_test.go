package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jkroepke/msi-builder/internal/msidb"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestChecksumSetsMSICHECKCRCS checks that checksum: true on a PE file with
// a checksum sets the attribute and MSICHECKCRCS, without which the engine
// never verifies it (FILE-09), and that a value from properties wins.
func TestChecksumSetsMSICHECKCRCS(t *testing.T) {
	t.Parallel()

	const base = `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
components:
  - files: [{ source: hello.exe, checksum: true }]
`

	for _, tc := range []struct {
		name, extra, want string
		checksum          uint32
	}{
		{"checksum", "", "1", 0x1234},
		{"no PE checksum", "", "", 0},
		{"property wins", "properties: { MSICHECKCRCS: 'yes' }\n", "yes", 0x1234},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}, Checksum: tc.checksum})

			if err := os.WriteFile(filepath.Join(dir, "hello.exe"), exe, 0o600); err != nil {
				t.Fatal(err)
			}

			cfg, err := config.Parse([]byte(base+tc.extra), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.BaseDir = dir

			database, err := msidb.Unmarshal(mustBuild(t, cfg).Package)
			if err != nil {
				t.Fatal(err)
			}

			got := ""

			for _, row := range rows(t, database, "Property") {
				if row[0] == "MSICHECKCRCS" {
					got = fmt.Sprint(row[1])
				}
			}

			if got != tc.want {
				t.Errorf("MSICHECKCRCS = %q, want %q", got, tc.want)
			}

			attrs, ok := cell(t, database, "File", "Attributes", func(r []any) bool { return r[0] == "hello.exe" }).(int)
			if !ok {
				t.Fatal("File.Attributes is not an integer")
			}

			if checked := attrs&0x400 != 0; checked != (tc.checksum != 0) {
				t.Errorf("File.Attributes = %#x", attrs)
			}
		})
	}
}
