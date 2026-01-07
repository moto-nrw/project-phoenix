// Package active_test tests the attendance service using the hermetic testing pattern.
//
// # HERMETIC TEST PATTERN
//
// Tests create their own fixtures, execute operations, and clean up afterward.
// This approach eliminates dependencies on seed data and prevents test pollution.
//
// STRUCTURE: ARRANGE-ACT-ASSERT
//
//	ARRANGE: Create test fixtures (real database records)
//	  student := testpkg.CreateTestStudent(t, db, "First", "Last", "1a")
//	  defer testpkg.CleanupActivityFixtures(t, db, student.ID)
//
//	ACT: Perform the operation under test
//	  result, err := service.GetStudentAttendanceStatus(ctx, student.ID)
//
//	ASSERT: Verify the results
//	  require.NoError(t, err)
//	  assert.Equal(t, "not_checked_in", result.Status)
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: setupTestDB and setupActiveService are defined in session_conflict_test.go
// and are reused here since both files are in package active_test.
//
// Attendance fixtures are provided by testpkg:
// - testpkg.CreateTestAttendance(t, db, studentID, staffID, deviceID, checkInTime, checkOutTime)
// - testpkg.CleanupActivityFixtures automatically cleans up attendance records by student_id

// =============================================================================
// Model Tests (No Database Required)
// =============================================================================

