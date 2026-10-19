package staffpostgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// resolve returns the caller's ambient transaction and school.
//
// It is deliberately a copy of the same resolution in parentstore and
// modules/communication/composition: all three are Communication compose
// seams, and one compose package may not import another. They stay in step
// because they resolve the same two context values; if that resolution ever
// grows a rule, it has to change in every copy.
//
// Missing transactions fail before SQL; roots and workflows supply the unit.
func resolve(ctx context.Context, db *bun.DB) (bun.IDB, int64, error) {
	if db == nil {
		return nil, 0, errors.New("communication staff store: database is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 && !tenant.IsAdminTx(ctx) {
		return nil, 0, tenant.ErrTenantRequired
	}
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("communication staffstore: transaction is required")
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx, tenantID, nil
	case *bun.Tx:
		if tx != nil {
			return tx, tenantID, nil
		}
	}
	// A transaction is in the context but is not one this adapter can run in.
	// Falling back to the plain pool would silently take the conversation
	// statements out of the caller's transaction — the append lock would be
	// released immediately and message order would stop matching commit order.
	// The legacy repositories panicked here; failing the call is the same
	// decision without taking the process down.
	return nil, 0, fmt.Errorf("communication staff store: unsupported transaction type %T", transaction)
}

func NewAmbientDatabase(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) { return resolve(ctx, db) }
}
