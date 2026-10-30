package wixconvert

import (
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/pkg/config"
)

// emit writes the output in msi-builder's key order.
func (c *converter) emit() {
	c.out.set("schema", "msi-builder/v1")
	c.out.order("vars", "product", "package", "upgrade", "installDir", "directories", "features", "components",
		"properties", "launchConditions", "customActions", "icons", "binaries", "firewall", "scheduleReboot", "ui")

	for _, d := range c.defines {
		c.out.child("vars").set(d[0], c.expand(c.pkg, d[1]))
	}

	c.markNeededDirs()
	tree := c.treeDirs()

	c.emitProduct()
	c.emitPackage()
	c.emitUpgrade()
	c.emitDirs(tree)
	c.emitComponents(tree)
	c.emitFeatures()
	c.emitProperties()
	c.emitLaunchConditions()
	c.emitActions()
	c.emitFiles()
	c.emitUI()
	c.emitErrorRows()
	c.emitActionRefs()
	c.guidNote()
}

// guidNote warns about components whose GUID WiX generated when the old
// version is removed late. msi-builder generates other GUIDs, so the
// upgrade sees new components, and removing the old package then deletes
// files the new one installed.
func (c *converter) guidNote() {
	upgrade := c.out.mapAt("upgrade")
	if upgrade == nil {
		return
	}

	if removeOld := upgrade.str("removeOld"); removeOld != "afterInstallExecute" && removeOld != "afterInstallFinalize" {
		return
	}

	var generated []string

	for _, comp := range c.comps {
		if comp.out != nil && comp.out.get("guid") == nil {
			generated = append(generated, c.compID(comp))
		}
	}

	if len(generated) == 0 {
		return
	}

	c.todof(nil, "", 0, "WiX generated the GUIDs of %s, and msi-builder generates other ones; with removeOld %s an upgrade "+
		"from the WiX package then deletes their files. Copy WiX's GUIDs into components[].guid: msi-builder inspect --table Component wix.msi",
		strings.Join(generated, ", "), upgrade.str("removeOld"))
}

func (c *converter) emitProduct() {
	product := c.out.child("product")
	product.set("name", c.value(c.pkg, "Name"))
	product.set("manufacturer", c.value(c.pkg, "Manufacturer"))
	product.set("version", c.value(c.pkg, "Version"))

	if lang := c.value(c.pkg, "Language"); lang != "" && lang != "1033" && lang != "0" {
		if n, err := strconv.Atoi(lang); err == nil {
			product.set("language", n)
		} else {
			c.todof(product, "", c.pkg.line, "Language=%q is not a number; set product.language", lang)
		}
	}

	product.set("upgradeCode", c.value(c.pkg, "UpgradeCode"))

	code := c.value(c.pkg, "ProductCode")
	if c.v3 {
		code = c.value(c.pkg, "Id")
	}

	if code != "" && code != "*" {
		product.set("productCode", code)
		product.comment("productCode", "A pinned product code; msi-builder derives one per build when this key is left out.")
	}

	if s := c.summary; s != nil {
		product.set("description", c.value(s, "Description"))
		product.set("comments", c.value(s, "Comments"))
	}

	c.emitARP(product)
}

// arpKeys maps ARP properties to product.arp keys.
var arpKeys = map[string]string{ //nolint:gochecknoglobals // lookup table
	"ARPCOMMENTS":        "comments",
	"ARPCONTACT":         "contact",
	"ARPHELPLINK":        "helpLink",
	"ARPURLINFOABOUT":    "aboutLink",
	"ARPURLUPDATEINFO":   "updateLink",
	"ARPPRODUCTICON":     "icon",
	"ARPNOREPAIR":        "noRepair",
	"ARPNOMODIFY":        "noModify",
	"ARPNOREMOVE":        "noRemove",
	"ARPSIZE":            "size",
	"ARPINSTALLLOCATION": "installLocation",
}

func (c *converter) emitARP(p *omap) {
	arp := p.child("arp")

	for _, elm := range c.properties {
		key, ok := arpKeys[elm.attr("Id")]
		if !ok {
			continue
		}

		val := c.value(elm, "Value")

		switch key {
		case "noRepair", "noModify", "noRemove":
			if val != "" && val != "0" {
				arp.set(key, true)
			}
		case "size":
			if n, err := strconv.Atoi(val); err == nil {
				arp.set(key, n)
			}
		default:
			arp.set(key, val)
		}
	}
}

