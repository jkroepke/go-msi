package wixconvert

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

func (cc *compConv) service(elm *elem) { //nolint:cyclop // a flat list of independent WiX attributes
	conv := cc.c
	svc := newMap()
	svc.order("name", "displayName", "description", "type", "start", "delayedAutoStart", "errorControl", "vital",
		"account", "password", "arguments", "dependencies", "failureActionsOnNonCrashFailures", "control", "recovery", "permissions")
	svc.head = comments(elm)
	svc.set("name", conv.value(elm, "Name"))
	svc.set("displayName", conv.value(elm, "DisplayName"))
	svc.set("description", conv.value(elm, "Description"))

	switch t := elm.attr("Type"); t {
	case "", "ownProcess":
	case "shareProcess":
		svc.set("type", t)
	default:
		conv.todof(svc, "", elm.line, "service Type=%q has no counterpart; drivers are not supported", t)
	}

	switch st := elm.attr("Start"); st {
	case "auto", "demand", "disabled":
		svc.set("start", st)
	case "":
	default:
		conv.todof(svc, "", elm.line, "service Start=%q has no counterpart", st)
	}

	switch ec := elm.attr("ErrorControl"); ec {
	case "", "normal":
	case "ignore", "critical":
		svc.set("errorControl", ec)
	default:
		conv.todof(svc, "", elm.line, "service ErrorControl=%q has no counterpart", ec)
	}

	if elm.yes("Vital") {
		svc.set("vital", true)
	}

	if a := conv.value(elm, "Account"); !strings.EqualFold(a, "LocalSystem") {
		svc.set("account", a)
	}

	svc.set("password", conv.value(elm, "Password"))
	svc.set("arguments", conv.value(elm, "Arguments"))

	for _, a := range []string{"LoadOrderGroup", "Interactive"} {
		if elm.has(a) && !elm.no(a) {
			conv.todof(svc, "", elm.line, "service %s has no counterpart", a)
		}
	}

	for _, k := range elm.kids {
		cc.serviceChild(svc, k)
	}

	cc.services = append(cc.services, svc)
	cc.serviceLines = append(cc.serviceLines, elm.line)
	cc.out.add("services", svc)
}

func (cc *compConv) serviceChild(svc *omap, kid *elem) { //nolint:cyclop // one branch per child element
	conv := cc.c

	switch kid.qname() {
	case "ServiceDependency":
		if kid.yes("Group") {
			conv.todof(svc, "", kid.line, "a dependency on a load order group has no counterpart")

			return
		}

		deps := svc.list("dependencies")
		svc.set("dependencies", append(deps, conv.value(kid, "Id")))
	case "ServiceConfig":
		if kid.yes("DelayedAutoStart") {
			svc.set("delayedAutoStart", true)
		}

		if kid.attr("FailureActionsWhen") == "failedToStopOrReturnedError" {
			svc.set("failureActionsOnNonCrashFailures", true)
		}

		for _, a := range []string{"PreShutdownDelay", "ServiceSid", "RequiredPrivilege"} {
			if kid.has(a) {
				conv.todof(svc, "", kid.line, "ServiceConfig %s has no counterpart", a)
			}
		}
	case "util:ServiceConfig":
		conv.recovery(svc, kid)
	default:
		if permissionElems[kid.qname()] {
			conv.permission(svc, "permissions", kid, rightsService)

			return
		}

		conv.todof(svc, "", kid.line, "<%s> in a ServiceInstall has no counterpart", kid.qname())
	}
}

func (c *converter) recovery(svc *omap, elm *elem) {
	recovery := newMap()

	if n, err := strconv.Atoi(elm.attr("ResetPeriodInDays")); err == nil {
		recovery.set("resetAfterDays", n)
	}

	var actions []string

	for _, a := range []string{"FirstFailureActionType", "SecondFailureActionType", "ThirdFailureActionType"} {
		switch v := elm.attr(a); v {
		case "":
		case "restart", "none":
			actions = append(actions, v)
		default:
			c.todof(svc, "recovery", elm.line, "the recovery action %q has no counterpart; restart and none are supported", v)

			actions = append(actions, "none")
		}
	}

	recovery.set("actions", actions)

	if v := elm.attr("RestartServiceDelayInSeconds"); v != "" {
		recovery.set("restartDelay", v+"s")
	}

	for _, a := range []string{"ProgramCommandLine", "RebootMessage"} {
		if elm.has(a) {
			c.todof(svc, "recovery", elm.line, "util:ServiceConfig %s has no counterpart", a)
		}
	}

	svc.set("recovery", recovery)
}

