package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewPresentStudents composes the current children of the Träger's school view
// (#5686, ADR 0151) on the caller's administrative transaction.
func NewPresentStudents() studentpresence.PresentStudents {
	return postgres.NewPresentStudents(tenant.TransactionFromContext, tenant.IsAdminTx)
}
