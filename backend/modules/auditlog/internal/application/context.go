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

// The readers below let modules/auditlog/audittest observe the handoff;
// production reads it in the Audit adapters themselves. They are values, not
// functions, so the dead-code check sees them referenced from production
// while modules/auditlog stays identical with and without testhooks (#5990).
var (
	// TenantIDFromContext returns the tenant the Audit adapters read, or zero.
	TenantIDFromContext = auditModels.TenantIDFromContext
	// TransactionFromContext returns the transaction the Audit adapters read.
	TransactionFromContext = auditModels.TransactionFromContext
)
