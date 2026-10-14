// Package compose builds the Scheduler Runtime's stores for the roots, so a
// root takes them through the owner's composition and not from its Postgres
// packages (ADR 0053).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/cleanupruns"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

// NewCleanupRunLedger builds the ledger of each school's last successful
// nightly cleanup run (#3950). It joins the tenant transaction of each call.
func NewCleanupRunLedger() scheduler.CleanupRunLedger {
	return cleanupruns.NewStore()
}
