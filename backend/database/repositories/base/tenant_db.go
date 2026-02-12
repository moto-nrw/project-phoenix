package base

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
