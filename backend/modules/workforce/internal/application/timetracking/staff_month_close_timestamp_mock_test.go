package timetracking

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type timestampMonthCloseSnapshotRepo struct {
	MonthSnapshots
	snapshots   []*MonthSnapshot
	lockedStaff []int64
}

func (r *timestampMonthCloseSnapshotRepo) LockStaffBalanceWrites(_ context.Context, staffID int64) error {
	r.lockedStaff = append(r.lockedStaff, staffID)
	return nil
}

func (r *timestampMonthCloseSnapshotRepo) LatestClosedMonth(context.Context, int64, int, int) (*MonthSnapshot, error) {
	return nil, nil
}

func (r *timestampMonthCloseSnapshotRepo) RecordClosedMonth(_ context.Context, snapshot MonthSnapshot) (MonthSnapshot, error) {
	r.snapshots = append(r.snapshots, &snapshot)
	return snapshot, nil
}

type timestampMonthCloseMonthService struct {
	WorkTimeMonthService
}

func (timestampMonthCloseMonthService) GetMonthSummaryAtMonthEnd(context.Context, int64, int, int) (*MonthSummary, error) {
	return &MonthSummary{}, nil
}

type timestampMonthCloseStaffLister struct{}

func (timestampMonthCloseStaffLister) ListStaffIDs(context.Context) ([]int64, error) {
	return []int64{42, 41}, nil
}

func TestStaffMonthCloseService_UsesOneTimestampForSchoolWideClose(t *testing.T) {
	t.Parallel()

	repo := &timestampMonthCloseSnapshotRepo{}
	service := NewStaffMonthCloseService(
		repo,
		timestampMonthCloseMonthService{},
		timestampMonthCloseStaffLister{},
		&wtmMockSettings{accountStart: "2025-01-01"},
		nil,
	)

	result, err := service.CloseMonth(context.Background(), 77, 2025, 8, "Lohnlauf")

	require.NoError(t, err)
	require.Len(t, result.Snapshots, 2)
	require.Equal(t, []int64{41, 42}, repo.lockedStaff, "school-wide close must acquire staff locks in ascending order")
	require.False(t, result.Snapshots[0].ClosedAt.IsZero())
	require.Equal(t, result.Snapshots[0].ClosedAt, result.Snapshots[1].ClosedAt)
}

type failedMonthCloseStaffQuery struct{ err error }

func (q failedMonthCloseStaffQuery) ListStaffIDs(context.Context) ([]int64, error) {
	return nil, q.err
}

func TestStaffMonthCloseService_StaffQueryFailureDoesNotWrite(t *testing.T) {
	t.Parallel()
	failure := errors.New("staff query unavailable")
	repo := &timestampMonthCloseSnapshotRepo{}
	service := NewStaffMonthCloseService(repo, timestampMonthCloseMonthService{}, failedMonthCloseStaffQuery{err: failure}, &wtmMockSettings{accountStart: "2025-01-01"}, nil)
	result, err := service.CloseMonth(context.Background(), 77, 2025, 8, "Lohnlauf")
	require.Nil(t, result)
	require.ErrorIs(t, err, failure)
	require.EqualError(t, err, "failed to list staff for month close: staff query unavailable")
	require.Empty(t, repo.lockedStaff)
	require.Empty(t, repo.snapshots)
}
