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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db, func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) })
	ctx := testpkg.Ctx(t)

	t.Run("creates attendance with physical device when device in context", func(t *testing.T) {
		// ARRANGE: Create fixtures using testpkg (proven to work)
		activity := testpkg.CreateTestActivityGroup(t, db, "rfid-checkin-test")
		room := testpkg.CreateTestRoom(t, db, "RFID Checkin Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "RFID", "Checkin", "2a")
		staff := testpkg.CreateTestStaff(t, db, "RFID", "Staff")
		rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-TEST-001")

		// Create context with both staff and device (simulates RFID check-in)
		staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
		deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

		visit := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
		}

		// ACT
		err := service.CreateVisit(deviceCtx, visit)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, visit.ID, "Visit should have been created with an ID")

		// Verify attendance was created with RFID device
		attendance := getAttendanceForStudent(t, db, student.ID, timezone.DateFromTime(visit.EntryTime))
		require.NotNil(t, attendance, "Attendance record should exist")
		assert.Equal(t, rfidDevice.ID, attendance.DeviceID, "Attendance should use RFID device")
		assert.Equal(t, staff.ID, attendance.CheckedInBy, "Attendance should have correct staff ID")
	})
}

func TestCreateVisit_CompletedVisitCreatesClosedAttendance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db, func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) })
	ctx := testpkg.Ctx(t)
	activity := testpkg.CreateTestActivityGroup(t, db, "completed-visit")
	room := testpkg.CreateTestRoom(t, db, "Completed Visit Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Completed", "Visit", "2a")
	staff := testpkg.CreateTestStaff(t, db, "Completed", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-COMPLETED-001")

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)
	entryTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).Add(-2 * time.Hour)
	exitTime := entryTime.Add(time.Hour)
	visit := &activeModels.Visit{
		StudentID: student.ID, ActiveGroupID: activeGroup.ID,
		EntryTime: entryTime, ExitTime: &exitTime,
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	attendance := getAttendanceForStudent(t, db, student.ID, timezone.DateFromTime(visit.EntryTime))
	require.NotNil(t, attendance)
	require.NotNil(t, attendance.CheckOutTime)
	require.NotNil(t, attendance.CheckedOutBy)
	assert.WithinDuration(t, exitTime, *attendance.CheckOutTime, time.Second)
	assert.Equal(t, staff.ID, *attendance.CheckedOutBy)
}

func TestUpdateVisit_ReconcilesMatchingAttendanceSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db)
	ctx := testpkg.Ctx(t)
	activity := testpkg.CreateTestActivityGroup(t, db, "revised-visit")
	room := testpkg.CreateTestRoom(t, db, "Revised Visit Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Revised", "Visit", "2a")
	staff := testpkg.CreateTestStaff(t, db, "Revised", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-REVISED-001")

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)
	entryTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).Add(-2 * time.Hour)
	visit := &activeModels.Visit{StudentID: student.ID, ActiveGroupID: activeGroup.ID, EntryTime: entryTime}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	exitTime := entryTime.Add(time.Hour)
	visit.ExitTime = &exitTime
	require.NoError(t, service.UpdateVisit(deviceCtx, visit))
	attendance := getAttendanceForStudent(t, db, student.ID, timezone.DateFromTime(visit.EntryTime))
	require.NotNil(t, attendance)
	require.NotNil(t, attendance.CheckOutTime)
	assert.WithinDuration(t, exitTime, *attendance.CheckOutTime, time.Second)

	visit.ExitTime = nil
	require.NoError(t, service.UpdateVisit(deviceCtx, visit))
	attendance = getAttendanceForStudent(t, db, student.ID, timezone.DateFromTime(visit.EntryTime))
	require.NotNil(t, attendance)
	assert.Nil(t, attendance.CheckOutTime)
	assert.Nil(t, attendance.CheckedOutBy)
}

func TestUpdateVisit_GroupMoveWithCheckoutClosesAttendanceSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db)
	ctx := testpkg.Ctx(t)
	activity := testpkg.CreateTestActivityGroup(t, db, "moved-visit-checkout")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Moved Visit Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Moved Visit Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, targetRoom.ID)
	student := testpkg.CreateTestStudent(t, db, "Moved", "Visit", "2a")
	staff := testpkg.CreateTestStaff(t, db, "Moved", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-MOVED-VISIT-001")

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)
	entryTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).Add(-2 * time.Hour)
	visit := &activeModels.Visit{
		StudentID: student.ID, ActiveGroupID: sourceGroup.ID, EntryTime: entryTime,
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	exitTime := entryTime.Add(time.Hour)
	visit.ActiveGroupID = targetGroup.ID
	visit.ExitTime = &exitTime
	require.NoError(t, service.UpdateVisit(deviceCtx, visit))

	attendance := getAttendanceForStudent(t, db, student.ID, timezone.DateFromTime(visit.EntryTime))
	require.NotNil(t, attendance)
	require.NotNil(t, attendance.CheckOutTime,
		"a combined group move and checkout must not leave daily attendance open")
	assert.WithinDuration(t, exitTime, *attendance.CheckOutTime, time.Second)
}

