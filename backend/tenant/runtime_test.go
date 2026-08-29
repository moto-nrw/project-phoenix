package tenant_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unknownCommitError struct{ error }

func (unknownCommitError) CommitOutcomeUnknown() {}

type transactionStartError struct{ error }

func (transactionStartError) TransactionNotStarted() {}

func TestUnitOfWorkRetriesOnlyOutermostRetrySafeCommand(t *testing.T) {
	t.Parallel()
	retryErr := errors.New("serialization failure")
	attempts := 0
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			attempts++
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(err error) bool { return errors.Is(err, retryErr) },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), uow)

	err = tenant.WithinTenantRetry(ctx, id, func(context.Context) error {
		if attempts == 1 {
			return retryErr
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts, "the outer retry-safe command owns the replay")

	attempts = 0
	err = tenant.WithinTenant(ctx, id, func(outerCtx context.Context) error {
		innerErr := tenant.WithinTenantRetry(outerCtx, id, func(context.Context) error {
			return retryErr
		})
		require.ErrorIs(t, innerErr, retryErr)
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts, "one outer and one joined nested call run; the nested call never replays")
}

func TestUnitOfWorkDropsFailedAttemptHooksBeforeRetry(t *testing.T) {
	t.Parallel()
	retryErr := errors.New("deadlock")
	attempts := 0
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			attempts++
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(err error) bool { return errors.Is(err, retryErr) },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	var hooks []int

	err = tenant.WithinTenantRetry(ctx, id, func(txCtx context.Context) error {
		attempt := attempts
		tenant.RegisterAfterCommit(txCtx, func() { hooks = append(hooks, attempt) })
		if attempt == 1 {
			return retryErr
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []int{2}, hooks, "a rolled-back attempt must not leak its after-commit hooks")
}

func TestUnitOfWorkObservesRollbackDurationAndRetries(t *testing.T) {
	t.Parallel()
	retryErr := errors.New("deadlock")
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(err error) bool { return errors.Is(err, retryErr) },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var observed tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) { observed = event })

	err = tenant.WithinTenantRetry(ctx, id, func(context.Context) error { return retryErr })

	require.ErrorIs(t, err, retryErr)
	assert.Equal(t, tenant.UnitOfWorkTransaction, observed.Kind)
	assert.Equal(t, tenant.UnitOfWorkRolledBack, observed.Result)
	assert.Equal(t, 3, observed.Retries)
	assert.GreaterOrEqual(t, observed.Duration, time.Duration(0))
}

func TestUnitOfWorkDistinguishesUnstartedAndUnknownCommitOutcomes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		result tenant.UnitOfWorkResult
	}{
		{name: "transaction not started", err: transactionStartError{error: errors.New("begin failed")}, result: tenant.UnitOfWorkNotStarted},
		{name: "commit unknown", err: unknownCommitError{error: errors.New("connection lost")}, result: tenant.UnitOfWorkCommitUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uow, err := tenant.NewUnitOfWork(
				func(context.Context, int64, func(context.Context, any) error) error { return tt.err },
				func(context.Context, func(context.Context, any) error) error { return tt.err },
				func(context.Context, tenant.SavepointAction) error { return nil },
				func(error) bool { return false },
			)
			require.NoError(t, err)
			id, err := tenant.NewTenantID(42)
			require.NoError(t, err)
			var observed tenant.UnitOfWorkEvent
			ctx := tenant.WithUnitOfWork(context.Background(), uow)
			ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) { observed = event })

			err = tenant.WithinTenant(ctx, id, func(context.Context) error { return nil })

			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.result, observed.Result)
		})
	}
}

func TestUnitOfWorkObservesPanicAndRepanics(t *testing.T) {
	t.Parallel()
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var observed tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) { observed = event })

	assert.PanicsWithValue(t, "boom", func() {
		_ = tenant.WithinTenant(ctx, id, func(context.Context) error { panic("boom") })
	})

	assert.Equal(t, tenant.UnitOfWorkTransaction, observed.Kind)
	assert.Equal(t, tenant.UnitOfWorkPanicked, observed.Result)
}

func TestUnitOfWorkDoesNotReportRollbackWhenAfterCommitHookPanics(t *testing.T) {
	t.Parallel()
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var observed []tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) { observed = append(observed, event) })

	assert.PanicsWithValue(t, "hook boom", func() {
		_ = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error {
			tenant.RegisterAfterCommit(txCtx, func() { panic("hook boom") })
			return nil
		})
	})

	require.Len(t, observed, 1)
	assert.Equal(t, tenant.UnitOfWorkCommitted, observed[0].Result)
}

