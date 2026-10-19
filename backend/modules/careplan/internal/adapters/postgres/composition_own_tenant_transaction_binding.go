package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func OwnTenantTransactionBinding(db *bun.DB, execute0 func(ctx context.Context, _ *bun.DB, rawID int64, fn func(context.Context, bun.Tx) error) error) ports.UnitOfWork {
	return func(ctx context.Context, command func(context.Context) error) error {
		return execute0(ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			return command(txCtx)
		})
	}
}
