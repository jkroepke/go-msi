package builder

import (
	"strings"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/internal/condition"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// removeFolders lists the removeFolders entries in the BuilderRemoveFolders
// property for the immediate action of the same name, which checks each
// folder and hands the accepted ones to the deferred, rollback and commit
// actions after it (specification section 17.8): the deferred one moves
// each folder by handle into a private backup folder, the rollback one
// moves it back, the commit one deletes it without following junctions.
func (b *build) removeFolders() error {
	var lines []string

	for _, comp := range b.cfg.Components {
		for _, entry := range comp.RemoveFolders {
			lines = append(lines, cadll.RemoveFoldersEntry(removeFolderCondition(comp.ID, entry), entry.Property))

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

	for _, name := range []string{cadll.RemoveFolders, cadll.RemoveFoldersUndo, cadll.RemoveFoldersExec, cadll.RemoveFoldersCommit} {
		if _, ok := b.cfg.Properties[name]; ok {
			b.errorf("properties."+name, "the property %s is reserved for removeFolders", name)

			return nil
		}
	}

	b.properties[cadll.RemoveFolders] = strings.Join(lines, "\n")

	b.removeFoldersActions()

	return nil
}

// removeFoldersActions schedules the immediate action right after
// RemoveFiles, when the component states and the directories' paths are
// known and the engine's own files are gone, and the rollback, deferred and
// commit actions after it; each of those runs when the immediate one gave
// it data.
func (b *build) removeFoldersActions() {
	actions := []*actionInfo{
		{name: cadll.RemoveFolders, typ: caDLL, source: cadll.Key, target: cadll.RemoveFolders},
		{name: cadll.RemoveFoldersUndo, typ: caDLL | caRollback | caNoImpersonate | caHideTarget, source: cadll.Key, target: cadll.RemoveFoldersExec},
		{name: cadll.RemoveFoldersExec, typ: caDLL | caDeferred | caNoImpersonate | caHideTarget, source: cadll.Key, target: cadll.RemoveFoldersExec},
		{name: cadll.RemoveFoldersCommit, typ: caDLL | caCommit | caNoImpersonate | caHideTarget, source: cadll.Key, target: cadll.RemoveFoldersExec},
	}

	// The data names the private backup folder; keep it out of the log,
	// where the account that started a repair could read it.
	for _, name := range []string{cadll.RemoveFoldersUndo, cadll.RemoveFoldersExec, cadll.RemoveFoldersCommit} {
		b.hidden[name] = true
	}

	b.standardAt(seqInstallExecute, actRemoveFiles)

	after := actRemoveFiles

	for _, action := range actions {
		b.addAction(action)

		cond := action.name
		if action.name == cadll.RemoveFolders {
			cond = ""
		}

		b.schedule(seqInstallExecute, action.name, cond, "", after, 0)
		after = action.name
	}
}

// removeFolderCondition is the condition under which the action removes an
// entry's folder: when the component is removed ($comp=2) unless the major
// upgrade of another product removes this one, which must keep the folder,
// or when it is installed locally ($comp=3), ANDed with the entry's own.
func removeFolderCondition(component string, entry *config.RemoveFolder) string {
	removed := "($" + component + "=2 AND NOT UPGRADINGPRODUCTCODE)"
	installed := "$" + component + "=3"

	var cond string

	switch entry.On {
	case config.RemoveOnInstall:
		cond = installed
	case config.RemoveOnBoth:
		cond = "(" + removed + " OR " + installed + ")"
	default:
		cond = removed
	}

	if entry.Condition != "" {
		cond += " AND (" + entry.Condition + ")"
	}

	return cond
}