func (c *converter) emitPackage() { //nolint:cyclop // a flat list of independent WiX attributes
	pkgOut := c.out.child("package")

	if c.platform == "" {
		pkgOut.set("platform", "x64")
		c.todof(pkgOut, "platform", c.pkg.line, "WiX takes the platform from wix build -arch; check package.platform")
	} else {
		pkgOut.set("platform", c.platform)
	}

	c.emitScope(pkgOut)

	ver := c.value(c.pkg, "InstallerVersion")
	if c.v3 && c.summary != nil {
		ver = c.value(c.summary, "InstallerVersion")
	}

	if n, err := strconv.Atoi(ver); err == nil {
		pkgOut.set("installerVersion", strconv.Itoa(n/100)+"."+strconv.Itoa(n%100))
	}

	cp := c.value(c.pkg, "Codepage")
	if n, err := strconv.Atoi(cp); err == nil {
		pkgOut.set("codepage", n)
	}

	if c.v3 && c.summary != nil && c.summary.no("Compressed") || !c.v3 && c.pkg.no("Compressed") {
		c.todof(pkgOut, "", c.pkg.line, "Compressed=\"no\": msi-builder always compresses the files into cabinets")
	}

	c.emitMedia(pkgOut)

	if s := c.summary; s != nil {
		sum := pkgOut.child("summary")
		sum.set("keywords", c.value(s, "Keywords"))

		if !c.v3 {
			sum.set("author", c.value(s, "Manufacturer"))
		}
	}
}

func (c *converter) emitScope(pkgOut *omap) {
	scope := c.value(c.pkg, "Scope")
	if c.v3 && c.summary != nil {
		scope = c.value(c.summary, "InstallScope")
	}

	scope, allUsers := c.allUsers(pkgOut, scope)
	if c.v3 && scope == "" && !allUsers {
		c.perUserNote(pkgOut)
	}

	if scope != "" && scope != "perMachine" {
		pkgOut.set("scope", scope)
	}
}

// allUsers applies an ALLUSERS property to the scope and reports whether
// the file sets one.
func (c *converter) allUsers(pkgOut *omap, scope string) (string, bool) {
	found := false

	for _, elm := range c.properties {
		if elm.attr("Id") != "ALLUSERS" {
			continue
		}

		found = true

		switch c.value(elm, "Value") {
		case "2":
			c.todof(pkgOut, "scope", elm.line,
				"ALLUSERS=2 lets the user choose the scope; use scope perUserOrMachine or perMachineOrUser")
		case "":
			scope = "perUser"
		}
	}

	return scope, found
}

// perUserNote notes a WiX v3 package that sets no scope at all.
func (c *converter) perUserNote(pkgOut *omap) {
	line := c.pkg.line
	if c.summary != nil {
		line = c.summary.line
	}

	c.todof(pkgOut, "scope", line,
		"the WiX v3 Package has no InstallScope and the file sets no ALLUSERS, which Windows Installer installs per user; "+
			"msi-builder installs per machine by default, set scope: perUser to keep the per-user install")
}

func (c *converter) emitMedia(pkgOut *omap) {
	if len(c.media) > 1 {
		c.todof(pkgOut, "", c.media[1].line, "several Media elements: msi-builder writes one cabinet, or several beside the package (embedCabinets: false)")
	}

	for _, medium := range c.media {
		if medium.no("EmbedCab") {
			pkgOut.set("embedCabinets", false)
			pkgOut.set("cabinet", c.value(medium, "Cabinet"))
		}

		if medium.attr("CompressionLevel") == valueNone {
			pkgOut.set("compression", "none")
		}

		if medium.has("DiskPrompt") || medium.has("VolumeLabel") {
			c.todof(pkgOut, "", medium.line, "DiskPrompt and VolumeLabel are left out: they only matter for packages on several disks")
		}
	}
}

