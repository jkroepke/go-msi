package config

import "github.com/jkroepke/go-msi/internal/condition"

// targetDir is the root of the directory tree.
const targetDir = "TARGETDIR"

// purge checks that a purged directory is a folder of the product: the
// action deletes whatever the directory property points to at uninstall.
func (v *validator) purge(cfg *Config, known *refs, field string, comp *Component) {
	if !comp.Purge.Enabled() {
		return
	}

	v.purgeFolder(cfg, known, field, comp)

	if comp.Purge.Condition != "" {
		if err := condition.Check(comp.Purge.Condition); err != nil {
			v.errorf(field+".purge.condition", "%v", err)
		}
	}
}

// purgeFolder rejects a purge of a folder that may not belong to the
// product: a standard folder, a folder that is its parent, or one below a
// folder a property can move.
func (v *validator) purgeFolder(cfg *Config, known *refs, field string, comp *Component) {
	dir := known.dirs[comp.Directory]

	switch {
	case dir == nil:
		// Reported for the directory already.
	case IsStandardFolder(comp.Directory) || known.dirParent[comp.Directory] == "":
		v.errorf(field+".purge", "%s is a standard folder; purge a folder below it", comp.Directory)
	case known.topFolder(comp.Directory) == targetDir:
		// TARGETDIR is public and falls back to ROOTDRIVE, the local drive
		// with the most free space (specification section 8.3).
		v.errorf(field+".purge", "%s is below TARGETDIR, whose drive can change; purge a folder below a fixed standard "+
			"folder, such as CommonAppDataFolder/Vendor/Product", comp.Directory)
	case dir.Name == "" || dir.Name == ".":
		v.errorf(field+".purge", "%s is its parent folder; purge a folder with its own name", comp.Directory)
	default:
		if movable := movableAncestor(cfg, known, comp.Directory); movable != "" {
			v.errorf(field+".purge", "%s can be pointed at any folder (a public id, configurable, or set by an action, "+
				"a property or a dialog field), so the purge could delete a folder the product does not own; purge a "+
				"folder with private ids below a fixed standard folder, such as CommonAppDataFolder/Vendor/Product", movable)
		}
	}
}

// setProperties lists the properties that a properties entry, a
// setDirectory or setProperty action or a dialog field sets.
func setProperties(cfg *Config) map[string]bool {
	set := make(map[string]bool, len(cfg.Properties))

	for name := range cfg.Properties {
		set[name] = true
	}

	for _, ca := range cfg.CustomActions {
		switch {
		case ca.SetDirectory != nil:
			set[ca.SetDirectory.Directory] = true
		case ca.SetProperty != nil:
			set[ca.SetProperty.Name] = true
		}
	}

	for _, dialog := range cfg.UI.Dialogs {
		for _, field := range dialog.Fields {
			set[field.Property] = true
		}
	}

	return set
}

// movableAncestor returns the first directory from dirID up to (not
// including) its root folder whose path something besides the Directory
// table can set (specification section 8.3): a public id, which the command
// line can set; a configurable one; or one named by a setDirectory or
// setProperty action, a properties entry or a dialog field.
func movableAncestor(cfg *Config, known *refs, dirID string) string {
	set := setProperties(cfg)

	for range 64 {
		parent, ok := known.dirParent[dirID]
		if !ok || parent == "" {
			return ""
		}

		if dir := known.dirs[dirID]; dir != nil && dir.Configurable || set[dirID] || IsPublicProperty(dirID) {
			return dirID
		}

		dirID = parent
	}

	return ""
}
