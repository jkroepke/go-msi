package builder

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jkroepke/go-msi/condition"
	"github.com/jkroepke/go-msi/config"
)

// writeProperties fills the Property table from the product identity, the ARP
// settings and the properties section. SecureCustomProperties and
// MsiHiddenProperties are written at the end, once every contributor has
// run (finishProperties).
func (b *build) writeProperties() error {
	p := b.cfg.Product

	b.properties["ProductCode"] = b.productCode
	b.properties["ProductName"] = p.Name
	b.properties["ProductVersion"] = p.Version
	b.properties["ProductLanguage"] = strconv.Itoa(*p.Language)
	b.properties["Manufacturer"] = p.Manufacturer
	b.properties["UpgradeCode"] = b.upgradeCode

	if b.cfg.Package.Scope == config.ScopePerMachine {
		b.properties["ALLUSERS"] = "1"
	}

	arp := p.ARP
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
		b.schedule("InstallExecuteSequence", "SetARPINSTALLLOCATION", "", "", "CostFinalize", 0)
		b.schedule("InstallUISequence", "SetARPINSTALLLOCATION", "", "", "CostFinalize", 0)
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
func (b *build) upgrade() error {
	u := b.cfg.Upgrade
	if u == nil {
		return nil
	}

	t := b.table("Upgrade")
	v := b.cfg.Product.Version

	// Older (and, with allowSameVersion, equal) versions are removed.
	attrs := 0
	if *u.MigrateFeatures {
		attrs |= upgradeMigrateFeatures
	}

	if *u.AllowSameVersion {
		attrs |= upgradeVersionMaxInclusive
	}

	t.Add(b.upgradeCode, nil, v, nil, attrs, nil, u.Property)
	b.secure[u.Property] = true

	if u.Downgrade != "allow" {
		t.Add(b.upgradeCode, v, nil, nil, upgradeOnlyDetect, nil, u.NewerProperty)
		b.secure[u.NewerProperty] = true

		if u.Downgrade == "error" {
			b.table("LaunchCondition").Add("NOT "+u.NewerProperty, u.DowngradeMessage)
		}
	}

	for i, r := range u.Related {
		code, err := config.NormalizeGUID(r.UpgradeCode)
		if err != nil {
			return fmt.Errorf("upgrade.related[%d]: %w", i, err)
		}

		a := 0

		if r.IncludeMin != nil && *r.IncludeMin {
			a |= upgradeVersionMinInclusive
		}

		if r.IncludeMax {
			a |= upgradeVersionMaxInclusive
		}

		if r.OnlyDetect {
			a |= upgradeOnlyDetect
		} else if *u.MigrateFeatures {
			a |= upgradeMigrateFeatures
		}

		t.Add(code, nullable(r.MinVersion), nullable(r.MaxVersion), nil, a, nil, r.Property)
		b.secure[r.Property] = true
	}

	for _, seq := range []string{"InstallExecuteSequence", "InstallUISequence"} {
		b.standardAt(seq, "FindRelatedProducts")

		if *u.MigrateFeatures {
			b.standardAt(seq, "MigrateFeatureStates")
		}
	}

	switch u.RemoveOld {
	case "afterInstallValidate":
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", "InstallValidate", 0)
	case "afterInstallExecute":
		b.standardAt("InstallExecuteSequence", "InstallExecute")
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", "InstallExecute", 0)
	case "afterInstallFinalize":
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", "InstallFinalize", 0)
	default:
		b.schedule("InstallExecuteSequence", "RemoveExistingProducts", "", "", "InstallInitialize", 0)
	}

	return nil
}

// launchConditions writes LaunchCondition rows after parsing each condition,
// because the engine ignores one that does not parse (section 9.1).
func (b *build) launchConditions() error {
	t := b.table("LaunchCondition")

	for i, lc := range b.cfg.LaunchConditions {
		p := fmt.Sprintf("launchConditions[%d]", i)

		if err := condition.Check(lc.Condition); err != nil {
			b.errorf(p+".condition", "%v; the engine ignores a launch condition that does not parse", err)

			continue
		}

		t.Add(lc.Condition, lc.Message)
	}

	return nil
}
