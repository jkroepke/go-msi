package wixconvert_test

import (
	"bytes"
	"fmt"
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
    <Component Id="Unmanaged" Directory="ProgramMenuFolder" Guid="" Permanent="yes">
      <Shortcut Name="Hello tools" Target="[#main.exe]" Arguments="tools" />
    </Component>
    <Component Directory="INSTALLFOLDER" Subdirectory="bin">
      <File Source="tool.exe" />
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
		// A Subdirectory folder has a generated ID in the package.
		{ID: "cmpE", Code: "{00000000-0000-4000-8000-000000000007}", Directory: "dirB7C3", KeyFile: "tool.exe"},
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
		"guid: '{00000000-0000-4000-8000-000000000007}'",
		// An unmanaged shortcut component keeps its shortcut too.
		"guid: none\n    permanent: true\n    shortcuts:\n      - name: Hello tools",
		"The guid of 4 components whose Guid WiX generated comes from hello.msi.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("YAML lacks %q:\n%s", want, text)
		}
	}

	cfg, err := config.Parse(res.YAML, config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if err := config.Validate(cfg).Err(); err != nil {
		t.Errorf("the converted file does not validate: %v", err)
	}

	var lines []int

	for _, n := range res.Notes {
		if strings.HasPrefix(n.Message, "hello.msi has no matching component") {
			lines = append(lines, n.Line)
		}
	}

	for _, want := range []string{
		`the WiX value HKCU\Software\Example\Hello\menu stays behind`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("YAML lacks %q:\n%s", want, text)
		}
	}

	if !slices.Equal(lines, []int{17, 20, 27}) {
		t.Errorf("unmatched components in lines %v, want [17 20 27]", lines)
	}

	// Without the package, the late removal names every component, also
	// the shortcut component.
	res, err = wixconvert.Convert(strings.NewReader(codesSource), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	want := wixconvert.Note{Line: 38, Message: "WiX generated the Guid of a.txt, Main, notes.txt, readme.txt, Menu and 2 more components; " +
		"msi-builder generates other codes"}

	if !slices.ContainsFunc(res.Notes, func(n wixconvert.Note) bool {
		return n.Line == want.Line && strings.HasPrefix(n.Message, want.Message)
	}) {
		t.Errorf("notes %v lack %v", res.Notes, want)
	}
}

