// Package active_test tests the visit helper functions in active service layer.
package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/services"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// =============================================================================
// CreateVisit with Device Tests
// =============================================================================

func TestCreateVisit_WithDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupVisitHelperService(t, db)
	ctx := context.Background()

	t.Run("creates attendance with physical device when device in context", func(t *testing.T) {
		// ARRANGE: Create fixtures using testpkg (proven to work)
		activity := testpkg.CreateTestActivityGroup(t, db, "rfid-checkin-test")
		room := testpkg.CreateTestRoom(t, db, "RFID Checkin Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "RFID", "Checkin", "2a")
		staff := testpkg.CreateTestStaff(t, db, "RFID", "Staff")
		rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-TEST-001")

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, rfidDevice.ID)

		// Create context with both staff and device (simulates RFID check-in)
		staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
		deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

		visit := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Now(),
		}

		// ACT
		err := service.CreateVisit(deviceCtx, visit)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, visit.ID, "Visit should have been created with an ID")

		// Verify attendance was created with RFID device
		attendance := getAttendanceForStudent(t, db, student.ID)
		require.NotNil(t, attendance, "Attendance record should exist")
		assert.Equal(t, rfidDevice.ID, attendance.DeviceID, "Attendance should use RFID device")
		assert.Equal(t, staff.ID, attendance.CheckedInBy, "Attendance should have correct staff ID")
	})
}

// =============================================================================
// Re-entry Tests (Student already has attendance for today)
// =============================================================================

func TestCreateVisit_ReEntry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupVisitHelperService(t, db)
	ctx := context.Background()

	t.Run("clears checkout time on re-entry", func(t *testing.T) {
		// ARRANGE: Create fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "reentry-test")
		room := testpkg.CreateTestRoom(t, db, "Reentry Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Reentry", "Student", "3a")
		staff := testpkg.CreateTestStaff(t, db, "Reentry", "Staff")
		rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-REENTRY-001")

		// Create existing attendance with checkout time (student left earlier)
		checkoutTime := time.Now().Add(-2 * time.Hour)
		existingAttendance := createAttendanceWithCheckout(t, db, student.ID, staff.ID, rfidDevice.ID, checkoutTime)

		defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, rfidDevice.ID, existingAttendance.ID)

		// Create context with staff and device
		staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
		deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

		visit := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Now(),
		}

		// ACT: Student re-enters
		err := service.CreateVisit(deviceCtx, visit)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, visit.ID, "Visit should have been created with an ID")

		// Verify checkout time was cleared
		attendance := getAttendanceForStudent(t, db, student.ID)
		require.NotNil(t, attendance, "Attendance record should exist")
		assert.Nil(t, attendance.CheckOutTime, "Checkout time should be cleared on re-entry")
	})
}

// =============================================================================
// WebManualDeviceCode Constant Test
// =============================================================================

func TestWebManualDeviceCode(t *testing.T) {
	// Verify the constant is set correctly
	assert.Equal(t, "WEB-MANUAL-001", active.WebManualDeviceCode, "WebManualDeviceCode should be 'WEB-MANUAL-001'")
}

// =============================================================================
// Helper Functions
// =============================================================================

func setupVisitHelperService(t *testing.T, db *bun.DB) active.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

func getAttendanceForStudent(t *testing.T, db *bun.DB, studentID int64) *activeModels.Attendance {
	t.Helper()

	var attendance activeModels.Attendance
	err := db.NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance`). // NOTE: singular, not plural!
		Where("student_id = ?", studentID).
		Where("date = CURRENT_DATE").
		Order("check_in_time DESC").
		Limit(1).
		Scan(context.Background())

	if err != nil {
		return nil
	}
	return &attendance
}

func createAttendanceWithCheckout(t *testing.T, db *bun.DB, studentID, staffID, deviceID int64, checkoutTime time.Time) *activeModels.Attendance {
	t.Helper()

	checkedOutBy := staffID
	attendance := &activeModels.Attendance{
		StudentID:    studentID,
		Date:         timezone.Today(),
		CheckInTime:  time.Now().Add(-4 * time.Hour),
		CheckOutTime: &checkoutTime,
		CheckedInBy:  staffID,
		CheckedOutBy: &checkedOutBy,
		DeviceID:     deviceID,
	}

	_, err := db.NewInsert().
		Model(attendance).
		ModelTableExpr(`active.attendance`). // NOTE: singular, not plural!
		Exec(context.Background())
	require.NoError(t, err, "Failed to create attendance with checkout")

	return attendance
}
