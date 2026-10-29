package builder

import (
	"fmt"

	"github.com/jkroepke/msi-builder/internal/sequence"
	"github.com/jkroepke/msi-builder/pkg/config"
)

// checkPerUserRollback warns when a deferred action that can fail a per-user
// install runs after RegisterProduct: a non-administrator's rollback cannot
// remove the registration (1403, access denied): the product stays
// registered and a reinstall fails, and a failure before PublishProduct
// leaves an entry only an administrator can remove (CA-01, specification
// section 8.2).
func (b *build) checkPerUserRollback() {
	if b.cfg.Package.Scope != config.ScopePerUser {
		return
	}

	seq := b.numbered[seqInstallExecute]

	register := sequence.Find(seq, "RegisterProduct")
	if register == 0 {
		return
	}

	for actionIndex, custom := range b.cfg.CustomActions {
		if custom.Execute != "deferred" || custom.Return == "ignore" {
			continue
		}

		for placementIndex, placement := range custom.Schedule {
			if table, ok := config.SequenceTable(placement.Sequence); !ok || table != seqInstallExecute {
				continue
			}

			if position := sequence.Find(seq, custom.ID); position > register {
				path := fmt.Sprintf("customActions[%d].schedule[%d]", actionIndex, placementIndex)
				b.warnf(path, "%s at %d runs after RegisterProduct (%d) in a per-user package; if it fails, "+
					"a non-administrator's rollback cannot remove the product registration (CA-01); "+
					"schedule it before RegisterProduct",
					custom.ID, position, register)
			}
		}
	}
}
