package wixconvert_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/exampletest"
	"github.com/jkroepke/msi-builder/internal/wixconvert"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// TestConvertSnapshots converts every testdata/*.wxs, compares the result
// with testdata/<name>.yaml and checks that msi-builder accepts it.
func TestConvertSnapshots(t *testing.T) {
	t.Parallel()

	sources, err := filepath.Glob(filepath.Join("testdata", "*.wxs"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no test sources: %v", err)
	}

	for _, src := range sources {
		name := strings.TrimSuffix(filepath.Base(src), ".wxs")

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}

			res, err := wixconvert.Convert(bytes.NewReader(data), wixconvert.Options{Source: filepath.Base(src)})
			if err != nil {
				t.Fatal(err)
			}

			exampletest.CompareSnapshot(t, filepath.Join("testdata", name+".yaml"), string(res.YAML))

			for _, n := range res.Notes {
				if !strings.Contains(string(res.YAML), n.Message) {
					t.Errorf("note %q is not a comment in the YAML file", n)
				}
			}

			vars := map[string]string{}
			for _, name := range res.Vars {
				vars[name] = "1.2.3"
			}

			cfg, err := config.Parse(res.YAML, config.Options{Vars: vars})
			if err != nil {
				t.Fatal(err)
			}

			if err := config.Validate(cfg).Err(); err != nil {
				t.Errorf("the converted file does not validate: %v", err)
			}
		})
	}
}

func TestConvertErrors(t *testing.T) {
	t.Parallel()

	for name, src := range map[string]string{
		"not xml":       `<Wix`,
		"not wix":       `<Project/>`,
		"bundle":        `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs"><Bundle/></Wix>`,
		"no package":    `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs"><Fragment/></Wix>`,
		"two packages":  `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs"><Package/><Package/></Wix>`,
		"two root tags": `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs"><Package/></Wix><Wix/>`,
	} {
		if _, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{}); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestConvertNotes(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="!(bind.FileVersion.hello.exe)" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <Feature Id="Main"><ComponentRef Id="Missing" /></Feature>
    <Feature Id="Other"><ComponentRef Id="Missing" /></Feature>
    <Merge Id="Runtime" SourceFile="runtime.msm" />
    <CustomAction Id="Init" Directory="INSTALLFOLDER" ExeCommand="[INSTALLFOLDER]tool.exe /s" Execute="deferred" Return="check" />
    <CustomAction Id="Old" Directory="INSTALLFOLDER" ExeCommand="[OLD_UNINSTALL] /S" Execute="deferred" Return="check" />
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	want := []wixconvert.Note{
		{Line: 0, Message: "ComponentRef Missing names no component of this file"},
		{Line: 2, Message: "!(bind.FileVersion.hello.exe) is a WiX bind variable; replace it"},
		{Line: 2, Message: "WiX takes the platform from wix build -arch; check package.platform"},
		{Line: 2, Message: "the WiX package has no MajorUpgrade; msi-builder adds the standard major upgrade (set upgrade: false to leave it out)"},
		{Line: 5, Message: "<Merge> is not supported: merge modules are not supported; install their files in components"},
		{Line: 6, Message: "quote the executable [INSTALLFOLDER]tool.exe: an unquoted path with a space is tried piece by piece by CreateProcess"},
		{Line: 7, Message: "check [OLD_UNINSTALL]: quote it if it holds a bare path; leave it if it holds a quoted command line such as an UninstallString"},
	}

	if len(res.Notes) != len(want) {
		t.Fatalf("notes %q, want %q", res.Notes, want)
	}

	for i := range want {
		if res.Notes[i] != want[i] {
			t.Errorf("note %d is %q, want %q", i, res.Notes[i], want[i])
		}
	}
}

// TestConvertV3Scope checks that a WiX v3 package without InstallScope,
// which Windows Installer installs per user, is not silently made a
// per-machine package.
func TestConvertV3Scope(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <Package Platform="x64" />
  </Product>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(res.YAML), "no InstallScope") {
		t.Errorf("no scope note in\n%s", res.YAML)
	}
}
