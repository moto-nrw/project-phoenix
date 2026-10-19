package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TenantRuntimeRunInTxBinding(ctx context.Context, fn func(context.Context) error, execute0 func(context.Context) context.Context, execute1 func(ctx context.Context, fn func(context.Context) error) error, execute2 func(ctx context.Context, fn func(context.Context) error) error) error {
	ctx = execute0(ctx)
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	scope := tenant.ScopeFromContext(ctx)
	if tenant.FromContext(ctx) == 0 && (scope == "platform" || (scope == "org" && tenant.OrgFromContext(ctx) > 0)) {
		return execute1(ctx, fn)
	}
	return execute2(ctx, fn)
}
