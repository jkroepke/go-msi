package wixconvert

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Schedule sequences and event names shared by several keys.
const (
	seqExecute  = "execute"
	seqUI       = "ui"
	onInstall   = "install"
	onUninstall = "uninstall"
	onBoth      = "both"
)

// sequenceNames maps WiX sequence elements to schedule sequences.
var sequenceNames = map[string]string{ //nolint:gochecknoglobals // lookup table
	"InstallExecuteSequence": seqExecute,
	"InstallUISequence":      seqUI,
	"AdminExecuteSequence":   "adminExecute",
	"AdminUISequence":        "adminUI",
}

// quietExec are the WiX extension entry points that run a command line
// without a window, which run actions do.
var quietExec = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"WixQuietExec": true, "WixQuietExec64": true, "CAQuietExec": true, "CAQuietExec64": true,
	"WixSilentExec": true, "WixSilentExec64": true, "CASilentExec": true, "CASilentExec64": true,
}

// quietExecProperties hold the command line of an immediate quiet exec.
var quietExecProperties = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"QtExecCmdLine": true, "QtExec64CmdLine": true, "WixQuietExecCmdLine": true, "WixQuietExec64CmdLine": true,
	"WixSilentExecCmdLine": true, "WixSilentExec64CmdLine": true,
}

// removeOldAfter are the actions after which upgrade.removeOld can remove
// the old version.
var removeOldAfter = map[string]bool{ //nolint:gochecknoglobals // lookup table
	"InstallValidate": true, "InstallInitialize": true, "InstallExecute": true, "InstallFinalize": true,
}

// placements reads the sequence tables into schedule entries by action.
func (c *converter) placements() map[string][]any {
	byAction := make(map[string][]any)

	for _, seq := range c.sequences {
		name, ok := sequenceNames[seq.name]
		if !ok {
			c.todof(nil, "", seq.line, "<%s> has no counterpart", seq.name)

			continue
		}

		for _, entry := range seq.kids {
			if entry.is("Custom") {
				id := entry.attr("Action")
				byAction[id] = append(byAction[id], c.placement(entry, name))
			} else {
				c.standardAction(entry, seq)
			}
		}
	}

	return byAction
}

// standardAction converts a standard action in a sequence.
func (c *converter) standardAction(entry, seq *elem) {
	switch {
	case entry.is("ScheduleReboot"):
		c.scheduleReboot(entry)
	case entry.is("RemoveExistingProducts") && c.major == nil && removeOldAfter[entry.attr("After")]:
		c.out.child("upgrade").set("removeOld", "after"+entry.attr("After"))
	case entry.is("Show"):
		c.todof(nil, "", entry.line, "<Show Dialog=%q> schedules a dialog; use the ui keys", entry.attr("Dialog"))
	default:
		c.todof(nil, "", entry.line, "rescheduling the standard action %s in %s has no counterpart", entry.name, seq.name)
	}
}

func (c *converter) placement(custom *elem, sequence string) *omap {
	placement := flowMap()
	placement.order("sequence", "before", "after", "at", "condition", "when")
	placement.set("sequence", sequence)
	placement.set("before", custom.attr("Before"))
	placement.set("after", custom.attr("After"))

	if at, err := strconv.Atoi(custom.attr("Sequence")); err == nil {
		placement.set("at", at)
	}

	c.setCondition(placement, c.condition(custom))

	if custom.has("OnExit") {
		c.todof(nil, "", custom.line, "actions that run on exit (OnExit) have no counterpart")
	}

	return placement
}

// condition returns the condition of a sequence entry: its Condition
// attribute, or in WiX v3 its text.
func (c *converter) condition(entry *elem) string {
	if entry.has("Condition") {
		return c.value(entry, "Condition")
	}

	return c.expand(entry, strings.TrimSpace(entry.text))
}

func (c *converter) scheduleReboot(entry *elem) {
	reboot := c.out.child("scheduleReboot")
	reboot.keep = true
	c.setCondition(reboot, c.condition(entry))

	if entry.attr("After") != "InstallFinalize" {
		c.todof(c.out, "scheduleReboot", entry.line, "ScheduleReboot is scheduled after InstallFinalize, not here")
	}
}

