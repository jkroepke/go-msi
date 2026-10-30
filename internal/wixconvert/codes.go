package wixconvert

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// BuiltPackage is the package WiX built from the source file.
type BuiltPackage struct {
	// Name names the package in comments, such as app.msi.
	Name       string
	Components []BuiltComponent
}

// BuiltComponent is a row of the Component table of the package WiX built
// from the source file.
type BuiltComponent struct {
	// ID is the Component key.
	ID string
	// Code is the ComponentId.
	Code string
	// Directory is the Directory_ column.
	Directory string
	// KeyFile is the long name of the key path file, empty when the key
	// path is a registry value or the folder.
	KeyFile string
}

// lateRemoval are the upgrade.removeOld values that install the new version
// before the old one is removed. The removal then deletes the resources of
// every component code the new version does not share (UPG-11).
var lateRemoval = map[string]bool{"afterInstallExecute": true, "afterInstallFinalize": true} //nolint:gochecknoglobals // lookup table

// pinCode gives a component whose Guid WiX generated the code it has in the
// built package, and reports whether the package has a matching component.
func (c *converter) pinCode(comp *comp, keyFile string) bool {
	built := c.builtComponent(comp, keyFile)
	if built == nil {
		return false
	}

	comp.out.set("guid", cmp.Or(built.Code, "none"))

	c.pinned++

	return true
}

// builtComponent finds a component in the built package: by its Id, or,
// when the source gave it none, by the name and folder of its key file.
func (c *converter) builtComponent(comp *comp, keyFile string) *BuiltComponent {
	if id := comp.e.attr("Id"); id != "" {
		for i := range c.built.Components {
			if c.built.Components[i].ID == id {
				return &c.built.Components[i]
			}
		}

		return nil
	}

	if keyFile == "" {
		return nil
	}

	var found *BuiltComponent

	for i := range c.built.Components {
		row := &c.built.Components[i]
		if !strings.EqualFold(row.KeyFile, keyFile) {
			continue
		}

		// A folder without an Id got a generated one in the package.
		if !strings.HasPrefix(comp.dir, anonymousDir) && row.Directory != comp.dir {
			continue
		}

		if found != nil {
			return nil
		}

		found = row
	}

	return found
}

// checkCodes notes the components that keep no code of the WiX package:
// with a built package every one it lacks, without one all of them at once
// when the upgrade removes the old version late.
func (c *converter) checkCodes() {
	var open []*comp

	for _, comp := range c.comps {
		if comp.out != nil && comp.generated && comp.out.get("guid") == nil {
			open = append(open, comp)
		}
	}

	if c.built == nil {
		c.noteLateRemoval(open)

		return
	}

	if c.pinned > 0 {
		c.header = append(c.header, fmt.Sprintf("The guid of %d components whose Guid WiX generated comes from %s.", c.pinned, c.built.Name))
	}

	for _, comp := range open {
		c.todof(comp.out, "", comp.e.line,
			"%s has no matching component; set guid to the ComponentId WiX gave this one (msi-builder inspect --table Component)",
			c.built.Name)
	}
}

// noteLateRemoval notes the components without a WiX Guid when the upgrade
// removes the old version after installing the new one.
func (c *converter) noteLateRemoval(open []*comp) {
	upgrade := c.out.mapAt("upgrade")
	if len(open) == 0 || upgrade == nil || !lateRemoval[upgrade.str("removeOld")] {
		return
	}

	line := 0
	if c.major != nil {
		line = c.major.line
	}

	const listed = 5

	ids := make([]string, 0, listed+1)
	for _, comp := range open[:min(len(open), listed)] {
		ids = append(ids, cmp.Or(comp.out.str("id"), "the component in line "+strconv.Itoa(comp.e.line)))
	}

	if len(open) > listed {
		ids = append(ids, fmt.Sprintf("%d more components", len(open)-listed))
	}

	c.todof(upgrade, "removeOld", line,
		"WiX generated the Guid of %s; msi-builder generates other codes, and removing the old version after installing "+
			"this one deletes their files on the first upgrade from the WiX package. Run msi-builder convert again with "+
			"--msi and the package WiX built, or set each guid from msi-builder inspect --table Component",
		strings.Join(ids, ", "))
}
