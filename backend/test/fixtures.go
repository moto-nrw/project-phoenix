package test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/enrollment"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/feedback"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// SQL constants to avoid duplication
const (
	whereIDEquals                 = "id = ?"
	whereIDIn                     = "id IN (?)"
	whereIDOrAccountID            = "id = ? OR account_id = ?"
	whereAccountIDIn              = "account_id IN (?)"
	whereTenantIDIn               = "tenant_id IN (?)"
	tableUsersTeachers            = "users.teachers"
	tableUsersStaff               = "users.staff"
	tableUsersPersons             = "users.persons"
	tableActiveVisits             = "active.visits"
	tableUsersRFIDCards           = "users.rfid_cards"
	tableEducationGradeTransition = "education.grade_transitions"
	testEmailFormat               = "%s-%d@test.local"
)

// cleanupDelete executes a delete query and logs any errors.
// This provides visibility into cleanup failures without causing test failures.
func cleanupDelete(tb testing.TB, query *bun.DeleteQuery, table string) {
	_, err := query.Exec(context.Background())
	if err != nil {
		tb.Logf("cleanup %s: %v", table, err)
	}
}

func withCleanupTenant(query *bun.DeleteQuery, tenantID int64) *bun.DeleteQuery {
	if tenantID <= 0 {
		return query
	}
	return query.Where("tenant_id = ?", tenantID)
}

func deleteStaffCaregiverBindings(
	tb testing.TB,
	db *bun.DB,
	staffID int64,
	teacherID int64,
	tenantID int64,
) {
	tb.Helper()

	if staffID <= 0 {
		return
	}

	cleanupDelete(tb, withCleanupTenant(db.NewDelete().
		TableExpr("active.group_supervisors").
		Where("staff_id = ?", staffID), tenantID),
		"active.group_supervisors")

	cleanupDelete(tb, withCleanupTenant(db.NewDelete().
		TableExpr("activities.supervisors").
		Where("staff_id = ?", staffID), tenantID),
		"activities.supervisors")

	cleanupDelete(tb, withCleanupTenant(db.NewDelete().
		TableExpr("education.group_substitution").
		Where("regular_staff_id = ? OR substitute_staff_id = ?", staffID, staffID), tenantID),
		"education.group_substitution")

	groupTeacherDelete := db.NewDelete().TableExpr("education.group_teacher")
	if teacherID > 0 {
		groupTeacherDelete = groupTeacherDelete.Where("teacher_id = ?", teacherID)
	} else if tenantID > 0 {
		groupTeacherDelete = groupTeacherDelete.Where(
			"teacher_id IN (SELECT id FROM users.teachers WHERE staff_id = ? AND tenant_id = ?)",
			staffID,
			tenantID,
		)
	} else {
		groupTeacherDelete = groupTeacherDelete.Where(
			"teacher_id IN (SELECT id FROM users.teachers WHERE staff_id = ?)",
			staffID,
		)
	}
	cleanupDelete(tb, withCleanupTenant(groupTeacherDelete, tenantID), "education.group_teacher")
}

// Fixture helpers for hermetic testing. Each helper creates a real database record
// with proper relationships and returns the created entity with its real ID.
// Tests should call these to create test data, then defer cleanup.

// CreateTestActivityCategory creates a real activity category in the database
func CreateTestActivityCategory(tb testing.TB, db *bun.DB, name string) *activities.Category {
	tb.Helper()

	// Make name unique to avoid conflicts with seeded data
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	category := &activities.Category{
		Name:  uniqueName,
		Color: "#CCCCCC",
	}
	category.SetTenantID(fixtureTenantID(tb))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := db.NewInsert().
		Model(category).
		ModelTableExpr(`activities.categories`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity category")

	return category
}

// CreateTestActivityGroup creates a real activity group in the database
// Activity groups require a category and creator, so this helper creates them automatically
func CreateTestActivityGroup(tb testing.TB, db *bun.DB, name string) *activities.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First create a category (activities.groups.category_id is required)
	category := CreateTestActivityCategory(tb, db, fmt.Sprintf("Category-%s-%d", name, time.Now().UnixNano()))

	// Create a staff member as the creator (activities.groups.created_by is required)
	staff := CreateTestStaff(tb, db, "Creator", name)

	// Create the activity group
	group := &activities.Group{
		Name:            name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
	}
	group.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity group")

	return group
}

// CreateTestRoom creates a real room in the database
func CreateTestRoom(tb testing.TB, db *bun.DB, name string) *facilities.Room {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make room name unique by appending timestamp
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	room := &facilities.Room{
		Name:     uniqueName,
		Building: "Test Building",
		Capacity: IntPtr(30),
	}
	room.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test room")

	return room
}

// CreateTestDevice creates a real IoT device in the database
func CreateTestDevice(tb testing.TB, db *bun.DB, deviceID string) *iot.Device {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make device ID unique by appending timestamp if needed
	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, time.Now().UnixNano())

	device := &iot.Device{
		DeviceID:   uniqueDeviceID,
		DeviceType: "terminal",
		Name:       StrPtr("Test Device"),
		Status:     iot.DeviceStatusActive,
		APIKey:     StrPtr("test-api-key-" + uniqueDeviceID),
	}
	device.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(device).
		ModelTableExpr(`iot.devices`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test device")

	return device
}

// EnsureWebManualDevice creates or returns the web manual device needed for
// manual check-ins. Every real school is provisioned with it
// (services/platform.createWebManualDevice); web check-ins fall back to it when
// no physical device is involved, and without it the attendance insert fails on
// fk_attendance_device_tenant.
//
// Deliberately NOT part of EnsureTestTenant: that would put a device row in
// every test tenant, and the dozens of tests that delete their school with raw
// SQL would then trip devices_tenant_id_fkey. Tests that exercise a web
// check-in ask for it explicitly (#2419).
func EnsureWebManualDevice(tb testing.TB, db *bun.DB) *iot.Device {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use upsert pattern to avoid race conditions
	device := &iot.Device{
		DeviceID:   iot.WebManualDeviceID,
		DeviceType: iot.DeviceTypeVirtual,
		Name:       StrPtr("Web-Portal (Manuell)"),
		Status:     iot.DeviceStatusActive,
	}
	device.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(device).
		ModelTableExpr(`iot.devices`).
		On("CONFLICT (tenant_id, device_id) WHERE archived_at IS NULL DO NOTHING").
		Exec(ctx)
	require.NoError(tb, err, "Failed to ensure web manual device")

	// Fetch the device (either just created or existing)
	var existingDevice iot.Device
	err = db.NewSelect().
		Model(&existingDevice).
		ModelTableExpr(`iot.devices AS "device"`).
		Where(`"device".device_id = ?`, iot.WebManualDeviceID).
		Where(`"device".tenant_id = ?`, device.TenantID).
		Where(`"device".archived_at IS NULL`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to fetch web manual device")

	return &existingDevice
}

// CreateTestPerson creates a real person in the database (required for staff creation)
func CreateTestPerson(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
	}
	person.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person")

	return person
}

// CreateTestStaff creates a real staff member in the database
// This requires a person, so it creates one automatically
func CreateTestStaff(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first
	person := CreateTestPerson(tb, db, firstName, lastName)

	// Create staff record
	staff := &users.Staff{
		PersonID: person.ID,
	}
	staff.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff")

	// Store person reference for convenience
	staff.Person = person

	return staff
}

// CreateTestStaffForPerson creates a staff record for an existing person
// Use this when you need to control the person record separately
func CreateTestStaffForPerson(tb testing.TB, db *bun.DB, personID int64) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	staff := &users.Staff{
		PersonID: personID,
	}
	staff.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff for person")

	return staff
}

// CreateTestStudent creates a real student in the database
// This requires a person, so it creates one automatically
func CreateTestStudent(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string) *users.Student {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first (Student has FK to Person)
	person := CreateTestPerson(tb, db, firstName, lastName)

	// Create student record
	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: schoolClass,
	}
	student.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student")

	return student
}

