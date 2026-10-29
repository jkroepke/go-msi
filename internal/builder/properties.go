package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/msi-builder/internal/condition"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// writeProperties fills the Property table from the product identity, the ARP
// settings and the properties section. SecureCustomProperties and
// MsiHiddenProperties are written at the end, once every contributor has
// run (finishProperties).
func (b *build) writeProperties() error { //nolint:cyclop // one pass over the format; splitting would scatter it
	product := b.cfg.Product

	b.properties["ProductCode"] = b.productCode
	b.properties["ProductName"] = product.Name
	b.properties["ProductVersion"] = product.Version
	b.properties["ProductLanguage"] = strconv.Itoa(b.language)
	b.properties["Manufacturer"] = product.Manufacturer
	b.properties["UpgradeCode"] = b.upgradeCode

	if b.cfg.Package.Scope == config.ScopePerMachine {
		b.properties["ALLUSERS"] = "1"
	}

	arp := product.ARP
	set := func(name, value string) {
		if value != "" {
			b.properties[name] = value
		}
	}

	set("ARPCOMMENTS", arp.Comments)
	set("ARPCONTACT", arp.Contact)
	set("ARPHELPLINK", arp.HelpLink)
	set("ARPURLINFOABOUT", arp.AboutLink)
	set("ARPURLUPDATEINFO", arp.UpdateLink)
	set("ARPPRODUCTICON", arp.Icon)

	if arp.NoRepair {
		b.properties["ARPNOREPAIR"] = "1"
	}

	if arp.NoModify {
		b.properties["ARPNOMODIFY"] = "1"
	}

	if arp.NoRemove {
		b.properties["ARPNOREMOVE"] = "1"
	}

	if arp.Size > 0 {
		b.properties["ARPSIZE"] = strconv.Itoa(arp.Size)
	}

	if arp.InstallLocation != "" {
		// ARPINSTALLLOCATION takes the resolved directory after costing.
		b.addAction(&actionInfo{name: "SetARPINSTALLLOCATION", typ: caSetProperty, source: "ARPINSTALLLOCATION", target: "[" + arp.InstallLocation + "]"})
		b.schedule("InstallExecuteSequence", "SetARPINSTALLLOCATION", "", "", actCostFinalize, 0)
		b.schedule("InstallUISequence", "SetARPINSTALLLOCATION", "", "", actCostFinalize, 0)
	}

	for _, name := range b.cfg.Properties.Names() {
		prop := b.cfg.Properties[name]

		// A searched property gets its default from an action after the
		// search, so that the guard can tell a command-line value from it.
		if prop.Value != "" && prop.Search == nil {
			b.properties[name] = prop.Value
		}

		public := config.IsPublicProperty(name)
		if (prop.Secure == nil && public) || (prop.Secure != nil && *prop.Secure) {
			b.secure[name] = true
		}

		if prop.Hidden {
			b.hidden[name] = true
		}
	}

	return nil
}

// finishProperties writes the Property table rows.
func (b *build) finishProperties() {
	if len(b.secure) > 0 {
		b.properties["SecureCustomProperties"] = strings.Join(sortedKeys(b.secure), ";")
	}

	if len(b.hidden) > 0 {
		b.properties["MsiHiddenProperties"] = strings.Join(sortedKeys(b.hidden), ";")
	}

	t := b.table("Property")

	for _, name := range sortedStringKeys(b.properties) {
		t.Add(name, b.properties[name])
	}
}

// Upgrade attribute bits (section 8.10).
const (
	upgradeMigrateFeatures     = 1
	upgradeOnlyDetect          = 2
	upgradeIgnoreRemoveFailure = 4
	upgradeVersionMinInclusive = 256
	upgradeVersionMaxInclusive = 512
	upgradeLanguagesExclusive  = 1024
)

