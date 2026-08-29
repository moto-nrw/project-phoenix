package tenant_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	ctx := tenant.WithRuntime(context.Background(), runtime)
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
	ctx := tenant.WithRuntime(context.Background(), runtime)
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
	var observed tenant.RuntimeEvent
	ctx := tenant.WithRuntime(context.Background(), runtime)
	ctx = tenant.WithRuntimeObserver(ctx, func(event tenant.RuntimeEvent) { observed = event })

	err = tenant.WithinTenant(ctx, id, func(context.Context) error {
		t.Fatal("callback must not run when transaction setup fails")
		return nil
	})

	require.ErrorIs(t, err, runtimeErr)
	assert.Equal(t, tenant.RuntimeTransaction, observed.Outcome)
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
	var observed []tenant.RuntimeEvent
	ctx := tenant.WithRuntime(context.Background(), runtime)
	ctx = tenant.WithRuntimeObserver(ctx, func(event tenant.RuntimeEvent) { observed = append(observed, event) })

	err = tenant.WithinTenant(ctx, id, func(outerCtx context.Context) error {
		innerErr := tenant.WithinTenant(outerCtx, id, func(context.Context) error { return assert.AnError })
		require.ErrorIs(t, innerErr, assert.AnError)
		return nil
	})

	require.NoError(t, err)
	require.Len(t, observed, 1)
	assert.Equal(t, tenant.RuntimeTransaction, observed[0].Outcome)
	assert.NoError(t, observed[0].Err)
}

func TestRuntimeObserverClassifiesMissingTenant(t *testing.T) {
	t.Parallel()
	var observed tenant.RuntimeEvent
	ctx := tenant.WithRuntimeObserver(context.Background(), func(event tenant.RuntimeEvent) { observed = event })

	err := tenant.WithTenantTx(ctx, struct{}{}, 0, func(context.Context, struct{}) error { return nil })

	require.ErrorIs(t, err, tenant.ErrInvalidTenantID)
	assert.Equal(t, tenant.RuntimeMissingTenant, observed.Outcome)
	require.ErrorIs(t, observed.Err, tenant.ErrInvalidTenantID)
}

func TestObserveMissingTenantReportsEntryPointRejection(t *testing.T) {
	t.Parallel()
	var observed tenant.RuntimeEvent
	ctx := tenant.WithRuntimeObserver(context.Background(), func(event tenant.RuntimeEvent) { observed = event })

	tenant.ObserveMissingTenant(ctx, tenant.ErrInvalidTenantID)

	assert.Equal(t, tenant.RuntimeMissingTenant, observed.Outcome)
	require.ErrorIs(t, observed.Err, tenant.ErrInvalidTenantID)
}
