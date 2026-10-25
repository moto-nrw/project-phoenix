package tenant

import (
	"context"
	"errors"
	"fmt"
)

// ErrTraegerRequired reports a Träger transaction without a positive Träger
// (organization) ID.
var ErrTraegerRequired = errors.New("tenant: Träger ID is required")

type traegerTxKey struct{}

// WithTraegerTransactions installs the adapter operation behind WithinTraeger.
// A runtime without it refuses Träger transactions instead of opening some
// other kind of unit.
func (uow UnitOfWork) WithTraegerTransactions(within func(context.Context, int64, func(context.Context, any) error) error) UnitOfWork {
	uow.withinTraeger = within
	return uow
}

// WithinTraeger runs fn in the Träger transaction of ADR 0086 section 5: the
// tenant role, no school, and app.current_org_id set to organizationID, so the
// RLS policies of the Träger's own tables admit exactly its rows. It never
// bypasses RLS and never selects a school. A nested call joins only an
// ambient Träger transaction of the same Träger.
func WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	if organizationID <= 0 {
		return ErrTraegerRequired
	}
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	if _, active := TransactionFromContext(ctx); active {
		if ambient := TraegerTxOrganization(ctx); ambient != organizationID {
			return fmt.Errorf("tenant: Träger transaction inside a foreign unit of work (Träger %d)", organizationID)
		}
		return fn(ctx)
	}
	runtime, err := unitOfWorkFromContext(ctx)
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkTransaction, Result: UnitOfWorkNotStarted, Err: err})
		return err
	}
	if runtime.withinTraeger == nil {
		return fmt.Errorf("%w: Träger transaction function is required", ErrRuntimeRequired)
	}
	ctx = ContextWithoutTenant(ctx)
	return runtime.execute(ctx, false, func(attemptCtx context.Context) error {
		return runtime.withinTraeger(attemptCtx, organizationID, func(txCtx context.Context, tx any) error {
			return runInTransaction(context.WithValue(txCtx, traegerTxKey{}, organizationID), tx, fn)
		})
	})
}

// TraegerTxOrganization returns the Träger of the ambient Träger transaction,
// or 0 outside one.
func TraegerTxOrganization(ctx context.Context) int64 {
	id, _ := ctx.Value(traegerTxKey{}).(int64)
	return id
}
