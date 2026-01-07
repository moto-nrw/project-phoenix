package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
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

// testData holds test entities needed for attendance tests
type testData struct {
	Student1 *users.Student
	Student2 *users.Student
	Staff1   *users.Staff
	Staff2   *users.Staff
	Device1  *iot.Device
	Person1  *users.Person
	Person2  *users.Person
	Person3  *users.Person
	Person4  *users.Person
}

// createTestData creates the required foreign key entities for attendance testing
func createTestData(t *testing.T, db *bun.DB) *testData {
	ctx := context.Background()
	data := &testData{}

	// Create persons for students and staff
	data.Person1 = &users.Person{
		Model:     base.Model{ID: 9001},
		FirstName: "Test",
		LastName:  "Student1",
	}
	data.Person2 = &users.Person{
		Model:     base.Model{ID: 9002},
		FirstName: "Test",
		LastName:  "Student2",
	}
	data.Person3 = &users.Person{
		Model:     base.Model{ID: 9003},
		FirstName: "Test",
		LastName:  "Staff1",
	}
	data.Person4 = &users.Person{
		Model:     base.Model{ID: 9004},
		FirstName: "Test",
		LastName:  "Staff2",
	}

	// Insert persons
	for _, person := range []*users.Person{data.Person1, data.Person2, data.Person3, data.Person4} {
		_, err := db.NewInsert().
			Model(person).
			ModelTableExpr(`users.persons AS "person"`).
			Exec(ctx)
		require.NoError(t, err, "Failed to create test person")
	}

	// Create staff entries
	data.Staff1 = &users.Staff{
		Model:    base.Model{ID: 9001},
		PersonID: data.Person3.ID,
	}
	data.Staff2 = &users.Staff{
		Model:    base.Model{ID: 9002},
		PersonID: data.Person4.ID,
	}

	for _, staff := range []*users.Staff{data.Staff1, data.Staff2} {
		_, err := db.NewInsert().
			Model(staff).
			ModelTableExpr(`users.staff AS "staff"`).
			Exec(ctx)
		require.NoError(t, err, "Failed to create test staff")
	}

	// Create students
	data.Student1 = &users.Student{
		Model:    base.Model{ID: 9001},
		PersonID: data.Person1.ID,
	}
	data.Student2 = &users.Student{
		Model:    base.Model{ID: 9002},
		PersonID: data.Person2.ID,
	}

	for _, student := range []*users.Student{data.Student1, data.Student2} {
		_, err := db.NewInsert().
			Model(student).
			ModelTableExpr(`users.students AS "student"`).
			Exec(ctx)
		require.NoError(t, err, "Failed to create test student")
	}

	// Create test device
	deviceID := "TestDevice1"
	apiKey := "test_api_key_123"
	data.Device1 = &iot.Device{
		Model:      base.Model{ID: 9001},
		DeviceID:   deviceID,
		DeviceType: "RFID",
		APIKey:     &apiKey,
		Status:     iot.DeviceStatusActive,
	}

	_, err := db.NewInsert().
		Model(data.Device1).
		ModelTableExpr(`iot.devices AS "device"`).
		Exec(ctx)
	require.NoError(t, err, "Failed to create test device")

	return data
}

// cleanupTestData removes test data from database
func cleanupTestData(t *testing.T, db *bun.DB, attendanceIDs ...int64) {
	ctx := context.Background()

	// Clean up attendance records
	for _, id := range attendanceIDs {
		_, err := db.NewDelete().
			Model((*active.Attendance)(nil)).
			ModelTableExpr(`active.attendance AS "attendance"`).
			Where(`"attendance".id = ?`, id).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup attendance record %d: %v", id, err)
		}
	}

	// Clean up test entities (in reverse dependency order)
	testIDs := []int64{9001, 9002}

	// Students
	for _, id := range testIDs {
		_, err := db.NewDelete().
			Model((*users.Student)(nil)).
			ModelTableExpr(`users.students AS "student"`).
			Where(`"student".id = ?`, id).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup student %d: %v", id, err)
		}
	}

	// Staff
	for _, id := range testIDs {
		_, err := db.NewDelete().
			Model((*users.Staff)(nil)).
			ModelTableExpr(`users.staff AS "staff"`).
			Where(`"staff".id = ?`, id).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup staff %d: %v", id, err)
		}
	}

	// Persons
	for _, id := range []int64{9001, 9002, 9003, 9004} {
		_, err := db.NewDelete().
			Model((*users.Person)(nil)).
			ModelTableExpr(`users.persons AS "person"`).
			Where(`"person".id = ?`, id).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: Failed to cleanup person %d: %v", id, err)
		}
	}

	// Device
	_, err := db.NewDelete().
		Model((*iot.Device)(nil)).
		ModelTableExpr(`iot.devices AS "device"`).
		Where(`"device".id = ?`, 9001).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: Failed to cleanup device: %v", err)
	}
}

