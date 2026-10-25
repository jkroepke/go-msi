package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/internal/testpe"
)

func TestBuildValidateInspect(t *testing.T) {
	dir := t.TempDir()

	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "minimal", "msi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	yaml := filepath.Join(dir, "msi.yaml")
	if err := os.WriteFile(yaml, src, 0o600); err != nil {
		t.Fatal(err)
	}

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	if err := os.WriteFile(filepath.Join(dir, "hello.exe"), exe, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	if code := run([]string{"validate", yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate: exit %d: %s", code, stderr.String())
	}

	out := filepath.Join(dir, "out.msi")

	if code := run([]string{"build", "-o", out, yaml}, &stdout, &stderr); code != 0 {
		t.Fatalf("build: exit %d: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "1 files") {
		t.Errorf("build output %q", stdout.String())
	}

	stdout.Reset()

	if code := run([]string{"inspect", "--table", "File", out}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect: exit %d: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "hello.exe") {
		t.Errorf("inspect output %q", stdout.String())
	}

	stdout.Reset()

	if code := run([]string{"inspect", out}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "summary  9:") {
		t.Errorf("inspect: exit %d, output %q", code, stdout.String())
	}

	if code := run([]string{"build", filepath.Join(dir, "missing.yaml")}, &stdout, &stderr); code != 1 {
		t.Errorf("missing file: exit %d", code)
	}

	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Errorf("no command: exit %d", code)
	}

	if code := run([]string{"version"}, &stdout, &stderr); code != 0 {
		t.Errorf("version: exit %d", code)
	}
}