func TestUnitOfWorkObserverReceivesPoolAndLockWaits(t *testing.T) {
	t.Parallel()
	var observed []tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWorkObserver(context.Background(), func(event tenant.UnitOfWorkEvent) {
		observed = append(observed, event)
	})

	tenant.ObservePoolWait(ctx, 3*time.Millisecond)
	tenant.ObserveLockWait(ctx, 4*time.Millisecond)

	require.Len(t, observed, 2)
	assert.Equal(t, tenant.UnitOfWorkPoolWait, observed[0].Kind)
	assert.Equal(t, 3*time.Millisecond, observed[0].Duration)
	assert.Equal(t, tenant.UnitOfWorkLockWait, observed[1].Kind)
	assert.Equal(t, 4*time.Millisecond, observed[1].Duration)
}

func TestWithinTenantDrainsNestedAfterCommitHooksOnlyAfterOutermostSuccess(t *testing.T) {
	t.Parallel()
	runtime, err := tenant.NewRuntime(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), runtime)
	var calls []string

	err = tenant.WithinTenant(ctx, id, func(outerCtx context.Context) error {
		tenant.RegisterAfterCommit(outerCtx, func() { calls = append(calls, "outer") })
		require.NoError(t, tenant.WithinTenant(outerCtx, id, func(innerCtx context.Context) error {
			tenant.RegisterAfterCommit(innerCtx, func() { calls = append(calls, "inner") })
			return nil
		}))
		assert.Empty(t, calls, "nested success must not drain hooks before the outer transaction commits")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"outer", "inner"}, calls)
}

func TestWithinAdminDrainsNestedAfterCommitHooksOnlyAfterOutermostSuccess(t *testing.T) {
	t.Parallel()
	runtime, err := tenant.NewRuntime(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), runtime)
	var calls []string

	err = tenant.WithinAdmin(ctx, func(outerCtx context.Context) error {
		tenant.RegisterAfterCommit(outerCtx, func() { calls = append(calls, "outer") })
		require.NoError(t, tenant.WithinAdmin(outerCtx, func(innerCtx context.Context) error {
			tenant.RegisterAfterCommit(innerCtx, func() { calls = append(calls, "inner") })
			return nil
		}))
		assert.Empty(t, calls, "nested success must not drain hooks before the outer transaction commits")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"outer", "inner"}, calls)
}

func TestRuntimeObserverReceivesActualTransactionResult(t *testing.T) {
	t.Parallel()
	runtimeErr := assert.AnError
	runtime, err := tenant.NewRuntime(
		func(context.Context, int64, func(context.Context, any) error) error { return runtimeErr },
		func(context.Context, func(context.Context, any) error) error { return nil },
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var observed tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWork(context.Background(), runtime)
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) { observed = event })

	err = tenant.WithinTenant(ctx, id, func(context.Context) error {
		t.Fatal("callback must not run when transaction setup fails")
		return nil
	})

	require.ErrorIs(t, err, runtimeErr)
	assert.Equal(t, tenant.UnitOfWorkTransaction, observed.Kind)
	require.ErrorIs(t, observed.Err, runtimeErr)
}

func TestRuntimeObserverIgnoresHandledNestedError(t *testing.T) {
	t.Parallel()
	runtime, err := tenant.NewRuntime(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
	)
	require.NoError(t, err)
	id, err := tenant.NewTenantID(42)
	require.NoError(t, err)
	var observed []tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWork(context.Background(), runtime)
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) { observed = append(observed, event) })

	err = tenant.WithinTenant(ctx, id, func(outerCtx context.Context) error {
		innerErr := tenant.WithinTenant(outerCtx, id, func(context.Context) error { return assert.AnError })
		require.ErrorIs(t, innerErr, assert.AnError)
		return nil
	})

	require.NoError(t, err)
	require.Len(t, observed, 1)
	assert.Equal(t, tenant.UnitOfWorkTransaction, observed[0].Kind)
	assert.NoError(t, observed[0].Err)
}

func TestRuntimeObserverClassifiesMissingTenant(t *testing.T) {
	t.Parallel()
	var observed tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWorkObserver(context.Background(), func(event tenant.UnitOfWorkEvent) { observed = event })

	err := tenant.WithTenantTx(ctx, struct{}{}, 0, func(context.Context, struct{}) error { return nil })

	require.ErrorIs(t, err, tenant.ErrInvalidTenantID)
	assert.Equal(t, tenant.UnitOfWorkMissingTenant, observed.Kind)
	require.ErrorIs(t, observed.Err, tenant.ErrInvalidTenantID)
}

func TestObserveMissingTenantReportsEntryPointRejection(t *testing.T) {
	t.Parallel()
	var observed tenant.UnitOfWorkEvent
	ctx := tenant.WithUnitOfWorkObserver(context.Background(), func(event tenant.UnitOfWorkEvent) { observed = event })

	tenant.ObserveMissingTenant(ctx, tenant.ErrInvalidTenantID)

	assert.Equal(t, tenant.UnitOfWorkMissingTenant, observed.Kind)
	require.ErrorIs(t, observed.Err, tenant.ErrInvalidTenantID)
}
