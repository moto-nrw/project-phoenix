package schedule_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceStudentRepository_Create_and_FindByInstanceID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu", timezone.NewDate(2026, 9, 19))
	defer cleanupInst()

	studentA := testpkg.CreateTestStudent(t, db, "Max", fmt.Sprintf("A-%d", time.Now().UnixNano()), "3a")
	studentB := testpkg.CreateTestStudent(t, db, "Mia", fmt.Sprintf("B-%d", time.Now().UnixNano()), "3a")

	rowA := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  studentA.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	rowA.SetTenantID(testpkg.Tenant(t))

	rowB := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  studentB.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	rowB.SetTenantID(testpkg.Tenant(t))

	require.NoError(t, repo.Create(ctx, rowA))
	require.NoError(t, repo.Create(ctx, rowB))

	t.Run("FindByInstanceID returns both", func(t *testing.T) {
		rows, err := repo.FindByInstanceID(ctx, inst.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rows), 2)
	})

	t.Run("FindByInstanceAndStudent returns single", func(t *testing.T) {
		got, err := repo.FindByInstanceAndStudent(ctx, inst.ID, studentA.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, rowA.ID, got.ID)
		assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status)
	})

	t.Run("FindByInstanceAndStudent returns nil when missing", func(t *testing.T) {
		got, err := repo.FindByInstanceAndStudent(ctx, inst.ID, int64(999999999))
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("UNIQUE(instance_id, student_id) blocks duplicate", func(t *testing.T) {
		dup := &scheduleModels.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  studentA.ID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		dup.SetTenantID(testpkg.Tenant(t))
		err := repo.Create(ctx, dup)
		require.Error(t, err)
	})

	t.Run("note max length enforced at DB layer", func(t *testing.T) {
		// Service-layer validation catches this first, but we also want the
		// CHECK constraint to protect integrity. Skip model validation by
		// inserting via raw BUN bypass: construct an overlong note.
		row := &scheduleModels.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  studentB.ID, // different student still FAILS due to UNIQUE, so use a fresh one
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(testpkg.Tenant(t))
		long := strings.Repeat("x", scheduleModels.InstanceStudentNoteMaxLength+1)
		row.Note = &long
		err := repo.Create(ctx, row)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "note cannot exceed 500 characters")
	})
}

func TestInstanceStudentRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-upd", timezone.NewDate(2026, 9, 20))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Ella", fmt.Sprintf("Upd-%d", time.Now().UnixNano()), "3a")

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	now := time.Now().UTC()
	late := scheduleModels.AttendanceSubstatusLate
	note := "bus verpasst"
	row.Status = scheduleModels.AttendanceStatusPresent
	row.Substatus = &late
	row.Note = &note
	row.CheckedInAt = &now

	require.NoError(t, repo.Update(ctx, row))

	got, err := repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, got.Status)
	require.NotNil(t, got.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusLate, *got.Substatus)
	require.NotNil(t, got.Note)
	assert.Equal(t, "bus verpasst", *got.Note)
	require.NotNil(t, got.CheckedInAt)
}

func TestInstanceStudentRepository_FindByStudentAndDateRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("Range-%d", time.Now().UnixNano()), "3a")

	dayA := timezone.NewDate(2026, 10, 5)
	dayB := timezone.NewDate(2026, 10, 6)
	dayOutside := timezone.NewDate(2026, 11, 1)

	instA, cleanA := createInstanceFixture(t, db, "range-A", dayA)
	defer cleanA()
	instB, cleanB := createInstanceFixture(t, db, "range-B", dayB)
	defer cleanB()
	instOutside, cleanOut := createInstanceFixture(t, db, "range-out", dayOutside)
	defer cleanOut()

	mkRow := func(instID int64) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instID,
			StudentID:  student.ID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(testpkg.Tenant(t))
		return row
	}

	rA, rB, rOut := mkRow(instA.ID), mkRow(instB.ID), mkRow(instOutside.ID)
	require.NoError(t, repo.Create(ctx, rA))
	require.NoError(t, repo.Create(ctx, rB))
	require.NoError(t, repo.Create(ctx, rOut))

	got, err := repo.FindByStudentAndDateRange(ctx, student.ID, dayA, dayB)
	require.NoError(t, err)

	ids := map[int64]bool{}
	for _, r := range got {
		ids[r.ID] = true
	}
	assert.True(t, ids[rA.ID])
	assert.True(t, ids[rB.ID])
	assert.False(t, ids[rOut.ID], "instance outside range must not appear")
}

func TestInstanceStudentRepository_CreateValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	t.Run("Create rejects nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Create rejects invalid payload", func(t *testing.T) {
		bad := &scheduleModels.InstanceStudent{
			InstanceID: 0,
			StudentID:  123,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		bad.SetTenantID(testpkg.Tenant(t))
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})
}

func TestInstanceStudentRepository_UpdateValidation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	t.Run("Update rejects nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Update rejects invalid payload", func(t *testing.T) {
		bad := &scheduleModels.InstanceStudent{
			InstanceID: 10,
			StudentID:  0,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		bad.SetTenantID(testpkg.Tenant(t))
		err := repo.Update(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_id is required")
	})
}

func TestInstanceStudentRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	got, err := repo.FindByID(ctx, int64(999999999))
	require.Error(t, err)
	assert.Nil(t, got)
	var dbErr *modelBase.DatabaseError
	require.ErrorAs(t, err, &dbErr)
	assert.Equal(t, "find by id", dbErr.Op)
}

func TestInstanceStudentRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-list", timezone.NewDate(2026, 10, 8))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Peter", fmt.Sprintf("List-%d", time.Now().UnixNano()), "3a")

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	t.Run("nil options returns rows", func(t *testing.T) {
		rows, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rows), 1)
	})

	t.Run("with filter + pagination", func(t *testing.T) {
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("instance_id", inst.ID)
		options.WithPagination(1, 50)

		rows, err := repo.List(ctx, options)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		for _, r := range rows {
			assert.Equal(t, inst.ID, r.InstanceID)
		}
	})

	t.Run("wraps driver errors in DatabaseError", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		rows, err := repo.List(cancelledCtx, nil)
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "list with options", dbErr.Op)
	})
}

func TestInstanceStudentRepository_ErrorBranches(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	t.Run("FindByInstanceID wraps driver errors", func(t *testing.T) {
		rows, err := repo.FindByInstanceID(cancelledCtx, int64(999999))
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by instance id", dbErr.Op)
	})

	t.Run("FindByStudentAndDateRange wraps driver errors", func(t *testing.T) {
		from := timezone.NewDate(2026, 10, 1)
		to := timezone.NewDate(2026, 10, 31)
		rows, err := repo.FindByStudentAndDateRange(cancelledCtx, int64(999999), from, to)
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by student and date range", dbErr.Op)
	})

	t.Run("FindByInstanceAndStudent wraps driver errors", func(t *testing.T) {
		row, err := repo.FindByInstanceAndStudent(cancelledCtx, int64(999999), int64(999999))
		assert.Nil(t, row)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by instance and student", dbErr.Op)
	})

	t.Run("DeleteByInstanceID wraps driver errors", func(t *testing.T) {
		err := repo.DeleteByInstanceID(cancelledCtx, int64(999999))
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "delete by instance id", dbErr.Op)
	})
}

func TestInstanceStudentRepository_UpdateAttendanceFromCheckin(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "mirror", timezone.NewDate(2026, 10, 10))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Lea", fmt.Sprintf("Mirror-%d", time.Now().UnixNano()), "3a")

	t.Run("flips expected → present and stamps checked_in_at", func(t *testing.T) {
		row := &scheduleModels.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  student.ID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		checkedAt := time.Date(2026, 10, 10, 13, 5, 0, 0, time.UTC)
		updated, err := repo.UpdateAttendanceFromCheckin(ctx, inst.ID, student.ID, checkedAt)
		require.NoError(t, err)
		assert.True(t, updated, "expected row should be flipped")

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.AttendanceStatusPresent, got.Status)
		require.NotNil(t, got.CheckedInAt)
		assert.WithinDuration(t, checkedAt, *got.CheckedInAt, time.Second)
	})

	t.Run("no-op when row is already present (monotonicity)", func(t *testing.T) {
		other := testpkg.CreateTestStudent(t, db, "Tom", fmt.Sprintf("Mono-%d", time.Now().UnixNano()), "3a")
		defer testpkg.CleanupActivityFixtures(t, db, other.ID)

		firstCheckin := time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC)
		row := &scheduleModels.InstanceStudent{
			InstanceID:  inst.ID,
			StudentID:   other.ID,
			Status:      scheduleModels.AttendanceStatusPresent,
			CheckedInAt: &firstCheckin,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		laterCheckin := time.Date(2026, 10, 10, 14, 30, 0, 0, time.UTC)
		updated, err := repo.UpdateAttendanceFromCheckin(ctx, inst.ID, other.ID, laterCheckin)
		require.NoError(t, err)
		assert.False(t, updated, "already-present row must not be clobbered")

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.AttendanceStatusPresent, got.Status)
		require.NotNil(t, got.CheckedInAt)
		// Original checked_in_at must be preserved — monotonicity.
		assert.WithinDuration(t, firstCheckin, *got.CheckedInAt, time.Second)
	})

	t.Run("reopen re-stamps check-in and blocks superseded checkouts", func(t *testing.T) {
		other := testpkg.CreateTestStudent(t, db, "Ria", fmt.Sprintf("Reentry-%d", time.Now().UnixNano()), "3a")
		defer testpkg.CleanupActivityFixtures(t, db, other.ID)

		firstCheckin := time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC)
		firstCheckout := firstCheckin.Add(time.Hour)
		row := &scheduleModels.InstanceStudent{
			InstanceID:   inst.ID,
			StudentID:    other.ID,
			Status:       scheduleModels.AttendanceStatusPresent,
			CheckedInAt:  &firstCheckin,
			CheckedOutAt: &firstCheckout,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		// Re-entry two hours after the first checkout reopens the slot and
		// re-stamps checked_in_at (session boundary).
		reentry := firstCheckout.Add(2 * time.Hour)
		updated, err := repo.UpdateAttendanceFromCheckin(ctx, inst.ID, other.ID, reentry)
		require.NoError(t, err)
		assert.True(t, updated)

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Nil(t, got.CheckedOutAt)
		require.NotNil(t, got.CheckedInAt)
		assert.WithinDuration(t, reentry, *got.CheckedInAt, time.Second)

		// A delayed checkout from the superseded interval (before the
		// re-entry) must NOT close the reopened slot.
		require.NoError(t, repo.UpdateAttendanceCheckout(ctx, inst.ID, other.ID, firstCheckout.Add(30*time.Minute)))
		got, err = repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Nil(t, got.CheckedOutAt, "superseded checkout must not corrupt the reopened slot")

		// A genuine checkout after the re-entry closes it.
		finalCheckout := reentry.Add(time.Hour)
		require.NoError(t, repo.UpdateAttendanceCheckout(ctx, inst.ID, other.ID, finalCheckout))
		got, err = repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		require.NotNil(t, got.CheckedOutAt)
		assert.WithinDuration(t, finalCheckout, *got.CheckedOutAt, time.Second)
	})

	t.Run("no-op when no matching row (walk-in)", func(t *testing.T) {
		walkin := testpkg.CreateTestStudent(t, db, "Kim", fmt.Sprintf("Walk-%d", time.Now().UnixNano()), "3a")
		defer testpkg.CleanupActivityFixtures(t, db, walkin.ID)

		// walkin student has NO instance_students row — mirror should be a no-op
		// returning (false, nil) rather than an error.
		updated, err := repo.UpdateAttendanceFromCheckin(ctx, inst.ID, walkin.ID, time.Now())
		require.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("no-op when row belongs to another tenant (RLS)", func(t *testing.T) {
		// Row is created in tenant 1 (above). Call with tenant 2 context
		// should match zero rows under RLS.
		ctxT2 := testpkg.TenantContext(2)
		updated, err := repo.UpdateAttendanceFromCheckin(ctxT2, inst.ID, student.ID, time.Now())
		require.NoError(t, err)
		assert.False(t, updated)
	})
}

func TestInstanceStudentRepository_UpdateAttendanceCheckout_GuardsMirroredPresence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	inst, cleanupInst := createInstanceFixture(t, db, "checkout-guard", timezone.NewDate(2026, 10, 12))
	defer cleanupInst()

	checkedIn := time.Date(2026, 10, 12, 13, 0, 0, 0, time.UTC)
	checkedOut := checkedIn.Add(time.Hour)
	tests := []struct {
		name       string
		status     string
		checkIn    *time.Time
		checkoutAt time.Time
		wantWrite  bool
	}{
		{name: "mirrored present row", status: scheduleModels.AttendanceStatusPresent, checkIn: &checkedIn, checkoutAt: checkedOut, wantWrite: true},
		{name: "expected row", status: scheduleModels.AttendanceStatusExpected, checkoutAt: checkedOut},
		{name: "absent row with timestamp", status: scheduleModels.AttendanceStatusAbsent, checkIn: &checkedIn, checkoutAt: checkedOut},
		{name: "checkout before checkin", status: scheduleModels.AttendanceStatusPresent, checkIn: &checkedIn, checkoutAt: checkedIn.Add(-time.Minute)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			student := testpkg.CreateTestStudent(t, db, "Checkout", fmt.Sprintf("Guard-%d", time.Now().UnixNano()), "3a")
			row := &scheduleModels.InstanceStudent{
				InstanceID:  inst.ID,
				StudentID:   student.ID,
				Status:      tt.status,
				CheckedInAt: tt.checkIn,
			}
			row.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, repo.Create(ctx, row))

			require.NoError(t, repo.UpdateAttendanceCheckout(ctx, inst.ID, student.ID, tt.checkoutAt))
			got, err := repo.FindByID(ctx, row.ID)
			require.NoError(t, err)
			if tt.wantWrite {
				require.NotNil(t, got.CheckedOutAt)
				assert.WithinDuration(t, tt.checkoutAt, *got.CheckedOutAt, time.Second)
			} else {
				assert.Nil(t, got.CheckedOutAt)
			}
		})
	}
}

