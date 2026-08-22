package active

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	t.Parallel()

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

func TestCalendarMonthHasEnded_RequiresFollowingDay(t *testing.T) {
	t.Parallel()

	key := monthKey{Year: 2026, Month: 7}

	assert.False(t, calendarMonthHasEnded(key, timezone.NewDate(2026, time.July, 31)),
		"the final calendar day is still running")
	assert.True(t, calendarMonthHasEnded(key, timezone.NewDate(2026, time.August, 1)),
		"the month is over on the following day")
}

type broadcastMonthCloseSnapshotRepo struct {
	activeModels.StaffMonthBalanceSnapshotRepository
	snapshot *activeModels.StaffMonthBalanceSnapshot
}

func (r *broadcastMonthCloseSnapshotRepo) LockStaffBalanceWrites(context.Context, int64) error {
	return nil
}

func (r *broadcastMonthCloseSnapshotRepo) GetLatestClosedThrough(context.Context, int64, int, int) (*activeModels.StaffMonthBalanceSnapshot, error) {
	return r.snapshot, nil
}

func (r *broadcastMonthCloseSnapshotRepo) Create(context.Context, *activeModels.StaffMonthBalanceSnapshot) error {
	return nil
}

func (r *broadcastMonthCloseSnapshotRepo) UpdateColumns(context.Context, *activeModels.StaffMonthBalanceSnapshot, ...string) (int64, error) {
	return 1, nil
}

func newBroadcastMonthCloseService(
	repo *broadcastMonthCloseSnapshotRepo,
	broadcaster realtime.Broadcaster,
) StaffMonthCloseService {
	events := []string{}
	service := NewStaffMonthCloseService(
		repo,
		&recordingMonthCloseMonthService{events: &events},
		&recordingMonthCloseStaffLister{events: &events},
		&wtmMockSettings{accountStart: "2025-01-01"},
		slog.New(slog.DiscardHandler),
	)
	service.(interface {
		SetBroadcaster(realtime.Broadcaster)
	}).SetBroadcaster(broadcaster)
	return service
}

func assertMonthCloseBroadcastAfterCommit(
	t *testing.T,
	broadcaster *testpkg.RecordingBroadcaster,
	commit func(),
) {
	t.Helper()
	assert.Empty(t, broadcaster.Events())

	commit()

	require.Len(t, broadcaster.EventsOfType(realtime.EventStaffTimeTrackingChanged), 1)
	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
}

func TestStaffMonthCloseService_CloseBroadcastsAfterCommit(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	service := newBroadcastMonthCloseService(&broadcastMonthCloseSnapshotRepo{}, broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	result, err := service.CloseMonth(ctx, 42, 2025, 8, "Monatsabschluss")

	require.NoError(t, err)
	assert.Equal(t, 1, result.ClosedStaff)
	assertMonthCloseBroadcastAfterCommit(t, broadcaster, commit)
}

func TestStaffMonthCloseService_ReopenBroadcastsAfterCommit(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	service := newBroadcastMonthCloseService(&broadcastMonthCloseSnapshotRepo{
		snapshot: &activeModels.StaffMonthBalanceSnapshot{
			StaffID: 41,
			Year:    2025,
			Month:   8,
		},
	}, broadcaster)
	ctx, commit := tenant.WithAfterCommitHooksForTest(
		tenant.WithTenantID(context.Background(), 42),
	)

	err := service.ReopenMonth(ctx, 41, 42, 2025, 8, "Korrektur")

	require.NoError(t, err)
	assertMonthCloseBroadcastAfterCommit(t, broadcaster, commit)
}
