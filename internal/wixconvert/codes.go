package wixconvert

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
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

	comp.out.set("guid", built.Code)
	comp.pinned = true

	if c.claimed == nil {
		c.claimed = make(map[string]bool)
	}

	c.claimed[built.ID] = true

	return true
}

// builtComponent finds a component in the built package: by its Id, or,
// when the source gave it none, by the Id of its key path or the name and
// folder of its key file.
func (c *converter) builtComponent(comp *comp, keyFile string) *BuiltComponent {
	if id := comp.e.attr("Id"); id != "" {
		return c.builtByID(id)
	}

	// WiX gives a component without an Id the Id of its key path, which
	// convert takes too.
	if comp.id != "" {
		if row := c.builtByID(comp.id); row != nil && c.compByID[row.ID] == comp && !c.claimed[row.ID] {
			return row
		}
	}

	return c.builtByKeyFile(comp, keyFile)
}

// builtByKeyFile returns the one free component of the built package with
// the key file in the component's folder, or nil.
func (c *converter) builtByKeyFile(comp *comp, keyFile string) *BuiltComponent {
	if keyFile == "" {
		return nil
	}

	// A folder without an Id got a generated one in the package.
	folder := c.dirs[comp.dir]
	anyDir := folder != nil && folder.generated

	var found *BuiltComponent

	for i := range c.built.Components {
		row := &c.built.Components[i]
		if !c.free(row) || !strings.EqualFold(row.KeyFile, keyFile) || !anyDir && row.Directory != comp.dir {
			continue
		}

		if found != nil {
			return nil
		}

		found = row
	}

	return found
}

// free reports whether a built component may match by key file: it has a
// code, and neither a source Id names it nor another component took it, so
// that two components never share a code.
func (c *converter) free(row *BuiltComponent) bool {
	_, named := c.compByID[row.ID]

	return row.Code != "" && !named && !c.claimed[row.ID]
}

// builtByID returns the component of the built package with an Id and a
// code, or nil.
func (c *converter) builtByID(id string) *BuiltComponent {
	for i := range c.built.Components {
		if c.built.Components[i].ID == id && c.built.Components[i].Code != "" {
			return &c.built.Components[i]
		}
	}

	return nil
}

// checkCodes notes the components that keep no code of the WiX package:
// with a built package every one it lacks, without one all of them at once
// when the upgrade removes the old version late.
func (c *converter) checkCodes() {
	open, pinned := c.openCodes()

	c.noteDroppedValues()
	c.noteAdvertised()
	c.noteHarvested()

	if c.built == nil {
		c.noteLateRemoval(open)

		return
	}

	if pinned > 0 {
		c.header = append(c.header, fmt.Sprintf("The guid of %d components whose Guid WiX generated comes from %s.", pinned, c.built.Name))
	}

	for _, comp := range open {
		c.todof(comp.out, "", comp.e.line,
			"%s has no matching component; set guid to the ComponentId WiX gave this one (msi-builder inspect --table Component)",
			c.built.Name)
	}
}

// noteDroppedValues notes the HKCU key path values that a shortcut
// component keeping its WiX code no longer writes: with a late removal the
// old product's removal keeps them, since the code still has a client, and
// nothing removes them later (seen in a real-world port, like INST-04 for
// HKCU values the new version did not list).
func (c *converter) noteDroppedValues() {
	upgrade := c.out.mapAt("upgrade")
	if upgrade == nil || !lateRemoval[upgrade.str("removeOld")] {
		return
	}

	for _, comp := range c.comps {
		if comp.out == nil || !hasCode(comp.out) || len(comp.droppedValues) == 0 {
			continue
		}

		c.todof(comp.out, "", comp.e.line,
			"msi-builder writes its own HKCU key path for this shortcut component; with the late removeOld the WiX value "+
				"HKCU\\%s stays behind after the upgrade and the uninstall, so remove it with removeRegistry in another component",
			strings.Join(comp.droppedValues, ", HKCU\\"))
	}
}