// =============================================================================
// Re-entry Tests (Student already has attendance for today)
// =============================================================================

func TestCreateVisit_ReEntry(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("keeps the completed session and creates a new one on re-entry", func(t *testing.T) {
		// ARRANGE: Create fixtures
		activity := testpkg.CreateTestActivityGroup(t, db, "reentry-test")
		room := testpkg.CreateTestRoom(t, db, "Reentry Room")
		activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
		student := testpkg.CreateTestStudent(t, db, "Reentry", "Student", "3a")
		staff := testpkg.CreateTestStaff(t, db, "Reentry", "Staff")
		rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-REENTRY-001")

		// Create existing attendance with checkout time (student left earlier)
		checkoutTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).Add(-2 * time.Hour)
		existingAttendance := createAttendanceWithCheckout(t, db, student.ID, staff.ID, rfidDevice.ID, checkoutTime)

		// Create context with staff and device
		staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
		deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)

		visit := &activeModels.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroup.ID,
			EntryTime:     time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
		}

		// ACT: Student re-enters
		err := service.CreateVisit(deviceCtx, visit)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, visit.ID, "Visit should have been created with an ID")

		// The latest row is a new open session.
		attendance := getAttendanceForStudent(t, db, student.ID, timezone.DateFromTime(visit.EntryTime))
		require.NotNil(t, attendance, "Attendance record should exist")
		assert.NotEqual(t, existingAttendance.ID, attendance.ID)
		assert.Nil(t, attendance.CheckOutTime)

		// The earlier session stays immutable so history does not lose its
		// checkout or count the school-time gap as attendance.
		var completed activeModels.Attendance
		require.NoError(t, db.NewSelect().Model(&completed).ModelTableExpr(`active.attendance`).Where("id = ?", existingAttendance.ID).Scan(context.Background()))
		require.NotNil(t, completed.CheckOutTime)
		assert.WithinDuration(t, checkoutTime, *completed.CheckOutTime, time.Second)
	})
}

// =============================================================================
// Auto-clear status flag Tests (sick / excused on check-in)
// =============================================================================

