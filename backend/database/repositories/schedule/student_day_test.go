package schedule_test

import (
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FindInstancesWithAttendanceByStudentAndDateRange must return one row per
// (instance, attendance) pair in the range, tenant-scoped, sorted.
func TestInstanceStudentRepository_FindInstancesWithAttendanceByStudentAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStudentRepository(db).(*scheduleRepo.InstanceStudentRepository)

	student := testpkg.CreateTestStudent(t, db, "Noah", fmt.Sprintf("SD-%d", time.Now().UnixNano()), "3a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	dayA := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	dayB := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	dayOutside := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

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
			time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC),
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
