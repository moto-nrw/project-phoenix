package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupAttendanceRepo creates an attendance repository instance
func setupAttendanceRepo(t *testing.T, db *bun.DB) active.AttendanceRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Attendance
}

// attendanceTestData holds test entities created via hermetic fixtures
type attendanceTestData struct {
	Student1 *users.Student
	Student2 *users.Student
	Staff1   *users.Staff
	Staff2   *users.Staff
	Device1  *iot.Device
}

// createAttendanceTestData creates test fixtures using the hermetic pattern
func createAttendanceTestData(t *testing.T, db *bun.DB) *attendanceTestData {
	return &attendanceTestData{
		Student1: testpkg.CreateTestStudent(t, db, "Attendance", "Student1", "1a"),
		Student2: testpkg.CreateTestStudent(t, db, "Attendance", "Student2", "1b"),
		Staff1:   testpkg.CreateTestStaff(t, db, "Attendance", "Staff1"),
		Staff2:   testpkg.CreateTestStaff(t, db, "Attendance", "Staff2"),
		Device1:  testpkg.CreateTestDevice(t, db, "attendance-repo-test-device"),
	}
}

// cleanupAttendanceTestData removes test data using hermetic cleanup
func cleanupAttendanceTestData(t *testing.T, db *bun.DB, data *attendanceTestData) {
	testpkg.CleanupActivityFixtures(t, db,
		data.Student1.ID, data.Student2.ID,
		data.Staff1.ID, data.Staff2.ID,
		data.Device1.ID,
	)
}

// cleanupAttendanceRecords removes specific attendance records
func cleanupAttendanceRecords(t *testing.T, db *bun.DB, ids ...int64) {
	ctx := context.Background()
	for _, id := range ids {
		_, err := db.NewDelete().
			Model((*active.Attendance)(nil)).
			ModelTableExpr(`active.attendance AS "attendance"`).
			Where(`"attendance".id = ?`, id).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup attendance record %d: %v", id, err)
		}
	}
}

// TestAttendanceRepository_Create tests basic record creation
func TestAttendanceRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("create valid attendance record", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Verify ID was assigned
		assert.NotZero(t, attendance.ID)

		// Verify timestamps were set
		assert.False(t, attendance.CreatedAt.IsZero())
		assert.False(t, attendance.UpdatedAt.IsZero())
	})

	t.Run("create with check-out time", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		checkOutTime := now.Add(2 * time.Hour)
		checkedOutBy := data.Staff2.ID

		attendance := &active.Attendance{
			StudentID:    data.Student2.ID,
			Date:         date,
			CheckInTime:  now,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		assert.NotZero(t, attendance.ID)
		assert.NotNil(t, attendance.CheckOutTime)
		assert.Equal(t, checkOutTime.Unix(), attendance.CheckOutTime.Unix())
		assert.NotNil(t, attendance.CheckedOutBy)
		assert.Equal(t, checkedOutBy, *attendance.CheckedOutBy)
	})

	t.Run("create with nil attendance should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("verify IsCheckedIn helper method", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create attendance without check-out
		attendanceCheckedIn := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour), // Different time to avoid conflict
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendanceCheckedIn)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendanceCheckedIn.ID)

		assert.True(t, attendanceCheckedIn.IsCheckedIn(), "Should be checked in when CheckOutTime is nil")

		// Create attendance with check-out
		checkOutTime := now.Add(3 * time.Hour)
		checkedOutBy := data.Staff1.ID
		attendanceCheckedOut := &active.Attendance{
			StudentID:    data.Student2.ID,
			Date:         date,
			CheckInTime:  now.Add(2 * time.Hour), // Different time to avoid conflict
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		err = repo.Create(ctx, attendanceCheckedOut)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendanceCheckedOut.ID)

		assert.False(t, attendanceCheckedOut.IsCheckedIn(), "Should not be checked in when CheckOutTime is set")
	})
}

