package compose

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// NewAuditRetention composes the setting protocol retention of the nightly
// cleanup (#4044). It runs on the tenant transaction of the school in
// context and refuses to run without one instead of falling back to a pool.
func NewAuditRetention() settings.AuditRetention {
	return retainedAuditRetention{Now: time.Now}
}
