package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

type masterDataBulkStub struct {
	rows           []*careplan.MasterDataReviewItem
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

func (s *masterDataBulkStub) GetBulkCandidate(_ context.Context, id int64) (*careplan.MasterDataReviewItem, error) {
	for _, row := range s.rows {
		if row.Request.ID == id {
			return row, nil
		}
	}
	return nil, masterdatarequests.ErrReviewNotFound
}

func (s *masterDataBulkStub) Decide(_ context.Context, input masterdatarequests.DecideInput) (*careplan.MasterDataReviewItem, error) {
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
	rows    []ports.ExcusedBulkCandidate
	decided []int64
	locked  []int64
	lockErr error
}

func (s *excusedBulkStub) LockExcusedBulkRequest(_ context.Context, requestID int64) error {
	s.locked = append(s.locked, requestID)
	return s.lockErr
}

func (s *excusedBulkStub) GetExcusedBulkCandidate(_ context.Context, id int64) (*ports.ExcusedBulkCandidate, error) {
	for i := range s.rows {
		if s.rows[i].ID == id {
			return &s.rows[i], nil
		}
	}
	return nil, nil
}

func (s *excusedBulkStub) ApproveExcusedBulk(_ context.Context, id int64, _ string, _ int64, _ string) error {
	s.decided = append(s.decided, id)
	return nil
}

// rollbackRecorder stands in for the tenant transaction's rollback marker.
type rollbackRecorder struct{ requested bool }

func (r *rollbackRecorder) MarkRollback(context.Context) { r.requested = true }

var (
	writeReviewer   = ports.ParentRequestRights{WriteQueues: true, Absences: true}
	absenceReviewer = ports.ParentRequestRights{Absences: true}
)

func rightsOf(rights ports.ParentRequestRights) ports.ParentRequestRightsResolver {
	return func(context.Context) ports.ParentRequestRights { return rights }
}

func newTestCoordinator(t *testing.T, rights ports.ParentRequestRights, deps ParentRequestCoordinatorDependencies) (*ParentRequestCoordinator, *rollbackRecorder) {
	t.Helper()
	rollback := &rollbackRecorder{}
	deps.Rights, deps.Rollback = rightsOf(rights), rollback
	coordinator, err := NewParentRequestCoordinator(deps)
	require.NoError(t, err)
	return coordinator, rollback
}

func pendingBulkMaster(id int64, updatedAt time.Time) *careplan.MasterDataReviewItem {
	return &careplan.MasterDataReviewItem{BulkEligible: true, Request: &careplan.StudentDataChangeRequest{
		ID: id, UpdatedAt: updatedAt, StudentID: id, Status: masterdatarequests.StatusPending,
	}}
}

var bulkNow = time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)

func TestSortedBulkRefsByStudentUsesOneCanonicalLockOrder(t *testing.T) {
	t.Parallel()

	masters := map[int64]*careplan.MasterDataReviewItem{
		1: pendingBulkMaster(1, time.Time{}),
		4: pendingBulkMaster(4, time.Time{}),
	}
	masters[1].Request.StudentID = 2
	masters[4].Request.StudentID = 1
	excused := map[int64]ports.ExcusedBulkCandidate{
		2: {ID: 2, StudentID: 1},
		3: {ID: 3, StudentID: 2},
	}
	refs := []parentrequests.Ref{
		{Kind: parentrequests.KindMasterData, ID: 1},
		{Kind: parentrequests.KindExcused, ID: 2},
		{Kind: parentrequests.KindExcused, ID: 3},
		{Kind: parentrequests.KindMasterData, ID: 4},
	}

	ordered := sortedBulkRefsByStudent(refs, masters, excused)

	assert.Equal(t, []parentrequests.Ref{refs[1], refs[3], refs[2], refs[0]}, ordered)
	assert.Equal(t, []int64{1, 2}, bulkStudentIDs(ordered, masters, excused))
}