// upgrade writes the Upgrade rows and schedules the upgrade actions.
func (b *build) upgrade() error { //nolint:cyclop // one pass over the format; splitting would scatter it
	upg := b.cfg.Upgrade
	if upg == nil || upg.Disabled {
		return nil
	}

	tbl := b.table("Upgrade")
	version := b.cfg.Product.Version

	// Older (and, with allowSameVersion, equal) versions are removed.
	attrs := 0
	if *upg.MigrateFeatures {
		attrs |= upgradeMigrateFeatures
	}

	if *upg.AllowSameVersion {
		attrs |= upgradeVersionMaxInclusive
	}

	tbl.Add(b.upgradeCode, nil, version, nil, attrs, nil, upg.Property)
	b.secure[upg.Property] = true

	if upg.Downgrade != "allow" {
		tbl.Add(b.upgradeCode, version, nil, nil, upgradeOnlyDetect, nil, upg.NewerProperty)
		b.secure[upg.NewerProperty] = true

		if upg.Downgrade == "error" {
			b.table("LaunchCondition").Add("NOT "+upg.NewerProperty, upg.DowngradeMessage)
		}
	}

	for i, related := range upg.Related {
		code, err := config.NormalizeGUID(related.UpgradeCode)
		if err != nil {
			return fmt.Errorf("upgrade.related[%d]: %w", i, err)
		}

		attrs := 0

		if related.IncludeMin != nil && *related.IncludeMin {
			attrs |= upgradeVersionMinInclusive
		}

		if related.IncludeMax {
			attrs |= upgradeVersionMaxInclusive
		}

		if related.OnlyDetect {
			attrs |= upgradeOnlyDetect
		} else if *upg.MigrateFeatures {
			attrs |= upgradeMigrateFeatures
		}

		tbl.Add(code, nullable(related.MinVersion), nullable(related.MaxVersion), nil, attrs, nil, related.Property)
		b.secure[related.Property] = true
	}

	for _, seq := range []string{"InstallExecuteSequence", "InstallUISequence"} {
		b.standardAt(seq, "FindRelatedProducts")

		if *upg.MigrateFeatures {
			b.standardAt(seq, "MigrateFeatureStates")
		}
	}

	switch upg.RemoveOld {
	case "afterInstallValidate":
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", actInstallValidate, 0)
	case "afterInstallExecute":
		b.standardAt("InstallExecuteSequence", "InstallExecute")
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", "InstallExecute", 0)
	case "afterInstallFinalize":
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", actInstallFinalize, 0)
	default:
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", actInstallInitialize, 0)
	}

	return nil
}

// launchConditions writes LaunchCondition rows after parsing each condition,
// because the engine ignores one that does not parse (section 9.1).
func (b *build) launchConditions() error {
	tbl := b.table("LaunchCondition")

	for i, launch := range b.cfg.LaunchConditions {
		p := fmt.Sprintf("launchConditions[%d]", i)

		if err := condition.Check(launch.Condition); err != nil {
			b.errorf(p+".condition", "%v; the engine ignores a launch condition that does not parse", err)

			continue
		}

		tbl.Add(launch.Condition, launch.Message)
	}

	return nil
}

// Registry locator types (RegLocator.Type).
const (
	regLocatorDirectory = 0
	regLocatorRaw       = 2
	regLocator64Bit     = 16
)

// rememberInstallDir reads back the folder installDir.remember stored. The
// search fills a separate public property, and a type-51 action copies it
// into the install dir only when the command line did not set one: SEARCH-03
// showed that searching straight into the install dir overrides a command
// line value, and that AppSearch fails with 2228 without a Signature table,
// even for a registry search. Both run in the UI and the execute sequence,
// so that a full-UI install shows the remembered folder (not tested).
func (b *build) rememberInstallDir() error {
	install := b.cfg.InstallDir
	if !install.Remember {
		return nil
	}

	property := install.ID + "_REMEMBERED"

	root := 2
	if config.RememberRoot(b.cfg) == "HKCU" {
		root = 1
	}

	typ := regLocatorRaw
	if b.is64 && !remember32Bit(b.cfg) {
		typ |= regLocator64Bit
	}

	b.table("Signature")
	b.table("AppSearch").Add(property, config.RememberComponentID)
	b.table("RegLocator").Add(config.RememberComponentID, root, config.RememberKey(b.cfg), config.RememberValueName, typ)
	b.secure[property] = true

	const action = config.RememberActionID

	b.addAction(&actionInfo{name: action, typ: caSetProperty, source: install.ID, target: "[" + property + "]"})

	for _, seq := range []string{seqInstallExecute, seqInstallUI} {
		b.standardAt(seq, "AppSearch")
		b.schedule(seq, action, property+" AND NOT "+install.ID, "", "AppSearch", 0)
	}

	return nil
}

// searchProperties writes the AppSearch and locator rows of the properties
// with a search. Each search fills a property of its own, and type-51
// actions copy the first result, or the default value, into the property
// only when it is still empty, so that a command-line value wins (SEARCH-03
// showed that a search straight into the property overrides one). Both run
// in the UI and the execute sequence; after a UI sequence the engine skips
// AppSearch in the execute sequence (reported), so the property must be
// secure to arrive there.
func (b *build) searchProperties() error {
	for _, name := range b.cfg.Properties.Names() {
		if prop := b.cfg.Properties[name]; len(prop.Search) > 0 {
			b.searchProperty(name, prop)
		}
	}

	return nil
}

