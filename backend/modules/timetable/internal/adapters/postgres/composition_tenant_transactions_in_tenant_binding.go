package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func TenantTransactionsInTenantBinding(ctx context.Context, tenantID int64, fn func(context.Context) error, execute0 func(ctx context.Context, _ *bun.DB, rawID int64, fn func(context.Context, bun.Tx) error) error, execute1 *bun.DB) error {
	return execute0(ctx, execute1, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}