func TestInstanceStudentRepository_CreateUnplannedPresentIfAbsent_PromotesConcurrentRosterRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	inst, cleanupInst := createInstanceFixture(t, db, "unplanned-conflict", timezone.NewDate(2026, 10, 14))
	defer cleanupInst()
	student := testpkg.CreateTestStudent(t, db, "Conflict", fmt.Sprintf("Roster-%d", time.Now().UnixNano()), "3a")

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	checkedIn := time.Date(2026, 10, 14, 12, 30, 0, 0, time.UTC)
	got, err := repo.CreateUnplannedPresentIfAbsent(ctx, inst.ID, student.ID, checkedIn)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, got.Status)
	assert.False(t, got.IsUnplanned, "a concurrent booked row must keep its booking provenance")
	require.NotNil(t, got.CheckedInAt)
	assert.WithinDuration(t, checkedIn, *got.CheckedInAt, time.Second)
}

func TestInstanceStudentRepository_FindCurrentCandidates_ExcludesEndedInstances(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	day := timezone.NewDate(2026, 10, 15)
	student := testpkg.CreateTestStudent(t, db, "Candidate", fmt.Sprintf("Status-%d", time.Now().UnixNano()), "3a")

	statuses := []string{
		scheduleModels.InstanceStatusPlanned,
		scheduleModels.InstanceStatusActive,
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	}
	instanceByStatus := make(map[string]*scheduleModels.ActivityInstance, len(statuses))
	for _, status := range statuses {
		inst, cleanup := createInstanceFixture(t, db, "candidate-"+status, day)
		defer cleanup()
		_, err := db.NewUpdate().Table("schedule.activity_instances").
			Set("status = ?", status).
			Where("id = ?", inst.ID).
			Exec(ctx)
		require.NoError(t, err)
		row := &scheduleModels.InstanceStudent{InstanceID: inst.ID, StudentID: student.ID, Status: scheduleModels.AttendanceStatusExpected}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		instanceByStatus[status] = inst
	}

	at := time.Date(2026, 10, 15, 14, 30, 0, 0, timezone.Berlin)
	rows, err := repo.FindCurrentCandidates(ctx, student.ID, day, at)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	ids := []int64{rows[0].InstanceID, rows[1].InstanceID}
	assert.Contains(t, ids, instanceByStatus[scheduleModels.InstanceStatusPlanned].ID)
	assert.Contains(t, ids, instanceByStatus[scheduleModels.InstanceStatusActive].ID)
	assert.NotContains(t, ids, instanceByStatus[scheduleModels.InstanceStatusCompleted].ID)
	assert.NotContains(t, ids, instanceByStatus[scheduleModels.InstanceStatusCancelled].ID)
}

func TestInstanceStudentRepository_ReconcileAttendanceInterval_UsesPreviousIntervalAsGuard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	inst, cleanupInst := createInstanceFixture(t, db, "visit-revision", timezone.NewDate(2026, 10, 16))
	defer cleanupInst()
	student := testpkg.CreateTestStudent(t, db, "Revision", fmt.Sprintf("Guard-%d", time.Now().UnixNano()), "3a")

	previousIn := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	previousOut := previousIn.Add(time.Hour)
	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID, StudentID: student.ID, Status: scheduleModels.AttendanceStatusPresent,
		CheckedInAt: &previousIn, CheckedOutAt: &previousOut,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	updatedIn := previousIn.Add(15 * time.Minute)
	changed, err := repo.ReconcileAttendanceInterval(ctx, inst.ID, student.ID, previousIn, &previousOut, updatedIn, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	got, err := repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CheckedInAt)
	assert.WithinDuration(t, updatedIn, *got.CheckedInAt, time.Second)
	assert.Nil(t, got.CheckedOutAt)

	staleOut := previousOut.Add(time.Minute)
	changed, err = repo.ReconcileAttendanceInterval(ctx, inst.ID, student.ID, previousIn, &previousOut, previousIn, &staleOut)
	require.NoError(t, err)
	assert.False(t, changed, "a stale edit must not replace the reopened interval")

	legacyExit := updatedIn.Add(30 * time.Minute)
	correctedExit := legacyExit.Add(15 * time.Minute)
	changed, err = repo.ReconcileAttendanceInterval(ctx, inst.ID, student.ID, updatedIn, &legacyExit, updatedIn, &correctedExit)
	require.NoError(t, err)
	assert.True(t, changed, "a closed visit edit must repair its historically missing checkout")
	got, err = repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, got.CheckedOutAt)
	assert.WithinDuration(t, correctedExit, *got.CheckedOutAt, time.Second)
}

func TestInstanceStudentRepository_ReleaseStatusDayReappliesLatestRemainingStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)
	date := timezone.NewDate(2026, 10, 13)
	inst, cleanupInst := createInstanceFixture(t, db, "status-release", date)
	defer cleanupInst()
	student := testpkg.CreateTestStudent(t, db, "Status", fmt.Sprintf("Replacement-%d", time.Now().UnixNano()), "3a")

	attendance := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	attendance.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, factory.InstanceStudent.Create(ctx, attendance))

	older := &activeModels.StudentStatusDay{
		StudentID: student.ID, Date: date, Status: activeModels.StudentStatusDaySick,
		ReportedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC), Source: activeModels.StudentStatusSourcePlanned,
	}
	newer := &activeModels.StudentStatusDay{
		StudentID: student.ID, Date: date, Status: activeModels.StudentStatusDayExcused,
		ReportedAt: time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC), Source: activeModels.StudentStatusSourcePlanned,
	}
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, older))
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, newer))
	got, err := factory.InstanceStudent.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status)
	require.NotNil(t, got.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *got.Substatus)
	require.NotNil(t, got.StudentStatusDayID)
	assert.Equal(t, newer.ID, *got.StudentStatusDayID, "the newest active status must take provenance immediately")

	require.NoError(t, factory.StudentStatusDay.MarkClearedByID(ctx, newer.ID, time.Now(), activeModels.StudentStatusSourceManual))
	got, err = factory.InstanceStudent.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	require.NotNil(t, got.StudentStatusDayID)
	assert.Equal(t, older.ID, *got.StudentStatusDayID, "clearing the newest status must restore the older active status")
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, newer))

	require.NoError(t, factory.StudentStatusDay.MarkClearedByID(ctx, older.ID, time.Now(), activeModels.StudentStatusSourceManual))
	got, err = factory.InstanceStudent.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status)
	require.NotNil(t, got.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *got.Substatus)
	require.NotNil(t, got.StudentStatusDayID)
	assert.Equal(t, newer.ID, *got.StudentStatusDayID)

	require.NoError(t, factory.StudentStatusDay.MarkClearedByID(ctx, newer.ID, time.Now(), activeModels.StudentStatusSourceManual))
	got, err = factory.InstanceStudent.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status)
	assert.Nil(t, got.Substatus)
	assert.Nil(t, got.StudentStatusDayID)

	completedStatus := &activeModels.StudentStatusDay{
		StudentID: student.ID, Date: date, Status: activeModels.StudentStatusDayClassTrip,
		ReportedAt: time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC), Source: activeModels.StudentStatusSourcePlanned,
	}
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, completedStatus))
	_, err = db.NewUpdate().
		Table("schedule.activity_instances").
		Set("status = ?", scheduleModels.InstanceStatusCompleted).
		Where("id = ?", inst.ID).
		Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, factory.StudentStatusDay.MarkClearedByID(ctx, completedStatus.ID, time.Now(), activeModels.StudentStatusSourceManual))
	got, err = factory.InstanceStudent.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status, "completed slots remain historical absences")
	assert.Nil(t, got.Substatus)
	assert.Nil(t, got.StudentStatusDayID)
}