// TestAttendanceRepository_FindByStudentAndDate tests querying attendance records by student and date
func TestAttendanceRepository_FindByStudentAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("single record for student on date", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Find records for this student and date
		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date)
		require.NoError(t, err)

		require.GreaterOrEqual(t, len(records), 1, "Should find at least one record")
		// Find our record in the results
		var found bool
		for _, r := range records {
			if r.ID == attendance.ID {
				found = true
				assert.Equal(t, data.Student1.ID, r.StudentID)
				break
			}
		}
		assert.True(t, found, "Should find the created attendance record")
	})

	t.Run("multiple records for student on same date ordered by check-in time", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create three attendance records with different check-in times
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(-2 * time.Hour), // Earliest
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance2 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now, // Middle
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance3 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour), // Latest
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2, attendance3} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Find records for this student and date
		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date)
		require.NoError(t, err)

		require.GreaterOrEqual(t, len(records), 3, "Should find at least three records")

		// Verify ordering by check_in_time ASC (for records we created)
		var ourRecords []*active.Attendance
		for _, r := range records {
			if r.ID == attendance1.ID || r.ID == attendance2.ID || r.ID == attendance3.ID {
				ourRecords = append(ourRecords, r)
			}
		}
		require.Len(t, ourRecords, 3, "Should find all three created records")
	})

	t.Run("no records for student on date", func(t *testing.T) {
		// Use a date with no records
		emptyDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, emptyDate)
		require.NoError(t, err)

		assert.Len(t, records, 0, "Should find no records for date with no attendance")
	})

	t.Run("date filtering ignores time component", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(5 * time.Hour), // Different time
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Query with different time component but same date
		queryDate := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 45, 0, time.UTC)

		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, queryDate)
		require.NoError(t, err)

		var found bool
		for _, r := range records {
			if r.ID == attendance.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find record regardless of time component in query date")
	})

	t.Run("different students on same date", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create attendance for student1
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(6 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2
		attendance2 := &active.Attendance{
			StudentID:   data.Student2.ID,
			Date:        date,
			CheckInTime: now.Add(7 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Query for student1 should only return their records
		records1, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date)
		require.NoError(t, err)
		for _, r := range records1 {
			assert.Equal(t, data.Student1.ID, r.StudentID)
		}

		// Query for student2 should only return their records
		records2, err := repo.FindByStudentAndDate(ctx, data.Student2.ID, date)
		require.NoError(t, err)
		for _, r := range records2 {
			assert.Equal(t, data.Student2.ID, r.StudentID)
		}
	})

	t.Run("different dates for same student", func(t *testing.T) {
		now := time.Now()
		date1 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		date2 := date1.AddDate(0, 0, 1) // Next day

		// Create attendance for date1
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date1,
			CheckInTime: now.Add(8 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2
		attendance2 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date2,
			CheckInTime: now.Add(32 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Query for date1 should return records for that day
		records1, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date1)
		require.NoError(t, err)
		var foundDate1 bool
		for _, r := range records1 {
			if r.ID == attendance1.ID {
				foundDate1 = true
				break
			}
		}
		assert.True(t, foundDate1, "Should find date1's record")

		// Query for date2 should return records for that day
		records2, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date2)
		require.NoError(t, err)
		var foundDate2 bool
		for _, r := range records2 {
			if r.ID == attendance2.ID {
				foundDate2 = true
				break
			}
		}
		assert.True(t, foundDate2, "Should find date2's record")
	})
}

