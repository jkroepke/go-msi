package builder

import (
	"fmt"
	"slices"
)

const (
	dlgInstallDir = "InstallDirDlg"
	ctlFolder     = "Folder"

	propLaunch = "LaunchOnFinish"
	ctlLaunch  = "LaunchCheckBox"
	actLaunch  = "LaunchOnFinish"

	// launchY places the launch check box below the finish page's text.
	launchY = 190
	launchH = 18
)

// installOnly are the pages that only a new installation shows: Change in
// the maintenance menu and the Next and Back buttons of an installed
// product step over them.
var installOnly = map[string]bool{dlgLicense: true, dlgInstallDir: true} //nolint:gochecknoglobals // lookup table

// installDirDialog adds the install folder page: the folder in a path edit
// and Change, which opens the folder browser on the same directory. Next
// sets the typed path as the directory's target, which checks it.
func (w *wizard) installDirDialog(dir string) {
	page := w.add(&uiDialog{name: dlgInstallDir, attrs: dlgPage, first: ctlFolder})
	page.innerHeader("Destination Folder", "Click Next to install to the default folder or click Change to choose another.")
	page.control(text("FolderLabel", margin+5, 60, dlgWidth-2*margin-10, 12, "Install [ProductName] to:"))
	page.control(&uiControl{
		name: ctlFolder, typ: typPathEdit, property: dir, ring: true,
		x: margin + 5, y: 75, w: 260, h: 18, attrs: ctlActive,
	})
	page.control(&uiControl{name: "ChangeFolder", typ: typPushButton, x: 292, y: 75, w: buttonW, h: buttonH, attrs: ctlActive, text: "&Change...", ring: true}).
		on("["+propBrowse+"]", dir, condAlways).
		on(evSpawnDialog, dlgBrowse, condAlways)
	page.navigation()
	page.find(ctlNext).on("SetTargetPath", dir, condAlways)

	w.browseDialog()
}

// launch adds ui.launch: a check box on the finish page and an action that
// Finish runs when it is checked. The program starts from the client
// process without waiting, so it runs as the user who started the setup.
// The box shows only after a new installation that installed the file.
func (b *build) launch(wiz *wizard) {
	cfg := b.cfg.UI.Launch
	if cfg == nil {
		return
	}

	file := b.fileByKey[cfg.File]
	if file == nil {
		b.errorf("ui.launch.file", "file %q is not defined", cfg.File)

		return
	}

	if b.action(actLaunch) != nil {
		b.errorf("ui.launch", "the custom action name %s is taken", actLaunch)

		return
	}

	target := `"[#` + file.Key + `]"`
	if cfg.Arguments != "" {
		target += " " + cfg.Arguments
	}

	if len(target) > maxTargetLength {
		b.errorf("ui.launch.arguments", "the command line is longer than %d characters", maxTargetLength)

		return
	}

	b.actions = append(b.actions, &actionInfo{name: actLaunch, typ: caExec | caAsyncNoWait, source: file.Component.dir.id, target: target, fromUI: true})

	shown := fmt.Sprintf(`NOT Installed AND NOT ACTION = "ADMIN" AND $%s = 3`, file.Component.cfg.ID)

	label := cfg.Text
	if label == "" {
		label = "&Launch [ProductName]"
	}

	exit := wiz.byName[dlgExit]
	box := exit.control(&uiControl{
		name: ctlLaunch, typ: typCheckBox, property: propLaunch, text: label, ring: true,
		x: sideTextX, y: launchY, w: sideTextW, h: launchH, attrs: ctlEnabled, // shown by its condition
	})
	box.when("Show", shown)

	finish := exit.find(ctlFinish)
	finish.events = slices.Insert(finish.events, 0, uiEvent{event: "DoAction", arg: actLaunch, cond: propLaunch + ` = "1" AND ` + shown})

	wiz.checkBoxes[propLaunch] = "1"

	if cfg.Checked == nil || *cfg.Checked {
		b.properties[propLaunch] = "1"
	}
}
