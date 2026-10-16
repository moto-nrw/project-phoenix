package compose

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantTransactions runs a callback in the tenant transaction of tenantID.
// The template, materialization and series services take it instead of the
// database handle.
type TenantTransactions interface {
	InTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
}

type tenantTransactions struct{ db *bun.DB }

func (t tenantTransactions) InTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, t.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// NewTenantTransactions binds the tenant transactions to the database; a nil
// database gives nil, which the services read as "not configured".
func NewTenantTransactions(db *bun.DB) TenantTransactions {
	if db == nil {
		return nil
	}
	return tenantTransactions{db: db}
}
