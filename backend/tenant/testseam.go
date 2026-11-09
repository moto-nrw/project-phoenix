package tenant

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant/internal/testseam"
)

// The registration below is the only production link to tenant/tenanttest. It
// exports nothing: testseam is importable only inside the tenant subtree.
func init() {
	testseam.WithTransaction = withTransaction
	testseam.WithTransactionHooks = withTransactionHooks
	testseam.RegisterAfterRollback = registerAfterRollback
}

func withTransactionHooks(ctx context.Context) (context.Context, func(), func()) {
	newCtx, h := withAfterCommitHooks(ctx)
	return newCtx, func() { runAfterCommitHooks(h) }, func() { runAfterRollbackHooks(h) }
}

// Rollback registration is retained only for the transaction-runtime outcome
// tests. Its last production consumer, Export Transfer, now commits durable
// evidence before dispatch and cannot depend on rollback compensation. Outside
// a transaction there is nothing that can roll back, so fn is ignored.
func registerAfterRollback(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	if h, ok := ctx.Value(afterCommitKey{}).(*afterCommitHooks); ok && h != nil {
		h.addRollback(fn)
	}
}

func (h *afterCommitHooks) addRollback(fn func()) {
	h.mu.Lock()
	h.rollbackFns = append(h.rollbackFns, fn)
	h.mu.Unlock()
}