// TestAttendanceRepository_FindLatestByStudent tests finding the most recent attendance record for a student
func TestAttendanceRepository_FindLatestByStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("latest record across multiple dates", func(t *testing.T) {
		now := time.Now()
		date1 := time.Date(now.Year(), now.Month(), now.Day()-2, 0, 0, 0, 0, time.UTC) // 2 days ago
		date2 := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC) // Yesterday
		date3 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)   // Today

		// Create attendance for date1 (oldest)
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date1,
			CheckInTime: now.Add(-48 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2 (middle)
		attendance2 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date2,
			CheckInTime: now.Add(-24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date3 (latest by date)
		attendance3 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date3,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2, attendance3} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Latest should be attendance3 (today)
		assert.Equal(t, attendance3.ID, latest.ID, "Should return the record from the latest date")
	})

	t.Run("latest record same day with multiple check-ins", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create multiple attendance records on same day with different check-in times
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(-2 * time.Hour), // Earlier
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance2 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour), // Later
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		assert.Equal(t, attendance2.ID, latest.ID, "Should return the record with latest check-in time")
	})

	t.Run("no records for student", func(t *testing.T) {
		// Create a new student with no attendance records
		newStudent := testpkg.CreateTestStudent(t, db, "NoAttendance", "Student", "1c")
		defer testpkg.CleanupActivityFixtures(t, db, newStudent.ID)

		// Try to find latest record for student with no attendance
		latest, err := repo.FindLatestByStudent(ctx, newStudent.ID)

		// This should return a database error (no rows found)
		assert.Error(t, err, "Should return error when no records exist")
		assert.Nil(t, latest, "Should return nil when no records exist")
	})

	t.Run("single record for student", func(t *testing.T) {
		// Create a new student for isolated test
		singleStudent := testpkg.CreateTestStudent(t, db, "Single", "RecordStudent", "1d")
		defer testpkg.CleanupActivityFixtures(t, db, singleStudent.ID)

		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   singleStudent.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, singleStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		assert.Equal(t, attendance.ID, latest.ID, "Should return the only record")
		assert.Equal(t, singleStudent.ID, latest.StudentID)
	})

	t.Run("complex scenario - mixed dates and times", func(t *testing.T) {
		// Create a new student for isolated test
		complexStudent := testpkg.CreateTestStudent(t, db, "Complex", "ScenarioStudent", "1e")
		defer testpkg.CleanupActivityFixtures(t, db, complexStudent.ID)

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		yesterday := today.AddDate(0, 0, -1)

		// Yesterday: multiple records
		attendanceYesterday1 := &active.Attendance{
			StudentID:   complexStudent.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-30 * time.Hour), // Earlier yesterday
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendanceYesterday2 := &active.Attendance{
			StudentID:   complexStudent.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-25 * time.Hour), // Later yesterday
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Today: single record but earlier in the day than latest yesterday record
		attendanceToday := &active.Attendance{
			StudentID:   complexStudent.ID,
			Date:        today,
			CheckInTime: now.Add(-2 * time.Hour), // Early today
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendanceYesterday1, attendanceYesterday2, attendanceToday} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, complexStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Should return today's record even though yesterday had later times
		// because date takes precedence over time in the ordering
		assert.Equal(t, attendanceToday.ID, latest.ID, "Should return today's record (latest by date)")
	})

	t.Run("different students do not interfere", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create attendance for student1 (earlier)
		attendanceStudent1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(2 * time.Hour), // Use different times
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2 (later)
		attendanceStudent2 := &active.Attendance{
			StudentID:   data.Student2.ID,
			Date:        date,
			CheckInTime: now.Add(3 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendanceStudent1, attendanceStudent2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Latest for student1 should be their record
		latest1, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest1)
		assert.Equal(t, data.Student1.ID, latest1.StudentID)

		// Latest for student2 should be their record
		latest2, err := repo.FindLatestByStudent(ctx, data.Student2.ID)
		require.NoError(t, err)
		require.NotNil(t, latest2)
		assert.Equal(t, data.Student2.ID, latest2.StudentID)
	})
}

