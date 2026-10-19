package parentpostgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// resolve returns the caller's ambient transaction and school.
//
// It is deliberately a copy of the same resolution in
// modules/communication/composition: both packages are Communication compose
// seams, and one compose package may not import another. The two stay in step
// because they resolve the same two context values; if that resolution ever
// grows a rule, it has to change in both places.
//
// Missing transactions fail before SQL; roots and workflows supply the unit.
func resolve(ctx context.Context, db *bun.DB) (bun.IDB, int64, error) {
	if db == nil {
		return nil, 0, errors.New("communication parent store: database is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 && !tenant.IsAdminTx(ctx) {
		return nil, 0, tenant.ErrTenantRequired
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("communication parentstore: transaction is required")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx, tenantID, nil
	case *bun.Tx:
		if tx != nil {
			return tx, tenantID, nil
		}
	}
	return nil, 0, errors.New("communication parentstore: transaction is required")
}

func NewParentRecordDatabase(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, TenantScope, error) {
		connection, schoolID, err := resolve(ctx, db)
		scope := ScopeFor(schoolID, tenant.IsAdminTx(ctx))
		if err != nil {
			return nil, scope, err
		}
		return connection, scope, scope.Validate()
	}
}
func NewProjectionDatabase(db *bun.DB) func(context.Context) (bun.IDB, int64, error) {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}
