package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func TransactionTenantScopeBinding(ctx context.Context, tenantID int64, callback func(context.Context) error, execute0 func(ctx context.Context, _ *bun.DB, rawID int64, fn func(context.Context, bun.Tx) error) error, execute1 *bun.DB) error {
	return execute0(ctx, execute1, tenantID, func(tenantCtx context.Context, _ bun.Tx) error {
		return callback(tenantCtx)
	})
}
