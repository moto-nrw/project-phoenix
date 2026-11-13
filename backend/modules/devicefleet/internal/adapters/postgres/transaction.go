package postgres

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Transaction adapts the shared runtime to the owner's transaction port.
type Transaction struct{ db *bun.DB }

func NewTransaction(db *bun.DB) Transaction {
	return Transaction{db: db}
}

func (t Transaction) TenantID(ctx context.Context) (int64, error) {
	if tenant.IsAdminTx(ctx) {
		return 0, devicefleet.ErrTenantRequired
	}
	_, scope, err := DatabaseRuntime(t.db)(ctx)
	return scope.TenantID, err
}

func (t Transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return TransactionRunWriteBinding(ctx, callback, tenant.WithinTenant)
}

func (t Transaction) AdminScope(ctx context.Context, callback func(context.Context) error) error {
	return TransactionAdminScopeBinding(ctx, callback, tenant.WithAdminTx, t.db)
}

func (t Transaction) TenantScope(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	return TransactionTenantScopeBinding(ctx, tenantID, callback, tenant.WithTenantTx, t.db)
}
