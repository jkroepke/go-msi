package wixconvert

import (
	"slices"
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

	// Without the attribute WiX restarts at once, while msi-builder
	// waits a minute by default.
	delay := elm.attr("RestartServiceDelayInSeconds")
	if delay == "" {
		delay = "0"
	}

	recovery.set("restartDelay", delay+"s")

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

// movableDir returns the first directory from dirID up to its root that can
// point anywhere: one whose output ID is public, which the command line sets,
// or one an action or property sets.
func (c *converter) movableDir(dirID string) string {
	set := c.setDirs()

	for folder := c.dirs[dirID]; folder != nil; folder = folder.parent {
		if id := c.dirRef(folder.id); set[folder.id] || config.IsPublicProperty(id) && !config.IsStandardFolder(id) {
			return id
		}
	}

	if c.dirs[dirID] == nil && (set[dirID] || config.IsPublicProperty(c.dirRef(dirID))) {
		return c.dirRef(dirID)
	}

	return ""
}

// setDirs returns the WiX IDs that a SetDirectory, SetProperty, custom
// action or property sets.
func (c *converter) setDirs() map[string]bool {
	set := make(map[string]bool)

	for _, setter := range c.setters {
		set[setter.attr("Id")] = true
	}

	for _, custom := range c.actionOrder {
		if custom.has("Value") {
			set[custom.attr(attrDirectory)+custom.attr(attrProperty)] = true
		}
	}

	for _, prop := range c.properties {
		set[prop.attr("Id")] = true
	}

	return set
}

func (cc *compConv) purge(elm *elem) {
	conv := cc.c

	target := elm.attr(attrProperty)
	if target == "" {
		target = cc.cp.dir
	}

	if movable := conv.movableDir(target); movable != "" {
		conv.todof(cc.out, "", elm.line,
			"util:RemoveFolderEx was not converted: purge refuses a folder below %s, a directory that can point anywhere; "+
				"keep the data in a folder with private (mixed-case) IDs below a fixed standard folder, such as "+
				"CommonAppDataFolder/Vendor/Product, and purge that", movable)

		return
	}

	if p := elm.attr(attrProperty); p != "" && p != cc.cp.dir {
		conv.todof(cc.out, "purge", elm.line,
			"util:RemoveFolderEx removes the folder in %s; purge removes the component's folder, so put this component there", p)
	}

	if on := elm.attr("On"); on != "" && on != "uninstall" {
		conv.todof(cc.out, "purge", elm.line, "purge runs on uninstall only, not On=%q", on)
	}

	if cond := conv.value(elm, "Condition"); cond != "" {
		cc.out.set("purge", newMap().set("condition", cond))

		return
	}

	cc.out.set("purge", true)
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
	c.firewallProtocol(rule, elm)

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

// firewallProtocol sets the protocol and port of a firewall rule. A WiX
// exception without Protocol is not limited to TCP, msi-builder's default:
// without a port it becomes any, and with one it needs a choice, because a
// Windows firewall rule with a port must name TCP or UDP.
func (c *converter) firewallProtocol(rule *omap, elm *elem) {
	protocol, port := c.value(elm, "Protocol"), c.value(elm, "Port")

	switch {
	case protocol != "":
	case port == "":
		protocol = "any"
	default:
		protocol = "tcp"

		c.todof(rule, "protocol", elm.line, "the firewall exception sets no Protocol; msi-builder opens port %s for tcp only, "+
			"add a udp rule if the program needs it", port)
	}

	rule.set("protocol", protocol)
	rule.set("port", port)
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

// namedRight is a util:PermissionEx attribute and the right it grants: a
// msi-builder right name, or else access mask bits.
type namedRight struct {
	attr  string
	right string
	mask  int
}

// fileRights maps util:PermissionEx attributes to file rights: names
// where msi-builder has one, else the access mask bits from the Windows
// file access rights.
var fileRights = []namedRight{ //nolint:gochecknoglobals // lookup table
	{"GenericAll", rightsFull, 0},
	{"FileAllRights", rightsFull, 0},
	{"GenericRead", "read", 0},
	{"GenericWrite", "", 0x120116}, // FILE_GENERIC_WRITE; write is 0x100116
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

// serviceRights maps util:PermissionEx attributes to service rights: names
// where msi-builder has one, else the access mask bits from the Windows
// service access rights.
var serviceRights = []namedRight{ //nolint:gochecknoglobals // lookup table
	{"GenericAll", rightsFull, 0},
	{"GenericRead", "read", 0},
	{"GenericWrite", "", 0x20002},   // READ_CONTROL | SERVICE_CHANGE_CONFIG
	{"GenericExecute", "", 0x20170}, // READ_CONTROL | start, stop, pause, user-defined control
	{"ServiceQueryConfig", "", 0x1},
	{"ServiceChangeConfig", "", 0x2},
	{"ServiceQueryStatus", "", 0x4},
	{"ServiceEnumerateDependents", "", 0x8},
	{"ServiceStart", "start", 0},
	{"ServiceStop", "stop", 0},
	{"ServicePauseContinue", "pauseContinue", 0},
	{"ServiceInterrogate", "", 0x80},
	{"ServiceUserDefinedControl", "", 0x100},
	{"Delete", "", 0x10000},
	{"ReadPermission", "", 0x20000},
	{"ChangePermission", "", 0x40000},
	{"TakeOwnership", "", 0x80000},
}

// wellKnownAccounts are the SDDL aliases of built-in accounts by their
// English names, which other Windows languages translate.
var wellKnownAccounts = map[string]string{ //nolint:gochecknoglobals // lookup table
	"administrators": "BA", "users": "BU", "guests": "BG", "power users": "PU", "everyone": "WD",
	"authenticated users": "AU", "system": "SY", "localsystem": "SY", "local service": "LS", "localservice": "LS",
	"network service": "NS", "networkservice": "NS", "interactive": "IU", "remote desktop users": "RD",
}

// grantAccount names a util:PermissionEx account the way grants take it:
// a built-in account by its SDDL alias, so the name resolves in every
// Windows language.
func grantAccount(domain, user string) string {
	switch strings.ToLower(domain) {
	case "", "builtin", "nt authority":
		if alias, ok := wellKnownAccounts[strings.ToLower(user)]; ok {
			return alias
		}
	}

	if domain != "" {
		return domain + `\` + user
	}

	return user
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
		account := grantAccount(c.value(elm, "Domain"), c.value(elm, "User"))

		g := newMap().set("account", account).set("rights", c.rights(target, key, elm, kind))
		perms.add("grant", g)
	}
}

func (c *converter) rights(target *omap, key string, elm *elem, kind string) []string {
	var (
		names []string
		mask  int
	)

	table := fileRights
	if kind == rightsService {
		table = serviceRights
	}

	for _, attrName := range elm.order {
		if !elm.yes(attrName) {
			continue
		}

		idx := slices.IndexFunc(table, func(r namedRight) bool { return r.attr == attrName })

		switch {
		case idx < 0 && kind == rightsService:
			c.todof(target, key, elm.line, "the service right %s has no counterpart; add its access mask", attrName)
		case idx < 0:
		case table[idx].right != "":
			names = append(names, table[idx].right)
		default:
			mask |= table[idx].mask
		}
	}

	if mask != 0 {
		names = append(names, "0x"+strings.ToUpper(strconv.FormatInt(int64(mask), 16)))
	}

	return names
}
