package test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/enrollment"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestRoomCapacityError returns an opaque admission failure for transport
// contract tests. The room belongs to the calling test's tenant.
func CreateTestRoomCapacityError(tb testing.TB) error {
	tb.Helper()
	room := CreateTestRoom(tb, SetupTestDB(tb), "capacity error")
	return &active.RoomCapacityError{
		RoomID: room.ID, RoomName: room.Name,
		CurrentOccupancy: 43, MaxCapacity: 43,
	}
}

// SQL constants to avoid duplication
const (
	whereIDEquals                 = "id = ?"
	whereIDIn                     = "id IN (?)"
	tableEducationGradeTransition = "education.grade_transitions"
	testEmailFormat               = "%s-%d@test.local"
)

// fixtureSeq counts every unique-suffix request in this binary.
var fixtureSeq int64

// uniqueFixtureSuffix returns a number no two fixtures in this process share.
// The clock alone is not enough: two parallel tests can read the same
// nanosecond, and the collision surfaces as a duplicate-key error on an
// unrelated unique index (idx_accounts_email was the one that found this).
// The counter guarantees uniqueness within the process; the timestamp only
// makes a collision between two processes on one database unlikely, it does
// not rule it out. Each package binary has its own clone, so that second case
// barely arises.
func uniqueFixtureSuffix() int64 {
	return time.Now().UnixNano() + atomic.AddInt64(&fixtureSeq, 1)
}

// UniqueSuffix is uniqueFixtureSuffix for tests that build their own unique
// names (e-mails, usernames, slugs). Reach for it instead of
// time.Now().UnixNano(): two parallel tests can read the same nanosecond, and
// the collision surfaces on whatever unique index the name happens to hit.
func UniqueSuffix() int64 { return uniqueFixtureSuffix() }

// InsertAuditTestWorkSession keeps Audit adapter tests on the neutral fixture
// boundary instead of importing the Bun ORM directly.
func InsertAuditTestWorkSession(ctx context.Context, db *bun.DB, session any) error {
	fixture, ok := session.(interface{ PrepareAuditWorkSession(int64) })
	if !ok {
		return fmt.Errorf("audit work-session fixture preparation is required")
	}
	fixture.PrepareAuditWorkSession(audit.TenantIDFromContext(ctx))
	_, err := db.NewInsert().Model(session).ModelTableExpr(`active.work_sessions AS "work_session"`).Exec(ctx)
	return err
}

