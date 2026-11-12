package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewTraegerCareBookings composes the booked care offerings of the Träger's
// occupancy (#4676, ADR 0154) on the caller's administrative transaction.
func NewTraegerCareBookings() careplan.TraegerCareBookings {
	return postgres.NewTraegerCareBookings(tenant.TransactionFromContext, tenant.IsAdminTx)
}
