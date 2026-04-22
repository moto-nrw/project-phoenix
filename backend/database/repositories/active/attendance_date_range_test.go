package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createAttendanceForDate inserts an attendance record for a specific calendar
// date (Berlin TZ). Uses DateOfUTC so the stored DATE matches what the repo
// query expects.
func createAttendanceForDate(t *testing.T, ctx context.Context, repo active.AttendanceRepository, studentID, staffID, deviceID int64, date time.Time) *active.Attendance {
	t.Helper()
	dateOnly := timezone.DateOfUTC(date)
	checkIn := date.Add(8 * time.Hour)
	checkOut := date.Add(15 * time.Hour)
	a := &active.Attendance{
		StudentID:    studentID,
		Date:         dateOnly,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  staffID,
		DeviceID:     &deviceID,
	}
	a.SetTenantID(1)
	err := repo.Create(ctx, a)
	require.NoError(t, err)
	return a
}

func TestAttendanceRepository_FindByStudentAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.TenantContext(1)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	berlin, _ := time.LoadLocation("Europe/Berlin")
	today := timezone.Today()
	yesterday := today.AddDate(0, 0, -1)
	twoDaysAgo := today.AddDate(0, 0, -2)
	threeDaysAgo := today.AddDate(0, 0, -3)

	// Create records for 4 days
	a1 := createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, threeDaysAgo)
	a2 := createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, twoDaysAgo)
	a3 := createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, yesterday)
	a4 := createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, today)
	defer func() {
		for _, id := range []int64{a1.ID, a2.ID, a3.ID, a4.ID} {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	// Also create one for Student2 to verify student isolation
	a5 := createAttendanceForDate(t, ctx, repo, data.Student2.ID, data.Staff1.ID, data.Device1.ID, today)
	defer testpkg.CleanupTableRecords(t, db, "active.attendance", a5.ID)

	t.Run("returns_all_records_in_range", func(t *testing.T) {
		start := threeDaysAgo
		end := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, berlin)

		results, err := repo.FindByStudentAndDateRange(ctx, data.Student1.ID, start, end)
		require.NoError(t, err)
		assert.Len(t, results, 4)
	})

	t.Run("includes_today_correctly", func(t *testing.T) {
		// This specifically tests the DateOfUTC fix — today's record must be
		// included when end is today's end-of-day.
		end := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, berlin)
		results, err := repo.FindByStudentAndDateRange(ctx, data.Student1.ID, today, end)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, data.Student1.ID, results[0].StudentID)
	})

	t.Run("narrows_range_correctly", func(t *testing.T) {
		// Only yesterday and twoDaysAgo
		end := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, berlin)
		results, err := repo.FindByStudentAndDateRange(ctx, data.Student1.ID, twoDaysAgo, end)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns_empty_for_no_matches", func(t *testing.T) {
		futureStart := today.AddDate(0, 0, 10)
		futureEnd := today.AddDate(0, 0, 15)
		results, err := repo.FindByStudentAndDateRange(ctx, data.Student1.ID, futureStart, futureEnd)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("isolates_by_student_id", func(t *testing.T) {
		end := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, berlin)
		results, err := repo.FindByStudentAndDateRange(ctx, data.Student2.ID, threeDaysAgo, end)
		require.NoError(t, err)
		assert.Len(t, results, 1, "should only return Student2's record")
	})

	t.Run("ordered_by_date_desc", func(t *testing.T) {
		end := time.Date(today.Year(), today.Month(), today.Day(), 23, 59, 59, 0, berlin)
		results, err := repo.FindByStudentAndDateRange(ctx, data.Student1.ID, threeDaysAgo, end)
		require.NoError(t, err)
		require.Len(t, results, 4)
		// First result should be the most recent date
		assert.True(t, !results[0].Date.Before(results[1].Date), "results should be ordered date DESC")
	})
}