// control is a ServiceControl of a service the component installs.
func (cc *compConv) resolveControls() {
	for _, elm := range cc.controls {
		name := cc.c.value(elm, "Name")

		var own *omap

		for _, s := range cc.services {
			if s.get("name") == name {
				own = s
			}
		}

		if own != nil {
			cc.ownControl(own, elm)
		} else {
			cc.foreignControl(elm)
		}
	}

	for i, s := range cc.services {
		if !cc.controlled[s] {
			cc.c.todof(s, "", cc.serviceLines[i],
				"the WiX package has no ServiceControl for %v; msi-builder starts it on install and stops and deletes it by default (services[].control)",
				s.get("name"))
		}
	}
}

// events reports whether a ServiceControl event value includes install and
// uninstall.
func events(v string) (bool, bool) {
	return v == onInstall || v == onBoth, v == onUninstall || v == onBoth
}

func (cc *compConv) ownControl(svc *omap, elm *elem) {
	if cc.controlled == nil {
		cc.controlled = make(map[*omap]bool)
	}

	cc.controlled[svc] = true
	ctl := newMap()
	ctl.order("startOnInstall", "stopOnInstall", "deleteOnInstall", "stopOnUninstall", "deleteOnUninstall", "wait")

	startInstall, startUninstall := events(elm.attr("Start"))
	stopInstall, stopUninstall := events(elm.attr("Stop"))
	removeInstall, removeUninstall := events(elm.attr("Remove"))

	set := func(key string, value, def bool) {
		if value != def {
			ctl.set(key, value)
		}
	}

	set("startOnInstall", startInstall, true)
	set("stopOnInstall", stopInstall, true)
	set("deleteOnInstall", removeInstall, false)
	set("stopOnUninstall", stopUninstall, true)
	set("deleteOnUninstall", removeUninstall, true)

	if elm.no("Wait") {
		ctl.set("wait", false)
	}

	if startUninstall {
		cc.c.todof(ctl, "", elm.line, "starting the package's own service on uninstall has no counterpart")
	}

	svc.set("control", ctl)
}

func (cc *compConv) foreignControl(elm *elem) {
	ctl := newMap()
	ctl.order("name", "stopOnInstall", "startOnInstall", "deleteOnInstall", "stopOnUninstall", "startOnUninstall", "deleteOnUninstall", "wait")
	ctl.set("name", cc.c.value(elm, "Name"))

	for _, event := range [][3]string{
		{"Stop", "stopOnInstall", "stopOnUninstall"},
		{"Start", "startOnInstall", "startOnUninstall"},
		{"Remove", "deleteOnInstall", "deleteOnUninstall"},
	} {
		onInstall, onUninstall := events(elm.attr(event[0]))
		if onInstall {
			ctl.set(event[1], true)
		}

		if onUninstall {
			ctl.set(event[2], true)
		}
	}

	if elm.no("Wait") {
		ctl.set("wait", false)
	}

	cc.out.add("serviceControl", ctl)
}

func (cc *compConv) createFolder(elm *elem) {
	conv := cc.c

	if d := elm.attr(attrDirectory); d != "" && d != cc.cp.dir {
		conv.todof(cc.out, "", elm.line, "CreateFolder for %s, another folder than the component's: move it to a component of its own", d)

		return
	}

	cc.out.set("createFolder", true)

	for _, k := range elm.kids {
		if permissionElems[k.qname()] {
			conv.permission(cc.out, "folderPermissions", k, rightsFile)
		} else {
			conv.todof(cc.out, "", k.line, "<%s> in a CreateFolder has no counterpart", k.qname())
		}
	}
}

