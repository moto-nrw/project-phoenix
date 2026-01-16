package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/iot"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// SQL constants to avoid duplication
const (
	whereAccountIDIn    = "account_id IN (?)"
	tableUsersTeachers  = "users.teachers"
	tableUsersStaff     = "users.staff"
	tableUsersPersons   = "users.persons"
	tableActiveVisits   = "active.visits"
	tableUsersRFIDCards = "users.rfid_cards"
)

// cleanupDelete executes a delete query and logs any unexpected errors.
// This provides visibility into cleanup failures without causing test failures.
// Expected errors (like "Model(nil interface)" from BUN) are silently ignored.
func cleanupDelete(tb testing.TB, query *bun.DeleteQuery, table string) {
	_, err := query.Exec(context.Background())
	if err != nil {
		// Filter out expected BUN errors from using nil model
		errStr := err.Error()
		if errStr == "bun: Model(nil interface *interface {})" ||
			errStr == "bun: Model(nil)" {
			return
		}
		tb.Logf("cleanup %s: %v", table, err)
	}
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
// Activity groups require a category, so this helper creates one automatically
func CreateTestActivityGroup(tb testing.TB, db *bun.DB, name string) *activities.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First create a category (activities.groups.category_id is required)
	category := CreateTestActivityCategory(tb, db, fmt.Sprintf("Category-%s-%d", name, time.Now().UnixNano()))

	// Create the activity group
	group := &activities.Group{
		Name:            name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
	}

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
		Capacity: intPtr(30),
	}

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
		DeviceType: "rfid_reader",
		Name:       stringPtr("Test Device"),
		Status:     iot.DeviceStatusActive,
		APIKey:     stringPtr("test-api-key-" + uniqueDeviceID),
	}

	err := db.NewInsert().
		Model(device).
		ModelTableExpr(`iot.devices`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test device")

	return device
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

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff")

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

	// Use today's date in local time (school operates in local timezone)
	// Repository queries use: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	attendance := &active.Attendance{
		StudentID:    studentID,
		Date:         today,
		CheckInTime:  checkInTime,
		CheckOutTime: checkOutTime,
		CheckedInBy:  staffID,
		DeviceID:     deviceID,
	}

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
// Example:
//
//	activity := CreateTestActivityGroup(t, db, "Test")
//	device := CreateTestDevice(t, db, "dev-001")
//	room := CreateTestRoom(t, db, "Room 1")
//	defer CleanupActivityFixtures(t, db, activity.ID, device.ID, room.ID)
func CleanupActivityFixtures(tb testing.TB, db *bun.DB, ids ...int64) {
	tb.Helper()

	if len(ids) == 0 {
		return
	}

	// Batch delete all fixtures matching the IDs
	// This is a simple approach that deletes from any table with these IDs
	// More sophisticated cleanup could track which table each ID belongs to

	for _, id := range ids {
		// Try to delete from each table type
		// Errors are logged but don't fail tests since we don't know which table each ID belongs to

		// ========================================
		// Education domain cleanup (FK-dependent order)
		// ========================================

		// Delete from education.group_substitution (depends on group and staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_substitution").
			Where("group_id = ? OR regular_staff_id = ? OR substitute_staff_id = ?", id, id, id),
			"education.group_substitution")

		// Delete from education.group_teacher (depends on group and teacher)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_teacher").
			Where("group_id = ? OR teacher_id = ?", id, id),
			"education.group_teacher")

		// Delete from users.teachers (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("id = ? OR staff_id = ?", id, id),
			tableUsersTeachers)

		// Delete from education.groups
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.groups").
			Where(whereIDEquals, id),
			"education.groups")

		// ========================================
		// Active domain cleanup
		// ========================================

		// Delete from active.visits by direct ID, by student_id, or by active_group_id
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableActiveVisits).
			Where("id = ? OR student_id = ? OR active_group_id = ?", id, id, id),
			tableActiveVisits)

		// Delete from active.visits (cascade cleanup via activities.groups reference)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableActiveVisits).
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ?)", id),
			"active.visits (cascade)")

		// Delete from active.groups by direct ID or by reference
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.groups").
			Where("id = ? OR group_id = ? OR device_id = ?", id, id, id),
			"active.groups")

		// ========================================
		// Activities domain cleanup
		// ========================================

		// Delete from activities.groups
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where(whereIDEquals, id),
			"activities.groups")

		// Delete from activities.categories
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.categories").
			Where(whereIDEquals, id),
			"activities.categories")

		// ========================================
		// IoT domain cleanup
		// ========================================

		// Delete from iot.devices
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("iot.devices").
			Where(whereIDEquals, id),
			"iot.devices")

		// ========================================
		// Facilities domain cleanup
		// ========================================

		// Delete from facilities.rooms
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("facilities.rooms").
			Where(whereIDEquals, id),
			"facilities.rooms")

		// ========================================
		// Users domain cleanup (FK-dependent order)
		// ========================================

		// Delete from users.guests (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guests").
			Where("id = ? OR staff_id = ?", id, id),
			"users.guests")

		// Delete from users.profiles (depends on account)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.profiles").
			Where(whereIDOrAccountID, id, id),
			"users.profiles")

		// Delete from active.attendance (by student_id before deleting student)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.attendance").
			Where("student_id = ?", id),
			"active.attendance")

		// Delete from users.students
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.students").
			Where(whereIDEquals, id),
			"users.students")

		// Delete from users.staff
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, id),
			tableUsersStaff)

		// Delete from users.persons (last, as it's referenced by students and staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersPersons).
			Where(whereIDEquals, id),
			tableUsersPersons)

		// ========================================
		// Active domain cleanup (continued)
		// ========================================

		// Delete from active.group_supervisors
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.group_supervisors").
			Where("id = ? OR staff_id = ? OR group_id = ?", id, id, id),
			"active.group_supervisors")

		// NOTE: Auth domain cleanup intentionally omitted here.
		// Use CleanupAuthFixtures(accountIDs...) for auth cleanup.
		// Reason: Using generic IDs against auth tables causes cross-domain
		// collisions (e.g., student ID 5 would delete role ID 5).

		// ========================================
		// Users domain extended cleanup
		// ========================================

		// Delete from users.privacy_consents (by student_id)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.privacy_consents").
			Where("id = ? OR student_id = ?", id, id),
			"users.privacy_consents")

		// Delete from users.persons_guardians (by person_id or guardian_account_id)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.persons_guardians").
			Where("id = ? OR person_id = ? OR guardian_account_id = ?", id, id, id),
			"users.persons_guardians")

		// Delete from users.guardian_profiles
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guardian_profiles").
			Where(whereIDEquals, id),
			"users.guardian_profiles")

		// Delete from users.rfid_cards (note: string ID, but try as int64)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersRFIDCards).
			Where(whereIDEquals, fmt.Sprintf("%d", id)),
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
		Model((*any)(nil)).
		Table("auth.tokens").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.tokens")

	// Delete account_roles (by account_id only - never by role_id!)
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_roles").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.account_roles")

	// Delete account_permissions (by account_id only - never by permission_id!)
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_permissions").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.account_permissions")

	// Finally delete the accounts themselves
	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts").
		Where("id IN (?)", bun.In(accountIDs)),
		"auth.accounts")
}

// CleanupParentAccountFixtures removes parent accounts by their IDs.
func CleanupParentAccountFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	cleanupDelete(tb, db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts_parents").
		Where("id IN (?)", bun.In(accountIDs)),
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
			Model((*interface{})(nil)).
			Table(tableUsersRFIDCards).
			Where(whereIDEquals, tagID),
			tableUsersRFIDCards)
	}
}
