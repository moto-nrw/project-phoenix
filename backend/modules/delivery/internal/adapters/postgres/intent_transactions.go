package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// TenantTransactions runs the provider steps of a claimed intent — the
// template render, the push authorization and the expired-subscription
// cleanup — in the tenant transaction of the intent's school, which the
// worker's own context does not carry (#2747).
type IntentTransactions struct{ db *bun.DB }

// NewTenantTransactions binds the provider steps to the root database.
func NewTenantTransactions(db *bun.DB) IntentTransactions {
	return IntentTransactions{db: db}
}

// WithinTenant runs fn in the tenant transaction of tenantID.
func (t IntentTransactions) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	if tenantID == 0 {
		return tenant.WithAdminTx(ctx, t.db, func(txCtx context.Context, _ bun.Tx) error {
			return fn(txCtx)
		})
	}
	return tenant.WithTenantTx(ctx, t.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}
