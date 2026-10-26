package config

import "fmt"

// Placement.When values.
const (
	WhenInstall     = "install"
	WhenUpgrade     = "upgrade"
	WhenUninstall   = "uninstall"
	WhenMaintenance = "maintenance"
)

// whenUninstall is the condition of a real uninstall that RUN-26 tested:
// true when the product is removed, false on the removal of the old
// product during a major upgrade.
const whenUninstall = `REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE`

// WhenCondition returns the condition a schedule's when names, or "" for
// none. upgrade and install tell a major upgrade from a first install by the
// upgrade property that FindRelatedProducts sets.
func WhenCondition(cfg *Config, when string) string {
	upgradeProperty := DefaultUpgradeProperty
	if cfg.Upgrade != nil && cfg.Upgrade.Property != "" {
		upgradeProperty = cfg.Upgrade.Property
	}

	switch when {
	case WhenInstall:
		if !upgrades(cfg) {
			return "NOT Installed"
		}

		return "NOT Installed AND NOT " + upgradeProperty
	case WhenUpgrade:
		return "NOT Installed AND " + upgradeProperty
	case WhenUninstall:
		return whenUninstall
	case WhenMaintenance:
		return `Installed AND NOT REMOVE~="ALL"`
	default:
		return ""
	}
}

// FullCondition returns the placement's condition with its when ANDed on.
func (p *Placement) FullCondition(cfg *Config) string {
	when := WhenCondition(cfg, p.When)

	switch {
	case when == "":
		return p.Condition
	case p.Condition == "":
		return when
	default:
		return fmt.Sprintf("(%s) AND (%s)", when, p.Condition)
	}
}

// upgrades reports whether the package finds and removes older versions.
func upgrades(cfg *Config) bool {
	return cfg.Upgrade != nil && !cfg.Upgrade.Disabled
}

// placementWhen checks a schedule's when.
func (v *validator) placementWhen(placementPath string, place Placement, cfg *Config) {
	v.oneOf(placementPath+".when", place.When, "", WhenInstall, WhenUpgrade, WhenUninstall, WhenMaintenance)

	if place.When == WhenUpgrade && !upgrades(cfg) {
		v.errorf(placementPath+".when", "upgrade needs the upgrade section, which finds older versions")
	}
}
