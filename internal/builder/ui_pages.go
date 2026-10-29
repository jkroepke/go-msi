package builder

// The pages of the wizard. Texts are msi-builder's own and come from
// config.UIStrings, which ui.strings can replace; the dialog keys follow
// the names the specification uses for the standard flow (spec section
// 10.3) so that ui.dialogs can anchor on them.

const (
	ctlInstall = "Install"
	ctlRepair  = "Repair"
	ctlRemove  = "Remove"
	ctlFinish  = "Finish"
	ctlRetry   = "Retry"
)

// commonDialogs adds the pages every wizard has.
func (w *wizard) commonDialogs(tree, hasComponents bool) {
	welcome := w.add(&uiDialog{name: dlgWelcome, attrs: dlgPage, first: ctlNext})
	welcome.sideHeader(w.text("WelcomeTitle"), w.text("WelcomeText"))
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
	ready.innerHeader(w.text("ReadyInstallTitle"), w.text("ReadyInstallText"))
	ready.control(strip())
	ready.control(button(ctlBack, backX, buttonY, w.text("Back")))
	ready.control(button(ctlInstall, nextX, buttonY, w.text("Install"))).on(evEndDialog, endReturn, condAlways)
	ready.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
	ready.def, ready.cancel = ctlInstall, ctlCancel

	repair := w.add(&uiDialog{name: dlgVerifyRepair, attrs: dlgPage, first: ctlRepair})
	repair.innerHeader(w.text("ReadyRepairTitle"), w.text("ReadyRepairText"))
	repair.control(strip())
	repair.control(button(ctlBack, backX, buttonY, w.text("Back"))).on(evNewDialog, dlgMaintType, condAlways)
	repair.control(button(ctlRepair, nextX, buttonY, w.text("Repair"))).
		on("ReinstallMode", "ecmus", condAlways).
		on("Reinstall", "ALL", condAlways).
		on(evEndDialog, endReturn, condAlways)
	repair.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
	repair.def, repair.cancel = ctlRepair, ctlCancel

	remove := w.add(&uiDialog{name: dlgVerifyRemove, attrs: dlgPage, first: ctlRemove})
	remove.innerHeader(w.text("ReadyRemoveTitle"), w.text("ReadyRemoveText"))
	remove.control(strip())
	remove.control(button(ctlBack, backX, buttonY, w.text("Back"))).on(evNewDialog, dlgMaintType, condAlways)
	remove.control(button(ctlRemove, nextX, buttonY, w.text("Remove"))).
		on("Remove", "ALL", condAlways).
		on(evEndDialog, endReturn, condAlways)
	remove.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
	remove.def, remove.cancel = ctlRemove, ctlCancel
}

func (w *wizard) maintenancePages(tree bool) {
	welcome := w.add(&uiDialog{name: dlgMaintWelcome, attrs: dlgPage, first: ctlNext})
	welcome.sideHeader(w.text("MaintenanceWelcomeTitle"), w.text("MaintenanceWelcomeText"))
	welcome.navigation()
	welcome.find(ctlBack).attrs = ctlDisabled
	welcome.find(ctlBack).ring = false
	welcome.find(ctlNext).on(evNewDialog, dlgMaintType, condAlways)

	menu := w.add(&uiDialog{name: dlgMaintType, attrs: dlgPage})
	menu.innerHeader(w.text("MaintenanceTypeTitle"), w.text("MaintenanceTypeText"))

	top := 65

	if tree {
		menu.control(&uiControl{
			name: "Change", typ: typPushButton, x: 40, y: top, w: 80, h: buttonH, attrs: ctlActive, text: w.text("MaintenanceChange"), ring: true,
		})
		menu.control(text("ChangeText", 130, top+2, 220, 20, w.text("MaintenanceChangeText")))
		menu.first = "Change"

		top += 40
	} else {
		menu.first = ctlRepair
	}

	menu.control(&uiControl{
		name: ctlRepair, typ: typPushButton, x: 40, y: top, w: 80, h: buttonH, attrs: ctlActive, text: w.text("MaintenanceRepair"), ring: true,
	}).
		on(evNewDialog, dlgVerifyRepair, condAlways).
		when("Disable", "ARPNOREPAIR")
	menu.control(text("RepairText", 130, top+2, 220, 20,
		w.text("MaintenanceRepairText")))

	top += 40

	menu.control(&uiControl{
		name: ctlRemove, typ: typPushButton, x: 40, y: top, w: 80, h: buttonH, attrs: ctlActive, text: w.text("MaintenanceRemove"), ring: true,
	}).
		on(evNewDialog, dlgVerifyRemove, condAlways).
		when("Disable", "ARPNOREMOVE")
	menu.control(text("RemoveText", 130, top+2, 220, 20, w.text("MaintenanceRemoveText")))

	menu.navigation()
	menu.find(ctlBack).on(evNewDialog, dlgMaintWelcome, condAlways)
	menu.find(ctlNext).attrs = ctlDisabled
	menu.find(ctlNext).ring = false
	menu.def = ""
}