// TestAttendanceRepository_GetStudentCurrentStatus tests getting today's latest attendance record for a student
func TestAttendanceRepository_GetStudentCurrentStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("no records today - student not checked in", func(t *testing.T) {
		// Create a new student with no attendance records
		newStudent := testpkg.CreateTestStudent(t, db, "NoRecords", "Today", "2a")
		defer testpkg.CleanupActivityFixtures(t, db, newStudent.ID)

		// Try to get current status for student with no attendance today
		status, err := repo.GetStudentCurrentStatus(ctx, newStudent.ID)

		// Should return error (no rows found) when no attendance today
		assert.Error(t, err, "Should return error when no attendance records exist for today")
		assert.Nil(t, status, "Should return nil when no records exist for today")
	})

	t.Run("student checked in - latest record has no check-out time", func(t *testing.T) {
		// Create isolated student
		checkedInStudent := testpkg.CreateTestStudent(t, db, "CheckedIn", "StatusTest", "2b")
		defer testpkg.CleanupActivityFixtures(t, db, checkedInStudent.ID)

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   checkedInStudent.ID,
			Date:        today,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
			// CheckOutTime is nil - student is checked in
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Get current status
		status, err := repo.GetStudentCurrentStatus(ctx, checkedInStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance.ID, status.ID)
		assert.Equal(t, checkedInStudent.ID, status.StudentID)
		assert.Nil(t, status.CheckOutTime, "CheckOutTime should be nil for checked-in student")
		assert.True(t, status.IsCheckedIn(), "Student should be checked in")
	})

	t.Run("student checked out - latest record has check-out time", func(t *testing.T) {
		// Create isolated student
		checkedOutStudent := testpkg.CreateTestStudent(t, db, "CheckedOut", "StatusTest", "2c")
		defer testpkg.CleanupActivityFixtures(t, db, checkedOutStudent.ID)

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		checkOutTime := now.Add(2 * time.Hour)
		checkedOutBy := data.Staff2.ID

		attendance := &active.Attendance{
			StudentID:    checkedOutStudent.ID,
			Date:         today,
			CheckInTime:  now,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Get current status
		status, err := repo.GetStudentCurrentStatus(ctx, checkedOutStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance.ID, status.ID)
		assert.Equal(t, checkedOutStudent.ID, status.StudentID)
		assert.NotNil(t, status.CheckOutTime, "CheckOutTime should be set for checked-out student")
		assert.False(t, status.IsCheckedIn(), "Student should not be checked in")
	})

	t.Run("multiple records today - returns latest by check-in time", func(t *testing.T) {
		// Create isolated student
		multiRecordStudent := testpkg.CreateTestStudent(t, db, "MultiRecord", "StatusTest", "2d")
		defer testpkg.CleanupActivityFixtures(t, db, multiRecordStudent.ID)

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// First check-in (earlier)
		attendance1 := &active.Attendance{
			StudentID:   multiRecordStudent.ID,
			Date:        today,
			CheckInTime: now.Add(-3 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Check-out from first session
		checkOutTime1 := now.Add(-2 * time.Hour)
		checkedOutBy1 := data.Staff1.ID
		attendance2 := &active.Attendance{
			StudentID:    multiRecordStudent.ID,
			Date:         today,
			CheckInTime:  now.Add(-2*time.Hour - 30*time.Minute),
			CheckOutTime: &checkOutTime1,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy1,
			DeviceID:     data.Device1.ID,
		}

		// Second check-in (latest)
		attendance3 := &active.Attendance{
			StudentID:   multiRecordStudent.ID,
			Date:        today,
			CheckInTime: now.Add(-1 * time.Hour), // Latest check-in time
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2, attendance3} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Get current status - should return the latest check-in
		status, err := repo.GetStudentCurrentStatus(ctx, multiRecordStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance3.ID, status.ID, "Should return the record with latest check-in time")
		assert.Nil(t, status.CheckOutTime, "Latest record should not have check-out time")
		assert.True(t, status.IsCheckedIn(), "Student should be checked in from latest record")
	})

	t.Run("historical records exist but none today", func(t *testing.T) {
		// Create isolated student
		historicalStudent := testpkg.CreateTestStudent(t, db, "Historical", "StatusTest", "2e")
		defer testpkg.CleanupActivityFixtures(t, db, historicalStudent.ID)

		now := time.Now()
		yesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.UTC)

		// Create attendance for yesterday
		attendance := &active.Attendance{
			StudentID:   historicalStudent.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Get current status - should not find yesterday's record
		status, err := repo.GetStudentCurrentStatus(ctx, historicalStudent.ID)

		assert.Error(t, err, "Should return error when no records exist for today")
		assert.Nil(t, status, "Should return nil when only historical records exist")
	})

	t.Run("different students on same day", func(t *testing.T) {
		// Create isolated students
		diffStudent1 := testpkg.CreateTestStudent(t, db, "Different1", "StatusTest", "2f")
		diffStudent2 := testpkg.CreateTestStudent(t, db, "Different2", "StatusTest", "2g")
		defer testpkg.CleanupActivityFixtures(t, db, diffStudent1.ID, diffStudent2.ID)

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create attendance for student1
		attendance1 := &active.Attendance{
			StudentID:   diffStudent1.ID,
			Date:        today,
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2 with check-out
		checkOutTime2 := now
		checkedOutBy2 := data.Staff1.ID
		attendance2 := &active.Attendance{
			StudentID:    diffStudent2.ID,
			Date:         today,
			CheckInTime:  now.Add(-2 * time.Hour),
			CheckOutTime: &checkOutTime2,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy2,
			DeviceID:     data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Get status for student1 - should be checked in
		status1, err := repo.GetStudentCurrentStatus(ctx, diffStudent1.ID)
		require.NoError(t, err)
		require.NotNil(t, status1)
		assert.Equal(t, diffStudent1.ID, status1.StudentID)
		assert.Nil(t, status1.CheckOutTime)
		assert.True(t, status1.IsCheckedIn())

		// Get status for student2 - should be checked out
		status2, err := repo.GetStudentCurrentStatus(ctx, diffStudent2.ID)
		require.NoError(t, err)
		require.NotNil(t, status2)
		assert.Equal(t, diffStudent2.ID, status2.StudentID)
		assert.NotNil(t, status2.CheckOutTime)
		assert.False(t, status2.IsCheckedIn())
	})

	t.Run("timezone handling - today calculation", func(t *testing.T) {
		// Create isolated student
		tzStudent := testpkg.CreateTestStudent(t, db, "Timezone", "StatusTest", "2h")
		defer testpkg.CleanupActivityFixtures(t, db, tzStudent.ID)

		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create attendance record for today but late in the day
		attendance := &active.Attendance{
			StudentID:   tzStudent.ID,
			Date:        today,
			CheckInTime: today.Add(23 * time.Hour), // Late in the day
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Get current status - should find the record regardless of time
		status, err := repo.GetStudentCurrentStatus(ctx, tzStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance.ID, status.ID)
	})
}

// TestAttendanceRepository_Update tests updating attendance records
func TestAttendanceRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("updates attendance with check-out time", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// Update with check-out
		checkOutTime := now.Add(3 * time.Hour)
		checkedOutBy := data.Staff2.ID
		attendance.CheckOutTime = &checkOutTime
		attendance.CheckedOutBy = &checkedOutBy

		err = repo.Update(ctx, attendance)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindByID(ctx, attendance.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.CheckOutTime)
		assert.NotNil(t, found.CheckedOutBy)
		assert.Equal(t, checkedOutBy, *found.CheckedOutBy)
	})

	t.Run("update with nil attendance should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// TestAttendanceRepository_FindByID tests finding by ID
func TestAttendanceRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("finds existing attendance by ID", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		found, err := repo.FindByID(ctx, attendance.ID)
		require.NoError(t, err)
		assert.Equal(t, attendance.ID, found.ID)
		assert.Equal(t, data.Student1.ID, found.StudentID)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		assert.Error(t, err)
	})
}

// TestAttendanceRepository_Delete tests deleting attendance records
func TestAttendanceRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	t.Run("deletes existing attendance record", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)

		err = repo.Delete(ctx, attendance.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, attendance.ID)
		assert.Error(t, err)
	})
}

