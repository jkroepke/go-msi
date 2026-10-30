package wixconvert

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/internal/folderglob"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// BuiltPackage is the package WiX built from the source file.
type BuiltPackage struct {
	// Name names the package in comments, such as app.msi.
	Name        string
	Components  []BuiltComponent
	Directories []BuiltDirectory
}

// BuiltDirectory is a row of the Directory table of the package WiX built
// from the source file.
type BuiltDirectory struct {
	// ID is the Directory key, Parent the Directory_Parent column.
	ID, Parent string
	// Name is the long target name of DefaultDir: "." or empty for a row
	// that stands for its parent.
	Name string
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

	c.claim(built)

	return true
}

// builtComponent finds a component in the built package: by its Id, or,
// when the source gave it none, by the name and folder of its key file.
func (c *converter) builtComponent(comp *comp, keyFile string) *BuiltComponent {
	if id := comp.e.attr("Id"); id != "" {
		return c.builtByID(id)
	}

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
	files := 0
	if c.built != nil {
		files = c.pinHarvested()
	}

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

	if files > 0 {
		c.header = append(c.header, fmt.Sprintf("The guids of %d files that WiX harvested with Files come from %s.", files, c.built.Name))
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

// noteHarvested notes the folder sources that a WiX Files glob became and
// that keep no code of the WiX package under a late removal: WiX gave every
// harvested file a component and code of its own, so the removal of the
// WiX package deletes the files whose code changed (UPG-11, UPG-12).
func (c *converter) noteHarvested() {
	upgrade := c.out.mapAt("upgrade")
	if upgrade == nil || !lateRemoval[upgrade.str("removeOld")] {
		return
	}

	for _, comp := range c.comps {
		if comp.out == nil || comp.folder == nil {
			continue
		}

		if len(comp.tied) > 0 {
			slices.Sort(comp.tied)
			c.todof(comp.folder, "", comp.e.line,
				"another Files glob into the same folder adds %s too, so convert kept no code for them and the late removeOld "+
					"deletes them on the first upgrade from the WiX package; add the ComponentId WiX gave each one to the guids "+
					"of one glob and exclude it from the other",
				strings.Join(comp.tied, ", "))
		}

		if comp.pinned {
			continue
		}

		if c.built != nil {
			c.todof(comp.folder, "", comp.e.line,
				"%s has no component for a file of this Files glob, so the late removeOld deletes these files on the first "+
					"upgrade from the WiX package; add the ComponentId WiX gave each file to guids (msi-builder inspect "+
					"--table Component), or remove the old version early (afterInstallInitialize)",
				c.built.Name)

			continue
		}

		c.todof(comp.folder, "", comp.e.line,
			"WiX gave each file of this Files glob a component of its own, so the late removeOld deletes these files on the "+
				"first upgrade from the WiX package; run msi-builder convert again with --msi and the package WiX built, "+
				"which keeps the code of each file in guids, or remove the old version early (afterInstallInitialize)")
	}
}

// pinHarvested gives each folder source that a WiX Files glob became the
// codes WiX gave the files it harvested, as guids keyed by the path of the
// file below the folder, and returns how many files it pinned. It runs
// after the components took theirs, so a file that a component lists keeps
// its code there. A file the folder source leaves out keeps none.
func (c *converter) pinHarvested() int {
	found := c.harvestedCodes()
	pinned := 0

	for _, comp := range c.comps {
		guids := found[comp]
		if len(guids) == 0 {
			continue
		}

		codes := newMap()

		for _, rel := range slices.Sorted(maps.Keys(guids)) {
			row := guids[rel]
			codes.set(rel, row.Code)
			c.claim(row)
		}

		comp.folder.set("guids", codes)
		comp.pinned = true
		pinned += len(guids)
	}

	return pinned
}

// harvest is a folder source with its folder in the built package.
type harvest struct {
	comp   *comp
	filter *folderglob.Filter
	target string
}

// harvests returns the folder sources whose folder the built package has.
func (c *converter) harvests() []harvest {
	var found []harvest

	for _, comp := range c.comps {
		if comp.out == nil || comp.folder == nil {
			continue
		}

		target := c.builtDir(c.dirs[comp.dir], comp.dir)

		filter, err := folderglob.New(comp.folder.list("include"), comp.folder.list("exclude"))
		if target == "" || err != nil {
			continue
		}

		found = append(found, harvest{comp: comp, filter: filter, target: target})
	}

	return found
}

// harvestedCodes returns, for each folder source, the components of the
// built package whose key file it adds, by the path below its folder. A
// file two folder sources add goes to the one whose folder is nearest
// above it; when both folders are the same, it goes to neither.
func (c *converter) harvestedCodes() map[*comp]map[string]*BuiltComponent {
	sources := c.harvests()
	found := make(map[*comp]map[string]*BuiltComponent)
	seen := make(map[*comp]map[string]bool)
	known := c.listedCodes()

	for i := range c.built.Components {
		row := &c.built.Components[i]
		if !c.free(row) || row.KeyFile == "" || known.has(row) {
			continue
		}

		src, rel, tied := c.nearestHarvest(sources, row)
		for _, other := range tied {
			other.comp.tied = append(other.comp.tied, rel)
		}

		if src == nil || seen[src.comp][strings.ToLower(rel)] {
			// Windows matches names ignoring case, and so does the build.
			continue
		}

		if found[src.comp] == nil {
			found[src.comp] = make(map[string]*BuiltComponent)
			seen[src.comp] = make(map[string]bool)
		}

		seen[src.comp][strings.ToLower(rel)] = true
		found[src.comp][rel] = row
	}

	return found
}

// nearestHarvest returns the folder source that adds the key file of row
// and whose folder is nearest above it, with the file's path below that
// folder. When none adds it, it returns nil; when several with the same
// folder do, nil and those sources.
func (c *converter) nearestHarvest(sources []harvest, row *BuiltComponent) (*harvest, string, []*harvest) {
	nearest := make([]*harvest, 0, len(sources))
	bestRel, depth := "", -1

	for i := range sources {
		folder, below := c.builtPath(row.Directory, sources[i].target)
		rel := path.Join(folder, row.KeyFile)

		if !below || !sources[i].filter.Included(rel) {
			continue
		}

		switch steps := strings.Count(rel, "/"); {
		case len(nearest) == 0 || steps < depth:
			nearest, bestRel, depth = append(nearest[:0], &sources[i]), rel, steps
		case steps == depth:
			nearest = append(nearest, &sources[i])
		}
	}

	switch len(nearest) {
	case 0:
		return nil, "", nil
	case 1:
		return nearest[0], bestRel, nil
	}

	return nil, bestRel, nearest
}

// listed are the built components that a component of the source stands
// for without having claimed them: its guid is their code, which WiX took
// from the source, or its key file is theirs, in the same folder, which
// pinCode could not tell apart from another one.
type listed struct {
	codes    map[string]bool
	keyFiles map[string]bool
}

// has reports whether a component of the source stands for row.
func (l listed) has(row *BuiltComponent) bool {
	guid, err := config.NormalizeGUID(row.Code)

	return err == nil && l.codes[guid] || l.keyFiles[strings.ToLower(row.Directory+"/"+row.KeyFile)]
}

// listedCodes collects the codes and key files of the source's components
// that are no folder source.
func (c *converter) listedCodes() listed {
	known := listed{codes: make(map[string]bool), keyFiles: make(map[string]bool)}

	for _, comp := range c.comps {
		if comp.out == nil || comp.folder != nil {
			continue
		}

		if guid, err := config.NormalizeGUID(comp.out.str("guid")); err == nil {
			known.codes[guid] = true
		}

		if dirID := c.builtDir(c.dirs[comp.dir], comp.dir); comp.keyFile != "" && dirID != "" {
			known.keyFiles[strings.ToLower(dirID+"/"+comp.keyFile)] = true
		}
	}

	return known
}

// claim marks a built component as taken, so that no other component gets
// its code.
func (c *converter) claim(row *BuiltComponent) {
	if c.claimed == nil {
		c.claimed = make(map[string]bool)
	}

	c.claimed[row.ID] = true
}

// builtDir returns the ID that the folder, with the source ID dirID, has
// in the built package, or "". A folder WiX gave an ID of its own is found
// by its name below its parent.
func (c *converter) builtDir(folder *dir, dirID string) string {
	if folder != nil && folder.generated {
		parent := ""
		if folder.parent != nil {
			parent = c.builtDir(folder.parent, folder.parent.id)
		}

		return c.builtChild(parent, folder.name)
	}

	if c.builtDirs()[dirID] == nil {
		return ""
	}

	return dirID
}

// builtChild returns the ID of the only folder of the built package named
// name below parent, or "".
func (c *converter) builtChild(parent, name string) string {
	if parent == "" || name == "" {
		return ""
	}

	found := ""

	for _, row := range c.built.Directories {
		if row.Parent != parent || !strings.EqualFold(row.Name, name) {
			continue
		}

		if found != "" {
			return ""
		}

		found = row.ID
	}

	return found
}

// builtPath returns the slash path of the folder dirID below the folder
// target in the built package, and whether dirID is target or below it.
func (c *converter) builtPath(dirID, target string) (string, bool) {
	dirs := c.builtDirs()

	var names []string

	// A parent loop would be a broken package; stop after every row.
	for range len(dirs) + 1 {
		if dirID == target {
			slices.Reverse(names)

			return path.Join(names...), true
		}

		row := dirs[dirID]
		if row == nil || row.Parent == dirID {
			return "", false
		}

		if row.Name != "" && row.Name != "." {
			names = append(names, row.Name)
		}

		dirID = row.Parent
	}

	return "", false
}

// builtDirs indexes the folders of the built package by ID.
func (c *converter) builtDirs() map[string]*BuiltDirectory {
	if c.builtDirIndex == nil {
		c.builtDirIndex = make(map[string]*BuiltDirectory, len(c.built.Directories))
		for i := range c.built.Directories {
			c.builtDirIndex[c.built.Directories[i].ID] = &c.built.Directories[i]
		}
	}

	return c.builtDirIndex
}

// openCodes returns the components whose Guid WiX generated and that have
// no code of the WiX package, and counts the ones that have one.
func (c *converter) openCodes() ([]*comp, int) {
	var open []*comp

	pinned := 0

	for _, comp := range c.comps {
		switch {
		case comp.folder != nil:
			// pinHarvested and noteHarvested cover it.
		case comp.out != nil && comp.pinned:
			pinned++
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
