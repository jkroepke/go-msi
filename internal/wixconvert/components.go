package wixconvert

import (
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// fileRef is a converted file, for shortcuts and firewall rules that point
// at it.
type fileRef struct {
	comp *comp
	out  *omap
	key  string
	dir  string
	name string
}

// pendingShortcut is a Shortcut outside a File element, placed once every
// file is known.
type pendingShortcut struct {
	e    *elem
	comp *comp
	tree map[*dir]bool
}

// Hints for unsupported elements.
const (
	hintTwice = "install the file twice, in two components"
	hintCOM   = "write the COM registration with registry entries"
	hintRun   = "a run action, if you need it"
)

// keyDirectory is the directory key of components, shortcuts and features.
const keyDirectory = "directory"

// unsupported are elements that msi-builder cannot express, with what to
// do instead (docs/Migrating from WiX.md, "Not supported").
var unsupported = map[string]string{ //nolint:gochecknoglobals // lookup table
	"IniFile":                  "ship the complete file",
	"CopyFile":                 hintTwice,
	"DuplicateFile":            hintTwice,
	"MoveFile":                 hintTwice,
	"Class":                    hintCOM,
	"TypeLib":                  hintCOM,
	"AppId":                    hintCOM,
	"ODBCDriver":               "write the ODBC registration with registry entries",
	"ODBCDataSource":           "write the ODBC registration with registry entries",
	"IsolateComponent":         "leave it out",
	"Merge":                    "merge modules are not supported; install their files in components",
	"MergeRef":                 "merge modules are not supported; install their files in components",
	"EmbeddedChainer":          "not supported",
	"util:XmlFile":             "ship the complete file, or edit it with a run action (no rollback)",
	"util:XmlConfig":           "ship the complete file, or edit it with a run action (no rollback)",
	"util:EventManifest":       "install the manifest and register it with wevtutil in run actions",
	"util:User":                hintRun,
	"util:Group":               hintRun,
	"util:FileShare":           hintRun,
	"util:PerformanceCategory": hintRun,
	"util:CloseApplication":    "the built-in files-in-use dialog and Restart Manager handle running programs",
	"util:TouchFile":           "leave it out",
}

func (c *converter) emitComponents(tree map[*dir]bool) {
	for _, cp := range c.comps {
		cp.out = c.component(cp, tree)
	}

	for _, pending := range c.pending {
		c.placeShortcut(pending)
	}

	c.placeTypes()

	for _, comp := range c.comps {
		if comp.dropIfEmpty && !hasContent(comp.out) {
			// Its notes move to the file header.
			for _, line := range comp.out.commentLines() {
				if strings.HasPrefix(line, "TODO") {
					c.header = append(c.header, line)
				}
			}

			comp.out = nil

			continue
		}

		c.out.add("components", comp.out)
	}
}

// hasContent reports whether a component installs anything.
func hasContent(component *omap) bool {
	for _, key := range component.keys {
		switch key {
		case "id", keyDirectory, "guid", "condition", "bitness", "keyPath":
		default:
			if _, ok := component.values[key]; ok {
				return true
			}
		}
	}

	return false
}

// componentFlags are the yes/no attributes of a Component with a key of
// the same meaning.
var componentFlags = [][2]string{ //nolint:gochecknoglobals // lookup table
	{"Permanent", "permanent"}, {"NeverOverwrite", "neverOverwrite"},
	{"SharedDllRefCount", "sharedDllRefCount"}, {"Transitive", "transitive"},
}

func (c *converter) component(comp *comp, tree map[*dir]bool) *omap {
	elm := comp.e
	out := newMap()
	comp.out = out
	out.order("id", keyDirectory, "guid", "files", "registry", "removeRegistry", "removeFiles", "services",
		"serviceControl", "eventLog", "environment", "purge", "createFolder", "condition", "permanent",
		"neverOverwrite", "sharedDllRefCount", "transitive", "keyPath", "bitness", "folderPermissions", "shortcuts")
	out.head = comments(elm)
	out.set("id", comp.id)
	out.set(keyDirectory, c.dirValue(comp.dir, tree))

	switch guid := c.value(elm, "Guid"); {
	case guid == "*":
	case guid == "" && elm.has("Guid"):
		out.set("guid", "none")
	default:
		out.set("guid", guid)
	}

	out.set("condition", c.value(elm, "Condition"))

	for _, flag := range componentFlags {
		if elm.yes(flag[0]) {
			out.set(flag[1], true)
		}
	}

	c.bitness(out, elm)

	cc := &compConv{c: c, cp: comp, out: out, tree: tree, keyIsDir: elm.yes("KeyPath")}
	for _, kid := range elm.kids {
		cc.child(kid)
	}

	cc.finish()

	return out
}

// Bitness values.
const (
	platformX64 = "x64"
	platformX86 = "x86"
	always32    = "always32"
	always64    = "always64"
)

// Names that several elements share.
const (
	attrDirectory  = "Directory"
	attrProperty   = "Property"
	elemPermission = "Permission"
	elemCondition  = "Condition"
	rightsFull     = "full"
)

// WiX standard wizards and their variables.
const (
	uiMondo            = "WixUI_Mondo"
	uiFeatureTree      = "WixUI_FeatureTree"
	varWixUILicenseRtf = "WixUILicenseRtf"
	varWixUIBannerBmp  = "WixUIBannerBmp"
	varWixUIDialogBmp  = "WixUIDialogBmp"
)

func (c *converter) bitness(out *omap, elm *elem) {
	is64 := c.platform == platformX64 || c.platform == "arm64" || c.platform == ""

	switch {
	case elm.attr("Bitness") == always32, elm.no("Win64"):
		if is64 {
			out.set("bitness", "32")
		}
	case elm.attr("Bitness") == always64, elm.yes("Win64"):
		if !is64 {
			out.set("bitness", "64")
		}
	}
}

// compConv converts the content of one component.
type compConv struct {
	c            *converter
	cp           *comp
	out          *omap
	tree         map[*dir]bool
	controlled   map[*omap]bool
	keyPath      string // the key path a File or RegistryValue asks for
	hkcuKeyPaths []*omap
	removeFolder []*omap
	services     []*omap
	serviceLines []int
	controls     []*elem
	shortcuts    int
	files        int
	keyIsDir     bool
}

// componentChildren convert the elements a Component holds.
var componentChildren = map[string]func(cc *compConv, e *elem){ //nolint:gochecknoglobals // dispatch table
	"File":                  (*compConv).file,
	"Files":                 (*compConv).folderFiles,
	"RegistryKey":           func(cc *compConv, e *elem) { cc.registry(e, e.attr("Root"), "") },
	"RegistryValue":         func(cc *compConv, e *elem) { cc.registry(e, e.attr("Root"), "") },
	"RemoveRegistryKey":     (*compConv).removeRegistry,
	"RemoveRegistryValue":   (*compConv).removeRegistry,
	"Environment":           (*compConv).environment,
	"ServiceInstall":        (*compConv).service,
	"ServiceControl":        func(cc *compConv, e *elem) { cc.controls = append(cc.controls, e) },
	"CreateFolder":          (*compConv).createFolder,
	"RemoveFile":            (*compConv).removeFile,
	"RemoveFolder":          (*compConv).removeFile,
	"util:RemoveFolderEx":   (*compConv).purge,
	elemShortcut:            (*compConv).shortcut,
	"util:InternetShortcut": (*compConv).shortcut,
	"util:EventSource":      (*compConv).eventSource,
	"fw:FirewallException":  func(cc *compConv, e *elem) { cc.c.firewall(e, cc.cp, "") },
	"ProgId":                (*compConv).progID,
	// WiX v3: the condition is the text.
	elemCondition: func(cc *compConv, e *elem) { cc.out.set("condition", cc.c.expand(e, strings.TrimSpace(e.text))) },
}

const elemShortcut = "Shortcut"

func (cc *compConv) child(elm *elem) {
	if hint, ok := unsupported[elm.qname()]; ok {
		cc.c.todof(cc.out, "", elm.line, "<%s> is not supported: %s", elm.qname(), hint)

		return
	}

	if fn, ok := componentChildren[elm.qname()]; ok {
		fn(cc, elm)

		return
	}

	cc.c.todof(cc.out, "", elm.line, "<%s> in a component has no counterpart", elm.qname())
}

func (cc *compConv) shortcut(e *elem) {
	cc.shortcuts++
	cc.c.pending = append(cc.c.pending, pendingShortcut{e: e, comp: cc.cp, tree: cc.tree})
}

func (cc *compConv) finish() { //nolint:cyclop // one pass in a fixed order; splitting would scatter it
	cc.resolveControls()

	if cc.keyIsDir && (cc.files > 0 || cc.out.get("registry") != nil) {
		cc.out.set("keyPath", keyDirectory)
	} else if cc.keyPath != "" {
		cc.out.set("keyPath", cc.keyPath)
	}

	if cc.shortcuts > 0 {
		cc.dropShortcutRows()

		return
	}

	profile := cc.inProfile()

	for _, folder := range cc.removeFolder {
		if profile && folder.empty() {
			cc.out.head = append(cc.out.head,
				"The RemoveFolder of this component was left out: msi-builder writes one for every folder in the user profile.")

			continue
		}

		cc.out.add("removeFiles", folder)
	}

	for _, value := range cc.hkcuKeyPaths {
		// Not the first registry value: name it as the key path.
		if cc.out.get("registry") != nil && cc.files == 0 && cc.keyPath == "" {
			cc.out.set("keyPath", value.get("id"))
		} else {
			value.del("id")
		}

		cc.out.add("registry", value)
	}
}

// dropShortcutRows leaves out the RemoveFolder rows and the HKCU key path
// of a WiX shortcut component, which msi-builder writes for every shortcut
// itself.
func (cc *compConv) dropShortcutRows() {
	if len(cc.hkcuKeyPaths) > 0 || len(cc.removeFolder) > 0 {
		cc.out.head = append(cc.out.head,
			"The RemoveFolder and HKCU key path of this shortcut component were left out:",
			"msi-builder writes both for every shortcut.")
	}

	cc.cp.dropIfEmpty = true

	if cc.keyPath != "" && cc.out.get("registry") == nil {
		cc.out.del("keyPath")
	}
}

// fileFlags are the yes/no attributes of a File with a key of the same
// meaning.
var fileFlags = [][2]string{{"Checksum", "checksum"}, {"ReadOnly", "readOnly"}, {"Hidden", "hidden"}, {"System", "system"}} //nolint:gochecknoglobals,lll // lookup table

func (cc *compConv) file(elm *elem) { //nolint:cyclop // a flat list of independent WiX attributes
	conv := cc.c
	file := newMap()
	file.order("source", "name", "id", "version", "vital", "checksum", "readOnly", "hidden", "system",
		"permissions", "shortcuts", "fileTypes")
	file.head = comments(elm)

	src, name := cc.fileSource(file, elm)
	file.set("id", elm.attr("Id"))
	file.set("version", conv.value(elm, "DefaultVersion"))
	file.set("version", conv.value(elm, "CompanionFile"))

	if elm.no("Vital") {
		file.set("vital", false)
	}

	for _, flag := range fileFlags {
		if elm.yes(flag[0]) {
			file.set(flag[1], true)
		}
	}

	if elm.has("Assembly") {
		conv.todof(file, "", elm.line, "assemblies (Assembly=%q) have no counterpart", elm.attr("Assembly"))
	}

	key := config.FileKey(&config.File{Source: src, Name: name, ID: elm.attr("Id")})
	ref := &fileRef{key: key, dir: cc.cp.dir, name: name, comp: cc.cp, out: file}
	conv.files = append(conv.files, ref)

	if elm.yes("KeyPath") && cc.files > 0 {
		cc.keyPath = name
	}

	cc.files++

	for _, kid := range elm.kids {
		switch {
		case kid.qname() == elemShortcut:
			conv.fileShortcut(file, kid, ref, cc.tree)
		case permissionElems[kid.qname()]:
			conv.permission(file, "permissions", kid, rightsFile)
		case kid.qname() == "fw:FirewallException":
			conv.firewall(kid, cc.cp, "[#"+key+"]")
		default:
			conv.todof(file, "", kid.line, "<%s> in a File has no counterpart", kid.qname())
		}
	}

	cc.out.add("files", file)
}

// fileSource sets the file's source and name and returns both.
func (cc *compConv) fileSource(file *omap, elm *elem) (string, string) {
	conv := cc.c

	src := elm.attr("Source")
	if src == "" {
		src = elm.attr("src")
	}

	name := conv.value(elm, "Name")
	if src == "" {
		src = name
		conv.todof(file, "", elm.line, "the File has no Source; WiX finds %q through its bind paths, set the path", name)
	}

	if strings.Contains(src, "!(bindpath.") {
		conv.todof(file, "", elm.line, "bind paths have no counterpart; write the path to the file")
	}

	src = sourcePath(conv.expand(elm, src))
	file.set("source", src)

	if name == "" {
		return src, baseName(src)
	}

	if name != baseName(src) {
		file.set("name", name)
	}

	return src, name
}

// permissionElems are the elements that set permissions.
var permissionElems = map[string]bool{"util:PermissionEx": true, "PermissionEx": true, elemPermission: true} //nolint:gochecknoglobals // lookup table

func (cc *compConv) registry(elm *elem, root, parentKey string) {
	conv := cc.c

	if r := elm.attr("Root"); r != "" {
		root = r
	}

	key := conv.value(elm, "Key")
	if parentKey != "" {
		key = joinKey(parentKey, key)
	}

	if elm.is("RegistryValue") {
		cc.registryValue(elm, root, key)

		return
	}

	cc.forcedKey(elm, root, key)

	for _, kid := range elm.kids {
		switch {
		case kid.is("RegistryKey"), kid.is("RegistryValue"):
			cc.registry(kid, root, key)
		case permissionElems[kid.qname()]:
			conv.todof(cc.out, "", kid.line, "permissions on registry keys: add permissions to a registry entry of this key by hand")
		default:
			conv.todof(cc.out, "", kid.line, "<%s> in a RegistryKey has no counterpart", kid.qname())
		}
	}
}

// forcedKey writes the key entry of a RegistryKey that is created or
// deleted as a whole.
func (cc *compConv) forcedKey(e *elem, root, key string) {
	action := e.attr("Action") // WiX v3
	forceCreate := e.yes("ForceCreateOnInstall") || action == "create" || action == "createAndRemoveOnUninstall"
	forceDelete := e.yes("ForceDeleteOnUninstall") || action == "createAndRemoveOnUninstall"

	if !forceCreate && !forceDelete {
		return
	}

	entry := newMap().set("root", root).set("key", key).set("type", "key")
	if forceDelete {
		entry.set("removeOnUninstall", true)
	}

	cc.out.add("registry", entry)
}

const typeMultiString = "multiString"

// registryTypes maps WiX value types to msi-builder's.
var registryTypes = map[string]string{ //nolint:gochecknoglobals // lookup table
	"":              "",
	"string":        "",
	"expandable":    "expandString",
	typeMultiString: typeMultiString,
	"integer":       "dword",
	"binary":        "binary",
}

func (cc *compConv) registryValue(elm *elem, root, key string) { //nolint:cyclop // a flat list of independent WiX attributes
	conv := cc.c
	entry := newMap()
	entry.order("id", "root", "key", "name", "type", "value", "append", "prepend")
	entry.head = comments(elm)

	typ, ok := registryTypes[elm.attr("Type")]
	if !ok {
		conv.todof(entry, "", elm.line, "registry Type=%q has no counterpart", elm.attr("Type"))
	}

	entry.set("root", root).set("key", key).set("name", conv.value(elm, "Name")).set("type", typ)
	entry.set("value", cc.registryData(elm, typ))

	switch elm.attr("Action") {
	case "append":
		entry.set("append", true)
	case "prepend":
		entry.set("prepend", true)
	}

	entry.set("id", elm.attr("Id"))

	if !elm.yes("KeyPath") {
		cc.out.add("registry", entry)

		return
	}

	if entry.get("id") == nil {
		entry.set("id", cc.keyPathID(elm))
	}

	// An HKCU key path beside no file may be the one WiX needs for a
	// shortcut; finish decides.
	if strings.EqualFold(root, "HKCU") && cc.files == 0 {
		cc.hkcuKeyPaths = append(cc.hkcuKeyPaths, entry)

		return
	}

	if cc.files > 0 || cc.out.get("registry") != nil {
		cc.keyPath = entry.str("id")
	} else if elm.attr("Id") == "" {
		entry.del("id")
	}

	cc.out.add("registry", entry)
}

// keyPathID names a registry value that is a component's key path.
func (cc *compConv) keyPathID(e *elem) string {
	if cc.cp.id != "" {
		return cc.cp.id + "_KeyPath"
	}

	return "KeyPath_line" + strconv.Itoa(e.line)
}

// registryData is the value of a RegistryValue: its Value, its text (WiX
// v3) or the list of a multi-string value.
func (cc *compConv) registryData(elm *elem, typ string) any {
	conv := cc.c

	if typ != typeMultiString {
		if elm.has("Value") {
			return conv.value(elm, "Value")
		}

		return conv.expand(elm, strings.TrimSpace(elm.text))
	}

	var values []string

	if v := conv.value(elm, "Value"); v != "" {
		values = append(values, v)
	}

	for _, kid := range elm.kids {
		if kid.is("MultiStringValue") || kid.is("MultiString") {
			v := kid.attr("Value")
			if v == "" {
				v = strings.TrimSpace(kid.text)
			}

			values = append(values, conv.expand(kid, v))
		}
	}

	return values
}

func joinKey(parent, key string) string {
	if key == "" {
		return parent
	}

	return strings.TrimRight(parent, `\`) + `\` + key
}

func (cc *compConv) removeRegistry(elm *elem) {
	conv := cc.c
	entry := newMap().set("root", elm.attr("Root")).set("key", conv.value(elm, "Key"))

	if elm.is("RemoveRegistryValue") {
		name := conv.value(elm, "Name")
		if name == "" {
			conv.todof(entry, "", elm.line,
				"RemoveRegistryValue without Name removes the default value; msi-builder's removeRegistry without name removes the key")
		}

		entry.set("name", name)
		cc.out.add("removeRegistry", entry)

		return
	}

	if elm.attr("Action") == "removeOnUninstall" {
		// A registry entry with removeOnUninstall would also create the
		// key on install, which WiX never does.
		conv.todof(cc.out, "", elm.line, "RemoveRegistryKey %s\\%s was left out: msi-builder has no removal on uninstall only, "+
			"and removeRegistry removes the key on install", elm.attr("Root"), conv.value(elm, "Key"))

		return
	}

	cc.out.add("removeRegistry", entry)
}

func (cc *compConv) environment(elm *elem) {
	conv := cc.c
	env := newMap()
	env.order("name", "value", "action", "scope", "removeOnUninstall")
	env.head = comments(elm)
	env.set("name", conv.value(elm, "Name")).set("value", conv.value(elm, "Value"))

	switch action, part := elm.attr("Action"), elm.attr("Part"); {
	case action == "create":
		env.set("action", "setIfAbsent")
	case action == "remove":
		env.set("action", "remove")
	case part == "last":
		env.set("action", "append")
	case part == "first":
		env.set("action", "prepend")
	default:
		env.set("action", "set")
	}

	if elm.yes("System") {
		env.set("scope", "machine")
	} else {
		env.set("scope", "user")
	}

	if elm.yes("Permanent") {
		env.set("removeOnUninstall", false)
	}

	if sep := elm.attr("Separator"); sep != "" && sep != ";" {
		conv.todof(env, "", elm.line, "Separator=%q has no counterpart; msi-builder joins with ;", sep)
	}

	cc.out.add("environment", env)
}

// inProfile reports whether the component's folder is in the user profile.
func (cc *compConv) inProfile() bool {
	folder := cc.c.dirs[cc.cp.dir]
	if folder == nil {
		return config.IsProfileFolder(cc.cp.dir)
	}

	top := cc.c.top(folder)

	return top != nil && config.IsProfileFolder(top.id)
}

// folderFiles converts a WiX 4 Files element, which harvests a glob, to a
// folder source with include and exclude patterns, or a plain file source
// when the Include names one file.
func (cc *compConv) folderFiles(elm *elem) {
	conv := cc.c
	full := sourcePath(conv.expand(elm, elm.attr("Include")))
	src, include := splitGlob(full)

	files := newMap()
	files.order("source", "include", "exclude")
	files.head = comments(elm)

	if include == "" {
		src = full
	} else if include != "**" {
		files.set("include", []string{include})
	}

	files.set("source", src)

	var exclude []string

	for _, kid := range elm.kids {
		pattern := sourcePath(conv.expand(kid, kid.attr("Files")))

		rel, below := strings.CutPrefix(pattern, src)
		if src == "./" {
			rel, below = pattern, !strings.HasPrefix(pattern, "../") && !strings.HasPrefix(pattern, "/")
		}

		switch {
		case !kid.is("Exclude"):
			conv.todof(files, "", kid.line, "<%s> in <Files> has no counterpart", kid.qname())
		case include == "" || !below:
			conv.todof(files, "", kid.line, "the Exclude %s is not below the folder %s; leave the file out by hand", pattern, src)
		default:
			exclude = append(exclude, rel)
		}
	}

	files.set("exclude", exclude)
	cc.out.add("files", files)
	cc.files++
}

// splitGlob splits a glob path into the folder before its first wildcard,
// with a trailing slash, and the pattern below that folder.
func splitGlob(pattern string) (string, string) {
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if strings.ContainsAny(part, "*?[") {
			folder := strings.Join(parts[:i], "/")
			if folder == "" {
				folder = "."
			}

			return folder + "/", strings.Join(parts[i:], "/")
		}
	}

	return pattern, ""
}
