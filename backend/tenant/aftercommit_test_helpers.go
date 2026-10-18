//go:build testhooks

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

// Rollback registration is retained only for the transaction-runtime outcome
// tests. Its last production consumer, Export Transfer, now commits durable
// evidence before dispatch and cannot depend on rollback compensation.
func (h *afterCommitHooks) addRollback(fn func()) {
	h.mu.Lock()
	h.rollbackFns = append(h.rollbackFns, fn)
	h.mu.Unlock()
}

// RegisterAfterRollback queues fn to run if the surrounding tenant transaction
// does not commit. Outside a transaction there is nothing that can roll back,
// so the callback is ignored.
func RegisterAfterRollback(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	if h, ok := ctx.Value(afterCommitKey{}).(*afterCommitHooks); ok && h != nil {
		h.addRollback(fn)
	}
}
