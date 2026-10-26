package builder

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
		{"icon not in Binary", &uiControl{name: "ErrorIcon", typ: "Icon", text: "Missing"}, "not in the Binary table"},
		{"bitmap not in Binary", &uiControl{name: "Banner", typ: "Bitmap", text: "Missing"}, "not in the Binary table"},
		{"good", (&uiControl{name: "OK", typ: typPushButton, attrs: ctlActive}).on("RMShutdownAndRestart", "0", condAlways), ""},
		{"good icon", &uiControl{name: "ErrorIcon", typ: "Icon", text: binError}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := &build{}
			wiz := &wizard{byName: map[string]*uiDialog{}}
			b.checkControl(wiz, "ui.Test", tc.ctl, map[string]bool{}, map[string]bool{binError: true})

			got := ""
			if len(b.diags) > 0 {
				got = b.diags[0].Message
			}

			if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
				t.Errorf("diagnostics %v, want %q", b.diags, tc.want)
			}
		})
	}
}
