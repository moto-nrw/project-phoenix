// Hermetic tests for the staff availability pool (#1884): categorization of
// staff against one block's window from shifts and same-day assignments.
package schedule_test

import (
	"fmt"
	"testing"
	"time"

	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func poolEntryOf(t *testing.T, pool *scheduleSvc.StaffPoolResult, staffID int64) scheduleSvc.StaffPoolEntry {
	t.Helper()
	for _, entry := range pool.Entries {
		if entry.StaffID == staffID {
			return entry
		}
	}
	t.Fatalf("staff %d not in pool", staffID)
	return scheduleSvc.StaffPoolEntry{}
}

func createPoolShift(t *testing.T, s *moveSetup, staffID int64, startHHMM, endHHMM string, cancelled bool) *scheduleModels.StaffShift {
	t.Helper()
	shift := &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      moveTestDate(),
		StartTime: parseMoveClock(t, startHHMM),
		EndTime:   parseMoveClock(t, endHHMM),
		Cancelled: cancelled,
		CreatedBy: staffID,
	}
	shift.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(shift).ModelTableExpr(`schedule.staff_shifts`).Exec(s.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.staff_shifts", shift.ID) })
	return shift
}

// TestGetStaffPoolForInstance_Categories covers every category in one scene:
// target 14:30–15:30; five staff members in distinct situations.
func TestGetStaffPoolForInstance_Categories(t *testing.T) {
	s := makeMoveSetup(t)
	defer s.cleanup(t)
	db := s.db
	suffix := time.Now().UnixNano()

	free := testpkg.CreateTestStaffForTenant(t, db, s.tenantID, "Frida", fmt.Sprintf("Frei-%d", suffix))
	partial := testpkg.CreateTestStaffForTenant(t, db, s.tenantID, "Paula", fmt.Sprintf("Teilweise-%d", suffix))
	absent := testpkg.CreateTestStaffForTenant(t, db, s.tenantID, "Abbi", fmt.Sprintf("Abwesend-%d", suffix))
	t.Cleanup(func() {
		testpkg.CleanupActivityFixturesForTenant(t, db, s.tenantID, free.ID, partial.ID, absent.ID)
	})

	// staffID: assigned to the overlapping source block, full shift.
	createMoveStaffRow(t, s, s.source.ID, s.staffID, nil)
	createPoolShift(t, s, s.staffID, "08:00", "16:00", false)
	// otherID: assigned to the target itself.
	createMoveStaffRow(t, s, s.target.ID, s.otherID, nil)
	// free: on shift covering the whole window, no assignment.
	createPoolShift(t, s, free.ID, "12:00", "16:00", false)
	// partial: shift overlaps the window but does not cover it (ends 15:00).
	createPoolShift(t, s, partial.ID, "12:00", "15:00", false)
	// absent: assigned to source but marked absent day-wide.
	createMoveStaffRow(t, s, s.source.ID, absent.ID, func(row *scheduleModels.InstanceStaff) {
		row.IsAbsent = true
		row.AbsenceReason = testpkg.StrPtr("krank")
	})
	createPoolShift(t, s, absent.ID, "08:00", "16:00", false)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)
	pool, err := serviceFactory.TimetableData.GetStaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	assert.True(t, pool.DienstplanInUse)
	assert.Equal(t, s.target.ID, pool.Instance.ID)

	moving := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, scheduleSvc.StaffPoolAssignedElsewhere, moving.Category)
	require.Len(t, moving.Assignments, 1)
	assert.Equal(t, s.source.ID, moving.Assignments[0].InstanceID)
	assert.Equal(t, "Schulhof", moving.Assignments[0].Title)
	assert.Equal(t, "14:00", moving.Assignments[0].StartTime)
	assert.True(t, moving.OnShift)
	assert.True(t, moving.CoversWindow)

	here := poolEntryOf(t, pool, s.otherID)
	assert.Equal(t, scheduleSvc.StaffPoolAssignedHere, here.Category)
	assert.False(t, here.OnShift, "no shift seeded for the target member")

	freeEntry := poolEntryOf(t, pool, free.ID)
	assert.Equal(t, scheduleSvc.StaffPoolOnShiftFree, freeEntry.Category)
	assert.True(t, freeEntry.CoversWindow)
	assert.Equal(t, []string{"12:00–16:00"}, freeEntry.ShiftWindows)

	partialEntry := poolEntryOf(t, pool, partial.ID)
	assert.Equal(t, scheduleSvc.StaffPoolOnShiftFree, partialEntry.Category)
	assert.True(t, partialEntry.OnShift)
	assert.False(t, partialEntry.CoversWindow, "shift ends before the block does")

	absentEntry := poolEntryOf(t, pool, absent.ID)
	assert.Equal(t, scheduleSvc.StaffPoolAbsent, absentEntry.Category, "day-wide absence outranks assignments and shifts")
	require.NotNil(t, absentEntry.AbsenceReason)
	assert.Equal(t, "krank", *absentEntry.AbsenceReason)
}

