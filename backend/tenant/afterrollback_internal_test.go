package tenant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterAfterRollback_QueuesAndRunsOnRollback(t *testing.T) {
	t.Parallel()

	ctx, hooks := withAfterRollbackHooks(context.Background())
	called := false

	RegisterAfterRollback(ctx, func() { called = true })

	assert.False(t, called, "rollback work must not run before the transaction outcome is known")
	runAfterRollbackHooks(hooks)
	assert.True(t, called)
	assert.Empty(t, hooks.drain(), "rollback hooks must be drained after execution")
}

func TestRegisterAfterRollback_NoTransactionIsNoop(t *testing.T) {
	t.Parallel()

	called := false

	RegisterAfterRollback(context.Background(), func() { called = true })

	assert.False(t, called, "without a transaction there is no rollback to compensate")
}

func TestRegisterAfterRollback_NilCallbackIsIgnored(t *testing.T) {
	t.Parallel()

	ctx, hooks := withAfterRollbackHooks(context.Background())

	RegisterAfterRollback(ctx, nil)

	assert.Empty(t, hooks.drain(), "a nil callback must never be queued")
	assert.NotPanics(t, func() { runAfterRollbackHooks(hooks) })
}

func TestWithAfterRollbackHooks_NestedCallsShareOneQueue(t *testing.T) {
	t.Parallel()

	outer, outerHooks := withAfterRollbackHooks(context.Background())
	inner, innerHooks := withAfterRollbackHooks(outer)

	assert.Same(t, outerHooks, innerHooks, "nested WithTenantTx calls must reuse the outermost queue")

	var order []string
	RegisterAfterRollback(inner, func() { order = append(order, "inner") })
	RegisterAfterRollback(outer, func() { order = append(order, "outer") })

	runAfterRollbackHooks(outerHooks)

	assert.Equal(t, []string{"inner", "outer"}, order, "hooks run in registration order")
}

func TestRunAfterRollbackHooks_NilQueueIsNoop(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() { runAfterRollbackHooks(nil) })
}
