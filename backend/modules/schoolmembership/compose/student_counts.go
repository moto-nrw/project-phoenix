package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewActiveStudentCounts composes the cross-school active student count of
// the operator billing report (#2791). It reads on the caller's
// administrative transaction and needs nothing else of the owner.
func NewActiveStudentCounts() schoolmembership.ActiveStudentCounts {
	return postgres.NewActiveStudentCounts(tenant.TransactionFromContext, tenant.IsAdminTx)
}

// NewSchoolRosters composes the rosters of the Träger's school view (#5686,
// ADR 0151) on the caller's administrative transaction.
func NewSchoolRosters() schoolmembership.SchoolRosters {
	return postgres.NewSchoolRosters(tenant.TransactionFromContext, tenant.IsAdminTx)
}
