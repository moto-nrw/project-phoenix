package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createAttendanceForDate inserts an attendance record for a specific calendar
// date (Berlin TZ).
func createAttendanceForDate(t *testing.T, ctx context.Context, repo studentpresence.AttendanceHistoryCommand, studentID, staffID, deviceID int64, date timezone.Date) {
	t.Helper()
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	checkOut := date.BerlinMidnight().Add(15 * time.Hour)
	a := studentpresence.Attendance{
		TenantID:     testpkg.Tenant(t),
		StudentID:    studentID,
		Date:         date.String(),
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CheckedInBy:  staffID,
		DeviceID:     deviceID,
	}
	_, err := repo.RecordAttendance(ctx, a)
	require.NoError(t, err)
}

func TestPresenceAttendance_DateRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	today := timezone.NewDate(2026, 8, 24)
	yesterday := today.AddDays(-1)
	twoDaysAgo := today.AddDays(-2)
	threeDaysAgo := today.AddDays(-3)

	// Create records for 4 days
	createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, threeDaysAgo)
	createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, twoDaysAgo)
	createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, yesterday)
	createAttendanceForDate(t, ctx, repo, data.Student1.ID, data.Staff1.ID, data.Device1.ID, today)

	// Also create one for Student2 to verify student isolation
	createAttendanceForDate(t, ctx, repo, data.Student2.ID, data.Staff1.ID, data.Device1.ID, today)

	t.Run("returns_all_records_in_range", func(t *testing.T) {
		results, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: threeDaysAgo.String(), UntilDate: today.String(), NewestFirst: true})
		require.NoError(t, err)
		assert.Len(t, results, 4)
	})

	t.Run("includes_today_correctly", func(t *testing.T) {
		// Today's record must be included when the range ends today.
		results, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: today.String(), UntilDate: today.String(), NewestFirst: true})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, data.Student1.ID, results[0].StudentID)
	})

	t.Run("narrows_range_correctly", func(t *testing.T) {
		// Only yesterday and twoDaysAgo
		results, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: twoDaysAgo.String(), UntilDate: yesterday.String(), NewestFirst: true})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns_empty_for_no_matches", func(t *testing.T) {
		futureStart := today.AddDays(10)
		futureEnd := today.AddDays(15)
		results, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: futureStart.String(), UntilDate: futureEnd.String(), NewestFirst: true})
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("isolates_by_student_id", func(t *testing.T) {
		results, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student2.ID}, FromDate: threeDaysAgo.String(), UntilDate: today.String(), NewestFirst: true})
		require.NoError(t, err)
		assert.Len(t, results, 1, "should only return Student2's record")
	})

	t.Run("ordered_by_date_desc", func(t *testing.T) {
		results, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: threeDaysAgo.String(), UntilDate: today.String(), NewestFirst: true})
		require.NoError(t, err)
		require.Len(t, results, 4)
		// First result should be the most recent date
		assert.Equal(t, []string{today.String(), yesterday.String(), twoDaysAgo.String(), threeDaysAgo.String()},
			[]string{results[0].Date, results[1].Date, results[2].Date, results[3].Date}, "results should be ordered date DESC")
	})
}
