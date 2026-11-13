package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewSickStudents composes the sick marks of the Träger's school view (#5686,
// ADR 0151) on the caller's administrative transaction.
func NewSickStudents() careplan.SickStudents {
	return postgres.NewSickStudents(tenant.TransactionFromContext, tenant.IsAdminTx)
}
