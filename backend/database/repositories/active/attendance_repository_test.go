package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
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

// TestAttendanceRepository_Create tests basic record creation
func TestPresenceAttendance_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("create valid attendance record", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		attendance := studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance, err := repo.RecordAttendance(ctx, attendance)
		require.NoError(t, err)

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

		attendance := studentpresence.Attendance{
			StudentID:    data.Student2.ID,
			Date:         date.String(),
			CheckInTime:  now,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		attendance, err := repo.RecordAttendance(ctx, attendance)
		require.NoError(t, err)

		assert.NotZero(t, attendance.ID)
		assert.NotNil(t, attendance.CheckOutTime)
		assert.Equal(t, checkOutTime.Unix(), attendance.CheckOutTime.Unix())
		assert.NotNil(t, attendance.CheckedOutBy)
		assert.Equal(t, checkedOutBy, *attendance.CheckedOutBy)
	})

	t.Run("create with empty attendance should fail", func(t *testing.T) {
		_, err := repo.RecordAttendance(ctx, studentpresence.Attendance{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid calendar date")
	})

	t.Run("preserves open and closed attendance", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Use a fresh student so the partial unique index on
		// (student_id, date) WHERE check_out_time IS NULL doesn't fight us:
		// Student1 already has an open row from the earlier sub-test.
		isolatedStudent := testpkg.CreateTestStudent(t, db, "IsCheckedIn", "Helper", "1f")

		// Create attendance without check-out
		attendanceCheckedIn := studentpresence.Attendance{
			StudentID:   isolatedStudent.ID,
			Date:        date.String(),
			CheckInTime: now.Add(1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendanceCheckedIn, err := repo.RecordAttendance(ctx, attendanceCheckedIn)
		require.NoError(t, err)

		assert.Nil(t, attendanceCheckedIn.CheckOutTime, "Should be checked in when CheckOutTime is nil")

		// Create attendance with check-out
		checkOutTime := now.Add(3 * time.Hour)
		checkedOutBy := data.Staff1.ID
		attendanceCheckedOut := studentpresence.Attendance{
			StudentID:    data.Student2.ID,
			Date:         date.String(),
			CheckInTime:  now.Add(2 * time.Hour), // Different time to avoid conflict
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		attendanceCheckedOut, err = repo.RecordAttendance(ctx, attendanceCheckedOut)
		require.NoError(t, err)

		assert.NotNil(t, attendanceCheckedOut.CheckOutTime, "Should not be checked in when CheckOutTime is set")
	})
}

func TestPresenceAttendance_ListOpenStudentIDsForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	now := time.Now()
	testpkg.CreateTestAttendance(t, db, data.Student1.ID, data.Staff1.ID, data.Device1.ID, now.Add(-30*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	testpkg.CreateTestAttendance(t, db, data.Student2.ID, data.Staff1.ID, data.Device1.ID, now.Add(-30*time.Minute), &checkOutTime)

	ids, err := repo.ListOpenAttendanceStudentIDs(ctx, timezone.TodayDate().String())

	require.NoError(t, err)
	assert.Contains(t, ids, data.Student1.ID)
	assert.NotContains(t, ids, data.Student2.ID)
}

// TestAttendanceRepository_FindByStudentAndDate tests querying attendance records by student and date
func TestPresenceAttendance_StudentDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("single record for student on date", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		attendance := &studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// Find records for this student and date
		records, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: date.String(), UntilDate: date.String()})
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

		checkout1 := now.Add(-90 * time.Minute)
		checkout2 := now.Add(30 * time.Minute)
		closedBy := data.Staff1.ID

		attendance1 := &studentpresence.Attendance{
			StudentID:    multiRowStudent.ID,
			Date:         date.String(),
			CheckInTime:  now.Add(-2 * time.Hour), // Earliest
			CheckOutTime: &checkout1,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendance2 := &studentpresence.Attendance{
			StudentID:    multiRowStudent.ID,
			Date:         date.String(),
			CheckInTime:  now, // Middle
			CheckOutTime: &checkout2,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendance3 := &studentpresence.Attendance{
			StudentID:   multiRowStudent.ID,
			Date:        date.String(),
			CheckInTime: now.Add(1 * time.Hour), // Latest, still open
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2, attendance3} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Find records for this student and date
		records, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{multiRowStudent.ID}, FromDate: date.String(), UntilDate: date.String()})
		require.NoError(t, err)

		require.GreaterOrEqual(t, len(records), 3, "Should find at least three records")

		// Verify ordering by check_in_time ASC (for records we created)
		var ourRecords []studentpresence.Attendance
		for _, r := range records {
			if r.ID == attendance1.ID || r.ID == attendance2.ID || r.ID == attendance3.ID {
				ourRecords = append(ourRecords, r)
			}
		}
		require.Len(t, ourRecords, 3, "Should find all three created records")
		assert.Equal(t, []int64{attendance1.ID, attendance2.ID, attendance3.ID}, []int64{ourRecords[0].ID, ourRecords[1].ID, ourRecords[2].ID}, "oldest check-in first")
	})

	t.Run("no records for student on date", func(t *testing.T) {
		// Use a date with no records
		emptyDate := timezone.NewDate(2023, 1, 1)

		records, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: emptyDate.String(), UntilDate: emptyDate.String()})
		require.NoError(t, err)

		assert.Len(t, records, 0, "Should find no records for date with no attendance")
	})

	t.Run("date filtering ignores time component", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Isolate from prior sub-tests' open rows — partial unique index
		// only allows one open attendance per (student_id, date).
		dateFilterStudent := testpkg.CreateTestStudent(t, db, "DateFilter", "Test", "1h")

		attendance := &studentpresence.Attendance{
			StudentID:   dateFilterStudent.ID,
			Date:        date.String(),
			CheckInTime: now.Add(5 * time.Hour), // Different time
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// timezone.Date carries no time component — querying the same
		// calendar date must find the record regardless of check-in time.
		records, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{dateFilterStudent.ID}, FromDate: date.String(), UntilDate: date.String()})
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

		attendance1 := &studentpresence.Attendance{
			StudentID:   diffStudentA.ID,
			Date:        date.String(),
			CheckInTime: now.Add(6 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance2 := &studentpresence.Attendance{
			StudentID:   diffStudentB.ID,
			Date:        date.String(),
			CheckInTime: now.Add(7 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Query for studentA should only return their records
		records1, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{diffStudentA.ID}, FromDate: date.String(), UntilDate: date.String()})
		require.NoError(t, err)
		for _, r := range records1 {
			assert.Equal(t, diffStudentA.ID, r.StudentID)
		}

		// Query for studentB should only return their records
		records2, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{diffStudentB.ID}, FromDate: date.String(), UntilDate: date.String()})
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

		// Create attendance for date1
		attendance1 := &studentpresence.Attendance{
			StudentID:   twoDayStudent.ID,
			Date:        date1.String(),
			CheckInTime: now.Add(8 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2
		attendance2 := &studentpresence.Attendance{
			StudentID:   twoDayStudent.ID,
			Date:        date2.String(),
			CheckInTime: now.Add(32 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Query for date1 should return records for that day
		records1, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{twoDayStudent.ID}, FromDate: date1.String(), UntilDate: date1.String()})
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
		records2, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{twoDayStudent.ID}, FromDate: date2.String(), UntilDate: date2.String()})
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
func TestPresenceAttendance_FindLatestByStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("latest record across multiple dates", func(t *testing.T) {
		now := time.Now()
		date1 := timezone.TodayDate().AddDays(-2) // 2 days ago
		date2 := timezone.TodayDate().AddDays(-1) // Yesterday
		date3 := timezone.TodayDate()             // Today

		// Different dates each, so the partial unique index isn't relevant
		// here — but use an isolated student for clean assertions.
		latestDateStudent := testpkg.CreateTestStudent(t, db, "LatestDate", "Test", "1l")

		// Create attendance for date1 (oldest)
		attendance1 := &studentpresence.Attendance{
			StudentID:   latestDateStudent.ID,
			Date:        date1.String(),
			CheckInTime: now.Add(-48 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date2 (middle)
		attendance2 := &studentpresence.Attendance{
			StudentID:   latestDateStudent.ID,
			Date:        date2.String(),
			CheckInTime: now.Add(-24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for date3 (latest by date)
		attendance3 := &studentpresence.Attendance{
			StudentID:   latestDateStudent.ID,
			Date:        date3.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2, attendance3} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Find latest record
		latest, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{latestDateStudent.ID}, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, latest, 1)

		// Latest should be attendance3 (today)
		assert.Equal(t, attendance3.ID, latest[0].ID, "Should return the record from the latest date")
	})

	t.Run("latest record same day with multiple check-ins", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Fresh student so we own the row count. Earlier row is closed
		// (realistic check-in/out cycle), the later row stays open — only
		// one open row per (student_id, date) is allowed by the partial
		// unique index.
		multiCheckinStudent := testpkg.CreateTestStudent(t, db, "MultiCheckin", "Same", "1m")

		earlyCheckout := now.Add(-1 * time.Hour)
		closedBy := data.Staff1.ID

		attendance1 := &studentpresence.Attendance{
			StudentID:    multiCheckinStudent.ID,
			Date:         date.String(),
			CheckInTime:  now.Add(-2 * time.Hour), // Earlier, closed
			CheckOutTime: &earlyCheckout,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendance2 := &studentpresence.Attendance{
			StudentID:   multiCheckinStudent.ID,
			Date:        date.String(),
			CheckInTime: now.Add(1 * time.Hour), // Later, open
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Find latest record
		latest, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{multiCheckinStudent.ID}, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, latest, 1)

		assert.Equal(t, attendance2.ID, latest[0].ID, "Should return the record with latest check-in time")
	})

	t.Run("no records for student", func(t *testing.T) {
		// Create a new student with no attendance records
		newStudent := testpkg.CreateTestStudent(t, db, "NoAttendance", "Student", "1c")

		// Try to find latest record for student with no attendance
		latest, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{newStudent.ID}, NewestFirst: true, Limit: 1})

		// An absent stay is an empty history, not a read failure.
		require.NoError(t, err)
		assert.Empty(t, latest, "Should return no rows when no records exist")
	})

	t.Run("single record for student", func(t *testing.T) {
		// Create a new student for isolated test
		singleStudent := testpkg.CreateTestStudent(t, db, "Single", "RecordStudent", "1d")

		now := time.Now()
		date := timezone.TodayDate()

		attendance := &studentpresence.Attendance{
			StudentID:   singleStudent.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// Find latest record
		latest, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{singleStudent.ID}, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, latest, 1)

		assert.Equal(t, attendance.ID, latest[0].ID, "Should return the only record")
		assert.Equal(t, singleStudent.ID, latest[0].StudentID)
	})

	t.Run("complex scenario - mixed dates and times", func(t *testing.T) {
		// Create a new student for isolated test
		complexStudent := testpkg.CreateTestStudent(t, db, "Complex", "ScenarioStudent", "1e")

		now := time.Now()
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)

		// Yesterday: multiple records — earlier is closed, later stays open
		// to respect the partial unique index on
		// (student_id, date) WHERE check_out_time IS NULL.
		yesterdayCheckout := now.Add(-26 * time.Hour)
		closedBy := data.Staff1.ID
		attendanceYesterday1 := &studentpresence.Attendance{
			StudentID:    complexStudent.ID,
			Date:         yesterday.String(),
			CheckInTime:  now.Add(-30 * time.Hour), // Earlier yesterday, closed
			CheckOutTime: &yesterdayCheckout,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}

		attendanceYesterday2 := &studentpresence.Attendance{
			StudentID:   complexStudent.ID,
			Date:        yesterday.String(),
			CheckInTime: now.Add(-25 * time.Hour), // Later yesterday, open
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Today: single record but earlier in the day than latest yesterday record
		attendanceToday := &studentpresence.Attendance{
			StudentID:   complexStudent.ID,
			Date:        today.String(),
			CheckInTime: now.Add(-2 * time.Hour), // Early today
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendanceYesterday1, attendanceYesterday2, attendanceToday} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Find latest record
		latest, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{complexStudent.ID}, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, latest, 1)

		// Should return today's record even though yesterday had later times
		// because date takes precedence over time in the ordering
		assert.Equal(t, attendanceToday.ID, latest[0].ID, "Should return today's record (latest by date)")
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

		attendanceStudent1 := &studentpresence.Attendance{
			StudentID:   noInterfereA.ID,
			Date:        date.String(),
			CheckInTime: now.Add(2 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendanceStudent2 := &studentpresence.Attendance{
			StudentID:   noInterfereB.ID,
			Date:        date.String(),
			CheckInTime: now.Add(3 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendanceStudent1, attendanceStudent2} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Latest for studentA should be their record
		latest1, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{noInterfereA.ID}, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, latest1, 1)
		assert.Equal(t, noInterfereA.ID, latest1[0].StudentID)

		// Latest for studentB should be their record
		latest2, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{noInterfereB.ID}, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, latest2, 1)
		assert.Equal(t, noInterfereB.ID, latest2[0].StudentID)
	})
}

