//go:build testhooks

package audit

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
)

// The readers below let tests observe the Audit handoff; production reads it
// in the Audit adapters themselves.

// TenantIDFromContext returns the tenant the Audit adapters read, or zero.
func TenantIDFromContext(ctx context.Context) int64 {
	return auditModels.TenantIDFromContext(ctx)
}

// TransactionFromContext returns the transaction the Audit adapters read.
func TransactionFromContext(ctx context.Context) (any, bool) {
	return auditModels.TransactionFromContext(ctx)
}