// A row that a source Id names is not matched again by key file.
func TestConvertBuiltCodesOnce(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <Component Id="X" Directory="INSTALLFOLDER">
      <File Source="a.txt" />
    </Component>
    <Component Directory="INSTALLFOLDER" Subdirectory="copy">
      <File Source="a.txt" />
    </Component>
  </Package>
</Wix>
`
	built := &wixconvert.BuiltPackage{Name: "hello.msi", Components: []wixconvert.BuiltComponent{
		{ID: "X", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "INSTALLFOLDER", KeyFile: "a.txt"},
	}}

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	if n := strings.Count(string(res.YAML), "guid: '{00000000-0000-4000-8000-000000000001}'"); n != 1 {
		t.Errorf("the code is pinned %d times:\n%s", n, res.YAML)
	}

	if !slices.ContainsFunc(res.Notes, func(n wixconvert.Note) bool {
		return n.Line == 9 && strings.HasPrefix(n.Message, "hello.msi has no matching component")
	}) {
		t.Errorf("notes %v lack the unmatched component in line 9", res.Notes)
	}
}

// A Files element: one file outside a component matches by its name, and a
// glob, which became a folder source, keeps the codes of the files it adds
// in guids, found by their path below its folder, also through a "."
// folder; a file it excludes, or one another component took or names by
// its Guid, keeps none there. A component in a folder WiX named matches its
// key file in that folder only, not a harvested file of the same name. A glob without a file in
// the package gets the note instead, also in a component with an Id.
func TestConvertBuiltCodesFiles(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade Schedule="afterInstallExecute" DowngradeErrorMessage="A newer version is installed." />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <Files Include="dist\tools\**" Directory="INSTALLFOLDER" Subdirectory="tools"><Exclude Files="dist\tools\*.pdb" /></Files>
    <Files Include="dist\LICENSE.txt" Directory="INSTALLFOLDER" />
    <Component Id="Tools" Directory="INSTALLFOLDER"><Files Include="dist\more\*.dll" /></Component>
    <Component Directory="INSTALLFOLDER" Subdirectory="tools"><File Source="build\helper.exe" /></Component>
    <Component Directory="INSTALLFOLDER" Subdirectory="tools" Guid="00000000-0000-4000-8000-000000000006"><File Source="build\config.xml" /></Component>
  </Package>
</Wix>
`
	built := &wixconvert.BuiltPackage{
		Name: "hello.msi",
		Components: []wixconvert.BuiltComponent{
			{ID: "fil1", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "INSTALLFOLDER", KeyFile: "LICENSE.txt"},
			{ID: "fil2", Code: "{00000000-0000-4000-8000-000000000002}", Directory: "dirA1", KeyFile: "tool.exe"},
			{ID: "fil4", Code: "{00000000-0000-4000-8000-000000000004}", Directory: "dirA2", KeyFile: "Help.md"},
			{ID: "fil5", Code: "{00000000-0000-4000-8000-000000000005}", Directory: "dirA1", KeyFile: "tool.pdb"},
			{ID: "Tools", Code: "{00000000-0000-4000-8000-000000000003}", Directory: "INSTALLFOLDER", KeyFile: "a.dll"},
			{ID: "cmp6", Code: "{00000000-0000-4000-8000-000000000006}", Directory: "dirA1", KeyFile: "config.xml"},
			{ID: "cmp7", Code: "{00000000-0000-4000-8000-000000000007}", Directory: "dirA1", KeyFile: "helper.exe"},
			{ID: "fil8", Code: "{00000000-0000-4000-8000-000000000008}", Directory: "dirA2", KeyFile: "helper.exe"},
		},
		Directories: []wixconvert.BuiltDirectory{
			{ID: "TARGETDIR", Name: "SourceDir"},
			{ID: "ProgramFiles64Folder", Parent: "TARGETDIR", Name: "PFiles64"},
			{ID: "INSTALLFOLDER", Parent: "ProgramFiles64Folder", Name: "Hello"},
			{ID: "dirA1", Parent: "INSTALLFOLDER", Name: "tools"},
			{ID: "dirDot", Parent: "dirA1", Name: "."},
			{ID: "dirA2", Parent: "dirDot", Name: "docs"},
		},
	}

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	if lines := globNotes(t, res.Notes, true); !slices.Equal(lines, []int{9}) {
		t.Errorf("glob notes in lines %v, want [9]", lines)
	}

	for _, want := range []string{
		"guid: '{00000000-0000-4000-8000-000000000001}'",
		"guids:\n",
		"docs/Help.md: '{00000000-0000-4000-8000-000000000004}'",
		"tool.exe: '{00000000-0000-4000-8000-000000000002}'",
		"docs/helper.exe: '{00000000-0000-4000-8000-000000000008}'",
		"The guids of 3 files that WiX harvested with Files come from hello.msi.",
	} {
		if !strings.Contains(string(res.YAML), want) {
			t.Errorf("YAML lacks %q:\n%s", want, res.YAML)
		}
	}

	for _, code := range []string{"000000000003", "000000000005"} {
		if strings.Contains(string(res.YAML), code) {
			t.Errorf("YAML has %s:\n%s", code, res.YAML)
		}
	}

	// The components below the glob's folder keep their own codes: one
	// that WiX generated, which pinCode took first, and one from the
	// source; the folder source lists neither.
	for _, file := range []string{" helper.exe:", "config.xml:"} {
		if strings.Contains(string(res.YAML), file) {
			t.Errorf("guids lists %s\n%s", file, res.YAML)
		}
	}

	if !strings.Contains(string(res.YAML), "guid: '{00000000-0000-4000-8000-000000000007}'") {
		t.Errorf("helper.exe lost its code:\n%s", res.YAML)
	}

	res, err = wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if lines := globNotes(t, res.Notes, false); !slices.Equal(lines, []int{7, 9}) {
		t.Errorf("without a package: glob notes in lines %v, want [7 9]", lines)
	}
}

