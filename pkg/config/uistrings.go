package config

import (
	"maps"
	"slices"
	"unicode/utf8"
)

// unitBytes is the UIText key and text of the byte unit.
const unitBytes = "bytes"

// uiStrings are the built-in wizard texts in English, by the key that
// ui.strings uses to replace them. All but the UIText rows (uiTextKeys) are
// formatted text, so [ProductName] is resolved at install time; on buttons,
// check boxes, radio buttons and the browse dialog's labels an "&" marks the
// access key.
var uiStrings = map[string]string{ //nolint:gochecknoglobals // lookup table
	// Shared by every page.
	"DialogTitle": "[ProductName] Setup",
	"Back":        "< &Back",
	"Next":        "&Next >",
	"Cancel":      "Cancel",
	"Install":     "&Install",
	"Repair":      "&Repair",
	"Remove":      "&Remove",
	"Finish":      "&Finish",
	"OK":          "&OK",
	"Yes":         "&Yes",
	"No":          "&No",
	"Abort":       "&Abort",
	"Retry":       "&Retry",
	"Ignore":      "&Ignore",
	"Exit":        "E&xit",
	"Return":      "&Return",

	"WelcomeTitle": "Welcome to the [ProductName] Setup Wizard",
	"WelcomeText":  "The Setup Wizard will install [ProductName] on your computer. Click Next to continue or Cancel to exit the Setup Wizard.",

	"LicenseTitle":  "End-User License Agreement",
	"LicenseText":   "Please read the following license agreement carefully.",
	"LicenseAccept": "I &accept the terms in the License Agreement",
	"LicensePrint":  "&Print",

	"InstallDirTitle":  "Destination Folder",
	"InstallDirText":   "Click Next to install to the default folder or click Change to choose another.",
	"InstallDirLabel":  "Install [ProductName] to:",
	"InstallDirChange": "&Change...",

	"InstallScopeTitle":   "Installation Scope",
	"InstallScopeText":    "Choose who can use [ProductName] on this computer.",
	"InstallScopeUser":    "Install just for &you ([LogonUser])",
	"InstallScopeMachine": "Install for &all users of this computer",
	"InstallScopeHint":    "Installing for all users needs administrator rights. Installing just for you does not, and only you can use [ProductName].",

	"CustomizeTitle":     "Custom Setup",
	"CustomizeText":      "Select the way you want features to be installed.",
	"CustomizeHint":      "Click the icons in the tree below to change the way features will be installed.",
	"CustomizeLocation":  "Location:",
	"CustomizeBrowse":    "B&rowse...",
	"CustomizeReset":     "Re&set",
	"CustomizeDiskUsage": "Disk &Usage",

	"DiskCostTitle": "Disk space requirements",
	"DiskCostText":  "The disk space required for the installation of the selected features.",
	"DiskCostHint": "The highlighted volumes do not have enough disk space available for the currently selected features. " +
		"You can remove some files from the highlighted volumes, install fewer features, or select different destination drives.",

	"BrowseTitle":      "Change destination folder",
	"BrowseText":       "Browse to the destination folder.",
	"BrowseLookIn":     "&Look in:",
	"BrowseUp":         "&Up",
	"BrowseNewFolder":  "Ne&w",
	"BrowseFolderName": "&Folder name:",

	"ReadyInstallTitle": "Ready to install [ProductName]",
	"ReadyInstallText":  "Click Install to begin the installation. Click Back to review or change any of your settings. Click Cancel to exit the wizard.",
	"ReadyRepairTitle":  "Ready to repair [ProductName]",
	"ReadyRepairText":   "Click Repair to repair the installation of [ProductName]. Click Back to return to the maintenance menu or Cancel to exit the wizard.",
	"ReadyRemoveTitle":  "Remove [ProductName]",
	"ReadyRemoveText":   "Click Remove to remove [ProductName] from your computer. Click Back to return to the maintenance menu or Cancel to exit the wizard.",

	"MaintenanceWelcomeTitle": "Welcome to the [ProductName] Setup Wizard",
	"MaintenanceWelcomeText": "The Setup Wizard lets you change the way [ProductName] features are installed on your computer or remove it from your computer. " +
		"Click Next to continue or Cancel to exit the Setup Wizard.",
	"MaintenanceTypeTitle":  "Change, repair, or remove installation",
	"MaintenanceTypeText":   "Select the operation you wish to perform.",
	"MaintenanceChange":     "&Change",
	"MaintenanceChangeText": "Lets you change the way features are installed.",
	"MaintenanceRepair":     "Re&pair",
	"MaintenanceRepairText": "Repairs errors in the most recent installation by fixing missing and corrupt files, shortcuts and registry entries.",
	"MaintenanceRemove":     "&Remove",
	"MaintenanceRemoveText": "Removes [ProductName] from your computer.",

	"ProgressTitle":  "Setting up [ProductName]",
	"ProgressText":   "Please wait while the Setup Wizard sets up [ProductName]. This may take several minutes.",
	"ProgressStatus": "Status:",

	"ExitTitle":     "Completed the [ProductName] Setup Wizard",
	"ExitText":      "Click the Finish button to exit the Setup Wizard.",
	"UserExitTitle": "[ProductName] Setup Wizard was interrupted",
	"UserExitText": "[ProductName] setup was interrupted. Your system has not been modified. To install this program at a later time, " +
		"run the Setup Wizard again. Click the Finish button to exit the Setup Wizard.",
	"FatalErrorTitle": "[ProductName] Setup Wizard ended prematurely",
	"FatalErrorText": "[ProductName] setup ended prematurely because of an error. Your system has not been modified. To install this " +
		"program at a later time, run the Setup Wizard again. Click the Finish button to exit the Setup Wizard.",
	"ResumeTitle": "Resuming the [ProductName] Setup Wizard",
	"ResumeText": "The Setup Wizard will complete the installation of [ProductName] on your computer. " +
		"Click Install to continue or Cancel to exit the Setup Wizard.",
	"Launch": "&Launch [ProductName]",

	"CancelText":      "Are you sure you want to cancel [ProductName] installation?",
	"WaitCostingText": "Please wait while the installer finishes determining your disk space requirements.",

	"FilesInUseTitle":  "Files in use",
	"FilesInUseText":   "Some files that need to be updated are currently in use.",
	"FilesInUseList":   "The following applications are using files that need to be updated by this setup. Close these applications and click Retry to continue.",
	"RMFilesInUseList": "The following applications are using files that need to be updated by this setup.",
	"RMUseRM":          "Close the applications and attempt to restart them.",
	"RMDontUseRM":      "Do not close applications. A reboot will be required to complete the setup.",

	// UIText rows: the selection tree, the volume list and the progress page.
	"AbsentPath":            "",
	unitBytes:               unitBytes,
	"Bytes":                 unitBytes,
	"GB":                    "GB",
	"KB":                    "KB",
	"MB":                    "MB",
	"MenuAbsent":            "Entire feature will be unavailable",
	"MenuAdvertise":         "Feature will be installed when required",
	"MenuAllCP":             "Entire feature will be installed to run from CD",
	"MenuAllLocal":          "Entire feature will be installed on local hard drive",
	"MenuAllNetwork":        "Entire feature will be installed to run from the network",
	"MenuCD":                "Will be installed to run from CD",
	"MenuLocal":             "Will be installed on local hard drive",
	"MenuNetwork":           "Will be installed to run from the network",
	"NewFolder":             "New Folder",
	"ScriptInProgress":      "Gathering required information...",
	"SelAbsentAbsent":       "This feature will remain uninstalled.",
	"SelAbsentAdvertise":    "This feature will be set to be installed when required.",
	"SelAbsentCD":           "This feature will be installed to run from CD.",
	"SelAbsentLocal":        "This feature will be installed on the local hard drive.",
	"SelAbsentNetwork":      "This feature will be installed to run from the network.",
	"SelAdvertiseAbsent":    "This feature will become unavailable.",
	"SelAdvertiseAdvertise": "Will be installed when required.",
	"SelAdvertiseCD":        "This feature will be available to run from CD.",
	"SelAdvertiseLocal":     "This feature will be installed on your local hard drive.",
	"SelAdvertiseNetwork":   "This feature will be available to run from the network.",
	"SelCDAbsent":           "This feature will be uninstalled completely; you will not be able to run it from CD.",
	"SelCDAdvertise":        "This feature will change from run from CD state to set to be installed when required.",
	"SelCDCD":               "This feature will remain on CD.",
	"SelCDLocal":            "This feature will change from run from CD state to be installed on the local hard drive.",
	"SelChildCostNeg":       "This feature frees up [1] on your hard drive.",
	"SelChildCostPos":       "This feature requires [1] on your hard drive.",
	"SelCostPending":        "Compiling cost for this feature...",
	"SelLocalAbsent":        "This feature will be completely removed.",
	"SelLocalAdvertise":     "This feature will be removed from your local hard drive but will be set to be installed when required.",
	"SelLocalCD":            "This feature will be removed from your local hard drive but will still be available to run from CD.",
	"SelLocalLocal":         "This feature will remain on your local hard drive.",
	"SelLocalNetwork":       "This feature will be removed from your local hard drive but will still be available to run from the network.",
	"SelNetworkAbsent":      "This feature will be uninstalled completely; you will not be able to run it from the network.",
	"SelNetworkAdvertise":   "This feature will change from run from network state to set to be installed when required.",
	"SelNetworkLocal":       "This feature will change from run from network state to be installed on the local hard drive.",
	"SelNetworkNetwork":     "This feature will remain to run from the network.",
	"SelParentCostNegNeg": "This feature frees up [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures free up [4] on your hard drive.",
	"SelParentCostNegPos": "This feature frees up [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures require [4] on your hard drive.",
	"SelParentCostPosNeg": "This feature requires [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures free up [4] on your hard drive.",
	"SelParentCostPosPos": "This feature requires [1] on your hard drive. It has [2] of [3] subfeatures selected. " +
		"The subfeatures require [4] on your hard drive.",
	"TimeRemaining":        "Time remaining: {[1] minutes }{[2] seconds}",
	"VolumeCostAvailable":  "Available",
	"VolumeCostDifference": "Difference",
	"VolumeCostRequired":   "Required",
	"VolumeCostSize":       "Disk Size",
	"VolumeCostVolume":     "Volume",
}

