package builder

import (
	"fmt"

	"github.com/jkroepke/go-msi/internal/sequence"
	"github.com/jkroepke/go-msi/pkg/config"
)

// checkWhen warns when a schedule's when is placed where its properties are
// not set yet: the upgrade properties before FindRelatedProducts, and
// REMOVE, which is reliable only after InstallValidate (specification
// section 8.8).
func (b *build) checkWhen() {
	for actionIndex, custom := range b.cfg.CustomActions {
		for placementIndex, placement := range custom.Schedule {
			table, ok := config.SequenceTable(placement.Sequence)
			if placement.When == "" || !ok {
				continue
			}

			path := fmt.Sprintf("customActions[%d].schedule[%d].when", actionIndex, placementIndex)
			b.checkWhenPosition(path, b.numbered[table], custom.ID, placement.When)
		}
	}
}

// checkWhenPosition checks one placement against the action that sets the
// properties its when tests.
func (b *build) checkWhenPosition(path string, seq []sequence.Numbered, action, when string) {
	position := sequence.Find(seq, action)

	switch when {
	case config.WhenInstall, config.WhenUpgrade:
		if find := sequence.Find(seq, "FindRelatedProducts"); find != 0 && position < find {
			b.warnf(path, "%s at %d runs before FindRelatedProducts (%d), which sets the upgrade property it tests",
				action, position, find)
		}
	case config.WhenUninstall, config.WhenMaintenance:
		if validate := sequence.Find(seq, actInstallValidate); validate == 0 || position < validate {
			b.warnf(path, "%s at %d runs before InstallValidate, and REMOVE is reliable only after it", action, position)
		}
	}
}
