package builder

import (
	"strings"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/internal/condition"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// removeFolderAnchor prefixes the private property that the package's own
// RemoveFile rows of a removeFolders entry name; the DLL sets it, and the
// properties of the subfolders get it with _<n> appended.
const removeFolderAnchor = "BuilderTree"

// removeFolders lists the removeFolders entries in the BuilderRemoveFolders
// property for the immediate action of the same name, which adds temporary
// RemoveFile rows for every subfolder of the folder an entry's property
// holds (specification section 17.8). Each entry also gets a `*` row of
// its own on an anchor property that only the action sets, so it removes
// nothing when the action skips the entry; it removes the files of the
// folder itself, and keeps a RemoveFile table in the package for the
// action to add rows to.
func (b *build) removeFolders() error {
	var lines []string

	for _, comp := range b.cfg.Components {
		for _, entry := range comp.RemoveFolders {
			mode := removeModes[entry.On]
			anchor := generatedID(removeFolderAnchor, comp.ID, entry.Property, entry.On)

			b.table("RemoveFile").Add(anchor, comp.ID, "*", anchor, mode)
			lines = append(lines, cadll.RemoveFoldersEntry(removeFolderCondition(comp.ID, entry), comp.ID, mode, anchor, entry.Property))

			// The action runs on the server, where only secure public
			// properties from the client arrive.
			for _, prop := range append(condition.Properties(entry.Condition), entry.Property) {
				if config.IsPublicProperty(prop) {
					b.secure[prop] = true
				}
			}
		}
	}

	if len(lines) == 0 || !b.useRunDLL() {
		return nil
	}

	if _, ok := b.cfg.Properties[cadll.RemoveFolders]; ok {
		b.errorf("properties."+cadll.RemoveFolders, "the property %s is reserved for removeFolders", cadll.RemoveFolders)

		return nil
	}

	b.properties[cadll.RemoveFolders] = strings.Join(lines, "\n")

	// After CostFinalize the directories are resolved and the component
	// states known; the rows must be there before RemoveFiles.
	b.addAction(&actionInfo{name: cadll.RemoveFolders, typ: caDLL, source: cadll.Key, target: cadll.RemoveFolders})
	b.schedule(seqInstallExecute, cadll.RemoveFolders, "", "", actCostFinalize, 0)

	return nil
}

// removeFolderCondition is the condition under which an entry removes its
// folder: the component is installed (install), or removed but not by the
// major upgrade of another product (uninstall), and the entry's condition.
func removeFolderCondition(component string, entry *config.RemoveFolder) string {
	installed := "$" + component + "=3"
	removed := "$" + component + "=2 AND NOT UPGRADINGPRODUCTCODE"

	var cond string

	switch entry.On {
	case config.RemoveOnInstall:
		cond = installed
	case config.RemoveOnBoth:
		cond = "(" + installed + " OR (" + removed + "))"
	default:
		cond = removed
	}

	if entry.Condition != "" {
		cond += " AND (" + entry.Condition + ")"
	}

	return cond
}