// globNotes returns the lines of the Files glob notes and fails on any
// other note about codes but the late removal of the files in lines 8 and
// 10.
func globNotes(t *testing.T, notes []wixconvert.Note, built bool) []int {
	t.Helper()

	var lines []int

	for _, note := range notes {
		switch {
		case strings.HasPrefix(note.Message, "WiX gave each file of this Files glob a component of its own"),
			strings.HasPrefix(note.Message, "hello.msi has no component for a file of this Files glob"):
			lines = append(lines, note.Line)
		case strings.Contains(note.Message, "no matching component"), strings.HasPrefix(note.Message, "WiX generated the Guid"):
			if built || !strings.Contains(note.Message, "the component in line 8, helper.exe;") {
				t.Errorf("unexpected note %v", note)
			}
		}
	}

	return lines
}

// Advertised shortcuts get a note when the old version is removed late,
// unless convert left them out.
func TestConvertAdvertisedLateRemoval(t *testing.T) {
	t.Parallel()

	const src = `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    %s
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <StandardDirectory Id="ProgramMenuFolder" />
    <Component Id="Main" Directory="INSTALLFOLDER" Guid="6d1c0a42-93b8-4e57-a2f1-0c7e5d9b3a61">
      <File Source="hello.exe">
        <Shortcut Name="Hello" Directory="ProgramMenuFolder" Advertise="yes" />
        <Shortcut Name="Hello again" Directory="ProgramMenuFolder" Advertise="yes" />
      </File>
    </Component>
    <Component Id="Menu" Directory="ProgramMenuFolder" Guid="2b8f4e17-5a0c-4d93-b6e2-9f1a3c7d5e08">
      <Shortcut Name="Hello docs" Advertise="yes" />
      <RegistryValue Root="HKCU" Key="Software\Example\Hello" Name="menu" Value="1" KeyPath="yes" />
      <Shortcut Name="Hello link" Target="[#hello.exe]" Advertise="yes" />
      <Shortcut Name="Hello folder" Target="[INSTALLFOLDER]" Advertise="yes" />
    </Component>
  </Package>
</Wix>
`

	const note = "the WiX package's advertised shortcuts (lines 10, 11, 17, 18) are deleted by this late removal"

	for upgrade, want := range map[string]bool{
		`<MajorUpgrade Schedule="afterInstallExecute" DowngradeErrorMessage="A newer version is installed." />`:    true,
		`<MajorUpgrade Schedule="afterInstallInitialize" DowngradeErrorMessage="A newer version is installed." />`: false,
		``: false,
	} {
		res, err := wixconvert.Convert(strings.NewReader(fmt.Sprintf(src, upgrade)), wixconvert.Options{})
		if err != nil {
			t.Fatal(err)
		}

		// The shortcut without a Target in line 15 is left out, so the
		// note does not list it.
		got := slices.ContainsFunc(res.Notes, func(n wixconvert.Note) bool { return strings.HasPrefix(n.Message, note) })
		if got != want {
			t.Errorf("%s: note %t, want %t: %v", upgrade, got, want, res.Notes)
		}
	}
}

// TestConvertRemoveFolderExDirectory checks that a util:RemoveFolderEx on a
// directory other than the install dir becomes a removeFolders entry that
// names the directory, whose ID the output keeps.
func TestConvertRemoveFolderExDirectory(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:util="http://wixtoolset.org/schemas/v4/wxs/util">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello">
        <Directory Id="DATADIR" Name="data" />
      </Directory>
    </StandardDirectory>
    <Feature Id="Main">
      <Component Directory="DATADIR">
        <File Source="hello.exe" />
        <util:RemoveFolderEx On="uninstall" Property="DATADIR" />
      </Component>
    </Feature>
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"- id: DATADIR", "directory: DATADIR", "- property: DATADIR"} {
		if !strings.Contains(string(res.YAML), want) {
			t.Errorf("the YAML lacks %q:\n%s", want, res.YAML)
		}
	}
}

