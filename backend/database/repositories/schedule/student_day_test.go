package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FindInstancesWithAttendanceByStudentAndDateRange must return one row per
// (instance, attendance) pair in the range, tenant-scoped, sorted.
func TestInstanceStudentRepository_FindInstancesWithAttendanceByStudentAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("SD-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	dayA := timezone.NewDate(2026, 10, 5)
	dayB := timezone.NewDate(2026, 10, 6)
	dayOutside := timezone.NewDate(2026, 11, 1)

	instA, cleanA := createInstanceFixture(t, db, "sd-A", dayA)
	defer cleanA()
	instB, cleanB := createInstanceFixture(t, db, "sd-B", dayB)
	defer cleanB()
	instOutside, cleanOut := createInstanceFixture(t, db, "sd-out", dayOutside)
	defer cleanOut()

	late := scheduleModels.AttendanceSubstatusLate
	note := "bus verpasst"
	checkedAt := time.Date(2026, 10, 5, 13, 5, 0, 0, time.UTC)

	mkRow := func(instID int64, status string, withExtras bool) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instID,
			StudentID:  student.ID,
			Status:     status,
		}
		if withExtras {
			row.Substatus = &late
			row.Note = &note
			row.CheckedInAt = &checkedAt
		}
		row.SetTenantID(1)
		return row
	}

	rowA := mkRow(instA.ID, scheduleModels.AttendanceStatusPresent, true)
	rowB := mkRow(instB.ID, scheduleModels.AttendanceStatusExpected, false)
	rowOut := mkRow(instOutside.ID, scheduleModels.AttendanceStatusExpected, false)

	require.NoError(t, repo.Create(ctx, rowA))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowA.ID)
	require.NoError(t, repo.Create(ctx, rowB))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowB.ID)
	require.NoError(t, repo.Create(ctx, rowOut))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowOut.ID)

	t.Run("returns joined rows in range, sorted", func(t *testing.T) {
		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, student.ID, dayA, dayB)
		require.NoError(t, err)
		require.Len(t, rows, 2)

		// Sorted by date ASC — dayA first.
		assert.Equal(t, instA.ID, rows[0].Instance.ID)
		assert.Equal(t, rowA.ID, rows[0].Attendance.ID)
		assert.Equal(t, scheduleModels.AttendanceStatusPresent, rows[0].Attendance.Status)
		require.NotNil(t, rows[0].Attendance.Substatus)
		assert.Equal(t, scheduleModels.AttendanceSubstatusLate, *rows[0].Attendance.Substatus)
		require.NotNil(t, rows[0].Attendance.Note)
		assert.Equal(t, "bus verpasst", *rows[0].Attendance.Note)
		require.NotNil(t, rows[0].Attendance.CheckedInAt)

		assert.Equal(t, instB.ID, rows[1].Instance.ID)
		assert.Equal(t, scheduleModels.AttendanceStatusExpected, rows[1].Attendance.Status)
	})

	t.Run("instance outside range not included", func(t *testing.T) {
		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, student.ID, dayA, dayB)
		require.NoError(t, err)
		for _, r := range rows {
			assert.NotEqual(t, instOutside.ID, r.Instance.ID,
				"instance outside the requested range must not leak in")
		}
	})

	t.Run("empty range returns empty slice", func(t *testing.T) {
		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(
			ctx, student.ID,
			timezone.NewDate(2027, 1, 1),
			timezone.NewDate(2027, 1, 2),
		)
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("different tenant context returns empty", func(t *testing.T) {
		ctxT2 := testpkg.TenantContext(2)
		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctxT2, student.ID, dayA, dayB)
		require.NoError(t, err)
		assert.Empty(t, rows, "tenant 2 must not see tenant 1 rows")
	})
}

