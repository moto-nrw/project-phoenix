package tenant

import (
	"context"
	"sync"
)

// afterCommitKey threads one *afterCommitHooks through nested WithTenantTx
// calls so the outermost call drains the queue.
type afterCommitKey struct{}

type afterCommitHooks struct {
	mu  sync.Mutex
	fns []func()
}

func (h *afterCommitHooks) add(fn func()) {
	h.mu.Lock()
	h.fns = append(h.fns, fn)
	h.mu.Unlock()
}

func (h *afterCommitHooks) drain() []func() {
	h.mu.Lock()
	fns := h.fns
	h.fns = nil
	h.mu.Unlock()
	return fns
}

// RegisterAfterCommit queues fn to run after the surrounding tenant tx
// commits. On rollback, queued hooks are dropped. Outside WithTenantTx,
// fn runs synchronously. A panic in one hook stops subsequent hooks.
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

// ContextWithoutAfterCommitHooks masks hooks inherited from an ambient
// transaction. Use it with ContextWithoutTx before starting an independently
// committed transaction, so its callbacks cannot drain the caller's hooks.
func ContextWithoutAfterCommitHooks(ctx context.Context) context.Context {
	return context.WithValue(ctx, afterCommitKey{}, (*afterCommitHooks)(nil))
}

// withAfterCommitHooks reuses the holder if already attached (nested call),
// else attaches a fresh one.
func withAfterCommitHooks(ctx context.Context) (context.Context, *afterCommitHooks) {
	if h, ok := ctx.Value(afterCommitKey{}).(*afterCommitHooks); ok && h != nil {
		return ctx, h
	}
	h := &afterCommitHooks{}
	return context.WithValue(ctx, afterCommitKey{}, h), h
}

// runAfterCommitHooks is exposed for tests.
func runAfterCommitHooks(h *afterCommitHooks) {
	if h == nil {
		return
	}
	for _, fn := range h.drain() {
		fn()
	}
}
