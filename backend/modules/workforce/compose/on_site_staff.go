package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewOnSiteStaff composes the staff read of the Träger's school view (#5686,
// ADR 0151) on the caller's administrative transaction.
func NewOnSiteStaff() workforce.OnSiteStaff {
	return postgres.NewOnSiteStaff(tenant.TransactionFromContext, tenant.IsAdminTx)
}
