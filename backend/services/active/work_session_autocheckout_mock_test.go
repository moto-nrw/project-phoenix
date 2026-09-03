package active

// Mock-based unit tests for AutoCheckoutDueSessions (#1798): closing open
// work sessions at the planned shift end from schedule.staff_shifts. Uses
// the func-field mocks from work_session_service_test.go plus a local shift
// repo mock; int64 literals are fake in-memory IDs, not DB rows.

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock for StaffShiftRepository
// ============================================================================

type wsMockStaffShiftRepository struct {
	findByStaffIDsAndDateFunc func(ctx context.Context, staffIDs []int64, date timezone.Date) ([]*scheduleModels.StaffShift, error)
}

func (m *wsMockStaffShiftRepository) Create(_ context.Context, _ *scheduleModels.StaffShift) error {
	return nil
}

func (m *wsMockStaffShiftRepository) FindByID(_ context.Context, _ any) (*scheduleModels.StaffShift, error) {
	return nil, errors.New("not implemented")
}

func (m *wsMockStaffShiftRepository) Update(_ context.Context, _ *scheduleModels.StaffShift) error {
	return nil
}

func (m *wsMockStaffShiftRepository) Delete(_ context.Context, _ any) error { return nil }

func (m *wsMockStaffShiftRepository) FindByDateRange(_ context.Context, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

func (m *wsMockStaffShiftRepository) FindByStaffAndDateRange(_ context.Context, _ int64, _, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

func (m *wsMockStaffShiftRepository) FindByStaffIDsAndDate(ctx context.Context, staffIDs []int64, date timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if m.findByStaffIDsAndDateFunc != nil {
		return m.findByStaffIDsAndDateFunc(ctx, staffIDs, date)
	}
	return nil, nil
}

func (m *wsMockStaffShiftRepository) FindByStaffIDsAndDates(_ context.Context, _ []int64, _ []timezone.Date) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

func (m *wsMockStaffShiftRepository) FindByOriginShiftID(_ context.Context, _ int64) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

// Interface-compile stubs for the generic methods surfaced for the #1843 sick
// cascade; auto-checkout never exercises them.
func (m *wsMockStaffShiftRepository) List(_ context.Context, _ map[string]any) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

func (m *wsMockStaffShiftRepository) ListWithOptions(_ context.Context, _ *modelBase.QueryOptions) ([]*scheduleModels.StaffShift, error) {
	return nil, nil
}

func (m *wsMockStaffShiftRepository) UpdateColumns(_ context.Context, _ *scheduleModels.StaffShift, _ ...string) (int64, error) {
	return 0, nil
}

func (m *wsMockStaffShiftRepository) FindUsedCalendarWeeks(_ context.Context, _, _ timezone.Date) ([]timezone.Date, error) {
	return nil, nil
}

func (m *wsMockStaffShiftRepository) DeleteUpcomingByStaffID(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *wsMockStaffShiftRepository) BulkCreate(context.Context, []*scheduleModels.StaffShift) error {
	return nil
}

func (m *wsMockStaffShiftRepository) DeleteNonDetachedBySeriesFrom(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

func (m *wsMockStaffShiftRepository) RepointDetachedSeriesFrom(context.Context, int64, int64, timezone.Date) (int64, error) {
	return 0, nil
}

// ============================================================================
// Helpers
// ============================================================================

// wallClock builds a wall-clock time for TIME-column fields.
func wallClock(hour, minute int) time.Time {
	return time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC)
}

// autoCheckoutFixture wires a service whose open-session list and shift
// lookup are canned. yesterday keeps every shift end safely in the past.
func autoCheckoutFixture(openSessions []*activeModels.WorkSession, shifts []*scheduleModels.StaffShift) (*workSessionService, *wsMockWorkSessionRepository, *wsMockWorkSessionBreakRepository, *wsMockWorkSessionEditRepository, *wsMockStaffShiftRepository) {
	service, sessionRepo, breakRepo, auditRepo, _ := wsCreateTestService()
	shiftRepo := &wsMockStaffShiftRepository{}
	service.SetStaffShiftRepo(shiftRepo)

	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return openSessions, nil
	}
	sessionRepo.lockOpenByIDFunc = func(_ context.Context, id int64) (*activeModels.WorkSession, error) {
		for _, session := range openSessions {
			if session != nil && session.ID == id && session.CheckOutTime == nil {
				return session, nil
			}
		}
		return nil, sql.ErrNoRows
	}
	shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return shifts, nil
	}
	return service, sessionRepo, breakRepo, auditRepo, shiftRepo
}

