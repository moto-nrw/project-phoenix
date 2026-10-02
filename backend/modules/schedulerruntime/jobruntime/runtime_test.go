package jobruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func zeroOf[T any](T) (zero T) { return zero }

func passthroughRuntime(t *testing.T) Runtime {
	t.Helper()
	runtime, err := New(testpkg.PassthroughTenantRuntime())
	require.NoError(t, err)
	return runtime
}

type unitOfWorkEvent struct {
	kind, result string
	retries      int
}

func TestNewRejectsAnUncomposedRuntime(t *testing.T) {
	t.Parallel()

	_, err := New(zeroOf(testpkg.PassthroughTenantRuntime()))

	require.ErrorContains(t, err, "runtime is required")
}

func TestWithinTenantScopesTheSchoolAndReportsTheTransaction(t *testing.T) {
	t.Parallel()
	runtime := passthroughRuntime(t)
	var events []unitOfWorkEvent
	ctx := runtime.Attach(context.Background(), func(kind, result string, _ time.Duration, retries int) {
		events = append(events, unitOfWorkEvent{kind: kind, result: result, retries: retries})
	})

	var tenantID int64
	var inTransaction bool
	err := runtime.WithinTenant(ctx, 4711, func(txCtx context.Context) error {
		tenantID = testpkg.TenantIDFromContext(txCtx)
		_, inTransaction = testpkg.TransactionFromContext(txCtx)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, int64(4711), tenantID)
	assert.True(t, inTransaction)
	assert.Equal(t, []unitOfWorkEvent{{kind: "transaction", result: "commit"}}, events)
}

func TestWithinTenantRejectsAMissingSchoolBeforeTheCallback(t *testing.T) {
	t.Parallel()
	runtime := passthroughRuntime(t)
	ctx := runtime.Attach(context.Background(), nil)

	for _, within := range []func(context.Context, int64, func(context.Context) error) error{runtime.WithinTenant, runtime.WithinTenantRetry} {
		called := false
		err := within(ctx, 0, func(context.Context) error {
			called = true
			return nil
		})

		require.Error(t, err)
		assert.False(t, called)
	}
}

func TestWithinTenantRetryReplaysARetryableFailure(t *testing.T) {
	t.Parallel()
	retryable := errors.New("serialization failure")
	runtime, err := New(testpkg.ScriptedTenantRuntime(nil, func(err error) bool { return errors.Is(err, retryable) }))
	require.NoError(t, err)
	var events []unitOfWorkEvent
	ctx := runtime.Attach(context.Background(), func(kind, result string, _ time.Duration, retries int) {
		events = append(events, unitOfWorkEvent{kind: kind, result: result, retries: retries})
	})

	attempts := 0
	err = runtime.WithinTenantRetry(ctx, 4712, func(context.Context) error {
		attempts++
		if attempts == 1 {
			return retryable
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, []unitOfWorkEvent{{kind: "transaction", result: "commit", retries: 1}}, events)
}

func TestWithinAdminRunsWithoutASchool(t *testing.T) {
	t.Parallel()
	runtime := passthroughRuntime(t)
	ctx := runtime.Attach(testpkg.ContextForTenant(context.Background(), 4713), nil)

	var tenantID int64 = -1
	require.NoError(t, runtime.WithinAdmin(ctx, func(txCtx context.Context) error {
		tenantID = testpkg.TenantIDFromContext(txCtx)
		return nil
	}))

	assert.Zero(t, tenantID)
}

func TestCommitGuardDecidesTheCommit(t *testing.T) {
	t.Parallel()
	runtime := passthroughRuntime(t)
	fenced := errors.New("lease not held")
	ctx := runtime.WithCommitGuard(runtime.Attach(context.Background(), nil), func(context.Context) error { return fenced })

	committed := false
	err := runtime.WithinTenant(ctx, 4714, func(txCtx context.Context) error {
		runtime.RegisterAfterCommit(txCtx, func() { committed = true })
		return nil
	})

	require.ErrorIs(t, err, fenced)
	assert.False(t, committed, "a fenced transaction runs no after-commit hook")
}

func TestRegisterAfterCommitWaitsForTheCommit(t *testing.T) {
	t.Parallel()
	runtime := passthroughRuntime(t)
	ctx := runtime.Attach(context.Background(), nil)

	var order []string
	require.NoError(t, runtime.WithinTenant(ctx, 4715, func(txCtx context.Context) error {
		runtime.RegisterAfterCommit(txCtx, func() { order = append(order, "after commit") })
		order = append(order, "in transaction")
		return nil
	}))
	runtime.RegisterAfterCommit(context.Background(), func() { order = append(order, "outside") })

	assert.Equal(t, []string{"in transaction", "after commit", "outside"}, order)
}

func TestTransactionsNeedNoPriorAttach(t *testing.T) {
	t.Parallel()
	runtime := passthroughRuntime(t)

	called := 0
	require.NoError(t, runtime.WithinTenant(context.Background(), 4716, func(context.Context) error {
		called++
		return nil
	}))
	require.NoError(t, runtime.WithinAdmin(context.Background(), func(context.Context) error {
		called++
		return nil
	}))

	assert.Equal(t, 2, called)
}
