package tenant_test

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func savepointTestContext(t *testing.T) (context.Context, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = bunDB.Close()
		_ = sqlDB.Close()
	})

	mock.ExpectBegin()
	tx, err := bunDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return modelBase.ContextWithTx(context.Background(), &tx), mock
}

func TestWithSavepoint_SuccessReleases(t *testing.T) {
	t.Parallel()

	ctx, mock := savepointTestContext(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))

	called := false
	err := tenant.WithSavepoint(ctx, func(context.Context) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithSavepoint_OperationErrorRollsBackAndRemainsOrdinary(t *testing.T) {
	t.Parallel()

	ctx, mock := savepointTestContext(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	operationErr := errors.New("operation failed")

	err := tenant.WithSavepoint(ctx, func(context.Context) error { return operationErr })

	assert.ErrorIs(t, err, operationErr)
	assert.NotErrorIs(t, err, tenant.ErrSavepointControl)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithSavepoint_OperationErrorDiscardsOnlyNewAfterCommitHooks(t *testing.T) {
	t.Parallel()

	ctx, mock := savepointTestContext(t)
	ctx, drain := tenant.WithAfterCommitHooksForTest(ctx)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))

	var calls []string
	tenant.RegisterAfterCommit(ctx, func() { calls = append(calls, "before") })
	operationErr := errors.New("operation failed")
	err := tenant.WithSavepoint(ctx, func(operationCtx context.Context) error {
		tenant.RegisterAfterCommit(operationCtx, func() { calls = append(calls, "failed operation") })
		return operationErr
	})
	require.ErrorIs(t, err, operationErr)

	drain()
	assert.Equal(t, []string{"before"}, calls,
		"rollback must discard hooks registered inside the savepoint without dropping earlier hooks")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithSavepoint_ControlFailuresAreFatal(t *testing.T) {
	t.Parallel()

	t.Run("missing transaction", func(t *testing.T) {
		called := false
		err := tenant.WithSavepoint(context.Background(), func(context.Context) error {
			called = true
			return nil
		})
		assert.ErrorIs(t, err, tenant.ErrSavepointControl)
		assert.False(t, called)
	})

	t.Run("create", func(t *testing.T) {
		ctx, mock := savepointTestContext(t)
		mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnError(errors.New("create failed"))
		called := false
		err := tenant.WithSavepoint(ctx, func(context.Context) error {
			called = true
			return nil
		})
		assert.ErrorIs(t, err, tenant.ErrSavepointControl)
		assert.False(t, called)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rollback", func(t *testing.T) {
		ctx, mock := savepointTestContext(t)
		mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnError(errors.New("rollback failed"))
		operationErr := errors.New("operation failed")
		err := tenant.WithSavepoint(ctx, func(context.Context) error { return operationErr })
		assert.ErrorIs(t, err, operationErr)
		assert.ErrorIs(t, err, tenant.ErrSavepointControl)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("release", func(t *testing.T) {
		ctx, mock := savepointTestContext(t)
		mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnError(errors.New("release failed"))
		err := tenant.WithSavepoint(ctx, func(context.Context) error { return nil })
		assert.ErrorIs(t, err, tenant.ErrSavepointControl)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
