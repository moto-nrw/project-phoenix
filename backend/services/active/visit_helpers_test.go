// Package active_test tests the visit helper functions in active service layer.
package active_test

import (
	"context"
	"log/slog"
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
	ctx := testpkg.TenantContext(1)

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
	ctx := testpkg.TenantContext(1)

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
// Auto-clear status flag Tests (sick / excused on check-in)
// =============================================================================

// TestCreateVisit_AutoClearsSick — with default settings (sick_clear_mode =
// next_checkin), a sick student's flag is cleared when they check in.
func TestCreateVisit_AutoClearsSick(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupVisitHelperService(t, db)
	ctx := testpkg.TenantContext(1)

	activity := testpkg.CreateTestActivityGroup(t, db, "autoclear-sick-test")
	room := testpkg.CreateTestRoom(t, db, "Autoclear Sick Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Autoclear", "Sick", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Autoclear", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-ACS-001")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, rfidDevice.ID)

	// Pre-state: mark student sick
	sickTrue := true
	now := time.Now()
	student.Sick = &sickTrue
	student.SickSince = &now
	_, err := db.NewUpdate().Model(student).Column("sick", "sick_since").Where("id = ?", student.ID).Exec(ctx)
	require.NoError(t, err)

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	// Re-read student — sick flag should be cleared.
	var reloaded struct {
		Sick      bool       `bun:"sick"`
		SickSince *time.Time `bun:"sick_since"`
	}
	err = db.NewSelect().
		Table("users.students").
		Column("sick", "sick_since").
		Where("id = ?", student.ID).
		Scan(ctx, &reloaded)
	require.NoError(t, err)
	assert.False(t, reloaded.Sick, "sick should be cleared on check-in under next_checkin mode")
	assert.Nil(t, reloaded.SickSince, "sick_since should be nil after clearing")
}

// TestCreateVisit_AutoClearsExcused_WhenSettingNextCheckin — when the tenant
// overrides operations.excused_clear_mode to "next_checkin", an excused
// student gets the flag cleared on check-in (same behavior path as sick).
func TestCreateVisit_AutoClearsExcused_WhenSettingNextCheckin(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupVisitHelperService(t, db)
	ctx := testpkg.TenantContext(1)

	// Insert a per-tenant override for excused_clear_mode = next_checkin.
	// We write directly to config.setting_values so the test doesn't depend
	// on the full permission-gated SetValue flow (the real write path has
	// its own tests).
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (1, 'operations.excused_clear_mode', '"next_checkin"', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = 1 AND setting_key = 'operations.excused_clear_mode'`).Exec(context.Background())
	}()

	activity := testpkg.CreateTestActivityGroup(t, db, "autoclear-exc-test")
	room := testpkg.CreateTestRoom(t, db, "Autoclear Excused Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Autoclear", "Excused", "4c")
	staff := testpkg.CreateTestStaff(t, db, "Autoclear", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-ACE-001")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, rfidDevice.ID)

	excusedTrue := true
	now := time.Now()
	student.Excused = &excusedTrue
	student.ExcusedSince = &now
	_, err = db.NewUpdate().Model(student).Column("excused", "excused_since").Where("id = ?", student.ID).Exec(ctx)
	require.NoError(t, err)

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	var reloaded struct {
		Excused      bool       `bun:"excused"`
		ExcusedSince *time.Time `bun:"excused_since"`
	}
	err = db.NewSelect().
		Table("users.students").
		Column("excused", "excused_since").
		Where("id = ?", student.ID).
		Scan(ctx, &reloaded)
	require.NoError(t, err)
	assert.False(t, reloaded.Excused, "excused must be cleared when clear_mode override is next_checkin")
	assert.Nil(t, reloaded.ExcusedSince)
}

// TestCreateVisit_DoesNotClearExcused_WhenDefaultMode — excused default is
// end_of_day, so check-in must NOT clear the flag.
func TestCreateVisit_DoesNotClearExcused_WhenDefaultMode(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupVisitHelperService(t, db)
	ctx := testpkg.TenantContext(1)

	activity := testpkg.CreateTestActivityGroup(t, db, "keepexc-test")
	room := testpkg.CreateTestRoom(t, db, "Keep Excused Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "KeepExc", "Student", "4b")
	staff := testpkg.CreateTestStaff(t, db, "KeepExc", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-KEX-001")
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, activeGroup.ID, student.ID, staff.ID, rfidDevice.ID)

	excusedTrue := true
	now := time.Now()
	student.Excused = &excusedTrue
	student.ExcusedSince = &now
	_, err := db.NewUpdate().Model(student).Column("excused", "excused_since").Where("id = ?", student.ID).Exec(ctx)
	require.NoError(t, err)

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     time.Now(),
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	var reloaded struct {
		Excused      bool       `bun:"excused"`
		ExcusedSince *time.Time `bun:"excused_since"`
	}
	err = db.NewSelect().
		Table("users.students").
		Column("excused", "excused_since").
		Where("id = ?", student.ID).
		Scan(ctx, &reloaded)
	require.NoError(t, err)
	assert.True(t, reloaded.Excused, "excused should remain set when clear_mode is end_of_day (default)")
	assert.NotNil(t, reloaded.ExcusedSince)
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
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
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
		Where("date = ?", timezone.TodayUTC()).
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
		Date:         timezone.TodayUTC(),
		CheckInTime:  time.Now().Add(-4 * time.Hour),
		CheckOutTime: &checkoutTime,
		CheckedInBy:  staffID,
		CheckedOutBy: &checkedOutBy,
		DeviceID:     deviceID,
	}
	attendance.SetTenantID(1)

	_, err := db.NewInsert().
		Model(attendance).
		ModelTableExpr(`active.attendance`). // NOTE: singular, not plural!
		Exec(context.Background())
	require.NoError(t, err, "Failed to create attendance with checkout")

	return attendance
}
