package tenant

import "context"

// WithAfterCommitHooksForTest exposes the otherwise-package-private hooks
// holder so other packages can unit-test handlers that call
// RegisterAfterCommit without spinning up a real bun.DB. The returned
// drain runs the queued hooks the same way the outermost WithTenantTx
// does on a successful COMMIT.
//
// Test-only — production paths must go through WithTenantTx.
func WithAfterCommitHooksForTest(ctx context.Context) (context.Context, func()) {
	newCtx, h := withAfterCommitHooks(ctx)
	return newCtx, func() { runAfterCommitHooks(h) }
}

// WithTransactionHooksForTest exposes both transaction outcomes to tests of
// components that must compensate an external side effect on rollback.
func WithTransactionHooksForTest(ctx context.Context) (context.Context, func(), func()) {
	newCtx, h := withAfterCommitHooks(ctx)
	return newCtx, func() { runAfterCommitHooks(h) }, func() { runAfterRollbackHooks(h) }
}
