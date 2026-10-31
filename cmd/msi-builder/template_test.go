package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/testpe"
)

const templateYAML = `schema: msi-builder/v1
product:
  name: Hello
  manufacturer: Example
  version: ${VERSION}
  upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301
package:
  platform: {{ var "ARCH" "x64" }}
components:
  - files:
      - source: hello.exe
{{- if eq (var "ARCH" "x64") "arm64" }}
      - source: hello-arm64.dll
{{- end }}
`

// TestTemplateFlag checks --template on validate and the render command.
func TestTemplateFlag(t *testing.T) {
	t.Parallel()

	yaml := filepath.Join(t.TempDir(), "msi.yaml")
	if err := os.WriteFile(yaml, []byte(templateYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	if code := run([]string{"validate", "--var", "VERSION=1.0.0", yaml}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "pass --template") {
		t.Errorf("validate without --template: exit %d, stderr %q", code, stderr.String())
	}

	stderr.Reset()

	if code := run([]string{"validate", "--template", "--var", "VERSION=1.0.0", yaml}, &stdout, &stderr); code != 0 {
		t.Errorf("validate --template: exit %d, stderr %q", code, stderr.String())
	}

	stdout.Reset()

	if code := run([]string{"render", "--var", "ARCH=arm64", yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("render: exit %d, stderr %q", code, stderr.String())
	}

	for _, want := range []string{"platform: arm64\n", "source: hello-arm64.dll\n", "version: ${VERSION}\n"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("render output %q lacks %q", stdout.String(), want)
		}
	}

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(filepath.Dir(yaml), "hello.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(filepath.Dir(yaml), "out.msi")
	if code := run([]string{"build", "--template", "--var", "VERSION=1.0.0", "-o", out, yaml}, &stdout, &stderr); code != 0 {
		t.Errorf("build --template: exit %d, stderr %q", code, stderr.String())
	}

	stderr.Reset()

	if code := run([]string{"render", filepath.Join(filepath.Dir(yaml), "missing.yaml")}, &stdout, &stderr); code != 1 {
		t.Errorf("render of a missing file: exit %d, stderr %q", code, stderr.String())
	}
}