func (c *converter) emitActions() {
	byAction := c.placements()
	c.quietExecCommands()
	c.reanchor(byAction)

	for _, custom := range c.actionOrder {
		actionID := custom.attr("Id")
		if c.consumed[custom] || actionID == c.launchAction {
			continue
		}

		action := c.action(custom)
		if action == nil {
			continue
		}

		action.set("schedule", byAction[actionID])
		delete(byAction, actionID)
		c.out.add("customActions", action)
	}

	for _, setter := range c.setters {
		if c.consumed[setter] {
			continue
		}

		action, schedule := c.setter(setter)
		id := action.str("id")
		action.set("schedule", append(schedule, byAction[id]...))
		delete(byAction, id)
		c.out.add("customActions", action)
	}

	for _, id := range slices.Sorted(maps.Keys(byAction)) {
		if c.actions[id] == nil && id != c.launchAction {
			c.todof(nil, "", 0, "the sequences schedule %s, which this file does not define (an extension's action?)", id)
		}
	}
}

// reanchor moves the placements anchored on a command line setter, which
// becomes part of its run action, to the setter's own anchor.
func (c *converter) reanchor(byAction map[string][]any) {
	anchors := make(map[string]*omap)

	for src := range c.consumed {
		if entries := byAction[src.attr("Id")]; len(entries) > 0 {
			if first, ok := entries[0].(*omap); ok {
				anchors[src.attr("Id")] = first
			}
		}
	}

	for _, entries := range byAction {
		for i, entry := range entries {
			placement, ok := entry.(*omap)
			if !ok {
				continue
			}

			anchor, ok := anchors[placement.str("after")]
			if !ok {
				anchor, ok = anchors[placement.str("before")]
			}

			if ok {
				entries[i] = moveAnchor(placement, anchor)
			}
		}
	}
}

// moveAnchor returns the placement with the anchor of another one.
func moveAnchor(placement, anchor *omap) *omap {
	moved := flowMap()
	moved.order("sequence", "before", "after", "at", "condition", "when")

	for _, key := range []string{"sequence", "condition", "when"} {
		moved.set(key, placement.get(key))
	}

	for _, key := range []string{"before", "after", "at"} {
		moved.set(key, anchor.get(key))
	}

	return moved
}

func immediate(ca *elem) bool {
	x := ca.attr("Execute")

	return x == "" || x == "immediate"
}

// commandSource is an element that sets a property a quiet exec action
// reads its command line from: a CustomAction with Property and Value, or
// a SetProperty.
type commandSource struct {
	e        *elem
	id       string
	property string
}

// commandSources lists the elements that set a property.
func (c *converter) commandSources() []commandSource {
	var sources []commandSource

	for _, custom := range c.actionOrder {
		if custom.has(attrProperty) && custom.has("Value") {
			sources = append(sources, commandSource{e: custom, id: custom.attr("Id"), property: custom.attr(attrProperty)})
		}
	}

	for _, setter := range c.setters {
		if setter.is("SetProperty") {
			// The action WiX makes for a SetProperty is named after the
			// property and the action it is scheduled next to.
			sources = append(sources, commandSource{e: setter, id: setter.attr("Id"), property: setter.attr("Id")})
		}
	}

	return sources
}

// passesCommand reports whether a source passes the command line to the
// quiet exec action qe: through CustomActionData, the property named like
// a deferred action, or through the extension's command line property for
// an immediate one.
func passesCommand(src commandSource, qe *elem) bool {
	return src.property == qe.attr("Id") || quietExecProperties[src.property] && immediate(qe)
}

