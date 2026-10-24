package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func TenantUnitOfWorkRunInTenantTxBinding(ctx context.Context, fn func(context.Context) error, execute0 func(ctx context.Context, _ *bun.DB, rawID int64, fn func(context.Context, bun.Tx) error) error) error {
	return execute0(ctx, (*bun.DB)(nil), tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}
