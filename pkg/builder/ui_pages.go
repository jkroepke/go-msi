package builder

// The pages of the wizard. Texts are msi-builder's own; the dialog keys follow
// the names the specification uses for the standard flow (spec section
// 10.3) so that ui.dialogs can anchor on them.

const (
	ctlInstall = "Install"
	ctlRepair  = "Repair"
	ctlRemove  = "Remove"
	ctlFinish  = "Finish"
	ctlRetry   = "Retry"
	unitBytes  = "bytes"
)

// commonDialogs adds the pages every wizard has.
func (w *wizard) commonDialogs(tree, hasComponents bool) {
	welcome := w.add(&uiDialog{name: dlgWelcome, attrs: dlgPage, first: ctlNext})
	welcome.sideHeader("Welcome to the [ProductName] Setup Wizard",
		"The Setup Wizard will install [ProductName] on your computer. Click Next to continue or Cancel to exit the Setup Wizard.")
	welcome.navigation()
	welcome.find(ctlBack).attrs = ctlDisabled
	welcome.find(ctlBack).ring = false

	if hasComponents {
		welcome.find(ctlNext).on("SpawnWaitDialog", dlgWaitCosting, "CostingComplete = 1")
	}

	w.verifyPages()
	w.maintenancePages(tree)
	w.endPages()
	w.progressPage()
	w.modalBoxes()
}

func (w *wizard) verifyPages() {
	ready := w.add(&uiDialog{name: dlgVerifyReady, attrs: dlgPage, first: ctlInstall})
	ready.innerHeader("Ready to install [ProductName]",
		"Click Install to begin the installation. Click Back to review or change any of your settings. Click Cancel to exit the wizard.")
	ready.control(strip())
	ready.control(button(ctlBack, backX, buttonY, "< &Back"))
	ready.control(button(ctlInstall, nextX, buttonY, "&Install")).on(evEndDialog, endReturn, condAlways)
	ready.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
	ready.def, ready.cancel = ctlInstall, ctlCancel

	repair := w.add(&uiDialog{name: dlgVerifyRepair, attrs: dlgPage, first: ctlRepair})
	repair.innerHeader("Ready to repair [ProductName]",
		"Click Repair to repair the installation of [ProductName]. Click Back to return to the maintenance menu or Cancel to exit the wizard.")
	repair.control(strip())
	repair.control(button(ctlBack, backX, buttonY, "< &Back")).on(evNewDialog, dlgMaintType, condAlways)
	repair.control(button(ctlRepair, nextX, buttonY, "&Repair")).
		on("ReinstallMode", "ecmus", condAlways).
		on("Reinstall", "ALL", condAlways).
		on(evEndDialog, endReturn, condAlways)
	repair.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
	repair.def, repair.cancel = ctlRepair, ctlCancel

	remove := w.add(&uiDialog{name: dlgVerifyRemove, attrs: dlgPage, first: ctlRemove})
	remove.innerHeader("Remove [ProductName]",
		"Click Remove to remove [ProductName] from your computer. Click Back to return to the maintenance menu or Cancel to exit the wizard.")
	remove.control(strip())
	remove.control(button(ctlBack, backX, buttonY, "< &Back")).on(evNewDialog, dlgMaintType, condAlways)
	remove.control(button(ctlRemove, nextX, buttonY, "&Remove")).
		on("Remove", "ALL", condAlways).
		on(evEndDialog, endReturn, condAlways)
	remove.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
	remove.def, remove.cancel = ctlRemove, ctlCancel
}