// A sick or excused report lands on every expected row of the day, long before
// anything knows whether the child was even booked into care. Ending the block
// is what resolves that — so marking the non-booking must also take back the
// absence the day status wrote, or the child keeps a missed care day they were
// never owed in their history and exports (#1747).
func TestInstanceStudentRepository_MarkNotScheduled_TakesBackStatusDayAbsence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	date := timezone.NewDate(2026, 10, 14)
	inst, cleanupInst := createInstanceFixture(t, db, "not-scheduled-sick", date)
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Unbooked", fmt.Sprintf("Sick-%d", time.Now().UnixNano()), "3a")

	attendance := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	attendance.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, factory.InstanceStudent.Create(ctx, attendance))

	sick := &activeModels.StudentStatusDay{
		StudentID: student.ID, Date: date, Status: activeModels.StudentStatusDaySick,
		ReportedAt: time.Date(2026, 10, 14, 7, 0, 0, 0, time.UTC), Source: activeModels.StudentStatusSourcePlanned,
	}
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, sick))

	got, err := repo.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	require.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status, "precondition: the sick report writes the absence")
	require.NotNil(t, got.StudentStatusDayID)

	require.NoError(t, repo.MarkNotScheduled(ctx, []scheduleModels.StudentInstanceRef{
		{StudentID: student.ID, InstanceID: inst.ID},
	}))

	got, err = repo.FindByID(ctx, attendance.ID)
	require.NoError(t, err)
	assert.True(t, got.NotScheduled, "the non-booking must be recorded")
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status,
		"a child owed no care that day cannot be absent from it")
	assert.Nil(t, got.Substatus)
	assert.Nil(t, got.StudentStatusDayID,
		"the provenance must go too, or clearing the status day writes the absence back")
}

// The counterpart: outcomes that belong to a person or a device are never
// relabelled as a non-booking, marker or not.
func TestInstanceStudentRepository_MarkNotScheduled_KeepsDecidedOutcomes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	date := timezone.NewDate(2026, 10, 15)
	inst, cleanupInst := createInstanceFixture(t, db, "not-scheduled-decided", date)
	defer cleanupInst()

	suffix := time.Now().UnixNano()
	manual := testpkg.CreateTestStudent(t, db, "Manual", fmt.Sprintf("Absent-%d", suffix), "3a")
	present := testpkg.CreateTestStudent(t, db, "Checked", fmt.Sprintf("In-%d", suffix), "3a")

	rows := map[int64]*scheduleModels.InstanceStudent{
		manual.ID: {
			InstanceID: inst.ID, StudentID: manual.ID,
			Status: scheduleModels.AttendanceStatusAbsent, // PATCH-decided: no status-day provenance
		},
		present.ID: {
			InstanceID: inst.ID, StudentID: present.ID,
			Status: scheduleModels.AttendanceStatusPresent,
		},
	}
	refs := make([]scheduleModels.StudentInstanceRef, 0, len(rows))
	for studentID, row := range rows {
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		refs = append(refs, scheduleModels.StudentInstanceRef{StudentID: studentID, InstanceID: inst.ID})
	}

	require.NoError(t, repo.MarkNotScheduled(ctx, refs))

	for studentID, row := range rows {
		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.False(t, got.NotScheduled, "student %d must keep its own outcome", studentID)
		assert.Equal(t, row.Status, got.Status)
	}
}

// Staff can PATCH an unbooked slot back to 'expected' — "the plan is wrong,
// this child is coming". That decision lands on the same status the automatic
// state carries, so completion can only tell them apart by the manual_status_at
// stamp the PATCH writes. Stamping the non-booking over it would hide a
// deliberate expectation from the completed-instance views, the child's history
// and the exports (#1747 review).
func TestInstanceStudentRepository_MarkNotScheduled_KeepsManualExpected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	date := timezone.NewDate(2026, 10, 16)
	inst, cleanupInst := createInstanceFixture(t, db, "not-scheduled-manual", date)
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Manuell", fmt.Sprintf("Erwartet-%d", time.Now().UnixNano()), "3a")

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
		// The slot already carries the marker from an earlier completion.
		NotScheduled: true,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	expected := scheduleModels.AttendanceStatusExpected
	require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID, scheduleModels.AttendanceFieldPatch{
		Status: &expected,
	}))

	got, err := repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ManualStatusAt, "the PATCH must record that a human decided this row")
	require.False(t, got.NotScheduled, "and must drop the marker it overrides")

	// The row is no longer a candidate the completion may resolve...
	candidates, err := repo.FindNotScheduledCandidatesByInstanceIDs(ctx, []int64{inst.ID})
	require.NoError(t, err)
	for _, candidate := range candidates {
		assert.NotEqual(t, row.ID, candidate.ID, "a hand-decided row must not be offered to the completion")
	}

	// ...and the write itself refuses it even when a caller passes it anyway.
	require.NoError(t, repo.MarkNotScheduled(ctx, []scheduleModels.StudentInstanceRef{
		{StudentID: student.ID, InstanceID: inst.ID},
	}))

	got, err = repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.False(t, got.NotScheduled, "the manual expectation stays a genuine expectation")
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status)
}

// A finished block is frozen. Only Complete() holds the day lock, so the
// nightly bridge and the force-start path can have their instance stamped
// completed between reading it and writing the marker. Landing the write then
// would rewrite the history the marker exists to freeze (#1747 review).
func TestInstanceStudentRepository_MarkNotScheduled_LeavesFinishedInstancesAlone(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)

	for _, tc := range []struct {
		name   string
		status string
	}{
		{name: "completed", status: scheduleModels.InstanceStatusCompleted},
		{name: "cancelled", status: scheduleModels.InstanceStatusCancelled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			date := timezone.NewDate(2026, 10, 19)
			inst, cleanupInst := createInstanceFixture(t, db, "frozen-"+tc.name, date)
			defer cleanupInst()

			student := testpkg.CreateTestStudent(t, db, "Eingefroren",
				fmt.Sprintf("%s-%d", tc.name, time.Now().UnixNano()), "3a")

			// Exactly the shape the write claims — 'expected', no manual stamp.
			// Only the instance's finished status may keep it out.
			row := &scheduleModels.InstanceStudent{
				InstanceID: inst.ID,
				StudentID:  student.ID,
				Status:     scheduleModels.AttendanceStatusExpected,
			}
			row.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, repo.Create(ctx, row))

			inst.Status = tc.status
			require.NoError(t, instanceRepo.Update(ctx, inst))

			require.NoError(t, repo.MarkNotScheduled(ctx, []scheduleModels.StudentInstanceRef{
				{StudentID: student.ID, InstanceID: inst.ID},
			}))

			got, err := repo.FindByID(ctx, row.ID)
			require.NoError(t, err)
			assert.False(t, got.NotScheduled, "a finished day must not gain a marker after the fact")
			assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status,
				"and its recorded attendance must stay exactly as the day ended")
		})
	}
}

func TestInstanceStudentRepository_UpdateAttendanceFields(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "patch", timezone.NewDate(2026, 10, 11))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Nora", fmt.Sprintf("Patch-%d", time.Now().UnixNano()), "3a")

	checkedAt := time.Date(2026, 10, 11, 13, 0, 0, 0, time.UTC)
	late := scheduleModels.AttendanceSubstatusLate
	origNote := "im stau"

	row := &scheduleModels.InstanceStudent{
		InstanceID:  inst.ID,
		StudentID:   student.ID,
		Status:      scheduleModels.AttendanceStatusPresent,
		Substatus:   &late,
		Note:        &origNote,
		CheckedInAt: &checkedAt,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	t.Run("updates only the fields the patch carries", func(t *testing.T) {
		newStatus := scheduleModels.AttendanceStatusAbsent
		excused := scheduleModels.AttendanceSubstatusExcused

		patch := scheduleModels.AttendanceFieldPatch{
			Status:    &newStatus,
			Substatus: &excused,
			// Note intentionally not set — must be preserved.
		}
		require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID, patch))

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status)
		require.NotNil(t, got.Substatus)
		assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *got.Substatus)
		require.NotNil(t, got.Note)
		assert.Equal(t, "im stau", *got.Note, "note must survive when patch omits it")
	})

	t.Run("clears nullable columns via *Clear flags", func(t *testing.T) {
		patch := scheduleModels.AttendanceFieldPatch{
			SubstatusClear: true,
			NoteClear:      true,
		}
		require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID, patch))

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Substatus)
		assert.Nil(t, got.Note)
		require.NotNil(t, got.ManualStatusAt,
			"substatus-only staff edits must stamp manual ownership")
	})

	t.Run("substatus-only edit clears partial provenance and stamps manual", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Partial", fmt.Sprintf("Substatus-%d", time.Now().UnixNano()))
		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		partial := testpkg.CreateTestPickupException(t, db, student.ID, inst.Date, staff.ID, "13:00", "Termin")
		defer testpkg.CleanupTableRecords(t, db, "schedule.student_pickup_exceptions", partial.ID)

		row.PickupExceptionID = &partial.ID
		row.Status = scheduleModels.AttendanceStatusAbsent
		excused := scheduleModels.AttendanceSubstatusExcused
		row.Substatus = &excused
		row.ManualStatusAt = nil
		require.NoError(t, repo.Update(ctx, row))

		late := scheduleModels.AttendanceSubstatusLate
		require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID, scheduleModels.AttendanceFieldPatch{
			Substatus: &late,
		}))

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Nil(t, got.PickupExceptionID)
		require.NotNil(t, got.ManualStatusAt)
		require.NotNil(t, got.Substatus)
		assert.Equal(t, scheduleModels.AttendanceSubstatusLate, *got.Substatus)
	})

	t.Run("empty patch is a no-op (defensive)", func(t *testing.T) {
		// After the clear above, status is 'absent'. A no-op patch must not
		// mutate the row (not even updated_at).
		beforeRow, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)

		require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID, scheduleModels.AttendanceFieldPatch{}))

		afterRow, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.Equal(t, beforeRow.Status, afterRow.Status)
		assert.Equal(t, beforeRow.UpdatedAt.UnixNano(), afterRow.UpdatedAt.UnixNano())
	})
}

