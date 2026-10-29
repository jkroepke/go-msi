package builder_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// perUserRollbackConfig is a per-user package with one deferred action;
// the placement and return mode are filled in per case.
const perUserRollbackConfig = `schema: msi-builder/v1
product: { name: A, manufacturer: B, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64, scope: %s }
components:
  - files: [{ source: a.txt }]
customActions:
  - id: Step
    exec: { command: '"[System64Folder]cmd.exe" /c exit 1', directory: INSTALLDIR }
    execute: deferred
    return: %s
    schedule: [{ sequence: execute, after: %s }]
`

// TestPerUserRollbackWarning checks the CA-01 warning: only a per-user
// deferred action that can fail and runs after RegisterProduct gets it.
func TestPerUserRollbackWarning(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		scope, ret, after string
		want              bool
	}{
		{"perUser", "check", "PublishProduct", true},
		{"perUser", "check", "RegisterProduct", true},
		{"perUser", "check", "InstallFiles", false},
		{"perUser", "ignore", "PublishProduct", false},
		{"perMachine", "check", "PublishProduct", false},
	} {
		t.Run(tc.scope+"-"+tc.ret+"-"+tc.after, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Parse(fmt.Appendf(nil, perUserRollbackConfig, tc.scope, tc.ret, tc.after), config.Options{})
			if err != nil {
				t.Fatal(err)
			}

			cfg.BaseDir = t.TempDir()
			if err := os.WriteFile(filepath.Join(cfg.BaseDir, "a.txt"), []byte("a"), 0o600); err != nil {
				t.Fatal(err)
			}

			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			if err != nil {
				t.Fatalf("build: %v\n%s", err, res.Diagnostics)
			}

			got := strings.Contains(fmt.Sprint(res.Diagnostics), "runs after RegisterProduct")
			if got != tc.want {
				t.Errorf("warning = %v, want %v: %s", got, tc.want, res.Diagnostics)
			}
		})
	}
}