func (c *converter) emitUpgrade() {
	upgrade := c.out.child("upgrade")
	upgradeCode := strings.Trim(strings.ToLower(c.pkg.attr("UpgradeCode")), "{}")

	if c.major != nil {
		c.emitMajor(upgrade, c.major)
	}

	for _, upgradeElem := range c.upgrades {
		c.emitRelated(upgrade, upgradeElem, strings.Trim(strings.ToLower(upgradeElem.attr("Id")), "{}") == upgradeCode)
	}

	if c.major == nil && len(c.upgrades) == 0 {
		c.todof(nil, "", c.pkg.line,
			"the WiX package has no MajorUpgrade; msi-builder adds the standard major upgrade (set upgrade: false to leave it out)")
	}

	text := c.allText()
	if strings.Contains(text, "WIX_UPGRADE_DETECTED") {
		upgrade.set("property", "WIX_UPGRADE_DETECTED")
	}

	if strings.Contains(text, "WIX_DOWNGRADE_DETECTED") {
		c.newerProperty(upgrade, "WIX_DOWNGRADE_DETECTED", 0)
	}
}

// newerProperty sets the property that the detection of newer versions sets,
// with a TODO when the file tests two of them.
func (c *converter) newerProperty(upgrade *omap, prop string, line int) {
	switch current := c.newerClaimed; {
	case current == "":
		c.newerClaimed = prop
		if prop != config.DefaultNewerProperty {
			upgrade.set("newerProperty", prop)
		}
	case prop != current:
		key := "newerProperty"
		if upgrade.str(key) == "" {
			key = ""
		}

		c.todof(upgrade, key, line, "the file tests both %s and %s for a newer version; msi-builder sets only %s, "+
			"use it in both places", current, prop, current)
	}
}

// emitMajor converts a MajorUpgrade element.
func (c *converter) emitMajor(upgrade *omap, major *elem) {
	switch schedule := major.attr("Schedule"); schedule {
	case "", "afterInstallValidate":
		// WiX's default; msi-builder's afterInstallInitialize also
		// removes the old version first, inside the transaction.
		if schedule != "" {
			upgrade.set("removeOld", schedule)
		}
	case "afterInstallInitialize", "afterInstallExecute", "afterInstallFinalize":
		upgrade.set("removeOld", schedule)
	default:
		c.todof(upgrade, "", major.line, "MajorUpgrade Schedule=%q has no counterpart", schedule)
	}

	if major.yes("AllowDowngrades") {
		upgrade.set("downgrade", "allow")
	}

	if major.yes("Disallow") {
		c.todof(upgrade, "", major.line, "MajorUpgrade Disallow=\"yes\": msi-builder always upgrades; set upgrade: false to turn upgrades off")
	}

	upgrade.set("downgradeMessage", c.value(major, "DowngradeErrorMessage"))

	if major.yes("IgnoreRemoveFailure") {
		upgrade.set("ignoreRemoveFailure", true)
	}

	if major.no("MigrateFeatures") {
		upgrade.set("migrateFeatures", false)
	}

	if major.has("RemoveFeatures") {
		c.todof(upgrade, "", major.line, "MajorUpgrade RemoveFeatures has no counterpart")
	}
}

// majorUpgradeProperty are the properties of the major upgrade msi-builder
// writes, which a related row cannot take.
var majorUpgradeProperty = map[string]bool{ //nolint:gochecknoglobals // lookup table
	config.DefaultUpgradeProperty: true, config.DefaultNewerProperty: true, "WIX_UPGRADE_DETECTED": true, "WIX_DOWNGRADE_DETECTED": true,
}

// valueAny and valueNone are WiX and msi-builder words for no limit and
// for nothing.
const (
	valueAny  = "any"
	valueNone = "none"
)

func orAny(version string) string {
	if version == "" {
		return valueAny
	}

	return version
}

// ownUpgradeRow handles an UpgradeVersion row of the package's own
// UpgradeCode that does not carry over to upgrade.related and reports
// whether it did.
func (c *converter) ownUpgradeRow(upgrade *omap, kid *elem) bool {
	switch {
	case kid.yes("OnlyDetect") && kid.no("IncludeMinimum") && !kid.has("Maximum") &&
		kid.attr("Minimum") == c.pkg.attr("Version") && kid.has("Property"):
		// The detection of newer versions, which msi-builder writes
		// itself; keep the property conditions test.
		c.newerProperty(upgrade, kid.attr("Property"), kid.line)
	case majorUpgradeProperty[kid.attr("Property")]:
		c.todof(upgrade, "", kid.line, "the UpgradeVersion property %s is one msi-builder sets for its own major upgrade; "+
			"check the conditions that test it", kid.attr("Property"))
	case !kid.yes("OnlyDetect"):
		c.todof(upgrade, "", kid.line, "an UpgradeVersion for the package's own UpgradeCode (versions %s to %s) is a hand-made major upgrade; "+
			"msi-builder writes its own, which removes every older version, check the upgrade keys",
			orAny(c.value(kid, "Minimum")), orAny(c.value(kid, "Maximum")))
	default:
		return false
	}

	return true
}

