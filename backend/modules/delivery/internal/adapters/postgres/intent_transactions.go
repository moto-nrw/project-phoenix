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

// WithinTenant runs fn in the tenant transaction of tenantID.
func (t IntentTransactions) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {

	return tenant.WithTenantTx(ctx, t.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// WithinSystem renders explicitly school-less identity/operator mail. A missing
// school on a tenant step never selects this boundary.
func (t IntentTransactions) WithinSystem(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithAdminTx(ctx, t.db, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
}

func NewTenantTransactions(db *bun.DB) IntentTransactions { return IntentTransactions{db: db} }
