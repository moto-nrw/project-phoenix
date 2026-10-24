package auditlog

import (
	"context"

	auditapp "github.com/moto-nrw/project-phoenix/modules/auditlog/internal/application"
)

// The Audit handoff: the Audit adapters read the tenant and transaction of
// the unit of work they run in from the context, not from the tenant runtime.
// The transaction runtime installs the two writers as its context adapters
// (modules/transactionruntime/compose, #3970); detaching from a transaction
// hands the adapters nil, which masks it.

// ContextWithTenant hands the tenant of a unit of work to the Audit adapters;
// zero clears it.
func ContextWithTenant(ctx context.Context, tenantID int64) context.Context {
	return auditapp.ContextWithTenant(ctx, tenantID)
}

// ContextWithTransaction hands the transaction of a unit of work to the Audit
// adapters; nil masks it.
func ContextWithTransaction(ctx context.Context, transaction any) context.Context {
	return auditapp.ContextWithTransaction(ctx, transaction)
}
