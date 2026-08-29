package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func savepointContext(t *testing.T, control func(tenant.SavepointAction) error) context.Context {
	t.Helper()
	runtime, err := tenant.NewRuntime(
		func(_ context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(context.Background(), struct{}{})
		},
		func(_ context.Context, fn func(context.Context, any) error) error {
			return fn(context.Background(), struct{}{})
		},
		func(_ context.Context, action tenant.SavepointAction) error { return control(action) },
	)
	require.NoError(t, err)
	return tenant.WithRuntime(context.Background(), runtime)
}

func TestWithSavepoint_Success(t *testing.T) {
	t.Parallel()
	var actions []tenant.SavepointAction
	ctx := savepointContext(t, func(action tenant.SavepointAction) error {
		actions = append(actions, action)
		return nil
	})

	require.NoError(t, tenant.WithSavepoint(ctx, func(context.Context) error { return nil }))
	assert.Equal(t, []tenant.SavepointAction{tenant.CreateSavepoint, tenant.ReleaseSavepoint}, actions)
}

func TestWithSavepoint_OperationFailureRollsBack(t *testing.T) {
	t.Parallel()
	operationErr := errors.New("operation failed")
	var actions []tenant.SavepointAction
	ctx := savepointContext(t, func(action tenant.SavepointAction) error {
		actions = append(actions, action)
		return nil
	})

	err := tenant.WithSavepoint(ctx, func(context.Context) error { return operationErr })
	assert.ErrorIs(t, err, operationErr)
	assert.NotErrorIs(t, err, tenant.ErrSavepointControl)
	assert.Equal(t, []tenant.SavepointAction{
		tenant.CreateSavepoint,
		tenant.RollbackSavepoint,
		tenant.ReleaseSavepoint,
	}, actions)
}

func TestWithSavepoint_FailsWithoutRuntime(t *testing.T) {
	t.Parallel()
	err := tenant.WithSavepoint(context.Background(), func(context.Context) error { return nil })
	assert.ErrorIs(t, err, tenant.ErrSavepointControl)
}

func TestWithSavepoint_ControlFailureIsFatal(t *testing.T) {
	t.Parallel()
	controlErr := errors.New("exec failed")
	ctx := savepointContext(t, func(tenant.SavepointAction) error { return controlErr })

	err := tenant.WithSavepoint(ctx, func(context.Context) error { return nil })
	assert.ErrorIs(t, err, tenant.ErrSavepointControl)
}
