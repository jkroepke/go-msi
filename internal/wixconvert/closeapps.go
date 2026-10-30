package wixconvert

import (
	"strings"
)

const elemCloseApp = "util:CloseApplication"

// closeNote lists what files[].close does differently from
// util:CloseApplication.
const closeNote = "WiX matches the running program by its file name, close by its installed path; RebootPrompt, Timeout " +
	"and ElevatedCloseMessage map only roughly; set the property MSIRESTARTMANAGERCONTROL to Disable, so that the " +
	"Restart Manager does not close the program as well"

// emitCloseApps converts util:CloseApplication elements that close a
// program the package installs into files[].close of its file.
func (c *converter) emitCloseApps() {
	for _, elm := range c.closeApps {
		target := strings.TrimSpace(c.literal(elm.attr("Target")))

		if !closesApp(elm) {
			c.todof(nil, "", elm.line, "<util:CloseApplication> for %s only detects the running program (Property %s), "+
				"which has no counterpart; files[].close on the program's file closes it instead", target, elm.attr("Property"))

			continue
		}

		refs := c.filesNamed(target)
		if len(refs) == 0 {
			c.todof(nil, "", elm.line, "<util:CloseApplication> closes %s, which is no file this package installs; "+
				"files[].close closes only files of the package", target)

			continue
		}

		for _, ref := range refs {
			c.closeFile(ref.out, elm)
		}
	}
}

// closesApp reports whether a util:CloseApplication closes or ends the
// program, rather than only setting its Property when it runs.
func closesApp(elm *elem) bool {
	return elm.yes("CloseMessage") || elm.yes("EndSessionMessage") || elm.yes("ElevatedCloseMessage") ||
		elm.yes("ElevatedEndSessionMessage") || elm.has("TerminateProcess")
}

// filesNamed returns the files of the package with an installed name.
func (c *converter) filesNamed(name string) []*fileRef {
	var refs []*fileRef

	for _, ref := range c.files {
		if name != "" && strings.EqualFold(ref.name, name) && ref.out != nil {
			refs = append(refs, ref)
		}
	}

	return refs
}

// closeFile sets close on a file: WiX ends the program only with
// TerminateProcess, close by default.
func (c *converter) closeFile(file *omap, elm *elem) {
	if elm.has("TerminateProcess") {
		file.set("close", true)
	} else {
		file.set("close", flowMap().set("terminate", false))
	}

	if cond := c.condition(elm); cond != "" {
		c.todof(file, "close", elm.line, "<util:CloseApplication> closes the program only when %s; close has no condition", cond)
	}

	c.todof(file, "close", elm.line, "close came from <util:CloseApplication>: "+closeNote)
}
