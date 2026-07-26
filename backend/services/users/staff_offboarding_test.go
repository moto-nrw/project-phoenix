package users_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	authSvcPkg "github.com/moto-nrw/project-phoenix/services/auth"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// offboardingFixtureTenant is the default tenant the shared fixtures
// (CreateTestStaff etc.) hardcode internally.
var offboardingFixtureTenant int64 = 1

func offboardingCredential(prefix string, suffix string) string {
	return prefix + suffix + "!"
}

type offboardingScenario struct {
	db      *bun.DB
	repos   *repositories.Factory
	authSvc authSvcPkg.AuthService
	svc     usersSvc.StaffOffboardingService
	ctx     context.Context
}

func newOffboardingScenario(t *testing.T) *offboardingScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)

	authCfg, err := authSvcPkg.NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	authService, err := authSvcPkg.NewService(repos, authCfg, db, nil)
	require.NoError(t, err)

	svc := usersSvc.NewStaffOffboardingService(usersSvc.StaffOffboardingServiceDependencies{
		PersonRepo:             repos.Person,
		StaffRepo:              repos.Staff,
		TeacherRepo:            repos.Teacher,
		GroupSupervisorRepo:    repos.GroupSupervisor,
		GroupTeacherRepo:       repos.GroupTeacher,
		GroupSubstitutionRepo:  repos.GroupSubstitution,
		ActivitySupervisorRepo: repos.ActivitySupervisor,
		InstanceStaffRepo:      repos.InstanceStaff,
		StaffShiftRepo:         repos.StaffShift,
		StaffAbsenceRepo:       repos.StaffAbsence,
		AccountTenantRepo:      repos.AccountTenant,
		RoleRepo:               repos.Role,
		AccountPermissionRepo:  repos.AccountPermission,
		DataDeletionRepo:       repos.DataDeletion,
		AuthService:            authService,
		DB:                     db,
	})

	testpkg.EnsureTestTenant(t, db, offboardingFixtureTenant)

	return &offboardingScenario{
		db:      db,
		repos:   repos,
		authSvc: authService,
		svc:     svc,
		ctx:     testpkg.TenantContext(offboardingFixtureTenant),
	}
}

// assignTenantRole links the account to a role inside the fixture tenant.
func assignTenantRole(t *testing.T, db *bun.DB, accountID int64, roleID int64) {
	t.Helper()
	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    roleID,
	}
	accountRole.SetTenantID(offboardingFixtureTenant)
	err := db.NewInsert().
		Model(accountRole).
		ModelTableExpr(`auth.account_roles`).
		Scan(testpkg.TenantContext(offboardingFixtureTenant))
	require.NoError(t, err)
}

func cleanupOffboardedStaffChain(t *testing.T, db *bun.DB, staffID, personID int64, accountID *int64) {
	t.Helper()
	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM audit.data_deletions WHERE staff_id = ?`, staffID)
	_, _ = db.ExecContext(ctx, `DELETE FROM users.teachers WHERE staff_id = ?`, staffID)
	_, _ = db.ExecContext(ctx, `DELETE FROM users.staff WHERE id = ?`, staffID)
	_, _ = db.ExecContext(ctx, `DELETE FROM users.persons WHERE id = ?`, personID)
	if accountID != nil {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.tokens WHERE account_id = ?`, *accountID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ?`, *accountID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, *accountID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users.persons WHERE account_id = ?`, *accountID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, *accountID)
	}
}