// TestConvertRemoveFolderExBothOwnFolder checks that On="both" on the
// component's own fixed folder keeps its uninstall half as purge.
func TestConvertRemoveFolderExBothOwnFolder(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:util="http://wixtoolset.org/schemas/v4/wxs/util">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="CommonAppDataFolder">
      <Directory Id="DataDir" Name="Hello" />
    </StandardDirectory>
    <Feature Id="Main">
      <Component Directory="DataDir">
        <File Source="hello.conf" />
        <util:RemoveFolderEx On="both" Property="DataDir" />
      </Component>
    </Feature>
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"purge: true", "the install half of util:RemoveFolderEx"} {
		if !strings.Contains(string(res.YAML), want) {
			t.Errorf("the YAML lacks %q:\n%s", want, res.YAML)
		}
	}
}

// TestConvertUnmappedNotes checks the TODOs of WiX constructs whose output
// cannot validate or would be lost silently.
func TestConvertUnmappedNotes(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:ui="http://wixtoolset.org/schemas/v4/wxs/ui"
     xmlns:fw="http://wixtoolset.org/schemas/v4/wxs/firewall">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade DowngradeErrorMessage="newer" />
    <Directory Id="VOLUME">
      <Directory Id="INSTALLFOLDER" Name="Hello">
        <Component><File Source="hello.exe"><fw:FirewallException Name="Hello" Port="4985" /></File></Component>
        <Component><File Source="hellod.exe"><fw:FirewallException Name="Hello" Port="4985" /></File></Component>
      </Directory>
    </Directory>
    <SetDirectory Id="VOLUME" Value="[WindowsVolume]" />
    <CustomAction Id="Notes" Directory="INSTALLFOLDER" ExeCommand="explorer.exe https://example.com/notes" Return="asyncNoWait" />
    <ui:WixUI Id="WixUI_Minimal" />
    <UI><Publish Dialog="ExitDialog" Control="Finish" Event="DoAction" Value="Notes" Condition="NOT Installed" /></UI>
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"the Directory VOLUME has no Name", "the Finish button runs the action Notes",
		"the firewall exception on line 7 has the same name",
	} {
		if !slices.ContainsFunc(res.Notes, func(n wixconvert.Note) bool { return strings.HasPrefix(n.Message, want) }) {
			t.Errorf("no note %q in %q", want, res.Notes)
		}
	}
}

