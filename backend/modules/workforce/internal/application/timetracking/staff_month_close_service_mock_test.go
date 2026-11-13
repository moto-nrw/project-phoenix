package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/moto-ogs/project-phoenix/tenant/tenanttest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

type concurrentMonthCloseSnapshotRepo struct {
	MonthSnapshots
	events   *[]string
	snapshot *MonthSnapshot
}

func (r *concurrentMonthCloseSnapshotRepo) LockStaffBalanceWrites(context.Context, int64) error {
	*r.events = append(*r.events, "lock")
	r.snapshot = &MonthSnapshot{
		StaffID: 41,
		Year:    2025,
		Month:   8,
	}
	return nil
}

func (r *concurrentMonthCloseSnapshotRepo) LatestClosedMonth(context.Context, int64, int, int) (*MonthSnapshot, error) {
	*r.events = append(*r.events, "snapshot")
	return r.snapshot, nil
}

func (r *concurrentMonthCloseSnapshotRepo) ClosedMonthSnapshotsForStaff(context.Context, []int64) ([]*MonthSnapshot, error) {
	*r.events = append(*r.events, "snapshot")
	return snapshotList(r.snapshot), nil
}

// snapshotList is the batch read's answer for a mock holding at most one
// active snapshot.
func snapshotList(snapshot *MonthSnapshot) []*MonthSnapshot {
	if snapshot == nil {
		return nil
	}
	return []*MonthSnapshot{snapshot}
}

func (r *concurrentMonthCloseSnapshotRepo) ClosedMonthSnapshots(context.Context, int, int) ([]*MonthSnapshot, error) {
	*r.events = append(*r.events, "pre-lock-snapshots")
	return nil, nil
}

