package builder //nolint:testpackage // the tests reach the unexported checkUI rules

import (
	"strings"
	"testing"
)

func TestCheckControlRules(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		ctl  *uiControl
		want string
	}{
		{"misspelt event", (&uiControl{name: "OK", typ: typPushButton, attrs: ctlActive}).on("RmShutdownAndRestart", "0", condAlways), `Windows Installer only knows "RMShutdownAndRestart"`},
		{"icon not in Binary", &uiControl{name: "ErrorIcon", typ: typIcon, text: "Missing"}, "not in the Binary table"},
		{"bitmap not in Binary", &uiControl{name: "Banner", typ: "Bitmap", text: "Missing"}, "not in the Binary table"},
		{"good", (&uiControl{name: "OK", typ: typPushButton, attrs: ctlActive}).on("RMShutdownAndRestart", "0", condAlways), ""},
		{"good icon", &uiControl{name: "ErrorIcon", typ: typIcon, text: binError}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bld := &build{}
			wiz := &wizard{byName: map[string]*uiDialog{}}
			bld.checkControl(wiz, "ui.Test", tc.ctl, map[string]bool{}, map[string]bool{binError: true})

			got := ""
			if len(bld.diags) > 0 {
				got = bld.diags[0].Message
			}

			if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
				t.Errorf("diagnostics %v, want %q", bld.diags, tc.want)
			}
		})
	}
}

// TestCheckDialogBounds guards against error 2826: a control that reaches
// past an edge of its dialog.
func TestCheckDialogBounds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		ctl  *uiControl
		want string
	}{
		{"inside", &uiControl{name: "Line", typ: "Line", x: 0, y: 234, w: 370, h: 0}, ""},
		{"past the right edge", &uiControl{name: "Line", typ: "Line", x: 0, y: 234, w: 373, h: 0}, "extends beyond the 370x270 dialog (error 2826)"},
		{"past the bottom edge", &uiControl{name: "Text", typ: "Text", x: 20, y: 260, w: 100, h: 20}, "error 2826"},
		{"left of the dialog", &uiControl{name: "Text", typ: "Text", x: -1, y: 20, w: 100, h: 20}, "error 2826"},
		{"above the dialog", &uiControl{name: "Text", typ: "Text", x: 20, y: -1, w: 100, h: 20}, "error 2826"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			bld := &build{}
			dialog := &uiDialog{name: "Test", first: tc.ctl.name, width: 370, height: 270, controls: []*uiControl{tc.ctl}}
			bld.checkDialog(&wizard{byName: map[string]*uiDialog{}}, dialog, map[string]bool{}, map[string]bool{})

			got := ""
			if len(bld.diags) > 0 {
				got = bld.diags[0].Message
			}

			if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
				t.Errorf("diagnostics %v, want %q", bld.diags, tc.want)
			}
		})
	}
}