// TestConvertWindows1252 checks that a source declared windows-1252 is
// decoded: 0xE9 is é there and invalid UTF-8.
func TestConvertWindows1252(t *testing.T) {
	t.Parallel()

	src := "<?xml version=\"1.0\" encoding=\"windows-1252\"?>\n" +
		`<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <SummaryInformation Description="Hello ` + "\xe9" + `dition" />
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(res.YAML), "Hello édition") {
		t.Errorf("the description was not decoded:\n%s", res.YAML)
	}
}

// TestConvertBuiltCodeOfKeyPathID checks that a component without an Id
// finds its built row by the Id of its key file, which WiX gives it.
func TestConvertBuiltCodeOfKeyPathID(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello">
        <Component>
          <File Id="main.exe" Source="main.exe" />
        </Component>
        <!-- WiX names it after the registry value, not the file. -->
        <Component>
          <RegistryKey Root="HKLM" Key="Software\Example\Hello">
            <RegistryValue Id="installed" Name="installed" Value="1" Type="string" KeyPath="yes" />
          </RegistryKey>
          <File Id="notes.txt" Source="notes.txt" />
        </Component>
        <!-- Its key path is no file, so no built row matches its file. -->
        <Component>
          <RegistryValue Root="HKLM" Key="Software\Example\Hello" Name="extra" Value="1" Type="string" KeyPath="yes" />
          <File Source="extra.txt" />
        </Component>
        <!-- An HKCU key path before a file stays the key path. -->
        <Component Id="Prefs">
          <RegistryValue Id="prefs" Root="HKCU" Key="Software\Example\Hello" Name="prefs" Value="1" Type="integer" KeyPath="yes" />
          <File Id="prefs.ini" Source="prefs.ini" />
        </Component>
      </Directory>
    </StandardDirectory>
  </Package>
</Wix>`

	built := &wixconvert.BuiltPackage{Name: "hello.msi", Components: []wixconvert.BuiltComponent{
		{ID: "main.exe", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "INSTALLFOLDER", KeyFile: "main.exe"},
		{ID: "installed", Code: "{00000000-0000-4000-8000-000000000002}", Directory: "INSTALLFOLDER"},
		{ID: "other", Code: "{00000000-0000-4000-8000-000000000003}", Directory: "INSTALLFOLDER", KeyFile: "extra.txt"},
	}}

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"guid: '{00000000-0000-4000-8000-000000000001}'", "- id: installed\n",
		"guid: '{00000000-0000-4000-8000-000000000002}'", "keyPath: installed\n",
		"keyPath: prefs\n",
	} {
		if !strings.Contains(string(res.YAML), want) {
			t.Errorf("YAML lacks %q:\n%s", want, res.YAML)
		}
	}

	if strings.Contains(string(res.YAML), "000000000003") {
		t.Errorf("a component with a registry key path took the code of a file component:\n%s", res.YAML)
	}
}

// TestConvertKeyFileIDs checks the Ids that components without one take
// from their key file: a define is filled in, a name over 72 characters is
// no Id, and a generated Id skips the names files took.
func TestConvertKeyFileIDs(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("A", 76) + ".dll"
	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <?define Exe = "hello.exe" ?>
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade DowngradeErrorMessage="newer" />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello">
        <Component><RegistryValue Root="HKLM" Key="Software\Example\Hello" Name="a" Value="1" Type="string" KeyPath="yes" /></Component>
        <Component><File Source="Component1" /></Component>
        <Component><File Source="dist\$(var.Exe)" /></Component>
        <Component><File Source="` + long + `" /></Component>
      </Directory>
    </StandardDirectory>
    <Feature Id="Main"><ComponentRef Id="hello.exe" /></Feature>
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(res.YAML), "id: "+long) {
		t.Errorf("a name over 72 characters became an Id:\n%s", res.YAML)
	}

	if !strings.Contains(string(res.YAML), "- id: hello.exe\n") {
		t.Errorf("the component of $(var.Exe) is not hello.exe:\n%s", res.YAML)
	}

	cfg, err := config.Parse(res.YAML, config.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if err := config.Validate(cfg).Err(); err != nil {
		t.Errorf("the converted file does not validate: %v\n%s", err, res.YAML)
	}
}

