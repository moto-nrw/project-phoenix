package base

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GetDB returns the transaction from context if available, otherwise the base DB.
// It checks for transactions stored via modelBase.ContextWithTx (the existing project pattern)
// and also handles the tx parameter from WithTenantTx/WithAdminTx's RunInTx callback.
//
// In Phase 1 this is a standalone function. In Phase 3, repositories will replace
// r.db.NewSelect() with GetDB(ctx, r.db).NewSelect() to participate in tenant transactions.
func GetDB(ctx context.Context, db bun.IDB) bun.IDB {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return tx
	}
	return db
}

// EnsureTenantID sets the entity's tenant_id from context if the entity implements
// TenantScoped and its tenant_id is currently zero. Call this in custom Create methods
// that bypass the base Repository.Create().
func EnsureTenantID(ctx context.Context, entity interface{}) {
	if ts, ok := entity.(modelBase.TenantScoped); ok && ts.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			ts.SetTenantID(tid)
		}
	}
}
