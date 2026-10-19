package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func PortalAppointmentWrite(ctx context.Context, command func(context.Context) error, execute func(context.Context, tenant.TenantID, func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return fmt.Errorf("calendar appointment command: %w", err)
	}
	return execute(ctx, id, command)
}
func PortalTenantWrite(ctx context.Context, db *bun.DB, id int64, command func(context.Context) error, execute func(context.Context, *bun.DB, int64, func(context.Context, bun.Tx) error) error) error {
	return execute(ctx, db, id, func(txCtx context.Context, _ bun.Tx) error { return command(txCtx) })
}
func PortalAdminWrite(ctx context.Context, db *bun.DB, command func(context.Context) error, execute func(context.Context, *bun.DB, func(context.Context, bun.Tx) error) error) error {
	return execute(ctx, db, func(txCtx context.Context, _ bun.Tx) error { return command(txCtx) })
}
func PortalDetachedContext(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
}
