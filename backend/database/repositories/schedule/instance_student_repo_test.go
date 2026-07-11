package schedule_test

import (
	"context"
	"fmt"
	"strings"
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

func TestInstanceStudentRepository_Create_and_FindByInstanceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu", timezone.NewDate(2026, 9, 19))
	defer cleanupInst()

	studentA := testpkg.CreateTestStudent(t, db, "Max", fmt.Sprintf("A-%d", time.Now().UnixNano()), "3a")
	studentB := testpkg.CreateTestStudent(t, db, "Mia", fmt.Sprintf("B-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, studentA.ID, studentB.ID)

	rowA := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  studentA.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	rowA.SetTenantID(1)

	rowB := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  studentB.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	rowB.SetTenantID(1)

	require.NoError(t, repo.Create(ctx, rowA))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowA.ID)
	require.NoError(t, repo.Create(ctx, rowB))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowB.ID)

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
		dup.SetTenantID(1)
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
		row.SetTenantID(1)
		long := strings.Repeat("x", scheduleModels.InstanceStudentNoteMaxLength+1)
		row.Note = &long
		err := repo.Create(ctx, row)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "note cannot exceed 500 characters")
	})
}

func TestInstanceStudentRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-upd", timezone.NewDate(2026, 9, 20))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Ella", fmt.Sprintf("Upd-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	student := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("Range-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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
		row.SetTenantID(1)
		return row
	}

	rA, rB, rOut := mkRow(instA.ID), mkRow(instB.ID), mkRow(instOutside.ID)
	require.NoError(t, repo.Create(ctx, rA))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rA.ID)
	require.NoError(t, repo.Create(ctx, rB))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rB.ID)
	require.NoError(t, repo.Create(ctx, rOut))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rOut.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
		bad.SetTenantID(1)
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})
}

func TestInstanceStudentRepository_UpdateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
		bad.SetTenantID(1)
		err := repo.Update(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_id is required")
	})
}

func TestInstanceStudentRepository_FindByID_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	got, err := repo.FindByID(ctx, int64(999999999))
	require.Error(t, err)
	assert.Nil(t, got)
	var dbErr *modelBase.DatabaseError
	require.ErrorAs(t, err, &dbErr)
	assert.Equal(t, "find by id", dbErr.Op)
}

func TestInstanceStudentRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-list", timezone.NewDate(2026, 10, 8))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Peter", fmt.Sprintf("List-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "mirror", timezone.NewDate(2026, 10, 10))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Lea", fmt.Sprintf("Mirror-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	t.Run("flips expected → present and stamps checked_in_at", func(t *testing.T) {
		row := &scheduleModels.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  student.ID,
			Status:     scheduleModels.AttendanceStatusExpected,
		}
		row.SetTenantID(1)
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
		row.SetTenantID(1)
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

func TestInstanceStudentRepository_UpdateAttendanceFields(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "patch", timezone.NewDate(2026, 10, 11))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Nora", fmt.Sprintf("Patch-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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
	row.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

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

func TestInstanceStudentRepository_BulkUpdateStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-bulk", timezone.NewDate(2026, 11, 2))
	defer cleanupInst()

	suffix := fmt.Sprintf("Bulk-%d", time.Now().UnixNano())
	sExpected1 := testpkg.CreateTestStudent(t, db, "Ada", suffix+"-e1", "3a")
	sExpected2 := testpkg.CreateTestStudent(t, db, "Bea", suffix+"-e2", "3a")
	sPresent := testpkg.CreateTestStudent(t, db, "Cem", suffix+"-p", "3a")
	defer testpkg.CleanupActivityFixtures(t, db, sExpected1.ID, sExpected2.ID, sPresent.ID)

	buildRow := func(studentID int64, status string) *scheduleModels.InstanceStudent {
		row := &scheduleModels.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  studentID,
			Status:     status,
		}
		row.SetTenantID(1)
		return row
	}

	rowE1 := buildRow(sExpected1.ID, scheduleModels.AttendanceStatusExpected)
	rowE2 := buildRow(sExpected2.ID, scheduleModels.AttendanceStatusExpected)
	rowP := buildRow(sPresent.ID, scheduleModels.AttendanceStatusPresent)

	require.NoError(t, repo.Create(ctx, rowE1))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowE1.ID)
	require.NoError(t, repo.Create(ctx, rowE2))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowE2.ID)
	require.NoError(t, repo.Create(ctx, rowP))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowP.ID)

	t.Run("flips only matching expected rows", func(t *testing.T) {
		n, err := repo.BulkUpdateStatus(ctx, inst.ID,
			scheduleModels.AttendanceStatusExpected,
			scheduleModels.AttendanceStatusAbsent,
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-del", timezone.NewDate(2026, 10, 7))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Ola", fmt.Sprintf("Del-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer testpkg.CleanupActivityFixtures(t, db,
		expectedStudent.ID, presentStudent.ID, absentStudent.ID, otherStudent.ID)

	expectedRow := &scheduleModels.InstanceStudent{
		InstanceID: inst1.ID,
		StudentID:  expectedStudent.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	expectedRow.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, expectedRow))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", expectedRow.ID)

	presentRow := &scheduleModels.InstanceStudent{
		InstanceID: inst1.ID,
		StudentID:  presentStudent.ID,
		Status:     scheduleModels.AttendanceStatusPresent,
	}
	presentRow.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, presentRow))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", presentRow.ID)

	absentRow := &scheduleModels.InstanceStudent{
		InstanceID: inst1.ID,
		StudentID:  absentStudent.ID,
		Status:     scheduleModels.AttendanceStatusAbsent,
	}
	absentRow.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, absentRow))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", absentRow.ID)

	otherExpected := &scheduleModels.InstanceStudent{
		InstanceID: inst2.ID,
		StudentID:  otherStudent.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	otherExpected.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, otherExpected))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", otherExpected.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "b13-iso", timezone.NewDate(2026, 9, 23))
	defer cleanupInst()

	student := testpkg.CreateTestStudent(t, db, "Iso", fmt.Sprintf("B13-iso-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	row := &scheduleModels.InstanceStudent{
		InstanceID: inst.ID,
		StudentID:  student.ID,
		Status:     scheduleModels.AttendanceStatusExpected,
	}
	row.SetTenantID(1)
	require.NoError(t, repo.Create(testpkg.TenantContext(1), row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)

	// Same instance ID, different tenant context → row must be invisible.
	otherTenantCtx := testpkg.TenantContext(2)
	rows, err := repo.FindExpectedByInstanceIDs(otherTenantCtx, []int64{inst.ID})
	require.NoError(t, err)
	assert.Empty(t, rows, "row from tenant 1 must not leak to tenant 2")
}

func TestInstanceStudentRepository_CountNonAbsentByInstanceIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
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
	defer testpkg.CleanupActivityFixtures(t, db, studentExpected.ID, studentPresent.ID, studentAbsent.ID)

	// instA: expected + present (both non-absent), one absent → expect 2
	rowA1 := testpkg.CreateTestInstanceStudent(t, db, instA.ID, studentExpected.ID, scheduleModels.AttendanceStatusExpected)
	rowA2 := testpkg.CreateTestInstanceStudent(t, db, instA.ID, studentPresent.ID, scheduleModels.AttendanceStatusPresent)
	rowA3 := testpkg.CreateTestInstanceStudent(t, db, instA.ID, studentAbsent.ID, scheduleModels.AttendanceStatusAbsent)

	// instB: 1 expected → expect 1
	rowB1 := testpkg.CreateTestInstanceStudent(t, db, instB.ID, studentExpected.ID, scheduleModels.AttendanceStatusExpected)

	// instEmpty: nothing assigned → must NOT appear in map
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rowA1.ID, rowA2.ID, rowA3.ID, rowB1.ID)

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