func shiftFor(staffID int64, date timezone.Date, startHour, endHour int) *scheduleModels.StaffShift {
	return &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      date,
		StartTime: wallClock(startHour, 0),
		EndTime:   wallClock(endHour, 0),
	}
}

// ============================================================================
// Tests
// ============================================================================

func TestAutoCheckout_ClosesDueSessionAtShiftEnd(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour), // 08:00
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, _, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)

	var closedID int64
	var closedAt time.Time
	var closedAuto bool
	sessionRepo.closeSessionFunc = func(_ context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error) {
		closedID, closedAt, closedAuto = id, checkOutTime, autoCheckedOut
		return true, nil
	}
	var auditEdits []*auditModels.WorkSessionEdit
	auditRepo.createBatchFunc = func(_ context.Context, edits []*auditModels.WorkSessionEdit) error {
		auditEdits = append(auditEdits, edits...)
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, int64(42), closedID)
	assert.True(t, closedAuto, "session must be marked AutoCheckedOut")
	assert.Equal(t, shift.EndInstant(), closedAt, "checkout must land exactly on the planned shift end")

	require.Len(t, auditEdits, 1)
	edit := auditEdits[0]
	assert.Equal(t, auditModels.SystemEditorID, edit.EditedBy, "audit actor must be the system sentinel")
	assert.Equal(t, auditModels.FieldCheckOutTime, edit.FieldName)
	require.NotNil(t, edit.NewValue)
	assert.Equal(t, shift.EndInstant().Format(time.RFC3339), *edit.NewValue)
	require.NoError(t, edit.Validate(), "system edits must pass model validation")
}

func TestAutoCheckout_SkipsCheckInAfterShiftEnd(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(17 * time.Hour), // 17:00, after shift end
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, _, _, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("session checked in after shift end must not be closed")
		return true, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_SkipsReopenedAfterShiftEndFromLockedRow(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	listedSession := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	listedSession.ID = 42
	reopenedAt := yesterday.BerlinMidnight().Add(16*time.Hour + 20*time.Minute)
	lockedSession := *listedSession
	lockedSession.ReopenedAt = &reopenedAt
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, _, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{listedSession},
		[]*scheduleModels.StaffShift{shift},
	)
	sessionRepo.lockOpenByIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return &lockedSession, nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("same-day reopened session must not be closed against the stale shift end")
		return true, nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("same-day reopened session must not be audited as auto-checked-out")
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_SkipsSessionWithoutShift(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	session := &activeModels.WorkSession{
		StaffID:     7,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42

	service, sessionRepo, _, _, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		nil, // no shifts planned
	)
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("session without a planned shift must not be closed")
		return true, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_MultipleShiftsLatestEndWins(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	morning := shiftFor(staffID, yesterday, 8, 12)
	afternoon := shiftFor(staffID, yesterday, 13, 17)

	service, sessionRepo, _, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{morning, afternoon},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error { return nil }

	var closedAt time.Time
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, checkOutTime time.Time, _ bool) (bool, error) {
		closedAt = checkOutTime
		return true, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, afternoon.EndInstant(), closedAt, "the latest shift end of the day must win")
}

func TestAutoCheckout_GraceNotElapsed(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, _, _, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("session must not be closed while the grace window is still open")
		return true, nil
	}

	// Grace long enough that yesterday-16:00 + grace is still in the future.
	count, err := service.AutoCheckoutDueSessions(context.Background(), 14*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_EndsActiveBreakAfterCloseClaim(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error { return nil }

	activeBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(-time.Hour)}
	activeBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{activeBreak}, nil
	}
	breakEnded := false
	closed := false
	breakRepo.endBreakFunc = func(_ context.Context, id int64, _ time.Time, _ int) error {
		assert.Equal(t, int64(9), id)
		assert.True(t, closed, "break must be ended only after auto-checkout wins the close race")
		breakEnded = true
		return nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		assert.False(t, breakEnded, "break must not be ended before the session close is claimed")
		closed = true
		return true, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.True(t, closed)
	assert.True(t, breakEnded)
}

func TestAutoCheckout_ReturnsErrorWhenActiveBreakEndFailsAfterClose(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("failed break close must not write an auto-checkout audit entry")
		return nil
	}
	closed := false
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		closed = true
		return true, nil
	}

	activeBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(-time.Hour)}
	activeBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{activeBreak}, nil
	}
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return errors.New("break row locked")
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.True(t, closed, "auto-checkout must claim the session before mutating breaks")
}