// TestAttendanceRepository_Create tests basic record creation
func TestAttendanceRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createTestData(t, db)
	defer cleanupTestData(t, db)

	t.Run("create valid attendance record", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)

		// Verify ID was assigned
		assert.NotZero(t, attendance.ID)

		// Verify timestamps were set
		assert.False(t, attendance.CreatedAt.IsZero())
		assert.False(t, attendance.UpdatedAt.IsZero())

		defer cleanupTestData(t, db, attendance.ID)
	})

	t.Run("create with check-out time", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
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

		assert.NotZero(t, attendance.ID)
		assert.NotNil(t, attendance.CheckOutTime)
		assert.Equal(t, checkOutTime.Unix(), attendance.CheckOutTime.Unix())
		assert.NotNil(t, attendance.CheckedOutBy)
		assert.Equal(t, checkedOutBy, *attendance.CheckedOutBy)

		defer cleanupTestData(t, db, attendance.ID)
	})

	t.Run("create with nil attendance should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("verify IsCheckedIn helper method", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create attendance without check-out
		attendanceCheckedIn := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendanceCheckedIn)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendanceCheckedIn.ID)

		assert.True(t, attendanceCheckedIn.IsCheckedIn(), "Should be checked in when CheckOutTime is nil")

		// Create attendance with check-out
		checkOutTime := now.Add(1 * time.Hour)
		checkedOutBy := data.Staff1.ID
		attendanceCheckedOut := &active.Attendance{
			StudentID:    data.Student2.ID,
			Date:         date,
			CheckInTime:  now,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		err = repo.Create(ctx, attendanceCheckedOut)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendanceCheckedOut.ID)

		assert.False(t, attendanceCheckedOut.IsCheckedIn(), "Should not be checked in when CheckOutTime is set")
	})
}

// TestAttendanceRepository_FindByStudentAndDate tests querying attendance records by student and date
func TestAttendanceRepository_FindByStudentAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createTestData(t, db)
	defer cleanupTestData(t, db)

	t.Run("single record for student on date", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Find records for this student and date
		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date)
		require.NoError(t, err)

		assert.Len(t, records, 1, "Should find exactly one record")
		assert.Equal(t, attendance.ID, records[0].ID)
		assert.Equal(t, data.Student1.ID, records[0].StudentID)
		assert.Equal(t, date.Unix(), records[0].Date.Unix())
	})

	t.Run("multiple records for student on same date ordered by check-in time", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

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
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID, attendance3.ID)

		// Find records for this student and date
		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date)
		require.NoError(t, err)

		assert.Len(t, records, 3, "Should find exactly three records")

		// Verify ordering by check_in_time ASC
		assert.True(t, records[0].CheckInTime.Before(records[1].CheckInTime), "First record should be earliest")
		assert.True(t, records[1].CheckInTime.Before(records[2].CheckInTime), "Second record should be middle")
		assert.Equal(t, attendance1.ID, records[0].ID, "First record should be attendance1")
		assert.Equal(t, attendance2.ID, records[1].ID, "Second record should be attendance2")
		assert.Equal(t, attendance3.ID, records[2].ID, "Third record should be attendance3")
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
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Query with different time component but same date
		queryDate := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 45, 0, now.Location())

		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, queryDate)
		require.NoError(t, err)

		assert.Len(t, records, 1, "Should find record regardless of time component in query date")
		assert.Equal(t, attendance.ID, records[0].ID)
	})

	t.Run("different students on same date", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create attendance for student1
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2
		attendance2 := &active.Attendance{
			StudentID:   data.Student2.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID)

		// Query for student1 should only return their record
		records1, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date)
		require.NoError(t, err)
		assert.Len(t, records1, 1, "Should find only student1's record")
		assert.Equal(t, data.Student1.ID, records1[0].StudentID)

		// Query for student2 should only return their record
		records2, err := repo.FindByStudentAndDate(ctx, data.Student2.ID, date)
		require.NoError(t, err)
		assert.Len(t, records2, 1, "Should find only student2's record")
		assert.Equal(t, data.Student2.ID, records2[0].StudentID)
	})

	t.Run("different dates for same student", func(t *testing.T) {
		now := time.Now()
		date1 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		date2 := date1.AddDate(0, 0, 1) // Next day

		// Create attendance for date1
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date1,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2
		attendance2 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date2,
			CheckInTime: now.Add(24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID)

		// Query for date1 should only return that day's record
		records1, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date1)
		require.NoError(t, err)
		assert.Len(t, records1, 1, "Should find only date1's record")
		assert.Equal(t, date1.Unix(), records1[0].Date.Unix())

		// Query for date2 should only return that day's record
		records2, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, date2)
		require.NoError(t, err)
		assert.Len(t, records2, 1, "Should find only date2's record")
		assert.Equal(t, date2.Unix(), records2[0].Date.Unix())
	})
}

