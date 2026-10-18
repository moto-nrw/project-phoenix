package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantRuntime is the tenant runtime the calendar portal and its
// appointment mail renderer run in: the tenant of the context, the
// appointment write that joins the request transaction, the tenant and
// administrative transactions on the root database, commit hooks and the
// detached context of a fire-and-forget notification (#2747).
type TenantRuntime struct{ db *bun.DB }

// NewTenantRuntime binds the runtime to the root database.
func NewTenantRuntime(db *bun.DB) *TenantRuntime {
	return &TenantRuntime{db: db}
}

// TenantID returns the tenant of ctx, or zero.
func (r *TenantRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// WithinAppointmentWrite runs command in the request transaction when one is
// open, otherwise in the transaction of the context's tenant.
func (r *TenantRuntime) WithinAppointmentWrite(ctx context.Context, command func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return fmt.Errorf("calendar appointment command: %w", err)
	}
	return tenant.WithinTenant(ctx, tenantID, command)
}

// WithinTenant runs command in the tenant transaction of tenantID.
func (r *TenantRuntime) WithinTenant(ctx context.Context, tenantID int64, command func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, r.db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return command(txCtx) })
}

// WithinAdmin runs command in the administrative transaction.
func (r *TenantRuntime) WithinAdmin(ctx context.Context, command func(context.Context) error) error {
	return tenant.WithAdminTx(ctx, r.db, func(txCtx context.Context, _ bun.Tx) error { return command(txCtx) })
}

// WithoutTransactionAndHooks detaches ctx from the transaction and its
// commit hooks.
func (r *TenantRuntime) WithoutTransactionAndHooks(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
}