// A file two Files globs add goes to the glob whose folder is nearest above
// it, among those whose source folder holds it when convert can look; with
// the same folder it goes to neither, and both name it.
func TestConvertBuiltCodesNestedGlobs(t *testing.T) {
	t.Parallel()

	const src = `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade Schedule="afterInstallExecute" DowngradeErrorMessage="A newer version is installed." />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <Files Include="bin\**" Directory="INSTALLFOLDER" />
    <Files Include="assets\**" Directory="INSTALLFOLDER" Subdirectory="assets" />
    %s
  </Package>
</Wix>
`

	built := &wixconvert.BuiltPackage{
		Name: "hello.msi",
		Components: []wixconvert.BuiltComponent{
			{ID: "fil1", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "INSTALLFOLDER", KeyFile: "app.exe"},
			{ID: "fil2", Code: "{00000000-0000-4000-8000-000000000002}", Directory: "dirA1", KeyFile: "logo.png"},
		},
		Directories: []wixconvert.BuiltDirectory{
			{ID: "TARGETDIR", Name: "SourceDir"},
			{ID: "ProgramFiles64Folder", Parent: "TARGETDIR", Name: "PFiles64"},
			{ID: "INSTALLFOLDER", Parent: "ProgramFiles64Folder", Name: "Hello"},
			{ID: "dirA1", Parent: "INSTALLFOLDER", Name: "assets"},
		},
	}

	res, err := wixconvert.Convert(strings.NewReader(fmt.Sprintf(src, "")), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		" app.exe: '{00000000-0000-4000-8000-000000000001}'",
		" logo.png: '{00000000-0000-4000-8000-000000000002}'",
	} {
		if !strings.Contains(string(res.YAML), want) {
			t.Errorf("YAML lacks %q:\n%s", want, res.YAML)
		}
	}

	if strings.Contains(string(res.YAML), "assets/logo.png") {
		t.Errorf("bin/ took logo.png:\n%s", res.YAML)
	}

	if lines := globNotes(t, res.Notes, true); len(lines) > 0 {
		t.Errorf("glob notes in lines %v, want none", lines)
	}

	// When only bin/ holds assets/logo.png next to the source, the nearer
	// assets/ glob does not take its code.
	res, err = wixconvert.Convert(strings.NewReader(fmt.Sprintf(src, "")), wixconvert.Options{
		Built: built, Exists: func(rel string) bool { return rel == "bin/assets/logo.png" || rel == "bin/app.exe" },
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(res.YAML), "assets/logo.png: '{00000000-0000-4000-8000-000000000002}'") {
		t.Errorf("bin/ lacks assets/logo.png:\n%s", res.YAML)
	}

	// A second glob into the install folder adds app.exe too.
	res, err = wixconvert.Convert(strings.NewReader(fmt.Sprintf(src, `<Files Include="lib\**" Directory="INSTALLFOLDER" />`)), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(res.YAML), "000000000001") {
		t.Errorf("a glob took a file another glob adds to the same folder:\n%s", res.YAML)
	}

	if lines := globNotes(t, res.Notes, true); len(lines) > 0 {
		t.Errorf("glob notes in lines %v, want only the tie notes", lines)
	}

	var tied []int

	for _, note := range res.Notes {
		if strings.HasPrefix(note.Message, "another Files glob into the same folder may add app.exe too") {
			tied = append(tied, note.Line)
		}
	}

	if !slices.Equal(tied, []int{7, 9}) {
		t.Errorf("tie notes in lines %v, want [7 9]", tied)
	}
}

// A file that two globs into the same folder add keeps no code, and both
// globs name it, also when each pinned other files, unless only one of
// their source folders holds it.
func TestConvertBuiltCodesTiedGlobs(t *testing.T) {
	t.Parallel()

	const src = `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade Schedule="afterInstallExecute" DowngradeErrorMessage="A newer version is installed." />
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <Files Include="bin\a*" Directory="INSTALLFOLDER" />
    <Files Include="lib\*.exe" Directory="INSTALLFOLDER" />
  </Package>
</Wix>
`

	built := &wixconvert.BuiltPackage{
		Name: "hello.msi",
		Components: []wixconvert.BuiltComponent{
			{ID: "fil1", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "INSTALLFOLDER", KeyFile: "app.exe"},
			{ID: "fil2", Code: "{00000000-0000-4000-8000-000000000002}", Directory: "INSTALLFOLDER", KeyFile: "abc.txt"},
			{ID: "fil3", Code: "{00000000-0000-4000-8000-000000000003}", Directory: "INSTALLFOLDER", KeyFile: "x.exe"},
		},
		Directories: []wixconvert.BuiltDirectory{
			{ID: "TARGETDIR", Name: "SourceDir"},
			{ID: "ProgramFiles64Folder", Parent: "TARGETDIR", Name: "PFiles64"},
			{ID: "INSTALLFOLDER", Parent: "ProgramFiles64Folder", Name: "Hello"},
		},
	}

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{Built: built})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{" abc.txt: '{00000000-0000-4000-8000-000000000002}'", " x.exe: '{00000000-0000-4000-8000-000000000003}'"} {
		if !strings.Contains(string(res.YAML), want) {
			t.Errorf("YAML lacks %q:\n%s", want, res.YAML)
		}
	}

	if strings.Contains(string(res.YAML), "000000000001") {
		t.Errorf("a glob took app.exe:\n%s", res.YAML)
	}

	var lines []int

	for _, note := range res.Notes {
		if strings.HasPrefix(note.Message, "another Files glob into the same folder may add app.exe too") {
			lines = append(lines, note.Line)
		}
	}

	if !slices.Equal(lines, []int{7, 8}) {
		t.Errorf("tie notes in lines %v, want [7 8]: %v", lines, res.Notes)
	}

	// With the source folders at hand, app.exe goes to the glob whose
	// folder holds it.
	res, err = wixconvert.Convert(strings.NewReader(src), wixconvert.Options{
		Built: built, Exists: func(rel string) bool { return rel == "lib/app.exe" },
	})
	if err != nil {
		t.Fatal(err)
	}

	if want := "app.exe: '{00000000-0000-4000-8000-000000000001}'\n"; !strings.Contains(string(res.YAML), want) ||
		strings.Index(string(res.YAML), want) < strings.Index(string(res.YAML), "source: lib/") {
		t.Errorf("lib/ lacks app.exe:\n%s", res.YAML)
	}

	for _, note := range res.Notes {
		if strings.HasPrefix(note.Message, "another Files glob") {
			t.Errorf("unexpected tie note %v", note)
		}
	}
}

