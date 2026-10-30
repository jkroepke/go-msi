package wixconvert

import (
	"maps"
	"slices"
	"strings"
)

// walkUI indexes the content of a UI element: a UIRef picks a wizard,
// everything else builds dialogs by hand.
func (c *converter) walkUI(e *elem) {
	for _, kid := range e.kids {
		switch {
		case kid.is("UIRef"):
			c.uiRef(kid)
		case kid.is("Publish") && kid.attr("Event") == "DoAction" && kid.attr("Dialog") == "ExitDialog":
			c.launchAction = kid.attr("Value")
		case kid.is("Property"):
			c.properties = append(c.properties, kid)
		case kid.is("ProgressText"):
			c.progressTexts = append(c.progressTexts, kid)
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
	c.uiLaunch(wizard)
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
