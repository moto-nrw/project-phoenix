package tenant

import (
	"context"
	"errors"
	"fmt"
)

// The Träger transaction (ADR 0086 section 5, ADR 0105) is the unit of work
// for tables that belong to a Träger, not to one of its schools. The adapter
// runs it under the request role with app.current_org_id set, so the Träger
// policy of such a table admits only the rows of that Träger and every school
// table admits none. It is neither a school transaction nor the
// administrative one, and it never nests in either.

// ErrOrganizationRequired reports a Träger transaction without a Träger.
var ErrOrganizationRequired = errors.New("tenant: Träger organization ID is required")

type traegerTxKey struct{}

// WithTraegerTransaction installs the adapter function that opens a Träger
// transaction. A unit of work without it refuses WithinTraeger.
func (uow UnitOfWork) WithTraegerTransaction(within func(context.Context, int64, func(context.Context, any) error) error) UnitOfWork {
	uow.withinTraeger = within
	return uow
}

// TraegerTransactionFromContext returns the Träger of the active Träger
// transaction, or zero outside one.
func TraegerTransactionFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(traegerTxKey{}).(int64)
	return id
}

// WithinTraeger runs fn in the Träger transaction of organizationID. A call
// inside the Träger transaction of the same Träger joins it; a school or
// administrative transaction, a bound school or another Träger is refused.
func WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	if organizationID <= 0 {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkMissingTenant, Err: ErrOrganizationRequired})
		return ErrOrganizationRequired
	}
	if fn == nil {
		return errors.New("tenant: callback is required")
	}
	runtime, err := unitOfWorkFromContext(ctx)
	if err == nil && runtime.withinTraeger == nil {
		err = fmt.Errorf("%w: Träger transaction function is required", ErrRuntimeRequired)
	}
	if err != nil {
		observeUnitOfWork(ctx, UnitOfWorkEvent{Kind: UnitOfWorkTransaction, Result: UnitOfWorkNotStarted, Err: err})
		return err
	}
	if _, tenantErr := TenantFromContext(ctx); tenantErr == nil || IsAdminTx(ctx) {
		return errors.New("tenant: a Träger transaction cannot run inside a school or administrative unit")
	}
	if current := TraegerTransactionFromContext(ctx); current != 0 && current != organizationID {
		return fmt.Errorf("tenant: nested Träger transaction with mismatched Träger (%d vs %d)", current, organizationID)
	}
	return runtime.execute(ctx, false, func(attemptCtx context.Context) error {
		return runtime.withinTraeger(attemptCtx, organizationID, func(txCtx context.Context, tx any) error {
			return runInTransaction(context.WithValue(txCtx, traegerTxKey{}, organizationID), tx, fn)
		})
	})
}