// TestOffboardStaff_WithAttendanceHistory is the headline regression for
// issue #695: staff with attendance records could never be deleted because of
// the ON DELETE RESTRICT FK. With soft delete the offboarding must succeed and
// the attendance history must survive untouched.
func TestOffboardStaff_WithAttendanceHistory(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, sc.db, "Attendance", "History")
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)
	student := testpkg.CreateTestStudent(t, sc.db, "Offboard", "Student", "1a")
	device := testpkg.CreateTestDevice(t, sc.db, fmt.Sprintf("offb-dev-%d", time.Now().UnixNano()))
	attendance := testpkg.CreateTestAttendance(t, sc.db, student.ID, staff.ID, device.ID, time.Now(), nil)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM active.attendance WHERE id = ?`, attendance.ID)
		testpkg.CleanupActivityFixtures(t, sc.db, student.ID, device.ID)
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, &account.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	var attendanceCount int
	err := sc.db.NewSelect().
		TableExpr(`active.attendance`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ? AND checked_in_by = ?`, attendance.ID, staff.ID).
		Scan(context.Background(), &attendanceCount)
	require.NoError(t, err)
	assert.Equal(t, 1, attendanceCount, "attendance history must survive offboarding")

	var deletedAt *time.Time
	err = sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &deletedAt)
	require.NoError(t, err)
	assert.NotNil(t, deletedAt, "staff row must be soft-deleted, not hard-deleted")

	var auditCount int
	err = sc.db.NewSelect().
		TableExpr(`audit.data_deletions`).
		ColumnExpr(`COUNT(*)`).
		Where(`staff_id = ? AND deletion_type = 'manual' AND deleted_by = 'test-admin'`, staff.ID).
		Scan(context.Background(), &auditCount)
	require.NoError(t, err)
	assert.Equal(t, 1, auditCount, "offboarding must write an audit.data_deletions record")
}

// TestOffboardStaff_RevokesAccountAccess covers bug 1 of issue #695: after
// deletion the Betreuer must no longer be able to log in.
func TestOffboardStaff_RevokesAccountAccess(t *testing.T) {
	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "123")
	emailAddr := fmt.Sprintf("offboard-login-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Off", "Boarded", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)
	role := testpkg.GetOrCreateTestRole(t, sc.db, "user")
	assignTenantRole(t, sc.db, account.ID, role.ID)
	testpkg.CreateTestToken(t, sc.db, account.ID, "refresh")

	t.Cleanup(func() {
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, person.ID, &account.ID)
	})

	_, _, err := sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.NoError(t, err, "login must work before offboarding")

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	exists, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, offboardingFixtureTenant)
	require.NoError(t, err)
	assert.False(t, exists, "account-tenant mapping must be inactive after offboarding")

	var active bool
	err = sc.db.NewSelect().
		TableExpr(`auth.accounts`).
		ColumnExpr(`active`).
		Where(`id = ?`, account.ID).
		Scan(context.Background(), &active)
	require.NoError(t, err)
	assert.False(t, active, "single-tenant account must be deactivated")

	var tokenCount int
	err = sc.db.NewSelect().
		TableExpr(`auth.tokens`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ?`, account.ID).
		Scan(context.Background(), &tokenCount)
	require.NoError(t, err)
	assert.Zero(t, tokenCount, "refresh tokens must be revoked")

	var personAccountID *int64
	err = sc.db.NewSelect().
		TableExpr(`users.persons`).
		ColumnExpr(`account_id`).
		Where(`id = ?`, person.ID).
		Scan(context.Background(), &personAccountID)
	require.NoError(t, err)
	assert.Nil(t, personAccountID, "person must be unlinked from the account")

	var roleCount int
	err = sc.db.NewSelect().
		TableExpr(`auth.account_roles`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ?`, account.ID).
		Scan(context.Background(), &roleCount)
	require.NoError(t, err)
	assert.Zero(t, roleCount, "tenant roles must be removed")

	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.Error(t, err, "login must fail after offboarding")
}

// TestOffboardStaff_MultiTenantAccountKeepsOtherSchool: offboarding at school A
// must not lock the account out of school B.
func TestOffboardStaff_MultiTenantAccountKeepsOtherSchool(t *testing.T) {
	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "123")
	emailAddr := fmt.Sprintf("offboard-multi-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Multi", "Tenant", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)

	otherTenant := account.ID + 80000
	testpkg.EnsureTestTenant(t, sc.db, otherTenant)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, otherTenant)

	t.Cleanup(func() {
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, person.ID, &account.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	var active bool
	err := sc.db.NewSelect().
		TableExpr(`auth.accounts`).
		ColumnExpr(`active`).
		Where(`id = ?`, account.ID).
		Scan(context.Background(), &active)
	require.NoError(t, err)
	assert.True(t, active, "account with another active school mapping must stay active")

	existsA, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, offboardingFixtureTenant)
	require.NoError(t, err)
	assert.False(t, existsA, "offboarded school mapping must be inactive")

	existsB, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, otherTenant)
	require.NoError(t, err)
	assert.True(t, existsB, "other school mapping must stay active")

	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.NoError(t, err, "login at the other school must keep working")
}

