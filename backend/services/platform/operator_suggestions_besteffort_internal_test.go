package platform

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func newTestServiceWithTx(t *testing.T) (*operatorSuggestionsService, sqlmock.Sqlmock, context.Context) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	mock.ExpectBegin()
	tx, err := bunDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	ctx := modelBase.ContextWithTx(context.Background(), &tx)

	svc := &operatorSuggestionsService{OperatorSuggestionsServiceConfig: OperatorSuggestionsServiceConfig{Logger: slog.Default()}}

	return svc, mock, ctx
}

func TestBestEffort_WithTx_FnSucceeds(t *testing.T) {
	t.Parallel()

	svc, mock, ctx := newTestServiceWithTx(t)

	mock.ExpectExec("SAVEPOINT sp_test_op").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT sp_test_op").WillReturnResult(sqlmock.NewResult(0, 0))

	called := false
	svc.bestEffort(ctx, "test_op", func() error {
		called = true
		return nil
	})

	assert.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBestEffort_WithTx_FnFails_RollsBackSavepoint(t *testing.T) {
	t.Parallel()

	svc, mock, ctx := newTestServiceWithTx(t)

	mock.ExpectExec("SAVEPOINT sp_test_op").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT sp_test_op").WillReturnResult(sqlmock.NewResult(0, 0))

	svc.bestEffort(ctx, "test_op", func() error {
		return errors.New("operation failed")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBestEffort_WithTx_SavepointCreationFails(t *testing.T) {
	t.Parallel()

	svc, mock, ctx := newTestServiceWithTx(t)

	mock.ExpectExec("SAVEPOINT sp_test_op").WillReturnError(errors.New("savepoint error"))

	called := false
	svc.bestEffort(ctx, "test_op", func() error {
		called = true
		return nil
	})

	// fn should NOT be called when savepoint creation fails
	assert.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithAdminTx_UsesExistingTxFromContext(t *testing.T) {
	t.Parallel()

	svc, _, ctx := newTestServiceWithTx(t)

	called := false
	err := tenant.WithAdminTxOrDirect(ctx, svc.DB, func(ctx context.Context) error {
		called = true
		// Verify the tx is still accessible in context
		tx, ok := modelBase.TxFromContext(ctx)
		assert.True(t, ok)
		assert.NotNil(t, tx)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestWithAdminTx_ExistingTx_PropagatesError(t *testing.T) {
	t.Parallel()

	svc, _, ctx := newTestServiceWithTx(t)

	expectedErr := errors.New("fn error")
	err := tenant.WithAdminTxOrDirect(ctx, svc.DB, func(ctx context.Context) error {
		return expectedErr
	})

	assert.ErrorIs(t, err, expectedErr)
}

func TestGetLogger_NilLogger_ReturnsSlogDefault(t *testing.T) {
	t.Parallel()

	svc := &operatorSuggestionsService{OperatorSuggestionsServiceConfig: OperatorSuggestionsServiceConfig{Logger: nil}}
	logger := svc.getLogger()
	assert.Equal(t, slog.Default(), logger)
}

func TestLogAction_SetChangesError(t *testing.T) {
	t.Parallel()

	svc := &operatorSuggestionsService{OperatorSuggestionsServiceConfig: OperatorSuggestionsServiceConfig{AuditLogRepo: &noopAuditLogRepo{}, Logger: slog.Default()}}

	ctx := context.Background()
	postID := int64(1)

	// A func value cannot be marshaled to JSON — triggers SetChanges error
	changes := map[string]any{
		"unmarshalable": func() {},
	}

	// Should not panic; the error is logged and bestEffort still runs
	svc.logAction(ctx, 1, "test_action", "test_resource", &postID, nil, changes)
}

func TestLogAction_NilChanges(t *testing.T) {
	t.Parallel()

	createCalled := false
	svc := &operatorSuggestionsService{OperatorSuggestionsServiceConfig: OperatorSuggestionsServiceConfig{AuditLogRepo: &noopAuditLogRepo{
		createFn: func() { createCalled = true },
	}, Logger: slog.Default()},
	}

	ctx := context.Background()
	postID := int64(1)

	// nil changes should skip SetChanges entirely
	svc.logAction(ctx, 1, "test_action", "test_resource", &postID, nil, nil)
	assert.True(t, createCalled)
}

// noopAuditLogRepo is a minimal audit log repo for internal tests
type noopAuditLogRepo struct {
	createFn func()
}

func (r *noopAuditLogRepo) Create(_ context.Context, _ *platform.OperatorAuditLog) error {
	if r.createFn != nil {
		r.createFn()
	}
	return nil
}

func (r *noopAuditLogRepo) FindByOperatorID(_ context.Context, _ int64, _ int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func (r *noopAuditLogRepo) FindByResourceType(_ context.Context, _ string, _ int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func (r *noopAuditLogRepo) FindByDateRange(_ context.Context, _, _ time.Time, _ int) ([]*platform.OperatorAuditLog, error) {
	return nil, nil
}

func TestGetLogger_WithLogger_ReturnsInjected(t *testing.T) {
	t.Parallel()

	injected := slog.Default().With("test", true)
	svc := &operatorSuggestionsService{OperatorSuggestionsServiceConfig: OperatorSuggestionsServiceConfig{Logger: injected}}
	logger := svc.getLogger()
	assert.Equal(t, injected, logger)
}
