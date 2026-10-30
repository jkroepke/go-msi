package wixconvert

import (
	"maps"
	"regexp"
	"slices"
	"strings"
)

// walkUI indexes the content of a UI element: a UIRef picks a wizard,
// everything else builds dialogs by hand.
func (c *converter) walkUI(e *elem) { //nolint:cyclop // one case per kind of UI child
	for _, kid := range e.kids {
		switch {
		case kid.is("UIRef"):
			c.uiRef(kid)
		case kid.is("Publish") && kid.attr("Event") == "DoAction" && kid.attr("Dialog") == "ExitDialog":
			c.launchAction = kid.attr("Value")
		case kid.is("Publish") && kid.attr("Event") == "NewDialog":
			c.uiNav = append(c.uiNav, kid)
		case kid.is("Property"):
			c.properties = append(c.properties, kid)
		case kid.is("Error"):
			c.errorRows = append(c.errorRows, kid)
		case kid.is("Dialog"):
			c.todof(nil, "", kid.line, "the hand-built dialog %s cannot be converted; ui.dialogs lays out text, password, checkbox and choice fields",
				kid.attr("Id"))
		default:
			c.todof(nil, "", kid.line, "<%s> in <UI> customizes the WiX dialogs and cannot be converted", kid.qname())
		}
	}
}

func (c *converter) uiRef(e *elem) {
	if id := e.attr("Id"); id != "WixUI_ErrorProgressText" && id != "WixUI_Common" {
		c.uiSet = id
		c.uiLine = e.line
	}
}

func (c *converter) chooseFeatures() bool {
	id := c.uiID()

	return id == uiFeatureTree || id == uiMondo
}

func (c *converter) uiID() string {
	if c.wixUI != nil {
		return c.wixUI.attr("Id")
	}

	return c.uiSet
}

func (c *converter) emitUI() {
	wizard := c.out.child("ui")
	wizard.order("enabled", "license", "chooseFeatures", "chooseDirectory", "launch", "finishText", "theme")

	line := c.uiLine
	if c.wixUI != nil {
		line = c.wixUI.line
	}

	switch wizardID := c.uiID(); wizardID {
	case "":
		c.uiNavigation(nil, "")

		return
	case "WixUI_Minimal":
		wizard.set("enabled", true)
	case "WixUI_InstallDir":
		wizard.set("enabled", true).set("chooseDirectory", true)
	case uiFeatureTree, uiMondo:
		wizard.set("enabled", true).set("chooseFeatures", true)

		if wizardID == uiMondo {
			c.todof(wizard, "chooseFeatures", line, "msi-builder has no setup type page (Typical, Custom, Complete)")
		}
	case "WixUI_Advanced":
		wizard.set("enabled", true)
		c.todof(wizard, "enabled", line, "WixUI_Advanced: set package.scope to perUserOrMachine or perMachineOrUser for the scope page")
	default:
		wizard.set("enabled", true)
		c.todof(wizard, "enabled", line, "the wizard %s is not a WiX standard wizard; check the ui keys", wizardID)
	}

	c.uiVariables(wizard)
	c.uiNavigation(wizard, c.uiID())
	c.uiLaunch(wizard)
}

// navRow is a NewDialog Publish row: the button that opens a dialog.
type navRow struct{ dialog, control, value string }

// licenseSkips lists, for each standard wizard, the two Publish rows that
// take the license page out of the wizard: Next on the welcome page opens
// the page after the license, and Back there returns to the welcome page.
var licenseSkips = map[string][2]navRow{ //nolint:gochecknoglobals // lookup table
	"WixUI_InstallDir": {{dlgWelcome, ctlNext, "InstallDirDlg"}, {"InstallDirDlg", ctlBack, dlgWelcome}},
	uiFeatureTree:      {{dlgWelcome, ctlNext, "CustomizeDlg"}, {"CustomizeDlg", ctlBack, dlgWelcome}},
	uiMondo:            {{dlgWelcome, ctlNext, "SetupTypeDlg"}, {"SetupTypeDlg", ctlBack, dlgWelcome}},
}

const (
	dlgWelcome = "WelcomeDlg"
	ctlNext    = "Next"
	ctlBack    = "Back"
)