// TestAttendanceRepository_GetStudentCurrentStatus tests getting today's latest attendance record for a student
func TestPresenceAttendance_GetStudentCurrentStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("no records today - student not checked in", func(t *testing.T) {
		// Create a new student with no attendance records
		newStudent := testpkg.CreateTestStudent(t, db, "NoRecords", "Today", "2a")

		// Try to get current status for student with no attendance today
		status, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{newStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})

		// A student without attendance today has an empty history.
		require.NoError(t, err)
		assert.Empty(t, status, "Should return no rows when no records exist for today")
	})

	t.Run("student checked in - latest record has no check-out time", func(t *testing.T) {
		// Create isolated student
		checkedInStudent := testpkg.CreateTestStudent(t, db, "CheckedIn", "StatusTest", "2b")

		now := time.Now()
		today := timezone.TodayDate()

		attendance := &studentpresence.Attendance{
			StudentID:   checkedInStudent.ID,
			Date:        today.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
			// CheckOutTime is nil - student is checked in
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// Get current status
		status, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{checkedInStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, status, 1)

		assert.Equal(t, attendance.ID, status[0].ID)
		assert.Equal(t, checkedInStudent.ID, status[0].StudentID)
		assert.Nil(t, status[0].CheckOutTime, "CheckOutTime should be nil for checked-in student")
		assert.Nil(t, status[0].CheckOutTime, "Student should be checked in")
	})

	t.Run("student checked out - latest record has check-out time", func(t *testing.T) {
		// Create isolated student
		checkedOutStudent := testpkg.CreateTestStudent(t, db, "CheckedOut", "StatusTest", "2c")

		now := time.Now()
		today := timezone.TodayDate()
		checkOutTime := now.Add(2 * time.Hour)
		checkedOutBy := data.Staff2.ID

		attendance := &studentpresence.Attendance{
			StudentID:    checkedOutStudent.ID,
			Date:         today.String(),
			CheckInTime:  now,
			CheckOutTime: &checkOutTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy,
			DeviceID:     data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// Get current status
		status, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{checkedOutStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, status, 1)

		assert.Equal(t, attendance.ID, status[0].ID)
		assert.Equal(t, checkedOutStudent.ID, status[0].StudentID)
		assert.NotNil(t, status[0].CheckOutTime, "CheckOutTime should be set for checked-out student")
		assert.NotNil(t, status[0].CheckOutTime, "Student should not be checked in")
	})

	t.Run("multiple records today - returns latest by check-in time", func(t *testing.T) {
		// Create isolated student
		multiRecordStudent := testpkg.CreateTestStudent(t, db, "MultiRecord", "StatusTest", "2d")

		now := time.Now()
		today := timezone.TodayDate()

		// First check-in (earlier) — closed so it doesn't conflict with the
		// later open row under the partial unique index on
		// (student_id, date) WHERE check_out_time IS NULL.
		earlyCheckout := now.Add(-2*time.Hour - 45*time.Minute)
		earlyClosedBy := data.Staff1.ID
		attendance1 := &studentpresence.Attendance{
			StudentID:    multiRecordStudent.ID,
			Date:         today.String(),
			CheckInTime:  now.Add(-3 * time.Hour),
			CheckOutTime: &earlyCheckout,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &earlyClosedBy,
			DeviceID:     data.Device1.ID,
		}

		// Re-entry, also closed
		checkOutTime1 := now.Add(-2 * time.Hour)
		checkedOutBy1 := data.Staff1.ID
		attendance2 := &studentpresence.Attendance{
			StudentID:    multiRecordStudent.ID,
			Date:         today.String(),
			CheckInTime:  now.Add(-2*time.Hour - 30*time.Minute),
			CheckOutTime: &checkOutTime1,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy1,
			DeviceID:     data.Device1.ID,
		}

		// Second check-in (latest, still open)
		attendance3 := &studentpresence.Attendance{
			StudentID:   multiRecordStudent.ID,
			Date:        today.String(),
			CheckInTime: now.Add(-1 * time.Hour), // Latest check-in time
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2, attendance3} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Get current status - should return the latest check-in
		status, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{multiRecordStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, status, 1)

		assert.Equal(t, attendance3.ID, status[0].ID, "Should return the record with latest check-in time")
		assert.Nil(t, status[0].CheckOutTime, "Latest record should not have check-out time")
		assert.Nil(t, status[0].CheckOutTime, "Student should be checked in from latest record")
	})

	t.Run("historical records exist but none today", func(t *testing.T) {
		// Create isolated student
		historicalStudent := testpkg.CreateTestStudent(t, db, "Historical", "StatusTest", "2e")

		now := time.Now()
		yesterday := timezone.TodayDate().AddDays(-1)

		// Create attendance for yesterday
		attendance := &studentpresence.Attendance{
			StudentID:   historicalStudent.ID,
			Date:        yesterday.String(),
			CheckInTime: now.Add(-24 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// Get current status - should not find yesterday's record
		require.NotZero(t, attendance.ID)
		status, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{historicalStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})

		require.NoError(t, err)
		assert.Empty(t, status, "Should return no rows when only historical records exist")
	})

	t.Run("different students on same day", func(t *testing.T) {
		// Create isolated students
		diffStudent1 := testpkg.CreateTestStudent(t, db, "Different1", "StatusTest", "2f")
		diffStudent2 := testpkg.CreateTestStudent(t, db, "Different2", "StatusTest", "2g")

		now := time.Now()
		today := timezone.TodayDate()

		// Create attendance for student1
		attendance1 := &studentpresence.Attendance{
			StudentID:   diffStudent1.ID,
			Date:        today.String(),
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		// Create attendance for student2 with check-out
		checkOutTime2 := now
		checkedOutBy2 := data.Staff1.ID
		attendance2 := &studentpresence.Attendance{
			StudentID:    diffStudent2.ID,
			Date:         today.String(),
			CheckInTime:  now.Add(-2 * time.Hour),
			CheckOutTime: &checkOutTime2,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &checkedOutBy2,
			DeviceID:     data.Device1.ID,
		}

		for _, att := range []*studentpresence.Attendance{attendance1, attendance2} {
			record, err := repo.RecordAttendance(ctx, *att)
			require.NoError(t, err)
			*att = record

		}

		// Get status for student1 - should be checked in
		status1, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{diffStudent1.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, status1, 1)
		assert.Equal(t, diffStudent1.ID, status1[0].StudentID)
		assert.Nil(t, status1[0].CheckOutTime)
		assert.Nil(t, status1[0].CheckOutTime)

		// Get status for student2 - should be checked out
		status2, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{diffStudent2.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, status2, 1)
		assert.Equal(t, diffStudent2.ID, status2[0].StudentID)
		assert.NotNil(t, status2[0].CheckOutTime)
		assert.NotNil(t, status2[0].CheckOutTime)
	})

	t.Run("timezone handling - today calculation", func(t *testing.T) {
		// Create isolated student
		tzStudent := testpkg.CreateTestStudent(t, db, "Timezone", "StatusTest", "2h")

		today := timezone.TodayDate()

		// Create attendance record for today but late in the day
		attendance := &studentpresence.Attendance{
			StudentID:   tzStudent.ID,
			Date:        today.String(),
			CheckInTime: today.BerlinMidnight().Add(23 * time.Hour), // Late in the day
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		// Get current status - should find the record regardless of time
		status, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{tzStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, status, 1)

		assert.Equal(t, attendance.ID, status[0].ID)
	})
}

// TestAttendanceRepository_Update tests updating attendance records
func TestPresenceAttendance_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("updates attendance with check-out time", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		attendance := studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance, err := repo.RecordAttendance(ctx, attendance)
		require.NoError(t, err)

		// Update with check-out
		checkOutTime := now.Add(3 * time.Hour)
		checkedOutBy := data.Staff2.ID
		attendance.CheckOutTime = &checkOutTime
		attendance.CheckedOutBy = &checkedOutBy

		_, err = repo.ReviseAttendance(ctx, attendance)
		require.NoError(t, err)

		// Verify update
		found, err := repo.FindAttendance(ctx, attendance.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.CheckOutTime)
		assert.NotNil(t, found.CheckedOutBy)
		assert.Equal(t, checkedOutBy, *found.CheckedOutBy)
	})

	t.Run("update with empty attendance should fail", func(t *testing.T) {
		_, err := repo.ReviseAttendance(ctx, studentpresence.Attendance{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid calendar date")
	})
}

// TestAttendanceRepository_FindByID tests finding by ID
func TestPresenceAttendance_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("finds existing attendance by ID", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		attendance := studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance, err := repo.RecordAttendance(ctx, attendance)
		require.NoError(t, err)

		found, err := repo.FindAttendance(ctx, attendance.ID)
		require.NoError(t, err)
		assert.Equal(t, attendance.ID, found.ID)
		assert.Equal(t, data.Student1.ID, found.StudentID)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.FindAttendance(ctx, int64(999999))
		assert.Error(t, err)
	})
}

// TestAttendanceRepository_Delete tests deleting attendance records
func TestPresenceAttendance_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("deletes existing attendance record", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		attendance := studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance, err := repo.RecordAttendance(ctx, attendance)
		require.NoError(t, err)

		err = repo.DeleteAttendance(ctx, attendance.ID)
		require.NoError(t, err)

		_, err = repo.FindAttendance(ctx, attendance.ID)
		assert.Error(t, err)
	})
}

