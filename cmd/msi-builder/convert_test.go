package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	wxs := filepath.Join(dir, "hello.wxs")
	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="$(var.Version)" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade DowngradeErrorMessage="A newer version is installed." />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <Component Directory="INSTALLFOLDER">
      <File Source="hello.exe" />
    </Component>
    <Merge Id="Runtime" SourceFile="runtime.msm" />
  </Package>
</Wix>
`

	if err := os.WriteFile(wxs, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	if code := run([]string{"convert", wxs}, &stdout, &stderr); code != 0 {
		t.Fatalf("convert: exit %d: %s", code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "version: ${Version}") || !strings.Contains(stdout.String(), "- source: hello.exe") {
		t.Errorf("stdout %q", stdout.String())
	}

	if want := wxs + ":10: TODO: <Merge> is not supported"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}

	out := filepath.Join(dir, "msi.yaml")

	stdout.Reset()
	stderr.Reset()

	if code := run([]string{"convert", "-o", out, wxs}, &stdout, &stderr); code != 0 || stdout.Len() != 0 {
		t.Fatalf("convert -o: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}

	if code := run([]string{"validate", "--var", "Version=1.0.0", out}, &stdout, &stderr); code != 0 {
		t.Errorf("validate: exit %d: %s", code, stderr.String())
	}

	stderr.Reset()

	if code := run([]string{"convert", "-o", out, wxs}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "--force") {
		t.Errorf("existing output: exit %d, stderr %q", code, stderr.String())
	}

	if code := run([]string{"convert", "--force", "-o", out, wxs}, &stdout, &stderr); code != 0 {
		t.Errorf("--force: exit %d, stderr %q", code, stderr.String())
	}

	stderr.Reset()

	if code := run([]string{"convert", filepath.Join(dir, "missing.wxs")}, &stdout, &stderr); code != 1 {
		t.Errorf("missing input: exit %d, stderr %q", code, stderr.String())
	}

	if code := run([]string{"convert"}, &stdout, &stderr); code != 2 {
		t.Errorf("no argument: exit %d", code)
	}
}
