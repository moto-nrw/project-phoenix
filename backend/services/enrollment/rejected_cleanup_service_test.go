package enrollment

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