// noteAdvertised notes the advertised shortcuts of the WiX package under a
// late removal. A real-world port lost all of them with every component
// code pinned: the old product's removal deleted them after this package
// had installed its plain shortcuts at the same paths, presumably because
// an advertised shortcut goes with its feature, not with its component.
func (c *converter) noteAdvertised() {
	upgrade := c.out.mapAt("upgrade")
	if len(c.advertised) == 0 || upgrade == nil || !lateRemoval[upgrade.str("removeOld")] {
		return
	}

	slices.Sort(c.advertised)

	numbers := make([]string, 0, len(c.advertised))
	for _, line := range c.advertised {
		numbers = append(numbers, strconv.Itoa(line))
	}

	lines := "line " + numbers[0]
	if len(numbers) > 1 {
		lines = "lines " + strings.Join(numbers, ", ")
	}

	c.todof(upgrade, "removeOld", 0,
		"the WiX package's advertised shortcuts (%s) are deleted by this late removal after this package installed its "+
			"own at the same paths, also with every guid pinned; remove the old version early (afterInstallInitialize), at "+
			"least for the first upgrade from WiX, or give the shortcuts other names",
		lines)
}

// noteHarvested notes the folder sources that a WiX Files glob became under
// a late removal: WiX gave every harvested file a component and code of its
// own, a folder source takes no guid, so the removal of the WiX package
// deletes those files (UPG-11).
func (c *converter) noteHarvested() {
	upgrade := c.out.mapAt("upgrade")
	if upgrade == nil || !lateRemoval[upgrade.str("removeOld")] {
		return
	}

	for _, comp := range c.comps {
		if comp.out == nil || !comp.harvests {
			continue
		}

		c.todof(comp.out, "", comp.e.line,
			"WiX gave each file of this Files glob a component of its own, and a folder source takes no guid, so the late "+
				"removeOld deletes these files on the first upgrade from the WiX package; remove the old version early "+
				"(afterInstallInitialize), or list the files with the guid WiX gave each one")
	}
}

// openCodes returns the components whose Guid WiX generated and that have
// no code of the WiX package, and counts the ones that have one.
func (c *converter) openCodes() ([]*comp, int) {
	var open []*comp

	pinned := 0

	for _, comp := range c.comps {
		switch {
		case comp.out != nil && comp.pinned:
			pinned++
		case comp.harvests:
			// noteHarvested covers it: a folder source takes no guid.
		case comp.generated && comp.out != nil && comp.out.get("guid") == nil:
			// A dropped component installs nothing, so it loses nothing.
			open = append(open, comp)
		}
	}

	return open, pinned
}

// noteLateRemoval notes the components without a WiX Guid when the upgrade
// removes the old version after installing the new one.
func (c *converter) noteLateRemoval(open []*comp) {
	upgrade := c.out.mapAt("upgrade")
	if len(open) == 0 || upgrade == nil || !lateRemoval[upgrade.str("removeOld")] {
		return
	}

	line := c.removeLine
	if c.major != nil {
		line = c.major.line
	}

	const listed = 5

	ids := make([]string, 0, listed)
	for _, comp := range open[:min(len(open), listed)] {
		ids = append(ids, cmp.Or(comp.e.attr("Id"), comp.id, "the component in line "+strconv.Itoa(comp.e.line)))
	}

	list := strings.Join(ids, ", ")

	switch more := len(open) - listed; {
	case more == 1:
		list += " and one more component"
	case more > 1:
		list += fmt.Sprintf(" and %d more components", more)
	}

	c.todof(upgrade, "removeOld", line,
		"WiX generated the Guid of %s; msi-builder generates other codes, and removing the old version after installing "+
			"this one deletes their files on the first upgrade from the WiX package. Run msi-builder convert again with "+
			"--msi and the package WiX built, or set each guid from msi-builder inspect --table Component",
		list)
}

// hasCode reports whether a converted component has a pinned code, not
// none: an unmanaged component is never removed, so it keeps nothing.
func hasCode(component *omap) bool {
	guid := component.str("guid")

	return guid != "" && guid != config.None
}
