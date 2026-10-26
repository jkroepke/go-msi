package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/go-msi/internal/condition"
	"github.com/jkroepke/go-msi/pkg/config"
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
	b.properties["ProductLanguage"] = strconv.Itoa(*product.Language)
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

		if prop.Value != "" {
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
	regLocatorRaw   = 2
	regLocator64Bit = 16
)

// rememberInstallDir reads back the folder installDir.remember stored. The
// search fills a separate public property, and a type-51 action copies it
// into the install dir only when the command line did not set one: RUN-25
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
