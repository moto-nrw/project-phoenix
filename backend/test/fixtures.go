package test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"golang.org/x/crypto/argon2"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// SQL constants to avoid duplication
const (
	whereIDEquals       = "id = ?"
	whereIDOrAccountID  = "id = ? OR account_id = ?"
	whereAccountIDIn    = "account_id IN (?)"
	tableUsersTeachers  = "users.teachers"
	tableUsersStaff     = "users.staff"
	tableUsersPersons   = "users.persons"
	tableActiveVisits   = "active.visits"
	tableUsersRFIDCards = "users.rfid_cards"
	testEmailFormat     = "%s-%d@test.local"
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

	// Use timezone.Today() for consistent Europe/Berlin timezone handling.
	// This matches the repository queries which also use timezone.Today().
	today := timezone.Today()

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

		// Delete from activities.student_enrollments (depends on activities.groups)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.student_enrollments").
			Where("activity_group_id = ?", id),
			"activities.student_enrollments")

		// Delete from activities.groups by ID or by category_id (to handle FK constraint)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where("id = ? OR category_id = ?", id, id),
			"activities.groups")

		// Delete from activities.categories (now safe after groups referencing them are deleted)
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

	err := db.NewInsert().
		Model(gt).
		ModelTableExpr(`education.group_teacher`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group teacher assignment")

	return gt
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
		GroupID:        activityGroupID,
		RoomID:         roomID,
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
	}

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
		StartDate: time.Now(),
	}

	err := db.NewInsert().
		Model(supervisor).
		ModelTableExpr(`active.group_supervisors`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test group supervisor")

	return supervisor
}

// Helper functions for pointer creation
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// CleanupPerson removes a person from the database by ID.
func CleanupPerson(tb testing.TB, db *bun.DB, personID int64) {
	tb.Helper()

	cleanupDelete(tb, db.NewDelete().
		Model((*interface{})(nil)).
		Table(tableUsersPersons).
		Where(whereIDEquals, personID),
		tableUsersPersons)
}

// CleanupAccount removes an account and related auth records from the database.
func CleanupAccount(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()

	CleanupAuthFixtures(tb, db, accountID)
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
		}
		_ = db.NewSelect().
			Model(&staff).
			TableExpr(tableUsersStaff).
			ColumnExpr("person_id").
			Where(whereIDEquals, staffID).
			Scan(ctx)

		// Delete teacher if exists (depends on staff)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("staff_id = ?", staffID),
			tableUsersTeachers)

		// Delete staff
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, staffID),
			tableUsersStaff)

		// Delete person if we found one
		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
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
			StaffID int64 `bun:"staff_id"`
		}
		_ = db.NewSelect().
			Model(&teacher).
			TableExpr(tableUsersTeachers).
			ColumnExpr("staff_id").
			Where(whereIDEquals, teacherID).
			Scan(ctx)

		// Get the staff to find the person ID and account ID
		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			TableExpr(tableUsersStaff).
			ColumnExpr("person_id").
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

		// Delete teacher
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where(whereIDEquals, teacherID),
			tableUsersTeachers)

		// Delete staff
		if teacher.StaffID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersStaff).
				Where(whereIDEquals, teacher.StaffID),
				tableUsersStaff)
		}

		// Delete person
		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
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

// hashPassword hashes a password using Argon2id (matches auth/userpass)
func hashPassword(password string) (string, error) {
	// Import the userpass package inline to hash the password
	// This uses the same algorithm as the auth service
	params := &argon2Params{
		memory:      64 * 1024,
		iterations:  1,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}

	salt := make([]byte, params.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, params.keyLength)

	// Encode as $argon2id$v=19$m=65536,t=1,p=2$<salt>$<hash>
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.memory, params.iterations, params.parallelism, b64Salt, b64Hash), nil
}

// argon2Params holds parameters for Argon2id hashing
type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
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

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff with account")

	// Store person reference for convenience
	staff.Person = person

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

	err := db.NewInsert().
		Model(teacher).
		ModelTableExpr(`users.teachers`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test teacher with account")

	// Store staff reference for convenience
	teacher.Staff = staff

	return teacher, account
}

