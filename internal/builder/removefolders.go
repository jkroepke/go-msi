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
			lines = append(lines, cadll.RemoveFoldersEntry(removeFolderCondition(entry), comp.ID, mode, anchor, entry.Property))

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

	// Before CostInitialize, after the searches: costing collects the
	// folders that RemoveFile.DirProperty names, and a property it has not
	// seen there fails InstallValidate with error 2727 (FILE-11).
	b.addAction(&actionInfo{name: cadll.RemoveFolders, typ: caDLL, source: cadll.Key, target: cadll.RemoveFolders})
	b.schedule(seqInstallExecute, cadll.RemoveFolders, "", actCostInitialize, "", 0)

	return nil
}

// removeFolderCondition is the condition under which the action lists an
// entry's folder. Component states are not known before costing, so the
// rows carry the component and the engine applies them only when it
// installs (mode 1) or removes (mode 2) the component; the condition only
// skips the listing where no row could apply: an uninstall entry when
// nothing is removed or when the major upgrade of another product removes
// this one, which must keep the folder.
func removeFolderCondition(entry *config.RemoveFolder) string {
	var cond string

	switch entry.On {
	case config.RemoveOnInstall:
	case config.RemoveOnBoth:
		cond = "NOT UPGRADINGPRODUCTCODE"
	default:
		cond = "REMOVE AND NOT UPGRADINGPRODUCTCODE"
	}

	switch {
	case entry.Condition == "":
	case cond == "":
		cond = entry.Condition
	default:
		cond += " AND (" + entry.Condition + ")"
	}

	return cond
}
