// Hermetic tests for the Timetable owner's staff availability pool (#1884): categorization of
// staff against one block's window from shifts and same-day assignments.
package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// poolSetup bundles the per-test fixtures: one tenant, one room, two staff,
// two same-day blocks (source 14:00–15:00, target 14:30–15:30, overlapping).
type poolSetup struct {
	db        *bun.DB
	ctx       context.Context
	tenantID  int64
	detection timetable.ConflictDetectionCapability
	roomID    int64
	staffID   int64 // planned on the source block by the tests that need it
	otherID   int64 // free second staff member
	source    *scheduleModels.ActivityInstance
	target    *scheduleModels.ActivityInstance
}

// poolTestDate is a fixed Monday; the pool reads one day and its week.
func poolTestDate() calendar.Date {
	return calendar.NewDate(2026, time.June, 22)
}

func parsePoolClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+hhmm)
	require.NoError(t, err)
	return parsed
}

func createPoolInstance(t *testing.T, s *poolSetup, title, startHHMM, endHHMM, status string) *scheduleModels.ActivityInstance {
	t.Helper()
	row := &scheduleModels.ActivityInstance{
		Date:      scheduleModels.Date(poolTestDate()),
		Title:     title,
		StartTime: parsePoolClock(t, startHHMM),
		EndTime:   parsePoolClock(t, endHHMM),
		RoomID:    s.roomID,
		Status:    status,
	}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).Exec(s.ctx)
	require.NoError(t, err)
	return row
}

func createPoolStaffRow(t *testing.T, s *poolSetup, instanceID, staffID int64, mutate func(*scheduleModels.InstanceStaff)) *scheduleModels.InstanceStaff {
	t.Helper()
	row := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	if mutate != nil {
		mutate(row)
	}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(s.ctx)
	require.NoError(t, err)
	return row
}

func makePoolSetup(t *testing.T) *poolSetup {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos, err := repositories.NewTimetableTestRepositories(db)
	require.NoError(t, err)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	suffix := time.Now().UnixNano()
	s := &poolSetup{
		db:        db,
		ctx:       testpkg.TenantContext(tenantID),
		tenantID:  tenantID,
		detection: newConflictDetection(t, repos),
	}
	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Pool-Room-%d", suffix))
	s.roomID = room.ID
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Mona", fmt.Sprintf("Pool-%d", suffix))
	other := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Otto", fmt.Sprintf("Other-%d", suffix))
	s.staffID = staff.ID
	s.otherID = other.ID
	s.source = createPoolInstance(t, s, "Schulhof", "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
	s.target = createPoolInstance(t, s, "Mensa", "14:30", "15:30", scheduleModels.InstanceStatusPlanned)
	return s
}

func poolEntryOf(t *testing.T, pool timetable.StaffPool, staffID int64) timetable.StaffPoolEntry {
	t.Helper()
	for _, entry := range pool.Entries {
		if entry.StaffID == staffID {
			return entry
		}
	}
	t.Fatalf("staff %d not in pool", staffID)
	return timetable.StaffPoolEntry{}
}

func createPoolShift(t *testing.T, s *poolSetup, staffID int64, startHHMM, endHHMM string, cancelled bool) *scheduleModels.StaffShift {
	t.Helper()
	shift := &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      scheduleModels.Date(poolTestDate()),
		StartTime: parsePoolClock(t, startHHMM),
		EndTime:   parsePoolClock(t, endHHMM),
		Cancelled: cancelled,
		CreatedBy: staffID,
	}
	shift.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(shift).ModelTableExpr(`schedule.staff_shifts`).Exec(s.ctx)
	require.NoError(t, err)
	return shift
}

// TestStaffPoolForInstance_Categories covers every category in one scene:
// target 14:30–15:30; five staff members in distinct situations.
func TestStaffPoolForInstance_Categories(t *testing.T) {
	t.Parallel()

	s := makePoolSetup(t)
	db := s.db
	suffix := time.Now().UnixNano()

	free := testpkg.CreateTestStaffForTenant(t, db, s.tenantID, "Frida", fmt.Sprintf("Frei-%d", suffix))
	partial := testpkg.CreateTestStaffForTenant(t, db, s.tenantID, "Paula", fmt.Sprintf("Teilweise-%d", suffix))
	absent := testpkg.CreateTestStaffForTenant(t, db, s.tenantID, "Abbi", fmt.Sprintf("Abwesend-%d", suffix))

	// staffID: assigned to the overlapping source block, full shift.
	createPoolStaffRow(t, s, s.source.ID, s.staffID, nil)
	createPoolShift(t, s, s.staffID, "08:00", "16:00", false)
	// otherID: assigned to the target itself.
	createPoolStaffRow(t, s, s.target.ID, s.otherID, nil)
	// free: on shift covering the whole window, no assignment.
	createPoolShift(t, s, free.ID, "12:00", "16:00", false)
	// partial: shift overlaps the window but does not cover it (ends 15:00).
	createPoolShift(t, s, partial.ID, "12:00", "15:00", false)
	// absent: assigned to source but marked absent day-wide.
	createPoolStaffRow(t, s, s.source.ID, absent.ID, func(row *scheduleModels.InstanceStaff) {
		row.IsAbsent = true
		row.AbsenceReason = testpkg.StrPtr("krank")
	})
	createPoolShift(t, s, absent.ID, "08:00", "16:00", false)

	pool, err := s.detection.StaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	assert.True(t, pool.DienstplanInUse)
	assert.Equal(t, s.target.ID, pool.InstanceID)

	moving := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, timetable.StaffPoolAssignedElsewhere, moving.Category)
	require.Len(t, moving.Assignments, 1)
	assert.Equal(t, s.source.ID, moving.Assignments[0].InstanceID)
	assert.Equal(t, "Schulhof", moving.Assignments[0].Title)
	assert.Equal(t, "14:00", moving.Assignments[0].StartTime)
	assert.True(t, moving.OnShift)
	assert.True(t, moving.CoversWindow)

	here := poolEntryOf(t, pool, s.otherID)
	assert.Equal(t, timetable.StaffPoolAssignedHere, here.Category)
	assert.False(t, here.OnShift, "no shift seeded for the target member")

	freeEntry := poolEntryOf(t, pool, free.ID)
	assert.Equal(t, timetable.StaffPoolOnShiftFree, freeEntry.Category)
	assert.True(t, freeEntry.CoversWindow)
	assert.Equal(t, []string{"12:00–16:00"}, freeEntry.ShiftWindows)

	partialEntry := poolEntryOf(t, pool, partial.ID)
	assert.Equal(t, timetable.StaffPoolOnShiftFree, partialEntry.Category)
	assert.True(t, partialEntry.OnShift)
	assert.False(t, partialEntry.CoversWindow, "shift ends before the block does")

	absentEntry := poolEntryOf(t, pool, absent.ID)
	assert.Equal(t, timetable.StaffPoolAbsent, absentEntry.Category, "day-wide absence outranks assignments and shifts")
	require.NotNil(t, absentEntry.AbsenceReason)
	assert.Equal(t, "krank", *absentEntry.AbsenceReason)
}

