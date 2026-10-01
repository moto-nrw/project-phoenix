package tenant

import "context"

type commitGuardKey struct{}

// WithCommitGuard makes every transaction opened under ctx call guard as its
// last statement before commit. A guard error rolls the transaction back.
// The Worker uses it to fence job writes with its lease term (#2726): a run
// that lost its lease cannot commit, whichever owner opened the transaction.
// Nested calls join the outer transaction and leave the guard to it.
func WithCommitGuard(ctx context.Context, guard func(context.Context) error) context.Context {
	return context.WithValue(ctx, commitGuardKey{}, guard)
}

// runInTransaction installs the adapter transaction for fn and, when fn
// opened it, asks the commit guard before the adapter commits.
func runInTransaction(txCtx context.Context, tx any, fn func(context.Context) error) error {
	_, nested := TransactionFromContext(txCtx)
	txCtx = withTransaction(txCtx, tx)
	if err := fn(txCtx); err != nil || nested {
		return err
	}
	if guard, ok := txCtx.Value(commitGuardKey{}).(func(context.Context) error); ok && guard != nil {
		return guard(txCtx)
	}
	return nil
}