// TestOffboardStaff_ReinviteSameEmailSameSchool covers bug 2 of issue #695:
// the same email must be re-invitable at the same school after offboarding,
// and acceptance must restore a fully working Betreuer on the same account.
func TestOffboardStaff_ReinviteSameEmailSameSchool(t *testing.T) {
	sc := newOffboardingScenario(t)

	invSvc := authSvcPkg.NewInvitationService(authSvcPkg.InvitationServiceConfig{
		InvitationRepo:    sc.repos.InvitationToken,
		AccountRepo:       sc.repos.Account,
		AccountTenantRepo: sc.repos.AccountTenant,
		RoleRepo:          sc.repos.Role,
		AccountRoleRepo:   sc.repos.AccountRole,
		PersonRepo:        sc.repos.Person,
		StaffRepo:         sc.repos.Staff,
		TeacherRepo:       sc.repos.Teacher,
		SchoolRepo:        sc.repos.School,
		Mailer:            email.NewMockMailer(),
		FrontendURL:       "http://localhost:3000",
		InvitationExpiry:  time.Hour,
		DB:                sc.db,
	})

	oldCredential := offboardingCredential("Offboard", "123")
	newCredential := offboardingCredential("Reinvited", "456")
	emailAddr := fmt.Sprintf("offboard-reinvite-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, oldCredential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Re", "Invited", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)
	role := testpkg.CreateTestSystemRole(t, sc.db, "user")

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM auth.invitation_tokens WHERE email = ?`, emailAddr)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM users.staff WHERE person_id IN (SELECT id FROM users.persons WHERE account_id = ?)`, account.ID)
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, person.ID, &account.ID)
		testpkg.CleanupTableRecords(t, sc.db, "auth.roles", role.ID)
	})

	// Before offboarding the re-invite is blocked (current production behavior).
	_, err := invSvc.CreateInvitation(sc.ctx, authSvcPkg.InvitationRequest{
		Email:     emailAddr,
		RoleID:    role.ID,
		TenantID:  offboardingFixtureTenant,
		CreatedBy: account.ID,
	})
	require.Error(t, err, "re-invite must be blocked while the mapping is active")
	require.ErrorIs(t, err, authSvcPkg.ErrAccountAlreadyHasTenantAccess)

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	invitation, err := invSvc.CreateInvitation(sc.ctx, authSvcPkg.InvitationRequest{
		Email:     emailAddr,
		RoleID:    role.ID,
		TenantID:  offboardingFixtureTenant,
		CreatedBy: account.ID,
	})
	require.NoError(t, err, "re-invite must succeed after offboarding")

	reactivated, err := invSvc.AcceptInvitation(context.Background(), invitation.Token, authSvcPkg.UserRegistrationData{
		FirstName:       "Re",
		LastName:        "Invited",
		Password:        newCredential,
		ConfirmPassword: newCredential,
	})
	require.NoError(t, err, "accepting the re-invitation must succeed")
	assert.Equal(t, account.ID, reactivated.ID, "the existing account must be re-attached, not duplicated")

	exists, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, offboardingFixtureTenant)
	require.NoError(t, err)
	assert.True(t, exists, "the tenant mapping must be reactivated")

	var staffCount int
	err = sc.db.NewSelect().
		TableExpr(`users.staff AS "staff"`).
		ColumnExpr(`COUNT(*)`).
		Join(`JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		Where(`"person".account_id = ? AND "staff".deleted_at IS NULL AND "person".deleted_at IS NULL`, account.ID).
		Scan(context.Background(), &staffCount)
	require.NoError(t, err)
	assert.Equal(t, 1, staffCount, "acceptance must recreate a live staff record")

	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, newCredential)
	require.NoError(t, err, "login with the new password must work after re-invitation")
}

// TestOffboardStaff_ActiveSupervisionBlocks: an active room supervision still
// blocks offboarding (no FK safety net remains, the pre-check is the guard).
func TestOffboardStaff_ActiveSupervisionBlocks(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Supervising", "Staff")
	activityGroup := testpkg.CreateTestActivityGroup(t, sc.db, "OffboardSupervision")
	room := testpkg.CreateTestRoom(t, sc.db, fmt.Sprintf("OffbRoom-%d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, sc.db, activityGroup.ID, room.ID)
	supervisor := testpkg.CreateTestGroupSupervisor(t, sc.db, staff.ID, activeGroup.ID, "supervisor")

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM active.group_supervisors WHERE id = ?`, supervisor.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM active.groups WHERE id = ?`, activeGroup.ID)
		testpkg.CleanupTableRecords(t, sc.db, "activities.groups", activityGroup.ID)
		testpkg.CleanupActivityFixtures(t, sc.db, room.ID, activityGroup.CategoryID)
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, nil)
	})

	err := sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin")
	require.Error(t, err)
	assert.True(t, errors.Is(err, usersSvc.ErrStaffInUse), "active supervision must block offboarding")

	var deletedAt *time.Time
	scanErr := sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &deletedAt)
	require.NoError(t, scanErr)
	assert.Nil(t, deletedAt, "blocked offboarding must not soft-delete the staff row")
}

// TestOffboardStaff_CleansUpAssignments: planned assignments the old
// ON DELETE CASCADE used to remove must be cleaned up explicitly.
func TestOffboardStaff_CleansUpAssignments(t *testing.T) {
	sc := newOffboardingScenario(t)

	teacher := testpkg.CreateTestTeacher(t, sc.db, "Assigned", "Teacher")
	staffID := teacher.StaffID
	educationGroup := testpkg.CreateTestEducationGroup(t, sc.db, "OffboardGroup")
	groupTeacher := testpkg.CreateTestGroupTeacher(t, sc.db, educationGroup.ID, teacher.ID)

	otherStaff := testpkg.CreateTestStaff(t, sc.db, "Other", "Substitute")
	today := timezone.TodayDate()
	substitution := testpkg.CreateTestGroupSubstitution(t, sc.db, educationGroup.ID, nil, staffID,
		today.AddDays(1), today.AddDays(8))

	var personID int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`person_id`).
		Where(`id = ?`, staffID).
		Scan(context.Background(), &personID))

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_substitution WHERE id = ?`, substitution.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.group_teacher WHERE id = ?`, groupTeacher.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM education.groups WHERE id = ?`, educationGroup.ID)
		cleanupOffboardedStaffChain(t, sc.db, otherStaff.ID, otherStaff.PersonID, nil)
		cleanupOffboardedStaffChain(t, sc.db, staffID, personID, nil)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staffID, "test-admin"))

	var gtCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`education.group_teacher`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ?`, groupTeacher.ID).
		Scan(context.Background(), &gtCount))
	assert.Zero(t, gtCount, "group-teacher assignment must be removed")

	var subCount int
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`education.group_substitution`).
		ColumnExpr(`COUNT(*)`).
		Where(`id = ?`, substitution.ID).
		Scan(context.Background(), &subCount))
	assert.Zero(t, subCount, "future substitution must be removed")

	var teacherDeletedAt *time.Time
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.teachers`).
		ColumnExpr(`deleted_at`).
		Where(`id = ?`, teacher.ID).
		Scan(context.Background(), &teacherDeletedAt))
	assert.NotNil(t, teacherDeletedAt, "teacher row must be soft-deleted")
}