// searchProperty writes the searches of one property.
func (b *build) searchProperty(name string, prop *config.Property) {
	// AppSearch fails with 2228 without a Signature table (SEARCH-03).
	b.table("Signature")

	var previous string

	for i, search := range prop.Search {
		suffix := ""
		if i > 0 {
			suffix = strconv.Itoa(i + 1)
		}

		signature := searchName("Search"+suffix+"_", name)
		found := config.SearchFoundProperty(name, i)

		if prop.Hidden {
			b.hidden[found] = true
		}

		b.locate(signature, search, false)
		b.table("AppSearch").Add(found, signature)

		apply := searchName("SearchApply"+suffix+"_", name)
		b.addAction(&actionInfo{name: apply, typ: caSetProperty, source: name, target: "[" + found + "]"})

		after := previous
		if after == "" {
			after = "AppSearch"
		}

		for _, seq := range []string{seqInstallExecute, seqInstallUI} {
			b.standardAt(seq, "AppSearch")
			b.schedule(seq, apply, found+" AND NOT "+name, "", after, 0)
		}

		previous = apply
	}

	if prop.Value != "" {
		fallback := searchName("SearchDefault_", name)
		b.addAction(&actionInfo{name: fallback, typ: caSetProperty, source: name, target: escapeFormatted(prop.Value)})

		for _, seq := range []string{seqInstallExecute, seqInstallUI} {
			b.schedule(seq, fallback, "NOT "+name, "", previous, 0)
		}
	}
}

// Locator types (RegLocator.Type, CompLocator.Type).
const (
	locatorFile = 1
)

// locate writes the locator rows of one search under signature. A folder
// search (under another search) never has a Signature row, so the engine
// returns the folder.
func (b *build) locate(signature string, search *config.Search, folder bool) {
	switch {
	case search.Registry != nil:
		reg := search.Registry

		typ := regLocatorRaw
		if folder || reg.Type == config.SearchDirectory {
			typ = regLocatorDirectory
		}

		if reg.Bitness == "64" || reg.Bitness == "" && b.is64 {
			typ |= regLocator64Bit
		}

		b.table("RegLocator").Add(signature, registryRoots[reg.Root], reg.Key, nullable(reg.Name), typ)
	case search.Directory != nil:
		dir := search.Directory
		b.table("DrLocator").Add(signature, b.locateParent(signature, dir.Under), nullable(dir.Path), optionalDepth(dir.Depth))
	case search.File != nil:
		b.fileSignature(signature, search.File)
		b.table("DrLocator").Add(signature, b.locateParent(signature, search.File.Under), nullable(search.File.Path), optionalDepth(search.File.Depth))
	case search.Component != nil:
		guid, _ := config.NormalizeGUID(search.Component.GUID)
		b.table("CompLocator").Add(signature, guid, locatorFile)
	}
}

// locateParent writes the folder search a DrLocator row is relative to and
// returns its signature, or nil.
func (b *build) locateParent(signature string, under *config.Search) any {
	if under == nil {
		return nil
	}

	parent := searchName("In_", signature)
	b.locate(parent, under, true)

	return parent
}

// fileSignature writes the Signature row of a file search.
func (b *build) fileSignature(signature string, file *config.FileSearch) {
	var minDate, maxDate any

	if file.MinDate != "" {
		minDate, _ = config.SearchDate(file.MinDate, false)
	}

	if file.MaxDate != "" {
		maxDate, _ = config.SearchDate(file.MaxDate, true)
	}

	langs := make([]string, 0, len(file.Languages))
	for _, lang := range file.Languages {
		langs = append(langs, strconv.Itoa(lang))
	}

	minVersion := file.MinVersion
	if minVersion != "" {
		minVersion = config.SearchMinVersion(minVersion, file.Languages)
	}

	b.table("Signature").Add(signature, file.Name, nullable(minVersion), nullable(file.MaxVersion),
		optionalSize(file.MinSize), optionalSize(file.MaxSize), minDate, maxDate, nullable(strings.Join(langs, ",")))
}

// optionalDepth is a DrLocator.Depth cell: null for 0, which the engine
// reads as 0 (SEARCH-05).
func optionalDepth(depth int) any {
	if depth == 0 {
		return nil
	}

	return depth
}

// optionalSize is a nullable DoubleInteger cell.
func optionalSize(size *int64) any {
	if size == nil {
		return nil
	}

	return int(*size)
}

// searchName joins a prefix and a property name into an identifier of at
// most 72 characters.
func searchName(prefix, name string) string {
	if len(prefix)+len(name) <= config.MaxIdentifierLength {
		return prefix + name
	}

	return generatedID(strings.TrimSuffix(prefix, "_"), name)
}

// escapeFormatted makes text a literal in a Formatted column: [ ] { } become
// [\x], the escape the formatted-text grammar defines (section 8.9).
func escapeFormatted(text string) string {
	var out strings.Builder

	for _, char := range text {
		switch char {
		case '[', ']', '{', '}':
			out.WriteString(`[\`)
			out.WriteRune(char)
			out.WriteByte(']')
		default:
			out.WriteRune(char)
		}
	}

	return out.String()
}

// remember32Bit reports whether the component installDir.remember adds is a
// 32-bit one, which writes to the 32-bit registry view.
func remember32Bit(cfg *config.Config) bool {
	for _, comp := range cfg.Components {
		if comp.ID == config.RememberComponentID {
			return comp.Bitness == "32"
		}
	}

	return false
}
