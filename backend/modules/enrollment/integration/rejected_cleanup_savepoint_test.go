package integration

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Inside an ambient tenant transaction the retention worker joins it through
// a savepoint, so a failed run rolls back only its own work. These tests run
// the composed worker over the real tenant runtime; the listing step stands
// in for the whole callback.

func rejectedCleanupAmbientTx(t *testing.T) (context.Context, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db := bun.NewDB(sqlDB, pgdialect.New())
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	controller := rejectedCleanupSavepointController{tx: &tx}
	uow, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error { return fn(ctx, tx) },
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, tx) },
		tenant.SavepointFunc(controller),
		func(error) bool { return false },
	)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), uow)
	return tenant.WithTransactionForTest(ctx, &tx), mock
}

type rejectedCleanupSavepointController struct{ tx *bun.Tx }

func (c rejectedCleanupSavepointController) CreateSavepoint(ctx context.Context) error {
	_, err := c.tx.ExecContext(ctx, "SAVEPOINT phoenix_operation")
	return err
}

func (c rejectedCleanupSavepointController) RollbackSavepoint(ctx context.Context) error {
	_, err := c.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT phoenix_operation")
	return err
}

func (c rejectedCleanupSavepointController) ReleaseSavepoint(ctx context.Context) error {
	_, err := c.tx.ExecContext(ctx, "RELEASE SAVEPOINT phoenix_operation")
	return err
}

// savepointCleanupRequests answers the listing with the configured outcome
// and records whether the worker reached it.
type savepointCleanupRequests struct {
	enrollmentCompose.RejectedRequestCleaner
	err    error
	called bool
}

func (r *savepointCleanupRequests) FullyRejectedRequestsBefore(context.Context, time.Time) ([]int64, error) {
	r.called = true
	return nil, r.err
}

type savepointCleanupRetention struct{}

func (savepointCleanupRetention) RejectedRetentionDays(context.Context) (int, error) { return 30, nil }

type savepointCleanupChildren struct {
	enrollmentCompose.RejectedChildren
}

type savepointCleanupLateInvites struct {
	enrollmentCompose.UsedLateInviteCleaner
}

type savepointCleanupDelivery struct {
	enrollmentCompose.EnrollmentDeletionDelivery
}

func runSavepointCleanup(ctx context.Context, requests *savepointCleanupRequests) error {
	_, err := enrollmentCompose.NewRejectedCleanup(enrollmentCompose.RejectedCleanupDependencies{
		Requests:    requests,
		Children:    savepointCleanupChildren{},
		LateInvites: savepointCleanupLateInvites{},
		Delivery:    savepointCleanupDelivery{},
		Settings:    savepointCleanupRetention{},
		Logger:      slog.New(slog.DiscardHandler),
	}).CleanupRejectedEnrollments(ctx)
	return err
}

func TestRejectedEnrollmentCleanupSavepointSuccess(t *testing.T) {
	t.Parallel()

	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))

	requests := &savepointCleanupRequests{}
	err := runSavepointCleanup(ctx, requests)

	require.NoError(t, err)
	assert.True(t, requests.called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointRollsBackCallbackFailure(t *testing.T) {
	t.Parallel()

	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	expected := errors.New("delete failed")

	err := runSavepointCleanup(ctx, &savepointCleanupRequests{err: expected})

	require.ErrorIs(t, err, expected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointRollbackFailureJoinsErrors(t *testing.T) {
	t.Parallel()

	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	rollbackErr := errors.New("rollback failed")
	mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnError(rollbackErr)
	callbackErr := errors.New("delete failed")

	err := runSavepointCleanup(ctx, &savepointCleanupRequests{err: callbackErr})

	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, rollbackErr)
	assert.ErrorContains(t, err, "savepoint control failed: rollback")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointRollbackReleaseFailureJoinsErrors(t *testing.T) {
	t.Parallel()

	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	releaseErr := errors.New("release failed")
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnError(releaseErr)
	callbackErr := errors.New("delete failed")

	err := runSavepointCleanup(ctx, &savepointCleanupRequests{err: callbackErr})

	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, releaseErr)
	assert.ErrorContains(t, err, "savepoint control failed: release after rollback")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointCreationFailureSkipsCallback(t *testing.T) {
	t.Parallel()

	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnError(errors.New("savepoint unavailable"))

	requests := &savepointCleanupRequests{}
	err := runSavepointCleanup(ctx, requests)

	require.EqualError(t, err, "savepoint control failed: create: savepoint unavailable")
	assert.False(t, requests.called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointReleaseFailureIsReturned(t *testing.T) {
	t.Parallel()

	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT phoenix_operation").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").WillReturnError(errors.New("release failed"))

	err := runSavepointCleanup(ctx, &savepointCleanupRequests{})

	require.EqualError(t, err, "savepoint control failed: release: release failed")
	require.NoError(t, mock.ExpectationsWereMet())
}