// TestOffboardStaff_Idempotent: deleting a non-existent staff member stays a
// no-op so the HTTP handler keeps returning 200.
func TestOffboardStaff_Idempotent(t *testing.T) {
	sc := newOffboardingScenario(t)
	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, 999999, "test-admin"))
}

// TestOffboardStaff_ExcludedFromListsAndPIN: offboarded staff must vanish from
// operational queries.
func TestOffboardStaff_ExcludedFromListsAndPIN(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, sc.db, "Listed", "Staff")
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)

	t.Cleanup(func() {
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, &account.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	listed, err := sc.repos.Staff.ListAllWithPerson(sc.ctx)
	require.NoError(t, err)
	for _, s := range listed {
		assert.NotEqual(t, staff.ID, s.ID, "offboarded staff must not appear in the staff list")
	}

	found, err := sc.repos.Staff.FindByID(sc.ctx, staff.ID)
	assert.Error(t, err, "FindByID must not return soft-deleted staff")
	assert.Nil(t, found)
}

// TestOffboardStaff_ClearsDirectPermissions: direct tenant-scoped permission
// grants must not survive offboarding — re-invitation reuses the account ID,
// so leftover grants would restore old elevated permissions.
func TestOffboardStaff_ClearsDirectPermissions(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, sc.db, "Direct", "Permission")
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)
	perm := testpkg.CreateTestPermission(t, sc.db,
		fmt.Sprintf("offb-perm-%d", time.Now().UnixNano()), "students", "read")
	require.NoError(t, sc.repos.AccountPermission.GrantPermission(sc.ctx, account.ID, perm.ID))

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM auth.account_permissions WHERE account_id = ?`, account.ID)
		testpkg.CleanupTableRecords(t, sc.db, "auth.permissions", perm.ID)
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, &account.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	var permCount int
	err := sc.db.NewSelect().
		TableExpr(`auth.account_permissions`).
		ColumnExpr(`COUNT(*)`).
		Where(`account_id = ? AND tenant_id = ?`, account.ID, offboardingFixtureTenant).
		Scan(context.Background(), &permCount)
	require.NoError(t, err)
	assert.Zero(t, permCount, "direct permission grants must be removed on offboarding")
}

// TestOffboardStaff_PreservesGuardianAccess: a dual-role teacher/guardian
// account loses staff access but must keep the guardian role and an active
// tenant mapping so the parents portal keeps working.
func TestOffboardStaff_PreservesGuardianAccess(t *testing.T) {
	sc := newOffboardingScenario(t)

	credential := offboardingCredential("Offboard", "123")
	emailAddr := fmt.Sprintf("offboard-guardian-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, sc.db, emailAddr, credential)
	person := testpkg.CreateTestPersonWithAccountID(t, sc.db, "Dual", "Role", account.ID)
	staff := testpkg.CreateTestStaffForPerson(t, sc.db, person.ID)
	testpkg.MapAccountToTenant(t, sc.db, account.ID, offboardingFixtureTenant)
	staffRole := testpkg.GetOrCreateTestRole(t, sc.db, "user")
	assignTenantRole(t, sc.db, account.ID, staffRole.ID)
	guardianRole := testpkg.GetOrCreateTestRole(t, sc.db, "guardian")
	assignTenantRole(t, sc.db, account.ID, guardianRole.ID)

	t.Cleanup(func() {
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, person.ID, &account.ID)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	countRole := func(roleID int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`auth.account_roles`).
			ColumnExpr(`COUNT(*)`).
			Where(`account_id = ? AND role_id = ?`, account.ID, roleID).
			Scan(context.Background(), &n))
		return n
	}
	assert.Zero(t, countRole(staffRole.ID), "staff role must be removed")
	assert.Equal(t, 1, countRole(guardianRole.ID), "guardian role must be kept")

	exists, err := sc.repos.AccountTenant.ExistsByAccountAndTenant(sc.ctx, account.ID, offboardingFixtureTenant)
	require.NoError(t, err)
	assert.True(t, exists, "tenant mapping must stay active for the guardian")

	var active bool
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`auth.accounts`).
		ColumnExpr(`active`).
		Where(`id = ?`, account.ID).
		Scan(context.Background(), &active))
	assert.True(t, active, "guardian account must stay active")

	var personAccountID *int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.persons`).
		ColumnExpr(`account_id`).
		Where(`id = ?`, person.ID).
		Scan(context.Background(), &personAccountID))
	assert.Nil(t, personAccountID, "staff person must still be unlinked from the account")

	// Staff portal login is refused for the now guardian-only account; the
	// parents portal remains the entry point.
	_, _, err = sc.authSvc.Login(context.Background(), emailAddr, credential)
	require.Error(t, err, "staff login must be refused for the guardian-only account")
	assert.ErrorIs(t, err, authSvcPkg.ErrParentMustUseParentPortal)
}

