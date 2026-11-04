package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewPresentStudentCounts composes the count of the Träger's school view
// (#5686, ADR 0151) on the caller's administrative transaction.
func NewPresentStudentCounts() studentpresence.PresentStudentCounts {
	return postgres.NewPresentStudentCounts(tenant.TransactionFromContext, tenant.IsAdminTx)
}