// uiNavigation converts the NewDialog Publish rows. Rows that skip the
// license page leave ui.license out, since the wizard never shows it; any
// other row reorders the WiX dialogs and stays a TODO.
func (c *converter) uiNavigation(wizard *omap, wizardID string) {
	skips, known := licenseSkips[wizardID]
	skipping := func(row *elem, want navRow) bool {
		return known && navRow{row.attr("Dialog"), row.attr("Control"), row.attr("Value")} == want && alwaysTrue(c.condition(row))
	}

	// The Back row only belongs to the skip when the Next row is there.
	skipped := slices.ContainsFunc(c.uiNav, func(row *elem) bool { return skipping(row, skips[0]) })

	for _, row := range c.uiNav {
		if skipped && (skipping(row, skips[0]) || skipping(row, skips[1])) {
			continue
		}

		c.todof(wizard, "", row.line, "<Publish> in <UI> customizes the WiX dialogs and cannot be converted")
	}

	if skipped {
		wizard.del("license")
	}
}

// alwaysTrue reports whether a Publish condition holds on a first install,
// the only time the wizard shows the license page.
func alwaysTrue(cond string) bool {
	switch strings.ToUpper(strings.Join(strings.Fields(cond), " ")) {
	case "", "1", "NOT INSTALLED":
		return true
	default:
		return false
	}
}

// uiVariables converts the WixVariables that the standard wizards read.
func (c *converter) uiVariables(wizard *omap) {
	set := map[string]func(string){
		varWixUILicenseRtf: func(v string) { wizard.set("license", v) },
		varWixUIBannerBmp:  func(v string) { wizard.child("theme").set("banner", v) },
		varWixUIDialogBmp:  func(v string) { wizard.child("theme").set("dialog", v) },
	}

	for _, name := range slices.Sorted(maps.Keys(c.wixVars)) {
		if !strings.HasPrefix(name, "WixUI") {
			continue
		}

		if setter, ok := set[name]; ok {
			setter(sourcePath(c.expand(c.pkg, c.wixVars[name])))
		} else {
			c.todof(wizard, "", 0, "the WiX variable %s has no counterpart", name)
		}
	}
}

func (c *converter) uiLaunch(wizard *omap) {
	props := make(map[string]*elem)
	for _, p := range c.properties {
		props[p.attr("Id")] = p
	}

	if p := props["WIXUI_EXITDIALOGOPTIONALTEXT"]; p != nil {
		wizard.set("finishText", c.value(p, "Value"))
	}

	target := props["WixShellExecTarget"]
	if target == nil && props["WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT"] == nil {
		return
	}

	launch := wizard.child("launch")

	if target != nil {
		v := c.value(target, "Value")
		if m := fileTarget.FindStringSubmatch(v); m != nil {
			launch.set("file", m[1])
		} else {
			c.todof(wizard, "launch", target.line, "WixShellExecTarget=%q: launch.file names a file of the package", v)
		}
	}

	if p := props["WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT"]; p != nil {
		launch.set("text", c.value(p, "Value"))
	}

	if p := props["WIXUI_EXITDIALOGOPTIONALCHECKBOX"]; p == nil || c.value(p, "Value") != "1" {
		launch.set("checked", false)
	}
}

// errorArg is a reference to an error argument, [1] to [9].
var errorArg = regexp.MustCompile(`\[[0-9]\]`)

// errorText returns the text of an error action: the message of the Error
// row it names by number, inlined because msi-builder writes no Error rows
// of its own, or the text itself.
func (c *converter) errorText(value string) string {
	for _, row := range c.errorRows {
		if c.value(row, "Id") != strings.TrimSpace(value) {
			continue
		}

		c.usedErrors[row] = true

		message := c.errorMessage(row)
		if message == "" {
			return value
		}

		if errorArg.MatchString(message) {
			c.todof(nil, "", row.line, "the Error row %s refers to error arguments such as [1], which are empty in an error action's text", value)
		}

		return message
	}

	return value
}

func (c *converter) errorMessage(row *elem) string {
	if row.has("Message") {
		return c.value(row, "Message")
	}

	return c.expand(row, strings.TrimSpace(row.text))
}

// emitErrorRows notes the Error rows no error action took in, such as
// replaced Windows Installer messages.
func (c *converter) emitErrorRows() {
	for _, row := range c.errorRows {
		if !c.usedErrors[row] {
			c.todof(nil, "", row.line, "the Error row %s has no counterpart; only error actions take in their message", row.attr("Id"))
		}
	}
}