// TestOffboardStaff_RemovesSameDayPlannedInstanceAssignments: same-day
// timetable assignments on instances that have not started yet must be
// removed (instance Start would copy them into active supervisors), while
// already-completed same-day instances keep their rows as history.
func TestOffboardStaff_RemovesSameDayPlannedInstanceAssignments(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "SameDay", "Planned")
	room := testpkg.CreateTestRoom(t, sc.db, fmt.Sprintf("OffbInstRoom-%d", time.Now().UnixNano()))
	today := timezone.TodayDate()

	makeInstance := func(title string) *scheduleModels.ActivityInstance {
		inst := &scheduleModels.ActivityInstance{
			Date:          today,
			Title:         title,
			StartTime:     time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			EndTime:       time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			RoomID:        room.ID,
			Status:        scheduleModels.InstanceStatusPlanned,
			IsSpontaneous: true,
		}
		inst.SetTenantID(offboardingFixtureTenant)
		require.NoError(t, sc.repos.ActivityInstance.Create(sc.ctx, inst))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, sc.db, "schedule.activity_instances", inst.ID) })
		return inst
	}
	plannedInst := makeInstance(fmt.Sprintf("offb-planned-%d", time.Now().UnixNano()))
	completedInst := makeInstance(fmt.Sprintf("offb-completed-%d", time.Now().UnixNano()))
	_, err := sc.db.ExecContext(context.Background(),
		`UPDATE schedule.activity_instances SET status = 'completed' WHERE id = ?`, completedInst.ID)
	require.NoError(t, err)

	makeAssignment := func(instanceID int64) *scheduleModels.InstanceStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: staff.ID}
		row.SetTenantID(offboardingFixtureTenant)
		require.NoError(t, sc.repos.InstanceStaff.Create(sc.ctx, row))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, sc.db, "schedule.instance_staff", row.ID) })
		return row
	}
	plannedRow := makeAssignment(plannedInst.ID)
	completedRow := makeAssignment(completedInst.ID)

	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, sc.db, room.ID)
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, nil)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	countAssignment := func(id int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`schedule.instance_staff`).
			ColumnExpr(`COUNT(*)`).
			Where(`id = ?`, id).
			Scan(context.Background(), &n))
		return n
	}
	assert.Zero(t, countAssignment(plannedRow.ID),
		"same-day planned assignment must be removed so instance Start cannot copy it")
	assert.Equal(t, 1, countAssignment(completedRow.ID),
		"same-day completed assignment must stay as history")
}

