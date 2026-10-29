package builder_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

func desktop(t *testing.T) *config.Config {
	t.Helper()

	return example(t, "desktop", map[string]string{"VERSION": "2.0.0"}, map[string][]byte{
		"viewer.exe":  testpe.Build(testpe.Options{FileVersion: "2.0.0.0", Languages: []int{1033}}),
		"LICENSE.txt": []byte("Example license\n"),
	})
}

// dualDesktop is the desktop example as a perUserOrMachine package, without
// the HKLM rows that one cannot have.
func dualDesktop(t *testing.T) *config.Config {
	t.Helper()

	cfg := desktop(t)
	cfg.Package.Scope = config.ScopePerUserOrMachine
	cfg.Components[0].RemoveRegistry = nil
	cfg.Properties = nil

	return cfg
}

// eventsOf returns the events of one control as "Event Argument [Condition]"
// in their order.
func eventsOf(t *testing.T, res *builder.Result, dialog, control string) []string {
	t.Helper()

	events := map[int]string{}

	for _, row := range rows(t, res.Database, "ControlEvent") {
		if row[0] == dialog && row[1] == control {
			order, ok := row[5].(int)
			if !ok {
				t.Fatalf("ordering %v", row)
			}

			events[order] = fmt.Sprintf("%v %v [%v]", row[2], row[3], row[4])
		}
	}

	out := make([]string, 0, len(events))
	for i := 1; i <= len(events); i++ {
		out = append(out, events[i])
	}

	return out
}

func TestChooseDirectory(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, desktop(t))

	for _, want := range []struct {
		dialog, control string
		next            map[string]string
	}{
		{"WelcomeDlg", "Next", map[string]string{"1": "LicenseAgreementDlg"}},
		{"LicenseAgreementDlg", "Next", map[string]string{"1": "InstallDirDlg"}},
		{"InstallDirDlg", "Back", map[string]string{"1": "LicenseAgreementDlg"}},
		{"InstallDirDlg", "Next", map[string]string{"1": "VerifyReadyDlg"}},
		{"VerifyReadyDlg", "Back", map[string]string{"NOT Installed": "InstallDirDlg", "Installed": "MaintenanceTypeDlg"}},
	} {
		if got := nextOf(t, res, want.dialog, want.control); fmt.Sprint(got) != fmt.Sprint(want.next) {
			t.Errorf("%s %s goes to %v, want %v", want.dialog, want.control, got, want.next)
		}
	}

	if got := controlRow(t, res, "InstallDirDlg", "Folder"); got[2] != "PathEdit" || got[8] != "INSTALLDIR" {
		t.Errorf("folder control %v", got)
	}

	// Next checks the typed path before it leaves the page.
	if got := eventsOf(t, res, "InstallDirDlg", "Next"); len(got) != 2 || got[0] != "SetTargetPath INSTALLDIR [1]" {
		t.Errorf("Next events %v", got)
	}

	if got := eventsOf(t, res, "InstallDirDlg", "ChangeFolder"); fmt.Sprint(got) != "[[_BrowseProperty] INSTALLDIR [1] SpawnDialog BrowseDlg [1]]" {
		t.Errorf("Change events %v", got)
	}

	controlRow(t, res, "BrowseDlg", "PathEdit")
	checkRings(t, res)
}