func TestAutoCheckout_ReturnsErrorWhenActiveBreakRecalcFailsAfterClose(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("failed break-minute recalc must not write an auto-checkout audit entry")
		return nil
	}
	var closed bool
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		closed = true
		return true, nil
	}

	activeBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(-time.Hour)}
	activeBreak.ID = 9
	getBreaksCalls := 0
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		getBreaksCalls++
		if getBreaksCalls <= 2 {
			return []*activeModels.WorkSessionBreak{activeBreak}, nil
		}
		return nil, errors.New("break-minute recalc failed")
	}
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.True(t, closed, "auto-checkout must claim the session before mutating breaks")
	assert.Equal(t, 3, getBreaksCalls)
}

func TestAutoCheckout_BreakCappedAtShiftEnd(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error { return nil }
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
	}

	// Break started 30 min before the planned end and was never ended — a
	// wall-clock "now" end would land after check_out_time.
	activeBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(-30 * time.Minute)}
	activeBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{activeBreak}, nil
	}
	var endedAt time.Time
	var endedDuration int
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, at time.Time, duration int) error {
		endedAt, endedDuration = at, duration
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, shift.EndInstant(), endedAt, "break must end at the planned shift end, not wall-clock now")
	assert.Equal(t, 30, endedDuration)
}

func TestAutoCheckout_SkipsBreakStartedAfterShiftEnd(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("late-break skip must not write an auto-checkout audit entry")
		return nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("late-break session must not be auto-closed at planned end")
		return false, nil
	}

	// Break started during the grace window, after the planned end.
	activeBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(10 * time.Minute)}
	activeBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{activeBreak}, nil
	}
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		t.Fatal("late break must stay open for manual checkout or nightly cleanup")
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_SkipsCompletedBreakStartedAfterShiftEnd(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("late completed break skip must not write an auto-checkout audit entry")
		return nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("late completed break session must not be auto-closed at planned end")
		return false, nil
	}

	endedAt := shift.EndInstant().Add(20 * time.Minute)
	completedBreak := &activeModels.WorkSessionBreak{
		StartedAt: shift.EndInstant().Add(10 * time.Minute),
		EndedAt:   &endedAt,
	}
	completedBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{completedBreak}, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_SkipsCompletedBreakEndedAfterShiftEnd(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("break crossing planned end must not write an auto-checkout audit entry")
		return nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		t.Fatal("break crossing planned end must not auto-close at planned end")
		return false, nil
	}

	endedAt := shift.EndInstant().Add(10 * time.Minute)
	completedBreak := &activeModels.WorkSessionBreak{
		StartedAt: shift.EndInstant().Add(-10 * time.Minute),
		EndedAt:   &endedAt,
	}
	completedBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{completedBreak}, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_RechecksBreaksAfterCloseBeforeAudit(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("late break discovered after close must roll back before audit")
		return nil
	}

	lateBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(10 * time.Minute)}
	lateBreak.ID = 9
	getBreaksCalls := 0
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		getBreaksCalls++
		if getBreaksCalls == 1 {
			return nil, nil
		}
		return []*activeModels.WorkSessionBreak{lateBreak}, nil
	}
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		t.Fatal("late break must not be ended at a checkout time before it started")
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "late break detected")
	assert.Equal(t, 0, count)
	assert.Equal(t, 2, getBreaksCalls)
}