func (cc *compConv) removeFile(elm *elem) {
	conv := cc.c
	row := newMap()
	row.order("id", "name", "directory", "on")

	if elm.is("RemoveFile") {
		row.set("name", conv.value(elm, "Name"))
	}

	if elm.has(attrProperty) {
		conv.todof(row, "", elm.line, "the folder comes from the property %s: msi-builder takes a directory ID", elm.attr(attrProperty))
	}

	if d := elm.attr(attrDirectory); d != "" && d != cc.cp.dir {
		row.set("directory", conv.dirRef(d))
	}

	switch on := elm.attr("On"); on {
	case onInstall, onBoth:
		row.set("on", on)
	}

	if elm.is("RemoveFolder") {
		cc.removeFolder = append(cc.removeFolder, row)

		return
	}

	cc.out.add("removeFiles", row)
}

// purge converts util:RemoveFolderEx. The removal of the component's own
// folder on uninstall, when its directory ID is private, becomes purge,
// whose deferred rmdir FILE-03 verified; any other folder, a public ID or
// another On becomes a removeFolders entry, which reads the property at
// install time.
func (cc *compConv) purge(elm *elem) {
	conv := cc.c
	prop := elm.attr(attrProperty)
	when := elm.attr("On")
	cond := conv.value(elm, "Condition")

	if (prop == "" || prop == cc.cp.dir) && (when == "" || when == "uninstall") && !config.IsPublicProperty(cc.cp.dir) &&
		cc.out.get("purge") == nil {
		if cond != "" {
			cc.out.set("purge", newMap().set("condition", cond))
		} else {
			cc.out.set("purge", true)
		}

		return
	}

	cc.removeFolders(elm, cmp.Or(prop, cc.cp.dir), when, cond)
}

// removeFolders writes the removeFolders entry of a util:RemoveFolderEx.
func (cc *compConv) removeFolders(elm *elem, prop, when, cond string) {
	conv := cc.c
	entry := newMap().set("property", conv.dirRef(prop))

	// The removal reads the property before costing, where the install
	// dir only has a path when installDir.remember searched for it; WiX
	// packages restore it with a RegistrySearch of their own.
	inst := conv.installDir
	if _, isDir := conv.dirs[prop]; isDir && (inst == nil || inst.id != prop) {
		conv.todof(cc.out, "removeFolders", elm.line,
			"util:RemoveFolderEx reads %s before costing, where a directory has no path yet; remember the path in the registry "+
				"and name a property with a search instead", prop)
	}

	if inst != nil && conv.dirRef(inst.id) == conv.dirRef(prop) {
		install := conv.out.child("installDir")
		if install.get("remember") == nil {
			install.set("remember", true)
			install.comment("remember", "util:RemoveFolderEx reads the install dir before costing; remember searches for it there.")
		}
	}

	switch when {
	case onInstall, onBoth:
		entry.set("on", when)
	}

	if cond != "" {
		entry.set("condition", cond)
	}

	cc.out.add("removeFolders", entry)
}

func (cc *compConv) eventSource(elm *elem) {
	conv := cc.c
	source := newMap()
	source.set("source", conv.value(elm, "Name"))
	source.set("log", conv.value(elm, "Log"))
	source.set("messageFile", conv.value(elm, "EventMessageFile"))

	types := 0

	for i, a := range []string{"SupportsErrors", "SupportsWarnings", "SupportsInformationals", "SupportsSuccessAudits", "SupportsFailureAudits"} {
		if elm.yes(a) {
			types |= 1 << i
		}
	}

	if types != 0 {
		source.set("typesSupported", types)
	}

	for _, a := range []string{"CategoryMessageFile", "CategoryCount", "ParameterMessageFile"} {
		if elm.has(a) {
			conv.todof(source, "", elm.line, "util:EventSource %s has no counterpart", a)
		}
	}

	cc.out.add("eventLog", source)
}

