package tenant

import (
	"context"
	"errors"
)

// TransactionRunner is the application-facing UnitOfWork facade for commands
// scoped to the tenant already present in context.
type TransactionRunner struct{}

func NewTransactionRunner() *TransactionRunner { return &TransactionRunner{} }

func (*TransactionRunner) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	return WithinCurrentTenant(ctx, fn)
}

func (*TransactionRunner) RunInTxWithRetry(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	id, err := TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return WithinTenantRetry(ctx, id, fn)
}

// ErrTransactionRequired reports a lock requested outside its caller's unit of work.
var ErrTransactionRequired = errors.New("tenant: transaction is required")

// RequireTransaction checks the school and ambient transaction without opening
// a unit of work. A lock must remain held until its caller commits.
func RequireTransaction(ctx context.Context) error {
	if !IsAdminTx(ctx) {
		if _, err := TenantFromContext(ctx); err != nil {
			return err
		}
	}
	if _, ok := TransactionFromContext(ctx); !ok {
		return ErrTransactionRequired
	}
	return nil
}

func (*TransactionRunner) RequireTransaction(ctx context.Context) error {
	return RequireTransaction(ctx)
}
