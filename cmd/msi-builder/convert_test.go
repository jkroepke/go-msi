package main

import (
	"bytes"
	"maps"
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

	if code := run([]string{"convert", "--force", wxs}, &stdout, &stderr); code != 2 {
		t.Errorf("--force without --output: exit %d", code)
	}

	if code := run([]string{"convert"}, &stdout, &stderr); code != 2 {
		t.Errorf("no argument: exit %d", code)
	}
}

// TestConvertKeepsBuiltCodes converts with --msi: a package with the
// component codes WiX generated stands in for WiX's build, one component
// found by its Id and one, without an Id, by its key file.
func TestConvertKeepsBuiltCodes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	built := `schema: msi-builder/v1
product: { name: Hello, manufacturer: Example, version: 1.0.0, upgradeCode: 3f2504e0-4f89-41d3-9a0c-0305e82c3301 }
package: { platform: x64 }
installDir: { id: INSTALLFOLDER, name: Hello }
components:
  - { id: Main, guid: 6d1c0a42-93b8-4e57-a2f1-0c7e5d9b3a61, files: [{ source: hello.exe }] }
  - { id: cmpNotes, guid: 2b8f4e17-5a0c-4d93-b6e2-9f1a3c7d5e08, files: [{ source: notes.txt }] }
  - id: Settings
    files: [{ source: settings.txt }]
    registry: [{ id: SettingsKey, root: HKLM, key: 'SOFTWARE\Example\Hello', name: Level, value: '1' }]
    keyPath: SettingsKey
`
	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade Schedule="afterInstallExecute" DowngradeErrorMessage="A newer version is installed." />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <Component Id="Main" Directory="INSTALLFOLDER">
      <File Source="hello.exe" />
    </Component>
    <Component Directory="INSTALLFOLDER">
      <File Source="notes.txt" />
    </Component>
    <Component Id="Readme" Directory="INSTALLFOLDER">
      <File Source="readme.txt" />
    </Component>
  </Package>
</Wix>
`

	for name, data := range map[string]string{"msi.yaml": built, "app.wxs": src, "hello.exe": "MZ", "notes.txt": "notes", "settings.txt": "level: 1"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer

	pkg := filepath.Join(dir, "hello-1.0.0.msi")
	if code := run([]string{"build", "-o", pkg, filepath.Join(dir, "msi.yaml")}, &stdout, &stderr); code != 0 {
		t.Fatalf("build: exit %d: %s", code, stderr.String())
	}

	// A registry key path is no key file, even with a file in the component.
	read, err := builtPackage(pkg)
	if err != nil {
		t.Fatal(err)
	}

	keyFiles := map[string]string{}
	for _, comp := range read.Components {
		keyFiles[comp.ID] = comp.KeyFile
	}

	if want := map[string]string{"Main": "hello.exe", "cmpNotes": "notes.txt", "Settings": ""}; !maps.Equal(keyFiles, want) {
		t.Errorf("key files %v, want %v", keyFiles, want)
	}

	stdout.Reset()
	stderr.Reset()

	wxs := filepath.Join(dir, "app.wxs")
	if code := run([]string{"convert", "--msi", pkg, wxs}, &stdout, &stderr); code != 0 {
		t.Fatalf("convert --msi: exit %d: %s", code, stderr.String())
	}

	for _, want := range []string{
		"guid: '{6D1C0A42-93B8-4E57-A2F1-0C7E5D9B3A61}'",
		"guid: '{2B8F4E17-5A0C-4D93-B6E2-9F1A3C7D5E08}'",
		"The guid of 2 components whose Guid WiX generated comes from hello-1.0.0.msi.",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout.String())
		}
	}

	if want := wxs + ":13: TODO: hello-1.0.0.msi has no matching component"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}

	stdout.Reset()
	stderr.Reset()

	// Without --msi, the late removal gets a note instead.
	if code := run([]string{"convert", wxs}, &stdout, &stderr); code != 0 {
		t.Fatalf("convert: exit %d: %s", code, stderr.String())
	}

	if want := wxs + ":3: TODO: WiX generated the Guid of Main, notes.txt, Readme;"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}

	if code := run([]string{"convert", "--msi", filepath.Join(dir, "notes.txt"), wxs}, &stdout, &stderr); code != 1 {
		t.Errorf("--msi with a file that is no package: exit %d", code)
	}
}
