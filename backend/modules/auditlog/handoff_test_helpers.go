//go:build testhooks

package auditlog

import (
	"context"

	auditapp "github.com/moto-nrw/project-phoenix/modules/auditlog/internal/application"
)

// The readers below let tests observe the Audit handoff; production reads it
// in the Audit adapters themselves.

// TenantIDFromContext returns the tenant the Audit adapters read, or zero.
func TenantIDFromContext(ctx context.Context) int64 {
	return auditapp.TenantIDFromContext(ctx)
}

// TransactionFromContext returns the transaction the Audit adapters read.
func TransactionFromContext(ctx context.Context) (any, bool) {
	return auditapp.TransactionFromContext(ctx)
}