// uiTextKeys are the ui.strings keys of the UIText rows that the selection
// tree, the volume list and the progress page read (spec section 7.2), in
// the order the builder writes them. UIText is plain text, not formatted.
// SelectionPath shows AbsentPath for a feature that will not be installed;
// without the row the control shows an error text.
var uiTextKeys = []string{ //nolint:gochecknoglobals // lookup table
	"AbsentPath", unitBytes, "Bytes", "GB", "KB", "MB",
	"MenuAbsent", "MenuAdvertise", "MenuAllCP", "MenuAllLocal", "MenuAllNetwork", "MenuCD", "MenuLocal", "MenuNetwork",
	"NewFolder", "ScriptInProgress",
	"SelAbsentAbsent", "SelAbsentAdvertise", "SelAbsentCD", "SelAbsentLocal", "SelAbsentNetwork",
	"SelAdvertiseAbsent", "SelAdvertiseAdvertise", "SelAdvertiseCD", "SelAdvertiseLocal", "SelAdvertiseNetwork",
	"SelCDAbsent", "SelCDAdvertise", "SelCDCD", "SelCDLocal",
	"SelChildCostNeg", "SelChildCostPos", "SelCostPending",
	"SelLocalAbsent", "SelLocalAdvertise", "SelLocalCD", "SelLocalLocal", "SelLocalNetwork",
	"SelNetworkAbsent", "SelNetworkAdvertise", "SelNetworkLocal", "SelNetworkNetwork",
	"SelParentCostNegNeg", "SelParentCostNegPos", "SelParentCostPosNeg", "SelParentCostPosPos",
	"TimeRemaining",
	"VolumeCostAvailable", "VolumeCostDifference", "VolumeCostRequired", "VolumeCostSize", "VolumeCostVolume",
}

