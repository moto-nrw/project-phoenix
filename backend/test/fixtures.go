package test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// SQL constants to avoid duplication
const (
	whereIDEquals      = "id = ?"
	whereIDOrAccountID = "id = ? OR account_id = ?"
	tableUsersTeachers = "users.teachers"
	tableUsersStaff    = "users.staff"
	tableUsersPersons  = "users.persons"
)

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
// Note: The Date field is set to today's UTC date (not derived from checkInTime)
// to match the repository's GetStudentCurrentStatus query which always queries
// for today's date. This ensures tests work correctly regardless of when they run.
func CreateTestAttendance(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, checkInTime time.Time, checkOutTime *time.Time) *active.Attendance {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use today's UTC date to match repository query behavior
	// (GetStudentCurrentStatus queries WHERE date = time.Now().UTC().Truncate(24*time.Hour))
	today := time.Now().UTC().Truncate(24 * time.Hour)

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Batch delete all fixtures matching the IDs
	// This is a simple approach that deletes from any table with these IDs
	// More sophisticated cleanup could track which table each ID belongs to

	for _, id := range ids {
		// Try to delete from each table type
		// Ignore errors since we don't know which table each ID belongs to

		// ========================================
		// Education domain cleanup (FK-dependent order)
		// ========================================

		// Delete from education.group_substitution (depends on group and staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_substitution").
			Where("group_id = ? OR regular_staff_id = ? OR substitute_staff_id = ?", id, id, id).
			Exec(ctx)

		// Delete from education.group_teacher (depends on group and teacher)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_teacher").
			Where("group_id = ? OR teacher_id = ?", id, id).
			Exec(ctx)

		// Delete from users.teachers (depends on staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("id = ? OR staff_id = ?", id, id).
			Exec(ctx)

		// Delete from education.groups
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.groups").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Active domain cleanup
		// ========================================

		// Delete from active.visits by direct ID, by student_id, or by active_group_id
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.visits").
			Where("id = ? OR student_id = ? OR active_group_id = ?", id, id, id).
			Exec(ctx)

		// Delete from active.visits (cascade cleanup via activities.groups reference)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.visits").
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ?)", id).
			Exec(ctx)

		// Delete from active.groups by direct ID or by reference
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.groups").
			Where("id = ? OR group_id = ? OR device_id = ?", id, id, id).
			Exec(ctx)

		// ========================================
		// Activities domain cleanup
		// ========================================

		// Delete from activities.groups
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from activities.categories
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.categories").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// IoT domain cleanup
		// ========================================

		// Delete from iot.devices
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("iot.devices").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Facilities domain cleanup
		// ========================================

		// Delete from facilities.rooms
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("facilities.rooms").
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Users domain cleanup (FK-dependent order)
		// ========================================

		// Delete from users.guests (depends on staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guests").
			Where("id = ? OR staff_id = ?", id, id).
			Exec(ctx)

		// Delete from users.profiles (depends on account)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.profiles").
			Where(whereIDOrAccountID, id, id).
			Exec(ctx)

		// Delete from active.attendance (by student_id before deleting student)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.attendance").
			Where("student_id = ?", id).
			Exec(ctx)

		// Delete from users.students
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.students").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from users.staff
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from users.persons (last, as it's referenced by students and staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersPersons).
			Where(whereIDEquals, id).
			Exec(ctx)

		// ========================================
		// Active domain cleanup (continued)
		// ========================================

		// Delete from active.group_supervisors
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.group_supervisors").
			Where("id = ? OR staff_id = ? OR group_id = ?", id, id, id).
			Exec(ctx)

		// NOTE: Auth domain cleanup intentionally omitted here.
		// Use CleanupAuthFixtures(accountIDs...) for auth cleanup.
		// Reason: Using generic IDs against auth tables causes cross-domain
		// collisions (e.g., student ID 5 would delete role ID 5).

		// ========================================
		// Users domain extended cleanup
		// ========================================

		// Delete from users.privacy_consents (by student_id)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.privacy_consents").
			Where("id = ? OR student_id = ?", id, id).
			Exec(ctx)

		// Delete from users.guardian_profiles
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guardian_profiles").
			Where(whereIDEquals, id).
			Exec(ctx)

		// Delete from users.rfid_cards (note: string ID, but try as int64)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.rfid_cards").
			Where(whereIDEquals, fmt.Sprintf("%d", id)).
			Exec(ctx)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use IN clause for efficiency instead of loop
	// Delete tokens first (depends on accounts)
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.tokens").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Delete account_roles (by account_id only - never by role_id!)
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_roles").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Delete account_permissions (by account_id only - never by permission_id!)
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.account_permissions").
		Where("account_id IN (?)", bun.In(accountIDs)).
		Exec(ctx)

	// Finally delete the accounts themselves
	_, _ = db.NewDelete().
		Model((*any)(nil)).
		Table("auth.accounts").
		Where("id IN (?)", bun.In(accountIDs)).
		Exec(ctx)
}

// CleanupRFIDCards removes RFID cards by their string IDs.
func CleanupRFIDCards(tb testing.TB, db *bun.DB, tagIDs ...string) {
	tb.Helper()

	if len(tagIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tagID := range tagIDs {
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.rfid_cards").
			Where(whereIDEquals, tagID).
			Exec(ctx)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = db.NewDelete().
		Model((*interface{})(nil)).
		Table(tableUsersPersons).
		Where(whereIDEquals, personID).
		Exec(ctx)
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
		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			Table(tableUsersStaff).
			Column("person_id").
			Where(whereIDEquals, staffID).
			Scan(ctx)

		// Delete teacher if exists (depends on staff)
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("staff_id = ?", staffID).
			Exec(ctx)

		// Delete staff
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, staffID).
			Exec(ctx)

		// Delete person if we found one
		if staff.PersonID > 0 {
			_, _ = db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID).
				Exec(ctx)
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
		var teacher struct {
			StaffID int64 `bun:"staff_id"`
		}
		_ = db.NewSelect().
			Model(&teacher).
			Table(tableUsersTeachers).
			Column("staff_id").
			Where(whereIDEquals, teacherID).
			Scan(ctx)

		// Get the staff to find the person ID and account ID
		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			Table(tableUsersStaff).
			Column("person_id").
			Where(whereIDEquals, teacher.StaffID).
			Scan(ctx)

		// Get the person to find the account ID
		var person struct {
			AccountID *int64 `bun:"account_id"`
		}
		_ = db.NewSelect().
			Model(&person).
			Table(tableUsersPersons).
			Column("account_id").
			Where(whereIDEquals, staff.PersonID).
			Scan(ctx)

		// Delete teacher
		_, _ = db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where(whereIDEquals, teacherID).
			Exec(ctx)

		// Delete staff
		if teacher.StaffID > 0 {
			_, _ = db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersStaff).
				Where(whereIDEquals, teacher.StaffID).
				Exec(ctx)
		}

		// Delete person
		if staff.PersonID > 0 {
			_, _ = db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID).
				Exec(ctx)
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
	uniqueEmail := fmt.Sprintf("%s-%d@test.local", email, time.Now().UnixNano())

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
	uniqueEmail := fmt.Sprintf("%s-%d@test.local", email, time.Now().UnixNano())

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
