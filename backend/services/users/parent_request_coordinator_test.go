package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type masterDataBulkStub struct {
	rows           []*MasterDataReviewItem
	decided        []int64
	locked         []int64
	lockedRequests []int64
	lockErr        error
	failID         int64
	failErr        error
}

func (s *masterDataBulkStub) LockBulkRequest(_ context.Context, requestID int64) error {
	s.lockedRequests = append(s.lockedRequests, requestID)
	return s.lockErr
}

func (s *masterDataBulkStub) LockBulkStudents(_ context.Context, studentIDs []int64) error {
	s.locked = append(s.locked, studentIDs...)
	return nil
}

func (s *masterDataBulkStub) ListPending(context.Context, modelBase.RequestQueueFilters) ([]*MasterDataReviewItem, *HistoryCursor, error) {
	return s.rows, nil, nil
}

func (s *masterDataBulkStub) GetBulkCandidate(_ context.Context, id int64) (*MasterDataReviewItem, error) {
	for _, row := range s.rows {
		if row.Request.ID == id {
			return row, nil
		}
	}
	return nil, ErrReviewNotFound
}

func (*masterDataBulkStub) ListHistory(context.Context, modelBase.RequestQueueFilters) ([]*MasterDataHistoryItem, *HistoryCursor, error) {
	return nil, nil, nil
}

func (s *masterDataBulkStub) Decide(_ context.Context, input MasterDataReviewDecideInput) (*MasterDataReviewItem, error) {
	if input.RequestID == s.failID {
		if s.failErr != nil {
			return nil, s.failErr
		}
		return nil, errors.New("apply failed")
	}
	s.decided = append(s.decided, input.RequestID)
	return nil, nil
}

type excusedBulkStub struct {
	rows    []ExcusedBulkCandidate
	decided []int64
	failID  int64
	failErr error
	locked  []int64
	lockErr error
}

func (s *excusedBulkStub) LockExcusedBulkRequest(_ context.Context, requestID int64) error {
	s.locked = append(s.locked, requestID)
	return s.lockErr
}

func (s *excusedBulkStub) GetExcusedBulkCandidate(_ context.Context, id int64) (*ExcusedBulkCandidate, error) {
	for i := range s.rows {
		if s.rows[i].ID == id {
			return &s.rows[i], nil
		}
	}
	return nil, nil
}

func (s *excusedBulkStub) ApproveExcusedBulk(_ context.Context, id int64, _ string, _ int64, _ string) error {
	if id == s.failID {
		return s.failErr
	}
	s.decided = append(s.decided, id)
	return nil
}

func pendingBulkMaster(id int64, updatedAt time.Time) *MasterDataReviewItem {
	return &MasterDataReviewItem{BulkEligible: true, Request: &userModels.StudentDataChangeRequest{
		Model: modelBase.Model{ID: id, UpdatedAt: updatedAt}, StudentID: id, Status: userModels.DataChangeStatusPending,
	}}
}

func TestSortedBulkRefsByStudentUsesOneCanonicalLockOrder(t *testing.T) {
	t.Parallel()

	masters := map[int64]*MasterDataReviewItem{
		1: pendingBulkMaster(1, time.Time{}),
		4: pendingBulkMaster(4, time.Time{}),
	}
	masters[1].Request.StudentID = 2
	masters[4].Request.StudentID = 1
	excused := map[int64]ExcusedBulkCandidate{
		2: {ID: 2, StudentID: 1},
		3: {ID: 3, StudentID: 2},
	}
	refs := []ParentRequestRef{
		{Kind: ParentRequestKindMasterData, ID: 1},
		{Kind: ParentRequestKindExcused, ID: 2},
		{Kind: ParentRequestKindExcused, ID: 3},
		{Kind: ParentRequestKindMasterData, ID: 4},
	}

	ordered := sortedBulkRefsByStudent(refs, masters, excused)

	assert.Equal(t, []ParentRequestRef{refs[1], refs[3], refs[2], refs[0]}, ordered)
	assert.Equal(t, []int64{1, 2}, bulkStudentIDs(ordered, masters, excused))
}

func bulkReviewContext(t *testing.T, permissions ...string) context.Context {
	t.Helper()
	ctx := tenant.WithRollbackMarker(t.Context())
	return context.WithValue(ctx, jwt.CtxPermissions, permissions)
}