// CreateTestStaffWithPIN creates a staff member with account and a hashed PIN.
// This is required for testing PIN validation flows.
func CreateTestStaffWithPIN(tb testing.TB, db *bun.DB, firstName, lastName, pin string) (*users.Staff, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff with account
	staff, account := CreateTestStaffWithAccount(tb, db, firstName, lastName)

	// Hash and set the PIN
	err := account.HashPIN(pin)
	require.NoError(tb, err, "Failed to hash PIN")

	// Update account with PIN hash
	_, err = db.NewUpdate().
		Model(account).
		ModelTableExpr(`auth.accounts`).
		Column("pin_hash").
		Where(whereIDEquals, account.ID).
		Exec(ctx)
	require.NoError(tb, err, "Failed to update account with PIN")

	return staff, account
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make name unique
	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	role := &auth.Role{
		Name:        uniqueName,
		Description: "Test role: " + name,
		IsSystem:    false,
	}

	err := db.NewInsert().
		Model(role).
		ModelTableExpr(`auth.roles`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test role")

	return role
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
func CreateTestGroupSubstitution(tb testing.TB, db *bun.DB, groupID int64, regularStaffID *int64, substituteStaffID int64, startDate, endDate time.Time) *education.GroupSubstitution {
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

	err := db.NewInsert().
		Model(account).
		ModelTableExpr(`auth.accounts_parents`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test parent account")

	return account
}

// CreateTestPersonGuardian creates a person-guardian relationship in the database.
// The guardianAccountID should be a parent account ID (from CreateTestParentAccount).
func CreateTestPersonGuardian(tb testing.TB, db *bun.DB, personID, guardianAccountID int64, relType string) *users.PersonGuardian {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pg := &users.PersonGuardian{
		PersonID:          personID,
		GuardianAccountID: guardianAccountID,
		RelationshipType:  users.RelationshipType(relType),
		IsPrimary:         true,
		Permissions:       "{}", // Valid empty JSON object
	}

	err := db.NewInsert().
		Model(pg).
		ModelTableExpr(`users.persons_guardians`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person guardian relationship")

	return pg
}

// ============================================================================
// Schedule Domain Fixtures
// ============================================================================

// CreateTestTimeframe creates a timeframe in the database.
// This is used for schedule-related tests that need a timeframe reference.
func CreateTestTimeframe(tb testing.TB, db *bun.DB, description string) *schedule.Timeframe {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make description unique
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

	err := db.NewInsert().
		Model(timeframe).
		ModelTableExpr(`schedule.timeframes`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test timeframe")

	return timeframe
}

// CleanupScheduleFixtures removes schedule-related fixtures from the database.
func CleanupScheduleFixtures(tb testing.TB, db *bun.DB, timeframeIDs ...int64) {
	tb.Helper()

	if len(timeframeIDs) == 0 {
		return
	}

	for _, id := range timeframeIDs {
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("schedule.timeframes").
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
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}

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
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}

	if opts != nil {
		invitation.FirstName = opts.FirstName
		invitation.LastName = opts.LastName
	}

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
			Model((*interface{})(nil)).
			Table("auth.invitation_tokens").
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
	role = auth.Role{
		Name:        fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		Description: "Test role for " + name,
		IsSystem:    false,
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

	err := db.NewInsert().
		Model(transition).
		ModelTableExpr(`education.grade_transitions`).
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
		Where("transition_id IN (?)", bun.In(transitionIDs)).
		Exec(ctx)

	// Delete mappings (depends on transition)
	_, _ = db.NewDelete().
		TableExpr("education.grade_transition_mappings").
		Where("transition_id IN (?)", bun.In(transitionIDs)).
		Exec(ctx)

	// Delete transitions
	_, _ = db.NewDelete().
		TableExpr("education.grade_transitions").
		Where("id IN (?)", bun.In(transitionIDs)).
		Exec(ctx)
}
