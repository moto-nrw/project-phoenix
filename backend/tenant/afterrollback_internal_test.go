package tenant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterAfterRollback_QueuesAndRunsOnRollback(t *testing.T) {
	ctx, hooks := withAfterRollbackHooks(context.Background())
	called := false

	RegisterAfterRollback(ctx, func() { called = true })

	assert.False(t, called, "rollback work must not run before the transaction outcome is known")
	runAfterRollbackHooks(hooks)
	assert.True(t, called)
	assert.Empty(t, hooks.drain(), "rollback hooks must be drained after execution")
}

func TestRegisterAfterRollback_NoTransactionIsNoop(t *testing.T) {
	called := false

	RegisterAfterRollback(context.Background(), func() { called = true })

	assert.False(t, called, "without a transaction there is no rollback to compensate")
}