// CreateAuditAdjustmentChain creates only the foreign-key chain required by
// enrollment-offering audit rows and returns phase, request, and child IDs.
func CreateAuditAdjustmentChain(tb testing.TB, db *bun.DB) (int64, int64, int64) {
	tb.Helper()
	phaseID := CreateTestEnrollmentPhase(tb, db).ID
	tenantID := fixtureTenantID(tb)
	ctx := TenantContext(tenantID)
	var requestID int64
	err := db.NewRaw(`
		INSERT INTO enrollment.requests (
			tenant_id, phase_id, guardian_first_name, guardian_last_name,
			guardian_email, consent_flags, legal_blocks_snapshot, custom_data,
			source_metadata, status_token, submitted_at
		) VALUES (?, ?, 'Anna', 'Audit', ?, '{}'::jsonb, '[]'::jsonb,
			'{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, tenantID, phaseID,
		fmt.Sprintf("audit-adjustment-%d@example.test", uniqueFixtureSuffix()),
		fmt.Sprintf("audit-adjustment-%d", uniqueFixtureSuffix()),
	).Scan(ctx, &requestID)
	require.NoError(tb, err)
	childID := CreateAuditAdjustmentChild(tb, db, requestID)
	return phaseID, requestID, childID
}

func CreateAuditAdjustmentChild(tb testing.TB, db *bun.DB, requestID int64) int64 {
	tb.Helper()
	tenantID := fixtureTenantID(tb)
	ctx := TenantContext(tenantID)
	var childID int64
	err := db.NewRaw(`
		INSERT INTO enrollment.request_children (
			tenant_id, request_id, first_name, last_name, date_of_birth,
			custom_data, status, activation_mode
		) VALUES (?, ?, 'Lina', ?, DATE '2018-04-15', '{}'::jsonb, 'approved', 'scheduled')
		RETURNING id
	`, tenantID, requestID, fmt.Sprintf("Audit%d", uniqueFixtureSuffix())).Scan(ctx, &childID)
	require.NoError(tb, err)
	return childID
}

func OrganizationIDForSchool(tb testing.TB, db *bun.DB, schoolID int64) int64 {
	tb.Helper()
	var organizationID int64
	require.NoError(tb, db.NewRaw(
		"SELECT organization_id FROM platform.schools WHERE id = ?", schoolID,
	).Scan(context.Background(), &organizationID))
	require.NotZero(tb, organizationID)
	return organizationID
}

// Fixture helpers for hermetic testing. Each helper creates a real database record
// with proper relationships and returns the created entity with its real ID.
// The package clone owns their row lifecycle; tests do not delete them.

// CreateTestActivityCategory creates a real activity category in the database
func CreateTestActivityCategory(tb testing.TB, db *bun.DB, name string) *activities.Category {
	tb.Helper()

	// Make name unique to avoid conflicts with seeded data
	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())
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
	category := CreateTestActivityCategory(tb, db, fmt.Sprintf("Category-%s-%d", name, uniqueFixtureSuffix()))

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
	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())

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
	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, uniqueFixtureSuffix())

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
	return CreateTestAttendanceForDate(tb, db, studentID, staffID, deviceID, timezone.TodayDate(), checkInTime, checkOutTime)
}

// CreateTestAttendanceForDate creates an attendance row on an explicit
// calendar date. Fixed-date tests use this variant so the fixture date cannot
// drift away from their injected service clock.
func CreateTestAttendanceForDate(tb testing.TB, db *bun.DB, studentID, staffID, deviceID int64, date timezone.Date, checkInTime time.Time, checkOutTime *time.Time) *active.Attendance {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attendance := &active.Attendance{
		StudentID:    studentID,
		Date:         date,
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
	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())

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

// ReserveMissingTeacherID advances the real teacher sequence without creating
// a row. It gives missing-row tests a valid, collision-free ID without a
// hardcoded sentinel or create-then-delete arrangement.
func ReserveMissingTeacherID(tb testing.TB, db *bun.DB) int64 {
	tb.Helper()

	var id int64
	err := db.NewSelect().
		ColumnExpr("nextval(pg_get_serial_sequence('users.teachers', 'id'))").
		Scan(Ctx(tb), &id)
	require.NoError(tb, err)
	return id
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

// cleanupRoleRecords removes roles and their role-permission/account-role associations.
func cleanupRoleRecords(tb testing.TB, db *bun.DB, roleIDs ...int64) {
	tb.Helper()
	if len(roleIDs) == 0 {
		return
	}

	ctx := TenantContext(fixtureTenantID(tb))

	_, err := db.NewDelete().
		TableExpr("auth.role_permissions").
		Where("role_id IN (?)", bun.List(roleIDs)).
		Exec(ctx)
	require.NoError(tb, err)

	_, err = db.NewDelete().
		TableExpr("auth.account_roles").
		Where("role_id IN (?)", bun.List(roleIDs)).
		Exec(ctx)
	require.NoError(tb, err)

	_, err = db.NewDelete().
		TableExpr("auth.roles").
		Where("id IN (?)", bun.List(roleIDs)).
		Exec(ctx)
	require.NoError(tb, err)
}

// cleanupPermissionRecords removes permissions and their role/account associations.
func cleanupPermissionRecords(tb testing.TB, db *bun.DB, permissionIDs ...int64) {
	tb.Helper()
	if len(permissionIDs) == 0 {
		return
	}

	ctx := TenantContext(fixtureTenantID(tb))

	_, err := db.NewDelete().
		TableExpr("auth.role_permissions").
		Where("permission_id IN (?)", bun.List(permissionIDs)).
		Exec(ctx)
	require.NoError(tb, err)

	_, err = db.NewDelete().
		TableExpr("auth.account_permissions").
		Where("permission_id IN (?)", bun.List(permissionIDs)).
		Exec(ctx)
	require.NoError(tb, err)

	_, err = db.NewDelete().
		TableExpr("auth.permissions").
		Where("id IN (?)", bun.List(permissionIDs)).
		Exec(ctx)
	require.NoError(tb, err)
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
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())

	account := &auth.Account{
		Email:  uniqueEmail,
		Active: true,
	}

	err := db.NewInsert().
		Model(account).
		ModelTableExpr(`auth.accounts`).
		Scan(ctx)
	require.NoError(tb, err, "Failed to create test account")

	claimAccountForTest(tb, db, account.ID)
	return account
}

// OwnTestAccount claims a surviving account when the test finishes. Service
// and repository tests can exercise a genuinely unmapped account while the
// package gate can still attribute any surviving account-owned rows.
func OwnTestAccount(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()
	tb.Cleanup(func() {
		var exists bool
		err := db.NewSelect().
			ColumnExpr("EXISTS (SELECT 1 FROM auth.accounts WHERE id = ?)", accountID).
			Scan(context.Background(), &exists)
		require.NoError(tb, err)
		if exists {
			EnsureAccountTenant(tb, db, accountID, fixtureTenantID(tb))
		}
	})
}

// OwnTestAccountWithIdentity owns an account whose school identity already
// carries the test tenant. The account mapping is the only missing ownership
// link because identity rows have tenant_id themselves.
func OwnTestAccountWithIdentity(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()
	OwnTestAccount(tb, db, accountID)
}

// OwnTestPasswordResetTokensForEmail registers teardown for reset tokens a
// password-reset service path creates without returning their IDs.
func OwnTestPasswordResetTokensForEmail(tb testing.TB, db *bun.DB, email string) {
	tb.Helper()
	tb.Cleanup(func() {
		_, err := db.NewDelete().
			Table("auth.password_reset_tokens").
			Where("account_id IN (SELECT id FROM auth.accounts WHERE email = ?)", email).
			Exec(context.Background())
		require.NoError(tb, err)
	})
}

// claimAccountForTest maps a fixture account to the tenant of the test that
// created it. An account carries no tenant_id of its own — the link to a
// school lives in auth.account_tenants — so without this row every test
// account is shared, tenant-less state and the leftover gate has to tolerate
// auth.accounts in three quarters of all packages (#2419 goal 2). With it,
// the account belongs to a tenant that dies with the clone, exactly like the
// person, staff and student rows around it.
func claimAccountForTest(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()
	EnsureAccountTenant(tb, db, accountID, fixtureTenantID(tb))
}

// UnclaimTestAccount removes the mapping claimAccountForTest added, giving
// back an account that belongs to no school at all.
//
// For tests whose SUBJECT is the mapping: "an account whose only school is
// deleted disappears", "an account without an active mapping is not
// addressable", "a fresh account is linked nowhere". Everything else wants
// the mapping — it is what makes a test account tenant-owned instead of
// shared state (#2419).
func UnclaimTestAccount(tb testing.TB, db *bun.DB, accountID int64) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx,
		`DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`,
		accountID, fixtureTenantID(tb))
	require.NoError(tb, err, "Failed to unclaim test account")
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

	claimAccountForTest(tb, db, account.ID)
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
// calendar tests wherever staff must be selectable recipients. The added rows
// inherit ownership from the account's test-tenant mapping.
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
	// Upsert: CreateTestAccount already mapped the account to this test's
	// tenant (claimAccountForTest); this call adds the activation timestamp
	// the calendar reachability query looks for.
	_, err := db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).
		On("CONFLICT (account_id, tenant_id) DO UPDATE").
		Set("status = EXCLUDED.status, activated_at = EXCLUDED.activated_at").
		Exec(ctx)
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
	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())

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

	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())

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

	// A system role has tenant_id NULL — shared state for every test in the
	// binary, so it goes away again with the test that made it (#2419).
	tb.Cleanup(func() { cleanupRoleRecords(tb, db, role.ID) })

	return role
}

// AssignLehrkraftSystemRole assigns the seeded lehrkraft system role (#1772)
// to the account, scoped to the given tenant. The role is created by
// migration in every schema the tests run against, so the lookup must
// succeed. The account-role row inherits the account's test ownership.
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
	uniqueSuffix := fmt.Sprintf("%d", uniqueFixtureSuffix())
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

	// auth.permissions is part of the clone-wide RBAC catalog and carries no
	// tenant, so this row is shared state: the fixture takes it back itself
	// (#2419).
	OwnTestPermission(tb, db, permission.ID)

	return permission
}

// OwnTestPermission registers exact-ID teardown for a permission created
// through a service or repository path.
func OwnTestPermission(tb testing.TB, db *bun.DB, permissionID int64) {
	tb.Helper()
	tb.Cleanup(func() { cleanupPermissionRecords(tb, db, permissionID) })
}

// CreateTestToken creates an auth token for testing.
// tokenType can be "access" or "refresh" to set appropriate expiry.
func CreateTestToken(tb testing.TB, db *bun.DB, accountID int64, tokenType string) *auth.Token {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate unique token value
	tokenValue := fmt.Sprintf("test-token-%s-%d", tokenType, uniqueFixtureSuffix())

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
		FamilyID:   fmt.Sprintf("family-%d", uniqueFixtureSuffix()),
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
func CreateTestRFIDCard(tb testing.TB, db *bun.DB, tagID string) *auth.RFIDCard {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Make tag ID unique - use only alphanumeric chars (no hyphens) to match normalization
	uniqueTagID := fmt.Sprintf("%s%d", tagID, uniqueFixtureSuffix())

	card := &auth.RFIDCard{
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
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())

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
		TargetType:        education.GroupSubstitutionTypeGroupHandover,
		GroupID:           groupID,
		RegularStaffID:    regularStaffID,
		SubstituteStaffID: substituteStaffID,
		StartDate:         startDate,
		EndDate:           endDate,
		Reason:            "Test substitution",
	}
	if regularStaffID != nil {
		substitution.TargetType = education.GroupSubstitutionTypeLegacy
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
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())
	username := fmt.Sprintf("parent-%d", uniqueFixtureSuffix())

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
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())
	token := fmt.Sprintf("test-token-%d", uniqueFixtureSuffix())

	invitation := &auth.InvitationToken{
		Email:     uniqueEmail,
		Token:     token,
		RoleID:    roleID,
		ExpiresAt: expiresAt,
	}
	if createdBy > 0 {
		invitation.CreatedBy = Int64Ptr(createdBy)
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
	uniqueEmail := fmt.Sprintf(testEmailFormat, email, uniqueFixtureSuffix())
	token := fmt.Sprintf("test-token-%d", uniqueFixtureSuffix())

	invitation := &auth.InvitationToken{
		Email:     uniqueEmail,
		Token:     token,
		RoleID:    roleID,
		ExpiresAt: expiresAt,
	}
	if createdBy > 0 {
		invitation.CreatedBy = Int64Ptr(createdBy)
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

// GetOrCreateTestRole returns the role called name: this tenant's own if it
// has one, otherwise the system role of that name (the RBAC catalog every
// clone inherits from the template), and it creates a tenant-owned one when
// neither exists.
//
// The order matters (#2419). A created role is now tenant-owned and carries
// the plain name — auth.roles is unique on (tenant_id, name) for tenant
// roles, so no two tests collide, and no test creates a role that another
// test can see. What stays shared is the catalog itself, which tests read and
// no longer delete: the per-row teardowns that used to remove a role fetched
// through this helper are gone, and with them the cascade onto other tests'
// invitation tokens that made TestInvitationService_ValidateInvitation/expired
// fail about one package run in six.
//
// The system role is deliberately preferred over creating a fresh one: the
// callers ask for "admin"/"user"/"teacher" and mean the privilege tier those
// carry. A freshly created stand-in has base_role "user" and fails every
// role-grant check the tests are actually about.
func GetOrCreateTestRole(tb testing.TB, db *bun.DB, name string) *auth.Role {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tenantID := fixtureTenantID(tb)

	// This tenant's own role first, the shared system role second.
	var role auth.Role
	err := db.NewSelect().
		Model(&role).
		ModelTableExpr(`auth.roles AS "role"`).
		Where(`"role".name = ?`, name).
		Where(`"role".tenant_id = ? OR "role".tenant_id IS NULL`, tenantID).
		OrderExpr(`"role".tenant_id NULLS LAST`).
		Limit(1).
		Scan(ctx)

	if err == nil {
		return &role
	}

	// Create a new role if not found
	// base_role is required by the role-create API, so every real custom role
	// carries one; without it the role has no privilege tier and role-grant
	// checks fail it closed.
	baseRole := auth.BaseRoleUser
	role = auth.Role{
		Name:        name,
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

// TestJWTSecret is the fixed secret shared by test routers and token helpers.
// Never use it outside tests.
const TestJWTSecret = "test-jwt-secret-32-chars-minimum"

// GetTestTokenAuth returns a TokenAuth instance for testing.
// A fresh instance avoids shared mutable initialization between parallel tests.
func GetTestTokenAuth(tb testing.TB) *jwt.TokenAuth {
	tb.Helper()

	tokenAuth, err := jwt.NewTokenAuthWithSecret(TestJWTSecret)
	require.NoError(tb, err, "Failed to create test TokenAuth")
	return tokenAuth
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
		ON CONFLICT DO NOTHING`,
		tenantID, fmt.Sprintf("Test Org %d", tenantID), fmt.Sprintf("test-org-%d", tenantID)); err != nil {
		return fmt.Errorf("ensure test organization: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (?, ?, ?, ?, ?, true)
		ON CONFLICT DO NOTHING`,
		tenantID, tenantID,
		fmt.Sprintf("Test School %d", tenantID),
		fmt.Sprintf("test-school-%d", tenantID),
		fmt.Sprintf("t%d", tenantID)); err != nil {
		return fmt.Errorf("ensure test school: %w", err)
	}

	return nil
}

// CreateTestTenant creates an organization + school pair that nobody else
// shares and returns the school id (= tenant id) plus its subdomain. Pair it
// with cleanupTestTenant.
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
// bootstrap advances it past the JWT-safe band, so generated IDs would not
// identify the tenant this helper just created.
func CreateTestTenant(tb testing.TB, db *bun.DB) (tenantID int64, subdomain string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tenantID = uniqueJWTSafeTenantID()
	token := fmt.Sprintf("%d-%d", tenantID, uniqueFixtureSuffix())
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

	return tenantID, subdomain
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

	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())

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

	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())

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

	uniqueDesc := fmt.Sprintf("%s-%d", description, uniqueFixtureSuffix())

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

	uniqueDeviceID := fmt.Sprintf("%s-%d", deviceID, uniqueFixtureSuffix())

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

	tokenValue := fmt.Sprintf("test-token-t%d-%d", tenantID, uniqueFixtureSuffix())

	token := &auth.Token{
		AccountID:  accountID,
		Token:      tokenValue,
		Expiry:     time.Now().Add(24 * time.Hour),
		Mobile:     false,
		FamilyID:   fmt.Sprintf("family-t%d-%d", tenantID, uniqueFixtureSuffix()),
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

	uniqueName := fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix())
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

// OwnTenantRows registers the exceptional row lifecycle needed by tests that
// mutate schema or run tenant-wide migrations inside a shared package clone.
// Ordinary tests rely on clone disposal instead; callers use this only when a
// later test's global constraint would otherwise inspect the earlier rows.
func OwnTenantRows(tb testing.TB, db *bun.DB, tenantIDs ...int64) {
	tb.Helper()
	tb.Cleanup(func() { cleanupTenantTestData(tb, db, tenantIDs...) })
}

func discoverTenantTables(ctx context.Context, db bun.IDB) ([]string, error) {
	var tables []string
	err := db.NewSelect().
		TableExpr("information_schema.columns AS c").
		ColumnExpr("format('%I.%I', c.table_schema, c.table_name)").
		Join("JOIN information_schema.tables AS t ON t.table_schema = c.table_schema AND t.table_name = c.table_name").
		Where("c.column_name = 'tenant_id'").
		Where("c.table_schema NOT IN ('pg_catalog', 'information_schema')").
		Where("t.table_type = 'BASE TABLE'").
		OrderExpr("c.table_schema, c.table_name").
		Scan(ctx, &tables)
	return tables, err
}

func discoverUnscopedAccountTables(ctx context.Context, db bun.IDB) ([]string, error) {
	var tables []string
	err := db.NewSelect().
		TableExpr("information_schema.columns AS c").
		ColumnExpr("format('%I.%I', c.table_schema, c.table_name)").
		Join("JOIN information_schema.tables AS t ON t.table_schema = c.table_schema AND t.table_name = c.table_name").
		Where("c.column_name = 'account_id'").
		Where("c.table_schema NOT IN ('pg_catalog', 'information_schema')").
		Where("t.table_type = 'BASE TABLE'").
		Where("NOT EXISTS (SELECT 1 FROM information_schema.columns tenant_column WHERE tenant_column.table_schema = c.table_schema AND tenant_column.table_name = c.table_name AND tenant_column.column_name = 'tenant_id')").
		OrderExpr("c.table_schema, c.table_name").
		Scan(ctx, &tables)
	return tables, err
}

type unscopedTenantDependent struct {
	Child         string `bun:"child"`
	Parent        string `bun:"parent"`
	JoinPredicate string `bun:"join_predicate"`
}

func discoverUnscopedTenantDependents(ctx context.Context, db bun.IDB) ([]unscopedTenantDependent, error) {
	var dependents []unscopedTenantDependent
	err := db.NewSelect().
		TableExpr("pg_constraint AS c").
		ColumnExpr("format('%I.%I', child_namespace.nspname, child.relname) AS child").
		ColumnExpr("format('%I.%I', parent_namespace.nspname, parent.relname) AS parent").
		ColumnExpr("string_agg(format('child.%I = parent.%I', child_column.attname, parent_column.attname), ' AND ' ORDER BY key.ordinality) AS join_predicate").
		Join("JOIN pg_class AS child ON child.oid = c.conrelid").
		Join("JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace").
		Join("JOIN pg_class AS parent ON parent.oid = c.confrelid").
		Join("JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace").
		Join("JOIN LATERAL unnest(c.conkey, c.confkey) WITH ORDINALITY AS key(child_attnum, parent_attnum, ordinality) ON TRUE").
		Join("JOIN pg_attribute AS child_column ON child_column.attrelid = child.oid AND child_column.attnum = key.child_attnum").
		Join("JOIN pg_attribute AS parent_column ON parent_column.attrelid = parent.oid AND parent_column.attnum = key.parent_attnum").
		Where("c.contype = 'f'").
		Where("NOT EXISTS (SELECT 1 FROM pg_attribute candidate_column WHERE candidate_column.attrelid = child.oid AND candidate_column.attname = 'tenant_id' AND candidate_column.attnum > 0 AND NOT candidate_column.attisdropped)").
		Where("EXISTS (SELECT 1 FROM pg_attribute candidate_column WHERE candidate_column.attrelid = parent.oid AND candidate_column.attname = 'tenant_id' AND candidate_column.attnum > 0 AND NOT candidate_column.attisdropped)").
		GroupExpr("child_namespace.nspname, child.relname, parent_namespace.nspname, parent.relname, c.oid").
		OrderExpr("child_namespace.nspname, child.relname, c.oid").
		Scan(ctx, &dependents)
	return dependents, err
}

// cleanupTenantTestData removes all test data for the specified tenant IDs
// from all tenant-scoped tables in one isolated test-database transaction.
func cleanupTenantTestData(tb testing.TB, db *bun.DB, tenantIDs ...int64) {
	tb.Helper()

	if len(tenantIDs) == 0 {
		return
	}

	// Changed-package runs execute this cross-schema cleanup alongside other
	// database-heavy packages, so allow lock holders to finish under CI load.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var accountIDs []int64
		if err := tx.NewSelect().
			TableExpr("auth.account_tenants").
			Column("account_id").
			Group("account_id").
			Having("bool_and(tenant_id IN (?))", bun.List(tenantIDs)).
			Scan(ctx, &accountIDs); err != nil {
			return err
		}
		var organizationIDs []int64
		if err := tx.NewSelect().
			TableExpr("platform.schools").
			Column("organization_id").
			Where("id IN (?)", bun.List(tenantIDs)).
			Scan(ctx, &organizationIDs); err != nil {
			return err
		}

		// Delete tenantless rows that depend on tenant-owned parents before FK
		// enforcement is suspended. Examples are auth.role_permissions and
		// config.work_time_model_entries.
		dependents, err := discoverUnscopedTenantDependents(ctx, tx)
		if err != nil {
			return err
		}
		for _, dependent := range dependents {
			statement := fmt.Sprintf(
				"DELETE FROM %s AS child USING %s AS parent WHERE parent.tenant_id IN (?) AND %s",
				dependent.Child, dependent.Parent, dependent.JoinPredicate)
			if _, err := tx.NewRaw(statement, bun.List(tenantIDs)).Exec(ctx); err != nil {
				return fmt.Errorf("cleanup %s through %s for tenants %v: %w", dependent.Child, dependent.Parent, tenantIDs, err)
			}
		}

		// Some tenant tables have intentional cyclic FKs. This superuser-only
		// test transaction disables their triggers locally, so the setting is
		// rolled back with the transaction and cannot leak to another test.
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}

		tables, err := discoverTenantTables(ctx, tx)
		if err != nil {
			return err
		}
		for _, table := range tables {
			if _, err := tx.NewDelete().
				TableExpr(table).
				Where("tenant_id IN (?)", bun.List(tenantIDs)).
				Exec(ctx); err != nil {
				return fmt.Errorf("cleanup %s for tenants %v: %w", table, tenantIDs, err)
			}
		}
		if _, err := tx.NewDelete().
			TableExpr("platform.schools").
			Where("id IN (?)", bun.List(tenantIDs)).
			Exec(ctx); err != nil {
			return fmt.Errorf("cleanup platform.schools %v: %w", tenantIDs, err)
		}
		if len(organizationIDs) > 0 {
			if _, err := tx.NewDelete().
				TableExpr("platform.organizations").
				Where("id IN (?)", bun.List(organizationIDs)).
				Where("NOT EXISTS (SELECT 1 FROM platform.schools WHERE organization_id = platform.organizations.id)").
				Exec(ctx); err != nil {
				return fmt.Errorf("cleanup platform.organizations %v: %w", organizationIDs, err)
			}
		}
		if len(accountIDs) == 0 {
			return nil
		}

		accountTables, err := discoverUnscopedAccountTables(ctx, tx)
		if err != nil {
			return err
		}
		for _, table := range accountTables {
			if _, err := tx.NewDelete().
				TableExpr(table).
				Where("account_id IN (?)", bun.List(accountIDs)).
				Exec(ctx); err != nil {
				return fmt.Errorf("cleanup %s for accounts %v: %w", table, accountIDs, err)
			}
		}
		if _, err := tx.NewDelete().
			TableExpr("auth.accounts").
			Where("id IN (?)", bun.List(accountIDs)).
			Exec(ctx); err != nil {
			return fmt.Errorf("cleanup auth.accounts %v: %w", accountIDs, err)
		}
		return nil
	})
	require.NoError(tb, err)
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

// CreateTestArrivalSchedule inserts one care day for a child. Pass
// arrivalHHMM="" for a care day without its own time — the child's class
// timetable supplies it then (#2414).
// staffID must reference users.staff(id) — the schema's created_by FK.
func CreateTestArrivalSchedule(tb testing.TB, db *bun.DB, studentID int64, weekday int, staffID int64, arrivalHHMM string) *schedule.StudentArrivalSchedule {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentArrivalSchedule{
		StudentID: studentID,
		Weekday:   weekday,
		CreatedBy: staffID,
	}
	if arrivalHHMM != "" {
		row.ExpectedArrival = parseTimeHHMM(tb, arrivalHHMM)
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
func CreateTestArrivalException(tb testing.TB, db *bun.DB, studentID int64, date CalendarDate, staffID int64, arrivalHHMM, reason string) *schedule.StudentArrivalException {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentArrivalException{
		StudentID:     studentID,
		ExceptionDate: schedule.Date(date.String()),
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
func CreateTestPickupException(tb testing.TB, db *bun.DB, studentID int64, date CalendarDate, staffID int64, pickupHHMM, reason string) *schedule.StudentPickupException {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.StudentPickupException{
		StudentID:     studentID,
		ExceptionDate: schedule.Date(date.String()),
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

// Date returns a calendar date for callers that should not depend on the
// application's date implementation directly.
func Date(year int, month time.Month, day int) timezone.Date {
	return timezone.NewDate(year, month, day)
}

func ScheduleDate(year int, month time.Month, day int) schedule.Date {
	return schedule.NewDate(year, month, day)
}

type CalendarDate interface {
	String() string
}

// TodayDate returns today's Berlin calendar date for fixture setup.
func TodayDate() timezone.Date {
	return timezone.TodayDate()
}

// WallClock returns a normalized clock-only value (PostgreSQL TIME) for
// callers that should not depend on the application's time implementation
// directly.
func WallClock(hour, minute int) time.Time {
	return timezone.NormalizeWallClock(time.Date(1, time.January, 1, hour, minute, 0, 0, time.UTC))
}

// CreateTestActivityInstance inserts a schedule.activity_instances row.
// Activity group / active group / status default to a planned template-backed
// instance; override via opts for lifecycle-edge tests.
func CreateTestActivityInstance(tb testing.TB, db *bun.DB, date CalendarDate, roomID int64, opts ActivityInstanceOpts) *schedule.ActivityInstance {
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
		title = fmt.Sprintf("Test Instance %d", uniqueFixtureSuffix())
	}

	row := &schedule.ActivityInstance{
		Date:             schedule.Date(date.String()),
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
// tenant — pass a suffixed one. The tenant-owned row dies with the clone.
func CreateTestCalendarPeriod(tb testing.TB, db *bun.DB, name string, start, end CalendarDate) *schedule.CalendarPeriod {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.CalendarPeriod{
		Name:            name,
		PeriodType:      schedule.PeriodTypeCustom,
		StartDate:       schedule.Date(start.String()),
		EndDate:         schedule.Date(end.String()),
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

// SetCalendarPeriodActive flips is_active on a fixture calendar period and
// keeps the in-memory row in sync. Tests use it both to arm the active-period
// invariant on a period from CreateTestCalendarPeriod (created inactive) and
// to deactivate a period mid-test.
func SetCalendarPeriodActive(tb testing.TB, db *bun.DB, period *schedule.CalendarPeriod, active bool) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	period.IsActive = active
	_, err := db.NewUpdate().Model(period).Column("is_active").WherePK().Exec(ctx)
	require.NoError(tb, err, "Failed to set test calendar period active state")
}

// CreateTestClosingDay inserts a schedule.closing_days row spanning
// [start, end] for the test tenant. The tenant-owned row dies with the clone.
func CreateTestClosingDay(tb testing.TB, db *bun.DB, start, end CalendarDate, reason string) *schedule.ClosingDay {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.ClosingDay{StartDate: schedule.Date(start.String()), EndDate: schedule.Date(end.String()), Reason: reason}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.closing_days`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test closing day")
	return row
}

// CreateTestDateframe inserts a schedule.dateframes row for the test tenant.
// Dateframes are instants (TIMESTAMPTZ), so the bounds are passed as such.
func CreateTestDateframe(tb testing.TB, db *bun.DB, name string, start, end time.Time) *schedule.Dateframe {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row := &schedule.Dateframe{Name: name, StartDate: start, EndDate: end}
	row.SetTenantID(fixtureTenantID(tb))

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.dateframes`).
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test dateframe")
	return row
}

// StaffNoticeOpts controls optional fields for NewTestStaffNotice.
type StaffNoticeOpts struct {
	Important               bool // default priority is "info"
	ValidUntil              *timezone.Date
	Inactive                bool
	RequiresAcknowledgement bool
}

// NewTestStaffNotice builds an unsaved Tagesinformation (#2180) with the
// fields a repository test needs: every day of the week, no week pattern,
// active unless opts.Inactive. It does not touch the database — the
// repository under test persists it, so the fixture stays out of that path.
func NewTestStaffNotice(tb testing.TB, title string, validFrom timezone.Date, createdBy int64, opts StaffNoticeOpts) *users.StaffNotice {
	tb.Helper()

	priority := users.StaffNoticePriorityInfo
	if opts.Important {
		priority = users.StaffNoticePriorityImportant
	}
	return &users.StaffNotice{
		Title:                   title,
		Priority:                priority,
		ValidFrom:               validFrom,
		ValidUntil:              opts.ValidUntil,
		Weekdays:                []int16{},
		RequiresAcknowledgement: opts.RequiresAcknowledgement,
		Active:                  !opts.Inactive,
		CreatedBy:               createdBy,
	}
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
// staff's own id. The tenant-owned row dies with the clone.
func CreateTestStaffShift(tb testing.TB, db *bun.DB, staffID int64, date CalendarDate, opts StaffShiftOpts) *schedule.StaffShift {
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
		Date:        schedule.Date(date.String()),
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
// leaves behind: a slot absence the day status owns. The package clone owns it.
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
// tenant 1 and returns the IDs. Its account-to-tenant mapping lets the package
// clone attribute the otherwise tenantless account rows.
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
	// Upsert for the same reason as in CreateTestCalendarStaff: the account
	// fixture already claimed this tenant.
	_, err = db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).
		On("CONFLICT (account_id, tenant_id) DO UPDATE").
		Set("status = EXCLUDED.status, activated_at = EXCLUDED.activated_at").
		Exec(ctx)
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

// CreateTestEnrollmentPhase creates a minimal active enrollment phase for
// the current test tenant covering the current school year.
func CreateTestEnrollmentPhase(tb testing.TB, db *bun.DB) *enrollmentOwner.Phase {
	tb.Helper()
	return createTestEnrollmentPhase(tb, db, nil)
}

// CreateTestEnrollmentPhaseForCalendarPeriod creates a phase linked to a real planning period.
func CreateTestEnrollmentPhaseForCalendarPeriod(tb testing.TB, db *bun.DB, periodID int64) *enrollmentOwner.Phase {
	tb.Helper()
	return createTestEnrollmentPhase(tb, db, &periodID)
}

func createTestEnrollmentPhase(tb testing.TB, db *bun.DB, periodID *int64) *enrollmentOwner.Phase {
	tb.Helper()
	ctx := WithTenantRuntime(tb, TenantContext(fixtureTenantID(tb)), db)
	phase := &enrollmentOwner.Phase{
		Name:                      fmt.Sprintf("Testphase-%d", uniqueFixtureSuffix()),
		Kind:                      "school_year",
		ServiceStartDate:          enrollmentOwner.Date(timezone.TodayDate().AddDays(-30)),
		ServiceEndDate:            enrollmentOwner.Date(timezone.TodayDate().AddDays(300)),
		CareOverflowMode:          "waitlist",
		CareOfferingSelectionMode: "optional",
		CalendarPeriodID:          periodID,
		IsActive:                  true,
	}
	err := enrollmentOwner.New().InsertPhase(ctx, phase)
	if err != nil {
		tb.Fatalf("create test enrollment phase: %v", err)
	}
	return phase
}

// CreateTestCareOffering creates a minimal active care offering in the given
// phase (fixed Mo-Fr).
func CreateTestCareOffering(tb testing.TB, db *bun.DB, phaseID int64, name string) *enrollment.CareOffering {
	tb.Helper()
	ctx := TenantContext(fixtureTenantID(tb))
	offering := &enrollment.CareOffering{
		PhaseID:            phaseID,
		Name:               fmt.Sprintf("%s-%d", name, uniqueFixtureSuffix()),
		DaysOfWeekMode:     enrollment.DaysOfWeekModeFixed,
		AvailableDays:      []string{"mon", "tue", "wed", "thu", "fri"},
		AutoAddGradeLevels: []int{},
		IsActive:           true,
		CountsAsCare:       true,
	}
	offering.TenantID = fixtureTenantID(tb)
	InsertTestCareOffering(tb, db, ctx, offering)
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
		cleanupClassListEntryFixtures(tb, db, entry.ID)
	})
	return entry
}

// cleanupClassListEntryFixtures removes class-list entries and their audit
// trail rows. Safe to call for already-deleted entries.
func cleanupClassListEntryFixtures(tb testing.TB, db *bun.DB, entryIDs ...int64) {
	tb.Helper()
	if len(entryIDs) == 0 {
		return
	}
	ctx := context.Background()
	_, err := db.NewDelete().TableExpr("audit.class_list_entry_changes").Where("entry_id IN (?)", bun.List(entryIDs)).Exec(ctx)
	require.NoError(tb, err)
	_, err = db.NewDelete().TableExpr("users.class_list_entries").Where("id IN (?)", bun.List(entryIDs)).Exec(ctx)
	require.NoError(tb, err)
}

// CreateTestCoGuardianForStudent adds a SECOND portal guardian to a child that
// already has one — the shape every "what does the other parent see" test
// needs (request sharing, Familienschutz, the co-guardian notice #2267).
//
// The returned chain names the new guardian; StudentID and TenantID are the
// child's, so a caller can compare the two guardians' views of one child.
func CreateTestCoGuardianForStudent(
	tb testing.TB, db *bun.DB, studentID int64, firstName, lastName string,
) ParentChain {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	account := CreateTestAccount(tb, db, "co-guardian")
	profile := &users.GuardianProfile{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  &account.Email,
		AccountID:              &account.ID,
		HasAccount:             true,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Exec(ctx)
	require.NoError(tb, err, "Failed to create co-guardian profile")

	link := &users.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  profile.ID,
		RelationshipType:   "parent",
		IsEmergencyContact: false,
		CanPickup:          true,
		EmergencyPriority:  2,
	}
	// A co-guardian, not the primary one: same portal access, no primacy.
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleCoGuardian)
	link.SetTenantID(fixtureTenantID(tb))
	_, err = db.NewInsert().Model(link).ModelTableExpr(`users.students_guardians`).Exec(ctx)
	require.NoError(tb, err, "Failed to link co-guardian to student")

	now := time.Now()
	mapping := &auth.AccountTenant{
		AccountID:   account.ID,
		TenantID:    fixtureTenantID(tb),
		Status:      auth.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	_, err = db.NewInsert().Model(mapping).ModelTableExpr(`auth.account_tenants`).
		On("CONFLICT (account_id, tenant_id) DO UPDATE").
		Set("status = EXCLUDED.status, activated_at = EXCLUDED.activated_at").
		Exec(ctx)
	require.NoError(tb, err, "Failed to create co-guardian account_tenants mapping")

	var guardianRoleID int64
	err = db.NewSelect().ColumnExpr("id").TableExpr("auth.roles").
		Where("name = ?", auth.BaseRoleGuardian).Scan(ctx, &guardianRoleID)
	require.NoError(tb, err, "Failed to find seeded guardian role")
	roleAssignment := &auth.AccountRole{AccountID: account.ID, RoleID: guardianRoleID}
	roleAssignment.SetTenantID(fixtureTenantID(tb))
	_, err = db.NewInsert().Model(roleAssignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(tb, err, "Failed to assign guardian role to co-guardian")

	return ParentChain{
		AccountID:         account.ID,
		TenantID:          fixtureTenantID(tb),
		GuardianProfileID: profile.ID,
		StudentID:         studentID,
		Email:             account.Email,
	}
}

// CreateTestParentAnnouncement creates a school-wide Elternmitteilung owned by
// the test's tenant, so tests that hang something off an announcement (its
// attachments, #2890) have a real row behind the composite foreign key.
//
// It stays a draft: publishing is a service concern, and the callers that need
// a live announcement publish it themselves.
func CreateTestParentAnnouncement(tb testing.TB, db *bun.DB, createdBy int64, title string) *users.ParentAnnouncement {
	tb.Helper()
	ctx := TenantContext(fixtureTenantID(tb))
	announcement := &users.ParentAnnouncement{
		Title:     fmt.Sprintf("%s-%d", title, uniqueFixtureSuffix()),
		Body:      "Testinhalt.",
		Priority:  users.ParentAnnouncementPriorityInfo,
		Active:    true,
		CreatedBy: createdBy,
	}
	announcement.SetTenantID(fixtureTenantID(tb))
	_, err := db.NewInsert().Model(announcement).
		ModelTableExpr(`users.parent_announcements AS "parent_announcement"`).
		Returning("*").Exec(ctx)
	if err != nil {
		tb.Fatalf("create test parent announcement: %v", err)
	}
	return announcement
}

// PublishTestParentAnnouncement stamps an announcement as published now, so
// tests can exercise the rules that only apply once it is out — above all that
// a published announcement is immutable (#2890).
func PublishTestParentAnnouncement(tb testing.TB, db *bun.DB, announcementID int64) {
	tb.Helper()
	ctx := TenantContext(fixtureTenantID(tb))
	_, err := db.NewUpdate().
		TableExpr("users.parent_announcements").
		Set("published_at = NOW()").
		Where("id = ?", announcementID).
		Exec(ctx)
	if err != nil {
		tb.Fatalf("publish test parent announcement: %v", err)
	}
}
