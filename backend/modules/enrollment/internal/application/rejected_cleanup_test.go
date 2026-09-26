package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type cleanupSettingsStub struct {
	days int
	err  error
}

func (s cleanupSettingsStub) RejectedRetentionDays(context.Context) (int, error) {
	return s.days, s.err
}

type cleanupRequestStub struct {
	ids       []int64
	listErr   error
	lockErr   map[int64]error
	deleteErr map[int64]error
	cutoff    time.Time
	locked    []int64
	deleted   []int64
}

func (s *cleanupRequestStub) DeleteRequestTree(ctx context.Context, id int64) error {
	return s.DeleteRequest(ctx, id)
}

func (s *cleanupRequestStub) FullyRejectedRequestsBefore(_ context.Context, cutoff time.Time) ([]int64, error) {
	s.cutoff = cutoff
	return s.ids, s.listErr
}
func (s *cleanupRequestStub) RequestByID(_ context.Context, id int64, _ bool) (*enrollment.Request, error) {
	s.locked = append(s.locked, id)
	if err := s.lockErr[id]; err != nil {
		return nil, err
	}
	return &enrollment.Request{}, nil
}
func (s *cleanupRequestStub) DeleteRequest(_ context.Context, id int64) error {
	if err := s.deleteErr[id]; err != nil {
		return err
	}
	s.deleted = append(s.deleted, id)
	return nil
}

type cleanupChildrenStub struct {
	byRequestID map[int64][]*enrollment.RequestChild
	errFor      map[int64]error
	locked      []int64
}

func (s *cleanupChildrenStub) ChildrenForRequest(_ context.Context, requestID int64, forUpdate bool) ([]*enrollment.RequestChild, error) {
	if !forUpdate {
		return nil, errors.New("cleanup must lock children")
	}
	s.locked = append(s.locked, requestID)
	if err := s.errFor[requestID]; err != nil {
		return nil, err
	}
	return s.byRequestID[requestID], nil
}

func eligibleCleanupChildren(requestIDs ...int64) *cleanupChildrenStub {
	reviewedAt := time.Now().Add(-365 * 24 * time.Hour)
	children := &cleanupChildrenStub{
		byRequestID: make(map[int64][]*enrollment.RequestChild, len(requestIDs)),
		errFor:      map[int64]error{},
	}
	for _, requestID := range requestIDs {
		children.byRequestID[requestID] = []*enrollment.RequestChild{{
			RequestID:  requestID,
			Status:     enrollmentModels.ChildStatusRejected,
			ReviewedAt: &reviewedAt,
		}}
	}
	return children
}

type cleanupOutboxStub struct {
	counts  map[int64]int64
	errFor  map[int64]error
	deleted []int64
}

type cleanupLateInvitesStub struct {
	counts  map[int64]int64
	errFor  map[int64]error
	deleted []int64
}

func (s *cleanupLateInvitesStub) DeleteLateInvitesByUsedRequestID(_ context.Context, requestID int64) (int64, error) {
	if err := s.errFor[requestID]; err != nil {
		return 0, err
	}
	s.deleted = append(s.deleted, requestID)
	return s.counts[requestID], nil
}

func (s *cleanupOutboxStub) CountRelatedEmails(_ context.Context, relatedType string, id int64) (int, error) {
	if relatedType != enrollmentRequestDeliveryType {
		return 0, errors.New("unexpected related type")
	}
	if err := s.errFor[id]; err != nil {
		return 0, err
	}
	return int(s.counts[id]), nil
}

func (s *cleanupOutboxStub) CancelRelatedEmails(_ context.Context, relatedType string, id int64, _ string) (int64, error) {
	if relatedType != enrollmentRequestDeliveryType {
		return 0, errors.New("unexpected related type")
	}
	if err := s.errFor[id]; err != nil {
		return 0, err
	}
	s.deleted = append(s.deleted, id)
	return s.counts[id], nil
}

func cleanupServiceForTest(requests *cleanupRequestStub, children *cleanupChildrenStub, outbox *cleanupOutboxStub, settings cleanupSettingsStub, lateInviteStubs ...*cleanupLateInvitesStub) *RejectedCleanup {
	lateInvites := &cleanupLateInvitesStub{counts: map[int64]int64{}, errFor: map[int64]error{}}
	if len(lateInviteStubs) > 0 && lateInviteStubs[0] != nil {
		lateInvites = lateInviteStubs[0]
	}
	return NewRejectedCleanup(RejectedCleanupDependencies{
		Requests:    requests,
		Children:    children,
		LateInvites: lateInvites,
		Delivery:    outbox,
		Settings:    settings,
		Logger:      slog.New(slog.DiscardHandler),
		Runtime: Runtime{
			InTransaction: func(context.Context) bool { return false },
			WithinCurrentTenant: func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		},
	})
}

func TestRejectedEnrollmentCleanup_DeletesUsedLateInvites(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11)
	lateInvites := &cleanupLateInvitesStub{counts: map[int64]int64{11: 2}, errFor: map[int64]error{}}
	outbox := &cleanupOutboxStub{counts: map[int64]int64{11: 3}, errFor: map[int64]error{}}

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}, lateInvites).CleanupRejectedEnrollments(context.Background())

	require.NoError(t, err)
	assert.Equal(t, enrollment.RejectedEnrollmentCleanupResult{DeletedRequests: 1, DeletedLateInvites: 2, DeletedOutboxRows: 3}, result)
	assert.Equal(t, []int64{11}, lateInvites.deleted)
	assert.Equal(t, []int64{11}, outbox.deleted)
	assert.Equal(t, []int64{11}, requests.deleted)
}

