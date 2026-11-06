package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTraegerGroupNames composes the group names of the Träger's occupancy
// (#4676, ADR 0154) on the caller's administrative transaction.
func NewTraegerGroupNames() schoolstructure.TraegerGroupNames {
	return postgres.NewTraegerGroupNames(tenant.TransactionFromContext, tenant.IsAdminTx)
}