// emitRelated converts the UpgradeVersion rows of an Upgrade element. For
// the package's own UpgradeCode only the detect-only rows carry over: the
// rows that remove older versions are the major upgrade msi-builder writes
// itself.
func (c *converter) emitRelated(upgrade *omap, upgradeElem *elem, own bool) {
	for _, kid := range upgradeElem.kids {
		switch {
		case !kid.is("UpgradeVersion"):
			c.todof(upgrade, "", kid.line, "<%s> in <Upgrade> has no counterpart", kid.qname())

			continue
		case own && c.ownUpgradeRow(upgrade, kid):
			continue
		}

		row := newMap()
		row.set("upgradeCode", c.value(upgradeElem, "Id"))
		row.set("minVersion", c.value(kid, "Minimum"))
		row.set("maxVersion", c.value(kid, "Maximum"))

		if kid.no("IncludeMinimum") {
			row.set("includeMin", false)
		}

		if kid.yes("IncludeMaximum") {
			row.set("includeMax", true)
		}

		if kid.yes("OnlyDetect") {
			row.set("onlyDetect", true)
		}

		row.set("property", c.value(kid, "Property"))

		for _, a := range []string{"Language", "ExcludeLanguages", "RemoveFeatures", "MigrateFeatures", "IgnoreRemoveFailure"} {
			if kid.has(a) {
				c.todof(row, "", kid.line, "UpgradeVersion %s has no counterpart in upgrade.related", a)
			}
		}

		upgrade.add("related", row)
	}
}

// allText is every attribute value and text of the file, for checks that
// look for a name anywhere.
func (c *converter) allText() string {
	var text strings.Builder

	var walk func(*elem)

	walk = func(elm *elem) {
		for _, v := range elm.attrs {
			text.WriteString(v)
			text.WriteByte('\n')
		}

		text.WriteString(elm.text)

		for _, k := range elm.kids {
			walk(k)
		}
	}

	walk(c.doc.root)

	return text.String()
}

// skippedProperties are properties that other keys express.
func skippedProperty(id string) bool {
	if _, ok := arpKeys[id]; ok {
		return true
	}

	switch id {
	case "ALLUSERS", "WixShellExecTarget", "QtExecCmdLine", "QtExec64CmdLine", "WixQuietExecCmdLine", "WixQuietExec64CmdLine", "WIXUI_INSTALLDIR",
		"WIXUI_EXITDIALOGOPTIONALCHECKBOX", "WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT", "WIXUI_EXITDIALOGOPTIONALTEXT":
		return true
	}

	return false
}

func (c *converter) emitProperties() {
	props := c.out.child("properties")

	for _, elm := range c.properties {
		propID := elm.attr("Id")
		if skippedProperty(propID) {
			continue
		}

		propOut := newMap()
		propOut.keep = true
		propOut.set("value", c.value(elm, "Value"))

		if elm.yes("Hidden") {
			propOut.set("hidden", true)
		}

		c.emitSearches(propOut, elm)

		var value any = propOut
		if len(propOut.keys) == 1 && propOut.keys[0] == "value" {
			value = propOut.get("value")
		}

		props.set(propID, value)

		if lines := comments(elm); len(lines) > 0 {
			props.comment(propID, lines...)
		}

		if comment := propOut.head; len(comment) > 0 {
			props.comment(propID, comment...)

			propOut.head = nil
		}
	}
}

func (c *converter) emitSearches(propOut *omap, prop *elem) {
	var searches []any

	for _, k := range prop.kids {
		if s := c.search(propOut, k); s != nil {
			searches = append(searches, s)
		}
	}

	switch len(searches) {
	case 0:
	case 1:
		propOut.set("search", searches[0])
	default:
		propOut.set("search", searches)
	}
}