// TestAttendanceRepository_FindLatestByStudent tests finding the most recent attendance record for a student
func TestAttendanceRepository_FindLatestByStudent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createTestData(t, db)
	defer cleanupTestData(t, db)

	t.Run("latest record across multiple dates", func(t *testing.T) {
		now := time.Now()
		date1 := time.Date(now.Year(), now.Month(), now.Day()-2, 0, 0, 0, 0, now.Location()) // 2 days ago
		date2 := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location()) // Yesterday
		date3 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())   // Today

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
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID, attendance3.ID)

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		assert.Equal(t, attendance3.ID, latest.ID, "Should return the record from the latest date")
		assert.Equal(t, date3.Unix(), latest.Date.Unix())
	})

	t.Run("latest record same day with multiple check-ins", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

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
			CheckInTime: now, // Later
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID)

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		assert.Equal(t, attendance2.ID, latest.ID, "Should return the record with latest check-in time")
		assert.Equal(t, now.Unix(), latest.CheckInTime.Unix())
	})

	t.Run("no records for student", func(t *testing.T) {
		// Try to find latest record for student with no attendance
		latest, err := repo.FindLatestByStudent(ctx, data.Student2.ID)

		// This should return a database error (no rows found)
		assert.Error(t, err, "Should return error when no records exist")
		assert.Nil(t, latest, "Should return nil when no records exist")
	})

	t.Run("single record for student", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		assert.Equal(t, attendance.ID, latest.ID, "Should return the only record")
		assert.Equal(t, data.Student1.ID, latest.StudentID)
		assert.Equal(t, date.Unix(), latest.Date.Unix())
	})

	t.Run("complex scenario - mixed dates and times", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		yesterday := today.AddDate(0, 0, -1)

		// Yesterday: multiple records
		attendanceYesterday1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-30 * time.Hour), // Earlier yesterday
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendanceYesterday2 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-25 * time.Hour), // Later yesterday
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Today: single record but earlier in the day than latest yesterday record
		attendanceToday := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: now.Add(-2 * time.Hour), // Early today
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendanceYesterday1, attendanceYesterday2, attendanceToday} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
		}
		defer cleanupTestData(t, db, attendanceYesterday1.ID, attendanceYesterday2.ID, attendanceToday.ID)

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Should return today's record even though yesterday had later times
		// because date takes precedence over time in the ordering
		assert.Equal(t, attendanceToday.ID, latest.ID, "Should return today's record (latest by date)")
		assert.Equal(t, today.Unix(), latest.Date.Unix())
	})

	t.Run("different students do not interfere", func(t *testing.T) {
		now := time.Now()
		date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create attendance for student1 (earlier)
		attendanceStudent1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date,
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2 (later)
		attendanceStudent2 := &active.Attendance{
			StudentID:   data.Student2.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendanceStudent1, attendanceStudent2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
		}
		defer cleanupTestData(t, db, attendanceStudent1.ID, attendanceStudent2.ID)

		// Latest for student1 should be their record
		latest1, err := repo.FindLatestByStudent(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, latest1)
		assert.Equal(t, attendanceStudent1.ID, latest1.ID)
		assert.Equal(t, data.Student1.ID, latest1.StudentID)

		// Latest for student2 should be their record
		latest2, err := repo.FindLatestByStudent(ctx, data.Student2.ID)
		require.NoError(t, err)
		require.NotNil(t, latest2)
		assert.Equal(t, attendanceStudent2.ID, latest2.ID)
		assert.Equal(t, data.Student2.ID, latest2.StudentID)
	})
}

