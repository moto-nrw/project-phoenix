package test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

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

// ============================================================================
// Multi-Tenancy Test Setup
// ============================================================================

// SetupTestOGS creates a unique OGS ID for testing and registers cleanup.
// This is the recommended way to start any test that creates tenant-scoped data.
// The cleanup runs automatically when the test completes.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    db := testpkg.SetupTestDB(t)
//	    ogsID := testpkg.SetupTestOGS(t, db)
//	    // Create fixtures with ogsID - cleanup is automatic
//	    student := testpkg.CreateTestStudent(t, db, "Test", "Student", "1a", ogsID)
//	}
func SetupTestOGS(t testing.TB, db *bun.DB) string {
	t.Helper()
	ogsID := fmt.Sprintf("test-ogs-%s-%d", t.Name(), time.Now().UnixNano())
	t.Cleanup(func() {
		CleanupDataByOGS(t, db, ogsID)
	})
	return ogsID
}

// GenerateTestOGSID creates a unique OGS ID for testing without automatic cleanup.
// Use SetupTestOGS instead for most cases.
func GenerateTestOGSID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// ============================================================================
// Multi-Tenancy Cleanup
// ============================================================================

// CleanupDataByOGS removes all test data for a specific OGS.
// This is called automatically by SetupTestOGS cleanup.
func CleanupDataByOGS(tb testing.TB, db *bun.DB, ogsID string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete in dependency order (children before parents)

	// Schedule domain
	_, _ = db.NewRaw(`DELETE FROM schedule.timeframes WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Active domain
	_, _ = db.NewRaw(`DELETE FROM active.visits WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM active.attendance WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM active.group_supervisors WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM active.groups WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Activities domain
	_, _ = db.NewRaw(`DELETE FROM activities.student_enrollments WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM activities.groups WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM activities.categories WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Education domain
	_, _ = db.NewRaw(`DELETE FROM education.group_substitution WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM education.group_teacher WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM education.groups WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Users domain (cascade-aware order)
	_, _ = db.NewRaw(`DELETE FROM users.privacy_consents WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.persons_guardians WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.guardian_profiles WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.profiles WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.guests WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.teachers WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.students WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.staff WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.rfid_cards WHERE ogs_id = ?`, ogsID).Exec(ctx)
	_, _ = db.NewRaw(`DELETE FROM users.persons WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// IoT domain
	_, _ = db.NewRaw(`DELETE FROM iot.devices WHERE ogs_id = ?`, ogsID).Exec(ctx)

	// Facilities domain
	_, _ = db.NewRaw(`DELETE FROM facilities.rooms WHERE ogs_id = ?`, ogsID).Exec(ctx)
}

// ============================================================================
// RLS Context Helpers
// ============================================================================

// FormatSetRLSQuery formats a SET LOCAL app.ogs_id query.
// This is exported so testutil middleware can use it.
func FormatSetRLSQuery(ogsID string) string {
	return fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", ogsID)
}

// SetRLSContext sets the RLS context for a database connection.
// This simulates what the tenant middleware does for authenticated requests.
// IMPORTANT: Use within a transaction for SET LOCAL to take effect.
func SetRLSContext(ctx context.Context, db bun.IDB, ogsID string) error {
	query := FormatSetRLSQuery(ogsID)
	_, err := db.ExecContext(ctx, query)
	return err
}

// SetRLSContextWithRole sets both the RLS context AND assumes the test_user role.
// This is necessary because the postgres superuser bypasses RLS even with FORCE ROW LEVEL SECURITY.
func SetRLSContextWithRole(ctx context.Context, db bun.IDB, ogsID string) error {
	_, err := db.ExecContext(ctx, "SET LOCAL ROLE test_user")
	if err != nil {
		return fmt.Errorf("failed to set role: %w", err)
	}
	query := fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", ogsID)
	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to set ogs_id: %w", err)
	}
	return nil
}

// ============================================================================
// Legacy Cleanup (ID-based)
// ============================================================================

// cleanupDelete executes a delete query and logs any errors.
// This provides visibility into cleanup failures without causing test failures.
func cleanupDelete(tb testing.TB, query *bun.DeleteQuery, table string) {
	_, err := query.Exec(context.Background())
	if err != nil {
		tb.Logf("cleanup %s: %v", table, err)
	}
}

// CleanupActivityFixtures removes activity-related and education-related test fixtures from the database.
// Pass activity group IDs, device IDs, room IDs, education group IDs, teacher IDs, or any combination.
// This is typically called in a defer statement to ensure cleanup happens.
//
// NOTE: Prefer using SetupTestOGS + CleanupDataByOGS for new tests.
func CleanupActivityFixtures(tb testing.TB, db *bun.DB, ids ...int64) {
	tb.Helper()

	if len(ids) == 0 {
		return
	}

	for _, id := range ids {
		// Education domain cleanup (FK-dependent order)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_substitution").
			Where("group_id = ? OR regular_staff_id = ? OR substitute_staff_id = ?", id, id, id),
			"education.group_substitution")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.group_teacher").
			Where("group_id = ? OR teacher_id = ?", id, id),
			"education.group_teacher")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("id = ? OR staff_id = ?", id, id),
			tableUsersTeachers)

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("education.groups").
			Where(whereIDEquals, id),
			"education.groups")

		// Active domain cleanup
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableActiveVisits).
			Where("id = ? OR student_id = ? OR active_group_id = ?", id, id, id),
			tableActiveVisits)

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableActiveVisits).
			Where("active_group_id IN (SELECT id FROM active.groups WHERE group_id = ?)", id),
			"active.visits (cascade)")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.groups").
			Where("id = ? OR group_id = ? OR device_id = ?", id, id, id),
			"active.groups")

		// Activities domain cleanup
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.student_enrollments").
			Where("activity_group_id = ?", id),
			"activities.student_enrollments")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.groups").
			Where("id = ? OR category_id = ?", id, id),
			"activities.groups")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("activities.categories").
			Where(whereIDEquals, id),
			"activities.categories")

		// IoT domain cleanup
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("iot.devices").
			Where(whereIDEquals, id),
			"iot.devices")

		// Facilities domain cleanup
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("facilities.rooms").
			Where(whereIDEquals, id),
			"facilities.rooms")

		// Users domain cleanup (FK-dependent order)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guests").
			Where("id = ? OR staff_id = ?", id, id),
			"users.guests")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.profiles").
			Where(whereIDOrAccountID, id, id),
			"users.profiles")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.attendance").
			Where("student_id = ?", id),
			"active.attendance")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.students").
			Where(whereIDEquals, id),
			"users.students")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, id),
			tableUsersStaff)

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersPersons).
			Where(whereIDEquals, id),
			tableUsersPersons)

		// Active domain cleanup (continued)
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("active.group_supervisors").
			Where("id = ? OR staff_id = ? OR group_id = ?", id, id, id),
			"active.group_supervisors")

		// Users domain extended cleanup
		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.privacy_consents").
			Where("id = ? OR student_id = ?", id, id),
			"users.privacy_consents")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.persons_guardians").
			Where("id = ? OR person_id = ? OR guardian_account_id = ?", id, id, id),
			"users.persons_guardians")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table("users.guardian_profiles").
			Where(whereIDEquals, id),
			"users.guardian_profiles")

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersRFIDCards).
			Where(whereIDEquals, fmt.Sprintf("%d", id)),
			tableUsersRFIDCards)
	}
}

// CleanupAuthFixtures removes auth account fixtures and their related records.
// Pass account IDs only - this will cascade delete tokens, roles, and permissions.
func CleanupAuthFixtures(tb testing.TB, db *bun.DB, accountIDs ...int64) {
	tb.Helper()

	if len(accountIDs) == 0 {
		return
	}

	cleanupDelete(tb, db.NewDelete().
		Table("auth.tokens").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.tokens")

	cleanupDelete(tb, db.NewDelete().
		Table("auth.account_roles").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.account_roles")

	cleanupDelete(tb, db.NewDelete().
		Table("auth.account_permissions").
		Where(whereAccountIDIn, bun.In(accountIDs)),
		"auth.account_permissions")

	cleanupDelete(tb, db.NewDelete().
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
// Activities Domain Fixtures
// ============================================================================

// CreateTestActivityCategory creates a real activity category in the database
func CreateTestActivityCategory(tb testing.TB, db *bun.DB, name string, ogsID string) *activities.Category {
	tb.Helper()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
	category := &activities.Category{
		Name:  uniqueName,
		Color: "#CCCCCC",
	}
	category.OgsID = ogsID

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
func CreateTestActivityGroup(tb testing.TB, db *bun.DB, name string, ogsID string) *activities.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First create a category (activities.groups.category_id is required)
	category := CreateTestActivityCategory(tb, db, fmt.Sprintf("Category-%s-%d", name, time.Now().UnixNano()), ogsID)

	// Create the activity group
	group := &activities.Group{
		Name:            name,
		MaxParticipants: 20,
		IsOpen:          true,
		CategoryID:      category.ID,
	}
	group.OgsID = ogsID

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`activities.groups AS "group"`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test activity group")

	return group
}

// ============================================================================
// Facilities Domain Fixtures
// ============================================================================

// CreateTestRoom creates a real room in the database
func CreateTestRoom(tb testing.TB, db *bun.DB, name string, ogsID string) *facilities.Room {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	room := &facilities.Room{
		Name:     uniqueName,
		Building: "Test Building",
		Capacity: intPtr(30),
	}
	room.OgsID = ogsID

	err := db.NewInsert().
		Model(room).
		ModelTableExpr(`facilities.rooms`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test room")

	return room
}

// ============================================================================
// IoT Domain Fixtures
// ============================================================================

// CreateTestDevice creates a real IoT device in the database
func CreateTestDevice(tb testing.TB, db *bun.DB, deviceID string, ogsID string) *iot.Device {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, time.Now().UnixNano())

	device := &iot.Device{
		DeviceID:   uniqueDeviceID,
		DeviceType: "rfid_reader",
		Name:       stringPtr("Test Device"),
		Status:     iot.DeviceStatusActive,
		APIKey:     stringPtr("test-api-key-" + uniqueDeviceID),
	}
	device.OgsID = ogsID

	err := db.NewInsert().
		Model(device).
		ModelTableExpr(`iot.devices`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test device")

	return device
}

// ============================================================================
// Users Domain Fixtures
// ============================================================================

// CreateTestPerson creates a real person in the database (required for staff creation)
func CreateTestPerson(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
	}
	person.OgsID = ogsID

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person")

	return person
}

// CreateTestStaff creates a real staff member in the database
// This requires a person, so it creates one automatically
func CreateTestStaff(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first
	person := CreateTestPerson(tb, db, firstName, lastName, ogsID)

	// Create staff record
	staff := &users.Staff{
		PersonID: person.ID,
	}
	staff.OgsID = ogsID

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
func CreateTestStaffForPerson(tb testing.TB, db *bun.DB, personID int64, ogsID string) *users.Staff {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	staff := &users.Staff{
		PersonID: personID,
	}
	staff.OgsID = ogsID

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff for person")

	return staff
}

// CreateTestStudent creates a real student in the database
// This requires a person, so it creates one automatically
func CreateTestStudent(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string, ogsID string) *users.Student {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person first (Student has FK to Person)
	person := CreateTestPerson(tb, db, firstName, lastName, ogsID)

	// Create student record
	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: schoolClass,
	}
	student.OgsID = ogsID

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student")

	return student
}

// ============================================================================
// Active Domain Fixtures
// ============================================================================

// CreateTestAttendance creates a real attendance record in the database
// This requires a student, staff, and device to already exist
// NOTE: active.attendance table does NOT have ogs_id column
func CreateTestAttendance(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, checkInTime time.Time, checkOutTime *time.Time) *active.Attendance {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

// CreateTestActiveGroup creates a real active group (session) in the database.
// This requires an ActivityGroup (activities.groups) and Room to exist.
func CreateTestActiveGroup(tb testing.TB, db *bun.DB, activityGroupID, roomID int64, ogsID string) *active.Group {
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
	activeGroup.OgsID = ogsID

	err := db.NewInsert().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test active group")

	return activeGroup
}

// CreateTestVisit creates a real visit record in the database.
// This requires a Student and ActiveGroup to already exist.
func CreateTestVisit(tb testing.TB, db *bun.DB, studentID, activeGroupID int64, entryTime time.Time, exitTime *time.Time, ogsID string) *active.Visit {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	visit := &active.Visit{
		StudentID:     studentID,
		ActiveGroupID: activeGroupID,
		EntryTime:     entryTime,
		ExitTime:      exitTime,
	}
	visit.OgsID = ogsID

	err := db.NewInsert().
		Model(visit).
		ModelTableExpr(`active.visits`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test visit")

	return visit
}

// CreateTestGroupSupervisor creates a real group supervisor record in the database.
// This requires a Staff and ActiveGroup to already exist.
// NOTE: active.group_supervisors table does NOT have ogs_id column
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

// ============================================================================
// Education Domain Fixtures
// ============================================================================

// CreateTestEducationGroup creates a real education group (Schulklasse) in the database.
// Note: This is different from CreateTestActivityGroup (activities.groups).
func CreateTestEducationGroup(tb testing.TB, db *bun.DB, name string, ogsID string) *education.Group {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uniqueName := fmt.Sprintf("%s-%d", name, time.Now().UnixNano())

	group := &education.Group{
		Name: uniqueName,
	}
	group.OgsID = ogsID

	err := db.NewInsert().
		Model(group).
		ModelTableExpr(`education.groups`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test education group")

	return group
}

// CreateTestTeacher creates a real teacher in the database.
// Teachers require a Staff record, which requires a Person record.
func CreateTestTeacher(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) *users.Teacher {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person)
	staff := CreateTestStaff(tb, db, firstName, lastName, ogsID)

	teacher := &users.Teacher{
		StaffID: staff.ID,
	}
	teacher.OgsID = ogsID

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
// NOTE: education.group_teacher table does NOT have ogs_id column
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

// CreateTestGroupSubstitution creates a teacher substitution record.
// NOTE: education.group_substitution table does NOT have ogs_id column
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

// ============================================================================
// Auth Domain Fixtures (No OgsID - auth is global)
// ============================================================================

// CreateTestAccount creates a real account in the database for authentication testing.
// The email is made unique by appending a timestamp.
// NOTE: Auth accounts don't have OgsID - they are global.
func CreateTestAccount(tb testing.TB, db *bun.DB, email string) *auth.Account {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

// CreateTestRole creates a role in the database for permission testing.
func CreateTestRole(tb testing.TB, db *bun.DB, name string) *auth.Role {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
func CreateTestPermission(tb testing.TB, db *bun.DB, name, resource, action string) *auth.Permission {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
func CreateTestToken(tb testing.TB, db *bun.DB, accountID int64, tokenType string) *auth.Token {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tokenValue := fmt.Sprintf("test-token-%s-%d", tokenType, time.Now().UnixNano())

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

// CreateTestParentAccount creates a parent account in the database.
func CreateTestParentAccount(tb testing.TB, db *bun.DB, email string) *auth.AccountParent {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

// ============================================================================
// Combined Auth + Tenant Fixtures
// ============================================================================

// CreateTestPersonWithAccountID creates a person linked to an existing account ID.
func CreateTestPersonWithAccountID(tb testing.TB, db *bun.DB, firstName, lastName string, accountID int64, ogsID string) *users.Person {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
		AccountID: &accountID,
	}
	person.OgsID = ogsID

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person with account ID")

	return person
}

// CreateTestPersonWithAccount creates a person linked to an account.
// This is needed for policy tests that look up users by account ID.
func CreateTestPersonWithAccount(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) (*users.Person, *auth.Account) {
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
	person.OgsID = ogsID

	err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test person with account")

	return person, account
}

// CreateTestStudentWithAccount creates a student with linked person and account.
func CreateTestStudentWithAccount(tb testing.TB, db *bun.DB, firstName, lastName, schoolClass string, ogsID string) (*users.Student, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person with account
	person, account := CreateTestPersonWithAccount(tb, db, firstName, lastName, ogsID)

	// Create student
	student := &users.Student{
		PersonID:    person.ID,
		SchoolClass: schoolClass,
	}
	student.OgsID = ogsID

	err := db.NewInsert().
		Model(student).
		ModelTableExpr(`users.students`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test student with account")

	return student, account
}

// CreateTestStaffWithAccount creates a staff member with linked person and account.
func CreateTestStaffWithAccount(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) (*users.Staff, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create person with account
	person, account := CreateTestPersonWithAccount(tb, db, firstName, lastName, ogsID)

	// Create staff
	staff := &users.Staff{
		PersonID: person.ID,
	}
	staff.OgsID = ogsID

	err := db.NewInsert().
		Model(staff).
		ModelTableExpr(`users.staff`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test staff with account")

	// Store person reference for convenience
	staff.Person = person

	return staff, account
}

// CreateTestTeacherWithAccount creates a teacher with full chain: Account -> Person -> Staff -> Teacher.
func CreateTestTeacherWithAccount(tb testing.TB, db *bun.DB, firstName, lastName string, ogsID string) (*users.Teacher, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff with account
	staff, account := CreateTestStaffWithAccount(tb, db, firstName, lastName, ogsID)

	// Create teacher
	teacher := &users.Teacher{
		StaffID: staff.ID,
	}
	teacher.OgsID = ogsID

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
func CreateTestStaffWithPIN(tb testing.TB, db *bun.DB, firstName, lastName, pin string, ogsID string) (*users.Staff, *auth.Account) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff with account
	staff, account := CreateTestStaffWithAccount(tb, db, firstName, lastName, ogsID)

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
// Users Domain Extended Fixtures
// ============================================================================

// CreateTestRFIDCard creates an RFID card in the database.
// NOTE: users.rfid_cards table does NOT have ogs_id column
func CreateTestRFIDCard(tb testing.TB, db *bun.DB, tagID string) *users.RFIDCard {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
// NOTE: users.guardian_profiles table does NOT have ogs_id column
func CreateTestGuardianProfile(tb testing.TB, db *bun.DB, email string) *users.GuardianProfile {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

// CreateTestGuest creates a guest instructor in the database.
// Note: Guest model uses base.Model (no OgsID), but we need ogsID for the underlying Staff
func CreateTestGuest(tb testing.TB, db *bun.DB, expertise string, ogsID string) *users.Guest {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create staff first (which creates person) - staff has OgsID
	staff := CreateTestStaff(tb, db, "Guest", "Instructor", ogsID)

	guest := &users.Guest{
		StaffID:           staff.ID,
		ActivityExpertise: expertise,
		Organization:      "Test Organization",
	}
	// Note: Guest uses base.Model, not TenantModel - no OgsID field

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
// Note: Profile model uses base.Model (no OgsID)
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
	// Note: Profile uses base.Model, not TenantModel - no OgsID field

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
// Note: PrivacyConsent model uses base.Model (no OgsID), but we need ogsID for the underlying Student
func CreateTestPrivacyConsent(tb testing.TB, db *bun.DB, prefix string, ogsID string) *users.PrivacyConsent {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create student first - student has OgsID
	student := CreateTestStudent(tb, db, "Consent", prefix, "1a", ogsID)

	now := time.Now()
	expiresAt := now.AddDate(1, 0, 0)
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
	// Note: PrivacyConsent uses base.Model, not TenantModel - no OgsID field

	err := db.NewInsert().
		Model(consent).
		ModelTableExpr(`users.privacy_consents`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test privacy consent")

	// Store student reference for cleanup
	consent.Student = student

	return consent
}

// CreateTestPersonGuardian creates a person-guardian relationship in the database.
// Note: PersonGuardian model uses base.Model (no OgsID)
func CreateTestPersonGuardian(tb testing.TB, db *bun.DB, personID, guardianAccountID int64, relType string) *users.PersonGuardian {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pg := &users.PersonGuardian{
		PersonID:          personID,
		GuardianAccountID: guardianAccountID,
		RelationshipType:  users.RelationshipType(relType),
		IsPrimary:         true,
		Permissions:       "{}",
	}
	// Note: PersonGuardian uses base.Model, not TenantModel - no OgsID field

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
// Note: Timeframe model uses base.Model (no OgsID)
func CreateTestTimeframe(tb testing.TB, db *bun.DB, description string) *schedule.Timeframe {
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
	// Note: Timeframe uses base.Model, not TenantModel - no OgsID field

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
// Legacy Cleanup Functions
// ============================================================================

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
func CleanupStaffFixtures(tb testing.TB, db *bun.DB, staffIDs ...int64) {
	tb.Helper()

	if len(staffIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, staffID := range staffIDs {
		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			TableExpr(tableUsersStaff).
			ColumnExpr("person_id").
			Where(whereIDEquals, staffID).
			Scan(ctx)

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where("staff_id = ?", staffID),
			tableUsersTeachers)

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersStaff).
			Where(whereIDEquals, staffID),
			tableUsersStaff)

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
func CleanupTeacherFixtures(tb testing.TB, db *bun.DB, teacherIDs ...int64) {
	tb.Helper()

	if len(teacherIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, teacherID := range teacherIDs {
		var teacher struct {
			StaffID int64 `bun:"staff_id"`
		}
		_ = db.NewSelect().
			Model(&teacher).
			TableExpr(tableUsersTeachers).
			ColumnExpr("staff_id").
			Where(whereIDEquals, teacherID).
			Scan(ctx)

		var staff struct {
			PersonID int64 `bun:"person_id"`
		}
		_ = db.NewSelect().
			Model(&staff).
			TableExpr(tableUsersStaff).
			ColumnExpr("person_id").
			Where(whereIDEquals, teacher.StaffID).
			Scan(ctx)

		var person struct {
			AccountID *int64 `bun:"account_id"`
		}
		_ = db.NewSelect().
			Model(&person).
			TableExpr(tableUsersPersons).
			ColumnExpr("account_id").
			Where(whereIDEquals, staff.PersonID).
			Scan(ctx)

		cleanupDelete(tb, db.NewDelete().
			Model((*interface{})(nil)).
			Table(tableUsersTeachers).
			Where(whereIDEquals, teacherID),
			tableUsersTeachers)

		if teacher.StaffID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersStaff).
				Where(whereIDEquals, teacher.StaffID),
				tableUsersStaff)
		}

		if staff.PersonID > 0 {
			cleanupDelete(tb, db.NewDelete().
				Model((*interface{})(nil)).
				Table(tableUsersPersons).
				Where(whereIDEquals, staff.PersonID),
				tableUsersPersons)
		}

		if person.AccountID != nil && *person.AccountID > 0 {
			CleanupAuthFixtures(tb, db, *person.AccountID)
		}
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
