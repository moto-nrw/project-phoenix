package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func HasTenantTransaction(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}
func DetachCallerTransaction(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTenant(tenant.ContextWithoutTransaction(ctx)))
}