func TestAutoCheckout_AuditFailureReturnsError(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, _, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		return errors.New("insert failed")
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.Error(t, err, "an unaudited system checkout must not commit silently")
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_SkipsAuditWhenCloseRacesWithManualCheckout(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, breakRepo, auditRepo, _ := autoCheckoutFixture(
		[]*activeModels.WorkSession{session},
		[]*scheduleModels.StaffShift{shift},
	)
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return false, nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error {
		t.Fatal("auto-checkout must not audit a row it did not close")
		return nil
	}
	activeBreak := &activeModels.WorkSessionBreak{StartedAt: shift.EndInstant().Add(-time.Hour)}
	activeBreak.ID = 9
	breakRepo.getBySessionIDFunc = func(_ context.Context, _ int64) ([]*activeModels.WorkSessionBreak, error) {
		return []*activeModels.WorkSessionBreak{activeBreak}, nil
	}
	breakRepo.endBreakFunc = func(_ context.Context, _ int64, _ time.Time, _ int) error {
		t.Fatal("auto-checkout must not mutate breaks after losing the close race")
		return nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_EndsActiveSupervisions(t *testing.T) {
	t.Parallel()

	yesterday := timezone.TodayDate().AddDays(-1)
	staffID := int64(7)
	session := &activeModels.WorkSession{
		StaffID:     staffID,
		Date:        yesterday,
		CheckInTime: yesterday.BerlinMidnight().Add(8 * time.Hour),
	}
	session.ID = 42
	shift := shiftFor(staffID, yesterday, 8, 16)

	service, sessionRepo, _, auditRepo, supervisorRepo := wsCreateTestService()
	shiftRepo := &wsMockStaffShiftRepository{}
	service.SetStaffShiftRepo(shiftRepo)
	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		return []*activeModels.WorkSession{session}, nil
	}
	sessionRepo.lockOpenByIDFunc = func(_ context.Context, _ int64) (*activeModels.WorkSession, error) {
		return session, nil
	}
	shiftRepo.findByStaffIDsAndDateFunc = func(_ context.Context, _ []int64, _ timezone.Date) ([]*scheduleModels.StaffShift, error) {
		return []*scheduleModels.StaffShift{shift}, nil
	}
	sessionRepo.closeSessionFunc = func(_ context.Context, _ int64, _ time.Time, _ bool) (bool, error) {
		return true, nil
	}
	auditRepo.createBatchFunc = func(_ context.Context, _ []*auditModels.WorkSessionEdit) error { return nil }

	var endedStaffID int64
	supervisorRepo.endAllActiveByStaffIDFunc = func(_ context.Context, id int64) (int, error) {
		endedStaffID = id
		return 1, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, staffID, endedStaffID, "auto-checkout must end open supervisions like manual checkout")
}

func TestAutoCheckout_NoShiftRepoIsNoop(t *testing.T) {
	t.Parallel()

	service, sessionRepo, _, _, _ := wsCreateTestService()
	sessionRepo.getOpenSessionsFunc = func(_ context.Context, _ timezone.Date) ([]*activeModels.WorkSession, error) {
		t.Fatal("open sessions must not be queried without a shift repo")
		return nil, nil
	}

	count, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAutoCheckout_QueriesOpenSessionsIncludingToday(t *testing.T) {
	t.Parallel()

	service, sessionRepo, _, _, _ := autoCheckoutFixture(nil, nil)
	service.nowFunc = func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) }

	var beforeDate timezone.Date
	sessionRepo.getOpenSessionsFunc = func(_ context.Context, d timezone.Date) ([]*activeModels.WorkSession, error) {
		beforeDate = d
		return nil, nil
	}

	_, err := service.AutoCheckoutDueSessions(context.Background(), 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, timezone.NewDate(2026, 8, 24).AddDays(1), beforeDate,
		"must pass tomorrow so today's open sessions are included (GetOpenSessions filters date < beforeDate)")
}

// FindByStaffIDsAndDateRange satisfies the batched interface method (#1417); this mock
// exercises the single-staff path only.
func (m *wsMockStaffShiftRepository) FindByStaffIDsAndDateRange(context.Context, []int64, timezone.Date, timezone.Date) (map[int64][]*scheduleModels.StaffShift, error) {
	return nil, nil
}