// TestAttendanceRepository_GetTodayByStudentID tests getting today's attendance
func TestPresenceAttendance_GetTodayByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("gets today's attendance for student", func(t *testing.T) {
		now := time.Now()
		today := timezone.TodayDate()

		attendance := &studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        today.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance)
		require.NoError(t, err)
		*attendance = record

		found, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{data.Student1.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		assert.Equal(t, attendance.ID, found[0].ID)
	})

	t.Run("returns error when no attendance today", func(t *testing.T) {
		// Create student with no attendance today
		newStudent := testpkg.CreateTestStudent(t, db, "NoAttendanceToday", "Test", "3a")

		found, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{newStudent.ID}, FromDate: timezone.TodayDate().String(), UntilDate: timezone.TodayDate().String(), NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

// TestAttendanceRepository_FindForDate tests finding all attendance for a date
func TestPresenceAttendance_FindForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("finds all attendance for specific date", func(t *testing.T) {
		now := time.Now()
		date := timezone.TodayDate()

		// Create multiple attendance records for same date
		attendance1 := &studentpresence.Attendance{
			StudentID:   data.Student1.ID,
			Date:        date.String(),
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		attendance2 := &studentpresence.Attendance{
			StudentID:   data.Student2.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		record, err := repo.RecordAttendance(ctx, *attendance1)
		require.NoError(t, err)
		*attendance1 = record

		record, err = repo.RecordAttendance(ctx, *attendance2)
		require.NoError(t, err)
		*attendance2 = record

		// Find all for date
		records, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{FromDate: date.String(), UntilDate: date.String()})
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

		records, err := repo.ListAttendance(ctx, studentpresence.AttendanceFilter{FromDate: emptyDate.String(), UntilDate: emptyDate.String()})
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
func TestPresenceAttendance_CreateIfNoOpenForToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("empty attendance returns error", func(t *testing.T) {
		_, inserted, err := repo.EnsureAttendance(ctx, studentpresence.Attendance{})
		assert.False(t, inserted)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid calendar date")
	})

	t.Run("first open insert succeeds and assigns ID", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Conflict", "First", "1z")

		now := time.Now()
		att := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        timezone.TodayDate().String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}

		att, inserted, err := repo.EnsureAttendance(ctx, att)
		require.NoError(t, err)
		assert.True(t, inserted, "first insert should succeed")
		assert.NotZero(t, att.ID, "row id should be populated")

	})

	t.Run("conflicting open insert returns inserted=false without error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Conflict", "Race", "1y")

		now := time.Now()
		date := timezone.TodayDate()
		first := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        date.String(),
			CheckInTime: now,
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		_, ok1, err1 := repo.EnsureAttendance(ctx, first)
		require.NoError(t, err1)
		require.True(t, ok1)

		second := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        date.String(),
			CheckInTime: now.Add(1 * time.Minute),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		_, ok2, err2 := repo.EnsureAttendance(ctx, second)
		require.NoError(t, err2, "ON CONFLICT must swallow the duplicate, not raise")
		assert.False(t, ok2, "second concurrent open row must report inserted=false")
	})

	t.Run("re-entry after checkout succeeds", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Conflict", "Reentry", "1x")

		now := time.Now()
		date := timezone.TodayDate()
		closedBy := data.Staff1.ID
		closeTime := now.Add(30 * time.Minute)

		closed := studentpresence.Attendance{
			StudentID:    student.ID,
			Date:         date.String(),
			CheckInTime:  now,
			CheckOutTime: &closeTime,
			CheckedInBy:  data.Staff1.ID,
			CheckedOutBy: &closedBy,
			DeviceID:     data.Device1.ID,
		}
		_, ok1, err1 := repo.EnsureAttendance(ctx, closed)
		require.NoError(t, err1)
		require.True(t, ok1)

		// Closed row doesn't occupy the partial index — a new open row is
		// fine on the same calendar day.
		reentry := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        date.String(),
			CheckInTime: now.Add(1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		_, ok2, err2 := repo.EnsureAttendance(ctx, reentry)
		require.NoError(t, err2)
		assert.True(t, ok2, "open insert after a closed row must succeed")

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
func TestPresenceAttendance_CloseOpenForToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	t.Run("closes the open row and clears yard_since", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Close", "Open", "2x")

		now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		yard := now.Add(-15 * time.Minute)
		open := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        timezone.NewDate(2026, 8, 24).String(),
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
			YardSince:   &yard,
		}
		open, err := repo.RecordAttendance(ctx, open)
		require.NoError(t, err)

		closed, err := repo.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, At: now, Date: timezone.DateFromTime(now).String(), StaffID: data.Staff2.ID, DeviceID: 0})
		require.NoError(t, err)
		require.Len(t, closed, 1, "open row must have been closed")
		assert.Equal(t, open.ID, closed[0].ID)
		require.NotNil(t, closed[0].CheckOutTime)
		assert.WithinDuration(t, now, *closed[0].CheckOutTime, time.Second)
		require.NotNil(t, closed[0].CheckedOutBy)
		assert.Equal(t, data.Staff2.ID, *closed[0].CheckedOutBy)
		assert.Nil(t, closed[0].YardSince, "yard sub-state must be cleared on checkout")
	})

	t.Run("no open row returns nil without error", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Close", "Idempotent", "2y")

		closed, err := repo.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, At: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC), Date: timezone.NewDate(2026, 8, 24).String(), StaffID: data.Staff1.ID})
		require.NoError(t, err)
		assert.Empty(t, closed, "no open row → idempotent success, repo returns nil")
	})

	t.Run("device-attributed checkout leaves checked_out_by NULL", func(t *testing.T) {
		// Mirrors the kiosk path where deviceID owns the close but no
		// staff PIN is in scope.
		student := testpkg.CreateTestStudent(t, db, "Close", "NoStaff", "2z")

		now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		open := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        timezone.NewDate(2026, 8, 24).String(),
			CheckInTime: now.Add(-1 * time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		open, err := repo.RecordAttendance(ctx, open)
		require.NoError(t, err)

		closed, err := repo.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, At: now, Date: timezone.DateFromTime(now).String(), StaffID: 0, DeviceID: data.Device1.ID})
		require.NoError(t, err)
		require.Len(t, closed, 1)
		assert.Equal(t, open.ID, closed[0].ID)
		assert.Nil(t, closed[0].CheckedOutBy, "staffID=0 must not write a bogus FK")
		require.NotNil(t, closed[0].CheckedOutDeviceID)
		assert.Equal(t, data.Device1.ID, *closed[0].CheckedOutDeviceID)
	})

	t.Run("rejects checkout device from another tenant", func(t *testing.T) {
		otherTenantID, _ := testpkg.CreateTestTenant(t, db)
		otherDevice := testpkg.CreateTestDeviceForTenant(t, db, otherTenantID, "cross-tenant-checkout-device")
		student := testpkg.CreateTestStudent(t, db, "Close", "CrossTenant", "2w")

		now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		open := studentpresence.Attendance{
			StudentID:   student.ID,
			Date:        timezone.NewDate(2026, 8, 24).String(),
			CheckInTime: now.Add(-time.Hour),
			CheckedInBy: data.Staff1.ID,
			DeviceID:    data.Device1.ID,
		}
		open, err := repo.RecordAttendance(ctx, open)
		require.NoError(t, err)

		closed, err := repo.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, At: now, Date: timezone.DateFromTime(now).String(), StaffID: 0, DeviceID: otherDevice.ID})
		require.Error(t, err)
		assert.Nil(t, closed)
		unchanged, err := repo.FindAttendance(ctx, open.ID)
		require.NoError(t, err)
		assert.Nil(t, unchanged.CheckOutTime, "rejected attribution must leave the stay open")
	})
}

