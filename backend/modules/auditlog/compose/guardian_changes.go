package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewGuardianChangeLog composes the guardian-change append command. It holds
// no database handle: every append runs in the caller's ambient tenant
// transaction and is refused without one.
func NewGuardianChangeLog() auditlog.GuardianChangeLog {
	return postgres.NewGuardianChanges(postgres.NewGuardianTransaction(tenant.TenantFromContext, tenant.TransactionFromContext))
}
