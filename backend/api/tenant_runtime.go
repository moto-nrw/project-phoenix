package api

import (
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/services"
)

// BindTenantRuntime binds the PostgreSQL runtime as the unit of work the
// serving root, the demo sidecar and the CLI roots run tenant work under,
// with the Audit handoff attached to its tenant and transaction.
func BindTenantRuntime(postgres *database.PostgresUnitOfWork, retryable func(error) bool) (apiCommon.TenantRuntime, error) {
	withTenant, withTransaction := services.AuditTransactionContext()
	return apiCommon.BindTenantRuntime(postgres.WithinTenant, postgres.WithinAdmin, postgres, retryable, apiCommon.TenantRuntimeContext{
		WithTenant: withTenant, WithTransaction: withTransaction,
	})
}
