package audit

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
)

// ContextWithTenant hands the tenant of a unit of work to the Audit adapters.
func ContextWithTenant(ctx context.Context, tenantID int64) context.Context {
	return auditModels.WithTenantID(ctx, tenantID)
}

// ContextWithTransaction hands the transaction of a unit of work to the Audit
// adapters; nil masks it.
func ContextWithTransaction(ctx context.Context, transaction any) context.Context {
	return auditModels.WithTransaction(ctx, transaction)
}