// TestOffboardStaff_RemovesPendingAndFutureAbsences: pending requests and
// not-yet-over absences must disappear from operational queries; past decided
// absences stay as history.
func TestOffboardStaff_RemovesPendingAndFutureAbsences(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Absent", "Staff")
	today := timezone.TodayDate()

	makeAbsence := func(absenceType, status string, start, end timezone.Date) *activeModels.StaffAbsence {
		absence := &activeModels.StaffAbsence{
			StaffID:     staff.ID,
			AbsenceType: absenceType,
			DateStart:   start,
			DateEnd:     end,
			Status:      status,
			CreatedBy:   staff.ID,
		}
		require.NoError(t, sc.repos.StaffAbsence.Create(sc.ctx, absence))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, sc.db, "active.staff_absences", absence.ID) })
		return absence
	}
	pendingRequest := makeAbsence(activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusRequested,
		today.AddDays(3), today.AddDays(5))
	futureApproved := makeAbsence(activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusApproved,
		today.AddDays(10), today.AddDays(12))
	pastApproved := makeAbsence(activeModels.AbsenceTypeSick, activeModels.AbsenceStatusApproved,
		today.AddDays(-10), today.AddDays(-8))
	pastQuestion := makeAbsence(activeModels.AbsenceTypeVacation, activeModels.AbsenceStatusQuestion,
		today.AddDays(-6), today.AddDays(-4))

	t.Cleanup(func() {
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, nil)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	countAbsence := func(id int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`active.staff_absences`).
			ColumnExpr(`COUNT(*)`).
			Where(`id = ?`, id).
			Scan(context.Background(), &n))
		return n
	}
	assert.Zero(t, countAbsence(pendingRequest.ID), "pending request must be removed")
	assert.Zero(t, countAbsence(futureApproved.ID), "future approved absence must be removed")
	assert.Zero(t, countAbsence(pastQuestion.ID), "past questioned request must be removed")
	assert.Equal(t, 1, countAbsence(pastApproved.ID), "past absence must stay as history")

	var auditedCount string
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`audit.data_deletions`).
		ColumnExpr(`metadata->>'staff_absences'`).
		Where(`staff_id = ?`, staff.ID).
		Scan(context.Background(), &auditedCount))
	assert.Equal(t, "3", auditedCount, "audit record must count the deleted absences")
}