// commandFor picks the source of a quiet exec action's command line. Each
// immediate quiet exec action reads the one shared property, so the source
// is the setter scheduled right before it.
func (c *converter) commandFor(quiet *elem, sources []commandSource) (commandSource, bool) {
	var candidates []commandSource

	for _, src := range sources {
		if src.property == quiet.attr("Id") {
			return src, true
		}

		if passesCommand(src, quiet) {
			candidates = append(candidates, src)
		}
	}

	switch len(candidates) {
	case 0:
		return commandSource{}, false
	case 1:
		return candidates[0], true
	}

	if src, ok := c.scheduledBefore(quiet, candidates); ok {
		return src, true
	}

	c.todof(nil, "", quiet.line,
		"several actions set the command line property of %s and none is scheduled right before it; check its run.command", quiet.attr("Id"))

	return candidates[0], true
}

// scheduledBefore finds the candidate that the sequences schedule right
// before the action: Before it, named in its After, or with the highest
// sequence number below its own.
func (c *converter) scheduledBefore(quiet *elem, candidates []commandSource) (commandSource, bool) {
	actionID := quiet.attr("Id")
	entries := c.sequenceEntries()

	for _, src := range candidates {
		for _, entry := range append(entries[src.id], src.e) {
			if entry.attr("Before") == actionID {
				return src, true
			}
		}

		for _, entry := range entries[actionID] {
			if entry.attr("After") == src.id {
				return src, true
			}
		}
	}

	return lowerSequence(entries, actionID, candidates)
}

// lowerSequence finds the candidate with the highest sequence number below
// the action's.
func lowerSequence(entries map[string][]*elem, actionID string, candidates []commandSource) (commandSource, bool) {
	best, bestAt := commandSource{}, -1

	for _, entry := range entries[actionID] {
		ownAt, err := strconv.Atoi(entry.attr("Sequence"))
		if err != nil {
			continue
		}

		for _, src := range candidates {
			for _, srcEntry := range entries[src.id] {
				if srcAt, err := strconv.Atoi(srcEntry.attr("Sequence")); err == nil && srcAt < ownAt && srcAt > bestAt {
					best, bestAt = src, srcAt
				}
			}
		}
	}

	return best, bestAt >= 0
}

// sequenceEntries indexes the Custom elements of the sequences by action.
func (c *converter) sequenceEntries() map[string][]*elem {
	entries := make(map[string][]*elem)

	for _, seq := range c.sequences {
		for _, entry := range seq.kids {
			if entry.is("Custom") {
				entries[entry.attr("Action")] = append(entries[entry.attr("Action")], entry)
			}
		}
	}

	return entries
}

// quietExecCommands picks the command line source of every quiet exec
// action. The sources become part of the run actions, not actions of their
// own.
func (c *converter) quietExecCommands() {
	sources := c.commandSources()
	c.quietCommands = make(map[string]commandSource)
	c.consumed = make(map[*elem]bool)

	for _, quiet := range c.actionOrder {
		if !quietExec[quiet.attr("DllEntry")] {
			continue
		}

		if src, ok := c.commandFor(quiet, sources); ok {
			c.quietCommands[quiet.attr("Id")] = src
			c.consumed[src.e] = true
		}
	}
}

// command returns the command line a quiet exec action runs.
func (c *converter) command(quiet *elem) (string, bool) {
	if src, ok := c.quietCommands[quiet.attr("Id")]; ok {
		return c.value(src.e, "Value"), true
	}

	for _, prop := range c.properties {
		if quietExecProperties[prop.attr("Id")] && immediate(quiet) {
			return c.value(prop, "Value"), true
		}
	}

	return "", false
}

func newAction(e *elem) *omap {
	action := newMap()
	action.order("id", "setProperty", "setDirectory", "error", "exec", "execProperty", "dll", "exe", "run",
		"execute", "impersonate", "return", "hideTarget", "once", "schedule")
	action.head = comments(e)

	return action
}

func flowMap() *omap {
	m := newMap()
	m.flow = true

	return m
}

// action converts a CustomAction; nil means it cannot be converted.
func (c *converter) action(custom *elem) *omap {
	action := newAction(custom)
	action.set("id", custom.attr("Id"))

	if c.setterKind(action, custom) {
		c.actionFlags(action, custom, false)

		return action
	}

	if !c.programKind(action, custom) {
		return nil
	}

	c.actionFlags(action, custom, true)

	return action
}

