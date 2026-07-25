package active

import (
	"context"
	"log/slog"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type concurrentMonthCloseSnapshotRepo struct {
	activeModels.StaffMonthBalanceSnapshotRepository
	events   *[]string
	snapshot *activeModels.StaffMonthBalanceSnapshot
}

func (r *concurrentMonthCloseSnapshotRepo) LockStaffBalanceWrites(context.Context, int64) error {
	*r.events = append(*r.events, "lock")
	r.snapshot = &activeModels.StaffMonthBalanceSnapshot{
		StaffID: 41,
		Year:    2025,
		Month:   8,
	}
	return nil
}

func (r *concurrentMonthCloseSnapshotRepo) GetLatestClosedThrough(context.Context, int64, int, int) (*activeModels.StaffMonthBalanceSnapshot, error) {
	*r.events = append(*r.events, "snapshot")
	return r.snapshot, nil
}

func (r *concurrentMonthCloseSnapshotRepo) GetByMonth(context.Context, int, int) ([]*activeModels.StaffMonthBalanceSnapshot, error) {
	*r.events = append(*r.events, "pre-lock-snapshots")
	return nil, nil
}

func (r *concurrentMonthCloseSnapshotRepo) Create(context.Context, *activeModels.StaffMonthBalanceSnapshot) error {
	*r.events = append(*r.events, "create")
	return nil
}

type recordingMonthCloseMonthService struct {
	WorkTimeMonthService
	events *[]string
}

func (s *recordingMonthCloseMonthService) GetMonthSummaryAtMonthEnd(context.Context, int64, int, int) (*MonthSummary, error) {
	*s.events = append(*s.events, "summary")
	return &MonthSummary{}, nil
}

type recordingMonthCloseStaffLister struct {
	events *[]string
}

func (l *recordingMonthCloseStaffLister) ListAllWithPerson(context.Context) ([]*userModels.Staff, error) {
	*l.events = append(*l.events, "staff")
	staff := &userModels.Staff{}
	staff.ID = 41
	return []*userModels.Staff{staff}, nil
}

func TestStaffMonthCloseService_RechecksIdempotencyAfterLock(t *testing.T) {
	events := []string{}
	repo := &concurrentMonthCloseSnapshotRepo{events: &events}
	service := NewStaffMonthCloseService(
		repo,
		&recordingMonthCloseMonthService{events: &events},
		&recordingMonthCloseStaffLister{events: &events},
		&wtmMockSettings{accountStart: "2025-01-01"},
		slog.New(slog.DiscardHandler),
	)

	result, err := service.CloseMonth(context.Background(), 42, 2025, 8, "Monatsabschluss")

	require.NoError(t, err)
	assert.Zero(t, result.ClosedStaff)
	assert.Equal(t, 1, result.SkippedStaff)
	assert.Empty(t, result.Snapshots)
	assert.Equal(t, []string{"staff", "lock", "snapshot"}, events)
}