// CreateTestAttendance creates a real attendance record in the database
// This requires a student, staff, and device to already exist
//
// Note: The Date field is set to today's local date (not derived from checkInTime)
// to match the repository's GetStudentCurrentStatus query which always queries
// for today's date using local timezone. This ensures tests work correctly
// regardless of when they run (e.g., 00:40 CET is still the same calendar day locally).
func CreateTestAttendance(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, checkInTime time.Time, checkOutTime *time.Time) *active.Attendance {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	today := timezone.TodayDate()

	attendance := &active.Attendance{
		StudentID:    studentID,
		Date:         today,
		CheckInTime:  checkInTime,
		CheckOutTime: checkOutTime,
		CheckedInBy:  staffID,
		DeviceID:     deviceID,
	}
	attendance.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(attendance).
		ModelTableExpr(`active.attendance`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test attendance record")

	return attendance
}

// CleanupActivityFixtures removes activity-related and education-related test fixtures from the database.
// Pass activity group IDs, device IDs, room IDs, education group IDs, teacher IDs, or any combination.
// This is typically called in a defer statement to ensure cleanup happens.
//
// Implicit tenant scope: deletes are confined to the default test tenant (id=1).
// Tests that build fixtures in a different tenant must call
// CleanupActivityFixturesForTenant instead.
//
// Example:
//
//	activity := CreateTestActivityGroup(t, db, "Test")
//	device := CreateTestDevice(t, db, "dev-001")
//	room := CreateTestRoom(t, db, "Room 1")
//	defer CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID)
func CleanupActivityFixtures(tb testing.TB, db *bun.DB, ids ...int64) {
	tb.Helper()
	CleanupActivityFixturesForTenant(tb, db, fixtureTenantID(tb), ids...)
}

// CleanupActivityFixturesForTenant is the tenant-scoped variant of
// CleanupActivityFixtures. Every DELETE is constrained to the supplied tenant_id,
// so a test running in an isolated tenant cannot have its rows clobbered by a
// concurrent test package whose call to CleanupActivityFixtures (tenant 1)
// happens to pass an int64 that numerically matches one of this test's
// auto-incremented fixture IDs.
//
// Use this together with the *ForTenant fixture builders when a test must run
// in a non-default tenant for isolation reasons.
func CleanupActivityFixturesForTenant(tb testing.TB, db *bun.DB, tenantID int64, ids ...int64) {
	tb.Helper()

	if len(ids) == 0 {
		return
	}

	// Batch delete all fixtures matching the IDs.
	// Each DELETE is paired with `tenant_id = ?` so cross-package raw-id
	// collisions (e.g. tenant 1's staff_id == tenant 99001's active_group_id)
	// cannot delete the wrong tenant's data.

	for _, id := range ids {
		// Try to delete from each table type
		// Errors are logged but don't fail tests since we don't know which table each ID belongs to

		// ========================================
		// Education domain cleanup (FK-dependent order)
		// ========================================

		// Delete from education.group_teacher (depends on group and teacher)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("education.group_teacher").
			Where("group_id = ? OR teacher_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"education.group_teacher")

		// Delete from education.group_substitution (depends on group and staff)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("education.group_substitution").
			Where("group_id = ? OR regular_staff_id = ? OR substitute_staff_id = ?", id, id, id).
			Where("tenant_id = ?", tenantID),
			"education.group_substitution")

		// Delete from users.teachers (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersTeachers).
			Where("id = ? OR staff_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			tableUsersTeachers)

		// Delete from education.groups
		cleanupDelete(tb, db.NewDelete().
			TableExpr("education.groups").
			Where(whereIDEquals, id).
			Where("tenant_id = ?", tenantID),
			"education.groups")

		// ========================================
		// Active domain cleanup
		// ========================================

		// Delete from active.visits by direct ID, by student_id, or by active_group_id
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableActiveVisits).
			Where("id = ? OR student_id = ? OR active_group_id = ?", id, id, id).
			Where("tenant_id = ?", tenantID),
			tableActiveVisits)

		// Delete from active.visits (cascade cleanup via activities.groups reference)
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableActiveVisits).
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ? AND tenant_id = ?)", id, tenantID).
			Where("tenant_id = ?", tenantID),
			"active.visits (cascade)")

		// Delete from active.group_supervisors before active.groups or users.staff
		// can cascade into it. Match only FK columns: matching the row's own
		// PK against a generic entity ID can delete unrelated supervisor rows
		// when auto-increment IDs collide across domains.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("active.group_supervisors").
			Where("staff_id = ? OR group_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"active.group_supervisors")

		// Delete from active.groups by direct ID or by reference.
		// room_id reference is required so facilities.rooms can be deleted
		// without tripping fk_active_groups_room_tenant.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("active.groups").
			Where("id = ? OR group_id = ? OR device_id = ? OR room_id = ?", id, id, id, id).
			Where("tenant_id = ?", tenantID),
			"active.groups")

		// ========================================
		// Activities domain cleanup
		// ========================================

		// Delete from activities.supervisors before activities.groups or
		// users.staff can cascade into it.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("activities.supervisors").
			Where("staff_id = ? OR group_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"activities.supervisors")

		// Delete from activities.student_enrollments (depends on activities.groups)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("activities.student_enrollments").
			Where("activity_group_id = ?", id).
			Where("tenant_id = ?", tenantID),
			"activities.student_enrollments")

		// Delete from activities.groups by ID, by category_id, or by created_by.
		// created_by reference is required so users.staff can be deleted without
		// tripping fk_activity_groups_created_by_tenant.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("activities.groups").
			Where("id = ? OR category_id = ? OR created_by = ?", id, id, id).
			Where("tenant_id = ?", tenantID),
			"activities.groups")

		// Delete from activities.categories (now safe after groups referencing them are deleted)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("activities.categories").
			Where(whereIDEquals, id).
			Where("tenant_id = ?", tenantID),
			"activities.categories")

		// ========================================
		// IoT domain cleanup
		// ========================================

		// Delete from iot.devices, but never the WEB-MANUAL-001 system device
		// (migration 1.7.5). Web-originated check-ins fall back to it, so
		// deleting it would break TestSchoolCheckin_* on every parallel run
		// where another test cleanup happens to pass its DB id.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("iot.devices").
			Where(whereIDEquals, id).
			Where("device_id != ?", "WEB-MANUAL-001").
			Where("tenant_id = ?", tenantID),
			"iot.devices")

		// ========================================
		// Facilities domain cleanup
		// ========================================

		// Delete from facilities.rooms, but never the system default room
		// (id=1, created by SetupTestDB). Concurrent test packages depend on it.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("facilities.rooms").
			Where(whereIDEquals, id).
			Where("id != ?", 1).
			Where("tenant_id = ?", tenantID),
			"facilities.rooms")

		// ========================================
		// Users domain cleanup (FK-dependent order)
		// ========================================

		// Delete from users.guests (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.guests").
			Where("id = ? OR staff_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"users.guests")

		// Delete from users.profiles (depends on account)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.profiles").
			Where(whereIDOrAccountID, id, id).
			Where("tenant_id = ?", tenantID),
			"users.profiles")

		// Delete from active.attendance by student_id or device_id.
		// device_id reference is required so iot.devices can be deleted without
		// tripping fk_attendance_device_tenant.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("active.attendance").
			Where("student_id = ? OR device_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"active.attendance")

		// Delete from users.students
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.students").
			Where(whereIDEquals, id).
			Where("tenant_id = ?", tenantID),
			"users.students")

		// Delete from active.work_sessions referencing this staff. The IoT
		// supervisor flow auto-creates a work_session via EnsureCheckedIn
		// when StartActivitySession runs in tests; the FK to users.staff has
		// no ON DELETE CASCADE, so the row must be cleared first.
		// active.work_session_breaks and audit.work_session_edits cascade
		// via session_id ON DELETE CASCADE.
		cleanupDelete(tb, db.NewDelete().
			TableExpr("active.work_sessions").
			Where("staff_id = ? OR created_by = ? OR updated_by = ?", id, id, id).
			Where("tenant_id = ?", tenantID),
			"active.work_sessions")

		// Delete from users.staff, but never the system staff fixture
		// (id=1, created by SetupTestDB). Tests across parallel packages share it.
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersStaff).
			Where(whereIDEquals, id).
			Where("id != ?", 1).
			Where("tenant_id = ?", tenantID),
			tableUsersStaff)

		// Delete from users.persons (last, as it's referenced by students and staff).
		// Skip the system person fixture (id=1, created by SetupTestDB) for the
		// same reason as users.staff above.
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersPersons).
			Where(whereIDEquals, id).
			Where("id != ?", 1).
			Where("tenant_id = ?", tenantID),
			tableUsersPersons)

		// NOTE: Auth domain cleanup intentionally omitted here.
		// Use CleanupAuthFixtures(accountIDs...) for auth cleanup.
		// Reason: Using generic IDs against auth tables causes cross-domain
		// collisions (e.g., student ID 5 would delete role ID 5).

		// ========================================
		// Users domain extended cleanup
		// ========================================

		// Delete from users.privacy_consents (by student_id)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.privacy_consents").
			Where("id = ? OR student_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"users.privacy_consents")

		// Delete from users.persons_guardians (by person_id or guardian_account_id)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.persons_guardians").
			Where("id = ? OR person_id = ? OR guardian_account_id = ?", id, id, id).
			Where("tenant_id = ?", tenantID),
			"users.persons_guardians")

		// Delete from users.students_guardians before users.guardian_profiles.
		// Since migration 1.15.127 the link → guardian FK is ON DELETE RESTRICT
		// (was CASCADE), so a guardian that is still linked cannot be deleted.
		// Clearing the link here keeps teardown order-independent: without it,
		// deleting guardian_profiles ahead of the student in the id list would
		// trip the FK and leave an orphan. (The link → student FK is CASCADE, so
		// deleting the student also clears it, but that only helps when the
		// student id is processed first.)
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.students_guardians").
			Where("student_id = ? OR guardian_profile_id = ?", id, id).
			Where("tenant_id = ?", tenantID),
			"users.students_guardians")

		// Delete from users.guardian_profiles
		cleanupDelete(tb, db.NewDelete().
			TableExpr("users.guardian_profiles").
			Where(whereIDEquals, id).
			Where("tenant_id = ?", tenantID),
			"users.guardian_profiles")

		// Delete from users.rfid_cards (note: string ID, but try as int64)
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersRFIDCards).
			Where(whereIDEquals, fmt.Sprintf("%d", id)).
			Where("tenant_id = ?", tenantID),
			tableUsersRFIDCards)
	}
}

// CleanupAuthFixtures removes auth account fixtures and their related records.
// Pass account IDs only - this will cascade delete:
//   - auth.tokens (by account_id)
//   - auth.account_roles (by account_id)
//   - auth.account_permissions (by account_id)
//   - auth.accounts (by id)
//
// NOTE: This does NOT touch auth.roles, auth.permissions, or auth.role_permissions
// since those are not account-specific. Use CleanupTableRecords for those if needed.
func CleanupAuthFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	// Use IN clause for efficiency instead of loop
	// Delete tokens first (depends on accounts)
	cleanupDelete(tb, db.NewDelete().
		Table("auth.tokens").
		Where(whereAccountIDIn, bun.List(accountIDs)),
		"auth.tokens")

	// Delete account_tenants (by account_id)
	cleanupDelete(tb, db.NewDelete().
		Table("auth.account_tenants").
		Where(whereAccountIDIn, bun.List(accountIDs)),
		"auth.account_tenants")

	// Delete account_roles (by account_id only - never by role_id!)
	cleanupDelete(tb, db.NewDelete().
		Table("auth.account_roles").
		Where(whereAccountIDIn, bun.List(accountIDs)),
		"auth.account_roles")

	// Delete account_permissions (by account_id only - never by permission_id!)
	cleanupDelete(tb, db.NewDelete().
		Table("auth.account_permissions").
		Where(whereAccountIDIn, bun.List(accountIDs)),
		"auth.account_permissions")

	// Delete grade_transitions that reference these accounts (created_by FK)
	cleanupDelete(tb, db.NewDelete().
		Table(tableEducationGradeTransition).
		Where("created_by IN (?)", bun.List(accountIDs)),
		tableEducationGradeTransition)

	// Finally delete the accounts themselves
	cleanupDelete(tb, db.NewDelete().
		Table("auth.accounts").
		Where(whereIDIn, bun.List(accountIDs)),
		"auth.accounts")
}

// CleanupParentAccountFixtures removes parent accounts by their IDs.
func CleanupParentAccountFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	cleanupDelete(tb, db.NewDelete().
		Table("auth.accounts_parents").
		Where(whereIDIn, bun.List(accountIDs)),
		"auth.accounts_parents")
}

// CleanupRFIDCards removes RFID cards by their string IDs.
func CleanupRFIDCards(tb testing.TB, db *bun.DB, tagIDs ...string) {
	tb.Helper()

	if len(tagIDs) == 0 {
		return
	}

	for _, tagID := range tagIDs {
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersRFIDCards).
			Where(whereIDEquals, tagID),
			tableUsersRFIDCards)
	}
}

// ============================================================================
// Education Domain Fixtures
// ============================================================================

// CreateTestEducationGroup creates a real education group (Schulklasse) in the database.
// Note: This is different from CreateTestActivityGroup (activities.groups).
func CreateTestEducationGroup(tb testing.TB, db *bun.DB, name string) *education.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make name unique by appending timestamp
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	group := &education.Group{
		Name: uniqueName,
	}
	group.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`education.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test education group")

	return group
}

// CreateTestTeacher creates a real teacher in the database.
// Teachers require a Staff record, which requires a Person record.
// Returns the teacher with Staff reference populated for cleanup.
func CreateTestTeacher(tb testing.TB, db *bun.DB, firstName, lastName string) *users.Teacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person)
	staff := CreateTestStaff(tb, db, firstName, lastName)

	teacher := &users.Teacher{
		StaffID: staff.ID,
	}
	teacher.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(teacher).
		ModelTableExpr(`users.teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test teacher")

	// Store staff reference for cleanup
	teacher.Staff = staff

	return teacher
}

// CreateTestGroupTeacher creates a group-teacher assignment in the database.
func CreateTestGroupTeacher(tb testing.TB, db *bun.DB, groupID, teacherID int64) *education.GroupTeacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gt := &education.GroupTeacher{
		GroupID:   groupID,
		TeacherID: teacherID,
	}
	gt.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(gt).
		ModelTableExpr(`education.group_teacher`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group teacher assignment")

	return gt
}

