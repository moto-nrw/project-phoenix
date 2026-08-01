package active

import (
	"context"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
)

type timestampMonthCloseSnapshotRepo struct {
	activeModels.StaffMonthBalanceSnapshotRepository
	snapshots []*activeModels.StaffMonthBalanceSnapshot
}

func (r *timestampMonthCloseSnapshotRepo) LockStaffBalanceWrites(context.Context, int64) error {
	return nil
}

func (r *timestampMonthCloseSnapshotRepo) GetLatestClosedThrough(context.Context, int64, int, int) (*activeModels.StaffMonthBalanceSnapshot, error) {
	return nil, nil
}

func (r *timestampMonthCloseSnapshotRepo) Create(_ context.Context, snapshot *activeModels.StaffMonthBalanceSnapshot) error {
	r.snapshots = append(r.snapshots, snapshot)
	return nil
}

type timestampMonthCloseMonthService struct {
	WorkTimeMonthService
}

func (timestampMonthCloseMonthService) GetMonthSummaryAtMonthEnd(context.Context, int64, int, int) (*MonthSummary, error) {
	return &MonthSummary{}, nil
}

type timestampMonthCloseStaffLister struct{}

func (timestampMonthCloseStaffLister) ListAllWithPerson(context.Context) ([]*userModels.Staff, error) {
	first := &userModels.Staff{}
	first.ID = 41
	second := &userModels.Staff{}
	second.ID = 42
	return []*userModels.Staff{first, second}, nil
}

func TestStaffMonthCloseService_UsesOneTimestampForSchoolWideClose(t *testing.T) {
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
	require.False(t, result.Snapshots[0].ClosedAt.IsZero())
	require.Equal(t, result.Snapshots[0].ClosedAt, result.Snapshots[1].ClosedAt)
}