func (w *wizard) endPages() {
	for _, page := range []struct {
		name, text, result string
	}{
		{dlgExit, "Exit", endReturn},
		{dlgUserExit, "UserExit", endExit},
		{dlgFatal, "FatalError", endExit},
	} {
		end := w.add(&uiDialog{name: page.name, attrs: dlgPage, first: ctlFinish, def: ctlFinish, cancel: ctlFinish})
		end.sideHeader(w.text(page.text+"Title"), w.text(page.text+"Text"))
		end.control(strip())
		end.control(disabledButton(ctlBack, backX, w.text("Back")))
		end.control(button(ctlFinish, nextX, buttonY, w.text("Finish"))).on(evEndDialog, page.result, condAlways)
		end.control(disabledButton(ctlCancel, cancelX, w.text("Cancel")))
	}

	resume := w.add(&uiDialog{name: dlgResume, attrs: dlgPage, first: ctlInstall, def: "Install", cancel: ctlCancel})
	resume.sideHeader(w.text("ResumeTitle"), w.text("ResumeText"))
	resume.control(strip())
	resume.control(disabledButton(ctlBack, backX, w.text("Back")))
	resume.control(button(ctlInstall, nextX, buttonY, w.text("Install"))).on(evEndDialog, endReturn, condAlways)
	resume.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
}

func (w *wizard) progressPage() {
	progress := w.add(&uiDialog{name: dlgProgressName, attrs: dlgProgress, first: ctlCancel, def: ctlCancel, cancel: ctlCancel})
	progress.innerHeader(w.text("ProgressTitle"), w.text("ProgressText"))
	progress.control(text("StatusLabel", margin, 100, 50, 10, w.text("ProgressStatus")))
	progress.control(text("ActionText", margin+50, 100, 280, 10, "")).subscribe("ActionText", "Text")
	progress.control(&uiControl{name: "ProgressBar", typ: "ProgressBar", x: margin, y: 115, w: dlgWidth - 2*margin, h: 10, attrs: ctlProgress}).
		subscribe("SetProgress", "Progress")
	progress.control(strip())
	progress.control(disabledButton(ctlBack, backX, w.text("Back")))
	progress.control(disabledButton(ctlNext, nextX, w.text("Next")))
	progress.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
}