// CreateTestClassTeacher creates a staff-to-school-class assignment (#1772).
func CreateTestClassTeacher(tb testing.TB, db *bun.DB, staffID int64, schoolClass string) *education.ClassTeacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ct := &education.ClassTeacher{
		StaffID:     staffID,
		SchoolClass: schoolClass,
	}
	ct.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(ct).
		ModelTableExpr(`education.class_teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test class teacher assignment")

	return ct
}

// ============================================================================
// Active Domain Fixtures (Sessions and Visits)
// ============================================================================

// CreateTestActiveGroup creates a real active group (session) in the database.
// This requires an ActivityGroup (activities.groups) and Room to exist.
// Use this for testing session management and visit tracking.
func CreateTestActiveGroup(tb testing.TB, db *bun.DB, activityGroupID, roomID int64) *active.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	activeGroup := &active.Group{
		GroupID:        &activityGroupID,
		RoomID:         roomID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}
	activeGroup.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test active group")

	return activeGroup
}

// CreateTestVisit creates a real visit record in the database.
// This requires a Student and ActiveGroup to already exist.
func CreateTestVisit(tb testing.TB, db *bun.DB, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time) *active.Visit {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	visit := &active.Visit{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     entryTime,
		ExitTime:      exitTime,
	}
	visit.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(visit).
		ModelTableExpr(`active.visits`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test visit")

	return visit
}

// CreateTestGroupSupervisor creates a real group supervisor record in the database.
// This requires a Staff and ActiveGroup to already exist.
func CreateTestGroupSupervisor(tb testing.TB, db *bun.DB, staffID, activeGroupID int64, role string) *active.GroupSupervisor {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	supervisor := &active.GroupSupervisor{
		StaffID:   staffID,
		GroupID:   activeGroupID,
		Role:      role,
		StartDate: timezone.TodayDate(),
	}
	supervisor.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(supervisor).
		ModelTableExpr(`active.group_supervisors`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group supervisor")

	return supervisor
}

// CleanupPerson removes a person from the database by ID.
func CleanupPerson(tb testing.TB, db *bun.DB, personID int64) {
	tb.Helper()

	cleanupDelete(tb, db.NewDelete().
		TableExpr(tableUsersPersons).
		Where(whereIDEquals, personID),
		tableUsersPersons)
}

// CleanupAccount removes an account and related auth records from the database.
//
// Stops at the auth schema. An account created through a flow that provisions
// its school identity (registration, link-to-tenant, invitation acceptance)
// also owns rows in users.*; pair this with CleanupSchoolIdentity.
func CleanupAccount(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()

	CleanupAuthFixtures(tb, db, accountID)
}

// CleanupAccountWithIdentity removes an account together with the school
// identity provisioned for it — the pairing tests need after registration,
// link-to-tenant, or invitation acceptance. Exists so the ordering constraint
// documented on CleanupSchoolIdentity is not every caller's problem.
func CleanupAccountWithIdentity(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()

	CleanupSchoolIdentity(tb, db, accountID)
	CleanupAccount(tb, db, accountID)
}

// CleanupSchoolIdentity removes the person → staff → teacher chain that gets
// provisioned for an account at a school (#2222).
//
// Call it BEFORE CleanupAccount: the persons are found through their
// account_id, which the account row's deletion takes with it.
func CleanupSchoolIdentity(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var personIDs []int64
	if err := db.NewSelect().
		TableExpr(tableUsersPersons).
		ColumnExpr("id").
		Where(whereAccountIDIn, bun.List(accountIDs)).
		Scan(ctx, &personIDs); err != nil || len(personIDs) == 0 {
		return
	}

	var staffIDs []int64
	_ = db.NewSelect().
		TableExpr(tableUsersStaff).
		ColumnExpr("id").
		Where("person_id IN (?)", bun.List(personIDs)).
		Scan(ctx, &staffIDs)

	if len(staffIDs) > 0 {
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersTeachers).
			Where("staff_id IN (?)", bun.List(staffIDs)),
			tableUsersTeachers)

		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersStaff).
			Where("id IN (?)", bun.List(staffIDs)),
			tableUsersStaff)
	}

	cleanupDelete(tb, db.NewDelete().
		TableExpr(tableUsersPersons).
		Where("id IN (?)", bun.List(personIDs)),
		tableUsersPersons)
}

// CleanupRoleRecords removes roles and their role-permission/account-role associations.
// Deliberately separate from CleanupAccount, which never deletes by role ID.
func CleanupRoleRecords(tb testing.TB, db *bun.DB, roleIDs ...int64) {
	tb.Helper()
	if len(roleIDs) == 0 {
		return
	}

	ctx := TenantContext(fixtureTenantID(tb))

	_, _ = db.NewDelete().
		TableExpr("auth.role_permissions").
		Where("role_id IN (?)", bun.List(roleIDs)).
		Exec(ctx)

	_, _ = db.NewDelete().
		TableExpr("auth.account_roles").
		Where("role_id IN (?)", bun.List(roleIDs)).
		Exec(ctx)

	_, err := db.NewDelete().
		TableExpr("auth.roles").
		Where("id IN (?)", bun.List(roleIDs)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup roles: %v", err)
	}
}

// CleanupPermissionRecords removes permissions and their role/account associations.
// Deliberately separate from CleanupAccount, which never deletes by permission ID.
func CleanupPermissionRecords(tb testing.TB, db *bun.DB, permissionIDs ...int64) {
	tb.Helper()
	if len(permissionIDs) == 0 {
		return
	}

	ctx := TenantContext(fixtureTenantID(tb))

	_, _ = db.NewDelete().
		TableExpr("auth.role_permissions").
		Where("permission_id IN (?)", bun.List(permissionIDs)).
		Exec(ctx)

	_, _ = db.NewDelete().
		TableExpr("auth.account_permissions").
		Where("permission_id IN (?)", bun.List(permissionIDs)).
		Exec(ctx)

	_, err := db.NewDelete().
		TableExpr("auth.permissions").
		Where("id IN (?)", bun.List(permissionIDs)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup permissions: %v", err)
	}
}

// CleanupStaffFixtures removes staff fixtures from the database.
// Pass a staff ID and it will clean up the staff, person, and any related records.
// If the staff has an account, call CleanupAuthFixtures separately with the account ID.
func CleanupStaffFixtures(tb testing.TB, db *bun.DB, staffIDs ...int64) {
	tb.Helper()

	if len(staffIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, staffID := range staffIDs {
		// First get the staff to find the person ID
		// Use TableExpr and ColumnExpr to generate valid SQL
		var staff struct {
			PersonID int64 `bun:"person_id"`
			TenantID int64 `bun:"tenant_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			TableExpr(tableUsersStaff).
			ColumnExpr("person_id", "tenant_id").
			Where(whereIDEquals, staffID).
			Scan(ctx)

		deleteStaffCaregiverBindings(tb, db, staffID, 0, staff.TenantID)

		// Delete teacher if exists (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersTeachers).
			Where("staff_id = ?", staffID),
			tableUsersTeachers)

		// Delete staff
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersStaff).
			Where(whereIDEquals, staffID),
			tableUsersStaff)

		// Delete person if we found one
		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				TableExpr(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID),
				tableUsersPersons)
		}
	}
}

