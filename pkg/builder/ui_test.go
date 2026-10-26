package builder_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jkroepke/go-msi/internal/testpe"
	"github.com/jkroepke/go-msi/pkg/builder"
	"github.com/jkroepke/go-msi/pkg/config"
)

// nextOf returns the NewDialog target of a control, with its condition.
func nextOf(t *testing.T, res *builder.Result, dialog, control string) map[string]string {
	t.Helper()

	out := make(map[string]string)

	for _, row := range rows(t, res.Database, "ControlEvent") {
		if row[0] == dialog && row[1] == control && row[2] == "NewDialog" {
			out[fmt.Sprint(row[4])] = fmt.Sprint(row[3])
		}
	}

	return out
}

func TestFeatureTreeWizard(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "0.32.0.5", Languages: []int{1033}})
	cfg := example(t, "windows_exporter", map[string]string{"VERSION": "0.32.0.5"}, map[string][]byte{
		"windows_exporter.exe": exe,
		"icon.ico":             append([]byte{0, 0, 1, 0, 1, 0}, make([]byte, 64)...),
	})
	res := mustBuild(t, cfg)
	database := res.Database

	// The added dialog sits between the feature tree and the ready page.
	if got := nextOf(t, res, "CustomizeDlg", "Next"); got["1"] != "ConfigDlg" {
		t.Errorf("CustomizeDlg Next goes to %v", got)
	}

	if got := nextOf(t, res, "ConfigDlg", "Next"); got["1"] != "VerifyReadyDlg" {
		t.Errorf("ConfigDlg Next goes to %v", got)
	}

	if got := nextOf(t, res, "VerifyReadyDlg", "Back"); got["1"] != "ConfigDlg" {
		t.Errorf("VerifyReadyDlg Back goes to %v", got)
	}

	// The page after the welcome page returns to the maintenance menu on an installed product.
	if got := nextOf(t, res, "CustomizeDlg", "Back"); got["NOT Installed"] != "WelcomeDlg" || got["Installed"] != "MaintenanceTypeDlg" {
		t.Errorf("CustomizeDlg Back goes to %v", got)
	}

	if got := nextOf(t, res, "MaintenanceTypeDlg", "Change"); got["1"] != "CustomizeDlg" {
		t.Errorf("Change goes to %v", got)
	}

	// Four edit fields bound to public properties, in one keyboard ring with the buttons.
	edits := 0

	for _, row := range rows(t, database, "Control") {
		if row[0] != "ConfigDlg" {
			continue
		}

		if row[2] == "Edit" {
			edits++

			if prop, ok := row[8].(string); !ok || !config.IsPublicProperty(prop) {
				t.Errorf("edit %v is bound to %v", row[1], row[8])
			}
		}
	}

	if edits != 4 {
		t.Errorf("%d edit controls on ConfigDlg", edits)
	}

	checkRings(t, res)

	// Entry points and end dialogs in the UI sequences (ICE20).
	for _, want := range []struct {
		table, action string
		seq           int
	}{
		{"InstallUISequence", "WelcomeDlg", 1230}, {"InstallUISequence", "MaintenanceWelcomeDlg", 1250}, {"InstallUISequence", "ProgressDlg", 1280},
		{"InstallUISequence", "ExitDlg", -1}, {"InstallUISequence", "FatalErrorDlg", -3},
		{"AdminUISequence", "ProgressDlg", 1280}, {"AdminUISequence", "UserExitDlg", -2},
	} {
		got := cell(t, database, want.table, "Sequence", func(r []any) bool { return r[0] == want.action })
		if got != want.seq {
			t.Errorf("%s %s at %v, want %d", want.table, want.action, got, want.seq)
		}
	}

	if cell(t, database, "Property", "Value", func(r []any) bool { return r[0] == "DefaultUIFont" }) != "DefaultFont" {
		t.Error("DefaultUIFont is not set")
	}

	if database.Table("ListBox") == nil || database.Table("Binary") == nil {
		t.Fatal("ListBox or Binary table missing")
	}

	if got := cell(t, database, "Binary", "Data", func(r []any) bool { return r[0] == "BannerBmp" }); got == nil {
		t.Error("BannerBmp missing")
	}

	for _, row := range rows(t, database, "Property") {
		if row[0] == "ARPNOMODIFY" {
			t.Error("a feature tree wizard must keep Modify in Apps & features")
		}
	}
}

// checkRings verifies that Control_Next forms one closed loop through
// Control_First on every dialog (ICE23), or no loop at all for the error box.
func checkRings(t *testing.T, res *builder.Result) {
	t.Helper()

	next := make(map[string]map[string]string)

	for _, row := range rows(t, res.Database, "Control") {
		dialog, control := fmt.Sprint(row[0]), fmt.Sprint(row[1])
		if next[dialog] == nil {
			next[dialog] = make(map[string]string)
		}

		if n, ok := row[10].(string); ok {
			next[dialog][control] = n
		}
	}

	for _, row := range rows(t, res.Database, "Dialog") {
		dialog, first := fmt.Sprint(row[0]), fmt.Sprint(row[7])
		ring := next[dialog]

		if len(ring) == 0 {
			continue
		}

		start, ok := ring[first]
		if !ok {
			t.Errorf("%s: Control_First %s is not in the ring", dialog, first)

			continue
		}

		seen := map[string]bool{first: true}

		for cur := start; cur != first; cur = ring[cur] {
			if seen[cur] || ring[cur] == "" {
				t.Errorf("%s: the ring does not close at %s", dialog, cur)

				break
			}

			seen[cur] = true
		}

		if len(seen) != len(ring) {
			t.Errorf("%s: %d of %d linked controls are on the loop", dialog, len(seen), len(ring))
		}
	}
}

