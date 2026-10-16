package compose

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type tenantTransactions struct{ db *bun.DB }

func (t tenantTransactions) InTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, t.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

// NewTenantTransactions binds the tenant transactions to the database; a nil
// database gives nil, which the services read as "not configured".
func NewTenantTransactions(db *bun.DB) ports.TenantTransactions {
	if db == nil {
		return nil
	}
	return tenantTransactions{db: db}
}

type tenantDatabase struct {
	tenantTransactions
	*postgres.Store
}

// NewTenantDatabase binds the tenant transactions and the advisory locks to
// the database; a nil database gives nil.
func NewTenantDatabase(db *bun.DB) ports.TenantDatabase {
	if db == nil {
		return nil
	}
	return tenantDatabase{tenantTransactions: tenantTransactions{db: db}, Store: postgres.New(databaseRuntime(db))}
}
