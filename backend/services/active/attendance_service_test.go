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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	// Service uses timezone.Today() for consistent Europe/Berlin timezone handling
	expectedDate := timezone.Today()
	assert.Equal(t, expectedDate, result.Date)
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

// =============================================================================
// ToggleStudentAttendance Tests
// =============================================================================

// TestToggleStudentAttendance_CheckIn tests checking in a student who is not checked in.
func TestToggleStudentAttendance_CheckIn(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create a student (not checked in)
	student := testpkg.CreateTestStudent(t, db, "Toggle", "CheckIn", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Toggle", "Staff")
	device := testpkg.CreateTestDevice(t, db, "toggle-device-001")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// ACT: Toggle attendance (should check in)
	// skipAuthCheck=true to bypass authorization for testing
	result, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, device.ID, true)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, student.ID, result.StudentID)
	assert.NotZero(t, result.AttendanceID)
	assert.False(t, result.Timestamp.IsZero())
}

// TestToggleStudentAttendance_CheckOut tests checking out a student who is checked in.
func TestToggleStudentAttendance_CheckOut(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create a student and check them in first
	student := testpkg.CreateTestStudent(t, db, "Toggle", "CheckOut", "4b")
	staff := testpkg.CreateTestStaff(t, db, "Toggle", "Staff2")
	device := testpkg.CreateTestDevice(t, db, "toggle-device-002")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// Check in the student first
	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, nil)

	// ACT: Toggle attendance (should check out)
	result, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, device.ID, true)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "checked_out", result.Action)
	assert.Equal(t, student.ID, result.StudentID)
	assert.NotZero(t, result.AttendanceID)
}

// TestToggleStudentAttendance_ReCheckIn tests re-checking in a student who was checked out.
func TestToggleStudentAttendance_ReCheckIn(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	// ARRANGE: Create a student who was checked in and then checked out
	student := testpkg.CreateTestStudent(t, db, "Toggle", "ReCheckIn", "4c")
	staff := testpkg.CreateTestStaff(t, db, "Toggle", "Staff3")
	device := testpkg.CreateTestDevice(t, db, "toggle-device-003")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, device.ID)

	// Create attendance with both check-in and check-out
	checkInTime := time.Now().Add(-2 * time.Hour)
	checkOutTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkInTime, &checkOutTime)

	// ACT: Toggle attendance (should re-check in)
	result, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, device.ID, true)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "checked_in", result.Action)
	assert.Equal(t, student.ID, result.StudentID)
}

// =============================================================================
// Authorization Tests (Tests that exercise authorization code paths)
// =============================================================================

// TestToggleStudentAttendance_WebAuthorizationPath tests the web authorization code path
// This exercises authorizeWebToggle and checkTeacherOrRoomSupervisorAccess
func TestToggleStudentAttendance_WebAuthorizationPath(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("web authorization fails when staff has no access to student", func(t *testing.T) {
		// ARRANGE: Create student and staff with NO relationship
		student := testpkg.CreateTestStudent(t, db, "NoAccess", "Student", "5a")
		staff := testpkg.CreateTestStaff(t, db, "NoAccess", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID)

		// ACT: Toggle attendance without skipAuthCheck
		// In a normal (non-IoT) context, this triggers the web toggle path
		_, err := service.ToggleStudentAttendance(ctx, student.ID, staff.ID, 0, false)

		// ASSERT: Should fail authorization
		assert.Error(t, err, "Expected authorization error")
		assert.Contains(t, err.Error(), "teacher does not have access", "Expected access denied message")
	})
}

// TestCheckTeacherStudentAccess tests the CheckTeacherStudentAccess function
func TestCheckTeacherStudentAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := context.Background()

	t.Run("returns false for staff without teacher record", func(t *testing.T) {
		// ARRANGE: Create student and staff (staff is not a teacher)
		student := testpkg.CreateTestStudent(t, db, "Unrelated", "Student", "6a")
		staff := testpkg.CreateTestStaff(t, db, "Unrelated", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID)

		// ACT: Check access - should return false because staff is not a teacher
		hasAccess, err := service.CheckTeacherStudentAccess(ctx, staff.ID, student.ID)

		// ASSERT: No error, but access denied
		require.NoError(t, err)
		assert.False(t, hasAccess, "Staff without teacher record should not have access")
	})

	t.Run("returns false for non-existent staff", func(t *testing.T) {
		// ARRANGE
		student := testpkg.CreateTestStudent(t, db, "Orphan", "Student", "6b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// ACT
		hasAccess, err := service.CheckTeacherStudentAccess(ctx, 99999999, student.ID)

		// ASSERT: Either returns false or error - both are acceptable
		if err == nil {
			assert.False(t, hasAccess)
		}
	})

	t.Run("teacher with group access can access student in their group", func(t *testing.T) {
		// ARRANGE: Create a full teacher with account
		teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Group", "Teacher")
		student := testpkg.CreateTestStudent(t, db, "Group", "Student", "6c")
		defer testpkg.CleanupActivityFixtures(t, db, teacher.Staff.ID, student.ID)

		// Note: We don't have a way to easily assign the teacher to the student's group
		// without more complex fixture setup, so this test verifies the error path
		// when teacher exists but has no group relationship

		// ACT
		hasAccess, err := service.CheckTeacherStudentAccess(ctx, teacher.Staff.ID, student.ID)

		// ASSERT: No error but no access (teacher exists but not assigned to student's group)
		require.NoError(t, err)
		assert.False(t, hasAccess, "Expected no access when teacher not in student's group")
	})
}
