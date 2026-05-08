package tenant

import (
	"context"
	"sync"
)

// afterCommitKey is the context key used to thread an *afterCommitHooks
// holder through every nested WithTenantTx in a request. Sharing one holder
// across nesting levels lets the OUTERMOST WithTenantTx be the single drain
// point — that is the only call site that issues a real COMMIT.
type afterCommitKey struct{}

// afterCommitHooks accumulates callbacks to fire after the surrounding
// tenant transaction successfully commits. The mutex guards append/drain
// because hooks may be registered from goroutines spawned inside fn (rare,
// but the contract should hold).
type afterCommitHooks struct {
	mu  sync.Mutex
	fns []func()
}

func (h *afterCommitHooks) add(fn func()) {
	h.mu.Lock()
	h.fns = append(h.fns, fn)
	h.mu.Unlock()
}

// drain returns the registered hooks and clears the holder so a second
// outer-level call (test reuse, retry) does not double-fire prior hooks.
func (h *afterCommitHooks) drain() []func() {
	h.mu.Lock()
	fns := h.fns
	h.fns = nil
	h.mu.Unlock()
	return fns
}

// RegisterAfterCommit schedules fn to run after the surrounding tenant
// transaction successfully commits. Hooks fire in registration order.
//
// When ctx is inside WithTenantTx (directly or through TenantTxMiddleware),
// fn is queued and the OUTERMOST WithTenantTx call drains the queue after
// db.RunInTx returns nil. If RunInTx returns an error or the middleware
// rolls back because the handler emitted 5xx, queued hooks are dropped —
// the database state never moved, so destructive cleanup (file unlinks,
// cache invalidations, broadcasts) must not run either.
//
// When ctx is NOT inside WithTenantTx (test fixtures, code paths that
// haven't been routed through the middleware yet), fn runs synchronously
// before this function returns. Callers MUST be safe with both behaviours.
//
// A panic in one hook prevents subsequent hooks from running. Callers that
// need every hook to run regardless should recover at the hook boundary.
func RegisterAfterCommit(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	if h, ok := ctx.Value(afterCommitKey{}).(*afterCommitHooks); ok && h != nil {
		h.add(fn)
		return
	}
	fn()
}

// withAfterCommitHooks attaches a fresh hooks holder to ctx. Used by the
// outermost WithTenantTx so nested calls share the same holder via context.
func withAfterCommitHooks(ctx context.Context) (context.Context, *afterCommitHooks) {
	if h, ok := ctx.Value(afterCommitKey{}).(*afterCommitHooks); ok && h != nil {
		// Already nested under a holder — keep it (drain happens at the
		// real outermost call). Nothing to attach.
		return ctx, h
	}
	h := &afterCommitHooks{}
	return context.WithValue(ctx, afterCommitKey{}, h), h
}

// runAfterCommitHooks fires the queued hooks in order. Exposed for tests.
func runAfterCommitHooks(h *afterCommitHooks) {
	if h == nil {
		return
	}
	for _, fn := range h.drain() {
		fn()
	}
}