// The #1747 "war an dem Tag nicht eingeplant" marker is the persisted
// not_scheduled column that Complete()/the nightly bridge stamp on the rows
// they spare the absence, and the per-student read must not resurface those
// rows as expected attendance. Everything else stays visible: rows with a real
// outcome, 'expected' rows on cancelled instances, and — critically — an
// unmarked 'expected' row on a completed instance, which is what an attendance
// PATCH reset writes and must never be mistaken for a non-booking.
func TestInstanceStudentRepository_FindInstancesWithAttendance_HidesNotScheduledOnCompleted(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Lina", fmt.Sprintf("NSC-%d", time.Now().UnixNano()), "1b")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	day := timezone.NewDate(2034, 6, 5)

	completedInst, cleanCompleted := createInstanceFixture(t, db, "nsc-done", day)
	defer cleanCompleted()
	cancelledInst, cleanCancelled := createInstanceFixture(t, db, "nsc-cxl", day)
	defer cleanCancelled()

	completedInst.Status = scheduleModels.InstanceStatusCompleted
	require.NoError(t, instanceRepo.Update(ctx, completedInst))
	cancelledInst.Status = scheduleModels.InstanceStatusCancelled
	require.NoError(t, instanceRepo.Update(ctx, cancelledInst))

	mkRow := func(instID int64, status string, notScheduled bool) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID:   instID,
			StudentID:    student.ID,
			Status:       status,
			NotScheduled: notScheduled,
		}
		row.SetTenantID(1)
		return row
	}

	spared := mkRow(completedInst.ID, scheduleModels.AttendanceStatusExpected, true)
	require.NoError(t, repo.Create(ctx, spared))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", spared.ID)
	cancelledExpected := mkRow(cancelledInst.ID, scheduleModels.AttendanceStatusExpected, false)
	require.NoError(t, repo.Create(ctx, cancelledExpected))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", cancelledExpected.ID)

	t.Run("spared row on a completed instance is hidden", func(t *testing.T) {
		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, student.ID, day, day)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, cancelledInst.ID, rows[0].Instance.ID,
			"the cancelled instance's expected row stays visible; the marked one must not")
	})

	t.Run("an unmarked expected row on a completed instance stays visible", func(t *testing.T) {
		// What an attendance PATCH reset writes. Nothing about it says the
		// child was never booked, so hiding it would erase a real expectation
		// from the history and the exports.
		unmarked, cleanUnmarked := createInstanceFixture(t, db, "nsc-reset", day)
		defer cleanUnmarked()
		unmarked.Status = scheduleModels.InstanceStatusCompleted
		require.NoError(t, instanceRepo.Update(ctx, unmarked))

		row := mkRow(unmarked.ID, scheduleModels.AttendanceStatusExpected, false)
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, student.ID, day, day)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		instanceIDs := []int64{rows[0].Instance.ID, rows[1].Instance.ID}
		assert.Contains(t, instanceIDs, unmarked.ID)
	})

	t.Run("a marked row somebody decided by hand stays visible", func(t *testing.T) {
		spared.Status = scheduleModels.AttendanceStatusAbsent
		require.NoError(t, repo.Update(ctx, spared))

		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, student.ID, day, day)
		require.NoError(t, err)
		assert.Len(t, rows, 2)
	})

	t.Run("a hand-set expectation stays visible on its own evidence", func(t *testing.T) {
		// Staff setting an unbooked slot back to 'expected' is the one decision
		// that lands on the exact shape the marker claims. The PATCH clears
		// not_scheduled on that write, so the pair should never coexist — but
		// the read must not depend on that pairing holding, or the day some
		// other writer forgets it a deliberate expectation goes invisible
		// (#1747 review).
		decided, cleanDecided := createInstanceFixture(t, db, "nsc-manual", day)
		defer cleanDecided()
		decided.Status = scheduleModels.InstanceStatusCompleted
		require.NoError(t, instanceRepo.Update(ctx, decided))

		row := mkRow(decided.ID, scheduleModels.AttendanceStatusExpected, true)
		decidedAt := time.Date(2034, 6, 5, 9, 30, 0, 0, time.UTC)
		row.ManualStatusAt = &decidedAt
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		rows, err := repo.FindInstancesWithAttendanceByStudentAndDateRange(ctx, student.ID, day, day)
		require.NoError(t, err)
		var found *scheduleModels.ScheduledInstanceRow
		for _, r := range rows {
			if r.Instance.ID == decided.ID {
				found = r
			}
		}
		require.NotNil(t, found, "a hand-decided row must never be hidden as a non-booking")
		require.NotNil(t, found.Attendance.ManualStatusAt,
			"and the read must carry the stamp, or every downstream verdict re-derives against the plan it overrides")
	})
}