// A glob into a WiX 6432 folder finds its folder under the name of the
// platform WiX built for; a glob whose folder the package lacks says so.
func TestConvertBuiltCodes6432Folder(t *testing.T) {
	t.Parallel()

	const src = `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Hello" Manufacturer="Example" Version="1.1.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <MajorUpgrade Schedule="afterInstallExecute" DowngradeErrorMessage="A newer version is installed." />
    <Files Include="dist\**" Directory="ProgramFiles6432Folder" Subdirectory="Hello" />
  </Package>
</Wix>
`

	for _, platform := range []string{"ProgramFiles64Folder", "ProgramFilesFolder"} {
		built := &wixconvert.BuiltPackage{
			Name: "hello.msi",
			Components: []wixconvert.BuiltComponent{
				{ID: "fil1", Code: "{00000000-0000-4000-8000-000000000001}", Directory: "dirH", KeyFile: "app.exe"},
			},
			Directories: []wixconvert.BuiltDirectory{
				{ID: "TARGETDIR", Name: "SourceDir"},
				{ID: platform, Parent: "TARGETDIR", Name: "PFiles"},
				{ID: "dirH", Parent: platform, Name: "Hello"},
			},
		}

		res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{Built: built})
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(res.YAML), "app.exe: '{00000000-0000-4000-8000-000000000001}'") {
			t.Errorf("%s: app.exe has no guid:\n%s", platform, res.YAML)
		}
	}

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{Built: &wixconvert.BuiltPackage{Name: "hello.msi"}})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.ContainsFunc(res.Notes, func(n wixconvert.Note) bool {
		return strings.HasPrefix(n.Message, "convert did not find the folder of this Files glob in hello.msi")
	}) {
		t.Errorf("no note on the missing folder: %v", res.Notes)
	}
}