// TestCreateVisit_AutoClearsSick — with default settings (sick_clear_mode =
// next_checkin), a sick student's flag is cleared when they check in.
func TestCreateVisit_AutoClearsSick(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "autoclear-sick-test")
	room := testpkg.CreateTestRoom(t, db, "Autoclear Sick Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Autoclear", "Sick", "4a")
	staff := testpkg.CreateTestStaff(t, db, "Autoclear", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-ACS-001")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db)
	ctx := testpkg.Ctx(t)

	// Insert a per-tenant override for excused_clear_mode = next_checkin.
	// We write directly to config.setting_values so the test doesn't depend
	// on the full permission-gated SetValue flow (the real write path has
	// its own tests).
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, 'operations.excused_clear_mode', '"next_checkin"', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'operations.excused_clear_mode'`, testpkg.Tenant(t)).Exec(context.Background())
	}()

	activity := testpkg.CreateTestActivityGroup(t, db, "autoclear-exc-test")
	room := testpkg.CreateTestRoom(t, db, "Autoclear Excused Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Autoclear", "Excused", "4c")
	staff := testpkg.CreateTestStaff(t, db, "Autoclear", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-ACE-001")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "keepexc-test")
	room := testpkg.CreateTestRoom(t, db, "Keep Excused Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "KeepExc", "Student", "4b")
	staff := testpkg.CreateTestStaff(t, db, "KeepExc", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-KEX-001")

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

func TestCreateVisit_ClearsPlannedStatusForToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db, func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) })
	repoFactory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "planned-clear-test")
	room := testpkg.CreateTestRoom(t, db, "Planned Clear Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Planned", "Clear", "4d")
	staff := testpkg.CreateTestStaff(t, db, "Planned", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-PCS-001")

	trueVal := true
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	student.Sick = &trueVal
	student.SickSince = &now
	student.Excused = &trueVal
	student.ExcusedSince = &now
	_, err := db.NewUpdate().Model(student).Column("sick", "sick_since", "excused", "excused_since").Where("id = ?", student.ID).Exec(ctx)
	require.NoError(t, err)

	today := timezone.DateFromTime(now)
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &activeModels.StudentStatusDay{
		StudentID:  student.ID,
		Date:       today,
		Status:     activeModels.StudentStatusDaySick,
		ReportedAt: now,
		Source:     activeModels.StudentStatusSourcePlanned,
	}))
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &activeModels.StudentStatusDay{
		StudentID:  student.ID,
		Date:       today,
		Status:     activeModels.StudentStatusDayExcused,
		ReportedAt: now,
		Source:     activeModels.StudentStatusSourcePlanned,
	}))

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)
	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     now,
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	activeRows, err := repoFactory.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, today, today)
	require.NoError(t, err)
	assert.Empty(t, activeRows)

	var reloaded struct {
		Sick         bool       `bun:"sick"`
		SickSince    *time.Time `bun:"sick_since"`
		Excused      bool       `bun:"excused"`
		ExcusedSince *time.Time `bun:"excused_since"`
	}
	err = db.NewSelect().
		Table("users.students").
		Column("sick", "sick_since", "excused", "excused_since").
		Where("id = ?", student.ID).
		Scan(ctx, &reloaded)
	require.NoError(t, err)
	assert.False(t, reloaded.Sick)
	assert.Nil(t, reloaded.SickSince)
	assert.False(t, reloaded.Excused)
	assert.Nil(t, reloaded.ExcusedSince)
}

// TestCreateVisit_ClearsParentStatusForToday covers the parent sick-note
// case: a guardian reported a future day sick (source=parent) without the
// live flag ever being set; once that day arrives and the child checks in,
// the next-checkin clear must drop the still-active parent row — otherwise
// status reads keep treating the child as sick after they showed up.
func TestCreateVisit_ClearsParentStatusForToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupVisitHelperService(t, db, func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) })
	repoFactory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "parent-clear-test")
	room := testpkg.CreateTestRoom(t, db, "Parent Clear Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Parent", "Clear", "4e")
	staff := testpkg.CreateTestStaff(t, db, "Parent", "Staff")
	rfidDevice := testpkg.CreateTestDevice(t, db, "RFID-PRC-001")

	// No live sick flag set — this is the future-reported path the live-flag
	// clear does not cover.
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	today := timezone.DateFromTime(now)
	require.NoError(t, repoFactory.StudentStatusDay.UpsertReported(ctx, &activeModels.StudentStatusDay{
		StudentID:  student.ID,
		Date:       today,
		Status:     activeModels.StudentStatusDaySick,
		ReportedAt: now,
		Source:     activeModels.StudentStatusSourceParent,
	}))

	staffCtx := context.WithValue(ctx, device.CtxStaff, staff)
	deviceCtx := context.WithValue(staffCtx, device.CtxDevice, rfidDevice)
	visit := &activeModels.Visit{
		StudentID:     student.ID,
		ActiveGroupID: activeGroup.ID,
		EntryTime:     now,
	}
	require.NoError(t, service.CreateVisit(deviceCtx, visit))

	activeRows, err := repoFactory.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, today, today)
	require.NoError(t, err)
	assert.Empty(t, activeRows, "parent-sourced sick day must be cleared on check-in")
}

// =============================================================================
// WebManualDeviceCode Constant Test
// =============================================================================

func TestWebManualDeviceCode(t *testing.T) {
	t.Parallel()

	// Verify the constant is set correctly
	assert.Equal(t, "WEB-MANUAL-001", active.WebManualDeviceCode, "WebManualDeviceCode should be 'WEB-MANUAL-001'")
}

// =============================================================================
// Helper Functions
// =============================================================================

func setupVisitHelperService(t *testing.T, db *bun.DB, clocks ...func() time.Time) active.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactoryForTests(repoFactory, db, slog.Default(), clocks...)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.Active
}

func getAttendanceForStudent(t *testing.T, db *bun.DB, studentID int64, date timezone.Date) *activeModels.Attendance {
	t.Helper()

	var attendance activeModels.Attendance
	err := db.NewSelect().
		Model(&attendance).
		ModelTableExpr(`active.attendance`). // NOTE: singular, not plural!
		Where("student_id = ?", studentID).
		Where("date = ?", date).
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
		Date:         timezone.DateFromTime(checkoutTime),
		CheckInTime:  checkoutTime.Add(-4 * time.Hour),
		CheckOutTime: &checkoutTime,
		CheckedInBy:  staffID,
		CheckedOutBy: &checkedOutBy,
		DeviceID:     deviceID,
	}
	attendance.SetTenantID(testpkg.Tenant(t))

	_, err := db.NewInsert().
		Model(attendance).
		ModelTableExpr(`active.attendance`). // NOTE: singular, not plural!
		Exec(context.Background())
	require.NoError(t, err, "Failed to create attendance with checkout")

	return attendance
}