func (r *concurrentMonthCloseSnapshotRepo) RecordClosedMonth(_ context.Context, value MonthSnapshot) (MonthSnapshot, error) {
	*r.events = append(*r.events, "create")
	return value, nil
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

func (l *recordingMonthCloseStaffLister) ListMonthCloseStaff(context.Context) ([]workforce.MonthCloseStaff, error) {
	*l.events = append(*l.events, "staff")
	return []workforce.MonthCloseStaff{{ID: 41}}, nil
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

	assert.False(t, calendarMonthHasEnded(key, NewDate(2026, time.July, 31)),
		"the final calendar day is still running")
	assert.True(t, calendarMonthHasEnded(key, NewDate(2026, time.August, 1)),
		"the month is over on the following day")
}

type broadcastMonthCloseSnapshotRepo struct {
	MonthSnapshots
	snapshot *MonthSnapshot
}

func (r *broadcastMonthCloseSnapshotRepo) LockStaffBalanceWrites(context.Context, int64) error {
	return nil
}

func (r *broadcastMonthCloseSnapshotRepo) LatestClosedMonth(context.Context, int64, int, int) (*MonthSnapshot, error) {
	return r.snapshot, nil
}

func (r *broadcastMonthCloseSnapshotRepo) ClosedMonthSnapshotsForStaff(context.Context, []int64) ([]*MonthSnapshot, error) {
	return snapshotList(r.snapshot), nil
}

func (r *broadcastMonthCloseSnapshotRepo) RecordClosedMonth(_ context.Context, value MonthSnapshot) (MonthSnapshot, error) {
	return value, nil
}

func (r *broadcastMonthCloseSnapshotRepo) ReopenMonthSnapshot(context.Context, int64, int64, time.Time, string) (int64, error) {
	return 1, nil
}

func newBroadcastMonthCloseService(
	repo *broadcastMonthCloseSnapshotRepo,
	broadcaster EventPublisher,
) StaffMonthCloseService {
	events := []string{}
	service := NewStaffMonthCloseService(
		repo,
		&recordingMonthCloseMonthService{events: &events},
		&recordingMonthCloseStaffLister{events: &events},
		&wtmMockSettings{accountStart: "2025-01-01"},
		slog.New(slog.DiscardHandler),
		WithMonthCloseEvents(broadcaster),
	)
	return service
}

func assertMonthCloseAnnounced(
	t *testing.T,
	broadcaster *recordingTimeTrackingPublisher,
	commit func(),
) {
	t.Helper()

	commit()

	require.Len(t, broadcaster.tenants, 1)
	assert.Equal(t, int64(42), broadcaster.tenants[0])
}

func TestStaffMonthCloseService_CloseAnnouncesTimeTrackingChange(t *testing.T) {
	t.Parallel()

	broadcaster := &recordingTimeTrackingPublisher{}
	service := newBroadcastMonthCloseService(&broadcastMonthCloseSnapshotRepo{}, broadcaster)
	ctx, commit := tenanttest.WithAfterCommitHooks(
		tenant.WithTenantID(context.Background(), 42),
	)

	result, err := service.CloseMonth(ctx, 42, 2025, 8, "Monatsabschluss")

	require.NoError(t, err)
	assert.Equal(t, 1, result.ClosedStaff)
	assertMonthCloseAnnounced(t, broadcaster, commit)
}

func TestStaffMonthCloseService_ReopenAnnouncesTimeTrackingChange(t *testing.T) {
	t.Parallel()

	broadcaster := &recordingTimeTrackingPublisher{}
	service := newBroadcastMonthCloseService(&broadcastMonthCloseSnapshotRepo{
		snapshot: &MonthSnapshot{
			StaffID: 41,
			Year:    2025,
			Month:   8,
		},
	}, broadcaster)
	ctx, commit := tenanttest.WithAfterCommitHooks(
		tenant.WithTenantID(context.Background(), 42),
	)

	err := service.ReopenMonth(ctx, 41, 42, 2025, 8, "Korrektur")

	require.NoError(t, err)
	assertMonthCloseAnnounced(t, broadcaster, commit)
}

// orderedMonthCloseSnapshotRepo records the close protocol across staff and
// holds the active snapshots the batch read returns.
type orderedMonthCloseSnapshotRepo struct {
	MonthSnapshots
	events    *[]string
	snapshots []*MonthSnapshot
}

func (r *orderedMonthCloseSnapshotRepo) LockStaffBalanceWrites(_ context.Context, staffID int64) error {
	*r.events = append(*r.events, fmt.Sprintf("lock:%d", staffID))
	return nil
}

func (r *orderedMonthCloseSnapshotRepo) ClosedMonthSnapshotsForStaff(_ context.Context, staffIDs []int64) ([]*MonthSnapshot, error) {
	*r.events = append(*r.events, fmt.Sprintf("snapshots:%v", staffIDs))
	return r.snapshots, nil
}

func (r *orderedMonthCloseSnapshotRepo) RecordClosedMonth(_ context.Context, value MonthSnapshot) (MonthSnapshot, error) {
	*r.events = append(*r.events, fmt.Sprintf("create:%d", value.StaffID))
	return value, nil
}

type orderedMonthCloseMonthService struct {
	WorkTimeMonthService
	events *[]string
}

func (s *orderedMonthCloseMonthService) GetMonthSummaryAtMonthEnd(_ context.Context, staffID int64, _, _ int) (*MonthSummary, error) {
	*s.events = append(*s.events, fmt.Sprintf("summary:%d", staffID))
	return &MonthSummary{}, nil
}

// #5821: every balance lock is taken in ascending staff order before the one
// snapshot read; the per-staff checks then run in the same order, so a later
// closed month of the second staff member still fails after the first one's
// close, with the same 409 sentinel and month.
func TestStaffMonthCloseService_LocksAllStaffBeforeOneSnapshotRead(t *testing.T) {
	t.Parallel()

	events := []string{}
	repo := &orderedMonthCloseSnapshotRepo{events: &events, snapshots: []*MonthSnapshot{
		{StaffID: 42, Year: 2025, Month: 9},
	}}
	service := NewStaffMonthCloseService(
		repo,
		&orderedMonthCloseMonthService{events: &events},
		timestampMonthCloseStaffLister{},
		&wtmMockSettings{accountStart: "2025-01-01"},
		slog.New(slog.DiscardHandler),
	)

	_, err := service.CloseMonth(context.Background(), 77, 2025, 8, "Monatsabschluss")

	require.ErrorIs(t, err, ErrLaterMonthClosed)
	assert.Contains(t, err.Error(), "2025-09 must be reopened first")
	assert.Equal(t, []string{"lock:41", "lock:42", "snapshots:[41 42]", "summary:41", "create:41"}, events)
}

// A month already closed for one staff member is skipped while the others
// are closed; an earlier closed month does not count as this month's close.
func TestStaffMonthCloseService_SkipsOnlyStaffWithThisMonthClosed(t *testing.T) {
	t.Parallel()

	events := []string{}
	repo := &orderedMonthCloseSnapshotRepo{events: &events, snapshots: []*MonthSnapshot{
		{StaffID: 41, Year: 2025, Month: 7},
		{StaffID: 41, Year: 2025, Month: 8},
		{StaffID: 42, Year: 2025, Month: 7},
	}}
	service := NewStaffMonthCloseService(
		repo,
		&orderedMonthCloseMonthService{events: &events},
		timestampMonthCloseStaffLister{},
		&wtmMockSettings{accountStart: "2025-01-01"},
		slog.New(slog.DiscardHandler),
	)

	result, err := service.CloseMonth(context.Background(), 77, 2025, 8, "Monatsabschluss")

	require.NoError(t, err)
	assert.Equal(t, 1, result.SkippedStaff)
	assert.Equal(t, 1, result.ClosedStaff)
	require.Len(t, result.Snapshots, 1)
	assert.Equal(t, int64(42), result.Snapshots[0].StaffID)
}