// setterKind converts the actions that set a value or show an error.
func (c *converter) setterKind(action *omap, custom *elem) bool {
	switch {
	case custom.has("Error"):
		action.set("error", c.value(custom, "Error"))
	case custom.has(attrProperty) && custom.has("Value"):
		action.set("setProperty", flowMap().set("name", custom.attr(attrProperty)).set("value", c.value(custom, "Value")))
	case custom.has(attrDirectory) && custom.has("Value"):
		action.set("setDirectory", flowMap().set("directory", c.dirRef(custom.attr(attrDirectory))).set("value", c.value(custom, "Value")))
	default:
		return false
	}

	return true
}

// programKind converts the actions that run a program or a DLL; false
// means the action has no counterpart.
func (c *converter) programKind(action *omap, custom *elem) bool { //nolint:cyclop // one branch per WiX custom action type
	binary := custom.attr("BinaryRef") + custom.attr("BinaryKey")
	file := custom.attr("FileRef") + custom.attr("FileKey")
	exe := c.value(custom, "ExeCommand")

	switch {
	case file != "" && custom.has("ExeCommand"):
		action.set("run", newMap().set("command", strings.TrimSpace(`"[#`+file+`]" `+exe)))
	case custom.has(attrDirectory) && custom.has("ExeCommand"):
		action.set("run", newMap().set("command", exe).set("directory", c.dirRef(custom.attr(attrDirectory))))

		c.quoteNote(action, custom.line, exe)
	case custom.has(attrProperty) && custom.has("ExeCommand"):
		action.set("execProperty", newMap().set("property", custom.attr(attrProperty)).set("arguments", exe))
	case binary != "" && quietExec[custom.attr("DllEntry")]:
		c.quietExecRun(action, custom)
	case binary != "" && strings.HasPrefix(binary, "Wix"):
		c.todof(nil, "", custom.line, "the custom action %s calls the WiX extension %s!%s, which has no counterpart",
			custom.attr("Id"), binary, custom.attr("DllEntry"))

		return false
	case binary != "" && custom.has("DllEntry"):
		action.set("dll", flowMap().set("binary", binary).set("entry", custom.attr("DllEntry")))
	case binary != "" && custom.has("ExeCommand"):
		action.set("exe", newMap().set("binary", binary).set("arguments", exe))
	case custom.has("Script") || custom.has("VBScriptCall") || custom.has("JScriptCall") || strings.TrimSpace(custom.text) != "":
		c.todof(nil, "", custom.line, "the custom action %s is a script; script custom actions are never supported, use a run action", custom.attr("Id"))

		return false
	default:
		c.todof(nil, "", custom.line, "the custom action %s has no counterpart", custom.attr("Id"))

		return false
	}

	return true
}

func (c *converter) quietExecRun(action *omap, ca *elem) {
	cmd, ok := c.command(ca)
	if !ok {
		c.todof(action, "", ca.line, "the command line of %s is set elsewhere; write it into run.command", ca.attr("DllEntry"))
	}

	run := newMap().set("command", cmd)
	run.keep = true
	action.set("run", run)
}

func (c *converter) actionFlags(action *omap, custom *elem, program bool) { //nolint:cyclop // a flat list of independent WiX attributes
	switch execute := custom.attr("Execute"); execute {
	case "", "immediate":
	case "deferred", "rollback", "commit":
		action.set("execute", execute)
	case "oncePerProcess", "firstSequence":
		action.set("once", execute)
	case "secondSequence":
		action.set("once", "clientRepeat")
	default:
		c.todof(action, "", custom.line, "Execute=%q has no counterpart", execute)
	}

	if custom.no("Impersonate") {
		action.set("impersonate", false)
	}

	// Programs need an explicit return in msi-builder.
	ret := custom.attr("Return")
	if ret == "" && program {
		ret = "check"
	}

	action.set("return", ret)

	if custom.yes("HideTarget") {
		action.set("hideTarget", true)
	}

	if custom.has("Win64") || custom.has("Bitness") {
		c.todof(action, "", custom.line, "the action's bitness follows package.platform in msi-builder")
	}
}