func (w *wizard) maintenancePages(tree bool) {
	welcome := w.add(&uiDialog{name: dlgMaintWelcome, attrs: dlgPage, first: ctlNext})
	welcome.sideHeader("Welcome to the [ProductName] Setup Wizard",
		"The Setup Wizard lets you change the way [ProductName] features are installed on your computer or remove it from your computer. "+
			"Click Next to continue or Cancel to exit the Setup Wizard.")
	welcome.navigation()
	welcome.find(ctlBack).attrs = ctlDisabled
	welcome.find(ctlBack).ring = false
	welcome.find(ctlNext).on(evNewDialog, dlgMaintType, condAlways)

	menu := w.add(&uiDialog{name: dlgMaintType, attrs: dlgPage})
	menu.innerHeader("Change, repair, or remove installation", "Select the operation you wish to perform.")

	top := 65

	if tree {
		menu.control(&uiControl{name: "Change", typ: typPushButton, x: 40, y: top, w: 80, h: buttonH, attrs: ctlActive, text: "&Change", ring: true})
		menu.control(text("ChangeText", 130, top+2, 220, 20, "Lets you change the way features are installed."))
		menu.first = "Change"

		top += 40
	} else {
		menu.first = ctlRepair
	}

	menu.control(&uiControl{name: ctlRepair, typ: typPushButton, x: 40, y: top, w: 80, h: buttonH, attrs: ctlActive, text: "Re&pair", ring: true}).
		on(evNewDialog, dlgVerifyRepair, condAlways).
		when("Disable", "ARPNOREPAIR")
	menu.control(text("RepairText", 130, top+2, 220, 20,
		"Repairs errors in the most recent installation by fixing missing and corrupt files, shortcuts and registry entries."))

	top += 40

	menu.control(&uiControl{name: ctlRemove, typ: typPushButton, x: 40, y: top, w: 80, h: buttonH, attrs: ctlActive, text: "&Remove", ring: true}).
		on(evNewDialog, dlgVerifyRemove, condAlways).
		when("Disable", "ARPNOREMOVE")
	menu.control(text("RemoveText", 130, top+2, 220, 20, "Removes [ProductName] from your computer."))

	menu.navigation()
	menu.find(ctlBack).on(evNewDialog, dlgMaintWelcome, condAlways)
	menu.find(ctlNext).attrs = ctlDisabled
	menu.find(ctlNext).ring = false
	menu.def = ""
}

func (w *wizard) endPages() {
	for _, page := range []struct {
		name, title, description, result string
	}{
		{dlgExit, "Completed the [ProductName] Setup Wizard", "Click the Finish button to exit the Setup Wizard.", endReturn},
		{dlgUserExit, "[ProductName] Setup Wizard was interrupted",
			"[ProductName] setup was interrupted. Your system has not been modified. To install this program at a later time, " +
				"run the Setup Wizard again. Click the Finish button to exit the Setup Wizard.", endExit},
		{dlgFatal, "[ProductName] Setup Wizard ended prematurely",
			"[ProductName] setup ended prematurely because of an error. Your system has not been modified. To install this " +
				"program at a later time, run the Setup Wizard again. Click the Finish button to exit the Setup Wizard.", endExit},
	} {
		end := w.add(&uiDialog{name: page.name, attrs: dlgPage, first: ctlFinish, def: ctlFinish, cancel: ctlFinish})
		end.sideHeader(page.title, page.description)
		end.control(strip())
		end.control(disabledButton(ctlBack, backX, "< &Back"))
		end.control(button(ctlFinish, nextX, buttonY, "&Finish")).on(evEndDialog, page.result, condAlways)
		end.control(disabledButton(ctlCancel, cancelX, "Cancel"))
	}

	resume := w.add(&uiDialog{name: dlgResume, attrs: dlgPage, first: ctlInstall, def: "Install", cancel: ctlCancel})
	resume.sideHeader("Resuming the [ProductName] Setup Wizard",
		"The Setup Wizard will complete the installation of [ProductName] on your computer. Click Install to continue or Cancel to exit the Setup Wizard.")
	resume.control(strip())
	resume.control(disabledButton(ctlBack, backX, "< &Back"))
	resume.control(button(ctlInstall, nextX, buttonY, "&Install")).on(evEndDialog, endReturn, condAlways)
	resume.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
}

func (w *wizard) progressPage() {
	progress := w.add(&uiDialog{name: dlgProgressName, attrs: dlgProgress, first: ctlCancel, def: ctlCancel, cancel: ctlCancel})
	progress.innerHeader("Setting up [ProductName]", "Please wait while the Setup Wizard sets up [ProductName]. This may take several minutes.")
	progress.control(text("StatusLabel", margin, 100, 50, 10, "Status:"))
	progress.control(text("ActionText", margin+50, 100, 280, 10, "")).subscribe("ActionText", "Text")
	progress.control(&uiControl{name: "ProgressBar", typ: "ProgressBar", x: margin, y: 115, w: dlgWidth - 2*margin, h: 10, attrs: ctlProgress}).
		subscribe("SetProgress", "Progress")
	progress.control(strip())
	progress.control(disabledButton(ctlBack, backX, "< &Back"))
	progress.control(disabledButton(ctlNext, nextX, "&Next >"))
	progress.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
}

