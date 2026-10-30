package config

import (
	"fmt"
	"strings"

	"github.com/jkroepke/msi-builder/internal/condition"
)

// removeFoldersPrefix starts the names the folder removal uses itself: the
// property that lists the entries and those of its deferred actions.
const removeFoldersPrefix = "BuilderRemoveFolders"

// componentRemoveFolders checks the folders a component removes with
// everything in them.
func (v *validator) componentRemoveFolders(cfg *Config, known *refs, compPath string, comp *Component) {
	seen := make(map[[2]string]bool)

	var set map[string]bool

	for j, entry := range comp.RemoveFolders {
		entryPath := fmt.Sprintf("%s.removeFolders[%d]", compPath, j)

		v.oneOf(entryPath+".on", entry.On, RemoveOnInstall, RemoveOnUninstall, RemoveOnBoth)

		v.removeFolderCondition(entryPath+".condition", entry.Condition)

		name := entry.Property
		if !v.removeFolderProperty(entryPath+".property", name) {
			continue
		}

		key := [2]string{name, entry.On}
		if seen[key] {
			v.errorf(entryPath, "this component already removes [%s] %s", name, entry.On)
		}

		seen[key] = true

		if set == nil {
			set = setProperties(cfg)
		}

		v.removeFolderSource(cfg, known, entryPath+".property", name, set)
	}
}

// removeFolderProperty checks the property of a removeFolders entry and
// reports whether it names one the removal may read.
func (v *validator) removeFolderProperty(field, name string) bool {
	switch {
	case name == "":
		v.errorf(field, "required: the directory ID or property that holds the folder")
	case !IsIdentifier(name):
		v.errorf(field, "%q is not a property name", name)
	case strings.HasPrefix(name, removeFoldersPrefix):
		v.errorf(field, "%s is reserved for the folder removal itself", name)
	case IsStandardFolder(name) || name == targetDir:
		v.errorf(field, "%s is a standard folder, which is never removed; name a folder of the product", name)
	default:
		return true
	}

	return false
}

// removeFolderSource checks where the folder comes from at removal, when
// the action reads it after costing: a directory has its path, but an
// install dir chosen at install time is only known again when
// installDir.remember searches for it; any other property needs something
// that sets it.
func (v *validator) removeFolderSource(cfg *Config, known *refs, field, name string, set map[string]bool) {
	switch {
	case name == cfg.InstallDir.ID && !cfg.InstallDir.Remember:
		v.warnf(field, "at uninstall %s holds its default path, not one chosen at install time; set installDir.remember: true "+
			"to remove the folder the product was installed to", name)
	case name == cfg.InstallDir.ID || known.dirs[name] != nil:
	case !set[name]:
		v.warnf(field, "nothing in the package sets %s, so only the command line can name the folder; "+
			"use a property with a search that finds the folder again at uninstall", name)
	}
}

// removeFolderCondition checks the condition of a removeFolders entry, which
// goes into one line of the BuilderRemoveFolders property.
func (v *validator) removeFolderCondition(field, cond string) {
	switch {
	case cond == "":
	case strings.ContainsFunc(cond, func(r rune) bool { return r < 0x20 }):
		v.errorf(field, "the condition cannot contain line breaks or control characters")
	default:
		if err := condition.Check(cond); err != nil {
			v.errorf(field, "%v", err)
		}
	}
}
