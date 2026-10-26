package builder

import (
	"strings"
)

// Control types that must bind a property (spec section 19.3).
var boundControls = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"CheckBox": true, "ComboBox": true, typDirCombo: true, typDirList: true, "Edit": true, typListBox: true,
	"ListView": true, "MaskedEdit": true, typPathEdit: true, "RadioButtonGroup": true, "SelectionTree": true, "VolumeSelectCombo": true,
}

// Events whose argument names a dialog (ICE44).
var dialogEvents = map[string]bool{ //nolint:gochecknoglobals // lookup table
	evNewDialog: true, evSpawnDialog: true, "SpawnWaitDialog": true, "SelectionBrowse": true,
}

var endDialogArgs = map[string]bool{ //nolint:gochecknoglobals // lookup table
	endReturn: true, endExit: true, "Retry": true, "Ignore": true,
	"ErrorOk": true, "ErrorCancel": true, "ErrorAbort": true, "ErrorRetry": true, "ErrorIgnore": true, "ErrorYes": true, "ErrorNo": true,
}

const (
	maxDialogName  = 72
	maxControlName = 50
)

// checkUI applies the UI rules of spec sections 7.2 and 7.5 to the
// generated tables: it guards the generator and the added dialogs.
func (b *build) checkUI(wiz *wizard) {
	styles := map[string]bool{styleDefault: true, styleTitle: true, styleBig: true}
	pictures := map[string]bool{binBanner: true, binDialog: true}

	for _, d := range wiz.dialogs {
		b.checkDialog(wiz, d, styles, pictures)
	}
}

func (b *build) checkDialog(wiz *wizard, dialog *uiDialog, styles, pictures map[string]bool) {
	path := "ui." + dialog.name

	if len(dialog.name) > maxDialogName {
		b.errorf(path, "dialog name is longer than %d characters", maxDialogName)
	}

	if dialog.find(dialog.first) == nil {
		b.errorf(path, "Control_First %q is not a control of the dialog (ICE23)", dialog.first)
	}

	for _, ctl := range dialog.controls {
		b.checkControl(wiz, path, ctl, styles, pictures)
	}
}

func (b *build) checkControl(wiz *wizard, path string, ctl *uiControl, styles, pictures map[string]bool) { //nolint:cyclop // one rule per branch
	if len(ctl.name) > maxControlName {
		b.errorf(path, "control name %q is longer than %d characters", ctl.name, maxControlName)
	}

	if ctl.typ == typPushButton && len(ctl.events) == 0 && ctl.attrs&ctlEnabled != 0 {
		b.errorf(path, "push button %s publishes no event (ICE17)", ctl.name)
	}

	if ctl.typ == "Bitmap" && !pictures[ctl.text] {
		b.errorf(path, "control %s shows %q, which is not in the Binary table (ICE17)", ctl.name, ctl.text)
	}

	if boundControls[ctl.typ] && ctl.property == "" {
		b.errorf(path, "control %s of type %s needs a property (error 2819)", ctl.name, ctl.typ)
	}

	if end := strings.IndexByte(ctl.text, '}'); strings.HasPrefix(ctl.text, "{\\") && end > 2 && ctl.typ != typScrollText {
		style := ctl.text[2:end]
		if !styles[style] {
			b.errorf(path, "control %s uses the text style %q, which is not in the TextStyle table (ICE31)", ctl.name, style)
		}
	}

	for _, event := range ctl.events {
		if _, ok := wiz.byName[event.arg]; dialogEvents[event.event] && !ok {
			b.errorf(path, "control %s opens dialog %q, which does not exist (ICE44)", ctl.name, event.arg)
		}

		if event.event == evEndDialog && !endDialogArgs[event.arg] {
			b.errorf(path, "control %s ends the dialog with %q, which is not an EndDialog argument", ctl.name, event.arg)
		}
	}
}
