package builder

import (
	"strings"
	"time"

	"github.com/jkroepke/msi-builder/internal/cadll"
)

// closeApps writes the files with a close block into the BuilderCloseApps
// property, one line per file with the condition that its component is
// installed, repaired or removed, and adds the actions on the embedded DLL
// that stop the running copies (specification section 17.8):
// BuilderCloseSchedule, immediate and before InstallValidate, so that the
// files-in-use check after it no longer finds the programs that closed,
// posts WM_CLOSE to their windows as the installing user and waits; the
// deferred BuilderCloseExec ends what is still running, as LocalSystem in a
// per-machine install, after StopServices has stopped the services
// gracefully and before RemoveFiles and InstallFiles touch the files. The
// paths it ends come from one type-51 action per file, run by the engine
// under the same condition, never from the impersonated schedule action.
func (b *build) closeApps() error {
	type setter struct{ name, cond, path string }

	var (
		lines   []string
		setters []setter
	)

	for _, file := range b.files {
		closing := file.cfg.Close
		if !closing.Enabled() {
			continue
		}

		comp := file.Component.cfg.ID
		cond := "$" + comp + "=2 OR $" + comp + "=3"
		path := "[" + file.Component.dir.id + "]" + escapeFormatted(file.LongName)
		// Validation caps the timeout at config.MaxCloseTimeout.
		ms := uint32(closing.TimeoutOrDefault() / time.Millisecond) //nolint:gosec // at most 300000
		lines = append(lines, cadll.CloseEntry(cond, ms, path))

		if closing.Terminates() {
			setters = append(setters, setter{generatedID("BuilderCloseSet", file.Key), cond, path})
		}
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
	b.standardAt(seqInstallExecute, actInstallValidate)
	b.schedule(seqInstallExecute, cadll.CloseSchedule, "", actInstallValidate, "", 0)

	if len(setters) == 0 {
		return nil
	}

	b.addAction(&actionInfo{name: cadll.CloseExec, typ: caDLL | caDeferred | caNoImpersonate, source: cadll.Key, target: cadll.CloseEntryPoint})
	b.standardAt(seqInstallExecute, actRemoveFiles)
	b.schedule(seqInstallExecute, cadll.CloseExec, cadll.CloseExec, actRemoveFiles, "", 0)

	// A fixed seed first, so that nothing an impersonated action set in
	// BuilderCloseExec before survives into the list; the DLL skips the
	// empty path it leaves.
	reset := "BuilderCloseReset"
	b.addAction(&actionInfo{name: reset, typ: caSetProperty, source: cadll.CloseExec, target: cadll.CloseExecSeparator})
	b.schedule(seqInstallExecute, reset, "", cadll.CloseExec, "", 0)

	for _, s := range setters {
		b.addAction(&actionInfo{
			name: s.name, typ: caSetProperty, source: cadll.CloseExec,
			target: "[" + cadll.CloseExec + "]" + s.path + cadll.CloseExecSeparator,
		})
		b.schedule(seqInstallExecute, s.name, s.cond, cadll.CloseExec, "", 0)
	}

	return nil
}