// modalBoxes adds the dialogs that the engine or a control spawns.
func (w *wizard) modalBoxes() {
	cancel := w.add(&uiDialog{name: dlgCancel, attrs: dlgVisible | dlgModal, width: 260, height: 85, first: "No", def: "No", cancel: "No"})
	cancel.control(text("Text", 20, 15, 220, 30, w.text("CancelText")))
	cancel.control(button("No", 132, 57, w.text("No"))).on(evEndDialog, endReturn, condAlways)
	cancel.control(button("Yes", 72, 57, w.text("Yes"))).on(evEndDialog, endExit, condAlways)

	wait := w.add(&uiDialog{name: dlgWaitCosting, attrs: dlgVisible | dlgModal, width: 260, height: 85, first: endReturn, def: endReturn, cancel: endReturn})
	wait.control(text("Text", 20, 15, 220, 30, w.text("WaitCostingText")))
	wait.control(button("Return", 102, 57, w.text("Return"))).on(evEndDialog, endExit, condAlways)

	errBox := w.add(&uiDialog{name: dlgErrorName, attrs: dlgErrorBox, width: 270, height: 105, first: "ErrorText", def: "O"})
	// Without an ErrorIcon control the engine logs error 2835 whenever it
	// shows this dialog; the icon must name a real icon in the Binary table
	// (an empty or dangling Text fails with 2888).
	errBox.control(&uiControl{name: "ErrorIcon", typ: typIcon, x: 15, y: 15, w: 24, h: 24, attrs: ctlErrorIcon, text: binError})
	errBox.control(text("ErrorText", 50, 15, 200, 45, ""))

	for _, btn := range []struct {
		name, key, result string
		x                 int
	}{
		{"A", "Abort", "ErrorAbort", 100},
		{"C", "Cancel", "ErrorCancel", 212},
		{"I", "Ignore", "ErrorIgnore", 212},
		{"N", "No", "ErrorNo", 156},
		{"O", "OK", "ErrorOk", 156},
		{"R", "Retry", "ErrorRetry", 156},
		{"Y", "Yes", "ErrorYes", 100},
	} {
		errBox.control(&uiControl{name: btn.name, typ: typPushButton, x: btn.x, y: 72, w: buttonW, h: buttonH, attrs: ctlActive, text: w.text(btn.key)}).
			on(evEndDialog, btn.result, condAlways)
	}

	files := w.add(&uiDialog{name: dlgFilesInUse, attrs: dlgFilesUse, first: ctlRetry, def: ctlRetry, cancel: endExit})
	files.innerHeader(w.text("FilesInUseTitle"), w.text("FilesInUseText"))
	files.control(text("Text", margin, 55, dlgWidth-2*margin, 30, w.text("FilesInUseList")))
	files.control(&uiControl{name: "List", typ: typListBox, property: propFilesUse, x: margin, y: 87, w: dlgWidth - 2*margin, h: 130, attrs: ctlList, ring: true})
	files.control(strip())
	files.control(button("Exit", backX, buttonY, w.text("Exit"))).on(evEndDialog, endExit, condAlways)
	files.control(button("Ignore", nextX, buttonY, w.text("Ignore"))).on(evEndDialog, "Ignore", condAlways)
	files.control(button(ctlRetry, cancelX, buttonY, w.text("Retry"))).on(evEndDialog, "Retry", condAlways)

	restart := w.add(&uiDialog{name: dlgRMFilesInUse, attrs: dlgFilesUse, first: "OK", def: "OK", cancel: ctlCancel})
	restart.innerHeader(w.text("FilesInUseTitle"), w.text("FilesInUseText"))
	restart.control(text("Text", margin, 55, dlgWidth-2*margin, 20, w.text("RMFilesInUseList")))
	restart.control(&uiControl{name: "List", typ: typListBox, property: propFilesUse, x: margin, y: 77, w: dlgWidth - 2*margin, h: 90, attrs: ctlList, ring: true})
	restart.control(&uiControl{
		name: "Options", typ: typRadioGroup, property: propRMOption, x: margin, y: 175, w: dlgWidth - 2*margin, h: 45, attrs: ctlActive, ring: true,
	})
	restart.control(strip())
	restart.control(button("OK", nextX, buttonY, w.text("OK"))).
		// The engine matches the event name case-sensitively: the
		// misspelt RmShutdownAndRestart ends the dialog with error 2866.
		on("RMShutdownAndRestart", "0", propRMOption+`="UseRM"`).
		on(evEndDialog, endReturn, condAlways)
	restart.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evEndDialog, endExit, condAlways)

	w.radios = append(w.radios,
		uiRadio{property: propRMOption, value: "UseRM", y: 0, text: w.text("RMUseRM")},
		uiRadio{property: propRMOption, value: "DontUseRM", y: 22, text: w.text("RMDontUseRM")},
	)
}

