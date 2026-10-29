//go:build windows

package nativetest_test

// Temporary probe for FILE-07: what ICE and msiexec do when two names in one
// folder share a short name. It fails on purpose so that CI prints its log;
// it is removed before merge.

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/ice"
	"github.com/jkroepke/msi-builder/pkg/builder"
	"github.com/jkroepke/msi-builder/pkg/config"
)

const probeHeader = `schema: msi-builder/v1
product:
  name: SN Probe %s
  manufacturer: Example
  version: 1.0.0
  upgradeCode: %s
package:
  platform: x64
directories:
  - id: ProgramFiles64Folder
    children:
      - id: PROBEDIR
        name: snprobe%s
        children:
          - id: DIRONE
            name: "DATA~1|Data One"
          - id: DIRTWO
            name: "DATA~1|Data Two"
`

var probeCases = []struct{ name, guid, body string }{
	{"a", "3f2504e0-4f89-41d3-9a0c-0305e82c3a01", `components:
  - id: Both
    directory: PROBEDIR
    files:
      - { source: one.txt, id: One, name: "PROG~1.TXT|Program One.txt" }
      - { source: two.txt, id: Two, name: "PROG~1.TXT|Program Two.txt" }
`},
	{"b", "3f2504e0-4f89-41d3-9a0c-0305e82c3a02", `components:
  - id: CompOne
    directory: PROBEDIR
    files:
      - { source: one.txt, id: One, name: "PROG~1.TXT|Program One.txt" }
  - id: CompTwo
    directory: PROBEDIR
    files:
      - { source: two.txt, id: Two, name: "PROG~1.TXT|Program Two.txt" }
`},
	{"c", "3f2504e0-4f89-41d3-9a0c-0305e82c3a03", `components:
  - id: InOne
    directory: DIRONE
    files:
      - { source: one.txt, id: One, name: one.txt }
  - id: InTwo
    directory: DIRTWO
    files:
      - { source: two.txt, id: Two, name: two.txt }
`},
	{"d", "3f2504e0-4f89-41d3-9a0c-0305e82c3a04", `components:
  - id: Mixed
    directory: PROBEDIR
    files:
      - { source: one.txt, id: One, name: "DATA~1|Data Three" }
      - { source: two.txt, id: Two, name: two.txt }
  - id: InOne
    directory: DIRONE
    files:
      - { source: two.txt, id: Three, name: three.txt }
`},
}

func TestZZShortNameProbe(t *testing.T) { //nolint:paralleltest // msiexec runs one installation at a time
	cub := iceCube(t)

	var out strings.Builder

	for _, c := range probeCases {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "msi.yaml"), []byte(fmt.Sprintf(probeHeader, c.name, c.guid, c.name)+c.body), 0o600)
		_ = os.WriteFile(filepath.Join(dir, "one.txt"), []byte("one"), 0o600)
		_ = os.WriteFile(filepath.Join(dir, "two.txt"), []byte("two"), 0o600)

		cfg, err := config.Load(filepath.Join(dir, "msi.yaml"), config.Options{})
		if err != nil {
			fmt.Fprintf(&out, "case %s: load: %v\n", c.name, err)

			continue
		}

		pkg := filepath.Join(dir, "probe.msi")

		res, err := builder.Build(cfg, builder.Options{Output: pkg})
		if err != nil {
			fmt.Fprintf(&out, "case %s: build: %v %v\n", c.name, err, res.Diagnostics)

			continue
		}

		fmt.Fprintf(&out, "=== case %s\n", c.name)

		findings, err := ice.Run(pkg, cub)
		fmt.Fprintf(&out, "ICE err=%v\n", err)

		for _, f := range findings {
			if f.Type == ice.Error || f.Type == ice.Failure || f.Type == ice.Warning {
				fmt.Fprintf(&out, "  %v\n", f)
			}
		}

		root := filepath.Join(os.Getenv("ProgramW6432"), "snprobe"+c.name)

		for _, props := range [][]string{nil, {"SHORTFILENAMES=1"}} {
			log := filepath.Join(dir, "install.log")
			args := append([]string{"/i", pkg, "/qn", "/l*v", log}, props...)
			//nolint:gosec // test-controlled arguments
			o, err := exec.CommandContext(t.Context(), "msiexec", args...).CombinedOutput()
			fmt.Fprintf(&out, "install %v: err=%v %s\n", props, err, o)

			for _, base := range []string{root, filepath.Join(os.Getenv("ProgramW6432"), "SNPROB~1")} {
				_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil //nolint:nilerr // listing only
					}

					content := ""
					if !d.IsDir() {
						b, _ := os.ReadFile(p)
						content = string(b)
					}

					fmt.Fprintf(&out, "  %s %q\n", p, content)

					return nil
				})
			}

			// The 8.3 names on disk of everything under Program Files that the probe touched.
			o, _ = exec.CommandContext(t.Context(), "cmd", "/c", "dir", "/s", "/x", root).CombinedOutput()
			fmt.Fprintf(&out, "  dir /x:\n%s\n", o)

			for _, line := range strings.Split(readLog(log), "\n") {
				if strings.Contains(line, "Error") || strings.Contains(line, "SHORTFILENAMES") || strings.Contains(line, "PROBEDIR") ||
					strings.Contains(line, "DIRONE") || strings.Contains(line, "DIRTWO") || strings.Contains(line, "File: ") {
					fmt.Fprintf(&out, "  log: %s\n", strings.TrimSpace(line))
				}
			}

			o, err = exec.CommandContext(t.Context(), "msiexec", "/x", pkg, "/qn").CombinedOutput()
			fmt.Fprintf(&out, "uninstall: err=%v %s\n", err, o)

			_, statErr := os.Stat(root)
			fmt.Fprintf(&out, "  root after uninstall: %v\n", statErr)
		}
	}

	t.Errorf("FILE-07 probe output:\n%s", out.String())
}
