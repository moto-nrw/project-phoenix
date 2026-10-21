package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewStaffPersonMerge() schoolmembership.StaffPersonMerge {
	return postgres.New(postgres.NewContextDatabase(tenant.TenantFromContext, tenant.TransactionFromContext, tenant.IsAdminTx))
}