// setterSequences are the sequences a SetProperty or SetDirectory runs in.
func (c *converter) setterSequences(action *omap, setter *elem) []string {
	switch seq := setter.attr("Sequence"); seq {
	case "", onBoth:
		return []string{seqUI, seqExecute}
	case seqExecute, seqUI:
		return []string{seq}
	default:
		c.todof(action, "", setter.line, "Sequence=%q has no counterpart", seq)

		return []string{seqExecute}
	}
}

// setter converts SetProperty and SetDirectory elements, which carry their
// own schedule.
func (c *converter) setter(setter *elem) (*omap, []any) {
	action := newAction(setter)
	setterID := setter.attr("Id")

	name := setter.attr("Action")
	if name == "" {
		name = "Set" + setterID
	}

	action.set("id", name)

	before, after := setter.attr("Before"), setter.attr("After")

	if setter.is("SetDirectory") {
		action.set("setDirectory", flowMap().set("directory", c.dirRef(setterID)).set("value", c.value(setter, "Value")))

		if before == "" && after == "" {
			after = "CostFinalize"
		}
	} else {
		action.set("setProperty", flowMap().set("name", setterID).set("value", c.value(setter, "Value")))
	}

	sequences := c.setterSequences(action, setter)
	schedule := make([]any, 0, len(sequences))

	for _, seq := range sequences {
		placement := flowMap().set("sequence", seq).set("before", before).set("after", after)
		c.setCondition(placement, c.value(setter, "Condition"))
		schedule = append(schedule, placement)
	}

	return action, schedule
}

// emitActionRefs notes references to actions of WiX extensions.
func (c *converter) emitActionRefs() {
	for _, ref := range c.actionRefs {
		refID := ref.attr("Id")

		if strings.Contains(refID, "BroadcastEnvironmentChange") || strings.Contains(refID, "BroadcastSettingChange") {
			c.todof(nil, "", ref.line,
				"%s has no counterpart: programs that are already running keep the old environment until they restart", refID)
		} else {
			c.todof(nil, "", ref.line, "the custom action %s comes from a WiX extension and has no counterpart", refID)
		}
	}
}

// whenConditions are the usual WiX conditions that a schedule's when
// expresses (docs/Migrating from WiX.md, "Custom actions").
var whenConditions = map[string]string{ //nolint:gochecknoglobals // lookup table
	`NOT Installed AND NOT WIX_UPGRADE_DETECTED`: onInstall,
	`WIX_UPGRADE_DETECTED`:                       "upgrade",
	`REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE`: onUninstall,
	`Installed AND NOT REMOVE~="ALL"`:            "maintenance",
}

// setCondition sets a schedule's condition, as when where one fits.
func (c *converter) setCondition(schedule *omap, cond string) {
	normalized := strings.Join(strings.Fields(cond), " ")

	// The when conditions name msi-builder's upgrade property, which
	// emitUpgrade sets to WIX_UPGRADE_DETECTED whenever the file uses it.
	if when, ok := whenConditions[normalized]; ok {
		schedule.set("when", when)

		return
	}

	schedule.set("condition", cond)
}

// quoteNote adds a note when the executable of a command line is an unquoted
// reference: a property alone may hold a quoted command line already (an
// UninstallString), anything else formats to a bare path that CreateProcess
// tries piece by piece (CWE-428), which msi-builder rejects.
func (c *converter) quoteNote(action *omap, line int, command string) {
	token, _, _ := strings.Cut(strings.TrimLeft(command, " \t"), " ")
	if strings.HasPrefix(token, `"`) || !strings.Contains(token, "[") {
		return
	}

	bare := len(token) >= 3 && token[0] == '[' && token[len(token)-1] == ']' &&
		strings.Count(token, "[") == 1 && strings.Count(token, "]") == 1 && !strings.ContainsRune(`#!$%\~`, rune(token[1]))
	if bare {
		c.todof(action, "run", line, "check %s: quote it if it holds a bare path; leave it if it holds a quoted command line such as an UninstallString", token)

		return
	}

	c.todof(action, "run", line, "quote the executable %s: an unquoted path with a space is tried piece by piece by CreateProcess", token)
}
