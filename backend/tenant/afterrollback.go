package tenant

import (
	"context"
	"sync"
)

// afterRollbackKey threads rollback callbacks through nested WithTenantTx
// calls. Unlike after-commit work, these callbacks compensate side effects
// that happened before the database transaction became durable.
type afterRollbackKey struct{}

type afterRollbackHooks struct {
	mu  sync.Mutex
	fns []func()
}

func (h *afterRollbackHooks) add(fn func()) {
	h.mu.Lock()
	h.fns = append(h.fns, fn)
	h.mu.Unlock()
}

func (h *afterRollbackHooks) drain() []func() {
	h.mu.Lock()
	fns := h.fns
	h.fns = nil
	h.mu.Unlock()
	return fns
}

// RegisterAfterRollback queues fn for execution only when the surrounding
// tenant transaction rolls back or fails to commit. Outside WithTenantTx it
// is a no-op because there is no transaction failure to compensate.
func RegisterAfterRollback(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	if h, ok := ctx.Value(afterRollbackKey{}).(*afterRollbackHooks); ok && h != nil {
		h.add(fn)
	}
}

func withAfterRollbackHooks(ctx context.Context) (context.Context, *afterRollbackHooks) {
	if h, ok := ctx.Value(afterRollbackKey{}).(*afterRollbackHooks); ok && h != nil {
		return ctx, h
	}
	h := &afterRollbackHooks{}
	return context.WithValue(ctx, afterRollbackKey{}, h), h
}

func runAfterRollbackHooks(h *afterRollbackHooks) {
	if h == nil {
		return
	}
	for _, fn := range h.drain() {
		fn()
	}
}