func TestChooseDirectoryWithFeatureTree(t *testing.T) {
	t.Parallel()

	cfg := example(t, "branded", map[string]string{"VERSION": "1.5.0"}, brandedInputs(t, nil))
	cfg.UI.ChooseDirectory = true
	cfg.UI.Dialogs = []*config.Dialog{{ID: "ExtraDlg", Title: "Extra", After: "InstallDirDlg", Fields: []config.DialogField{{Property: "PROP"}}}}
	res := mustBuild(t, cfg)

	// An installed product's Change and Back step over the license and the
	// install folder page.
	for _, want := range []struct {
		dialog, control string
		next            map[string]string
	}{
		{"LicenseAgreementDlg", "Next", map[string]string{"1": "InstallDirDlg"}},
		{"InstallDirDlg", "Next", map[string]string{"1": "ExtraDlg"}},
		{"ExtraDlg", "Back", map[string]string{"NOT Installed": "InstallDirDlg", "Installed": "MaintenanceTypeDlg"}},
		{"ExtraDlg", "Next", map[string]string{"1": "CustomizeDlg"}},
		{"CustomizeDlg", "Back", map[string]string{"1": "ExtraDlg"}},
		{"MaintenanceTypeDlg", "Change", map[string]string{"1": "ExtraDlg"}},
	} {
		if got := nextOf(t, res, want.dialog, want.control); fmt.Sprint(got) != fmt.Sprint(want.next) {
			t.Errorf("%s %s goes to %v, want %v", want.dialog, want.control, got, want.next)
		}
	}

	checkRings(t, res)
}

func TestLaunchCheckBox(t *testing.T) {
	t.Parallel()

	res := mustBuild(t, desktop(t))

	// The example's searched property adds its own type-51 actions.
	var launch []any

	for _, row := range rows(t, res.Database, "CustomAction") {
		if row[0] == "LaunchOnFinish" {
			launch = row
		}
	}

	if launch == nil || fmt.Sprint(launch[:4]) != `[LaunchOnFinish 226 INSTALLDIR "[#viewer.exe]"]` {
		t.Errorf("custom action %v", launch)
	}

	if strings.Contains(fmt.Sprint(res.Diagnostics), "not scheduled") {
		t.Errorf("diagnostics %s", res.Diagnostics)
	}

	shown := `NOT Installed AND NOT ACTION = "ADMIN" AND $viewer.exe = 3`

	// The box sits on the bottom strip, off the background picture; it
	// starts visible and is hidden by condition.
	if got := controlRow(t, res, "ExitDlg", "LaunchCheckBox"); got[2] != "CheckBox" || got[4] != 243 || got[7] != 3 || got[8] != "LaunchOnFinish" ||
		got[9] != "&Launch [ProductName]" {
		t.Errorf("check box %v", got)
	}

	if got := cell(t, res.Database, "ControlCondition", "Condition", func(r []any) bool {
		return r[0] == "ExitDlg" && r[1] == "LaunchCheckBox" && r[2] == "Hide"
	}); got != "NOT ("+shown+")" {
		t.Errorf("Hide condition %v", got)
	}

	want := fmt.Sprintf(`[DoAction LaunchOnFinish [LaunchOnFinish = "1" AND %s] EndDialog Return [1]]`, shown)
	if got := eventsOf(t, res, "ExitDlg", "Finish"); fmt.Sprint(got) != want {
		t.Errorf("Finish events %v", got)
	}

	if got := cell(t, res.Database, "Property", "Value", func(r []any) bool { return r[0] == "LaunchOnFinish" }); got != "1" {
		t.Errorf("LaunchOnFinish = %v", got)
	}

	checkRings(t, res)

	cfg := desktop(t)
	unchecked := false
	cfg.UI.Launch.Checked = &unchecked
	cfg.UI.Launch.Arguments = `--open "[PersonalFolder]"`
	res = mustBuild(t, cfg)

	for _, row := range rows(t, res.Database, "Property") {
		if row[0] == "LaunchOnFinish" {
			t.Errorf("unchecked box sets %v", row)
		}
	}

	if got := cell(t, res.Database, "CustomAction", "Target", func(r []any) bool { return r[0] == "LaunchOnFinish" }); got != `"[#viewer.exe]" --open "[PersonalFolder]"` {
		t.Errorf("target %v", got)
	}
}