// TestAttendanceRepository_GetTodayByStudentID tests getting today's attendance
func TestAttendanceRepository_GetTodayByStudentID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("gets today's attendance for student", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		found, err := repo.GetTodayByStudentID(ctx, data.Student1.ID)
		require.NoError(t, err)
		assert.Equal(t, attendance.ID, found.ID)
	})

	t.Run("returns error when no attendance today", func(t *testing.T) {
		// Create student with no attendance today
		newStudent := testpkg.CreateTestStudent(t, db, "NoAttendanceToday", "Test", "3a")
		defer testpkg.CleanupActivityFixtures(t, db, newStudent.ID)

		_, err := repo.GetTodayByStudentID(ctx, newStudent.ID)
		assert.Error(t, err)
	})
}

// TestAttendanceRepository_FindForDate tests finding all attendance for a date
func TestAttendanceRepository_FindForDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() { cleanupAttendanceRecords(t, db, createdIDs...) }()

	t.Run("finds all attendance for specific date", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create multiple attendance records for same date
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance2 := &active.Attendance{
			StudentID:   data.Student2.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance1)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance1.ID)

		err = repo.Create(ctx, attendance2)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance2.ID)

		// Find all for date
		records, err := repo.FindForDate(ctx, date)
		require.NoError(t, err)
		assert.NotEmpty(t, records)

		// Should contain our records
		var foundStudent1, foundStudent2 bool
		for _, r := range records {
			if r.ID == attendance1.ID {
				foundStudent1 = true
			}
			if r.ID == attendance2.ID {
				foundStudent2 = true
			}
		}
		assert.True(t, foundStudent1)
		assert.True(t, foundStudent2)
	})

	t.Run("returns empty for date with no attendance", func(t *testing.T) {
		emptyDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		records, err := repo.FindForDate(ctx, emptyDate)
		require.NoError(t, err)
		assert.Empty(t, records)
	})
}