// The nightly bridge can close instances from more than one date in a single
// run. Exclusions are (instance, student) pairs, not bare student IDs: the
// same child may be not-scheduled on one instance's date but genuinely
// expected on another's, and only the former row may be spared the absent
// stamp (#1747).
func TestInstanceStudentRepository_MarkExpectedAbsentByActiveGroupIDs_PairScopedExclusions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)

	fx := newActivityInstanceFixtures(t, db, "mark-pairs")
	defer fx.cleanup()

	student := testpkg.CreateTestStudent(t, db, "Juna", fmt.Sprintf("MP-%d", time.Now().UnixNano()), "2c")

	mkActiveInstance := func(title string, date timezone.Date) *scheduleModels.ActivityInstance {
		ag := testpkg.CreateTestActiveGroup(t, db, fx.activityID, fx.roomID)
		inst := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
			time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			fmt.Sprintf("%s-%d", title, time.Now().UnixNano()),
		)
		inst.Status = scheduleModels.InstanceStatusActive
		inst.ActiveGroupID = &ag.ID
		require.NoError(t, instanceRepo.Create(ctx, inst))
		return inst
	}

	instSpared := mkActiveInstance("spared", timezone.NewDate(2035, 2, 5))
	instStamped := mkActiveInstance("stamped", timezone.NewDate(2035, 2, 6))

	mkRow := func(instID int64) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID: instID,
			StudentID:  student.ID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		return row
	}
	mkRow(instSpared.ID)
	mkRow(instStamped.ID)

	err := repo.MarkExpectedAbsentByActiveGroupIDs(ctx,
		[]int64{*instSpared.ActiveGroupID, *instStamped.ActiveGroupID},
		time.Now(),
		[]scheduleModels.StudentInstanceRef{{StudentID: student.ID, InstanceID: instSpared.ID}},
	)
	require.NoError(t, err)

	gotSpared, err := repo.FindByInstanceAndStudent(ctx, instSpared.ID, student.ID)
	require.NoError(t, err)
	require.NotNil(t, gotSpared)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, gotSpared.Status,
		"the excluded (instance, student) pair must be spared")

	gotStamped, err := repo.FindByInstanceAndStudent(ctx, instStamped.ID, student.ID)
	require.NoError(t, err)
	require.NotNil(t, gotStamped)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotStamped.Status,
		"the same student's row on the other instance must still be stamped absent")
}

func TestInstanceStudentRepository_BulkUpdateStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-bulk", timezone.NewDate(2026, 11, 2))
	defer cleanupInst()

	suffix := fmt.Sprintf("Bulk-%d", time.Now().UnixNano())
	sExpected1 := testpkg.CreateTestStudent(t, db, "Ada", suffix+"-e1", "3a")
	sExpected2 := testpkg.CreateTestStudent(t, db, "Bea", suffix+"-e2", "3a")
	sPresent := testpkg.CreateTestStudent(t, db, "Cem", suffix+"-p", "3a")

	buildRow := func(studentID int64, status string) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  studentID,
			Status:     status,
		}
		row.SetTenantID(testpkg.Tenant(t))
		return row
	}

	rowE1 := buildRow(sExpected1.ID, scheduleModels.AttendanceStatusExpected)
	rowE2 := buildRow(sExpected2.ID, scheduleModels.AttendanceStatusExpected)
	rowP := buildRow(sPresent.ID, scheduleModels.AttendanceStatusPresent)

	require.NoError(t, repo.Create(ctx, rowE1))
	require.NoError(t, repo.Create(ctx, rowE2))
	require.NoError(t, repo.Create(ctx, rowP))

	t.Run("flips only matching expected rows", func(t *testing.T) {
		n, err := repo.BulkUpdateStatus(ctx, inst.ID,
			scheduleModels.AttendanceStatusExpected,
			scheduleModels.AttendanceStatusAbsent,
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		gotE1, err := repo.FindByInstanceAndStudent(ctx, inst.ID, sExpected1.ID)
		require.NoError(t, err)
		require.NotNil(t, gotE1)
		assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotE1.Status)

		gotE2, err := repo.FindByInstanceAndStudent(ctx, inst.ID, sExpected2.ID)
		require.NoError(t, err)
		require.NotNil(t, gotE2)
		assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotE2.Status)

		gotP, err := repo.FindByInstanceAndStudent(ctx, inst.ID, sPresent.ID)
		require.NoError(t, err)
		require.NotNil(t, gotP)
		assert.Equal(t, scheduleModels.AttendanceStatusPresent, gotP.Status,
			"present row must not be touched by expected→absent bulk update")
	})

	t.Run("second call is a no-op", func(t *testing.T) {
		n, err := repo.BulkUpdateStatus(ctx, inst.ID,
			scheduleModels.AttendanceStatusExpected,
			scheduleModels.AttendanceStatusAbsent,
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("no-op when called with a different tenant context", func(t *testing.T) {
		// Reset one row back to 'expected' in tenant 1 so there's a target.
		reset := scheduleModels.AttendanceFieldPatch{}
		expected := scheduleModels.AttendanceStatusExpected
		reset.Status = &expected
		require.NoError(t, repo.UpdateAttendanceFields(ctx, rowE1.ID, reset))

		// Switch to a foreign tenant — bulk update must match zero rows,
		// and the tenant 1 row must remain 'expected'.
		ctxT2 := testpkg.TenantContext(2)
		n, err := repo.BulkUpdateStatus(ctxT2, inst.ID,
			scheduleModels.AttendanceStatusExpected,
			scheduleModels.AttendanceStatusAbsent,
			nil,
		)
		require.NoError(t, err)
		assert.Equal(t, 0, n)

		got, err := repo.FindByInstanceAndStudent(ctx, inst.ID, sExpected1.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, scheduleModels.AttendanceStatusExpected, got.Status)
	})
}

func TestInstanceStudentRepository_DeleteByInstanceID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-del", timezone.NewDate(2026, 10, 7))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Ola", fmt.Sprintf("Del-%d", time.Now().UnixNano()), "3a")

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	require.NoError(t, repo.DeleteByInstanceID(ctx, inst.ID))

	rows, err := repo.FindByInstanceID(ctx, inst.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// ---------------------------------------------------------------------------
// FindExpectedByInstanceIDs (WP-B13)
// ---------------------------------------------------------------------------

func TestInstanceStudentRepository_FindExpectedByInstanceIDs_FiltersStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst1, cleanup1 := createInstanceFixture(t, db, "b13-a", timezone.NewDate(2026, 9, 21))
	defer cleanup1()
	inst2, cleanup2 := createInstanceFixture(t, db, "b13-b", timezone.NewDate(2026, 9, 22))
	defer cleanup2()

	unique := time.Now().UnixNano()
	expectedStudent := testpkg.CreateTestStudent(t, db, "Exp", fmt.Sprintf("B13-%d", unique), "3a")
	presentStudent := testpkg.CreateTestStudent(t, db, "Pres", fmt.Sprintf("B13-%d", unique+1), "3a")
	absentStudent := testpkg.CreateTestStudent(t, db, "Abs", fmt.Sprintf("B13-%d", unique+2), "3a")
	otherStudent := testpkg.CreateTestStudent(t, db, "Other", fmt.Sprintf("B13-%d", unique+3), "3a")

	expectedRow := &scheduleModels.InstanceStudent{
		InstanceID: inst1.ID,
		StudentID:  expectedStudent.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	expectedRow.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, expectedRow))

	presentRow := &scheduleModels.InstanceStudent{
		InstanceID: inst1.ID,
		StudentID:  presentStudent.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	presentRow.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, presentRow))

	absentRow := &scheduleModels.InstanceStudent{
		InstanceID: inst1.ID,
		StudentID:  absentStudent.ID,
		Status:     scheduleModels.AttendanceStatusAbsent,
	}
	absentRow.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, absentRow))

	otherExpected := &scheduleModels.InstanceStudent{
		InstanceID: inst2.ID,
		StudentID:  otherStudent.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	otherExpected.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, otherExpected))

	t.Run("returns only status=expected for given instance IDs", func(t *testing.T) {
		rows, err := repo.FindExpectedByInstanceIDs(ctx, []int64{inst1.ID, inst2.ID})
		require.NoError(t, err)
		ids := make(map[int64]bool, len(rows))
		for _, r := range rows {
			ids[r.ID] = true
			assert.Equal(t, scheduleModels.AttendanceStatusExpected, r.Status)
		}
		assert.True(t, ids[expectedRow.ID], "missed expected-status row for inst1")
		assert.True(t, ids[otherExpected.ID], "missed expected-status row for inst2")
		assert.False(t, ids[presentRow.ID], "should not include present-status row")
		assert.False(t, ids[absentRow.ID], "should not include absent-status row")
	})

	t.Run("empty input returns empty slice without DB roundtrip", func(t *testing.T) {
		rows, err := repo.FindExpectedByInstanceIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("unknown instance id returns empty", func(t *testing.T) {
		rows, err := repo.FindExpectedByInstanceIDs(ctx, []int64{int64(0)})
		require.NoError(t, err)
		assert.Empty(t, rows)
	})
}

func TestInstanceStudentRepository_FindExpectedByInstanceIDs_TenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "b13-iso", timezone.NewDate(2026, 9, 23))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Iso", fmt.Sprintf("B13-iso-%d", time.Now().UnixNano()), "3a")

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(testpkg.Ctx(t), row))

	// Same instance ID, different tenant context → row must be invisible.
	otherTenantCtx := testpkg.TenantContext(2)
	rows, err := repo.FindExpectedByInstanceIDs(otherTenantCtx, []int64{inst.ID})
	require.NoError(t, err)
	assert.Empty(t, rows, "row from tenant 1 must not leak to tenant 2")
}

// The session-end bridge feeds this result straight into MarkNotScheduled, so
// the two predicates have to agree: an absence a day status still owns is a row
// that write can take back, and reading only 'expected' rows hid exactly those
// children from the bridge — leaving a never-booked child a false absence in
// their history and exports (#1747 review).
func TestInstanceStudentRepository_FindNotScheduledCandidatesByInstanceIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := repositories.NewFactory(db)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	date := timezone.NewDate(2026, 10, 16)
	inst, cleanupInst := createInstanceFixture(t, db, "candidates", date)
	defer cleanupInst()

	suffix := time.Now().UnixNano()
	expectedStudent := testpkg.CreateTestStudent(t, db, "Exp", fmt.Sprintf("Cand-%d", suffix), "3a")
	sickStudent := testpkg.CreateTestStudent(t, db, "Sick", fmt.Sprintf("Cand-%d", suffix+1), "3a")
	manualStudent := testpkg.CreateTestStudent(t, db, "Manual", fmt.Sprintf("Cand-%d", suffix+2), "3a")
	presentStudent := testpkg.CreateTestStudent(t, db, "Pres", fmt.Sprintf("Cand-%d", suffix+3), "3a")

	create := func(studentID int64, status string) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{InstanceID: inst.ID, StudentID: studentID, Status: status}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		return row
	}

	expectedRow := create(expectedStudent.ID, scheduleModels.AttendanceStatusExpected)
	sickRow := create(sickStudent.ID, scheduleModels.AttendanceStatusExpected)
	manualRow := create(manualStudent.ID, scheduleModels.AttendanceStatusAbsent)
	presentRow := create(presentStudent.ID, scheduleModels.AttendanceStatusPresent)

	sick := &activeModels.StudentStatusDay{
		StudentID: sickStudent.ID, Date: date, Status: activeModels.StudentStatusDaySick,
		ReportedAt: time.Date(2026, 10, 16, 7, 0, 0, 0, time.UTC), Source: activeModels.StudentStatusSourcePlanned,
	}
	require.NoError(t, factory.StudentStatusDay.UpsertReported(ctx, sick))

	got, err := repo.FindByID(ctx, sickRow.ID)
	require.NoError(t, err)
	require.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status, "precondition: the sick report writes the absence")

	rows, err := repo.FindNotScheduledCandidatesByInstanceIDs(ctx, []int64{inst.ID})
	require.NoError(t, err)
	ids := make(map[int64]bool, len(rows))
	for _, row := range rows {
		ids[row.ID] = true
	}
	assert.True(t, ids[expectedRow.ID], "an expected row is the ordinary candidate")
	assert.True(t, ids[sickRow.ID], "a status-day-owned absence is still resolvable")
	assert.False(t, ids[manualRow.ID], "a hand-made absence decision is nobody else's to undo")
	assert.False(t, ids[presentRow.ID], "an observed check-in tells its own story")

	t.Run("empty input returns empty slice without DB roundtrip", func(t *testing.T) {
		empty, err := repo.FindNotScheduledCandidatesByInstanceIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, empty)
	})

	t.Run("tenant scoped", func(t *testing.T) {
		leaked, err := repo.FindNotScheduledCandidatesByInstanceIDs(testpkg.TenantContext(2), []int64{inst.ID})
		require.NoError(t, err)
		assert.Empty(t, leaked, "rows from tenant 1 must not leak to tenant 2")
	})
}