// TestStaffPoolForInstance_NoShiftsMeansDienstplanNotInUse: without any
// shift in the calendar week, not_on_shift carries no signal.
func TestStaffPoolForInstance_NoShiftsMeansDienstplanNotInUse(t *testing.T) {
	t.Parallel()

	s := makePoolSetup(t)

	pool, err := s.detection.StaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	assert.False(t, pool.DienstplanInUse)
	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, timetable.StaffPoolNotOnShift, entry.Category)
	assert.Empty(t, entry.ShiftWindows)
}

// TestStaffPoolForInstance_TouchingWindowsDoNotOverlap: a block ending
// exactly when the target starts does not occupy the person.
func TestStaffPoolForInstance_TouchingWindowsDoNotOverlap(t *testing.T) {
	t.Parallel()

	s := makePoolSetup(t)
	before := createPoolInstance(t, s, "Davor", "13:30", "14:30", scheduleModels.InstanceStatusPlanned)
	createPoolStaffRow(t, s, before.ID, s.staffID, nil)
	createPoolShift(t, s, s.staffID, "12:00", "16:00", false)

	// Target runs 14:30–15:30; the other block ends 14:30 sharp.
	pool, err := s.detection.StaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, timetable.StaffPoolOnShiftFree, entry.Category)
	assert.Empty(t, entry.Assignments)
}

// TestStaffPoolForInstance_CancelledShiftIgnored: a cancelled shift does
// not make someone available.
func TestStaffPoolForInstance_CancelledShiftIgnored(t *testing.T) {
	t.Parallel()

	s := makePoolSetup(t)
	createPoolShift(t, s, s.staffID, "08:00", "16:00", true)

	pool, err := s.detection.StaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, timetable.StaffPoolNotOnShift, entry.Category)
	assert.False(t, entry.OnShift)
}

// TestStaffPoolForInstance_CancelledOnlyWeekMeansDienstplanNotInUse: a week
// whose shifts are ALL cancelled offers nobody a usable shift, so it must
// report dienstplan_in_use=false and show the missing-Dienstplan hint instead
// of misleading not_on_shift categories.
func TestStaffPoolForInstance_CancelledOnlyWeekMeansDienstplanNotInUse(t *testing.T) {
	t.Parallel()

	s := makePoolSetup(t)
	createPoolShift(t, s, s.staffID, "08:00", "16:00", true)
	createPoolShift(t, s, s.otherID, "08:00", "12:00", true)

	pool, err := s.detection.StaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	assert.False(t, pool.DienstplanInUse, "cancelled shifts do not mark the week as planned")
	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, timetable.StaffPoolNotOnShift, entry.Category)
}

func TestStaffPoolForInstance_TerminalBlockAbsenceRemainsDayWide(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		t.Run(status, func(t *testing.T) {
			s := makePoolSetup(t)

			terminal := createPoolInstance(t, s, "Historie", "09:00", "10:00", status)
			createPoolStaffRow(t, s, terminal.ID, s.staffID, func(row *scheduleModels.InstanceStaff) {
				row.IsAbsent = true
				row.AbsenceReason = testpkg.StrPtr("krank")
			})
			// A non-absent terminal assignment is history, not current occupancy.
			createPoolStaffRow(t, s, terminal.ID, s.otherID, nil)
			createPoolShift(t, s, s.staffID, "08:00", "16:00", false)
			createPoolShift(t, s, s.otherID, "08:00", "16:00", false)

			pool, err := s.detection.StaffPoolForInstance(s.ctx, s.target.ID)
			require.NoError(t, err)

			absent := poolEntryOf(t, pool, s.staffID)
			assert.Equal(t, timetable.StaffPoolAbsent, absent.Category)
			require.NotNil(t, absent.AbsenceReason)
			assert.Equal(t, "krank", *absent.AbsenceReason)

			historicalOnly := poolEntryOf(t, pool, s.otherID)
			assert.Equal(t, timetable.StaffPoolOnShiftFree, historicalOnly.Category)
			assert.Empty(t, historicalOnly.Assignments)
		})
	}
}
