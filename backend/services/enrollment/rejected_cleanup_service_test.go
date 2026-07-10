package enrollment

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type cleanupSettingsStub struct {
	days int
	err  error
}

func (s cleanupSettingsStub) HasTenantOverride(context.Context, string) (bool, error) {
	return false, nil
}
func (s cleanupSettingsStub) ResolveBool(context.Context, string) (bool, error) {
	return false, nil
}
func (s cleanupSettingsStub) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (s cleanupSettingsStub) ResolveInt(_ context.Context, key string) (int, error) {
	if key != configModel.KeyEnrollmentRejectedRetentionDays {
		return 0, errors.New("unexpected key")
	}
	return s.days, s.err
}

type cleanupRequestStub struct {
	ids       []int64
	listErr   error
	deleteErr map[int64]error
	cutoff    time.Time
	deleted   []int64
}

func (s *cleanupRequestStub) ListFullyRejectedBefore(_ context.Context, cutoff time.Time) ([]int64, error) {
	s.cutoff = cutoff
	return s.ids, s.listErr
}
func (s *cleanupRequestStub) DeleteByID(_ context.Context, id int64) error {
	if err := s.deleteErr[id]; err != nil {
		return err
	}
	s.deleted = append(s.deleted, id)
	return nil
}

type cleanupOutboxStub struct {
	counts  map[int64]int64
	errFor  map[int64]error
	deleted []int64
}

func (s *cleanupOutboxStub) DeleteByRelatedEntity(_ context.Context, relatedType string, id int64) (int64, error) {
	if relatedType != platformModels.EmailRelatedTypeEnrollmentRequest {
		return 0, errors.New("unexpected related type")
	}
	if err := s.errFor[id]; err != nil {
		return 0, err
	}
	s.deleted = append(s.deleted, id)
	return s.counts[id], nil
}

func cleanupServiceForTest(requests *cleanupRequestStub, outbox *cleanupOutboxStub, settings cleanupSettingsStub) *rejectedEnrollmentCleanupService {
	return &rejectedEnrollmentCleanupService{
		requests: requests,
		outbox:   outbox,
		settings: settings,
		logger:   slog.New(slog.DiscardHandler),
		runInTx: func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	}
}

func TestRejectedEnrollmentCleanup_DeletesOnlyRepositorySelectedRequests(t *testing.T) {
	requests := &cleanupRequestStub{ids: []int64{11, 12}, deleteErr: map[int64]error{}}
	outbox := &cleanupOutboxStub{counts: map[int64]int64{11: 2, 12: 1}, errFor: map[int64]error{}}
	before := time.Now().Add(-30 * 24 * time.Hour)

	result, err := cleanupServiceForTest(requests, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.NoError(t, err)
	assert.Equal(t, RejectedEnrollmentCleanupResult{DeletedRequests: 2, DeletedOutboxRows: 3}, result)
	assert.Equal(t, []int64{11, 12}, outbox.deleted)
	assert.Equal(t, []int64{11, 12}, requests.deleted)
	assert.WithinDuration(t, before, requests.cutoff, 2*time.Second)
}

func TestRejectedEnrollmentCleanup_ResolutionFailurePerformsNoDeletes(t *testing.T) {
	requests := &cleanupRequestStub{ids: []int64{11}, deleteErr: map[int64]error{}}
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{}}

	_, err := cleanupServiceForTest(requests, outbox, cleanupSettingsStub{err: errors.New("settings unavailable")}).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.Empty(t, outbox.deleted)
	assert.Empty(t, requests.deleted)
}

func TestRejectedEnrollmentCleanup_StopsOnDependentDeleteFailure(t *testing.T) {
	requests := &cleanupRequestStub{ids: []int64{11, 12}, deleteErr: map[int64]error{}}
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{11: errors.New("delete failed")}}

	result, err := cleanupServiceForTest(requests, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.Zero(t, result)
	assert.Empty(t, requests.deleted)
	assert.Empty(t, outbox.deleted)
}

func rejectedCleanupAmbientTx(t *testing.T) (context.Context, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db := bun.NewDB(sqlDB, pgdialect.New())
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	return modelBase.ContextWithTx(context.Background(), &tx), mock
}

func TestRejectedEnrollmentCleanupSavepointSuccess(t *testing.T) {
	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))

	called := false
	err := newRejectedEnrollmentCleanupTxRunner(nil)(ctx, func(context.Context) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointRollsBackCallbackFailure(t *testing.T) {
	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	expected := errors.New("delete failed")

	err := newRejectedEnrollmentCleanupTxRunner(nil)(ctx, func(context.Context) error { return expected })

	require.ErrorIs(t, err, expected)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointRollbackFailureJoinsErrors(t *testing.T) {
	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	rollbackErr := errors.New("rollback failed")
	mock.ExpectExec("ROLLBACK TO SAVEPOINT enrollment_rejected_cleanup").WillReturnError(rollbackErr)
	callbackErr := errors.New("delete failed")

	err := newRejectedEnrollmentCleanupTxRunner(nil)(ctx, func(context.Context) error { return callbackErr })

	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, rollbackErr)
	assert.ErrorContains(t, err, "rollback rejected enrollment cleanup savepoint")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointRollbackReleaseFailureJoinsErrors(t *testing.T) {
	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ROLLBACK TO SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	releaseErr := errors.New("release failed")
	mock.ExpectExec("RELEASE SAVEPOINT enrollment_rejected_cleanup").WillReturnError(releaseErr)
	callbackErr := errors.New("delete failed")

	err := newRejectedEnrollmentCleanupTxRunner(nil)(ctx, func(context.Context) error { return callbackErr })

	require.ErrorIs(t, err, callbackErr)
	require.ErrorIs(t, err, releaseErr)
	assert.ErrorContains(t, err, "release rejected enrollment cleanup savepoint")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointCreationFailureSkipsCallback(t *testing.T) {
	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT enrollment_rejected_cleanup").WillReturnError(errors.New("savepoint unavailable"))

	called := false
	err := newRejectedEnrollmentCleanupTxRunner(nil)(ctx, func(context.Context) error {
		called = true
		return nil
	})

	require.EqualError(t, err, "create rejected enrollment cleanup savepoint: savepoint unavailable")
	assert.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectedEnrollmentCleanupSavepointReleaseFailureIsReturned(t *testing.T) {
	ctx, mock := rejectedCleanupAmbientTx(t)
	mock.ExpectExec("SAVEPOINT enrollment_rejected_cleanup").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RELEASE SAVEPOINT enrollment_rejected_cleanup").WillReturnError(errors.New("release failed"))

	err := newRejectedEnrollmentCleanupTxRunner(nil)(ctx, func(context.Context) error { return nil })

	require.EqualError(t, err, "release rejected enrollment cleanup savepoint: release failed")
	require.NoError(t, mock.ExpectationsWereMet())
}
