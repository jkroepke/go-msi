package builder_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jkroepke/msi-builder/internal/builder"
	"github.com/jkroepke/msi-builder/internal/testpe"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// removeConfig is the minimal example with a wizard and the given dialogs.
func removeConfig(t *testing.T, dialogs ...*config.Dialog) *config.Config {
	t.Helper()

	exe := testpe.Build(testpe.Options{FileVersion: "1.0.0.0", Languages: []int{1033}})
	cfg := example(t, "minimal", nil, map[string][]byte{"hello.exe": exe})
	cfg.UI.Enabled = true
	cfg.UI.Dialogs = dialogs
	cfg.Properties = config.Properties{"DELETE_CONFIGURATION": {}}

	return cfg
}

func checkBoxDialog(id, after, flow string) *config.Dialog {
	return &config.Dialog{
		ID: id, Title: "Remove options", After: after, Flow: flow,
		Fields: []config.DialogField{{Property: "DELETE_CONFIGURATION", Label: "&Delete the configuration", Type: config.FieldCheckBox}},
	}
}

// TestRemoveFlowDialogs puts two remove dialogs between Remove in the
// maintenance menu and VerifyRemoveDlg, the first one after
// MaintenanceTypeDlg by default, and leaves the install flow alone.
func TestRemoveFlowDialogs(t *testing.T) {
	t.Parallel()

	cfg := removeConfig(t,
		checkBoxDialog("RemoveOptionsDlg", "", config.FlowRemove),
		checkBoxDialog("RemoveMoreDlg", "RemoveOptionsDlg", config.FlowRemove),
	)
	res := mustBuild(t, cfg)

	for _, tc := range []struct{ dialog, control, want string }{
		{"MaintenanceTypeDlg", "Remove", "RemoveOptionsDlg"},
		{"RemoveOptionsDlg", "Back", "MaintenanceTypeDlg"},
		{"RemoveOptionsDlg", "Next", "RemoveMoreDlg"},
		{"RemoveMoreDlg", "Back", "RemoveOptionsDlg"},
		{"RemoveMoreDlg", "Next", "VerifyRemoveDlg"},
		{"VerifyRemoveDlg", "Back", "RemoveMoreDlg"},
		{"WelcomeDlg", "Next", "VerifyReadyDlg"},
	} {
		if got := nextOf(t, res, tc.dialog, tc.control); len(got) != 1 || got["1"] != tc.want {
			t.Errorf("%s %s goes to %v, want %s", tc.dialog, tc.control, got, tc.want)
		}
	}

	// Remove still ends the dialog only on the confirmation page.
	if got := eventsOf(t, res, "VerifyRemoveDlg", "Remove"); !slices.Equal(got, []string{"Remove ALL [1]", "EndDialog Return [1]"}) {
		t.Errorf("VerifyRemoveDlg Remove publishes %v", got)
	}

	// The removal can be reached from Change in Add or Remove Programs.
	for _, row := range rows(t, res.Database, "Property") {
		if row[0] == "ARPNOMODIFY" {
			t.Errorf("ARPNOMODIFY = %v with a remove dialog", row[1])
		}
	}

	checkRings(t, res)
}

// TestRemoveFlowNoModify keeps an explicit ARPNOMODIFY.
func TestRemoveFlowNoModify(t *testing.T) {
	t.Parallel()

	cfg := removeConfig(t, checkBoxDialog("RemoveOptionsDlg", "", config.FlowRemove))
	cfg.Product.ARP.NoModify = true
	res := mustBuild(t, cfg)

	if got := cell(t, res.Database, "Property", "Value", func(r []any) bool { return r[0] == "ARPNOMODIFY" }); got != "1" {
		t.Errorf("ARPNOMODIFY = %v", got)
	}
}

func TestRemoveFlowErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		dialogs []*config.Dialog
		want    string
	}{
		{"install anchor", []*config.Dialog{checkBoxDialog("RemoveOptionsDlg", "WelcomeDlg", config.FlowRemove)}, "in the remove flow; use MaintenanceTypeDlg"},
		{"confirmation anchor", []*config.Dialog{checkBoxDialog("RemoveOptionsDlg", "VerifyRemoveDlg", config.FlowRemove)}, "in the remove flow"},
		{"remove anchor in install", []*config.Dialog{
			checkBoxDialog("RemoveOptionsDlg", "", config.FlowRemove),
			checkBoxDialog("ExtraDlg", "RemoveOptionsDlg", ""),
		}, "in the install flow; use WelcomeDlg"},
		{"maintenance anchor in install", []*config.Dialog{checkBoxDialog("ExtraDlg", "MaintenanceTypeDlg", "")}, "in the install flow"},
		{"confirmation name", []*config.Dialog{checkBoxDialog("VerifyRemoveDlg", "", config.FlowRemove)}, "is a wizard dialog"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := builder.Build(removeConfig(t, tc.dialogs...), builder.Options{BuildTime: buildTime})
			if err == nil || !strings.Contains(fmt.Sprint(res.Diagnostics), tc.want) {
				t.Errorf("err %v, diagnostics %s", err, res.Diagnostics)
			}
		})
	}
}