// firewall converts a firewall exception into a firewall rule that follows
// the component. program is the file the rule is nested in, if any.
func (c *converter) firewall(elm *elem, comp *comp, program string) { //nolint:cyclop // a flat list of independent WiX attributes
	rule := newMap()
	rule.order("name", "description", "component", "program", "protocol", "port", "remoteAddresses", "profiles")
	rule.head = comments(elm)
	rule.set("name", c.value(elm, "Name"))
	rule.set("description", c.value(elm, "Description"))
	rule.set("component", c.compID(comp))

	if p := c.value(elm, "Program"); p != "" {
		program = p
	}

	if f := elm.attr("File"); f != "" {
		program = "[#" + f + "]"
	}

	rule.set("program", program)
	rule.set("protocol", c.value(elm, "Protocol"))
	rule.set("port", c.value(elm, "Port"))

	var remote []string

	switch scope := elm.attr("Scope"); scope {
	case "", "any":
	case "localSubnet":
		remote = append(remote, "LocalSubnet")
	default:
		c.todof(rule, "", elm.line, "firewall Scope=%q has no counterpart; list the addresses in remoteAddresses", scope)
	}

	for _, kid := range elm.kids {
		if kid.qname() == "fw:RemoteAddress" {
			v := kid.attr("Value")
			if v == "" {
				v = strings.TrimSpace(kid.text)
			}

			remote = append(remote, c.expand(kid, v))
		} else {
			c.todof(rule, "", kid.line, "<%s> in a firewall exception has no counterpart", kid.qname())
		}
	}

	rule.set("remoteAddresses", strings.Join(remote, ","))

	switch profile := elm.attr("Profile"); profile {
	case "", "all":
		rule.set("profiles", "all")
	default:
		rule.set("profiles", profile)
	}

	for _, a := range []string{"IgnoreFailure", "Outbound", "Interface", "InterfaceType", "LocalScope", "EdgeTraversal", "Grouping"} {
		if elm.has(a) && !elm.no(a) {
			c.todof(rule, "", elm.line, "firewall %s has no counterpart; msi-builder writes inbound rules", a)
		}
	}

	c.out.add("firewall", rule)
}

// compID returns the component's ID, giving it one when WiX generated it.
func (c *converter) compID(comp *comp) string {
	if comp.id == "" {
		c.anonymous++
		comp.id = "Component" + strconv.Itoa(c.anonymous)

		if comp.out != nil {
			comp.out.set("id", comp.id)
		}
	}

	return comp.id
}

func (cc *compConv) progID(elm *elem) { //nolint:cyclop // a flat list of independent WiX attributes
	conv := cc.c

	if elm.yes("Advertise") {
		conv.todof(cc.out, "", elm.line, "advertised ProgIds are never supported; converted as plain file types")
	}

	fileType := newMap()
	fileType.order("progId", "description", "iconIndex", "extensions", "verbs")
	fileType.set("progId", conv.value(elm, "Id"))
	fileType.set("description", conv.value(elm, "Description"))

	if n, err := strconv.Atoi(elm.attr("IconIndex")); err == nil && elm.has("Icon") {
		fileType.set("iconIndex", n)
	}

	target := ""

	for _, ext := range elm.kids {
		if !ext.is("Extension") {
			conv.todof(fileType, "", ext.line, "<%s> in a ProgId has no counterpart", ext.qname())

			continue
		}

		x := newMap().set("extension", conv.value(ext, "Id")).set("contentType", conv.value(ext, "ContentType"))
		fileType.add("extensions", x)

		for _, verb := range ext.kids {
			if !verb.is("Verb") {
				conv.todof(fileType, "", verb.line, "<%s> in an Extension has no counterpart", verb.qname())

				continue
			}

			verbOut := newMap().set("id", conv.value(verb, "Id")).set("text", conv.value(verb, "Command"))
			if verb.has("Argument") {
				verbOut.set("arguments", conv.value(verb, "Argument"))

				if verbOut.get("arguments") == nil {
					verbOut.values["arguments"] = ""
					verbOut.addKey("arguments")
				}
			}

			fileType.add("verbs", verbOut)

			if f := verb.attr("TargetFile"); f != "" {
				target = f
			}
		}
	}

	conv.pendingTypes = append(conv.pendingTypes, pendingType{e: elm, comp: cc.cp, target: target, out: fileType, parent: cc.out})
}