// TestConvertCloseApplication checks the util:CloseApplication cases that
// cannot become files[].close: a target that is no program, one a Files
// glob may install, and the Property a closing element also sets.
func TestConvertCloseApplication(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:util="http://wixtoolset.org/schemas/v4/wxs/util">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello" />
    </StandardDirectory>
    <ComponentGroup Id="Harvested" Directory="INSTALLFOLDER">
      <Files Include="dist\**" />
    </ComponentGroup>
    <ComponentGroup Id="Own" Directory="INSTALLFOLDER">
      <Component>
        <File Source="hello.exe" />
        <File Source="hello.com" />
      </Component>
    </ComponentGroup>
    <Feature Id="Main"><ComponentGroupRef Id="Harvested" /><ComponentGroupRef Id="Own" /></Feature>
    <util:CloseApplication Target="hello.com" CloseMessage="yes" />
    <util:CloseApplication Target="example.exe" CloseMessage="yes" />
    <util:CloseApplication Target="hello.exe" CloseMessage="yes" Property="HELLORUNNING" PromptToContinue="yes" />
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	out := string(res.YAML)
	for _, want := range []string{
		"closes hello.com, which is no program ending in .exe",
		"closes example.exe, which is no single file of this package; when a Files glob (line 7) installs it",
		"also sets the property HELLORUNNING while hello.exe in INSTALLFOLDER runs",
		"the attributes RebootPrompt, PromptToContinue of <util:CloseApplication> map to it only roughly or not at all",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the YAML lacks %q:\n%s", want, out)
		}
	}

	if got := strings.Count(out, "close:"); got != 1 {
		t.Errorf("the YAML closes %d files, want only hello.exe:\n%s", got, out)
	}
}

// TestConvertCloseApplicationV3Src checks that a v3 File with the
// deprecated src attribute is found by its installed name.
func TestConvertCloseApplicationV3Src(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi" xmlns:util="http://schemas.microsoft.com/wix/UtilExtension">
  <Product Id="*" Name="Hello" Language="1033" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <Package InstallerVersion="500" Compressed="yes" />
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLFOLDER" Name="Hello">
          <Component Id="C" Guid="*">
            <File Id="hello" src="bin\hello.exe" KeyPath="yes" />
          </Component>
        </Directory>
      </Directory>
    </Directory>
    <Feature Id="Main"><ComponentRef Id="C" /></Feature>
    <util:CloseApplication Id="x" Target="hello.exe" CloseMessage="yes">NOT Installed</util:CloseApplication>
  </Product>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(res.YAML), "close: {terminate: false}") {
		t.Errorf("hello.exe is not closed:\n%s", res.YAML)
	}
}

// TestConvertCloseApplicationUndefinedVar checks that variables defined
// only on the WiX command line, in the Target or before the file name in
// the Source, still close the file, and that files of one source in
// several folders each get their notes.
func TestConvertCloseApplicationUndefinedVar(t *testing.T) {
	t.Parallel()

	src := `<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs" xmlns:util="http://wixtoolset.org/schemas/v4/wxs/util">
  <Package Name="Hello" Manufacturer="Example" Version="1.0.0" UpgradeCode="3f2504e0-4f89-41d3-9a0c-0305e82c3301">
    <StandardDirectory Id="ProgramFiles64Folder">
      <Directory Id="INSTALLFOLDER" Name="Hello">
        <Component>
          <File Source="bin\$(var.ExeName)" />
          <File Source="$(var.App.TargetDir)hello.exe" />
        </Component>
        <Directory Id="X86" Name="x86">
          <Component>
            <File Source="$(var.App.TargetDir)hello.exe" />
          </Component>
        </Directory>
      </Directory>
    </StandardDirectory>
    <util:CloseApplication Target="$(var.ExeName)" CloseMessage="yes" />
    <util:CloseApplication Target="hello.exe" CloseMessage="yes" Timeout="10" />
  </Package>
</Wix>`

	res, err := wixconvert.Convert(strings.NewReader(src), wixconvert.Options{})
	if err != nil {
		t.Fatal(err)
	}

	out := string(res.YAML)
	if got := strings.Count(out, "close: {terminate: false}"); got != 3 {
		t.Errorf("the YAML closes %d files, want 3:\n%s", got, out)
	}

	// Both copies of hello.exe get every note, told apart by their folder.
	if got := strings.Count(out, "the attributes RebootPrompt, Timeout of"); got != 2 {
		t.Errorf("the YAML notes Timeout on %d files, want 2:\n%s", got, out)
	}
}