// TestAttendance_IsCheckedIn tests the IsCheckedIn helper method on the Attendance model.
// This is a pure model test - it doesn't need a database connection.
func TestAttendance_IsCheckedIn(t *testing.T) {
	tests := []struct {
		name           string
		checkOutTime   *time.Time
		expectedResult bool
	}{
		{
			name:           "Student is checked in (no checkout time)",
			checkOutTime:   nil,
			expectedResult: true,
		},
		{
			name:           "Student is checked out (has checkout time)",
			checkOutTime:   func() *time.Time { t := time.Now(); return &t }(),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attendance := &active.Attendance{
				CheckOutTime: tt.checkOutTime,
			}

			result := attendance.IsCheckedIn()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// =============================================================================
// Service Integration Tests (Hermetic Pattern with Real Database)
// =============================================================================

// TestGetStudentAttendanceStatus_NotCheckedIn tests the scenario where a student
// has no attendance record for today (not checked in).
func TestGetStudentAttendanceStatus_NotCheckedIn(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create a student (but NO attendance record)
	student := testpkg.CreateTestStudent(t, db, "NotCheckedIn", "Student", "2a")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	// ACT: Get attendance status for student without check-in
	result, err := service.GetStudentAttendanceStatus(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, student.ID, result.StudentID)
	assert.Equal(t, "not_checked_in", result.Status)
	assert.Equal(t, time.Now().UTC().Truncate(24*time.Hour), result.Date.UTC().Truncate(24*time.Hour))
	assert.Nil(t, result.CheckInTime)
	assert.Nil(t, result.CheckOutTime)
	assert.Empty(t, result.CheckedInBy)
	assert.Empty(t, result.CheckedOutBy)
}

// TestGetStudentAttendanceStatus_CheckedIn tests the scenario where a student
// has checked in today (active attendance record).
func TestGetStudentAttendanceStatus_CheckedIn(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures
	student := testpkg.CreateTestStudent(t, db, "CheckedIn", "Student", "2b")
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff")
	device := testpkg.CreateTestDevice(t, db, "attendance-device-001")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// Create an attendance record (checked in, not checked out)
	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, nil)

	// ACT: Get attendance status
	result, err := service.GetStudentAttendanceStatus(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, student.ID, result.StudentID)
	assert.Equal(t, "checked_in", result.Status)
	assert.NotNil(t, result.CheckInTime)
	assert.Nil(t, result.CheckOutTime)
}

// TestGetStudentAttendanceStatus_CheckedOut tests the scenario where a student
// has checked in and then checked out.
func TestGetStudentAttendanceStatus_CheckedOut(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create fixtures
	student := testpkg.CreateTestStudent(t, db, "CheckedOut", "Student", "2c")
	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Staff2")
	device := testpkg.CreateTestDevice(t, db, "attendance-device-002")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// Create an attendance record with check-out time
	checkInTime := time.Now().Add(-2 * time.Hour)
	checkOutTime := time.Now().Add(-30 * time.Minute)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, &checkOutTime)

	// ACT: Get attendance status
	result, err := service.GetStudentAttendanceStatus(ctx, student.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, student.ID, result.StudentID)
	assert.Equal(t, "checked_out", result.Status)
	assert.NotNil(t, result.CheckInTime)
	assert.NotNil(t, result.CheckOutTime)
}

// TestGetStudentsAttendanceStatuses tests batch retrieval of attendance statuses
// for multiple students with mixed states.
func TestGetStudentsAttendanceStatuses(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create three students with different attendance states
	studentNotCheckedIn := testpkg.CreateTestStudent(t, db, "NotIn", "Student1", "3a")
	studentCheckedIn := testpkg.CreateTestStudent(t, db, "CheckedIn", "Student2", "3a")
	studentCheckedOut := testpkg.CreateTestStudent(t, db, "CheckedOut", "Student3", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Multi", "Supervisor")
	device := testpkg.CreateTestDevice(t, db, "attendance-device-003")
	defer testpkg.CleanupActivityFixtures(t, db,
		studentNotCheckedIn.ID, studentCheckedIn.ID, studentCheckedOut.ID,
		staff.ID, device.ID)

	// Create attendance records:
	// - studentNotCheckedIn: no record
	// - studentCheckedIn: checked in, not checked out
	// - studentCheckedOut: checked in and checked out
	checkInTime := time.Now().Add(-2 * time.Hour)
	checkOutTime := time.Now().Add(-30 * time.Minute)

	testpkg.CreateTestAttendance(t, db, studentCheckedIn.ID, staff.ID, device.ID, checkInTime, nil)
	testpkg.CreateTestAttendance(t, db, studentCheckedOut.ID, staff.ID, device.ID, checkInTime, &checkOutTime)

	// ACT: Get statuses for all three students
	studentIDs := []int64{studentNotCheckedIn.ID, studentCheckedIn.ID, studentCheckedOut.ID}
	statuses, err := service.GetStudentsAttendanceStatuses(ctx, studentIDs)

	// ASSERT
	require.NoError(t, err)
	require.Len(t, statuses, 3)

	// Verify student not checked in
	notCheckedInStatus := statuses[studentNotCheckedIn.ID]
	require.NotNil(t, notCheckedInStatus, "Expected status for studentNotCheckedIn")
	assert.Equal(t, "not_checked_in", notCheckedInStatus.Status)
	assert.Nil(t, notCheckedInStatus.CheckInTime)
	assert.Nil(t, notCheckedInStatus.CheckOutTime)

	// Verify student checked in
	checkedInStatus := statuses[studentCheckedIn.ID]
	require.NotNil(t, checkedInStatus, "Expected status for studentCheckedIn")
	assert.Equal(t, "checked_in", checkedInStatus.Status)
	assert.NotNil(t, checkedInStatus.CheckInTime)
	assert.Nil(t, checkedInStatus.CheckOutTime)

	// Verify student checked out
	checkedOutStatus := statuses[studentCheckedOut.ID]
	require.NotNil(t, checkedOutStatus, "Expected status for studentCheckedOut")
	assert.Equal(t, "checked_out", checkedOutStatus.Status)
	assert.NotNil(t, checkedOutStatus.CheckInTime)
	assert.NotNil(t, checkedOutStatus.CheckOutTime)
}

// TestGetStudentsAttendanceStatuses_EmptyInput tests that empty input returns
// an empty result without error.
func TestGetStudentsAttendanceStatuses_EmptyInput(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ACT: Get statuses with empty input
	statuses, err := service.GetStudentsAttendanceStatuses(ctx, []int64{})

	// ASSERT
	require.NoError(t, err)
	assert.Empty(t, statuses)
}