// modalBoxes adds the dialogs that the engine or a control spawns.
func (w *wizard) modalBoxes() {
	cancel := w.add(&uiDialog{name: dlgCancel, attrs: dlgVisible | dlgModal, width: 260, height: 85, first: "No", def: "No", cancel: "No"})
	cancel.control(text("Text", 20, 15, 220, 30, "Are you sure you want to cancel [ProductName] installation?"))
	cancel.control(button("No", 132, 57, "&No")).on(evEndDialog, endReturn, condAlways)
	cancel.control(button("Yes", 72, 57, "&Yes")).on(evEndDialog, endExit, condAlways)

	wait := w.add(&uiDialog{name: dlgWaitCosting, attrs: dlgVisible | dlgModal, width: 260, height: 85, first: endReturn, def: endReturn, cancel: endReturn})
	wait.control(text("Text", 20, 15, 220, 30, "Please wait while the installer finishes determining your disk space requirements."))
	wait.control(button("Return", 102, 57, "&Return")).on(evEndDialog, endExit, condAlways)

	errBox := w.add(&uiDialog{name: dlgErrorName, attrs: dlgErrorBox, width: 270, height: 105, first: "ErrorText", def: "O"})
	// Without an ErrorIcon control the engine logs error 2835 whenever it
	// shows this dialog; the icon must name a real icon in the Binary table
	// (an empty or dangling Text fails with 2888).
	errBox.control(&uiControl{name: "ErrorIcon", typ: typIcon, x: 15, y: 15, w: 24, h: 24, attrs: ctlErrorIcon, text: binError})
	errBox.control(text("ErrorText", 50, 15, 200, 45, ""))

	for _, btn := range []struct {
		name, label, result string
		x                   int
	}{
		{"A", "&Abort", "ErrorAbort", 100},
		{"C", "Cancel", "ErrorCancel", 212},
		{"I", "&Ignore", "ErrorIgnore", 212},
		{"N", "&No", "ErrorNo", 156},
		{"O", "&OK", "ErrorOk", 156},
		{"R", "&Retry", "ErrorRetry", 156},
		{"Y", "&Yes", "ErrorYes", 100},
	} {
		errBox.control(&uiControl{name: btn.name, typ: typPushButton, x: btn.x, y: 72, w: buttonW, h: buttonH, attrs: ctlActive, text: btn.label}).
			on(evEndDialog, btn.result, condAlways)
	}

	files := w.add(&uiDialog{name: dlgFilesInUse, attrs: dlgFilesUse, first: ctlRetry, def: ctlRetry, cancel: endExit})
	files.innerHeader("Files in use", "Some files that need to be updated are currently in use.")
	files.control(text("Text", margin, 55, dlgWidth-2*margin, 30,
		"The following applications are using files that need to be updated by this setup. Close these applications and click Retry to continue."))
	files.control(&uiControl{name: "List", typ: typListBox, property: propFilesUse, x: margin, y: 87, w: dlgWidth - 2*margin, h: 130, attrs: ctlList, ring: true})
	files.control(strip())
	files.control(button("Exit", backX, buttonY, "E&xit")).on(evEndDialog, endExit, condAlways)
	files.control(button("Ignore", nextX, buttonY, "&Ignore")).on(evEndDialog, "Ignore", condAlways)
	files.control(button(ctlRetry, cancelX, buttonY, "&Retry")).on(evEndDialog, "Retry", condAlways)

	restart := w.add(&uiDialog{name: dlgRMFilesInUse, attrs: dlgFilesUse, first: "OK", def: "OK", cancel: ctlCancel})
	restart.innerHeader("Files in use", "Some files that need to be updated are currently in use.")
	restart.control(text("Text", margin, 55, dlgWidth-2*margin, 20, "The following applications are using files that need to be updated by this setup."))
	restart.control(&uiControl{name: "List", typ: typListBox, property: propFilesUse, x: margin, y: 77, w: dlgWidth - 2*margin, h: 90, attrs: ctlList, ring: true})
	restart.control(&uiControl{
		name: "Options", typ: "RadioButtonGroup", property: propRMOption, x: margin, y: 175, w: dlgWidth - 2*margin, h: 45, attrs: ctlActive, ring: true,
	})
	restart.control(strip())
	restart.control(button("OK", nextX, buttonY, "&OK")).
		// The engine matches the event name case-sensitively: the
		// misspelt RmShutdownAndRestart ends the dialog with error 2866.
		on("RMShutdownAndRestart", "0", propRMOption+`="UseRM"`).
		on(evEndDialog, endReturn, condAlways)
	restart.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evEndDialog, endExit, condAlways)

	w.radios = append(w.radios,
		uiRadio{property: propRMOption, value: "UseRM", y: 0, text: "Close the applications and attempt to restart them."},
		uiRadio{property: propRMOption, value: "DontUseRM", y: 22, text: "Do not close applications. A reboot will be required to complete the setup."},
	)
}

