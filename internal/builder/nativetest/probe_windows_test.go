//go:build windows

package nativetest_test

// Temporary probe for the WiX issue triage: every observation is reported
// with t.Errorf so that CI shows it. Replaced by permanent tests before merge.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/pkg/config"
)


type probeReport struct{ lines []string }

func (r *probeReport) add(format string, args ...any) {
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func probeBuild(t *testing.T, dir, name, yaml string, inputs map[string][]byte) (string, string) {
	t.Helper()

	for file, data := range inputs {
		p := filepath.Join(dir, file)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)

		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg, err := config.Parse([]byte(yaml), config.Options{})
	if err != nil {
		return "", "parse: " + err.Error()
	}

	cfg.BaseDir = dir

	res, err := builder.Build(cfg, builder.Options{BuildTime: exampletest.BuildTime})
	if err != nil {
		return "", fmt.Sprintf("build: %v %v", err, res.Diagnostics)
	}

	path := filepath.Join(dir, name+".msi")
	if err := os.WriteFile(path, res.Package, 0o600); err != nil {
		t.Fatal(err)
	}

	return path, fmt.Sprint(res.Diagnostics)
}

func probeMsiexec(log string, args ...string) int {
	//nolint:gosec // test paths
	err := exec.CommandContext(context.Background(), "msiexec", append(args, "/qn", "/l*v", log)...).Run()
	if exit, ok := err.(*exec.ExitError); ok { //nolint:errorlint // probe
		return exit.ExitCode()
	}

	if err != nil {
		return -1
	}

	return 0
}

func grepLog(log string, re *regexp.Regexp) []string {
	data := readLog(log)

	var out []string

	for line := range strings.Lines(data) {
		if re.MatchString(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}

	if len(out) > 12 {
		out = out[:12]
	}

	return out
}

func TestProbeWixIssues(t *testing.T) { //nolint:paralleltest // msiexec
	if testing.Short() {
		t.Skip("probe")
	}

	r := &probeReport{}

	probeDisabledService(t, r)

	t.Errorf("PROBE REPORT\n%s", strings.Join(r.lines, "\n"))
}

const probeSvcYAML = `schema: msi-builder/v1
product: { name: MsiBuilderProbeSvc, manufacturer: msi-builder tests, version: 1.0.0, upgradeCode: 4b8e2f19-7a3c-4d56-9e01-c2a7f5b8d934 }
package: { platform: x64 }
installDir: { name: MsiBuilderProbeSvc }
components:
  - files: [{ source: svc.exe }]
    services:
      - { name: MsiBuilderProbeSvc, start: disabled%s }
`

func probeDisabledService(t *testing.T, r *probeReport) {
	t.Helper()

	for _, extra := range []string{"", ", control: { startOnInstall: false }"} {
		dir := t.TempDir()
		path, diags := probeBuild(t, dir, "svc", fmt.Sprintf(probeSvcYAML, extra), map[string][]byte{"svc.exe": testService(t)})

		if path == "" {
			r.add("SVC %q: %s", extra, diags)

			continue
		}

		log := filepath.Join(dir, "i.log")
		e := probeMsiexec(log, "/i", path)
		r.add("SVC disabled %q: install=%d state=%q log=%q", extra, e, serviceState(t, "MsiBuilderProbeSvc"),
			grepLog(log, regexp.MustCompile(`(?i)Error 19|StartServices|Return value 3`)))
		_ = probeMsiexec(filepath.Join(dir, "x.log"), "/x", path)
		_, _ = exec.CommandContext(context.Background(), "sc", "delete", "MsiBuilderProbeSvc").CombinedOutput()
		_ = os.RemoveAll(filepath.Join(os.Getenv("ProgramFiles"), "MsiBuilderProbeSvc"))
	}
}