// CleanupTeacherFixtures removes teacher fixtures from the database.
// Pass a teacher ID and it will clean up the full chain: teacher -> staff -> person.
// Also cleans up the associated account via CleanupAuthFixtures.
func CleanupTeacherFixtures(tb testing.TB, db *bun.DB, teacherIDs ...int64) {
	tb.Helper()

	if len(teacherIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, teacherID := range teacherIDs {
		// Get the teacher to find the staff ID
		// Use TableExpr and ColumnExpr to generate valid SQL
		var teacher struct {
			StaffID  int64 `bun:"staff_id"`
			TenantID int64 `bun:"tenant_id"`
		}
		_ = db.NewSelect().
			Model(&teacher).
			TableExpr(tableUsersTeachers).
			ColumnExpr("staff_id", "tenant_id").
			Where(whereIDEquals, teacherID).
			Scan(ctx)

		// Get the staff to find the person ID and account ID
		var staff struct {
			PersonID int64 `bun:"person_id"`
			TenantID int64 `bun:"tenant_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			TableExpr(tableUsersStaff).
			ColumnExpr("person_id", "tenant_id").
			Where(whereIDEquals, teacher.StaffID).
			Scan(ctx)

		// Get the person to find the account ID
		var person struct {
			AccountID *int64 `bun:"account_id"`
		}
		_ = db.NewSelect().
			Model(&person).
			TableExpr(tableUsersPersons).
			ColumnExpr("account_id").
			Where(whereIDEquals, staff.PersonID).
			Scan(ctx)

		tenantID := teacher.TenantID
		if tenantID == 0 {
			tenantID = staff.TenantID
		}
		deleteStaffCaregiverBindings(tb, db, teacher.StaffID, teacherID, tenantID)

		// Delete teacher
		cleanupDelete(tb, db.NewDelete().
			TableExpr(tableUsersTeachers).
			Where(whereIDEquals, teacherID),
			tableUsersTeachers)

		// Delete staff
		if teacher.StaffID > 0 {
			cleanupDelete(tb, db.NewDelete().
				TableExpr(tableUsersStaff).
				Where(whereIDEquals, teacher.StaffID),
				tableUsersStaff)
		}

		// Delete person
		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				TableExpr(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID),
				tableUsersPersons)
		}

		// Delete account if exists
		if person.AccountID != nil && *person.AccountID > 0 {
			CleanupAuthFixtures(tb, db, *person.AccountID)
		}
	}
}

// CreateTestPersonWithAccountID creates a person linked to an existing account ID.
// Use this when you already have an account and want to link a person to it.
func CreateTestPersonWithAccountID(tb testing.TB, db *bun.DB, firstName, lastName string, accountID int64) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
		AccountID: &accountID,
	}
	person.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person with account ID")

	return person
}

// ============================================================================
// Auth Domain Fixtures (Accounts)
// ============================================================================

// CreateTestAccount creates a real account in the database for authentication testing.
// The email is made unique by appending a timestamp.
func CreateTestAccount(tb testing.TB, db *bun.DB, email string) *auth.Account {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make email unique
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, time.Now().UnixNano())

	account := &auth.Account{
		Email:  uniqueEmail,
		Active: true,
	}

	err := db.NewInsert().
		Model(account).
		ModelTableExpr(`auth.accounts`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test account")

	return account
}

// CreateTestAccountWithPassword creates an account with a hashed password.
// This is needed for login tests where the password needs to be verified.
func CreateTestAccountWithPassword(tb testing.TB, db *bun.DB, email, password string) *auth.Account {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Hash the password using Argon2id (same as production)
	hashedPassword, err := hashPassword(password)
	require.NoError(tb, err, "Failed to hash password")

	account := &auth.Account{
		Email:        email,
		Active:       true,
		PasswordHash: &hashedPassword,
	}

	err = db.NewInsert().
		Model(account).
		ModelTableExpr(`auth.accounts`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test account with password")

	return account
}

var (
	hashCacheMu sync.Mutex
	hashCache   = map[string]string{}
)

// hashPassword returns an Argon2id hash for the password, memoizing per
// password string. Fixtures reuse a handful of passwords across thousands
// of tests, and each uncached hash costs real CPU and memory. Reused salts
// are fine for test data; verification decodes params and salt from the
// hash itself. The mutex is held across the hash on purpose so concurrent
// requests for the same password compute it once.
func hashPassword(password string) (string, error) {
	hashCacheMu.Lock()
	defer hashCacheMu.Unlock()
	if h, ok := hashCache[password]; ok {
		return h, nil
	}
	h, err := hashPasswordUncached(password)
	if err != nil {
		return "", err
	}
	hashCache[password] = h
	return h, nil
}

// hashPasswordUncached hashes a password with the same algorithm as the
// auth service, using the cheap test-only params: these are throwaway test
// credentials, and the params travel inside the encoded hash, so
// verification still works.
func hashPasswordUncached(password string) (string, error) {
	return userpass.HashPassword(password, cheapArgon2Params)
}

// CreateTestPersonWithAccount creates a person linked to an account.
// This is needed for policy tests that look up users by account ID.
func CreateTestPersonWithAccount(tb testing.TB, db *bun.DB, firstName, lastName string) (*users.Person, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create account first
	account := CreateTestAccount(tb, db, fmt.Sprintf("%s.%s", firstName, lastName))

	// Create person with account reference
	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
		AccountID: &account.ID,
	}
	person.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person with account")

	return person, account
}

// CreateTestStudentWithAccount creates a student with linked person and account.
// Returns the student with PersonID set, and the associated account for auth context.
func CreateTestStudentWithAccount(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string) (*users.Student, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person with account
	person, account := CreateTestPersonWithAccount(tb, db, firstName, lastName)

	// Create student
	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: schoolClass,
	}
	student.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student with account")

	return student, account
}

// CreateTestStaffWithAccount creates a staff member with linked person and account.
func CreateTestStaffWithAccount(tb testing.TB, db *bun.DB, firstName, lastName string) (*users.Staff, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person with account
	person, account := CreateTestPersonWithAccount(tb, db, firstName, lastName)

	// Create staff
	staff := &users.Staff{
		PersonID: person.ID,
	}
	staff.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff with account")

	// Store person reference for convenience
	staff.Person = person

	return staff, account
}

// CreateTestCalendarStaff creates a staff member that is reachable for calendar
// invitations. On top of CreateTestStaffWithAccount it adds an active
// account_tenants mapping for tenant 1 and the base "user" role (which carries
// calendar:own via migration 1.15.171), mirroring a real onboarded staff
// account so FindReachableCalendarStaffIDs treats them as invitable. Use this in
// calendar tests wherever staff must be selectable recipients. Cleanup: the
// added rows are removed by CleanupAuthFixtures (account_tenants + account_roles
// by account_id), which calendar tests already call for the account.
func CreateTestCalendarStaff(tb testing.TB, db *bun.DB, firstName, lastName string) (*users.Staff, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	staff, account := CreateTestStaffWithAccount(tb, db, firstName, lastName)

	now := time.Now()
	mapping := &auth.AccountTenant{
		AccountID:   account.ID,
		TenantID:    fixtureTenantID(tb),
		Status:      auth.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	_, err := db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).Exec(ctx)
	require.NoError(tb, err, "Failed to create staff account_tenants mapping")

	var userRoleID int64
	err = db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", auth.BaseRoleUser).
		Scan(ctx, &userRoleID)
	require.NoError(tb, err, "Failed to find seeded user role")

	roleAssignment := &auth.AccountRole{AccountID: account.ID, RoleID: userRoleID}
	roleAssignment.SetTenantID(fixtureTenantID(tb))
	_, err = db.NewInsert().Model(roleAssignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(tb, err, "Failed to assign user role to staff account")

	return staff, account
}

// CreateTestTeacherWithAccount creates a teacher with full chain: Account → Person → Staff → Teacher.
// Returns the teacher and account for auth context testing.
func CreateTestTeacherWithAccount(tb testing.TB, db *bun.DB, firstName, lastName string) (*users.Teacher, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff with account
	staff, account := CreateTestStaffWithAccount(tb, db, firstName, lastName)

	// Create teacher
	teacher := &users.Teacher{
		StaffID: staff.ID,
	}
	teacher.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(teacher).
		ModelTableExpr(`users.teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test teacher with account")

	// Store staff reference for convenience
	teacher.Staff = staff

	return teacher, account
}

// AssignStudentToGroup updates a student's group assignment.
// This is used to set up the teacher-student-group relationship for policy testing.
func AssignStudentToGroup(tb testing.TB, db *bun.DB, studentID, groupID int64) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students`).
		Set("group_id = ?", groupID).
		Where(whereIDEquals, studentID).
		Exec(ctx)
	require.NoError(tb, err, "Failed to assign student to group")
}

// ============================================================================
// Auth Domain Extended Fixtures
// ============================================================================

// CreateTestRole creates a role in the database for permission testing.
func CreateTestRole(tb testing.TB, db *bun.DB, name string) *auth.Role {
	tb.Helper()
	return CreateTestRoleForTenant(tb, db, name, fixtureTenantID(tb))
}

// CreateTestRoleForTenant creates a role scoped to the given tenant.
// Use this when the test operates under a specific tenant context so that
// FindByID's tenant filter (tenant_id = ? OR tenant_id IS NULL) can find it.
func CreateTestRoleForTenant(tb testing.TB, db *bun.DB, name string, tenantID int64) *auth.Role {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make name unique
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	// base_role is required by the role-create API, so every real custom role
	// carries one; without it the role has no privilege tier and role-grant
	// checks fail it closed.
	baseRole := auth.BaseRoleUser
	role := &auth.Role{
		Name:        uniqueName,
		Description: "Test role: " + name,
		IsSystem:    false,
		TenantID:    &tenantID,
		BaseRole:    &baseRole,
	}

	err := db.NewInsert().
		Model(role).
		ModelTableExpr(`auth.roles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test role")

	return role
}

// CreateTestSystemRole creates a system role (tenant_id IS NULL, is_system = true).
// System roles are immutable and visible to all tenants.
func CreateTestSystemRole(tb testing.TB, db *bun.DB, name string) *auth.Role {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	role := &auth.Role{
		Name:        uniqueName,
		Description: "System role: " + name,
		IsSystem:    true,
		TenantID:    nil,
	}

	// Real system roles are identified by their canonical name ("admin",
	// "user"), which this fixture has to uniquify. Carry the tier in base_role
	// instead so role-grant checks see the same privilege level the caller asked
	// for rather than an unrecognizable name.
	if slices.Contains(auth.ValidBaseRoles(), strings.ToLower(strings.TrimSpace(name))) {
		baseRole := strings.ToLower(strings.TrimSpace(name))
		role.BaseRole = &baseRole
	}

	err := db.NewInsert().
		Model(role).
		ModelTableExpr(`auth.roles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test system role")

	return role
}

// AssignLehrkraftSystemRole assigns the seeded lehrkraft system role (#1772)
// to the account, scoped to the given tenant. The role is created by
// migration in every schema the tests run against, so the lookup must
// succeed. Cleanup: CleanupAuthFixtures removes auth.account_roles rows by
// account_id.
func AssignLehrkraftSystemRole(tb testing.TB, db *bun.DB, accountID, tenantID int64) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var roleID int64
	err := db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", "lehrkraft").
		Where("is_system = TRUE").
		Scan(ctx, &roleID)
	require.NoError(tb, err, "seeded lehrkraft system role must exist")

	roleAssignment := &auth.AccountRole{AccountID: accountID, RoleID: roleID}
	roleAssignment.SetTenantID(tenantID)
	_, err = db.NewInsert().
		Model(roleAssignment).
		ModelTableExpr(`auth.account_roles`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to assign lehrkraft system role")
}

// CreateTestPermission creates a permission in the database.
// Note: The database has a unique constraint on (resource, action), so each call
// creates a unique resource to avoid constraint violations.
func CreateTestPermission(tb testing.TB, db *bun.DB, name, resource, action string) *auth.Permission {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make name and resource unique to avoid constraint violations
	// The database has idx_permissions_resource_action unique constraint
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	uniqueName := fmt.Sprintf("%s-%s", name, uniqueSuffix)
	uniqueResource := fmt.Sprintf("%s-%s", resource, uniqueSuffix)

	permission := &auth.Permission{
		Name:        uniqueName,
		Description: "Test permission: " + name,
		Resource:    uniqueResource,
		Action:      action,
	}

	err := db.NewInsert().
		Model(permission).
		ModelTableExpr(`auth.permissions`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test permission")

	return permission
}

// CreateTestToken creates an auth token for testing.
// tokenType can be "access" or "refresh" to set appropriate expiry.
func CreateTestToken(tb testing.TB, db *bun.DB, accountID int64, tokenType string) *auth.Token {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate unique token value
	tokenValue := fmt.Sprintf("test-token-%s-%d", tokenType, time.Now().UnixNano())

	// Set expiry based on token type
	var expiry time.Time
	if tokenType == "refresh" {
		expiry = time.Now().Add(24 * time.Hour)
	} else {
		expiry = time.Now().Add(15 * time.Minute)
	}

	token := &auth.Token{
		AccountID:  accountID,
		Token:      tokenValue,
		Expiry:     expiry,
		Mobile:     false,
		FamilyID:   fmt.Sprintf("family-%d", time.Now().UnixNano()),
		Generation: 0,
	}
	token.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(token).
		ModelTableExpr(`auth.tokens`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test token")

	return token
}

// ============================================================================
// Users Domain Extended Fixtures
// ============================================================================

// CreateTestRFIDCard creates an RFID card in the database.
// The ID is uppercase alphanumeric only (no hyphens) to match normalization in PersonRepository.
func CreateTestRFIDCard(tb testing.TB, db *bun.DB, tagID string) *users.RFIDCard {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make tag ID unique - use only alphanumeric chars (no hyphens) to match normalization
	uniqueTagID := fmt.Sprintf("%s%d", tagID, time.Now().UnixNano())

	card := &users.RFIDCard{
		Active: true,
	}
	card.ID = uniqueTagID
	card.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(card).
		ModelTableExpr(`users.rfid_cards`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test RFID card")

	return card
}

// LinkRFIDToStudent links an RFID card to a person by updating their tag_id field.
// This is needed for the checkin workflow which looks up persons by tag_id.
func LinkRFIDToStudent(tb testing.TB, db *bun.DB, personID int64, tagID string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewUpdate().
		ModelTableExpr(`users.persons AS "person"`).
		Set("tag_id = ?", tagID).
		Where(whereIDEquals, personID).
		Exec(ctx)
	require.NoError(tb, err, "Failed to link RFID to person")
}

// CreateTestGuardianProfile creates a guardian profile in the database.
func CreateTestGuardianProfile(tb testing.TB, db *bun.DB, email string) *users.GuardianProfile {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make email unique
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, time.Now().UnixNano())

	profile := &users.GuardianProfile{
		FirstName:              "Guardian",
		LastName:               "Test",
		Email:                  &uniqueEmail,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")

	return profile
}

// ============================================================================
// Education Domain Extended Fixtures
// ============================================================================

// CreateTestGroupSubstitution creates a teacher substitution record.
// regularStaffID can be nil if no regular staff is being substituted.
func CreateTestGroupSubstitution(tb testing.TB, db *bun.DB, groupID int64, regularStaffID *int64, substituteStaffID int64, startDate, endDate timezone.Date) *education.GroupSubstitution {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	substitution := &education.GroupSubstitution{
		GroupID:           groupID,
		RegularStaffID:    regularStaffID,
		SubstituteStaffID: substituteStaffID,
		StartDate:         startDate,
		EndDate:           endDate,
		Reason:            "Test substitution",
	}
	substitution.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(substitution).
		ModelTableExpr(`education.group_substitution`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group substitution")

	return substitution
}

// CreateTestGuest creates a guest instructor in the database.
// This requires a Staff record, which is created automatically.
func CreateTestGuest(tb testing.TB, db *bun.DB, expertise string) *users.Guest {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person)
	staff := CreateTestStaff(tb, db, "Guest", "Instructor")

	guest := &users.Guest{
		StaffID:           staff.ID,
		ActivityExpertise: expertise,
		Organization:      "Test Organization",
	}
	guest.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(guest).
		ModelTableExpr(`users.guests`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test guest")

	// Store staff reference for cleanup
	guest.Staff = staff

	return guest
}

// CreateTestProfile creates a user profile in the database.
// This requires an Account, which is created automatically.
func CreateTestProfile(tb testing.TB, db *bun.DB, prefix string) *users.Profile {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create account first
	account := CreateTestAccount(tb, db, prefix)

	profile := &users.Profile{
		AccountID: account.ID,
		Avatar:    "https://example.com/avatar.png",
		Bio:       "Test bio for " + prefix,
		Settings:  `{"theme": "dark"}`,
	}
	profile.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.profiles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test profile")

	// Store account reference for convenience
	profile.Account = account

	return profile
}

// CreateTestPrivacyConsent creates a privacy consent record in the database.
// This requires a Student, which is created automatically.
func CreateTestPrivacyConsent(tb testing.TB, db *bun.DB, prefix string) *users.PrivacyConsent {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create student first
	student := CreateTestStudent(tb, db, "Consent", prefix, "1a")

	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0) // 1 year from now
	durationDays := 365

	consent := &users.PrivacyConsent{
		StudentID:         student.ID,
		PolicyVersion:     "v1.0",
		Accepted:          true,
		AcceptedAt:        &now,
		ExpiresAt:         &expiresAt,
		DurationDays:      &durationDays,
		RenewalRequired:   false,
		DataRetentionDays: 30,
	}
	consent.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(consent).
		ModelTableExpr(`users.privacy_consents`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test privacy consent")

	// Store student reference for cleanup
	consent.Student = student

	return consent
}

// CreateTestParentAccount creates a parent account in the database.
func CreateTestParentAccount(tb testing.TB, db *bun.DB, email string) *auth.AccountParent {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make email unique
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, time.Now().UnixNano())
	username := fmt.Sprintf("parent-%d", time.Now().UnixNano())

	account := &auth.AccountParent{
		Email:    uniqueEmail,
		Username: &username,
		Active:   true,
	}
	account.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(account).
		ModelTableExpr(`auth.accounts_parents`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test parent account")

	return account
}

// ============================================================================
// Schedule Domain Fixtures
// ============================================================================

// CleanupScheduleFixtures removes schedule-related fixtures from the database.
func CleanupScheduleFixtures(tb testing.TB, db *bun.DB, timeframeIDs ...int64) {
	tb.Helper()

	if len(timeframeIDs) == 0 {
		return
	}

	for _, id := range timeframeIDs {
		cleanupDelete(tb, db.NewDelete().
			TableExpr("schedule.timeframes").
			Where(whereIDEquals, id),
			"schedule.timeframes")
	}
}

// ============================================================================
// Auth Domain Extended Fixtures (Invitations)
// ============================================================================

// InvitationTokenOptions contains optional fields for creating test invitation tokens.
type InvitationTokenOptions struct {
	FirstName *string
	LastName  *string
}

// CreateTestInvitationToken creates an invitation token in the database.
// Requires a role and creator account to exist.
func CreateTestInvitationToken(tb testing.TB, db *bun.DB, email string, roleID, createdBy int64, expiresAt time.Time) *auth.InvitationToken {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make email unique
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, time.Now().UnixNano())
	token := fmt.Sprintf("test-token-%d", time.Now().UnixNano())

	invitation := &auth.InvitationToken{
		Email:     uniqueEmail,
		Token:     token,
		RoleID:    roleID,
		ExpiresAt: expiresAt,
	}
	if createdBy > 0 {
		invitation.CreatedBy = base.Int64Ptr(createdBy)
	}
	invitation.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(invitation).
		ModelTableExpr(`auth.invitation_tokens`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test invitation token")

	return invitation
}

// CreateTestInvitationTokenWithOptions creates an invitation token with optional fields.
func CreateTestInvitationTokenWithOptions(tb testing.TB, db *bun.DB, email string, roleID, createdBy int64, expiresAt time.Time, opts *InvitationTokenOptions) *auth.InvitationToken {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make email unique
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, time.Now().UnixNano())
	token := fmt.Sprintf("test-token-%d", time.Now().UnixNano())

	invitation := &auth.InvitationToken{
		Email:     uniqueEmail,
		Token:     token,
		RoleID:    roleID,
		ExpiresAt: expiresAt,
	}
	if createdBy > 0 {
		invitation.CreatedBy = base.Int64Ptr(createdBy)
	}

	if opts != nil {
		invitation.FirstName = opts.FirstName
		invitation.LastName = opts.LastName
	}
	invitation.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(invitation).
		ModelTableExpr(`auth.invitation_tokens`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test invitation token with options")

	return invitation
}

// CleanupInvitationFixtures removes invitation tokens from the database.
func CleanupInvitationFixtures(tb testing.TB, db *bun.DB, invitationIDs ...int64) {
	tb.Helper()

	if len(invitationIDs) == 0 {
		return
	}

	for _, id := range invitationIDs {
		cleanupDelete(tb, db.NewDelete().
			TableExpr("auth.invitation_tokens").
			Where(whereIDEquals, id),
			"auth.invitation_tokens")
	}
}

// GetOrCreateTestRole gets an existing role by name or creates a test role.
// This is useful for invitation tests that need a valid role.
func GetOrCreateTestRole(tb testing.TB, db *bun.DB, name string) *auth.Role {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to find existing role first
	var role auth.Role
	err := db.NewSelect().
		Model(&role).
		ModelTableExpr(`auth.roles AS "role"`).
		Where(`"role".name = ?`, name).
		Scan(ctx)

	if err == nil {
		return &role
	}

	// Create a new role if not found
	tenantID := fixtureTenantID(tb)
	// base_role is required by the role-create API, so every real custom role
	// carries one; without it the role has no privilege tier and role-grant
	// checks fail it closed.
	baseRole := auth.BaseRoleUser
	role = auth.Role{
		Name:        fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		Description: "Test role for " + name,
		IsSystem:    false,
		TenantID:    &tenantID,
		BaseRole:    &baseRole,
	}

	err = db.NewInsert().
		Model(&role).
		ModelTableExpr(`auth.roles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test role")

	return &role
}

// ============================================================================
// JWT Test Helpers
// ============================================================================

// TestTokenAuth is a shared TokenAuth instance for tests using a known secret.
// This allows tests to generate valid JWT tokens without needing the app config.
var testTokenAuthInstance *jwt.TokenAuth

// testJWTSecret is a fixed secret for testing (never use in production)
const testJWTSecret = "test-jwt-secret-32-chars-minimum"

// GetTestTokenAuth returns a TokenAuth instance for testing.
// Uses a singleton pattern to ensure all tests use the same secret.
func GetTestTokenAuth(tb testing.TB) *jwt.TokenAuth {
	tb.Helper()

	if testTokenAuthInstance == nil {
		var err error
		testTokenAuthInstance, err = jwt.NewTokenAuthWithSecret(testJWTSecret)
		require.NoError(tb, err, "Failed to create test TokenAuth")
	}

	return testTokenAuthInstance
}

// CreateTestJWT creates a valid JWT access token for the given account ID.
// This token can be used in the Authorization header for authenticated API requests.
func CreateTestJWT(tb testing.TB, accountID int64, permissions []string) string {
	tb.Helper()

	tokenAuth := GetTestTokenAuth(tb)

	claims := jwt.AppClaims{
		ID:          int(accountID),
		Sub:         fmt.Sprintf("%d", accountID), // Required claim - subject identifier
		Roles:       []string{"user"},
		Permissions: permissions,
		TenantID:    fixtureTenantID(tb),
	}

	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(tb, err, "Failed to create test JWT")

	return token
}

// ============================================================================
// Grade Transition Domain Fixtures
// ============================================================================

// CreateTestGradeTransition creates a grade transition in the database.
func CreateTestGradeTransition(tb testing.TB, db *bun.DB, academicYear string, createdBy int64) *education.GradeTransition {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transition := &education.GradeTransition{
		AcademicYear: academicYear,
		Status:       education.TransitionStatusDraft,
		CreatedBy:    createdBy,
	}
	transition.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(transition).
		ModelTableExpr(tableEducationGradeTransition).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test grade transition")

	return transition
}

// CreateTestGradeTransitionMapping creates a mapping for a grade transition.
func CreateTestGradeTransitionMapping(tb testing.TB, db *bun.DB, transitionID int64, fromClass string, toClass *string) *education.GradeTransitionMapping {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mapping := &education.GradeTransitionMapping{
		TransitionID: transitionID,
		FromClass:    fromClass,
		ToClass:      toClass,
	}
	mapping.SetTenantID(fixtureTenantID(tb))

	err := db.NewInsert().
		Model(mapping).
		ModelTableExpr(`education.grade_transition_mappings`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test grade transition mapping")

	return mapping
}

// CleanupGradeTransitionFixtures removes grade transition fixtures from the database.
// Pass transition IDs and it will clean up the transition, mappings, and history.
func CleanupGradeTransitionFixtures(tb testing.TB, db *bun.DB, transitionIDs ...int64) {
	tb.Helper()

	if len(transitionIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete history first (depends on transition)
	_, _ = db.NewDelete().
		TableExpr("education.grade_transition_history").
		Where("transition_id IN (?)", bun.List(transitionIDs)).
		Exec(ctx)

	// Delete mappings (depends on transition)
	_, _ = db.NewDelete().
		TableExpr("education.grade_transition_mappings").
		Where("transition_id IN (?)", bun.List(transitionIDs)).
		Exec(ctx)

	// Delete transitions
	_, _ = db.NewDelete().
		TableExpr(tableEducationGradeTransition).
		Where(whereIDIn, bun.List(transitionIDs)).
		Exec(ctx)
}

// ============================================================================
// Multi-Tenancy Isolation Test Fixtures (WP 3.19)
// ============================================================================

// EnsureTestTenant creates the platform.organizations and platform.schools
// rows required by the FK constraint on tenant_id. Uses ON CONFLICT DO NOTHING
// so it's safe to call multiple times with the same ID.
func EnsureTestTenant(tb testing.TB, db *bun.DB, tenantID int64) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(tb, ensureTestTenant(ctx, db, tenantID), "Failed to ensure test tenant")
}

// ensureTestTenant is the error-returning core of EnsureTestTenant, shared
// with the process-once clone bootstrap in db_clone.go.
func ensureTestTenant(ctx context.Context, db *bun.DB, tenantID int64) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.organizations (id, name, slug, active)
		VALUES (?, ?, ?, true)
		ON CONFLICT (id) DO NOTHING`,
		tenantID, fmt.Sprintf("Test Org %d", tenantID), fmt.Sprintf("test-org-%d", tenantID)); err != nil {
		return fmt.Errorf("ensure test organization: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (?, ?, ?, ?, ?, true)
		ON CONFLICT (id) DO NOTHING`,
		tenantID, tenantID,
		fmt.Sprintf("Test School %d", tenantID),
		fmt.Sprintf("test-school-%d", tenantID),
		fmt.Sprintf("t%d", tenantID)); err != nil {
		return fmt.Errorf("ensure test school: %w", err)
	}

	// Advance sequences past the explicitly inserted ID so that tests using
	// auto-generated IDs (nextval) don't collide.
	if _, err := db.ExecContext(ctx, `
		SELECT setval(pg_get_serial_sequence('platform.organizations', 'id'),
			GREATEST((SELECT last_value FROM platform.organizations_id_seq), ?))`,
		tenantID); err != nil {
		return fmt.Errorf("advance org sequence past explicit tenant ID: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		SELECT setval(pg_get_serial_sequence('platform.schools', 'id'),
			GREATEST((SELECT last_value FROM platform.schools_id_seq), ?))`,
		tenantID); err != nil {
		return fmt.Errorf("advance school sequence past explicit tenant ID: %w", err)
	}

	return nil
}

// CreateTestTenant creates an organization + school pair that nobody else
// shares and returns the school id (= tenant id) plus its subdomain. Pair it
// with CleanupTestTenant.
//
// Prefer this over EnsureTestTenant(db, 42) whenever a test mutates the school
// row itself — flipping `active`, stamping `deleted_at` — or asserts on
// tenant-wide state. EnsureTestTenant's ON CONFLICT DO NOTHING means a literal
// ID silently joins whatever rows a parallel package left behind.
//
// The ID deliberately does NOT come from UniqueTestTenantID: those are
// nanosecond-scale and do not survive a JWT round-trip (JSON numbers decode as
// float64, exact only below 2^53), so anything asserting on the tenant_id
// claim — or any refresh, which compares the claim against the stored tenant —
// would work off a rounded value. Nor can the ID be left to the sequence:
// EnsureTestTenant setvals it up to whatever nanosecond ID it was handed, so a
// nextval-assigned school inherits the same problem.
func CreateTestTenant(tb testing.TB, db *bun.DB) (tenantID int64, subdomain string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tenantID = uniqueJWTSafeTenantID()
	token := fmt.Sprintf("%d-%d", tenantID, time.Now().UnixNano())
	subdomain = fmt.Sprintf("t%d", tenantID)

	_, err := db.ExecContext(ctx, `
		INSERT INTO platform.organizations (id, name, slug, active)
		VALUES (?, ?, ?, true)`,
		tenantID, "Test Org "+token, "test-org-"+token)
	require.NoError(tb, err, "Failed to create test organization")

	_, err = db.ExecContext(ctx, `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (?, ?, ?, ?, ?, true)`,
		tenantID, tenantID, "Test School "+token, "test-school-"+token, subdomain)
	require.NoError(tb, err, "Failed to create test school")

	// Push both sequences clear of the WHOLE band, not just past this ID:
	// setting them to the ID itself would make the next nextval collide with
	// the next ID this helper hands out.
	for _, seq := range []string{"platform.organizations", "platform.schools"} {
		_, err = db.ExecContext(ctx, fmt.Sprintf(`
			SELECT setval(pg_get_serial_sequence('%s', 'id'),
				GREATEST((SELECT last_value FROM %s_id_seq), ?))`, seq, seq),
			testTenantIDCeiling)
		require.NoError(tb, err, "Failed to advance sequence past the test tenant band")
	}

	return tenantID, subdomain
}

// CleanupTestTenant removes the school + owning organization rows created by
// CreateTestTenant, plus the audit rows that reference the school. Call it
// AFTER the account cleanup that owns the account_tenants and account_roles
// rows, otherwise the school delete trips their FKs.
func CleanupTestTenant(tb testing.TB, db *bun.DB, tenantIDs ...int64) {
	tb.Helper()

	if len(tenantIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The organization id is not the school id, so resolve it before the
	// school rows disappear.
	var orgIDs []int64
	if err := db.NewSelect().
		ColumnExpr("DISTINCT organization_id").
		TableExpr("platform.schools").
		Where(whereIDIn, bun.List(tenantIDs)).
		Scan(ctx, &orgIDs); err != nil {
		tb.Logf("cleanup lookup platform.schools organization_id: %v", err)
	}

	// Auth events are written from a detached goroutine, so a row can still
	// land between the test body and this cleanup; deleting them first keeps
	// the school delete from failing on the FK.
	cleanupDelete(tb, db.NewDelete().
		Table("audit.auth_events").
		Where(whereTenantIDIn, bun.List(tenantIDs)),
		"audit.auth_events")

	// ensureTestTenant provisions the virtual WEB-MANUAL-001 device the same
	// way a real school gets one, so the school row cannot be deleted while it
	// is still there (devices_tenant_id_fkey).
	cleanupDelete(tb, db.NewDelete().
		Table("iot.devices").
		Where(whereTenantIDIn, bun.List(tenantIDs)),
		"iot.devices")

	cleanupDelete(tb, db.NewDelete().
		Table("platform.schools").
		Where(whereIDIn, bun.List(tenantIDs)),
		"platform.schools")

	if len(orgIDs) > 0 {
		cleanupDelete(tb, db.NewDelete().
			Table("platform.organizations").
			Where(whereIDIn, bun.List(orgIDs)),
			"platform.organizations")
	}
}

// MapAccountToTenant creates an active account_tenants mapping without
// ensuring the tenant infrastructure (organization/school) exists first.
// Use this when the tenant has already been ensured via EnsureTestTenant.
func MapAccountToTenant(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at)
		 VALUES (?, ?, 'active', NOW(), NOW())
		 ON CONFLICT (account_id, tenant_id) DO NOTHING`, accountID, tenantID)
	require.NoError(t, err)
}

// EnsureAccountTenant creates an active account_tenants mapping so that
// resolveAccountTenantDefault can find a tenant for the account during login.
// Uses ON CONFLICT DO NOTHING so it is safe to call multiple times.
func EnsureAccountTenant(tb testing.TB, db *bun.DB, accountID, tenantID int64) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	EnsureTestTenant(tb, db, tenantID)

	_, err := db.ExecContext(ctx, `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, created_at, updated_at)
		VALUES (?, ?, 'active', NOW(), NOW())
		ON CONFLICT (account_id, tenant_id) DO NOTHING`,
		accountID, tenantID)
	require.NoError(tb, err, "Failed to ensure account tenant mapping")
}

// CreateTestPersonForTenant creates a person belonging to a specific tenant.
func CreateTestPersonForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName string) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
	}
	person.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person for tenant")

	return person
}