// HasPlannedSlotsInRange is the tenant-wide care-plan signal: planned
// assignment rows count, walk-in rows (is_unplanned) and rows outside the
// range do not. The far-future window keeps concurrent fixtures from other
// packages out of the tenant-wide EXISTS.
func TestInstanceStudentRepository_HasPlannedSlotsInRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Mila", fmt.Sprintf("HP-%d", time.Now().UnixNano()), "2b")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	from := timezone.NewDate(2032, 3, 2)
	to := timezone.NewDate(2032, 3, 6)
	dayOutside := timezone.NewDate(2032, 4, 1)

	instIn, cleanIn := createInstanceFixture(t, db, "hp-in", from)
	defer cleanIn()
	instOut, cleanOut := createInstanceFixture(t, db, "hp-out", dayOutside)
	defer cleanOut()

	mkRow := func(instID int64, unplanned bool) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID:  instID,
			StudentID:   student.ID,
			Status:      scheduleModels.AttendanceStatusPresent,
			IsUnplanned: unplanned,
		}
		row.SetTenantID(1)
		return row
	}

	walkIn := mkRow(instIn.ID, true)
	require.NoError(t, repo.Create(ctx, walkIn))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", walkIn.ID)
	plannedOutside := mkRow(instOut.ID, false)
	require.NoError(t, repo.Create(ctx, plannedOutside))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", plannedOutside.ID)

	t.Run("walk-in in range and planned row outside range are no evidence", func(t *testing.T) {
		has, err := repo.HasPlannedSlotsInRange(ctx, from, to)
		require.NoError(t, err)
		assert.False(t, has)
	})

	// A second assignment on the same instance would collide with the walk-in
	// row (unique instance+student) — flip the walk-in to planned instead:
	// the signal must react to the planned state itself.
	walkIn.IsUnplanned = false
	require.NoError(t, repo.Update(ctx, walkIn))

	t.Run("planned row in range flips the signal", func(t *testing.T) {
		has, err := repo.HasPlannedSlotsInRange(ctx, from, to)
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("different tenant context sees no evidence", func(t *testing.T) {
		has, err := repo.HasPlannedSlotsInRange(testpkg.TenantContext(2), from, to)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("wraps driver errors in DatabaseError", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		has, err := repo.HasPlannedSlotsInRange(cancelledCtx, from, to)
		assert.False(t, has)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "check planned slots in range", dbErr.Op)
	})
}

// A cancelled instance keeps its instance_students rows, but a planned booking
// on a cancelled-only occurrence must not count as care-plan evidence — it
// would re-activate assignment hints and export columns without a usable slot
// (#1920). The far-future window keeps concurrent fixtures from other packages
// out of the tenant-wide EXISTS.
func TestInstanceStudentRepository_HasPlannedSlotsInRange_CancelledInstance(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Emil", fmt.Sprintf("HPC-%d", time.Now().UnixNano()), "4c")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	from := timezone.NewDate(2033, 5, 2)
	to := timezone.NewDate(2033, 5, 6)

	inst, cleanInst := createInstanceFixture(t, db, "hpc", from)
	defer cleanInst()

	planned := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	planned.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, planned))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", planned.ID)

	t.Run("planned row on a live instance counts", func(t *testing.T) {
		has, err := repo.HasPlannedSlotsInRange(ctx, from, to)
		require.NoError(t, err)
		assert.True(t, has)
	})

	inst.Status = scheduleModels.InstanceStatusCancelled
	require.NoError(t, instanceRepo.Update(ctx, inst))

	t.Run("cancelled instance is no evidence despite the surviving row", func(t *testing.T) {
		has, err := repo.HasPlannedSlotsInRange(ctx, from, to)
		require.NoError(t, err)
		assert.False(t, has)
	})
}
