package wixconvert_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
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
    <CustomAction Id="Quoted" Directory="INSTALLFOLDER" ExeCommand="&quot;[INSTALLFOLDER]tool.exe&quot; /s" Execute="deferred" Return="check" />
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

// TestConvertUINavigation checks that Publish rows that reorder the WiX
// dialogs stay TODOs unless they take out the whole license page.
func TestConvertUINavigation(t *testing.T) {
	t.Parallel()

	const todo = "<Publish> in <UI> customizes the WiX dialogs and cannot be converted"

	for name, tc := range map[string]struct {
		ui      string
		want    []wixconvert.Note
		license bool
	}{
		"no wizard": {
			ui:   `<UI Id="MyUI"><Publish Dialog="WelcomeDlg" Control="Next" Event="NewDialog" Value="InstallDirDlg" /></UI>`,
			want: []wixconvert.Note{{Line: 4, Message: todo}},
		},
		"license skipped": {
			ui: `<ui:WixUI Id="WixUI_InstallDir" InstallDirectory="INSTALLFOLDER" /><UI>
<Publish Dialog="WelcomeDlg" Control="Next" Event="NewDialog" Value="InstallDirDlg" Condition="NOT Installed" />
<Publish Dialog="InstallDirDlg" Control="Back" Event="NewDialog" Value="WelcomeDlg" Condition="NOT Installed" /></UI>`,
		},
		"back row alone": {
			ui: `<ui:WixUI Id="WixUI_InstallDir" InstallDirectory="INSTALLFOLDER" /><UI>
<Publish Dialog="InstallDirDlg" Control="Back" Event="NewDialog" Value="WelcomeDlg" /></UI>`,
			want:    []wixconvert.Note{{Line: 5, Message: todo}},
			license: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:ui="http://wixtoolset.org/schemas/v4/wxs/ui">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade DowngradeErrorMessage="newer" /><WixVariable Id="WixUILicenseRtf" Value="license.rtf" />
    ` + tc.ui + `
  </Package>
</Wix>`

			res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
			if err != nil {
				t.Fatal(err)
			}

			var notes []wixconvert.Note

			for _, note := range res.Notes {
				if note.Message == todo {
					notes = append(notes, note)
				}
			}

			if !slices.Equal(notes, tc.want) {
				t.Errorf("notes %q, want %q", notes, tc.want)
			}

			if got := strings.Contains(string(res.YAML), "license: license.rtf"); got != tc.license {
				t.Errorf("license kept: %v, want %v\n%s", got, tc.license, res.YAML)
			}
		})
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

// codesSource has components that a built package finds by Id, by key file,
// and not at all, plus a shortcut component.
const codesSource = `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello">
        <Directory Id="DocsDir" Name="docs" />
        <Directory Name="extra">
          <Component>
            <File Source="a.txt" />
          </Component>
        </Directory>
      </Directory>
    </StandardDirectory>
    <StandardDirectory Id="ProgramMenuFolder" />
    <Component Id="Main" Directory="INSTALLFOLDER">
      <File Id="main.exe" Source="main.exe" />
    </Component>
    <Component Directory="INSTALLFOLDER">
      <File Source="notes.txt" />
    </Component>
    <Component Directory="DocsDir">
      <File Source="readme.txt" />
    </Component>
    <Component Id="Menu" Directory="ProgramMenuFolder">
      <Shortcut Name="Hello" Target="[#main.exe]" />
      <RegistryValue Root="HKCU" Key="Software\Example\Hello" Name="menu" Value="1" KeyPath="yes" />
    </Component>
    <Component Directory="ProgramMenuFolder">
      <Shortcut Name="Hello notes" Target="[#main.exe]" Arguments="notes" />
      <RegistryValue Root="HKCU" Key="Software\Example\Hello" Name="notes" Value="1" KeyPath="yes" />
    </Component>
    <InstallExecuteSequence>
      <RemoveExistingProducts After="InstallExecute" />
    </InstallExecuteSequence>
  </Package>
</Wix>
`

func TestConvertBuiltCodes(t *testing.T) {
	t.Parallel()

	built := &wixconvert.BuiltPackage{Name: "hello.msi", Components: []wixconvert.BuiltComponent{
		{ID: "Main", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "INSTALLFOLDER", KeyFile: "main.exe"},
		// Two rows with the key file notes.txt: no match.
		{ID: "cmpA", Code: "{00000000-0000-4000-8000-000000000002}", Directory: "INSTALLFOLDER", KeyFile: "notes.txt"},
		{ID: "cmpB", Code: "{00000000-0000-4000-8000-000000000003}", Directory: "INSTALLFOLDER", KeyFile: "notes.txt"},
		// Another folder: no match.
		{ID: "cmpC", Code: "{00000000-0000-4000-8000-000000000004}", Directory: "OtherDir", KeyFile: "readme.txt"},
		// A folder without an Id matches any folder.
		{ID: "cmpD", Code: "{00000000-0000-4000-8000-000000000005}", Directory: "dirE1A2", KeyFile: "a.txt"},
		{ID: "Menu", Code: "{00000000-0000-4000-8000-000000000006}", Directory: "ProgramMenuFolder"},
	}}

	res, err := wixconvert.Convert(strings.NewReader(codesSource), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	text := string(res.YAML)
	for _, want := range []string{
		"guid: '{00000000-0000-4000-8000-000000000001}'",
		"guid: '{00000000-0000-4000-8000-000000000005}'",
		// The shortcut component keeps its code and its shortcut.
		"guid: '{00000000-0000-4000-8000-000000000006}'\n    shortcuts:\n      - name: Hello\n        file: main.exe",
		"The guid of 3 components whose Guid WiX generated comes from hello.msi.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("YAML lacks %q:\n%s", want, text)
		}
	}

	var lines []int

	for _, n := range res.Notes {
		if strings.HasPrefix(n.Message, "hello.msi has no matching component") {
			lines = append(lines, n.Line)
		}
	}

	for _, want := range []string{
		"hello.msi has no matching component for this shortcut component",
		`the WiX value HKCU\Software\Example\Hello\menu presumably stays behind`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("YAML lacks %q:\n%s", want, text)
		}
	}

	if !slices.Equal(lines, []int{17, 20, 27}) {
		t.Errorf("unmatched components in lines %v, want [17 20 27]", lines)
	}

	// Without the package, the late removal names every component, also
	// the shortcut component whose shortcut moved onto the file.
	res, err = wixconvert.Convert(strings.NewReader(codesSource), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	want := wixconvert.Note{Line: 32, Message: "WiX generated the Guid of the component in line 7, Main, the component in line 17, " +
		"the component in line 20, Menu and one more component; msi-builder generates other codes"}

	if !slices.ContainsFunc(res.Notes, func(n wixconvert.Note) bool {
		return n.Line == want.Line && strings.HasPrefix(n.Message, want.Message)
	}) {
		t.Errorf("notes %v lack %v", res.Notes, want)
	}
}
