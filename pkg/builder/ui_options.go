package builder

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

const (
	dlgInstallDir = "InstallDirDlg"
	ctlFolder     = "Folder"

	propLaunch = "LaunchOnFinish"
	ctlLaunch  = "LaunchCheckBox"
	actLaunch  = "LaunchOnFinish"
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

	file := b.launchFile(cfg.File)
	if file == nil {
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

	b.addAction(&actionInfo{name: actLaunch, typ: caExec | caAsyncNoWait, source: file.Component.dir.id, target: target, fromUI: true})

	shown := fmt.Sprintf(`NOT Installed AND NOT ACTION = "ADMIN" AND $%s = 3`, file.Component.cfg.ID)

	caption := cfg.Text
	if caption == "" {
		caption = "&Launch [ProductName]"
	}

	// The box sits on the bottom strip, left of the buttons: Windows paints
	// the page's background bitmap over an opaque control placed on it
	// until the mouse hovers over the control, and a CheckBox fills its
	// rectangle with the button face colour, which the strip has too.
	exit := wiz.byName[dlgExit]
	exit.control(&uiControl{
		name: ctlLaunch, typ: typCheckBox, property: propLaunch, text: caption, ring: true,
		x: margin, y: buttonY, w: backX - margin - 5, h: buttonH, attrs: ctlActive,
	}).when("Hide", "NOT ("+shown+")")

	finish := exit.find(ctlFinish)
	finish.events = slices.Insert(finish.events, 0, uiEvent{event: "DoAction", arg: actLaunch, cond: propLaunch + ` = "1" AND ` + shown})

	wiz.checkBoxes[propLaunch] = "1"

	if cfg.Checked == nil || *cfg.Checked {
		b.properties[propLaunch] = "1"
	}
}

// launchFile returns the program ui.launch starts, or nil after an error.
// key is a File key or the long name of a file without an id.
func (b *build) launchFile(key string) *fileInfo {
	file := b.fileByKey[key]
	if file == nil {
		for _, fi := range b.files {
			if config.FileByRef([]*config.File{fi.cfg}, key) != nil {
				file = fi

				break
			}
		}
	}

	if file == nil {
		b.errorf("ui.launch.file", "file %q is not defined", key)

		return nil
	}

	// A type 34 action starts the file with process creation, which runs
	// programs only, not documents or libraries.
	if ext := strings.ToLower(path.Ext(file.LongName)); !file.isPE || ext != ".exe" && ext != ".com" {
		b.errorf("ui.launch.file", "%s is not a program (a PE file named .exe or .com), so the launch action cannot start it", key)

		return nil
	}

	return file
}
