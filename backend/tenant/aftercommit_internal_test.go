package tenant

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAfterCommitHooks_AddAndDrain locks down the queue contract used by
// WithTenantTx: callbacks accumulate in registration order and drain
// returns them all at once, leaving the queue empty for re-use.
func TestAfterCommitHooks_AddAndDrain(t *testing.T) {
	h := &afterCommitHooks{}

	var calls []int
	h.add(func() { calls = append(calls, 1) })
	h.add(func() { calls = append(calls, 2) })
	h.add(func() { calls = append(calls, 3) })

	drained := h.drain()
	assert.Len(t, drained, 3, "drain must return all queued fns")

	for _, fn := range drained {
		fn()
	}
	assert.Equal(t, []int{1, 2, 3}, calls, "fns must run in registration order")

	// Drain again — must be empty.
	assert.Empty(t, h.drain(), "drain must reset the queue")
}

// TestAfterCommitHooks_AddIsConcurrencySafe covers the mutex on add — a
// service could legally fan out post-commit hooks from goroutines spawned
// inside a tx (unlikely but legal). Without the lock, slice growth would
// race.
func TestAfterCommitHooks_AddIsConcurrencySafe(t *testing.T) {
	h := &afterCommitHooks{}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.add(func() {})
		}()
	}
	wg.Wait()

	assert.Len(t, h.drain(), 50)
}

// TestWithAfterCommitHooks_AttachesFreshHolder covers the path that runs
// for the OUTERMOST WithTenantTx call: no holder in ctx → fresh one is
// installed and threaded through context.
func TestWithAfterCommitHooks_AttachesFreshHolder(t *testing.T) {
	ctx := context.Background()
	newCtx, h := withAfterCommitHooks(ctx)

	assert.NotNil(t, h, "fresh holder must be returned")
	assert.NotEqual(t, ctx, newCtx, "ctx must be wrapped with the holder")

	got, ok := newCtx.Value(afterCommitKey{}).(*afterCommitHooks)
	assert.True(t, ok, "holder must be retrievable via afterCommitKey{}")
	assert.Same(t, h, got, "retrieved holder must be the same pointer")
}

// TestWithAfterCommitHooks_ReusesExistingHolder covers the nested-call
// path: a WithTenantTx already-in-progress reuses the holder so nested
// RegisterAfterCommit calls all queue onto the SAME drain.
func TestWithAfterCommitHooks_ReusesExistingHolder(t *testing.T) {
	ctx := context.Background()
	outerCtx, outerH := withAfterCommitHooks(ctx)

	innerCtx, innerH := withAfterCommitHooks(outerCtx)

	assert.Same(t, outerH, innerH, "nested call must reuse the existing holder")
	assert.Equal(t, outerCtx, innerCtx, "ctx must NOT be re-wrapped on nested call")
}

func TestContextWithoutAfterCommitHooksIsolatesIndependentTransactions(t *testing.T) {
	outerCtx, outerHooks := withAfterCommitHooks(context.Background())
	isolateCtx := ContextWithoutAfterCommitHooks(outerCtx)
	innerCtx, innerHooks := withAfterCommitHooks(isolateCtx)

	assert.NotSame(t, outerHooks, innerHooks)
	assert.NotEqual(t, outerCtx, innerCtx)
}

// TestRunAfterCommitHooks_NilHolderIsNoop covers the safety guard: if
// WithTenantTx never reached the outer-attach path (e.g. a defensive
// caller path), runAfterCommitHooks must not panic on nil.
func TestRunAfterCommitHooks_NilHolderIsNoop(t *testing.T) {
	assert.NotPanics(t, func() {
		runAfterCommitHooks(nil)
	})
}

// TestRunAfterCommitHooks_DrainsAndExecutes pins the drain-and-run order
// that the OUTERMOST WithTenantTx relies on after a successful COMMIT.
func TestRunAfterCommitHooks_DrainsAndExecutes(t *testing.T) {
	h := &afterCommitHooks{}
	var ran []int
	h.add(func() { ran = append(ran, 1) })
	h.add(func() { ran = append(ran, 2) })

	runAfterCommitHooks(h)

	assert.Equal(t, []int{1, 2}, ran)
	assert.Empty(t, h.drain(), "queue must be empty after run")
}

// TestRegisterAfterCommit_QueuesOntoHolderInContext covers the queued
// path (caller is inside a WithTenantTx) — fn must NOT execute inline,
// it must wait for the outer commit drain.
func TestRegisterAfterCommit_QueuesOntoHolderInContext(t *testing.T) {
	ctx, h := withAfterCommitHooks(context.Background())

	called := false
	RegisterAfterCommit(ctx, func() { called = true })

	assert.False(t, called, "fn must NOT run inline when a tx holder is present")
	assert.Len(t, h.drain(), 1, "fn must be queued onto the holder")
}