// CreateTestStudentForTenant creates a student (and person) belonging to a specific tenant.
func CreateTestStudentForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName, className string) *users.Student {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first (Student has FK to Person)
	person := CreateTestPersonForTenant(tb, db, tenantID, firstName, lastName)

	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: className,
	}
	student.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student for tenant")

	student.Person = person
	return student
}

// CreateTestRoomForTenant creates a room belonging to a specific tenant.
func CreateTestRoomForTenant(tb testing.TB, db *bun.DB, tenantID int64, name string) *facilities.Room {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	room := &facilities.Room{
		Name:     uniqueName,
		Building: "Test Building",
		Capacity: IntPtr(30),
	}
	room.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test room for tenant")

	return room
}

// CreateTestEducationGroupForTenant creates an education group belonging to a specific tenant.
func CreateTestEducationGroupForTenant(tb testing.TB, db *bun.DB, tenantID int64, name string) *education.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	group := &education.Group{
		Name: uniqueName,
	}
	group.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`education.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test education group for tenant")

	return group
}

// CreateTestTimeframeForTenant creates a timeframe belonging to a specific tenant.
func CreateTestTimeframeForTenant(tb testing.TB, db *bun.DB, tenantID int64, description string) *schedule.Timeframe {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueDesc := fmt.Sprintf("%s-%d", description, time.Now().UnixNano())

	now := time.Now()
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 16, 0, 0, 0, now.Location())

	timeframe := &schedule.Timeframe{
		StartTime:   startTime,
		EndTime:     &endTime,
		IsActive:    true,
		Description: uniqueDesc,
	}
	timeframe.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(timeframe).
		ModelTableExpr(`schedule.timeframes`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test timeframe for tenant")

	return timeframe
}

// CreateTestDeviceForTenant creates an IoT device belonging to a specific tenant.
func CreateTestDeviceForTenant(tb testing.TB, db *bun.DB, tenantID int64, deviceID string) *iot.Device {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, time.Now().UnixNano())

	device := &iot.Device{
		DeviceID:   uniqueDeviceID,
		DeviceType: "terminal",
		Name:       StrPtr("Test Device " + uniqueDeviceID),
		Status:     iot.DeviceStatusActive,
		APIKey:     StrPtr("test-api-key-" + uniqueDeviceID),
	}
	device.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(device).
		ModelTableExpr(`iot.devices`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test device for tenant")

	return device
}

// CreateTestTokenForTenant creates an auth token belonging to a specific tenant.
// Requires an existing account ID (accounts are not tenant-scoped).
func CreateTestTokenForTenant(tb testing.TB, db *bun.DB, tenantID int64, accountID int64) *auth.Token {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tokenValue := fmt.Sprintf("test-token-t%d-%d", tenantID, time.Now().UnixNano())

	token := &auth.Token{
		AccountID:  accountID,
		Token:      tokenValue,
		Expiry:     time.Now().Add(24 * time.Hour),
		Mobile:     false,
		FamilyID:   fmt.Sprintf("family-t%d-%d", tenantID, time.Now().UnixNano()),
		Generation: 0,
	}
	token.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(token).
		ModelTableExpr(`auth.tokens`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test token for tenant")

	return token
}

// CreateTestFeedbackEntryForTenant creates a feedback entry belonging to a specific tenant.
// Requires an existing student ID within the same tenant.
func CreateTestFeedbackEntryForTenant(tb testing.TB, db *bun.DB, tenantID int64, studentID int64) *feedback.Entry {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()

	entry := &feedback.Entry{
		Value:     feedback.ValuePositive,
		Day:       timezone.DateFromTime(now),
		Time:      now,
		StudentID: studentID,
	}
	entry.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(entry).
		ModelTableExpr(`feedback.entries`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test feedback entry for tenant")

	return entry
}

// CreateTestStaffForTenant creates a staff member (and person) belonging to a specific tenant.
func CreateTestStaffForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName string) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := CreateTestPersonForTenant(tb, db, tenantID, firstName, lastName)

	staff := &users.Staff{
		PersonID: person.ID,
	}
	staff.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff for tenant")

	staff.Person = person
	return staff
}

// CreateTestStaffWithAccountForTenant is CreateTestStaffWithAccount for a
// caller-owned tenant instead of the shared tenant 1. Use it whenever the test
// also needs the account's tenant mapping, roles, or the school row itself —
// those all hang off the tenant id, and tenant 1 is shared with every other
// package running in parallel.
func CreateTestStaffWithAccountForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName string) (*users.Staff, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	account := CreateTestAccount(tb, db, fmt.Sprintf("%s.%s", firstName, lastName))

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
		AccountID: &account.ID,
	}
	person.SetTenantID(tenantID)
	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person with account for tenant")

	staff := &users.Staff{PersonID: person.ID}
	staff.SetTenantID(tenantID)
	err = db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff with account for tenant")

	staff.Person = person
	return staff, account
}

// CreateTestClassTeacherForTenant is CreateTestClassTeacher for a caller-owned
// tenant. The class-day surface reads these rows under RLS, so the assignment
// must live in the same tenant as the JWT the test presents.
func CreateTestClassTeacherForTenant(tb testing.TB, db *bun.DB, tenantID, staffID int64, schoolClass string) *education.ClassTeacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ct := &education.ClassTeacher{
		StaffID:     staffID,
		SchoolClass: schoolClass,
	}
	ct.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(ct).
		ModelTableExpr(`education.class_teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test class teacher assignment for tenant")

	return ct
}