// featureDialogs adds the feature tree with Browse and the disk cost page.
func (w *wizard) featureDialogs() {
	custom := w.add(&uiDialog{name: dlgCustomize, attrs: dlgCosting, first: "Tree"})
	custom.innerHeader("Custom Setup", "Select the way you want features to be installed.")
	custom.control(text("Hint", margin+5, 55, dlgWidth-2*margin-10, 20, "Click the icons in the tree below to change the way features will be installed."))
	custom.control(&uiControl{name: "Tree", typ: "SelectionTree", property: propBrowse, x: margin + 5, y: 85, w: 175, h: 95, attrs: ctlList, ring: true})
	custom.control(text("ItemDescription", 215, 85, 135, 50, "")).subscribe("SelectionDescription", "Text")
	custom.control(text("ItemSize", 215, 140, 135, 40, "")).subscribe("SelectionSize", "Text")
	custom.control(text("LocationLabel", margin+5, 192, 50, 10, "Location:")).subscribe("SelectionPathOn", "Visible")
	custom.control(text("Location", margin+55, 192, 215, 24, "")).subscribe("SelectionPath", "Text").subscribe("SelectionPathOn", "Visible")
	custom.control(button("Browse", cancelX, 190, "B&rowse...")).
		on("SelectionBrowse", dlgBrowse, condAlways).
		subscribe("SelectionPathOn", "Visible")
	custom.control(strip())
	custom.control(&uiControl{name: "Reset", typ: typPushButton, x: margin, y: buttonY, w: 70, h: buttonH, attrs: ctlActive, text: "Re&set", ring: true}).
		on("Reset", "0", condAlways)
	custom.control(&uiControl{
		name: "DiskCost", typ: typPushButton, x: margin + 76, y: buttonY, w: 70, h: buttonH, attrs: ctlActive, text: "Disk &Usage", ring: true,
	}).on(evSpawnDialog, dlgDiskCost, condAlways)
	custom.control(button(ctlBack, backX, buttonY, "< &Back"))
	custom.control(button(ctlNext, nextX, buttonY, "&Next >"))
	custom.control(button(ctlCancel, cancelX, buttonY, "Cancel")).on(evSpawnDialog, dlgCancel, condAlways)
	custom.def, custom.cancel = ctlNext, ctlCancel

	w.browseDialog()

	cost := w.add(&uiDialog{name: dlgDiskCost, attrs: dlgCosting, first: "OK", def: "OK", cancel: "OK"})
	cost.innerHeader("Disk space requirements", "The disk space required for the installation of the selected features.")
	cost.control(text("Text", margin, 53, dlgWidth-2*margin, 40,
		"The highlighted volumes do not have enough disk space available for the currently selected features. "+
			"You can remove some files from the highlighted volumes, install fewer features, or select different destination drives."))
	cost.control(&uiControl{
		name: "VolumeList", typ: "VolumeCostList", x: margin, y: 100, w: dlgWidth - 2*margin, h: 120, attrs: ctlVolumes, text: "{120}{70}{70}{70}{70}",
	})
	cost.control(strip())
	cost.control(button("OK", cancelX, buttonY, "&OK")).on(evEndDialog, endReturn, condAlways)
}

// browseDialog adds the folder browser that the feature tree and the
// install folder page open. It edits the directory named by
// _BrowseProperty.
func (w *wizard) browseDialog() {
	if w.byName[dlgBrowse] != nil {
		return
	}

	browse := w.add(&uiDialog{name: dlgBrowse, attrs: dlgPage, first: typDirList, def: "OK", cancel: ctlCancel})
	browse.innerHeader("Change destination folder", "Browse to the destination folder.")
	browse.control(label("LookInLabel", margin+5, 58, 44, 10, "&Look in:"))
	browse.control(&uiControl{name: typDirCombo, typ: typDirCombo, property: propBrowse, x: 70, y: 55, w: 215, h: 80, attrs: ctlDirCombo, ring: true})
	browse.control(&uiControl{name: "Up", typ: typPushButton, x: 292, y: 55, w: 26, h: 19, attrs: ctlActive, text: "&Up", ring: true}).
		on("DirectoryListUp", "0", condAlways)
	browse.control(&uiControl{name: "NewFolder", typ: typPushButton, x: 322, y: 55, w: 26, h: 19, attrs: ctlActive, text: "Ne&w", ring: true}).
		on("DirectoryListNew", "0", condAlways)
	browse.control(&uiControl{name: typDirList, typ: typDirList, property: propBrowse, x: margin + 5, y: 83, w: 323, h: 98, attrs: ctlDirList, ring: true})
	browse.control(label("FolderLabel", margin+5, 190, 100, 10, "&Folder name:"))
	browse.control(&uiControl{name: typPathEdit, typ: typPathEdit, property: propBrowse, x: margin + 5, y: 202, w: 323, h: 18, attrs: ctlPathEdit, ring: true})
	browse.control(strip())
	browse.control(button("OK", nextX, buttonY, "&OK")).
		on("SetTargetPath", "["+propBrowse+"]", condAlways).
		on(evEndDialog, endReturn, condAlways)
	browse.control(button(ctlCancel, cancelX, buttonY, "Cancel")).
		on("Reset", "0", condAlways).
		on(evEndDialog, endReturn, condAlways)
}

