package builder

import (
	"strings"

	"github.com/jkroepke/msi-builder/internal/cadll"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// removeFolders lists the removeFolders entries in the BuilderRemoveFolders
// property for the immediate action of the same name, which checks each
// folder and hands the accepted ones to the deferred, rollback and commit
// actions after it (specification section 17.10): the deferred one moves
// each folder by handle into a private backup folder, the rollback one
// moves it back, the commit one deletes it without following junctions.
func (b *build) removeFolders() error {
	var lines []string

	// The properties are not made secure: an administrator's values reach
	// the server anyway (PROP-01), and a secure folder property would let
	// a standard user's command line steer every other use of it in an
	// elevated session, where the DLL removes nothing.
	for _, comp := range b.cfg.Components {
		for _, entry := range comp.RemoveFolders {
			_, packageDir := b.dirs[entry.Property]
			lines = append(lines, cadll.RemoveFoldersEntry(removeFolderCondition(comp.ID, entry, packageDir), entry.Property))
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

	// The data names the backup folder; keep it out of the log. Its
	// safety does not rest on that: the DLL checks the folder's owner and
	// access list before it uses it.
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
// or when it is installed locally ($comp=3), ANDed with the entry's own. A
// folder of the package (packageDir) goes only with the whole product
// (REMOVE="ALL"): removing one feature must not take away the files the
// components of the others keep there.
func removeFolderCondition(component string, entry *config.RemoveFolder, packageDir bool) string {
	removed := "($" + component + "=2 AND NOT UPGRADINGPRODUCTCODE)"
	if packageDir {
		removed = "($" + component + `=2 AND REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE)`
	}

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
