package config_test

import (
	"testing"

	"github.com/jkroepke/msi-builder/pkg/config"
)

func TestPlacementFullCondition(t *testing.T) {
	t.Parallel()

	withUpgrade := &config.Config{Upgrade: &config.Upgrade{Property: "OLDFOUND"}}
	noUpgrade := &config.Config{Upgrade: &config.Upgrade{Disabled: true}}

	for _, tc := range []struct {
		name string
		cfg  *config.Config
		pl   config.Placement
		want string
	}{
		{"none", withUpgrade, config.Placement{Condition: "A"}, "A"},
		{"uninstall", withUpgrade, config.Placement{When: config.WhenUninstall}, `REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE`},
		{"uninstall and condition", withUpgrade, config.Placement{When: config.WhenUninstall, Condition: "A OR B"},
			`(REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE) AND (A OR B)`},
		{"install", withUpgrade, config.Placement{When: config.WhenInstall}, "NOT Installed AND NOT OLDFOUND"},
		{"install without upgrade", noUpgrade, config.Placement{When: config.WhenInstall}, "NOT Installed"},
		{"upgrade", withUpgrade, config.Placement{When: config.WhenUpgrade}, "NOT Installed AND OLDFOUND"},
		{"upgrade with related", &config.Config{Upgrade: &config.Upgrade{Property: "OLDFOUND", Related: []config.RelatedProduct{
			{Property: "RENAMEDFOUND"}, {Property: "DETECTED", OnlyDetect: true},
		}}}, config.Placement{When: config.WhenUpgrade}, "NOT Installed AND (OLDFOUND OR RENAMEDFOUND)"},
		{"maintenance", withUpgrade, config.Placement{When: config.WhenMaintenance}, `Installed AND NOT REMOVE~="ALL"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.pl.FullCondition(tc.cfg); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