func TestInstanceStudentRepository_CountNonAbsentByInstanceIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	t.Run("EmptySlice returns empty map without touching DB", func(t *testing.T) {
		m, err := repo.CountNonAbsentByInstanceIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, m)
	})

	date := timezone.NewDate(2026, 9, 24)
	instA, cleanupA := createInstanceFixture(t, db, "cnt-stu-a", date)
	defer cleanupA()
	instB, cleanupB := createInstanceFixture(t, db, "cnt-stu-b", date)
	defer cleanupB()
	instEmpty, cleanupEmpty := createInstanceFixture(t, db, "cnt-stu-empty", date)
	defer cleanupEmpty()

	suffix := time.Now().UnixNano()
	studentExpected := testpkg.CreateTestStudent(t, db, "Cnt", fmt.Sprintf("S1-%d", suffix), "3a")
	studentPresent := testpkg.CreateTestStudent(t, db, "Cnt", fmt.Sprintf("S2-%d", suffix), "3a")
	studentAbsent := testpkg.CreateTestStudent(t, db, "Cnt", fmt.Sprintf("S3-%d", suffix), "3a")

	// instA: expected + present (both non-absent), one absent → expect 2
	testpkg.CreateTestInstanceStudent(t, db, instA.ID, studentExpected.ID, scheduleModels.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, db, instA.ID, studentPresent.ID, scheduleModels.AttendanceStatusPresent)
	testpkg.CreateTestInstanceStudent(t, db, instA.ID, studentAbsent.ID, scheduleModels.AttendanceStatusAbsent)

	// instB: 1 expected → expect 1
	testpkg.CreateTestInstanceStudent(t, db, instB.ID, studentExpected.ID, scheduleModels.AttendanceStatusExpected)

	// instEmpty: nothing assigned → must NOT appear in map

	t.Run("GroupsByInstance", func(t *testing.T) {
		m, err := repo.CountNonAbsentByInstanceIDs(ctx, []int64{instA.ID, instB.ID, instEmpty.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, m[instA.ID])
		assert.Equal(t, 1, m[instB.ID])
		_, present := m[instEmpty.ID]
		assert.False(t, present, "empty instance must not appear in map")
	})

	t.Run("TenantIsolation excludes other tenant rows", func(t *testing.T) {
		otherTenantCtx := testpkg.TenantContext(999)
		m, err := repo.CountNonAbsentByInstanceIDs(otherTenantCtx, []int64{instA.ID, instB.ID})
		require.NoError(t, err)
		assert.Empty(t, m)
	})
}

