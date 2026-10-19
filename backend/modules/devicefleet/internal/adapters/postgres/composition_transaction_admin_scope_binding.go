package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func TransactionAdminScopeBinding(ctx context.Context, callback func(context.Context) error, execute0 func(ctx context.Context, _ *bun.DB, fn func(context.Context, bun.Tx) error) error, execute1 *bun.DB) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return execute0(ctx, execute1, func(adminCtx context.Context, _ bun.Tx) error {
		return callback(adminCtx)
	})
}
