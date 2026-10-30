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
	case name == removeFoldersProperty || strings.HasPrefix(name, removeFoldersPrefix):
		v.errorf(field, "%s is reserved for the folder removal itself", name)
	case IsStandardFolder(name) || name == targetDir:
		v.errorf(field, "%s is a standard folder, which is never removed; name a folder of the product", name)
	default:
		return true
	}

	return false
}

// removeFolderSource checks that something sets the property before
// costing, where the removal reads it: a directory gets its path only at
// CostFinalize, after that, except the install dir that installDir.remember
// searches for.
func (v *validator) removeFolderSource(cfg *Config, known *refs, field, name string, set map[string]bool) {
	switch {
	case name == cfg.InstallDir.ID && cfg.InstallDir.Remember:
	case name == cfg.InstallDir.ID:
		v.errorf(field, "%s gets its path at CostFinalize, after the removal reads it; set installDir.remember: true, "+
			"whose search sets %s before", name, name)
	case known.dirs[name] != nil && !hasSearch(cfg, name):
		v.errorf(field, "the directory %s gets its path at CostFinalize, after the removal reads it; remember the path in "+
			"the registry and name a property with a search", name)
	case !set[name]:
		v.warnf(field, "nothing in the package sets %s, so only the command line can name the folder; "+
			"use a property with a search that finds the folder again at uninstall", name)
	}
}

// hasSearch reports whether a properties entry fills name with a search.
func hasSearch(cfg *Config, name string) bool {
	prop := cfg.Properties[name]

	return prop != nil && len(prop.Search) > 0
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