// TestAttendanceRepository_CloseOpenForTodayUsesCallerDate pins that the
// repository closes rows of the CALLER-supplied calendar day instead of
// re-deriving "today" internally (review #2372): the batch school checkout
// snapshots one date for the whole run, and a batch crossing Berlin midnight
// must keep closing the snapshot day's rows — not silently switch to the new
// day mid-batch.
func TestPresenceAttendance_CloseOpenForTodayUsesCallerDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)
	data := createAttendanceTestData(t, db)

	student := testpkg.CreateTestStudent(t, db, "Close", "SnapshotDay", "2w")

	now := time.Now()
	yesterday := timezone.TodayDate().AddDays(-1)
	open := studentpresence.Attendance{
		StudentID:   student.ID,
		Date:        yesterday.String(),
		CheckInTime: now.Add(-25 * time.Hour),
		CheckedInBy: data.Staff1.ID,
		DeviceID:    data.Device1.ID,
	}
	open, err := repo.RecordAttendance(ctx, open)
	require.NoError(t, err)

	// A close scoped to the CURRENT day must not touch yesterday's open row.
	closed, err := repo.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, At: now, Date: timezone.TodayDate().String(), StaffID: data.Staff1.ID, DeviceID: 0})
	require.NoError(t, err)
	assert.Empty(t, closed, "a different-day close must not match the snapshot day's row")

	// The same close scoped to the snapshot day closes exactly that row.
	closed, err = repo.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, At: now, Date: yesterday.String(), StaffID: data.Staff1.ID, DeviceID: 0})
	require.NoError(t, err)
	require.Len(t, closed, 1, "the caller-supplied day's open row must be closed")
	assert.Equal(t, open.ID, closed[0].ID)
}

func newPresence(t *testing.T, db *bun.DB) *studentpresence.Module {
	t.Helper()
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	return module
}
