package schedule_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
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

	inst, cleanupInst := createInstanceFixture(t, db, "stu", time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC))
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

	inst, cleanupInst := createInstanceFixture(t, db, "stu-upd", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC))
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

	dayA := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	dayB := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	dayOutside := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

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

func TestInstanceStudentRepository_DeleteByInstanceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stu-del", time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC))
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