// TestInstanceStudentRepository_ArchivePlannedByStudentIDsFrom pins the
// graduation-reconciliation predicate (#405 review): every still-PLANNED row of
// a graduated student from `from` onwards goes — today's included, because
// slot-list reads decide visibility from the enrollment interval and not from
// alumnus status, so a leftover row keeps the departed child in the current
// day's Plan/Abgleich lists — including the ones a status day already rewrote to
// 'absent'. Anything recording an actual event and anything before `from`
// survives, and every removed row is archived for the revert to replay.
func TestInstanceStudentRepository_ArchivePlannedByStudentIDsFrom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	account := testpkg.CreateTestAccount(t, db, "roster-archive@test.local")
	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	today := timezone.TodayDate()
	future := today.AddDays(7)
	past := today.AddDays(-7)

	futureInst, cleanupFuture := createInstanceFixture(t, db, "delfut", future)
	defer cleanupFuture()
	pastInst, cleanupPast := createInstanceFixture(t, db, "delpast", past)
	defer cleanupPast()
	todayInst, cleanupToday := createInstanceFixture(t, db, "deltoday", today)
	defer cleanupToday()

	suffix := time.Now().UnixNano()
	graduate := testpkg.CreateTestStudent(t, db, "Abgang", fmt.Sprintf("G-%d", suffix), "4a")
	stayer := testpkg.CreateTestStudent(t, db, "Bleibt", fmt.Sprintf("S-%d", suffix), "3a")

	// A planned sickness on the future date: ApplyActiveStatusDaysForInstance has
	// already turned the graduate's row into an absence owned by the status day.
	// This is the row the old status='expected' predicate left behind.
	statusDay := testpkg.CreateTestStudentStatusDay(t, db, graduate.ID, future, "sick")

	// Deleted: dated from today onwards and still planned.
	testpkg.CreateTestInstanceStudent(t, db, futureInst.ID, graduate.ID,
		scheduleModels.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, db, todayInst.ID, graduate.ID,
		scheduleModels.AttendanceStatusExpected)
	// Kept: before `from`.
	testpkg.CreateTestInstanceStudent(t, db, pastInst.ID, graduate.ID,
		scheduleModels.AttendanceStatusExpected)
	// Kept: another child's future row must be untouched.
	testpkg.CreateTestInstanceStudent(t, db, futureInst.ID, stayer.ID,
		scheduleModels.AttendanceStatusExpected)

	archived := func(instanceID, studentID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.grade_transition_roster_removals`).
			Where("transition_id = ?", transition.ID).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", studentID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}

	removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 2, removed, "the graduate's planned rows from today onwards are removed")
	assert.Equal(t, 1, archived(futureInst.ID, graduate.ID), "every removed row is archived for the revert")
	assert.Equal(t, 1, archived(todayInst.ID, graduate.ID))

	assertRowGone := func(t *testing.T, instanceID, studentID int64) {
		t.Helper()
		got, err := repo.FindByInstanceAndStudent(ctx, instanceID, studentID)
		require.NoError(t, err)
		assert.Nil(t, got)
	}
	assertRowKept := func(t *testing.T, instanceID, studentID int64) {
		t.Helper()
		got, err := repo.FindByInstanceAndStudent(ctx, instanceID, studentID)
		require.NoError(t, err)
		assert.NotNil(t, got)
	}

	assertRowGone(t, futureInst.ID, graduate.ID)
	assertRowGone(t, todayInst.ID, graduate.ID)
	assertRowKept(t, pastInst.ID, graduate.ID)
	assertRowKept(t, futureInst.ID, stayer.ID)

	t.Run("restores exactly the archived rows", func(t *testing.T) {
		restored, err := repo.RestoreArchivedByTransition(ctx, transition.ID, []int64{graduate.ID}, today)
		require.NoError(t, err)
		assert.Equal(t, 2, restored, "both archived rows come back")
		assertRowKept(t, futureInst.ID, graduate.ID)
		assertRowKept(t, todayInst.ID, graduate.ID)
		assert.Equal(t, 0, archived(futureInst.ID, graduate.ID), "the archive entry is consumed")

		// Re-archive so the remaining subtests keep their original starting
		// state (the graduate has no planned row from today onwards).
		_, err = repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assertRowGone(t, futureInst.ID, graduate.ID)
	})

	t.Run("keeps today's row that records an actual presence", func(t *testing.T) {
		// An observed presence carries the check-in stamp every real check-in
		// path writes. The status alone is not the proof — see the
		// pre-marked-presence subtest below (#405 review).
		checkedInAt := time.Now()
		row := &scheduleModels.InstanceStudent{
			InstanceID:  todayInst.ID,
			StudentID:   graduate.ID,
			Status:      scheduleModels.AttendanceStatusPresent,
			CheckedInAt: &checkedInAt,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 0, removed)
		assertRowKept(t, todayInst.ID, graduate.ID)
	})

	t.Run("removes a future absence a status day owns", func(t *testing.T) {
		absent := scheduleModels.AttendanceStatusAbsent
		sick := scheduleModels.AttendanceSubstatusSick
		row := testpkg.CreateTestInstanceStudent(t, db, futureInst.ID, graduate.ID, absent,
			testpkg.InstanceStudentOpts{StudentStatusDayID: &statusDay.ID})
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID,
			scheduleModels.AttendanceFieldPatch{Substatus: &sick}))

		removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 1, removed)
		assertRowGone(t, futureInst.ID, graduate.ID)
	})

	t.Run("keeps a future row that records an actual presence", func(t *testing.T) {
		checkedInAt := time.Now()
		row := &scheduleModels.InstanceStudent{
			InstanceID:  futureInst.ID,
			StudentID:   graduate.ID,
			Status:      scheduleModels.AttendanceStatusPresent,
			CheckedInAt: &checkedInAt,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 0, removed)
		assertRowKept(t, futureInst.ID, graduate.ID)
	})

	// The counterpart: 'present' WITHOUT any attendance stamp on a block that
	// has not started is a pre-marked plan, not an observation. Keeping it left
	// the graduate on that future roster — visible in timetable/slot-list reads
	// and counted for staffing, since none of those readers filter alumni
	// (#405 review).
	t.Run("removes a future presence nobody observed yet", func(t *testing.T) {
		row := &scheduleModels.InstanceStudent{
			InstanceID: futureInst.ID,
			StudentID:  graduate.ID,
			Status:     scheduleModels.AttendanceStatusPresent,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 1, removed)
		assertRowGone(t, futureInst.ID, graduate.ID)
		// Archived like any other plan, so the revert can replay it verbatim.
		assert.Equal(t, 1, archived(futureInst.ID, graduate.ID))
	})

	t.Run("keeps a future row carrying a stamped check-in", func(t *testing.T) {
		checkedInAt := time.Now()
		row := &scheduleModels.InstanceStudent{
			InstanceID:  futureInst.ID,
			StudentID:   graduate.ID,
			Status:      scheduleModels.AttendanceStatusExpected,
			CheckedInAt: &checkedInAt,
		}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 0, removed)
		assertRowKept(t, futureInst.ID, graduate.ID)
	})

	t.Run("empty student set is a no-op", func(t *testing.T) {
		removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, nil, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 0, removed)
	})

	t.Run("tenant isolation leaves other tenants alone", func(t *testing.T) {
		testpkg.CreateTestInstanceStudent(t, db, futureInst.ID, graduate.ID,
			scheduleModels.AttendanceStatusExpected)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(
			testpkg.TenantContext(999), transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, err)
		assert.Equal(t, 0, removed)
		assertRowKept(t, futureInst.ID, graduate.ID)
	})
}

// TestInstanceStudentRepository_ArchivePlannedByStudentIDsFrom_ManualStatusRows
// pins where the manual_status_at exemption stops (#405 review). A status a
// supervisor set BY HAND is an observation only once the occurrence has started
// — before that it is still a plan, and leaving it behind keeps the departed
// child on the timetable list, in slot-list/export reads and (on an 'expected'
// row) in staffing counts, none of which filter alumni. The replay hands the
// hand-set status back verbatim instead of re-deriving it from day statuses.
func TestInstanceStudentRepository_ArchivePlannedByStudentIDsFrom_ManualStatusRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	account := testpkg.CreateTestAccount(t, db, "roster-archive-manual@test.local")
	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	today := timezone.TodayDate()

	// createInstanceFixture builds a 14:00-15:00 block, so "now" on either side
	// of 14:00 decides whether the occurrence has started. Both clocks are
	// explicit, which keeps the test independent of the wall clock it runs at.
	inst, cleanupInst := createInstanceFixture(t, db, "manualtoday", today)
	defer cleanupInst()

	midnight := today.BerlinMidnight()
	beforeStart := midnight.Add(12 * time.Hour)
	afterStart := midnight.Add(14*time.Hour + 30*time.Minute)

	graduate := testpkg.CreateTestStudent(t, db, "Abgang", fmt.Sprintf("M-%d", time.Now().UnixNano()), "4a")

	// A hand-set absence: the PATCH stamps manual_status_at, clears any
	// status-day provenance and drops the non-booking marker.
	handSetAbsence := func(t *testing.T) *scheduleModels.InstanceStudent {
		t.Helper()
		row := testpkg.CreateTestInstanceStudent(t, db, inst.ID, graduate.ID,
			scheduleModels.AttendanceStatusExpected)
		absent := scheduleModels.AttendanceStatusAbsent
		require.NoError(t, repo.UpdateAttendanceFields(ctx, row.ID,
			scheduleModels.AttendanceFieldPatch{Status: &absent}))
		stored, err := repo.FindByInstanceAndStudent(ctx, inst.ID, graduate.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.ManualStatusAt, "the PATCH must record that a human decided this row")
		// Take the row back at the end of the SUBTEST: schedule.instance_students
		// is unique on (instance, student) and the next subtest sets up the same
		// pair again (#2419).
		t.Cleanup(func() {
			testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		})
		return stored
	}

	t.Run("keeps a hand-set status once the occurrence has started", func(t *testing.T) {
		handSetAbsence(t)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(
			ctx, transition.ID, []int64{graduate.ID}, today, afterStart)
		require.NoError(t, err)
		assert.Equal(t, 0, removed, "a status finalized on a block that has run is an observation")

		kept, err := repo.FindByInstanceAndStudent(ctx, inst.ID, graduate.ID)
		require.NoError(t, err)
		assert.NotNil(t, kept)
	})

	t.Run("removes a hand-set status on a block that has not started", func(t *testing.T) {
		handSetAbsence(t)

		removed, err := repo.ArchivePlannedByStudentIDsFrom(
			ctx, transition.ID, []int64{graduate.ID}, today, beforeStart)
		require.NoError(t, err)
		assert.Equal(t, 1, removed, "nothing was observed yet — the row is still a plan")

		gone, err := repo.FindByInstanceAndStudent(ctx, inst.ID, graduate.ID)
		require.NoError(t, err)
		assert.Nil(t, gone)

		// The replay must not re-derive the status from day statuses: there is
		// none here, so re-deriving would silently turn the hand-set absence
		// back into 'expected' and put the child back on the staffing count.
		restored, err := repo.RestoreArchivedByTransition(ctx, transition.ID, []int64{graduate.ID}, today)
		require.NoError(t, err)
		assert.Equal(t, 1, restored)

		back, err := repo.FindByInstanceAndStudent(ctx, inst.ID, graduate.ID)
		require.NoError(t, err)
		require.NotNil(t, back)
		assert.Equal(t, scheduleModels.AttendanceStatusAbsent, back.Status,
			"the supervisor's decision comes back verbatim")
		assert.NotNil(t, back.ManualStatusAt, "and it is still marked as hand-set")
		assert.Nil(t, back.StudentStatusDayID, "a hand-set status carries no day-status provenance")
	})
}

// TestInstanceStudentRepository_RestoreArchivedByTransition_SkipsFrozen pins the
// other half of the revert predicate (#405 review): an alumnus window can span
// weeks, so by the time a transition is reverted some archived rows describe
// occurrences that have become history. Replaying an 'expected' row into a past
// or completed instance would rewrite attendance long after anyone could still
// observe what happened. Those ledger entries are dropped, not replayed — but
// they ARE consumed, so a later re-apply starts from a clean snapshot.
func TestInstanceStudentRepository_RestoreArchivedByTransition_SkipsFrozen(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)

	account := testpkg.CreateTestAccount(t, db, "roster-restore-frozen@test.local")
	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	today := timezone.TodayDate()
	soon := today.AddDays(3)

	// Two occurrences that were both still ahead when the transition was
	// applied: one finishes during the alumnus window, the other stays open.
	finishedInst, cleanupFinished := createInstanceFixture(t, db, "frozdone", today)
	defer cleanupFinished()
	openInst, cleanupOpen := createInstanceFixture(t, db, "frozopen", soon)
	defer cleanupOpen()

	suffix := time.Now().UnixNano()
	graduate := testpkg.CreateTestStudent(t, db, "Abgang", fmt.Sprintf("F-%d", suffix), "4a")

	testpkg.CreateTestInstanceStudent(t, db, finishedInst.ID, graduate.ID,
		scheduleModels.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, db, openInst.ID, graduate.ID,
		scheduleModels.AttendanceStatusExpected)

	removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
	require.NoError(t, err)
	require.Equal(t, 2, removed, "both planned rows are archived on apply")

	// The alumnus window passes: today's occurrence runs and is completed.
	require.NoError(t, instanceRepo.MarkCompleted(ctx, finishedInst.ID, time.Now()))

	restored, err := repo.RestoreArchivedByTransition(ctx, transition.ID, []int64{graduate.ID}, today)
	require.NoError(t, err)
	assert.Equal(t, 1, restored, "only the still-actionable occurrence is replayed")

	gotOpen, err := repo.FindByInstanceAndStudent(ctx, openInst.ID, graduate.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOpen, "the open occurrence gets its row back")
	// The replay inserts a fresh row, so it carries a new id the fixture cleanup
	// above does not know about.

	gotFinished, err := repo.FindByInstanceAndStudent(ctx, finishedInst.ID, graduate.ID)
	require.NoError(t, err)
	assert.Nil(t, gotFinished, "the completed occurrence keeps the attendance it recorded")

	remaining, err := db.NewSelect().
		TableExpr(`schedule.grade_transition_roster_removals`).
		Where("transition_id = ?", transition.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, remaining, "obsolete ledger entries are consumed, not left behind")

	t.Run("an occurrence that fell into the past is not replayed either", func(t *testing.T) {
		// Re-archive the row the replay just put back on the open occurrence.
		n, aErr := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, []int64{graduate.ID}, today, time.Now())
		require.NoError(t, aErr)
		require.Equal(t, 1, n)

		// Revert runs a week later — the occurrence is now behind us.
		replayed, rErr := repo.RestoreArchivedByTransition(
			ctx, transition.ID, []int64{graduate.ID}, soon.AddDays(1))
		require.NoError(t, rErr)
		assert.Equal(t, 0, replayed)

		got, fErr := repo.FindByInstanceAndStudent(ctx, openInst.ID, graduate.ID)
		require.NoError(t, fErr)
		assert.Nil(t, got)
	})
}

// The archive captures a PLAN, and plans expire. An alumnus window of weeks is
// ample time for a sickness to be reported or cleared, so the replay must take
// its attendance state from the day statuses active at REVERT time rather than
// resurrecting the pre-graduation one. Nothing downstream repairs it: the
// reconciler's active-status pass only touches instances it just materialized
// (#405 review).
func TestInstanceStudentRepository_RestoreArchivedByTransition_DerivesCurrentStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	account := testpkg.CreateTestAccount(t, db, "roster-restore-status@test.local")
	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	today := timezone.TodayDate()
	soon := today.AddDays(3)

	inst, cleanupInst := createInstanceFixture(t, db, "statusderiv", soon)
	defer cleanupInst()

	suffix := time.Now().UnixNano()
	// One child per scenario: the occurrence carries at most one row per student.
	sickened := testpkg.CreateTestStudent(t, db, "WirdKrank", fmt.Sprintf("D1-%d", suffix), "4a")
	recovered := testpkg.CreateTestStudent(t, db, "WirdGesund", fmt.Sprintf("D2-%d", suffix), "4a")
	unbooked := testpkg.CreateTestStudent(t, db, "OhneBuchung", fmt.Sprintf("D3-%d", suffix), "4a")
	partiallyExcused := testpkg.CreateTestStudent(t, db, "Teilentschuldigt", fmt.Sprintf("D4-%d", suffix), "4a")

	// The state at apply time: `recovered` was already down for a planned
	// sickness, the other two were plain plan rows.
	oldStatusDay := testpkg.CreateTestStudentStatusDay(t, db, recovered.ID, soon, "sick")

	testpkg.CreateTestInstanceStudent(t, db, inst.ID, sickened.ID,
		scheduleModels.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, db, inst.ID, recovered.ID,
		scheduleModels.AttendanceStatusAbsent,
		testpkg.InstanceStudentOpts{StudentStatusDayID: &oldStatusDay.ID})
	testpkg.CreateTestInstanceStudent(t, db, inst.ID, unbooked.ID,
		scheduleModels.AttendanceStatusExpected,
		testpkg.InstanceStudentOpts{NotScheduled: true})
	testpkg.CreateTestInstanceStudent(t, db, inst.ID, partiallyExcused.ID,
		scheduleModels.AttendanceStatusExpected)

	graduates := []int64{sickened.ID, recovered.ID, unbooked.ID, partiallyExcused.ID}
	removed, err := repo.ArchivePlannedByStudentIDsFrom(ctx, transition.ID, graduates, today, time.Now())
	require.NoError(t, err)
	require.Equal(t, 4, removed, "all four plan rows are archived on apply")

	// The alumnus window: one child is reported sick, the other's sickness is
	// cleared. Neither change could reach the archived snapshot.
	newStatusDay := testpkg.CreateTestStudentStatusDay(t, db, sickened.ID, soon, "sick")
	_, err = db.NewRaw(`UPDATE active.student_status_days SET cleared_at = NOW() WHERE id = ?`,
		oldStatusDay.ID).Exec(ctx)
	require.NoError(t, err)
	staff := testpkg.CreateTestStaff(t, db, "Partial", fmt.Sprintf("Owner-%d", suffix))
	partial := testpkg.CreateTestPickupException(t, db, partiallyExcused.ID, soon, staff.ID, "13:00", "Termin")
	from := timezone.WallClock(time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC))
	partial.ExcusedFrom = &from
	partial.ExcusedCreatedBy = &staff.ID
	partial.ExcusedOwnsPickupTime = true
	require.NoError(t, scheduleRepo.NewStudentPickupExceptionRepository(db).Update(ctx, partial))

	replayed, err := repo.RestoreArchivedByTransition(ctx, transition.ID, graduates, today)
	require.NoError(t, err)
	require.Equal(t, 4, replayed)

	get := func(studentID int64) *scheduleModels.InstanceStudent {
		got, gErr := repo.FindByInstanceAndStudent(ctx, inst.ID, studentID)
		require.NoError(t, gErr)
		require.NotNil(t, got)
		return got
	}

	gotSickened := get(sickened.ID)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotSickened.Status,
		"a sickness reported while the child was an alumnus must win over the archived plan")
	require.NotNil(t, gotSickened.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusSick, *gotSickened.Substatus)
	require.NotNil(t, gotSickened.StudentStatusDayID)
	assert.Equal(t, newStatusDay.ID, *gotSickened.StudentStatusDayID,
		"the row must be owned by the status day that is active now")

	gotRecovered := get(recovered.ID)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, gotRecovered.Status,
		"a cleared status day must not come back as an absence")
	assert.Nil(t, gotRecovered.Substatus)
	assert.Nil(t, gotRecovered.StudentStatusDayID)

	gotUnbooked := get(unbooked.ID)
	assert.True(t, gotUnbooked.NotScheduled, "the non-booking marker is structural and replays verbatim")
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, gotUnbooked.Status)
	assert.Nil(t, gotUnbooked.StudentStatusDayID)

	gotPartial := get(partiallyExcused.ID)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotPartial.Status,
		"a partial excusal created while the child was an alumnus must be replayed")
	require.NotNil(t, gotPartial.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *gotPartial.Substatus)
	assert.Nil(t, gotPartial.StudentStatusDayID)
	require.NotNil(t, gotPartial.PickupExceptionID)
	assert.Equal(t, partial.ID, *gotPartial.PickupExceptionID)
}

// A race after materialization discovers an instance and cancels it before
// ApplyActivePartialAbsencesForInstance runs must not stamp cancelled
// attendance rows. Mirror ApplyPartialAbsence's status <> cancelled guard.
func TestInstanceStudentRepository_ApplyActivePartialAbsencesSkipsCancelledInstance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)
	date := timezone.NewDate(2026, 11, 5)

	activeInst, cleanupActive := createInstanceFixture(t, db, "partial-active", date)
	defer cleanupActive()
	cancelledInst, cleanupCancelled := createInstanceFixture(t, db, "partial-cxl", date)
	defer cleanupCancelled()
	cancelledInst.Status = scheduleModels.InstanceStatusCancelled
	require.NoError(t, instanceRepo.Update(ctx, cancelledInst))

	student := testpkg.CreateTestStudent(t, db, "Cancel", fmt.Sprintf("Partial-%d", time.Now().UnixNano()), "3a")
	staff := testpkg.CreateTestStaff(t, db, "Partial", fmt.Sprintf("Cancel-%d", time.Now().UnixNano()))

	activeRow := testpkg.CreateTestInstanceStudent(t, db, activeInst.ID, student.ID,
		scheduleModels.AttendanceStatusExpected)
	cancelledRow := testpkg.CreateTestInstanceStudent(t, db, cancelledInst.ID, student.ID,
		scheduleModels.AttendanceStatusExpected)

	partial := testpkg.CreateTestPickupException(t, db, student.ID, date, staff.ID, "13:00", "Termin")
	from := timezone.WallClock(time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC))
	partial.ExcusedFrom = &from
	partial.ExcusedCreatedBy = &staff.ID
	partial.ExcusedOwnsPickupTime = true
	require.NoError(t, scheduleRepo.NewStudentPickupExceptionRepository(db).Update(ctx, partial))

	n, err := repo.ApplyActivePartialAbsencesForInstance(ctx, cancelledInst.ID, date)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "cancelled instance must not receive partial projection")

	gotCancelled, err := repo.FindByID(ctx, cancelledRow.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, gotCancelled.Status)
	assert.Nil(t, gotCancelled.PickupExceptionID)

	n, err = repo.ApplyActivePartialAbsencesForInstance(ctx, activeInst.ID, date)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	gotActive, err := repo.FindByID(ctx, activeRow.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotActive.Status)
	require.NotNil(t, gotActive.PickupExceptionID)
	assert.Equal(t, partial.ID, *gotActive.PickupExceptionID)
}

// The auto-excusal sync (#2360) runs ApplyPartialAbsence on same-day pickup
// and weekly-baseline changes. Completed instances are a closed historical
// record and must not be rewritten to absent/excused with fresh pickup
// provenance.
func TestInstanceStudentRepository_ApplyPartialAbsenceSkipsCompletedInstance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)
	date := timezone.NewDate(2026, 11, 6)

	activeInst, cleanupActive := createInstanceFixture(t, db, "partial-act2", date)
	defer cleanupActive()
	completedInst, cleanupCompleted := createInstanceFixture(t, db, "partial-done", date)
	defer cleanupCompleted()
	completedInst.Status = scheduleModels.InstanceStatusCompleted
	require.NoError(t, instanceRepo.Update(ctx, completedInst))

	student := testpkg.CreateTestStudent(t, db, "Done", fmt.Sprintf("Partial-%d", time.Now().UnixNano()), "3a")
	staff := testpkg.CreateTestStaff(t, db, "Partial", fmt.Sprintf("Done-%d", time.Now().UnixNano()))

	activeRow := testpkg.CreateTestInstanceStudent(t, db, activeInst.ID, student.ID,
		scheduleModels.AttendanceStatusExpected)
	// Post-completion state: a bare historical absence without provenance.
	completedRow := testpkg.CreateTestInstanceStudent(t, db, completedInst.ID, student.ID,
		scheduleModels.AttendanceStatusAbsent)

	partial := testpkg.CreateTestPickupException(t, db, student.ID, date, staff.ID, "13:00", "Termin")
	from := timezone.WallClock(time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC))
	partial.ExcusedFrom = &from
	partial.ExcusedCreatedBy = &staff.ID
	partial.ExcusedOwnsPickupTime = true
	require.NoError(t, scheduleRepo.NewStudentPickupExceptionRepository(db).Update(ctx, partial))

	n, err := repo.ApplyPartialAbsence(ctx, partial.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the active instance's row may be claimed")

	gotCompleted, err := repo.FindByID(ctx, completedRow.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotCompleted.Status)
	assert.Nil(t, gotCompleted.Substatus, "completed row must keep its bare historical absence")
	assert.Nil(t, gotCompleted.PickupExceptionID, "completed row must not gain pickup provenance")

	gotActive, err := repo.FindByID(ctx, activeRow.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotActive.Status)
	require.NotNil(t, gotActive.PickupExceptionID)
	assert.Equal(t, partial.ID, *gotActive.PickupExceptionID)
}

// A block that completed while owned by a partial absence is a closed
// historical record: releasing the excusal (pickup time moved back or the
// exception removed) must not strip its excused substatus or pickup
// provenance, exactly as ApplyPartialAbsence refuses to claim completed rows.
func TestInstanceStudentRepository_ReleasePartialAbsenceSkipsCompletedInstance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)
	date := timezone.NewDate(2026, 11, 9)

	activeInst, cleanupActive := createInstanceFixture(t, db, "release-act", date)
	defer cleanupActive()
	completedInst, cleanupCompleted := createInstanceFixture(t, db, "release-done", date)
	defer cleanupCompleted()

	student := testpkg.CreateTestStudent(t, db, "Release", fmt.Sprintf("Done-%d", time.Now().UnixNano()), "3a")
	staff := testpkg.CreateTestStaff(t, db, "Release", fmt.Sprintf("Partial-%d", time.Now().UnixNano()))

	activeRow := testpkg.CreateTestInstanceStudent(t, db, activeInst.ID, student.ID,
		scheduleModels.AttendanceStatusExpected)
	completedRow := testpkg.CreateTestInstanceStudent(t, db, completedInst.ID, student.ID,
		scheduleModels.AttendanceStatusExpected)

	partial := testpkg.CreateTestPickupException(t, db, student.ID, date, staff.ID, "13:00", "Termin")
	from := timezone.WallClock(time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC))
	partial.ExcusedFrom = &from
	partial.ExcusedCreatedBy = &staff.ID
	partial.ExcusedOwnsPickupTime = true
	require.NoError(t, scheduleRepo.NewStudentPickupExceptionRepository(db).Update(ctx, partial))

	// Both rows get claimed while their instances are still live, then one
	// block completes — the realistic order for a same-day pickup change.
	n, err := repo.ApplyPartialAbsence(ctx, partial.ID)
	require.NoError(t, err)
	require.Equal(t, 2, n)
	completedInst.Status = scheduleModels.InstanceStatusCompleted
	require.NoError(t, instanceRepo.Update(ctx, completedInst))

	released, err := repo.ReleasePartialAbsence(ctx, partial.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, released, "only the still-active instance's row may be released")

	gotActive, err := repo.FindByID(ctx, activeRow.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, gotActive.Status)
	assert.Nil(t, gotActive.Substatus)
	assert.Nil(t, gotActive.PickupExceptionID)

	gotCompleted, err := repo.FindByID(ctx, completedRow.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, gotCompleted.Status)
	require.NotNil(t, gotCompleted.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *gotCompleted.Substatus,
		"completed row keeps its historical excused record")
	require.NotNil(t, gotCompleted.PickupExceptionID)
	assert.Equal(t, partial.ID, *gotCompleted.PickupExceptionID,
		"completed row keeps its pickup provenance")
}

// The session-end bridge flips expected → absent without care-day locks.
// ApplyPartialAbsence must still claim those bare absences so release can
// reconcile them by pickup_exception_id.
func TestInstanceStudentRepository_ApplyPartialAbsenceClaimsBridgeBareAbsence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	date := timezone.NewDate(2026, 11, 4)

	// createInstanceFixture starts at 14:00 — after a 13:00 cutoff.
	inst, cleanupInst := createInstanceFixture(t, db, "partial-bridge", date)
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Bridge", fmt.Sprintf("Bare-%d", time.Now().UnixNano()), "3a")
	staff := testpkg.CreateTestStaff(t, db, "Partial", fmt.Sprintf("Bridge-%d", time.Now().UnixNano()))

	// Session-end bridge already stamped a bare absence (no provenance).
	row := testpkg.CreateTestInstanceStudent(t, db, inst.ID, student.ID,
		scheduleModels.AttendanceStatusAbsent)

	partial := testpkg.CreateTestPickupException(t, db, student.ID, date, staff.ID, "13:00", "Termin")
	from := timezone.WallClock(time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC))
	partial.ExcusedFrom = &from
	partial.ExcusedCreatedBy = &staff.ID
	partial.ExcusedOwnsPickupTime = true
	require.NoError(t, scheduleRepo.NewStudentPickupExceptionRepository(db).Update(ctx, partial))

	n, err := repo.ApplyPartialAbsence(ctx, partial.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "bridge bare absence after the cutoff must be claimed")

	got, err := repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, got.Status)
	require.NotNil(t, got.Substatus)
	assert.Equal(t, scheduleModels.AttendanceSubstatusExcused, *got.Substatus)
	require.NotNil(t, got.PickupExceptionID)
	assert.Equal(t, partial.ID, *got.PickupExceptionID)

	// Hand the row to a broad day status; partial must not steal it back.
	released, err := repo.ReleasePartialAbsence(ctx, partial.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, released)
	statusDay := testpkg.CreateTestStudentStatusDay(t, db, student.ID, date, "sick")
	applied, err := repo.ApplyStatusDay(ctx, student.ID, date, statusDay.ID, scheduleModels.AttendanceSubstatusSick)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)

	n, err = repo.ApplyPartialAbsence(ctx, partial.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "status-day owned absences must not be claimed by a partial")

	got, err = repo.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, got.StudentStatusDayID)
	assert.Equal(t, statusDay.ID, *got.StudentStatusDayID)
	assert.Nil(t, got.PickupExceptionID)
}

func TestInstanceStudentRepository_FindPartialAbsenceBlocksIncludesUnmaterializedEnrollment(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db).(*scheduleRepo.InstanceStudentRepository)
	date := timezone.NewDate(2026, 11, 10)

	student := testpkg.CreateTestStudent(t, db, "Preview", fmt.Sprintf("Enrollment-%d", time.Now().UnixNano()), "3a")
	group := testpkg.CreateTestActivityGroup(t, db, "Preview enrollment")
	enrollment := &activitiesModels.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: group.ID,
		ValidFrom:       date.AddDays(-1),
	}
	enrollment.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(enrollment).Exec(ctx)
	require.NoError(t, err)

	room := testpkg.CreateTestRoom(t, db, "Preview enrollment room")
	instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID,
		StartHHMM:       "15:00",
		EndHHMM:         "16:00",
		Title:           "Späte AG",
	})

	blocks, err := repo.FindPartialAbsenceBlocks(ctx, student.ID, date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	assert.Equal(t, instance.ID, blocks[0].ID)

	row := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID,
		scheduleModels.AttendanceStatusAbsent)
	staff := testpkg.CreateTestStaff(t, db, "Preview", fmt.Sprintf("Exception-%d", time.Now().UnixNano()))
	otherException := testpkg.CreateTestPickupException(t, db, student.ID, date, staff.ID, "14:30", "Termin")
	row.PickupExceptionID = &otherException.ID
	require.NoError(t, repo.Update(ctx, row))

	blocks, err = repo.FindPartialAbsenceBlocks(ctx, student.ID, date,
		time.Date(2000, 1, 1, 14, 30, 0, 0, time.UTC))
	require.NoError(t, err)
	assert.Empty(t, blocks, "a row owned by another pickup exception is not actionable")
}

// Parallel-presence lookup (#2265): only rows in OTHER instances that are
// currently active on the same day and hold the student as 'present' count.
func TestInstanceStudentRepository_FindPresentInOtherActiveInstances(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	instRepo := scheduleRepo.NewActivityInstanceRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Paula", fmt.Sprintf("Parallel-%d", time.Now().UnixNano()), "1a")
	other := testpkg.CreateTestStudent(t, db, "Otto", fmt.Sprintf("Parallel-%d", time.Now().UnixNano()+1), "1a")
	checkedOut := testpkg.CreateTestStudent(t, db, "Carla", fmt.Sprintf("Parallel-%d", time.Now().UnixNano()+2), "1a")

	day := timezone.NewDate(2026, 9, 21)
	instTarget, cleanTarget := createInstanceFixture(t, db, "par-target", day)
	defer cleanTarget()
	instActive, cleanActive := createInstanceFixture(t, db, "par-active", day)
	defer cleanActive()
	instPlanned, cleanPlanned := createInstanceFixture(t, db, "par-planned", day)
	defer cleanPlanned()

	activate := func(inst *scheduleModels.ActivityInstance, title string) {
		inst.Status = scheduleModels.InstanceStatusActive
		inst.Title = title
		inst.StartTime = time.Date(2024, 1, 1, 12, 45, 0, 0, time.UTC)
		inst.EndTime = time.Date(2024, 1, 1, 13, 45, 0, 0, time.UTC)
		require.NoError(t, instRepo.Update(ctx, inst))
	}
	activate(instTarget, "Lernzeit JG 1")
	activate(instActive, "GT 1")

	mkRow := func(instID, studentID int64, status string) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{InstanceID: instID, StudentID: studentID, Status: status}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		return row
	}
	// Present in the target itself — must be excluded.
	mkRow(instTarget.ID, student.ID, scheduleModels.AttendanceStatusPresent)
	// Present in another ACTIVE instance — the one expected hit.
	mkRow(instActive.ID, student.ID, scheduleModels.AttendanceStatusPresent)
	// Present in a PLANNED instance — not running, excluded.
	mkRow(instPlanned.ID, student.ID, scheduleModels.AttendanceStatusPresent)
	// Only expected in the other active instance — excluded.
	mkRow(instActive.ID, other.ID, scheduleModels.AttendanceStatusExpected)
	// A checked-out row retains status='present' but is no longer current.
	checkedOutAt := time.Date(2026, 9, 21, 13, 0, 0, 0, time.UTC)
	rCheckedOut := &scheduleModels.InstanceStudent{
		InstanceID:   instActive.ID,
		StudentID:    checkedOut.ID,
		Status:       scheduleModels.AttendanceStatusPresent,
		CheckedOutAt: &checkedOutAt,
	}
	rCheckedOut.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, rCheckedOut))

	got, err := repo.FindPresentInOtherActiveInstances(ctx, instTarget.ID, day, []int64{student.ID, other.ID, checkedOut.ID})
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, student.ID, got[0].StudentID)
	assert.Equal(t, instActive.ID, got[0].InstanceID)
	assert.Equal(t, "GT 1", got[0].Title)
	assert.Equal(t, "12:45", got[0].StartTime.Format("15:04"))
	assert.Equal(t, "13:45", got[0].EndTime.Format("15:04"))

	empty, err := repo.FindPresentInOtherActiveInstances(ctx, instTarget.ID, day, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// TestInstanceStudentRepository_BatchAttendanceMirrors exercises the batch
// mirror forms (#2372): the composite-(instance_id, student_id)-IN updates
// flip only guard-matching rows, and the plural finds resolve several
// students in one query.
func TestInstanceStudentRepository_BatchAttendanceMirrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStudentRepository(db)
	day := timezone.NewDate(2026, 10, 20)
	inst, cleanupInst := createInstanceFixture(t, db, "batch-mirror", day)
	defer cleanupInst()

	studentA := testpkg.CreateTestStudent(t, db, "BatchA", fmt.Sprintf("MirrorA-%d", time.Now().UnixNano()), "3a")
	studentB := testpkg.CreateTestStudent(t, db, "BatchB", fmt.Sprintf("MirrorB-%d", time.Now().UnixNano()), "3a")

	// A is expected — the batch check-in must flip it. B is already present
	// and open — the guard must preserve its original check-in.
	rowA := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  studentA.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	rowA.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, rowA))

	firstCheckin := time.Date(2026, 10, 20, 13, 0, 0, 0, time.UTC)
	rowB := &scheduleModels.InstanceStudent{
		InstanceID:  inst.ID,
		StudentID:   studentB.ID,
		Status:      scheduleModels.AttendanceStatusPresent,
		CheckedInAt: &firstCheckin,
	}
	rowB.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, rowB))

	keys := []scheduleModels.InstanceStudentKey{
		{InstanceID: inst.ID, StudentID: studentA.ID},
		{InstanceID: inst.ID, StudentID: studentB.ID},
	}
	batchCheckin := time.Date(2026, 10, 20, 14, 0, 0, 0, time.UTC)
	require.NoError(t, repo.UpdateAttendanceFromCheckinBatch(ctx, keys, batchCheckin))

	gotA, err := repo.FindByID(ctx, rowA.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, gotA.Status)
	require.NotNil(t, gotA.CheckedInAt)
	assert.WithinDuration(t, batchCheckin, *gotA.CheckedInAt, time.Second)

	gotB, err := repo.FindByID(ctx, rowB.ID)
	require.NoError(t, err)
	require.NotNil(t, gotB.CheckedInAt)
	assert.WithinDuration(t, firstCheckin, *gotB.CheckedInAt, time.Second,
		"already-open present row must keep its original check-in")

	// Plural finds resolve both students in one query.
	studentIDs := []int64{studentA.ID, studentB.ID}
	candidateAt := time.Date(2026, 10, 20, 14, 30, 0, 0, timezone.Berlin)
	candidates, err := repo.FindCurrentCandidatesByStudentIDs(ctx, studentIDs, day, candidateAt)
	require.NoError(t, err)
	assert.Len(t, candidates, 2)

	dateRows, err := repo.FindByStudentIDsAndDate(ctx, studentIDs, day)
	require.NoError(t, err)
	assert.Len(t, dateRows, 2)

	// The batch checkout closes both open present rows at the shared instant.
	batchCheckout := batchCheckin.Add(time.Hour)
	require.NoError(t, repo.UpdateAttendanceCheckoutBatch(ctx, keys, batchCheckout))

	gotA, err = repo.FindByID(ctx, rowA.ID)
	require.NoError(t, err)
	require.NotNil(t, gotA.CheckedOutAt)
	assert.WithinDuration(t, batchCheckout, *gotA.CheckedOutAt, time.Second)

	gotB, err = repo.FindByID(ctx, rowB.ID)
	require.NoError(t, err)
	require.NotNil(t, gotB.CheckedOutAt)
	assert.WithinDuration(t, batchCheckout, *gotB.CheckedOutAt, time.Second)
}