// search converts a search element, with a nested search as the inner
// search that is relative to it.
func (c *converter) search(propOut *omap, elm *elem) *omap {
	kind, found := c.searchKind(propOut, elm)
	if found == nil {
		return nil
	}

	outer := newMap().set(kind, found)

	for _, kid := range elm.kids {
		if inner := c.search(propOut, kid); inner != nil {
			// The nested search looks below the folder this one finds.
			attachUnder(inner, outer)

			return inner
		}
	}

	return outer
}

// searchKind converts the attributes of one search element.
func (c *converter) searchKind(propOut *omap, elm *elem) (string, *omap) {
	found := newMap()

	switch {
	case elm.is("RegistrySearch"):
		c.registrySearch(propOut, elm, found)

		return "registry", found
	case elm.is("DirectorySearch"):
		found.set("path", c.value(elm, "Path"))

		if n, err := strconv.Atoi(elm.attr("Depth")); err == nil && n > 0 {
			found.set("depth", n)
		}

		return "directory", found
	case elm.is("FileSearch"):
		c.fileSearch(elm, found)

		return "file", found
	case elm.is("ComponentSearch"):
		found.set("guid", c.value(elm, "Guid"))

		return "component", found
	}

	c.todof(propOut, "", elm.line, "<%s> has no counterpart in properties.search", elm.qname())

	return "", nil
}

// attachUnder sets outer as the under search of the outermost search in
// the chain that inner starts.
func attachUnder(inner, outer *omap) {
	for {
		var found *omap

		for _, key := range inner.keys {
			if m := inner.mapAt(key); m != nil {
				found = m
			}
		}

		if found == nil {
			return
		}

		next := found.mapAt("under")
		if next == nil {
			found.set("under", outer)

			return
		}

		inner = next
	}
}

func (c *converter) registrySearch(propOut *omap, elm *elem, found *omap) {
	root := elm.attr("Root")
	if root == "HKMU" {
		c.todof(propOut, "", elm.line, "a registry search cannot read HKMU; pick HKLM or HKCU")
	}

	found.set("root", root)
	found.set("key", c.value(elm, "Key"))
	found.set("name", c.value(elm, "Name"))

	switch {
	case elm.attr("Bitness") == "always64", elm.yes("Win64"):
		found.set("bitness", "64")
	case elm.attr("Bitness") == "always32", elm.no("Win64"):
		found.set("bitness", "32")
	}

	switch elm.attr("Type") {
	case "directory":
		found.set("type", "directory")
	case "file":
		c.todof(propOut, "", elm.line, "RegistrySearch Type=\"file\" has no counterpart; search the folder and add a file search under it")
	}
}

func (c *converter) fileSearch(elm *elem, found *omap) {
	found.set("name", c.value(elm, "Name"))

	for _, attr := range []string{"MinVersion", "MaxVersion", "MinDate", "MaxDate"} {
		found.set(lowerFirst(attr), c.value(elm, attr))
	}

	for _, attr := range []string{"MinSize", "MaxSize"} {
		if n, err := strconv.Atoi(elm.attr(attr)); err == nil {
			found.set(lowerFirst(attr), n)
		}
	}
}

func (c *converter) emitLaunchConditions() {
	for _, elm := range c.launches {
		cond := c.value(elm, "Condition")
		if elm.is("Condition") { // v3: the condition is the text
			cond = c.expand(elm, strings.TrimSpace(elm.text))
		}

		c.out.add("launchConditions", newMap().set("condition", cond).set("message", c.value(elm, "Message")))
	}
}

// emitFiles writes icons and binaries.
func (c *converter) emitFiles() {
	for _, e := range c.icons {
		c.out.child("icons").set(e.attr("Id"), sourcePath(c.value(e, "SourceFile")))
	}

	for _, elm := range c.binaries {
		src := elm.attr("SourceFile")
		if src == "" {
			src = elm.attr("src")
		}

		c.out.child("binaries").set(elm.attr("Id"), sourcePath(c.expand(elm, src)))
	}
}

// comments returns an element's XML comments as comment lines.
func comments(e *elem) []string {
	var lines []string

	for _, text := range e.comments {
		for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}

	return lines
}

// lowerFirst turns a WiX attribute name into the msi-builder key.
func lowerFirst(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}