// uiTexts are the UIText rows the selection tree, the volume list and the
// progress page read (spec section 7.2).
var uiTexts = [][2]string{ //nolint:gochecknoglobals // lookup table
	// SelectionPath shows AbsentPath for a feature that will not be
	// installed; without the row the control shows an error text.
	{"AbsentPath", ""},
	{unitBytes, unitBytes},
	{"Bytes", unitBytes},
	{"GB", "GB"},
	{"KB", "KB"},
	{"MB", "MB"},
	{"MenuAbsent", "Entire feature will be unavailable"},
	{"MenuAdvertise", "Feature will be installed when required"},
	{"MenuAllCP", "Entire feature will be installed to run from the network"},
	{"MenuAllLocal", "Entire feature will be installed on local hard drive"},
	{"MenuAllNetwork", "Entire feature will be installed to run from the network"},
	{"MenuCD", "Will be installed to run from CD"},
	{"MenuLocal", "Will be installed on local hard drive"},
	{"MenuNetwork", "Will be installed to run from the network"},
	{"NewFolder", "New Folder"},
	{"ScriptInProgress", "Gathering required information..."},
	{"SelAbsentAbsent", "This feature will remain uninstalled."},
	{"SelAbsentAdvertise", "This feature will be set to be installed when required."},
	{"SelAbsentCD", "This feature will be installed to run from CD."},
	{"SelAbsentLocal", "This feature will be installed on the local hard drive."},
	{"SelAbsentNetwork", "This feature will be installed to run from the network."},
	{"SelAdvertiseAbsent", "This feature will become unavailable."},
	{"SelAdvertiseAdvertise", "Will be installed when required."},
	{"SelAdvertiseCD", "This feature will be available to run from CD."},
	{"SelAdvertiseLocal", "This feature will be installed on your local hard drive."},
	{"SelAdvertiseNetwork", "This feature will be available to run from the network."},
	{"SelCDAbsent", "This feature will be uninstalled completely; you will not be able to run it from CD."},
	{"SelCDAdvertise", "This feature will change from run from CD state to set to be installed when required."},
	{"SelCDCD", "This feature will remain on CD."},
	{"SelCDLocal", "This feature will change from run from CD state to be installed on the local hard drive."},
	{"SelChildCostNeg", "This feature frees up [1] on your hard drive."},
	{"SelChildCostPos", "This feature requires [1] on your hard drive."},
	{"SelCostPending", "Compiling cost for this feature..."},
	{"SelLocalAbsent", "This feature will be completely removed."},
	{"SelLocalAdvertise", "This feature will be removed from your local hard drive but will be set to be installed when required."},
	{"SelLocalCD", "This feature will be removed from your local hard drive but will still be available to run from CD."},
	{"SelLocalLocal", "This feature will remain on your local hard drive."},
	{"SelLocalNetwork", "This feature will be removed from your local hard drive but will still be available to run from the network."},
	{"SelNetworkAbsent", "This feature will be uninstalled completely; you will not be able to run it from the network."},
	{"SelNetworkAdvertise", "This feature will change from run from network state to set to be installed when required."},
	{"SelNetworkLocal", "This feature will change from run from network state to be installed on the local hard drive."},
	{"SelNetworkNetwork", "This feature will remain to run from the network."},
	{"SelParentCostNegNeg", "This feature frees up [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures free up [4] on your hard drive."},
	{"SelParentCostNegPos", "This feature frees up [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures require [4] on your hard drive."},
	{"SelParentCostPosNeg", "This feature requires [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures free up [4] on your hard drive."},
	{"SelParentCostPosPos", "This feature requires [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures require [4] on your hard drive."},
	{"TimeRemaining", "Time remaining: {[1] minutes }{[2] seconds}"},
	{"VolumeCostAvailable", "Available"},
	{"VolumeCostDifference", "Difference"},
	{"VolumeCostRequired", "Required"},
	{"VolumeCostSize", "Disk Size"},
	{"VolumeCostVolume", "Volume"},
}
