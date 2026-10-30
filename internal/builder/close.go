package builder

import (
	"strings"
	"time"

	"github.com/jkroepke/msi-builder/internal/cadll"
)

// closeApps writes the files with a close block into the BuilderCloseApps
// property, one line per file with the condition that its component is
// installed, repaired or removed, and adds the two actions on the embedded
// DLL that stop the running copies (specification section 17.8):
// BuilderCloseSchedule, immediate and before InstallValidate, so that the
// files-in-use check after it no longer finds the programs that closed,
// posts WM_CLOSE to their windows as the installing user and waits; the
// deferred BuilderCloseExec ends what is still running, as LocalSystem in a
// per-machine install, after StopServices has stopped the services
// gracefully and before RemoveFiles and InstallFiles touch the files.
func (b *build) closeApps() error {
	var lines []string

	for _, file := range b.files {
		closing := file.cfg.Close
		if !closing.Enabled() {
			continue
		}

		var flags uint32
		if closing.Terminates() {
			flags |= cadll.CloseTerminate
		}

		comp := file.Component.cfg.ID
		cond := "$" + comp + "=2 OR $" + comp + "=3"
		// Validation caps the timeout at config.MaxCloseTimeout.
		ms := uint32(closing.TimeoutOrDefault() / time.Millisecond) //nolint:gosec // at most 300000
		lines = append(lines, cadll.CloseEntry(cond, ms, flags,
			"["+file.Component.dir.id+"]"+file.LongName))
	}

	if len(lines) == 0 || !b.useRunDLL() {
		return nil
	}

	for _, name := range []string{cadll.CloseProperty, cadll.CloseExec} {
		if _, ok := b.cfg.Properties[name]; ok {
			b.errorf("properties."+name, "the property %s is reserved for closing running programs", name)

			return nil
		}
	}

	b.properties[cadll.CloseProperty] = strings.Join(lines, "\n")

	b.addAction(&actionInfo{name: cadll.CloseSchedule, typ: caDLL, source: cadll.Key, target: cadll.CloseSchedule})
	b.addAction(&actionInfo{name: cadll.CloseExec, typ: caDLL | caDeferred | caNoImpersonate, source: cadll.Key, target: cadll.CloseEntryPoint})

	b.standardAt(seqInstallExecute, actInstallValidate)
	b.standardAt(seqInstallExecute, actRemoveFiles)
	b.schedule(seqInstallExecute, cadll.CloseSchedule, "", actInstallValidate, "", 0)
	b.schedule(seqInstallExecute, cadll.CloseExec, cadll.CloseExec, actRemoveFiles, "", 0)

	return nil
}
