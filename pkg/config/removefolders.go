package config

import (
	"fmt"
	"strings"

	"github.com/jkroepke/msi-builder/internal/condition"
)

// removeFoldersReserved are the names the folder removal uses itself: the
// property that lists the entries and the prefix of the generated ones.
const (
	removeFoldersProperty = "BuilderRemoveFolders"
	removeFoldersPrefix   = "BuilderTree"
)

// componentRemoveFolders checks the folders a component removes with
// everything in them.
func (v *validator) componentRemoveFolders(cfg *Config, known *refs, compPath string, comp *Component) {
	seen := make(map[[2]string]bool)

	var set map[string]bool

	for j, entry := range comp.RemoveFolders {
		entryPath := fmt.Sprintf("%s.removeFolders[%d]", compPath, j)

		v.oneOf(entryPath+".on", entry.On, RemoveOnInstall, RemoveOnUninstall, RemoveOnBoth)

		if entry.Condition != "" {
			if err := condition.Check(entry.Condition); err != nil {
				v.errorf(entryPath+".condition", "%v", err)
			}
		}

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

		if known.dirs[name] == nil && !set[name] {
			v.warnf(entryPath+".property", "nothing in the package sets %s, so only the command line can name the folder; "+
				"use a directory ID or a property with a value or a search", name)
		}
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
	case name == removeFoldersProperty || strings.HasPrefix(name, removeFoldersPrefix):
		v.errorf(field, "%s is reserved for the folder removal itself", name)
	case IsStandardFolder(name) || name == targetDir:
		v.errorf(field, "%s is a standard folder, which is never removed; name a folder of the product", name)
	default:
		return true
	}

	return false
}