// featureDialogs adds the feature tree with Browse and the disk cost page.
func (w *wizard) featureDialogs() {
	custom := w.add(&uiDialog{name: dlgCustomize, attrs: dlgCosting, first: "Tree"})
	custom.innerHeader(w.text("CustomizeTitle"), w.text("CustomizeText"))
	custom.control(text("Hint", margin+5, 55, dlgWidth-2*margin-10, 20, w.text("CustomizeHint")))
	custom.control(&uiControl{name: "Tree", typ: "SelectionTree", property: propBrowse, x: margin + 5, y: 85, w: 175, h: 95, attrs: ctlList, ring: true})
	custom.control(text("ItemDescription", 215, 85, 135, 50, "")).subscribe("SelectionDescription", "Text")
	custom.control(text("ItemSize", 215, 140, 135, 40, "")).subscribe("SelectionSize", "Text")
	custom.control(text("LocationLabel", margin+5, 192, 50, 10, w.text("CustomizeLocation"))).subscribe("SelectionPathOn", "Visible")
	custom.control(text("Location", margin+55, 192, 215, 24, "")).subscribe("SelectionPath", "Text").subscribe("SelectionPathOn", "Visible")
	custom.control(button("Browse", cancelX, 190, w.text("CustomizeBrowse"))).
		on("SelectionBrowse", dlgBrowse, condAlways).
		subscribe("SelectionPathOn", "Visible")
	custom.control(strip())
	custom.control(&uiControl{
		name: "Reset", typ: typPushButton, x: margin, y: buttonY, w: 70, h: buttonH, attrs: ctlActive, text: w.text("CustomizeReset"), ring: true,
	}).
		on("Reset", "0", condAlways)
	custom.control(&uiControl{
		name: "DiskCost", typ: typPushButton, x: margin + 76, y: buttonY, w: 70, h: buttonH, attrs: ctlActive, text: w.text("CustomizeDiskUsage"), ring: true,
	}).on(evSpawnDialog, dlgDiskCost, condAlways)
	custom.control(button(ctlBack, backX, buttonY, w.text("Back")))
	custom.control(button(ctlNext, nextX, buttonY, w.text("Next")))
	custom.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).on(evSpawnDialog, dlgCancel, condAlways)
	custom.def, custom.cancel = ctlNext, ctlCancel

	w.browseDialog()

	cost := w.add(&uiDialog{name: dlgDiskCost, attrs: dlgCosting, first: "OK", def: "OK", cancel: "OK"})
	cost.innerHeader(w.text("DiskCostTitle"), w.text("DiskCostText"))
	cost.control(text("Text", margin, 53, dlgWidth-2*margin, 40, w.text("DiskCostHint")))
	cost.control(&uiControl{
		name: "VolumeList", typ: "VolumeCostList", x: margin, y: 100, w: dlgWidth - 2*margin, h: 120, attrs: ctlVolumes, text: "{120}{70}{70}{70}{70}",
	})
	cost.control(strip())
	cost.control(button("OK", cancelX, buttonY, w.text("OK"))).on(evEndDialog, endReturn, condAlways)
}

// browseDialog adds the folder browser that the feature tree and the
// install folder page open. It edits the directory named by
// _BrowseProperty.
func (w *wizard) browseDialog() {
	if w.byName[dlgBrowse] != nil {
		return
	}

	browse := w.add(&uiDialog{name: dlgBrowse, attrs: dlgPage, first: typDirList, def: "OK", cancel: ctlCancel})
	browse.innerHeader(w.text("BrowseTitle"), w.text("BrowseText"))
	browse.control(label("LookInLabel", margin+5, 58, 44, 10, w.text("BrowseLookIn")))
	browse.control(&uiControl{name: typDirCombo, typ: typDirCombo, property: propBrowse, x: 70, y: 55, w: 215, h: 80, attrs: ctlDirCombo, ring: true})
	browse.control(&uiControl{name: "Up", typ: typPushButton, x: 292, y: 55, w: 26, h: 19, attrs: ctlActive, text: w.text("BrowseUp"), ring: true}).
		on("DirectoryListUp", "0", condAlways)
	browse.control(&uiControl{name: "NewFolder", typ: typPushButton, x: 322, y: 55, w: 26, h: 19, attrs: ctlActive, text: w.text("BrowseNewFolder"), ring: true}).
		on("DirectoryListNew", "0", condAlways)
	browse.control(&uiControl{name: typDirList, typ: typDirList, property: propBrowse, x: margin + 5, y: 83, w: 323, h: 98, attrs: ctlDirList, ring: true})
	browse.control(label("FolderLabel", margin+5, 190, 100, 10, w.text("BrowseFolderName")))
	browse.control(&uiControl{name: typPathEdit, typ: typPathEdit, property: propBrowse, x: margin + 5, y: 202, w: 323, h: 18, attrs: ctlPathEdit, ring: true})
	browse.control(strip())
	browse.control(button("OK", nextX, buttonY, w.text("OK"))).
		on("SetTargetPath", "["+propBrowse+"]", condAlways).
		on(evEndDialog, endReturn, condAlways)
	browse.control(button(ctlCancel, cancelX, buttonY, w.text("Cancel"))).
		on("Reset", "0", condAlways).
		on(evEndDialog, endReturn, condAlways)
}