// pendingType is a ProgId that goes onto the file its verbs start, once
// every file is known.
type pendingType struct {
	e      *elem
	comp   *comp
	out    *omap
	parent *omap
	target string
}

func (c *converter) placeTypes() {
	for _, pending := range c.pendingTypes {
		var file *fileRef

		for _, f := range c.files {
			if pending.target != "" && f.key == pending.target || pending.target == "" && f.comp == pending.comp && file == nil {
				file = f
			}
		}

		if file == nil {
			c.todof(pending.parent, "", pending.e.line, "the ProgId %s opens no file of the package; file types go on the file they start", pending.e.attr("Id"))

			continue
		}

		file.out.add("fileTypes", pending.out)
	}
}

// Rights names of grants.
const (
	rightsFile    = "file"
	rightsService = "service"
)

// fileRights maps util:PermissionEx attributes to file rights: names
// where msi-builder has one, else the access mask bits from the Windows
// file access rights.
var fileRights = []struct { //nolint:gochecknoglobals // lookup table
	attr  string
	right string
	mask  int
}{
	{"GenericAll", rightsFull, 0},
	{"FileAllRights", rightsFull, 0},
	{"GenericRead", "read", 0},
	{"GenericWrite", "write", 0},
	{"GenericExecute", "", 0x1200A0},
	{"Read", "", 0x1},
	{"CreateFile", "", 0x2},
	{"Write", "", 0x2},
	{"CreateChild", "", 0x4},
	{"Append", "", 0x4},
	{"CreateSubdirectory", "", 0x4},
	{"ReadExtendedAttributes", "", 0x8},
	{"WriteExtendedAttributes", "", 0x10},
	{"Execute", "", 0x20},
	{"Traverse", "", 0x20},
	{"DeleteChild", "", 0x40},
	{"ReadAttributes", "", 0x80},
	{"WriteAttributes", "", 0x100},
	{"Delete", "", 0x10000},
	{"ReadPermission", "", 0x20000},
	{"ChangePermission", "", 0x40000},
	{"TakeOwnership", "", 0x80000},
	{"Synchronize", "", 0x100000},
}

var serviceRights = map[string]string{ //nolint:gochecknoglobals // lookup table
	"GenericAll":           rightsFull,
	"GenericRead":          "read",
	"ServiceStart":         "start",
	"ServiceStop":          "stop",
	"ServicePauseContinue": "pauseContinue",
}

// permission converts a permission element into the permissions key of target.
func (c *converter) permission(target *omap, key string, elm *elem, kind string) {
	perms := target.mapAt(key)
	if perms == nil {
		perms = newMap()
		perms.order("sddl", "grant", "condition")
		target.set(key, perms)
	}

	switch elm.qname() {
	case "PermissionEx":
		if sddl := c.value(elm, "Sddl"); sddl != "" {
			perms.set("sddl", sddl)

			return
		}

		c.todof(target, key, elm.line, "a PermissionEx without Sddl has no counterpart")
	case elemPermission:
		c.todof(target, key, elm.line, "<Permission> replaces the object's permissions; write them as an SDDL string in %s.sddl", key)
	case "util:PermissionEx":
		account := c.value(elm, "User")
		if d := c.value(elm, "Domain"); d != "" {
			account = d + `\` + account
		}

		g := newMap().set("account", account).set("rights", c.rights(target, key, elm, kind))
		perms.add("grant", g)
	}
}

func (c *converter) rights(target *omap, key string, elm *elem, kind string) []string {
	var (
		names []string
		mask  int
	)

	for _, attrName := range elm.order {
		if !elm.yes(attrName) {
			continue
		}

		if kind == rightsService {
			if r, ok := serviceRights[attrName]; ok {
				names = append(names, r)
			} else {
				c.todof(target, key, elm.line, "the service right %s has no name in msi-builder; add its access mask", attrName)
			}

			continue
		}

		for _, right := range fileRights {
			if right.attr != attrName {
				continue
			}

			if right.right != "" {
				names = append(names, right.right)
			} else {
				mask |= right.mask
			}
		}
	}

	if mask != 0 {
		names = append(names, "0x"+strings.ToUpper(strconv.FormatInt(int64(mask), 16)))
	}

	return names
}