func TestParentRequestCoordinatorValidatesEveryVersionBeforeApplying(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	master := &masterDataBulkStub{rows: []*MasterDataReviewItem{pendingBulkMaster(1, now)}}
	excused := &excusedBulkStub{rows: []ExcusedBulkCandidate{{ID: 2, UpdatedAt: now, Eligible: true}}}
	service := NewParentRequestCoordinator(master, excused)

	ctx := bulkReviewContext(t, "users:update")
	err := service.BulkApprove(ctx, BulkApproveParentRequestsInput{
		Requests: []ParentRequestRef{
			{Kind: ParentRequestKindMasterData, ID: 1, ExpectedVersion: ParentRequestVersion(now)},
			{Kind: ParentRequestKindExcused, ID: 2, ExpectedVersion: "stale"},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, ErrParentRequestStale)
	assert.Empty(t, master.decided)
	assert.Empty(t, excused.decided)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestParentRequestCoordinatorMarksRollbackWhenApplyFails(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	master := &masterDataBulkStub{
		rows: []*MasterDataReviewItem{pendingBulkMaster(1, now), pendingBulkMaster(2, now)}, failID: 2,
	}
	service := NewParentRequestCoordinator(master, &excusedBulkStub{})

	ctx := bulkReviewContext(t, "users:update")
	err := service.BulkApprove(ctx, BulkApproveParentRequestsInput{
		Requests: []ParentRequestRef{
			{Kind: ParentRequestKindMasterData, ID: 1, ExpectedVersion: ParentRequestVersion(now)},
			{Kind: ParentRequestKindMasterData, ID: 2, ExpectedVersion: ParentRequestVersion(now)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.Error(t, err)
	assert.Equal(t, []int64{1}, master.decided, "the ambient transaction must roll the first write back")
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestParentRequestCoordinatorRejectsIneligibleKindWithoutApplying(t *testing.T) {
	t.Parallel()

	service := NewParentRequestCoordinator(&masterDataBulkStub{}, &excusedBulkStub{})
	ctx := bulkReviewContext(t, "users:update")

	err := service.BulkApprove(ctx, BulkApproveParentRequestsInput{
		Requests: []ParentRequestRef{
			{Kind: "offering", ID: 3, ExpectedVersion: "v"},
			{Kind: ParentRequestKindMasterData, ID: 4, ExpectedVersion: "v"},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, ErrBulkIneligible)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestParentRequestCoordinatorReportsDecisionRaceAsStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	master := &masterDataBulkStub{
		rows:   []*MasterDataReviewItem{pendingBulkMaster(1, now), pendingBulkMaster(2, now)},
		failID: 2, failErr: userModels.ErrChangeRequestNotPending,
	}
	service := NewParentRequestCoordinator(master, &excusedBulkStub{})

	ctx := bulkReviewContext(t, "users:update")
	err := service.BulkApprove(ctx, BulkApproveParentRequestsInput{
		Requests: []ParentRequestRef{
			{Kind: ParentRequestKindMasterData, ID: 1, ExpectedVersion: ParentRequestVersion(now)},
			{Kind: ParentRequestKindMasterData, ID: 2, ExpectedVersion: ParentRequestVersion(now)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, ErrParentRequestStale)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestParentRequestCoordinatorReportsLockRaceAsStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	master := &masterDataBulkStub{
		rows: []*MasterDataReviewItem{
			pendingBulkMaster(1, now),
			pendingBulkMaster(2, now),
		},
		lockErr: userModels.ErrChangeRequestNotPending,
	}
	service := NewParentRequestCoordinator(master, &excusedBulkStub{})
	ctx := bulkReviewContext(t, "users:update")

	err := service.BulkApprove(ctx, BulkApproveParentRequestsInput{
		Requests: []ParentRequestRef{
			{Kind: ParentRequestKindMasterData, ID: 1, ExpectedVersion: ParentRequestVersion(now)},
			{Kind: ParentRequestKindMasterData, ID: 2, ExpectedVersion: ParentRequestVersion(now)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, ErrParentRequestStale)
	assert.Empty(t, master.decided)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestParentRequestCoordinatorRejectsMasterDataForAbsenceOnlyReviewer(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	master := &masterDataBulkStub{rows: []*MasterDataReviewItem{pendingBulkMaster(1, now), pendingBulkMaster(2, now)}}
	service := NewParentRequestCoordinator(master, &excusedBulkStub{})
	ctx := bulkReviewContext(t, "users:absence")

	err := service.BulkApprove(ctx, BulkApproveParentRequestsInput{
		Requests: []ParentRequestRef{
			{Kind: ParentRequestKindMasterData, ID: 1, ExpectedVersion: ParentRequestVersion(now)},
			{Kind: ParentRequestKindMasterData, ID: 2, ExpectedVersion: ParentRequestVersion(now)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, ErrParentRequestForbidden)
	assert.Empty(t, master.decided)
	assert.True(t, tenant.RollbackRequested(ctx))
}