func TestFinishText(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.UI.FinishText = "Open a new terminal to use [ProductName] & its tools.\n"
	res := mustBuild(t, cfg)

	// The paragraph sits below the standard text, over the background
	// picture, and keeps "&" as it is.
	if got := controlRow(t, res, "ExitDlg", "FinishText"); fmt.Sprint(got[2:10]) != "[Text 135 135 220 89 196611 <nil> Open a new terminal to use [ProductName] & its tools.]" {
		t.Errorf("finish text %v", got)
	}

	if got := controlRow(t, res, "ExitDlg", "Description"); got[6] != 40 {
		t.Errorf("description %v", got)
	}

	if got := cell(t, res.Database, "ControlCondition", "Condition", func(r []any) bool {
		return r[0] == "ExitDlg" && r[1] == "FinishText" && r[2] == "Hide"
	}); got != `Installed OR ACTION = "ADMIN"` {
		t.Errorf("Hide condition %v", got)
	}

	// The other end pages keep their layout.
	if got := controlRow(t, res, "UserExitDlg", "Description"); got[6] != 80 {
		t.Errorf("UserExitDlg description %v", got)
	}

	checkRings(t, res)
}

func TestLaunchLongCommandLine(t *testing.T) {
	t.Parallel()

	cfg := desktop(t)
	cfg.UI.Launch.Arguments = strings.Repeat("x", 250)
	res := mustBuild(t, cfg)

	if !strings.Contains(fmt.Sprint(res.Diagnostics), "LaunchOnFinish: Target is") {
		t.Errorf("diagnostics %s", res.Diagnostics)
	}
}

// TestInstallOnlyPageLinks covers the flows around the license and install
// folder pages that the other tests leave out.
func TestInstallOnlyPageLinks(t *testing.T) {
	t.Parallel()

	field := []config.DialogField{{Property: "PROP"}}

	for _, tc := range []struct {
		name  string
		setup func(*config.Config)
		links map[[2]string]map[string]string
	}{
		{"page before both", func(c *config.Config) {
			c.UI.Dialogs = []*config.Dialog{{ID: "ExtraDlg", Title: "Extra", After: "WelcomeDlg", Fields: field}}
		}, map[[2]string]map[string]string{
			{"ExtraDlg", "Next"}:            {"NOT Installed": "LicenseAgreementDlg", "Installed": "VerifyReadyDlg"},
			{"ExtraDlg", "Back"}:            {"NOT Installed": "WelcomeDlg", "Installed": "MaintenanceTypeDlg"},
			{"LicenseAgreementDlg", "Back"}: {"1": "ExtraDlg"},
			{"InstallDirDlg", "Next"}:       {"1": "VerifyReadyDlg"},
			{"VerifyReadyDlg", "Back"}:      {"NOT Installed": "InstallDirDlg", "Installed": "ExtraDlg"},
		}},
		{"feature tree without license", func(c *config.Config) {
			c.UI.Wizard, c.UI.ChooseFeatures = true, true
			c.UI.License = ""
		}, map[[2]string]map[string]string{
			{"WelcomeDlg", "Next"}:           {"1": "InstallDirDlg"},
			{"InstallDirDlg", "Back"}:        {"1": "WelcomeDlg"},
			{"InstallDirDlg", "Next"}:        {"1": "CustomizeDlg"},
			{"CustomizeDlg", "Back"}:         {"NOT Installed": "InstallDirDlg", "Installed": "MaintenanceTypeDlg"},
			{"MaintenanceTypeDlg", "Change"}: {"1": "CustomizeDlg"},
		}},
		{"feature tree with license", func(c *config.Config) { c.UI.Wizard, c.UI.ChooseFeatures = true, true }, map[[2]string]map[string]string{
			{"CustomizeDlg", "Back"}:         {"NOT Installed": "InstallDirDlg", "Installed": "MaintenanceTypeDlg"},
			{"MaintenanceTypeDlg", "Change"}: {"1": "CustomizeDlg"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := desktop(t)
			tc.setup(cfg)
			res := mustBuild(t, cfg)

			for at, want := range tc.links {
				if got := nextOf(t, res, at[0], at[1]); fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("%s %s goes to %v, want %v", at[0], at[1], got, want)
				}
			}

			checkRings(t, res)
		})
	}
}

func TestWizardOptionErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		change func(*config.Config)
		want   string
	}{
		{"unknown file", func(c *config.Config) { c.UI.Launch.File = "missing.exe" }, `file "missing.exe" is not defined`},
		{"no wizard", func(c *config.Config) { c.UI.Wizard = false; c.UI.License = "" }, "an install folder page needs a wizard"},
		{"launch without wizard", func(c *config.Config) { c.UI.Wizard = false; c.UI.License = ""; c.UI.ChooseDirectory = false }, "a launch check box needs a wizard"},
		{"finish text without wizard", func(c *config.Config) {
			c.UI.Wizard = false
			c.UI.License = ""
			c.UI.ChooseDirectory = false
			c.UI.Launch = nil
			c.UI.FinishText = "x"
		}, "a finish page text needs a wizard"},
		{"blank finish text", func(c *config.Config) { c.UI.FinishText = " \n" }, "the finish page text is blank"},
		{"not a program", func(c *config.Config) {
			c.Components[0].Files = append(c.Components[0].Files, &config.File{Source: "LICENSE.txt", Name: "readme.txt"})
			c.UI.Launch.File = "readme.txt"
		}, "readme.txt is not a program"},
		{"library", func(c *config.Config) {
			c.Components[0].Files = append(c.Components[0].Files, &config.File{Source: "viewer.exe", Name: "helper.dll"})
			c.UI.Launch.File = "helper.dll"
		}, "helper.dll is not a program"},
		{"property taken", func(c *config.Config) { c.Properties = config.Properties{"LaunchOnFinish": {Value: "0"}} }, "set by the ui.launch check box"},
		{"install dir not configurable", func(c *config.Config) {
			c.InstallDir = config.InstallDir{}
			c.Directories = []*config.Directory{{ID: "ProgramFiles64Folder", Children: []*config.Directory{{ID: "APPDIR", Name: "Viewer"}}}}
			c.Components[0].Directory = "APPDIR"
		}, "installDir.id to name a configurable directory"},
		{"action name", func(c *config.Config) {
			c.CustomActions = append(c.CustomActions, &config.CustomAction{
				ID: "LaunchOnFinish", Error: "x", Schedule: []config.Placement{{Sequence: "execute", Before: "InstallFinalize"}},
			})
		}, "LaunchOnFinish is taken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := desktop(t)
			tc.change(cfg)

			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), tc.want) {
				t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
			}
		})
	}
}

// TestUIStrings replaces every built-in wizard text and checks that each
// replacement reaches the UI tables, so that no page keeps a hard-coded text.
func TestUIStrings(t *testing.T) {
	t.Parallel()

	// A dual package, so the scope page is in it too.
	cfg := dualDesktop(t)
	cfg.UI.Wizard, cfg.UI.ChooseFeatures = true, true
	cfg.UI.Strings = make(map[string]string)

	for key := range config.UIStrings() {
		cfg.UI.Strings[key] = "«" + key + "»"
	}

	res := mustBuild(t, cfg)

	var all strings.Builder

	for _, table := range []string{"Dialog", "Control", "RadioButton", "UIText"} {
		for _, row := range rows(t, res.Database, table) {
			for _, cell := range row {
				if text, ok := cell.(string); ok {
					all.WriteString(text + "\n")
				}
			}
		}
	}

	for key := range config.UIStrings() {
		if !strings.Contains(all.String(), "«"+key+"»") {
			t.Errorf("ui.strings.%s is not used", key)
		}
	}

	if strings.Contains(all.String(), "&Next >") {
		t.Error("a page still shows the built-in Next button")
	}
}
