//go:build testhooks

// Package audittest lets tests observe the Audit handoff. It is a leaf so the
// readers do not change how modules/auditlog, and the packages importing it,
// compile under testhooks (#5990). Production reads the handoff in the Audit
// adapters themselves.
package audittest

import (
	"context"

	auditapp "github.com/moto-ogs/project-phoenix/modules/auditlog/internal/application"
)

// TenantIDFromContext returns the tenant the Audit adapters read, or zero.
func TenantIDFromContext(ctx context.Context) int64 {
	return auditapp.TenantIDFromContext(ctx)
}

// TransactionFromContext returns the transaction the Audit adapters read.
func TransactionFromContext(ctx context.Context) (any, bool) {
	return auditapp.TransactionFromContext(ctx)
}