func TestMinimalWizard(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.UI.Wizard = "minimal"
	res := mustBuild(t, cfg)

	if got := nextOf(t, res, "WelcomeDlg", "Next"); got["1"] != "VerifyReadyDlg" {
		t.Errorf("WelcomeDlg Next goes to %v", got)
	}

	for _, row := range rows(t, res.Database, "Dialog") {
		if row[0] == "CustomizeDlg" {
			t.Error("minimal wizard has a feature tree")
		}
	}

	if cell(t, res.Database, "Property", "Value", func(r []any) bool { return r[0] == "ARPNOMODIFY" }) != "1" {
		t.Error("ARPNOMODIFY is not set without feature selection")
	}

	checkRings(t, res)
}

func TestWizardErrors(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})

	for _, tc := range []struct {
		name   string
		dialog config.Dialog
		want   string
	}{
		{"bad anchor", config.Dialog{ID: "ExtraDlg", Title: "x", After: "VerifyReadyDlg", Fields: []config.DialogField{{Property: "PROP"}}}, "not a wizard page"},
		{"wizard name", config.Dialog{ID: "CancelDlg", Title: "x", After: "WelcomeDlg", Fields: []config.DialogField{{Property: "PROP"}}}, "is a wizard dialog"},
		{"license name", config.Dialog{ID: "LicenseAgreementDlg", Title: "x", After: "WelcomeDlg", Fields: []config.DialogField{{Property: "PROP"}}}, "is a wizard dialog"},
		{"install folder name", config.Dialog{ID: "InstallDirDlg", Title: "x", After: "WelcomeDlg", Fields: []config.DialogField{{Property: "PROP"}}}, "is a wizard dialog"},
		{"too many fields", config.Dialog{ID: "ExtraDlg", Title: "x", After: "WelcomeDlg", Fields: []config.DialogField{
			{Property: "A1"}, {Property: "A2"}, {Property: "A3"}, {Property: "A4"}, {Property: "A5"},
		}}, "at most 4 fields"},
		{"two check box values", config.Dialog{ID: "ExtraDlg", Title: "x", After: "WelcomeDlg", Fields: []config.DialogField{
			{Property: "OPT", Type: config.FieldCheckBox}, {Property: "OPT", Type: config.FieldCheckBox, Value: "yes"},
		}}, "another check box sets OPT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
			cfg.UI.Wizard = "minimal"
			dialog := tc.dialog
			cfg.UI.Dialogs = []*config.Dialog{&dialog}

			res, err := builder.Build(cfg, builder.Options{BuildTime: buildTime})
			if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), tc.want) {
				t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
			}
		})
	}
}

// TestDialogFieldTypes checks the password and checkbox fields of an added
// dialog: a password edit, a hidden property, and a CheckBox row with the
// value a checked box sets.
func TestDialogFieldTypes(t *testing.T) {
	t.Parallel()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.UI.Wizard = "minimal"
	cfg.UI.Dialogs = []*config.Dialog{{
		ID: "ExtraDlg", Title: "Options", After: "WelcomeDlg", Fields: []config.DialogField{
			{Property: "API_TOKEN", Label: "Token", Type: config.FieldPassword},
			{Property: "DELETE_DATA", Label: "&Delete data on uninstall", Type: config.FieldCheckBox, Value: "yes"},
		},
	}}
	res := mustBuild(t, cfg)

	controls := make(map[string][]any)

	for _, row := range rows(t, res.Database, "Control") {
		if row[0] == "ExtraDlg" {
			controls[fmt.Sprint(row[1])] = row
		}
	}

	if edit := controls["Edit1"]; edit == nil || edit[2] != "Edit" || fmt.Sprint(edit[7]) != "2097155" {
		t.Errorf("password edit %v", edit)
	}

	if box := controls["CheckBox2"]; box == nil || box[2] != "CheckBox" || box[8] != "DELETE_DATA" || box[9] != "&Delete data on uninstall" {
		t.Errorf("check box %v", box)
	}

	if controls["Label2"] != nil {
		t.Error("a check box field has a separate label")
	}

	if got := cell(t, res.Database, "CheckBox", "Value", func(r []any) bool { return r[0] == "DELETE_DATA" }); got != "yes" {
		t.Errorf("CheckBox value %v", got)
	}

	hidden := cell(t, res.Database, "Property", "Value", func(r []any) bool { return r[0] == "MsiHiddenProperties" })
	if !strings.Contains(fmt.Sprint(hidden), "API_TOKEN") {
		t.Errorf("MsiHiddenProperties %v", hidden)
	}

	if first := cell(t, res.Database, "Dialog", "Control_First", func(r []any) bool { return r[0] == "ExtraDlg" }); first != "Edit1" {
		t.Errorf("first control %v", first)
	}

	checkRings(t, res)
}