// CreateTestActivityCategoryForTenant creates an activity category belonging to a specific tenant.
func CreateTestActivityCategoryForTenant(tb testing.TB, db *bun.DB, tenantID int64, name string) *activities.Category {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	category := &activities.Category{
		Name:  uniqueName,
		Color: "#CCCCCC",
	}
	category.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(category).
		ModelTableExpr(`activities.categories`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity category for tenant")

	return category
}

// CreateTestActivityGroupForTenant creates an activity group belonging to a specific tenant.
// Automatically creates a category and staff (creator) for the tenant.
func CreateTestActivityGroupForTenant(tb testing.TB, db *bun.DB, tenantID int64, name string) *activities.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	category := CreateTestActivityCategoryForTenant(tb, db, tenantID, fmt.Sprintf("Cat-%s", name))
	staff := CreateTestStaffForTenant(tb, db, tenantID, "Creator", name)

	group := &activities.Group{
		Name:            name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
		CreatedBy:       &staff.ID,
	}
	group.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity group for tenant")

	return group
}

// CreateTestActiveGroupForTenant creates an active group (session) belonging to a specific tenant.
// Self-contained: creates its own room and activity group dependencies.
func CreateTestActiveGroupForTenant(tb testing.TB, db *bun.DB, tenantID int64) *active.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	room := CreateTestRoomForTenant(tb, db, tenantID, "IsolationRoom")
	activityGroup := CreateTestActivityGroupForTenant(tb, db, tenantID, "IsolationActivity")

	now := time.Now()
	activityGroupID := activityGroup.ID
	activeGroup := &active.Group{
		GroupID:        &activityGroupID,
		RoomID:         room.ID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}
	activeGroup.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test active group for tenant")

	return activeGroup
}

// CreateTestActiveGroupWithIDsForTenant creates an active group bound to
// caller-supplied activity group and room IDs in the given tenant. Use this
// when the test owns the room and activity group fixtures (so it can clean
// them up explicitly) rather than letting CreateTestActiveGroupForTenant
// auto-create and leak them.
func CreateTestActiveGroupWithIDsForTenant(tb testing.TB, db *bun.DB, tenantID, activityGroupID, roomID int64) *active.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now()
	activeGroup := &active.Group{
		GroupID:        &activityGroupID,
		RoomID:         roomID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}
	activeGroup.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test active group with explicit IDs for tenant")

	return activeGroup
}

// CreateTestVisitForTenant creates a visit belonging to a specific tenant.
func CreateTestVisitForTenant(tb testing.TB, db *bun.DB, tenantID int64, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time) *active.Visit {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	visit := &active.Visit{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     entryTime,
		ExitTime:      exitTime,
	}
	visit.SetTenantID(tenantID)

	err := db.NewInsert().
		Model(visit).
		ModelTableExpr(`active.visits`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test visit for tenant")

	return visit
}

// CreateTestSuggestionPostForTenant creates a suggestion post belonging to a specific tenant.
func CreateTestSuggestionPostForTenant(tb testing.TB, db *bun.DB, tenantID int64, accountID int64) *suggestions.Post {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Isolation Post T%d-%d", tenantID, time.Now().UnixNano()),
		Description: "Test suggestion post for tenant isolation",
		AuthorID:    accountID,
		Status:      suggestions.StatusOpen,
	}
	post.SetTenantID(tenantID)

	_, err := db.NewInsert().
		Model(post).
		ModelTableExpr(`suggestions.posts`).
		Returning("*").
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test suggestion post for tenant")

	return post
}

// CreateTestDataDeletionForTenant creates a data deletion audit record belonging to a specific tenant.
func CreateTestDataDeletionForTenant(tb testing.TB, db *bun.DB, tenantID int64, studentID int64) *audit.DataDeletion {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deletion := audit.NewDataDeletion(
		studentID,
		audit.DeletionTypeManual,
		10,
		"test-system",
	)
	deletion.SetTenantID(tenantID)
	deletion.DeletionReason = "Tenant isolation test"

	_, err := db.NewInsert().
		Model(deletion).
		ModelTableExpr(`audit.data_deletions`).
		Returning("*").
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test data deletion for tenant")

	return deletion
}