func TestParentRequestCoordinatorDoesNotBypassTheRightsPort(t *testing.T) {
	t.Parallel()
	service, rollback := newTestCoordinator(t, ports.ParentRequestRights{}, ParentRequestCoordinatorDependencies{})
	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: "v1"},
			{Kind: parentrequests.KindMasterData, ID: 2, ExpectedVersion: "v2"},
		},
		Reason: "Reviewed", ReviewerID: 99,
	})
	require.ErrorIs(t, err, parentrequests.ErrForbidden)
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorValidatesEveryVersionBeforeApplying(t *testing.T) {
	t.Parallel()
	master := &masterDataBulkStub{rows: []*careplan.MasterDataReviewItem{pendingBulkMaster(1, bulkNow)}}
	excused := &excusedBulkStub{rows: []ports.ExcusedBulkCandidate{{ID: 2, UpdatedAt: bulkNow, Eligible: true}}}
	service, rollback := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{MasterData: master, Excused: excused})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
			{Kind: parentrequests.KindExcused, ID: 2, ExpectedVersion: "stale"},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, parentrequests.ErrStale)
	assert.Empty(t, master.decided)
	assert.Empty(t, excused.decided)
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorMarksRollbackWhenApplyFails(t *testing.T) {
	t.Parallel()
	master := &masterDataBulkStub{rows: []*careplan.MasterDataReviewItem{pendingBulkMaster(1, bulkNow), pendingBulkMaster(2, bulkNow)}, failID: 2}
	service, rollback := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{MasterData: master, Excused: &excusedBulkStub{}})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
			{Kind: parentrequests.KindMasterData, ID: 2, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.Error(t, err)
	assert.Equal(t, []int64{1}, master.decided, "the ambient transaction must roll the first write back")
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorRejectsIneligibleKindWithoutApplying(t *testing.T) {
	t.Parallel()
	service, rollback := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{MasterData: &masterDataBulkStub{}, Excused: &excusedBulkStub{}})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindOffering, ID: 3, ExpectedVersion: "v"},
			{Kind: parentrequests.KindMasterData, ID: 4, ExpectedVersion: "v"},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, parentrequests.ErrBulkIneligible)
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorReportsDecisionRaceAsStale(t *testing.T) {
	t.Parallel()
	master := &masterDataBulkStub{
		rows:   []*careplan.MasterDataReviewItem{pendingBulkMaster(1, bulkNow), pendingBulkMaster(2, bulkNow)},
		failID: 2, failErr: careplan.ErrStudentDataRequestNotPending,
	}
	service, rollback := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{MasterData: master, Excused: &excusedBulkStub{}})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
			{Kind: parentrequests.KindMasterData, ID: 2, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, parentrequests.ErrStale)
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorReportsLockRaceAsStale(t *testing.T) {
	t.Parallel()
	master := &masterDataBulkStub{
		rows:    []*careplan.MasterDataReviewItem{pendingBulkMaster(1, bulkNow), pendingBulkMaster(2, bulkNow)},
		lockErr: masterdatarequests.ErrReviewNotPending,
	}
	service, rollback := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{MasterData: master, Excused: &excusedBulkStub{}})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
			{Kind: parentrequests.KindMasterData, ID: 2, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, parentrequests.ErrStale)
	assert.Equal(t, "parent requests: request version is stale: master_data 1 changed before locking", err.Error(),
		"the students route renders this text verbatim")
	assert.Empty(t, master.decided)
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorRejectsMasterDataForAbsenceOnlyReviewer(t *testing.T) {
	t.Parallel()
	master := &masterDataBulkStub{rows: []*careplan.MasterDataReviewItem{pendingBulkMaster(1, bulkNow), pendingBulkMaster(2, bulkNow)}}
	service, rollback := newTestCoordinator(t, absenceReviewer, ParentRequestCoordinatorDependencies{MasterData: master, Excused: &excusedBulkStub{}})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
			{Kind: parentrequests.KindMasterData, ID: 2, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
		},
		Reason: "Geprüft", ReviewerID: 99,
	})

	require.ErrorIs(t, err, parentrequests.ErrForbidden)
	assert.Empty(t, master.decided)
	assert.True(t, rollback.requested)
}

func TestParentRequestCoordinatorRequiresTheSchoolsReasonForABulkApproval(t *testing.T) {
	t.Parallel()
	master := &masterDataBulkStub{rows: []*careplan.MasterDataReviewItem{pendingBulkMaster(1, bulkNow), pendingBulkMaster(2, bulkNow)}}
	service, rollback := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{MasterData: master, Excused: &excusedBulkStub{}})

	err := service.BulkApprove(context.Background(), parentrequests.BulkApproveInput{
		Requests: []parentrequests.Ref{
			{Kind: parentrequests.KindMasterData, ID: 1, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
			{Kind: parentrequests.KindMasterData, ID: 2, ExpectedVersion: careplan.ParentRequestVersion(bulkNow)},
		},
		Reason: "  ", ReasonRequired: true, ReviewerID: 99,
	})

	require.ErrorIs(t, err, parentrequests.ErrReasonRequired)
	assert.Equal(t, "parent requests: a reason is required", err.Error())
	assert.Empty(t, master.decided)
	assert.True(t, rollback.requested)
}
