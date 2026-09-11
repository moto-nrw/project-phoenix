package active

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/DATA-DOG/go-sqlmock"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureStaffPresenceKeepsCheckInFailureBestEffort(t *testing.T) {
	t.Parallel()

	called := false
	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return nil, errors.New("check-in failed")
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresence(context.Background(), 42, activeModels.WorkSessionSourceApp)

	assert.True(t, called)
}

func TestEnsureStaffPresenceAcceptsClosedSessionSkip(t *testing.T) {
	t.Parallel()

	called := false
	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return nil, nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresence(context.Background(), 42, activeModels.WorkSessionSourceApp)

	assert.True(t, called)
}

func TestEnsureStaffPresenceForAttendanceResultSkipsIdempotentCheckout(t *testing.T) {
	t.Parallel()

	called := false
	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return nil, nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresenceForAttendanceResult(context.Background(), 42, &AttendanceResult{
		Action: "checked_out",
	})

	assert.False(t, called)
}

func TestEnsureStaffPresenceForAttendanceResultStampsMutatingCheckout(t *testing.T) {
	t.Parallel()

	called := false
	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			called = true
			return nil, nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresenceForAttendanceResult(context.Background(), 42, &AttendanceResult{
		Action:       "checked_out",
		AttendanceID: 101,
	})

	assert.True(t, called)
}

func TestEnsureStaffPresenceRollsBackFailedCheckInToSavepoint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, mock := newSessionSQLMockDB(t)
	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	txCtx := tenant.WithTransactionForTest(withSessionTestRuntime(t, ctx, db), &tx)

	mock.ExpectExec("SAVEPOINT phoenix_operation").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT phoenix_operation").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT phoenix_operation").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	workSessions := &workSessionServiceForSessionUnitTest{
		ensureCheckedInFunc: func(_ context.Context, _ int64, _ string) (*activeModels.WorkSession, error) {
			return nil, errors.New("check-in failed")
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		WorkSessionService: workSessions,
		Logger:             slog.New(slog.DiscardHandler),
	}}

	svc.ensureStaffPresence(txCtx, 42, activeModels.WorkSessionSourceApp)

	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