// TestAttendanceRepository_GetStudentCurrentStatus tests getting today's latest attendance record for a student
func TestAttendanceRepository_GetStudentCurrentStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	}()

	repo := setupAttendanceRepo(t, db)
	ctx := context.Background()
	data := createTestData(t, db)
	defer cleanupTestData(t, db)

	t.Run("no records today - student not checked in", func(t *testing.T) {
		// Try to get current status for student with no attendance today
		status, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)

		// Should return error (no rows found) when no attendance today
		assert.Error(t, err, "Should return error when no attendance records exist for today")
		assert.Nil(t, status, "Should return nil when no records exist for today")
	})

	t.Run("student checked in - latest record has no check-out time", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
			// CheckOutTime is nil - student is checked in
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Get current status
		status, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance.ID, status.ID)
		assert.Equal(t, data.Student1.ID, status.StudentID)
		assert.Equal(t, today.Unix(), status.Date.Unix())
		assert.Nil(t, status.CheckOutTime, "CheckOutTime should be nil for checked-in student")
		assert.True(t, status.IsCheckedIn(), "Student should be checked in")
	})

	t.Run("student checked out - latest record has check-out time", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		checkOutTime := now.Add(2 * time.Hour)
		checkedOutBy := data.Staff2.ID

		attendance := &active.Attendance{
			StudentID:    data.Student1.ID,
			Date:         today,
			CheckInTime:  now,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Get current status
		status, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance.ID, status.ID)
		assert.Equal(t, data.Student1.ID, status.StudentID)
		assert.NotNil(t, status.CheckOutTime, "CheckOutTime should be set for checked-out student")
		assert.Equal(t, checkOutTime.Unix(), status.CheckOutTime.Unix())
		assert.False(t, status.IsCheckedIn(), "Student should not be checked in")
	})

	t.Run("multiple records today - returns latest by check-in time", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// First check-in (earlier)
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: now.Add(-3 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Check-out from first session
		checkOutTime1 := now.Add(-2 * time.Hour)
		checkedOutBy1 := data.Staff1.ID
		attendance2 := &active.Attendance{
			StudentID:    data.Student1.ID,
			Date:         today,
			CheckInTime:  now.Add(-3 * time.Hour),
			CheckOutTime: &checkOutTime1,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy1,
			DeviceID:     data.Device1.ID,
		}

		// Second check-in (latest)
		attendance3 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: now.Add(-1 * time.Hour), // Latest check-in time
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2, attendance3} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID, attendance3.ID)

		// Get current status - should return the latest check-in
		status, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance3.ID, status.ID, "Should return the record with latest check-in time")
		assert.Equal(t, now.Add(-1*time.Hour).Unix(), status.CheckInTime.Unix())
		assert.Nil(t, status.CheckOutTime, "Latest record should not have check-out time")
		assert.True(t, status.IsCheckedIn(), "Student should be checked in from latest record")
	})

	t.Run("historical records exist but none today", func(t *testing.T) {
		now := time.Now()
		yesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())

		// Create attendance for yesterday
		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Get current status - should not find yesterday's record
		status, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)

		assert.Error(t, err, "Should return error when no records exist for today")
		assert.Nil(t, status, "Should return nil when only historical records exist")
	})

	t.Run("different students on same day", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create attendance for student1
		attendance1 := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2 with check-out
		checkOutTime2 := now
		checkedOutBy2 := data.Staff1.ID
		attendance2 := &active.Attendance{
			StudentID:    data.Student2.ID,
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
		}
		defer cleanupTestData(t, db, attendance1.ID, attendance2.ID)

		// Get status for student1 - should be checked in
		status1, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, status1)
		assert.Equal(t, data.Student1.ID, status1.StudentID)
		assert.Nil(t, status1.CheckOutTime)
		assert.True(t, status1.IsCheckedIn())

		// Get status for student2 - should be checked out
		status2, err := repo.GetStudentCurrentStatus(ctx, data.Student2.ID)
		require.NoError(t, err)
		require.NotNil(t, status2)
		assert.Equal(t, data.Student2.ID, status2.StudentID)
		assert.NotNil(t, status2.CheckOutTime)
		assert.False(t, status2.IsCheckedIn())
	})

	t.Run("timezone handling - today calculation", func(t *testing.T) {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// Create attendance record for today but late in the day
		attendance := &active.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today,
			CheckInTime: today.Add(23 * time.Hour), // Late in the day
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		defer cleanupTestData(t, db, attendance.ID)

		// Get current status - should find the record regardless of time
		status, err := repo.GetStudentCurrentStatus(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, attendance.ID, status.ID)
		assert.Equal(t, today.Unix(), status.Date.Unix())
	})
}