func TestRejectedEnrollmentCleanup_StopsOnLateInviteDeleteFailure(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11)
	lateInvites := &cleanupLateInvitesStub{counts: map[int64]int64{}, errFor: map[int64]error{11: errors.New("delete failed")}}
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{}}

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}, lateInvites).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "delete used enrollment late invites")
	assert.Zero(t, result)
	assert.Empty(t, outbox.deleted)
	assert.Empty(t, requests.deleted)
}

func TestRejectedEnrollmentCleanup_DeletesOnlyRepositorySelectedRequests(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11, 12}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11, 12)
	outbox := &cleanupOutboxStub{counts: map[int64]int64{11: 2, 12: 1}, errFor: map[int64]error{}}
	before := time.Now().Add(-30 * 24 * time.Hour)

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.NoError(t, err)
	assert.Equal(t, enrollment.RejectedEnrollmentCleanupResult{DeletedRequests: 2, DeletedOutboxRows: 3}, result)
	assert.Equal(t, []int64{11, 12}, requests.locked)
	assert.Equal(t, []int64{11, 12}, children.locked)
	assert.Equal(t, []int64{11, 12}, outbox.deleted)
	assert.Equal(t, []int64{11, 12}, requests.deleted)
	assert.WithinDuration(t, before, requests.cutoff, 2*time.Second)
}

func TestRejectedEnrollmentCleanup_ResolutionFailurePerformsNoDeletes(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11)
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{}}

	_, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{err: errors.New("settings unavailable")}).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.Empty(t, requests.locked)
	assert.Empty(t, children.locked)
	assert.Empty(t, outbox.deleted)
	assert.Empty(t, requests.deleted)
}

func TestRejectedEnrollmentCleanup_StopsOnDependentDeleteFailure(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11, 12}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11, 12)
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{11: errors.New("delete failed")}}

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.Zero(t, result)
	assert.Empty(t, requests.deleted)
	assert.Empty(t, outbox.deleted)
}

func TestRejectedEnrollmentCleanup_RechecksLockedChildrenBeforeDeleting(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11, 12}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11, 12)
	reviewedAt := time.Now().Add(-365 * 24 * time.Hour)
	children.byRequestID[11] = []*enrollment.RequestChild{{
		RequestID:  11,
		Status:     enrollmentModels.ChildStatusUnderReview,
		ReviewedAt: &reviewedAt,
	}}
	outbox := &cleanupOutboxStub{counts: map[int64]int64{11: 3, 12: 2}, errFor: map[int64]error{}}

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.NoError(t, err)
	assert.Equal(t, enrollment.RejectedEnrollmentCleanupResult{DeletedRequests: 1, DeletedOutboxRows: 2}, result)
	assert.Equal(t, []int64{11, 12}, requests.locked)
	assert.Equal(t, []int64{11, 12}, children.locked)
	assert.Equal(t, []int64{12}, outbox.deleted)
	assert.Equal(t, []int64{12}, requests.deleted)
}

func TestRejectedEnrollmentCleanup_StopsOnRequestLockFailure(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{
		ids:       []int64{11},
		lockErr:   map[int64]error{11: errors.New("lock failed")},
		deleteErr: map[int64]error{},
	}
	children := eligibleCleanupChildren(11)
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{}}

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "lock rejected enrollment request")
	assert.Zero(t, result)
	assert.Empty(t, children.locked)
	assert.Empty(t, outbox.deleted)
	assert.Empty(t, requests.deleted)
}

func TestRejectedEnrollmentCleanup_StopsOnChildLockFailure(t *testing.T) {
	t.Parallel()

	requests := &cleanupRequestStub{ids: []int64{11}, deleteErr: map[int64]error{}}
	children := eligibleCleanupChildren(11)
	children.errFor[11] = errors.New("lock failed")
	outbox := &cleanupOutboxStub{counts: map[int64]int64{}, errFor: map[int64]error{}}

	result, err := cleanupServiceForTest(requests, children, outbox, cleanupSettingsStub{days: 30}).CleanupRejectedEnrollments(context.Background())

	require.Error(t, err)
	assert.ErrorContains(t, err, "lock rejected enrollment request children")
	assert.Zero(t, result)
	assert.Empty(t, outbox.deleted)
	assert.Empty(t, requests.deleted)
}

func TestChildrenRemainFullyRejectedBefore(t *testing.T) {
	t.Parallel()

	cutoff := time.Now()
	oldReview := cutoff.Add(-time.Hour)
	newReview := cutoff.Add(time.Hour)

	tests := []struct {
		name     string
		children []*enrollment.RequestChild
		want     bool
	}{
		{name: "no children"},
		{name: "nil child", children: []*enrollment.RequestChild{nil}},
		{name: "reopened child", children: []*enrollment.RequestChild{{Status: enrollmentModels.ChildStatusUnderReview, ReviewedAt: &oldReview}}},
		{name: "missing review time", children: []*enrollment.RequestChild{{Status: enrollmentModels.ChildStatusRejected}}},
		{name: "review exactly at cutoff", children: []*enrollment.RequestChild{{Status: enrollmentModels.ChildStatusRejected, ReviewedAt: &cutoff}}},
		{name: "review after cutoff", children: []*enrollment.RequestChild{{Status: enrollmentModels.ChildStatusRejected, ReviewedAt: &newReview}}},
		{name: "all rejected before cutoff", children: []*enrollment.RequestChild{
			{Status: enrollmentModels.ChildStatusRejected, ReviewedAt: &oldReview},
			{Status: enrollmentModels.ChildStatusRejected, ReviewedAt: &oldReview},
		}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, childrenRemainFullyRejectedBefore(tt.children, cutoff))
		})
	}
}