// uiStringWidths are the column widths of the texts that are not stored in
// an unbounded Control.Text: Dialog.Title, RadioButton.Text and UIText.Text.
var uiStringWidths = map[string]int{ //nolint:gochecknoglobals // lookup table
	"DialogTitle": 128, "RMUseRM": 64, "RMDontUseRM": 64, "InstallScopeUser": 64, "InstallScopeMachine": 64,
}

// uiTextWidth is the width of UIText.Text.
const uiTextWidth = 255

// UITextKeys returns the ui.strings keys that are UIText rows, in the order
// the builder writes them.
func UITextKeys() []string {
	return slices.Clone(uiTextKeys)
}

// UIStrings returns the built-in wizard texts in English by their ui.strings
// key.
func UIStrings() map[string]string {
	return maps.Clone(uiStrings)
}

// UIString returns the wizard text for key: the ui.strings value when the
// configuration sets one, else the built-in English text. It panics on a key
// that is not a built-in text, which is a programming error.
func (u *UI) UIString(key string) string {
	text, known := uiStrings[key]
	if !known {
		panic("config: unknown wizard text " + key)
	}

	if override, set := u.Strings[key]; set {
		return override
	}

	return text
}

// uiStrings checks ui.strings.
func (v *validator) uiStrings(cfg *Config) {
	if len(cfg.UI.Strings) > 0 && !cfg.UI.Enabled {
		v.errorf("ui.strings", "wizard texts need a wizard")
	}

	v.checkUIStrings("ui.strings", cfg.UI.Strings)
}

// checkUIStrings checks wizard texts at path: known keys, and no empty text
// where the built-in one is not empty, since a blank button or title is a
// mistake.
func (v *validator) checkUIStrings(path string, texts map[string]string) {
	for _, key := range slices.Sorted(maps.Keys(texts)) {
		builtIn, known := uiStrings[key]

		switch {
		case !known:
			v.errorf(path+"."+key, "%s is not a wizard text", key)
		case texts[key] == "" && builtIn != "":
			v.errorf(path+"."+key, "the text is empty; leave the key out to keep the built-in text")
		default:
			if width := uiStringWidth(key); width > 0 {
				if n := utf8.RuneCountInString(texts[key]); n > width {
					v.warnf(path+"."+key, "%d characters exceed the column width of %d (ICE03)", n, width)
				}
			}
		}
	}
}

// uiStringWidth returns the column width of a text, or 0 when its column is
// unbounded.
func uiStringWidth(key string) int {
	if slices.Contains(uiTextKeys, key) {
		return uiTextWidth
	}

	return uiStringWidths[key]
}