func TestOffboardStaff_RemovesUpcomingStaffShifts(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Shifted", "Offboard")
	today := timezone.TodayDate()

	makeShift := func(date timezone.Date, startHour int) *scheduleModels.StaffShift {
		shift := &scheduleModels.StaffShift{
			StaffID:   staff.ID,
			Date:      date,
			StartTime: time.Date(1, 1, 1, startHour, 0, 0, 0, time.UTC),
			EndTime:   time.Date(1, 1, 1, startHour+4, 0, 0, 0, time.UTC),
			CreatedBy: staff.ID,
		}
		require.NoError(t, sc.repos.StaffShift.Create(sc.ctx, shift))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, sc.db, "schedule.staff_shifts", shift.ID) })
		return shift
	}
	past := makeShift(today.AddDays(-1), 8)
	sameDay := makeShift(today, 8)
	future := makeShift(today.AddDays(1), 8)

	t.Cleanup(func() {
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, nil)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	countShift := func(id int64) int {
		var n int
		require.NoError(t, sc.db.NewSelect().
			TableExpr(`schedule.staff_shifts`).
			ColumnExpr(`COUNT(*)`).
			Where(`id = ?`, id).
			Scan(context.Background(), &n))
		return n
	}
	assert.Equal(t, 1, countShift(past.ID), "past staff shift must stay as history")
	assert.Zero(t, countShift(sameDay.ID), "same-day staff shift must be removed")
	assert.Zero(t, countShift(future.ID), "future staff shift must be removed")

	var auditedCount string
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`audit.data_deletions`).
		ColumnExpr(`metadata->>'staff_shifts'`).
		Where(`staff_id = ?`, staff.ID).
		Scan(context.Background(), &auditedCount))
	assert.Equal(t, "2", auditedCount, "audit record must count the deleted staff shifts")
}

// TestOffboardStaff_ClearsWorkTimeModelAssignment: the soft-deleted staff row
// must not keep its work_time_model_id, or the RESTRICT FK blocks model
// deletion while the live-staff pre-check reports zero assignments.
func TestOffboardStaff_ClearsWorkTimeModelAssignment(t *testing.T) {
	sc := newOffboardingScenario(t)

	staff := testpkg.CreateTestStaff(t, sc.db, "Modeled", "Staff")
	model := &configModel.WorkTimeModel{
		Name:               fmt.Sprintf("offb-model-%d", time.Now().UnixNano()),
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.January, 5),
	}
	require.NoError(t, sc.repos.WorkTimeModel.Create(sc.ctx, model, []*configModel.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 300},
	}))
	_, err := sc.db.ExecContext(context.Background(),
		`UPDATE users.staff SET work_time_model_id = ? WHERE id = ?`, model.ID, staff.ID)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM config.work_time_model_entries WHERE work_time_model_id = ?`, model.ID)
		_, _ = sc.db.ExecContext(ctx, `DELETE FROM config.work_time_models WHERE id = ?`, model.ID)
		cleanupOffboardedStaffChain(t, sc.db, staff.ID, staff.PersonID, nil)
	})

	require.NoError(t, sc.svc.OffboardStaff(sc.ctx, staff.ID, "test-admin"))

	var workTimeModelID *int64
	require.NoError(t, sc.db.NewSelect().
		TableExpr(`users.staff`).
		ColumnExpr(`work_time_model_id`).
		Where(`id = ?`, staff.ID).
		Scan(context.Background(), &workTimeModelID))
	assert.Nil(t, workTimeModelID, "work_time_model_id must be cleared on offboarding")

	require.NoError(t, sc.repos.WorkTimeModel.Delete(sc.ctx, model.ID),
		"work-time-model deletion must succeed once the offboarded staff no longer references it")
}