// TestGetStaffPoolForInstance_NoShiftsMeansDienstplanNotInUse: without any
// shift in the calendar week, not_on_shift carries no signal.
func TestGetStaffPoolForInstance_NoShiftsMeansDienstplanNotInUse(t *testing.T) {
	s := makeMoveSetup(t)
	defer s.cleanup(t)

	repoFactory := repositories.NewFactory(s.db)
	serviceFactory, err := services.NewFactory(repoFactory, s.db, slog.Default())
	require.NoError(t, err)
	pool, err := serviceFactory.TimetableData.GetStaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	assert.False(t, pool.DienstplanInUse)
	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, scheduleSvc.StaffPoolNotOnShift, entry.Category)
	assert.Empty(t, entry.ShiftWindows)
}

// TestGetStaffPoolForInstance_TouchingWindowsDoNotOverlap: a block ending
// exactly when the target starts does not occupy the person.
func TestGetStaffPoolForInstance_TouchingWindowsDoNotOverlap(t *testing.T) {
	s := makeMoveSetup(t)
	defer s.cleanup(t)
	before := createMoveInstance(t, s, "Davor", "13:30", "14:30", scheduleModels.InstanceStatusPlanned)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", before.ID) })
	createMoveStaffRow(t, s, before.ID, s.staffID, nil)
	createPoolShift(t, s, s.staffID, "12:00", "16:00", false)

	repoFactory := repositories.NewFactory(s.db)
	serviceFactory, err := services.NewFactory(repoFactory, s.db, slog.Default())
	require.NoError(t, err)
	// Target runs 14:30–15:30; the other block ends 14:30 sharp.
	pool, err := serviceFactory.TimetableData.GetStaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, scheduleSvc.StaffPoolOnShiftFree, entry.Category)
	assert.Empty(t, entry.Assignments)
}

// TestGetStaffPoolForInstance_CancelledShiftIgnored: a cancelled shift does
// not make someone available.
func TestGetStaffPoolForInstance_CancelledShiftIgnored(t *testing.T) {
	s := makeMoveSetup(t)
	defer s.cleanup(t)
	createPoolShift(t, s, s.staffID, "08:00", "16:00", true)

	repoFactory := repositories.NewFactory(s.db)
	serviceFactory, err := services.NewFactory(repoFactory, s.db, slog.Default())
	require.NoError(t, err)
	pool, err := serviceFactory.TimetableData.GetStaffPoolForInstance(s.ctx, s.target.ID)
	require.NoError(t, err)

	entry := poolEntryOf(t, pool, s.staffID)
	assert.Equal(t, scheduleSvc.StaffPoolNotOnShift, entry.Category)
	assert.False(t, entry.OnShift)
}

func TestGetStaffPoolForInstance_TerminalBlockAbsenceRemainsDayWide(t *testing.T) {
	for _, status := range []string{
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		t.Run(status, func(t *testing.T) {
			s := makeMoveSetup(t)
			defer s.cleanup(t)

			terminal := createMoveInstance(t, s, "Historie", "09:00", "10:00", status)
			t.Cleanup(func() {
				testpkg.CleanupTableRecords(t, s.db, "schedule.activity_instances", terminal.ID)
			})
			createMoveStaffRow(t, s, terminal.ID, s.staffID, func(row *scheduleModels.InstanceStaff) {
				row.IsAbsent = true
				row.AbsenceReason = testpkg.StrPtr("krank")
			})
			// A non-absent terminal assignment is history, not current occupancy.
			createMoveStaffRow(t, s, terminal.ID, s.otherID, nil)
			createPoolShift(t, s, s.staffID, "08:00", "16:00", false)
			createPoolShift(t, s, s.otherID, "08:00", "16:00", false)

			repoFactory := repositories.NewFactory(s.db)
			serviceFactory, err := services.NewFactory(repoFactory, s.db, slog.Default())
			require.NoError(t, err)
			pool, err := serviceFactory.TimetableData.GetStaffPoolForInstance(s.ctx, s.target.ID)
			require.NoError(t, err)

			absent := poolEntryOf(t, pool, s.staffID)
			assert.Equal(t, scheduleSvc.StaffPoolAbsent, absent.Category)
			require.NotNil(t, absent.AbsenceReason)
			assert.Equal(t, "krank", *absent.AbsenceReason)

			historicalOnly := poolEntryOf(t, pool, s.otherID)
			assert.Equal(t, scheduleSvc.StaffPoolOnShiftFree, historicalOnly.Category)
			assert.Empty(t, historicalOnly.Assignments)
		})
	}
}