// CleanupTenantTestData removes all test data for the specified tenant IDs
// from all tenant-scoped tables, in FK-safe order.
func CleanupTenantTestData(tb testing.TB, db *bun.DB, tenantIDs ...int64) {
	tb.Helper()

	if len(tenantIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete in reverse-FK order (children before parents).
	// Each delete is best-effort; failures are logged but do not fail the test.
	tables := []string{
		"feedback.entries",
		"auth.tokens",
		"schedule.timeframes",
		"iot.devices",
		"suggestions.votes",
		"suggestions.comment_reads",
		"suggestions.comments",
		"suggestions.posts",
		"audit.data_deletions",
		"audit.auth_events",
		"active.visits",
		"active.group_supervisors",
		"active.groups",
		"activities.student_enrollments",
		"activities.groups",
		"schedule.planning_tracks",
		"activities.categories",
		"education.group_teacher",
		"education.group_substitution",
		"education.groups",
		"users.students",
		"users.staff",
		"users.persons",
		"facilities.rooms",
	}

	for _, table := range tables {
		_, err := db.NewDelete().
			TableExpr(table).
			Where("tenant_id IN (?)", bun.List(tenantIDs)).
			Exec(ctx)
		if err != nil {
			tb.Logf("cleanup %s for tenants %v: %v", table, tenantIDs, err)
		}
	}
}

// ============================================================================
// WP-B11 fixtures: timetable student day/week
// ============================================================================

// parseTimeHHMM turns "HH:MM" into a time.Time anchored on 2000-01-01 (so
// only the clock components matter for the TIME column). Invalid input fails
// the test loudly.
func parseTimeHHMM(tb testing.TB, hhmm string) time.Time {
	tb.Helper()
	t, err := time.Parse("15:04", hhmm)
	require.NoError(tb, err, "invalid HH:MM literal %q", hhmm)
	return time.Date(2000, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC)
}

// CreateTestArrivalSchedule inserts a weekly arrival schedule for a student.
// staffID must reference users.staff(id) — the schema's created_by FK.
func CreateTestArrivalSchedule(tb testing.TB, db *bun.DB, studentID int64, weekday int, staffID int64, arrivalHHMM string) *schedule.StudentArrivalSchedule {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentArrivalSchedule{
		StudentID:       studentID,
		Weekday:         weekday,
		ExpectedArrival: parseTimeHHMM(tb, arrivalHHMM),
		CreatedBy:       staffID,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.student_arrival_schedules`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test arrival schedule")
	return row
}

// CreateTestArrivalException inserts a date-specific arrival exception.
// Pass arrivalHHMM="" to signal absence on that date (ExpectedArrival=NULL).
func CreateTestArrivalException(tb testing.TB, db *bun.DB, studentID int64, date timezone.Date, staffID int64, arrivalHHMM, reason string) *schedule.StudentArrivalException {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentArrivalException{
		StudentID:     studentID,
		ExceptionDate: date,
		CreatedBy:     staffID,
	}
	if arrivalHHMM != "" {
		t := parseTimeHHMM(tb, arrivalHHMM)
		row.ExpectedArrival = &t
	}
	if reason != "" {
		row.Reason = &reason
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.student_arrival_exceptions`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test arrival exception")
	return row
}

// CreateTestPickupSchedule inserts a weekly pickup schedule for a student.
func CreateTestPickupSchedule(tb testing.TB, db *bun.DB, studentID int64, weekday int, staffID int64, pickupHHMM string) *schedule.StudentPickupSchedule {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentPickupSchedule{
		StudentID:  studentID,
		Weekday:    weekday,
		PickupTime: parseTimeHHMM(tb, pickupHHMM),
		CreatedBy:  staffID,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.student_pickup_schedules`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test pickup schedule")
	return row
}

// CreateTestPickupException inserts a date-specific pickup exception.
// Pass pickupHHMM="" for absence (PickupTime=NULL).
func CreateTestPickupException(tb testing.TB, db *bun.DB, studentID int64, date timezone.Date, staffID int64, pickupHHMM, reason string) *schedule.StudentPickupException {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentPickupException{
		StudentID:     studentID,
		ExceptionDate: date,
		CreatedBy:     staffID,
	}
	if pickupHHMM != "" {
		t := parseTimeHHMM(tb, pickupHHMM)
		row.PickupTime = &t
	}
	if reason != "" {
		row.Reason = &reason
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.student_pickup_exceptions`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test pickup exception")
	return row
}

// ActivityInstanceOpts lets tests tweak non-default instance fields without
// bloating the basic helper signature.
type ActivityInstanceOpts struct {
	Status          string // defaults to InstanceStatusPlanned
	ActivityGroupID *int64 // nil = spontaneous instance
	ActiveGroupID   *int64 // set for active/completed instances
	StartHHMM       string // defaults to "14:00"
	EndHHMM         string // defaults to "15:00"
	Title           string // defaults to "Test Instance"
	IsSpontaneous   bool
	// CalendarPeriodID marks the row as MATERIALIZED from a template: the
	// materializer is the only writer that sets it (the manual create path
	// leaves it NULL even when it links an activity group for metadata). Set it
	// whenever a test stands in for a materialized instance — readers such as
	// ActivityInstanceRepository.FindPlannedTemplateBackedFrom use the column to
	// tell an enrollment-derived roster from a hand-typed one (#405 review).
	CalendarPeriodID *int64
}

// CreateTestActivityInstance inserts a schedule.activity_instances row.
// Activity group / active group / status default to a planned template-backed
// instance; override via opts for lifecycle-edge tests.
func CreateTestActivityInstance(tb testing.TB, db *bun.DB, date timezone.Date, roomID int64, opts ActivityInstanceOpts) *schedule.ActivityInstance {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status := opts.Status
	if status == "" {
		status = schedule.InstanceStatusPlanned
	}
	startHHMM := opts.StartHHMM
	if startHHMM == "" {
		startHHMM = "14:00"
	}
	endHHMM := opts.EndHHMM
	if endHHMM == "" {
		endHHMM = "15:00"
	}
	title := opts.Title
	if title == "" {
		title = fmt.Sprintf("Test Instance %d", time.Now().UnixNano())
	}

	row := &schedule.ActivityInstance{
		Date:             date,
		ActivityGroupID:  opts.ActivityGroupID,
		CalendarPeriodID: opts.CalendarPeriodID,
		ActiveGroupID:    opts.ActiveGroupID,
		Title:            title,
		StartTime:        parseTimeHHMM(tb, startHHMM),
		EndTime:          parseTimeHHMM(tb, endHHMM),
		RoomID:           roomID,
		Status:           status,
		IsSpontaneous:    opts.IsSpontaneous,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.activity_instances`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test activity instance")
	return row
}

// CreateTestCalendarPeriod inserts a schedule.calendar_periods row spanning
// [start, end]. The period is created INACTIVE so it cannot collide with the
// active-period invariants other tests rely on; callers that only need an id to
// stamp on a materialized instance want exactly this. Names must be unique per
// tenant — pass a suffixed one. Clean up with CleanupTableRecords(…,
// "schedule.calendar_periods", id).
func CreateTestCalendarPeriod(tb testing.TB, db *bun.DB, name string, start, end timezone.Date) *schedule.CalendarPeriod {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.CalendarPeriod{
		Name:            name,
		PeriodType:      schedule.PeriodTypeCustom,
		StartDate:       start,
		EndDate:         end,
		WeekCycleLength: 1,
		IsActive:        false,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.calendar_periods`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test calendar period")
	return row
}

// StaffShiftOpts controls optional fields for CreateTestStaffShift.
type StaffShiftOpts struct {
	StartHHMM   string // default "08:00"
	EndHHMM     string // default "16:00"
	Notes       string
	Cancelled   bool
	ShiftTypeID *int64
}

// CreateTestStaffShift inserts a Dienstplan shift (schedule.staff_shifts) for
// the staff member on the given date, tenant 1. created_by is stamped with the
// staff's own id. Cleanup: CleanupTableRecords(t, db, "schedule.staff_shifts", row.ID).
func CreateTestStaffShift(tb testing.TB, db *bun.DB, staffID int64, date timezone.Date, opts StaffShiftOpts) *schedule.StaffShift {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startHHMM := opts.StartHHMM
	if startHHMM == "" {
		startHHMM = "08:00"
	}
	endHHMM := opts.EndHHMM
	if endHHMM == "" {
		endHHMM = "16:00"
	}

	row := &schedule.StaffShift{
		StaffID:     staffID,
		Date:        date,
		StartTime:   parseTimeHHMM(tb, startHHMM),
		EndTime:     parseTimeHHMM(tb, endHHMM),
		Notes:       opts.Notes,
		Cancelled:   opts.Cancelled,
		ShiftTypeID: opts.ShiftTypeID,
		CreatedBy:   staffID,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.staff_shifts`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test staff shift")
	return row
}

// InstanceStudentOpts controls optional fields for CreateTestInstanceStudent.
type InstanceStudentOpts struct {
	// NotScheduled sets the #1747 non-booking marker ending a block stamps on
	// children the care plan did not place there that day.
	NotScheduled bool
	// StudentStatusDayID records that a broad day status (sick / excused /
	// class trip) owns this row's outcome, the provenance ApplyStatusDay
	// writes. A manual decision and a check-in both clear it, so a nil value
	// is what separates "a human decided this" from "a day status did".
	StudentStatusDayID *int64
	// ManualStatusAt reproduces an attendance PATCH: a human set this row's
	// status by hand. Completion must leave such a row alone rather than stamp
	// it as a non-booking (#1747).
	ManualStatusAt *time.Time
	// CheckedInAt is the stamp every real check-in path writes. It — not the
	// 'present' status on its own — is what marks a row as an OBSERVED presence
	// (#405 review).
	CheckedInAt *time.Time
}

// CreateTestInstanceStudent inserts one instance_students row. Status defaults
// to AttendanceStatusExpected when empty; opts is optional and only the first
// entry is read.
func CreateTestInstanceStudent(tb testing.TB, db *bun.DB, instanceID, studentID int64, status string, opts ...InstanceStudentOpts) *schedule.InstanceStudent {
	tb.Helper()

	if status == "" {
		status = schedule.AttendanceStatusExpected
	}
	var opt InstanceStudentOpts
	if len(opts) > 0 {
		opt = opts[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.InstanceStudent{
		InstanceID:         instanceID,
		StudentID:          studentID,
		Status:             status,
		NotScheduled:       opt.NotScheduled,
		StudentStatusDayID: opt.StudentStatusDayID,
		ManualStatusAt:     opt.ManualStatusAt,
		CheckedInAt:        opt.CheckedInAt,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.instance_students`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test instance student")
	return row
}

// CreateTestStudentStatusDay inserts one reported broad day status (sick /
// excused / class trip) for a student on a date. Callers that pass its ID into
// InstanceStudentOpts.StudentStatusDayID reproduce the state ApplyStatusDay
// leaves behind: a slot absence the day status owns. Clean up with
// CleanupStudentStatusDays.
func CreateTestStudentStatusDay(tb testing.TB, db *bun.DB, studentID int64, date timezone.Date, status string) *active.StudentStatusDay {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &active.StudentStatusDay{
		StudentID:  studentID,
		Date:       date,
		Status:     status,
		ReportedAt: time.Now(),
		Source:     active.StudentStatusSourceManual,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`active.student_status_days`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test student status day")
	return row
}

// CleanupStudentStatusDays removes status-day rows by ID. Safe to defer.
func CleanupStudentStatusDays(tb testing.TB, db *bun.DB, ids ...int64) {
	tb.Helper()
	CleanupTableRecords(tb, db, "active.student_status_days", ids...)
}

// InstanceStaffOpts controls optional fields for CreateTestInstanceStaff.
type InstanceStaffOpts struct {
	RoomID       *int64
	IsPrimary    bool
	IsSubstitute bool
	IsAbsent     bool
}

// CreateTestInstanceStaff inserts one schedule.instance_staff row. Defaults
// yield a non-primary, non-substitute, non-absent assignment in the instance's
// main room.
func CreateTestInstanceStaff(tb testing.TB, db *bun.DB, instanceID, staffID int64, opts InstanceStaffOpts) *schedule.InstanceStaff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.InstanceStaff{
		InstanceID:   instanceID,
		StaffID:      staffID,
		RoomID:       opts.RoomID,
		IsPrimary:    opts.IsPrimary,
		IsSubstitute: opts.IsSubstitute,
		IsAbsent:     opts.IsAbsent,
	}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.instance_staff`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test instance staff")
	return row
}

// CleanupInstanceStaffFixtures removes instance_staff rows by ID. Callers may
// pass zero IDs (skipped silently).
func CleanupInstanceStaffFixtures(tb testing.TB, db *bun.DB, ids ...int64) {
	tb.Helper()
	nonzero := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			nonzero = append(nonzero, id)
		}
	}
	if len(nonzero) == 0 {
		return
	}
	CleanupTableRecords(tb, db, "schedule.instance_staff", nonzero...)
}

// CleanupScheduleFixturesB11 drops arrival/pickup/instance fixtures by ID.
// Table cleanup order matters: instance_students before activity_instances
// (FK), arrival/pickup exceptions before their student rows. Callers can
// pass zero IDs (skipped silently).
func CleanupScheduleFixturesB11(
	tb testing.TB, db *bun.DB,
	arrivalScheduleIDs, arrivalExceptionIDs, pickupScheduleIDs, pickupExceptionIDs []int64,
	instanceStudentIDs, activityInstanceIDs []int64,
) {
	tb.Helper()

	nonzero := func(ids []int64) []int64 {
		out := make([]int64, 0, len(ids))
		for _, id := range ids {
			if id > 0 {
				out = append(out, id)
			}
		}
		return out
	}

	byTable := []struct {
		table string
		ids   []int64
	}{
		{"schedule.instance_students", nonzero(instanceStudentIDs)},
		{"schedule.activity_instances", nonzero(activityInstanceIDs)},
		{"schedule.student_arrival_exceptions", nonzero(arrivalExceptionIDs)},
		{"schedule.student_arrival_schedules", nonzero(arrivalScheduleIDs)},
		{"schedule.student_pickup_exceptions", nonzero(pickupExceptionIDs)},
		{"schedule.student_pickup_schedules", nonzero(pickupScheduleIDs)},
	}
	for _, g := range byTable {
		if len(g.ids) == 0 {
			continue
		}
		CleanupTableRecords(tb, db, g.table, g.ids...)
	}
}

// ParentChain bundles the IDs of a fully-wired loginable-parent → child
// relationship, mirroring what the guardian-invitation accept flow
// produces: an auth account, a guardian profile linked to it, an active
// account_tenants mapping, and a students_guardians link to a student.
// All rows live in tenant 1 so the parent-portal cross-tenant queries
// resolve.
type ParentChain struct {
	AccountID         int64
	TenantID          int64
	GuardianProfileID int64
	StudentID         int64
	PersonID          int64
	Email             string
}

// CreateTestParentGuardianChain wires the full parent→child chain for
// tenant 1 and returns the IDs. Use CleanupParentGuardianChain (deferred)
// to tear it down.
func CreateTestParentGuardianChain(tb testing.TB, db *bun.DB) ParentChain {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	student := CreateTestStudent(tb, db, "Felix", "Schneider", "1a")
	account := CreateTestAccount(tb, db, "parent")

	profile := &users.GuardianProfile{
		FirstName:              "Sabine",
		LastName:               "Schneider",
		Email:                  &account.Email,
		AccountID:              &account.ID,
		HasAccount:             true,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Exec(ctx)
	require.NoError(tb, err, "Failed to create test guardian profile")

	link := &users.StudentGuardian{
		StudentID:          student.ID,
		GuardianProfileID:  profile.ID,
		RelationshipType:   "parent",
		IsEmergencyContact: true,
		CanPickup:          true,
		EmergencyPriority:  1,
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRolePrimaryGuardian)
	link.IsPrimary = true
	link.SetTenantID(fixtureTenantID(tb))
	_, err = db.NewInsert().Model(link).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(tb, err, "Failed to create students_guardians link")

	now := time.Now()
	mapping := &auth.AccountTenant{
		AccountID:   account.ID,
		TenantID:    fixtureTenantID(tb),
		Status:      auth.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	_, err = db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).Exec(ctx)
	require.NoError(tb, err, "Failed to create account_tenants mapping")

	// Assign the auth guardian role, mirroring production: both
	// guardianInvitationService.linkProfileToAccount and
	// guardianService.ensureGuardianRole grant this role, and parent login
	// (plus reachability checks like FindActivePortalProfilesByIDs) rejects
	// accounts without it. Without this a portal-capable guardian couldn't
	// exist in the real system. The 'guardian' role is seeded by migration
	// 1.7.4, so it always exists in the test DB.
	var guardianRoleID int64
	err = db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", auth.BaseRoleGuardian).
		Scan(ctx, &guardianRoleID)
	require.NoError(tb, err, "Failed to find seeded guardian role")

	roleAssignment := &auth.AccountRole{AccountID: account.ID, RoleID: guardianRoleID}
	roleAssignment.SetTenantID(fixtureTenantID(tb))
	_, err = db.NewInsert().Model(roleAssignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(tb, err, "Failed to assign guardian role")

	return ParentChain{
		AccountID:         account.ID,
		TenantID:          fixtureTenantID(tb),
		GuardianProfileID: profile.ID,
		StudentID:         student.ID,
		PersonID:          student.PersonID,
		Email:             account.Email,
	}
}

// CleanupParentGuardianChain removes every row created by
// CreateTestParentGuardianChain plus any parent notes / status days that
// tests attached to the chain's student. Safe to defer.
func CleanupParentGuardianChain(tb testing.TB, db *bun.DB, c ParentChain) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec := func(query string, arg int64) {
		if _, err := db.ExecContext(ctx, query, arg); err != nil {
			tb.Logf("cleanup warning: %v", err)
		}
	}
	exec(`DELETE FROM users.student_data_change_requests WHERE student_id = ?`, c.StudentID)
	exec(`DELETE FROM users.guardian_phone_numbers WHERE guardian_profile_id = ?`, c.GuardianProfileID)
	// Parent messaging rows FIRST: threads reference users.students and
	// auth.accounts WITHOUT ON DELETE CASCADE, so any thread/message/read a test
	// created on this chain blocks the student/account deletes below with an FK
	// violation (and leaks rows into the shared test DB). Deleting the threads
	// would cascade messages + reads via their thread_id FK, but delete all three
	// explicitly by student so a stray row never survives.
	exec(`DELETE FROM users.parent_message_reads WHERE thread_id IN (SELECT id FROM users.parent_message_threads WHERE student_id = ?)`, c.StudentID)
	exec(`DELETE FROM users.parent_messages WHERE student_id = ?`, c.StudentID)
	exec(`DELETE FROM users.parent_message_threads WHERE student_id = ?`, c.StudentID)
	exec(`DELETE FROM active.student_status_days WHERE student_id = ?`, c.StudentID)
	exec(`DELETE FROM users.students_guardians WHERE student_id = ?`, c.StudentID)
	exec(`DELETE FROM auth.account_roles WHERE account_id = ?`, c.AccountID)
	exec(`DELETE FROM auth.account_tenants WHERE account_id = ?`, c.AccountID)
	exec(`DELETE FROM users.guardian_profiles WHERE id = ?`, c.GuardianProfileID)
	exec(`DELETE FROM users.students WHERE id = ?`, c.StudentID)
	exec(`DELETE FROM users.persons WHERE id = ?`, c.PersonID)
	exec(`DELETE FROM auth.accounts WHERE id = ?`, c.AccountID)
}

// CleanupParentMessagingForAccount removes parent-messaging rows that reference
// an account directly: message reads keyed by account_id and messages sent by
// the account (sender_account_id). Both columns FK auth.accounts(id) WITHOUT ON
// DELETE CASCADE, so a test that sends a message or records a read from a
// SEPARATE staff/reader account (distinct from the guardian chain) must clear
// those rows before deleting that account — otherwise CleanupAuthFixtures hits
// an FK violation and leaks the account into the shared test DB.
//
// CleanupParentGuardianChain already deletes these rows by student_id, but defers
// run LIFO: the staff-account cleanup is registered last and so runs first.
// Register this helper as the LAST defer (it then runs FIRST) so it clears the
// FK ahead of the account delete.
func CleanupParentMessagingForAccount(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec := func(query string, arg int64) {
		if _, err := db.ExecContext(ctx, query, arg); err != nil {
			tb.Logf("cleanup warning: %v", err)
		}
	}
	for _, id := range accountIDs {
		exec(`DELETE FROM users.parent_message_reads WHERE account_id = ?`, id)
		exec(`DELETE FROM users.parent_messages WHERE sender_account_id = ?`, id)
	}
}

// CreateTestEnrollmentPhase creates a minimal active enrollment phase for
// tenant 1 covering the current school year, with cleanup registered.
func CreateTestEnrollmentPhase(tb testing.TB, db *bun.DB) *enrollment.Phase {
	tb.Helper()
	ctx := TenantContext(fixtureTenantID(tb))
	phase := &enrollment.Phase{
		Name:                      fmt.Sprintf("Testphase-%d", time.Now().UnixNano()),
		Kind:                      "school_year",
		ServiceStartDate:          timezone.TodayDate().AddDays(-30),
		ServiceEndDate:            timezone.TodayDate().AddDays(300),
		CareOverflowMode:          "waitlist",
		CareOfferingSelectionMode: "optional",
		IsActive:                  true,
	}
	phase.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(phase).ModelTableExpr(`enrollment.phases AS "phase"`).Returning("*").Exec(ctx)
	if err != nil {
		tb.Fatalf("create test enrollment phase: %v", err)
	}
	tb.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("enrollment.phases").Where("id = ?", phase.ID).Exec(context.Background())
	})
	return phase
}

// CreateTestCareOffering creates a minimal active care offering in the given
// phase (fixed Mo-Fr), with cleanup registered.
func CreateTestCareOffering(tb testing.TB, db *bun.DB, phaseID int64, name string) *enrollment.CareOffering {
	tb.Helper()
	ctx := TenantContext(fixtureTenantID(tb))
	offering := &enrollment.CareOffering{
		PhaseID:            phaseID,
		Name:               fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		DaysOfWeekMode:     enrollment.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon", "tue", "wed", "thu", "fri"},
		AutoAddGradeLevels: []int{},
		IsActive:           true,
		CountsAsCare:       true,
	}
	offering.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(offering).ModelTableExpr(`enrollment.care_offerings AS "care_offering"`).Returning("*").Exec(ctx)
	if err != nil {
		tb.Fatalf("create test care offering: %v", err)
	}
	tb.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("enrollment.care_offerings").Where("id = ?", offering.ID).Exec(context.Background())
	})
	return offering
}

// CreateTestClassListEntry creates a class-list-only entry (#2382) for the
// default test tenant, with cleanup (entry + audit trail) registered.
func CreateTestClassListEntry(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string) *users.ClassListEntry {
	tb.Helper()
	return CreateTestClassListEntryForTenant(tb, db, fixtureTenantID(tb), firstName, lastName, schoolClass)
}

// CreateTestClassListEntryForTenant creates a class-list-only entry (#2382)
// for a specific tenant, with cleanup (entry + audit trail) registered.
func CreateTestClassListEntryForTenant(tb testing.TB, db *bun.DB, tenantID int64, firstName, lastName, schoolClass string) *users.ClassListEntry {
	tb.Helper()
	ctx := TenantContext(tenantID)
	entry := &users.ClassListEntry{
		FirstName:   firstName,
		LastName:    lastName,
		SchoolClass: schoolClass,
	}
	entry.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(entry).ModelTableExpr(`users.class_list_entries AS "class_list_entry"`).Returning("*").Exec(ctx)
	if err != nil {
		tb.Fatalf("create test class list entry: %v", err)
	}
	tb.Cleanup(func() {
		CleanupClassListEntryFixtures(tb, db, entry.ID)
	})
	return entry
}

// CleanupClassListEntryFixtures removes class-list entries and their audit
// trail rows. Safe to call for already-deleted entries.
func CleanupClassListEntryFixtures(tb testing.TB, db *bun.DB, entryIDs ...int64) {
	tb.Helper()
	if len(entryIDs) == 0 {
		return
	}
	ctx := context.Background()
	_, _ = db.NewDelete().TableExpr("audit.class_list_entry_changes").Where("entry_id IN (?)", bun.List(entryIDs)).Exec(ctx)
	_, _ = db.NewDelete().TableExpr("users.class_list_entries").Where("id IN (?)", bun.List(entryIDs)).Exec(ctx)
}
