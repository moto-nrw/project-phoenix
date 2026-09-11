package tenant

import "context"

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
