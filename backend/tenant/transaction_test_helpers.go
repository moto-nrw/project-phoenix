package tenant

import "context"

// WithTransactionForTest installs an adapter transaction for tests that need
// to exercise transaction-aware repository behavior without opening another
// UnitOfWork. Production code must obtain this context from WithinTenant or
// WithinAdmin.
func WithTransactionForTest(ctx context.Context, tx any) context.Context {
	return withTransaction(ctx, tx)
}
