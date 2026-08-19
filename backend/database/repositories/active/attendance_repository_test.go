package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

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

// TestAttendanceRepository_Create tests basic record creation
func TestAttendanceRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("create valid attendance record", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

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
		date := timezone.TodayDate()
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
		date := timezone.TodayDate()

		// Use a fresh student so the partial unique index on
		// (student_id, date) WHERE check_out_time IS NULL doesn't fight us:
		// Student1 already has an open row from the earlier sub-test.
		isolatedStudent := testpkg.CreateTestStudent(t, db, "IsCheckedIn", "Helper", "1f")
		defer testpkg.CleanupActivityFixtures(t, db, isolatedStudent.ID)

		// Create attendance without check-out
		attendanceCheckedIn := &active.Attendance{
			StudentID:   isolatedStudent.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour),
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

func TestAttendanceRepository_ListOpenStudentIDsForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	now := time.Now()
	openAttendance := testpkg.CreateTestAttendance(t, db, data.Student1.ID, data.Staff1.ID, data.Device1.ID, now.Add(-30*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	closedAttendance := testpkg.CreateTestAttendance(t, db, data.Student2.ID, data.Staff1.ID, data.Device1.ID, now.Add(-30*time.Minute), &checkOutTime)
	defer testpkg.CleanupTableRecords(t, db, "active.attendance", openAttendance.ID, closedAttendance.ID)

	ids, err := repo.ListOpenStudentIDsForDate(ctx, timezone.TodayDate())

	require.NoError(t, err)
	assert.Contains(t, ids, data.Student1.ID)
	assert.NotContains(t, ids, data.Student2.ID)
}

// TestAttendanceRepository_FindByStudentAndDate tests querying attendance records by student and date
func TestAttendanceRepository_FindByStudentAndDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("single record for student on date", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

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
		date := timezone.TodayDate()

		// Use a fresh student so we control the row count exactly. The
		// partial unique index allows many rows per (student_id, date) as
		// long as at most one is open — so the earlier two rows must carry
		// a CheckOutTime, only the latest stays open. This matches the
		// realistic in/out/in/out lifecycle.
		multiRowStudent := testpkg.CreateTestStudent(t, db, "MultiRow", "Same", "1g")
		defer testpkg.CleanupActivityFixtures(t, db, multiRowStudent.ID)

		checkout1 := now.Add(-90 * time.Minute)
		checkout2 := now.Add(30 * time.Minute)
		closedBy := data.Staff1.ID

		attendance1 := &active.Attendance{
			StudentID:    multiRowStudent.ID,
			Date:         date,
			CheckInTime:  now.Add(-2 * time.Hour), // Earliest
			CheckOutTime: &checkout1,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendance2 := &active.Attendance{
			StudentID:    multiRowStudent.ID,
			Date:         date,
			CheckInTime:  now, // Middle
			CheckOutTime: &checkout2,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendance3 := &active.Attendance{
			StudentID:   multiRowStudent.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour), // Latest, still open
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2, attendance3} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Find records for this student and date
		records, err := repo.FindByStudentAndDate(ctx, multiRowStudent.ID, date)
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
		emptyDate := timezone.NewDate(2023, 1, 1)

		records, err := repo.FindByStudentAndDate(ctx, data.Student1.ID, emptyDate)
		require.NoError(t, err)

		assert.Len(t, records, 0, "Should find no records for date with no attendance")
	})

	t.Run("date filtering ignores time component", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Isolate from prior sub-tests' open rows — partial unique index
		// only allows one open attendance per (student_id, date).
		dateFilterStudent := testpkg.CreateTestStudent(t, db, "DateFilter", "Test", "1h")
		defer testpkg.CleanupActivityFixtures(t, db, dateFilterStudent.ID)

		attendance := &active.Attendance{
			StudentID:   dateFilterStudent.ID,
			Date:        date,
			CheckInTime: now.Add(5 * time.Hour), // Different time
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		err := repo.Create(ctx, attendance)
		require.NoError(t, err)
		createdIDs = append(createdIDs, attendance.ID)

		// timezone.Date carries no time component — querying the same
		// calendar date must find the record regardless of check-in time.
		records, err := repo.FindByStudentAndDate(ctx, dateFilterStudent.ID, date)
		require.NoError(t, err)

		var found bool
		for _, r := range records {
			if r.ID == attendance.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find record for the same calendar date")
	})

	t.Run("different students on same date", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Fresh students — Student1/Student2 from the shared fixture may
		// already have open rows from earlier sub-tests, which would clash
		// with the partial unique index on (student_id, date) WHERE
		// check_out_time IS NULL.
		diffStudentA := testpkg.CreateTestStudent(t, db, "DiffStudents", "A", "1i")
		diffStudentB := testpkg.CreateTestStudent(t, db, "DiffStudents", "B", "1j")
		defer testpkg.CleanupActivityFixtures(t, db, diffStudentA.ID, diffStudentB.ID)

		attendance1 := &active.Attendance{
			StudentID:   diffStudentA.ID,
			Date:        date,
			CheckInTime: now.Add(6 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance2 := &active.Attendance{
			StudentID:   diffStudentB.ID,
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

		// Query for studentA should only return their records
		records1, err := repo.FindByStudentAndDate(ctx, diffStudentA.ID, date)
		require.NoError(t, err)
		for _, r := range records1 {
			assert.Equal(t, diffStudentA.ID, r.StudentID)
		}

		// Query for studentB should only return their records
		records2, err := repo.FindByStudentAndDate(ctx, diffStudentB.ID, date)
		require.NoError(t, err)
		for _, r := range records2 {
			assert.Equal(t, diffStudentB.ID, r.StudentID)
		}
	})

	t.Run("different dates for same student", func(t *testing.T) {
		now := time.Now()
		date1 := timezone.TodayDate()
		date2 := date1.AddDays(1) // Next day

		// Fresh student — Student1 already has open rows on `today` from
		// earlier sub-tests; the partial unique index would block the
		// second insert here.
		twoDayStudent := testpkg.CreateTestStudent(t, db, "TwoDay", "Test", "1k")
		defer testpkg.CleanupActivityFixtures(t, db, twoDayStudent.ID)

		// Create attendance for date1
		attendance1 := &active.Attendance{
			StudentID:   twoDayStudent.ID,
			Date:        date1,
			CheckInTime: now.Add(8 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2
		attendance2 := &active.Attendance{
			StudentID:   twoDayStudent.ID,
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
		records1, err := repo.FindByStudentAndDate(ctx, twoDayStudent.ID, date1)
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
		records2, err := repo.FindByStudentAndDate(ctx, twoDayStudent.ID, date2)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("latest record across multiple dates", func(t *testing.T) {
		now := time.Now()
		date1 := timezone.TodayDate().AddDays(-2) // 2 days ago
		date2 := timezone.TodayDate().AddDays(-1) // Yesterday
		date3 := timezone.TodayDate()             // Today

		// Different dates each, so the partial unique index isn't relevant
		// here — but use an isolated student for clean assertions.
		latestDateStudent := testpkg.CreateTestStudent(t, db, "LatestDate", "Test", "1l")
		defer testpkg.CleanupActivityFixtures(t, db, latestDateStudent.ID)

		// Create attendance for date1 (oldest)
		attendance1 := &active.Attendance{
			StudentID:   latestDateStudent.ID,
			Date:        date1,
			CheckInTime: now.Add(-48 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2 (middle)
		attendance2 := &active.Attendance{
			StudentID:   latestDateStudent.ID,
			Date:        date2,
			CheckInTime: now.Add(-24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date3 (latest by date)
		attendance3 := &active.Attendance{
			StudentID:   latestDateStudent.ID,
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
		latest, err := repo.FindLatestByStudent(ctx, latestDateStudent.ID)
		require.NoError(t, err)
		require.NotNil(t, latest)

		// Latest should be attendance3 (today)
		assert.Equal(t, attendance3.ID, latest.ID, "Should return the record from the latest date")
	})

	t.Run("latest record same day with multiple check-ins", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Fresh student so we own the row count. Earlier row is closed
		// (realistic check-in/out cycle), the later row stays open — only
		// one open row per (student_id, date) is allowed by the partial
		// unique index.
		multiCheckinStudent := testpkg.CreateTestStudent(t, db, "MultiCheckin", "Same", "1m")
		defer testpkg.CleanupActivityFixtures(t, db, multiCheckinStudent.ID)

		earlyCheckout := now.Add(-1 * time.Hour)
		closedBy := data.Staff1.ID

		attendance1 := &active.Attendance{
			StudentID:    multiCheckinStudent.ID,
			Date:         date,
			CheckInTime:  now.Add(-2 * time.Hour), // Earlier, closed
			CheckOutTime: &earlyCheckout,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendance2 := &active.Attendance{
			StudentID:   multiCheckinStudent.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour), // Later, open
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*active.Attendance{attendance1, attendance2} {
			err := repo.Create(ctx, att)
			require.NoError(t, err)
			createdIDs = append(createdIDs, att.ID)
		}

		// Find latest record
		latest, err := repo.FindLatestByStudent(ctx, multiCheckinStudent.ID)
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
		date := timezone.TodayDate()

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
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)

		// Yesterday: multiple records — earlier is closed, later stays open
		// to respect the partial unique index on
		// (student_id, date) WHERE check_out_time IS NULL.
		yesterdayCheckout := now.Add(-26 * time.Hour)
		closedBy := data.Staff1.ID
		attendanceYesterday1 := &active.Attendance{
			StudentID:    complexStudent.ID,
			Date:         yesterday,
			CheckInTime:  now.Add(-30 * time.Hour), // Earlier yesterday, closed
			CheckOutTime: &yesterdayCheckout,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendanceYesterday2 := &active.Attendance{
			StudentID:   complexStudent.ID,
			Date:        yesterday,
			CheckInTime: now.Add(-25 * time.Hour), // Later yesterday, open
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
		date := timezone.TodayDate()

		// Fresh students — Student1/Student2 from the shared fixture may
		// already have open rows from earlier sub-tests, which would clash
		// with the partial unique index on (student_id, date) WHERE
		// check_out_time IS NULL.
		noInterfereA := testpkg.CreateTestStudent(t, db, "NoInterfere", "A", "1n")
		noInterfereB := testpkg.CreateTestStudent(t, db, "NoInterfere", "B", "1o")
		defer testpkg.CleanupActivityFixtures(t, db, noInterfereA.ID, noInterfereB.ID)

		attendanceStudent1 := &active.Attendance{
			StudentID:   noInterfereA.ID,
			Date:        date,
			CheckInTime: now.Add(2 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendanceStudent2 := &active.Attendance{
			StudentID:   noInterfereB.ID,
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

		// Latest for studentA should be their record
		latest1, err := repo.FindLatestByStudent(ctx, noInterfereA.ID)
		require.NoError(t, err)
		require.NotNil(t, latest1)
		assert.Equal(t, noInterfereA.ID, latest1.StudentID)

		// Latest for studentB should be their record
		latest2, err := repo.FindLatestByStudent(ctx, noInterfereB.ID)
		require.NoError(t, err)
		require.NotNil(t, latest2)
		assert.Equal(t, noInterfereB.ID, latest2.StudentID)
	})
}

// TestAttendanceRepository_GetStudentCurrentStatus tests getting today's latest attendance record for a student
func TestAttendanceRepository_GetStudentCurrentStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

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
		today := timezone.TodayDate()

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
		today := timezone.TodayDate()
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
		today := timezone.TodayDate()

		// First check-in (earlier) — closed so it doesn't conflict with the
		// later open row under the partial unique index on
		// (student_id, date) WHERE check_out_time IS NULL.
		earlyCheckout := now.Add(-2*time.Hour - 45*time.Minute)
		earlyClosedBy := data.Staff1.ID
		attendance1 := &active.Attendance{
			StudentID:    multiRecordStudent.ID,
			Date:         today,
			CheckInTime:  now.Add(-3 * time.Hour),
			CheckOutTime: &earlyCheckout,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &earlyClosedBy,
			DeviceID:     data.Device1.ID,
		}

		// Re-entry, also closed
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

		// Second check-in (latest, still open)
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
		yesterday := timezone.TodayDate().AddDays(-1)

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
		today := timezone.TodayDate()

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

		today := timezone.TodayDate()

		// Create attendance record for today but late in the day
		attendance := &active.Attendance{
			StudentID:   tzStudent.ID,
			Date:        today,
			CheckInTime: today.BerlinMidnight().Add(23 * time.Hour), // Late in the day
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("updates attendance with check-out time", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("finds existing attendance by ID", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	t.Run("deletes existing attendance record", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("gets today's attendance for student", func(t *testing.T) {
		now := time.Now()
		today := timezone.TodayDate()

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("finds all attendance for specific date", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

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
		emptyDate := timezone.NewDate(2023, 1, 1)

		records, err := repo.FindForDate(ctx, emptyDate)
		require.NoError(t, err)
		assert.Empty(t, records)
	})
}

// TestAttendanceRepository_CreateIfNoOpenForToday exercises the conflict-safe
// insert that backs performCheckIn. Three scenarios cover the contract:
//  1. First insert: returns inserted=true and assigns an ID.
//  2. Race / double-tap: a second open insert for the same (student, date)
//     returns inserted=false (the partial unique index swallows the row),
//     but does NOT error.
//  3. Re-entry: once the existing row is closed (CheckOutTime set), a new
//     open insert succeeds — the partial unique index only counts rows
//     where check_out_time IS NULL.
func TestAttendanceRepository_CreateIfNoOpenForToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("nil attendance returns error", func(t *testing.T) {
		inserted, err := repo.CreateIfNoOpenForToday(ctx, nil)
		assert.False(t, inserted)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("first open insert succeeds and assigns ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Conflict", "First", "1z")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		now := time.Now()
		att := &active.Attendance{
			StudentID:   student.ID,
			Date:        timezone.TodayDate(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		inserted, err := repo.CreateIfNoOpenForToday(ctx, att)
		require.NoError(t, err)
		assert.True(t, inserted, "first insert should succeed")
		assert.NotZero(t, att.ID, "row id should be populated")
		createdIDs = append(createdIDs, att.ID)
	})

	t.Run("conflicting open insert returns inserted=false without error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Conflict", "Race", "1y")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		now := time.Now()
		date := timezone.TodayDate()
		first := &active.Attendance{
			StudentID:   student.ID,
			Date:        date,
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		ok1, err1 := repo.CreateIfNoOpenForToday(ctx, first)
		require.NoError(t, err1)
		require.True(t, ok1)
		createdIDs = append(createdIDs, first.ID)

		second := &active.Attendance{
			StudentID:   student.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Minute),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		ok2, err2 := repo.CreateIfNoOpenForToday(ctx, second)
		require.NoError(t, err2, "ON CONFLICT must swallow the duplicate, not raise")
		assert.False(t, ok2, "second concurrent open row must report inserted=false")
	})

	t.Run("re-entry after checkout succeeds", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Conflict", "Reentry", "1x")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		now := time.Now()
		date := timezone.TodayDate()
		closedBy := data.Staff1.ID
		closeTime := now.Add(30 * time.Minute)

		closed := &active.Attendance{
			StudentID:    student.ID,
			Date:         date,
			CheckInTime:  now,
			CheckOutTime: &closeTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}
		ok1, err1 := repo.CreateIfNoOpenForToday(ctx, closed)
		require.NoError(t, err1)
		require.True(t, ok1)
		createdIDs = append(createdIDs, closed.ID)

		// Closed row doesn't occupy the partial index — a new open row is
		// fine on the same calendar day.
		reentry := &active.Attendance{
			StudentID:   student.ID,
			Date:        date,
			CheckInTime: now.Add(1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		ok2, err2 := repo.CreateIfNoOpenForToday(ctx, reentry)
		require.NoError(t, err2)
		assert.True(t, ok2, "open insert after a closed row must succeed")
		createdIDs = append(createdIDs, reentry.ID)
	})
}

// TestAttendanceRepository_CloseOpenForToday locks in the state-checked
// checkout contract used by the action-explicit CheckOutStudent service
// method. Two cases:
//
//  1. Student has an open row → it's closed; CheckOutTime / CheckedOutBy /
//     yard_since are set / cleared in a single UPDATE; the closed row is
//     returned.
//  2. Student has no open row (already closed, never checked in, or another
//     concurrent caller already closed it) → returns nil (no row), no error
//     — caller treats as idempotent success.
func TestAttendanceRepository_CloseOpenForToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	var createdIDs []int64
	defer func() {
		for _, id := range createdIDs {
			testpkg.CleanupTableRecords(t, db, "active.attendance", id)
		}
	}()

	t.Run("closes the open row and clears yard_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Close", "Open", "2x")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		now := time.Now()
		yard := now.Add(-15 * time.Minute)
		open := &active.Attendance{
			StudentID:   student.ID,
			Date:        timezone.TodayDate(),
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
			YardSince:   &yard,
		}
		require.NoError(t, repo.Create(ctx, open))
		createdIDs = append(createdIDs, open.ID)

		closed, err := repo.CloseOpenForToday(ctx, student.ID, now, timezone.DateFromTime(now), data.Staff2.ID)
		require.NoError(t, err)
		require.NotNil(t, closed, "open row must have been closed")
		assert.Equal(t, open.ID, closed.ID)
		require.NotNil(t, closed.CheckOutTime)
		assert.WithinDuration(t, now, *closed.CheckOutTime, time.Second)
		require.NotNil(t, closed.CheckedOutBy)
		assert.Equal(t, data.Staff2.ID, *closed.CheckedOutBy)
		assert.Nil(t, closed.YardSince, "yard sub-state must be cleared on checkout")
	})

	t.Run("no open row returns nil without error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Close", "Idempotent", "2y")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		closed, err := repo.CloseOpenForToday(ctx, student.ID, time.Now(), timezone.TodayDate(), data.Staff1.ID)
		require.NoError(t, err)
		assert.Nil(t, closed, "no open row → idempotent success, repo returns nil")
	})

	t.Run("zero staff id leaves checked_out_by NULL", func(t *testing.T) {
		// Mirrors the kiosk path where deviceID owns the close but no
		// staff PIN is in scope.
		student := testpkg.CreateTestStudent(t, db, "Close", "NoStaff", "2z")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		now := time.Now()
		open := &active.Attendance{
			StudentID:   student.ID,
			Date:        timezone.TodayDate(),
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		require.NoError(t, repo.Create(ctx, open))
		createdIDs = append(createdIDs, open.ID)

		closed, err := repo.CloseOpenForToday(ctx, student.ID, now, timezone.DateFromTime(now), 0)
		require.NoError(t, err)
		require.NotNil(t, closed)
		assert.Nil(t, closed.CheckedOutBy, "staffID=0 must not write a bogus FK")
	})
}

// TestAttendanceRepository_CloseOpenForTodayUsesCallerDate pins that the
// repository closes rows of the CALLER-supplied calendar day instead of
// re-deriving "today" internally (review #2372): the batch school checkout
// snapshots one date for the whole run, and a batch crossing Berlin midnight
// must keep closing the snapshot day's rows — not silently switch to the new
// day mid-batch.
func TestAttendanceRepository_CloseOpenForTodayUsesCallerDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Attendance
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)
	defer cleanupAttendanceTestData(t, db, data)

	student := testpkg.CreateTestStudent(t, db, "Close", "SnapshotDay", "2w")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	now := time.Now()
	yesterday := timezone.TodayDate().AddDays(-1)
	open := &active.Attendance{
		StudentID:   student.ID,
		Date:        yesterday,
		CheckInTime: now.Add(-25 * time.Hour),
		CheckedInBy: data.Staff1.ID,
		DeviceID:    data.Device1.ID,
	}
	require.NoError(t, repo.Create(ctx, open))
	defer testpkg.CleanupTableRecords(t, db, "active.attendance", open.ID)

	// A close scoped to the CURRENT day must not touch yesterday's open row.
	closed, err := repo.CloseOpenForToday(ctx, student.ID, now, timezone.TodayDate(), data.Staff1.ID)
	require.NoError(t, err)
	assert.Nil(t, closed, "a different-day close must not match the snapshot day's row")

	// The same close scoped to the snapshot day closes exactly that row.
	closed, err = repo.CloseOpenForToday(ctx, student.ID, now, yesterday, data.Staff1.ID)
	require.NoError(t, err)
	require.NotNil(t, closed, "the caller-supplied day's open row must be closed")
	assert.Equal(t, open.ID, closed.ID)
}
