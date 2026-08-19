// Integration tests for operator-led school access management (issue #1021).
package platform_test

import (
	"context"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The second school every access test grants into. High ID so it cannot
// collide with the default test tenant or with other packages' fixtures.
const accessTargetTenantID int64 = 1021001

// systemRoleID resolves a platform system role by name from the live schema
// instead of hardcoding the seeded IDs.
func systemRoleID(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	var id int64
	err := db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = ?", name).
		Where("tenant_id IS NULL").
		Where("is_system = true").
		Scan(context.Background(), &id)
	require.NoError(t, err, "system role %q must exist in the test schema", name)
	return id
}

func createTenantRole(t *testing.T, db *bun.DB, name string, tenantID int64, baseRole *string) *authModels.Role {
	t.Helper()
	role := &authModels.Role{Name: name, IsSystem: false, BaseRole: baseRole}
	role.SetTenantID(tenantID)
	require.NoError(t, db.NewInsert().Model(role).ModelTableExpr(`auth.roles`).Scan(context.Background()))
	return role
}

func cleanupTenantRole(t *testing.T, db *bun.DB, roleID int64) {
	t.Helper()
	_, err := db.NewDelete().TableExpr(`auth.account_roles`).Where("role_id = ?", roleID).Exec(context.Background())
	require.NoError(t, err)
}

func roleNamesAt(entries []platformSvc.AccountTenantAccessEntry, tenantID int64) []string {
	for _, entry := range entries {
		if entry.TenantID != tenantID {
			continue
		}
		names := make([]string, 0, len(entry.Roles))
		for _, role := range entry.Roles {
			names = append(names, role.Name)
		}
		return names
	}
	return nil
}

func entryFor(entries []platformSvc.AccountTenantAccessEntry, tenantID int64) *platformSvc.AccountTenantAccessEntry {
	for i := range entries {
		if entries[i].TenantID == tenantID {
			return &entries[i]
		}
	}
	return nil
}

// setupAccessTestAccount creates an account that already belongs to the default
// test school, plus the second school the tests grant access to.
func setupAccessTestAccount(t *testing.T, db *bun.DB) (*authModels.Account, func()) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "access-target")
	testpkg.EnsureAccountTenant(t, db, account.ID, testSchoolID)
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)

	person := testpkg.CreateTestPerson(t, db, "Zugriff", "Testperson")
	linkPersonToAccount(t, db, person.ID, account.ID)

	return account, func() {
		cleanupAccessFixtures(t, db, account.ID)
	}
}

func linkPersonToAccount(t *testing.T, db *bun.DB, personID, accountID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		Table("users.persons").
		Set("account_id = ?", accountID).
		Where("id = ?", personID).
		Exec(context.Background())
	require.NoError(t, err)
}

// cleanupAccessFixtures removes the staff/teacher/person rows the grant path
// creates. They hang off the account, so they are keyed by account_id.
func cleanupAccessFixtures(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `
		DELETE FROM users.teachers WHERE staff_id IN (
			SELECT s.id FROM users.staff s
			JOIN users.persons p ON p.id = s.person_id
			WHERE p.account_id = ?)`, accountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		DELETE FROM users.staff WHERE person_id IN (
			SELECT id FROM users.persons WHERE account_id = ?)`, accountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM audit.auth_events WHERE account_id = ?`, accountID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM users.persons WHERE account_id = ?`, accountID)
	require.NoError(t, err)
}

func TestIntegration_GrantAccountTenantAccess_AddsSchoolWithRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")

	entries, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID},
		operator.ID, testClientIP)
	require.NoError(t, err)

	granted := entryFor(entries, accessTargetTenantID)
	require.NotNil(t, granted, "the new school must show up in the returned access list")
	assert.Equal(t, authModels.AccountTenantStatusActive, granted.Status)
	assert.Equal(t, []string{"admin"}, roleNamesAt(entries, accessTargetTenantID))

	// The account must be usable at the new school, which means a person and a
	// staff record carrying its name.
	assert.True(t, granted.HasPerson, "grant must create a person at the target school")
	assert.True(t, granted.HasStaff, "grant must create a staff record at the target school")

	// The original school is untouched.
	assert.NotNil(t, entryFor(entries, testSchoolID), "existing access must survive")
}

// A school's own caregiver-tier role gets the caregiver profile the platform
// user role gets (#2222). It used to be withheld because the provisioning
// keyed on auth.roles.is_system, which left such an account in the staff list
// with empty groups and empty supervisions — the same bug one level down from
// the missing staff record. The tier decides now, not the origin of the role.
func TestIntegration_GrantAccountTenantAccess_CustomUserBaseCreatesCaregiverProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	baseRole := authModels.BaseRoleUser
	role := createTenantRole(t, db, "zugriff-custom-user", accessTargetTenantID, &baseRole)
	defer cleanupTenantRole(t, db, role.ID)

	entries, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: role.ID}, operator.ID, testClientIP)
	require.NoError(t, err)
	assert.Equal(t, []string{role.Name}, roleNamesAt(entries, accessTargetTenantID))

	teacherCount, err := db.NewSelect().
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Join(`JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Where(`"p".account_id = ?`, account.ID).
		Where(`"p".tenant_id = ?`, accessTargetTenantID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, teacherCount, "custom user-base roles need the caregiver profile their tier reads through")
}

func TestIntegration_GrantAccountTenantAccess_RejectsDuplicate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	require.NoError(t, err)

	_, err = service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	var conflict *platformSvc.ConflictError
	require.ErrorAs(t, err, &conflict, "granting the same school twice must conflict")
}

func TestIntegration_GrantAccountTenantAccess_RejectsGuardianRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	guardianRoleID := systemRoleID(t, db, authModels.BaseRoleGuardian)

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: guardianRoleID}, operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid, "guardian access must go through the guardian invitation flow")
}

func TestIntegration_GrantAccountTenantAccess_RejectsForeignTenantRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	// A custom role that exists only at the ORIGINAL school.
	foreignRole := testpkg.CreateTestRoleForTenant(t, db, "zugriff-fremdrolle", testSchoolID)

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: foreignRole.ID}, operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid, "a role of another school must not be assignable")
}

func TestIntegration_UpdateAccountTenantRole_ReplacesAdminKeepsCaregiver(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")
	userRoleID := systemRoleID(t, db, "user")

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: userRoleID}, operator.ID, testClientIP)
	require.NoError(t, err)

	// user -> admin: the caregiver role stays, because removing it has to run
	// through the caregiver capability checks.
	entries, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID, adminRoleID, operator.ID, testClientIP)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"user", "admin"}, roleNamesAt(entries, accessTargetTenantID))

	// admin -> user: the administrative role IS removed.
	entries, err = service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID, userRoleID, operator.ID, testClientIP)
	require.NoError(t, err)
	assert.Equal(t, []string{"user"}, roleNamesAt(entries, accessTargetTenantID))
}

func TestIntegration_GrantAccountTenantAccess_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "access-lehrkraft-regrant")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	// A live caregiver identity at the target school WITHOUT an active
	// mapping — exactly the state a revoke leaves behind (person, staff and
	// teacher rows are deliberately kept for the history, see
	// RevokeAccountTenantAccess).
	staff := testpkg.CreateTestStaffForTenant(t, db, accessTargetTenantID, "Gestrandet", "Betreuung")
	linkPersonToAccount(t, db, staff.PersonID, account.ID)
	teacher := &userModels.Teacher{StaffID: staff.ID}
	teacher.SetTenantID(accessTargetTenantID)
	require.NoError(t, db.NewInsert().Model(teacher).ModelTableExpr(`users.teachers`).Scan(ctx))
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()
	operator := testpkg.CreateTestOperator(t, db)

	// Granting the school as Lehrkraft would revive the stranded caregiver
	// profile under a class_day-only JWT — must be rejected like on the
	// role-update path.
	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: systemRoleID(t, db, "lehrkraft")}, operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid, "a live caregiver profile must block a lehrkraft grant")
}

func TestIntegration_UpdateAccountTenantRole_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	// Granting the caregiver role materializes the local person/staff/teacher
	// identity at the target school.
	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: systemRoleID(t, db, "user")}, operator.ID, testClientIP)
	require.NoError(t, err)

	// Switching that account to Lehrkraft would strand the users.teachers
	// profile and its supervisions — the UI blocks it, the service must too.
	_, err = service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID,
		systemRoleID(t, db, "lehrkraft"), operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid, "a caregiver profile must block the switch to lehrkraft")
}

func TestIntegration_UpdateAccountTenantRole_RequiresExistingAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")

	_, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID, adminRoleID, operator.ID, testClientIP)

	var notFound *platformSvc.AccountTenantAccessNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_RevokeAccountTenantAccess_DeactivatesMappingAndRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	require.NoError(t, err)

	entries, err := service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
	require.NoError(t, err)

	revoked := entryFor(entries, accessTargetTenantID)
	require.NotNil(t, revoked, "a revoked mapping stays visible as inactive")
	assert.Equal(t, authModels.AccountTenantStatusInactive, revoked.Status)
	assert.Empty(t, roleNamesAt(entries, accessTargetTenantID), "tenant-scoped roles must be removed")

	// The account keeps its original school and therefore stays active.
	assert.NotNil(t, entryFor(entries, testSchoolID))
	assertAccountActive(t, db, account.ID, true)
}

func TestIntegration_RevokeAccountTenantAccess_AllowsCustomRoleNamedUser(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	role := createTenantRole(t, db, authModels.BaseRoleUser, accessTargetTenantID, nil)
	defer cleanupTenantRole(t, db, role.ID)

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: role.ID}, operator.ID, testClientIP)
	require.NoError(t, err)

	entries, err := service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
	require.NoError(t, err)
	assert.Equal(t, authModels.AccountTenantStatusInactive, entryFor(entries, accessTargetTenantID).Status)
}

func TestIntegration_RevokeAccountTenantAccess_RejectsRolesOwnedByOtherFeatures(t *testing.T) {
	t.Parallel()
	for _, roleName := range []string{authModels.BaseRoleGuardian, authModels.BaseRoleUser, "teacher"} {
		t.Run(roleName, func(t *testing.T) {
			db := testpkg.SetupTestDB(t)

			service := buildProvisioningService(t, db)
			ctx := context.Background()
			account, cleanupAccount := setupAccessTestAccount(t, db)
			defer cleanupAccount()
			operator := testpkg.CreateTestOperator(t, db)

			_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
				platformSvc.GrantAccountTenantAccessRequest{RoleID: systemRoleID(t, db, "admin")}, operator.ID, testClientIP)
			require.NoError(t, err)

			var roleID int64
			if roleName == "teacher" {
				// The migration removes this retired role. Recreate the exact legacy
				// shape to prove an old database row cannot bypass caregiver checks.
				legacyRole := &authModels.Role{Name: "teacher", IsSystem: true}
				require.NoError(t, db.NewInsert().Model(legacyRole).ModelTableExpr(`auth.roles`).Scan(ctx))
				roleID = legacyRole.ID
				defer func() {
					_, deleteErr := db.NewDelete().TableExpr(`auth.account_roles`).Where("role_id = ?", legacyRole.ID).Exec(ctx)
					require.NoError(t, deleteErr)
				}()
			} else {
				roleID = systemRoleID(t, db, roleName)
			}

			assignment := &authModels.AccountRole{AccountID: account.ID, RoleID: roleID}
			assignment.SetTenantID(accessTargetTenantID)
			_, err = db.NewInsert().Model(assignment).ModelTableExpr(`auth.account_roles`).Exec(ctx)
			require.NoError(t, err)

			_, err = service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
			var invalid *platformSvc.InvalidDataError
			require.ErrorAs(t, err, &invalid)

			entries, err := service.ListAccountTenantAccess(ctx, account.ID)
			require.NoError(t, err)
			assert.Equal(t, authModels.AccountTenantStatusActive, entryFor(entries, accessTargetTenantID).Status)
		})
	}
}

func TestIntegration_RevokeAccountTenantAccess_DeactivatesAccountWithoutRemainingSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-single-school")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
	require.NoError(t, err)

	assertAccountActive(t, db, account.ID, false)
}

func TestIntegration_ListAccountTenantAccess_UnknownAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)

	_, err := service.ListAccountTenantAccess(context.Background(), 0)

	var notFound *platformSvc.AccountNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_ListAccountTenantAccess_ReturnsSchoolsWithRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	require.NoError(t, err)

	entries, err := service.ListAccountTenantAccess(ctx, account.ID)
	require.NoError(t, err)

	granted := entryFor(entries, accessTargetTenantID)
	require.NotNil(t, granted)
	assert.Equal(t, []string{"admin"}, roleNamesAt(entries, accessTargetTenantID))
	assert.NotEmpty(t, granted.SchoolName)
	assert.NotEmpty(t, granted.OrganizationName)
	assert.True(t, granted.SchoolActive)
	assert.NotNil(t, entryFor(entries, testSchoolID), "the original school is listed as well")
}

func TestIntegration_ListAccountTenantAccess_UnknownAccountID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)

	_, err := service.ListAccountTenantAccess(context.Background(), 99999999)

	var notFound *platformSvc.AccountNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_GrantAccountTenantAccess_RequiresNamesWithoutPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	// An account that carries no person anywhere: there is no name to copy, so
	// the operator has to supply one.
	account := testpkg.CreateTestAccount(t, db, "access-nameless")
	testpkg.EnsureAccountTenant(t, db, account.ID, testSchoolID)
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: systemRoleID(t, db, "admin")},
		operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)

	entries, listErr := service.ListAccountTenantAccess(ctx, account.ID)
	require.NoError(t, listErr)
	assert.Empty(t, roleNamesAt(entries, accessTargetTenantID))
}

func TestIntegration_GrantAccountTenantAccess_ReactivatesAccountAfterRestoringLastRevokedSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-reactivate")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)

	// Revoking the only school deactivates the account...
	_, err := service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
	require.NoError(t, err)
	assertAccountActive(t, db, account.ID, false)

	// Restoring the final revoked school access also restores login capability.
	entries, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{
			RoleID:    systemRoleID(t, db, "user"),
			FirstName: "Wieder",
			LastName:  "Aktiv",
			Position:  "Gruppenleitung",
		}, operator.ID, testClientIP)
	require.NoError(t, err)

	granted := entryFor(entries, accessTargetTenantID)
	require.NotNil(t, granted)
	assert.Equal(t, authModels.AccountTenantStatusActive, granted.Status)
	assertAccountActive(t, db, account.ID, true)

	// The caregiver role also creates the teacher record, carrying the position.
	var position string
	err = db.NewSelect().
		ColumnExpr(`"t".role`).
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Join(`JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Where(`"p".account_id = ?`, account.ID).
		Scan(ctx, &position)
	require.NoError(t, err)
	assert.Equal(t, "Gruppenleitung", position)
}

func TestIntegration_GrantAccountTenantAccess_DoesNotReactivateManuallyDeactivatedAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	_, err := db.NewUpdate().
		Table("auth.accounts").
		Set("active = ?", false).
		Where("id = ?", account.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: systemRoleID(t, db, "admin")},
		operator.ID, testClientIP)
	require.NoError(t, err)
	assertAccountActive(t, db, account.ID, false)
}

func TestIntegration_RevokeAccountTenantAccess_UnknownSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.RevokeAccountTenantAccess(context.Background(), account.ID, 99999999, operator.ID, testClientIP)

	var notFound *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_RevokeAccountTenantAccess_WithoutExistingAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.RevokeAccountTenantAccess(context.Background(), account.ID, accessTargetTenantID, operator.ID, testClientIP)

	var notFound *platformSvc.AccountTenantAccessNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_UpdateAccountTenantRole_ToCaregiverCreatesLocalIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-no-person")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	source := testpkg.CreateTestPerson(t, db, "Bestehend", "Betreuung")
	linkPersonToAccount(t, db, source.ID, account.ID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)

	entries, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID,
		systemRoleID(t, db, "user"), operator.ID, testClientIP)
	require.NoError(t, err)
	assert.Equal(t, []string{"user"}, roleNamesAt(entries, accessTargetTenantID))

	updated := entryFor(entries, accessTargetTenantID)
	require.NotNil(t, updated)
	assert.True(t, updated.HasPerson, "the caregiver role requires a local person record")
	assert.True(t, updated.HasStaff, "the caregiver role requires a local staff record")

	teacherCount, err := db.NewSelect().
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Join(`JOIN users.persons AS "p" ON "p".id = "s".person_id`).
		Where(`"p".account_id = ?`, account.ID).
		Where(`"p".tenant_id = ?`, accessTargetTenantID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, teacherCount)
}

func TestIntegration_UpdateAccountTenantRole_ToCaregiverRequiresIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "access-no-caregiver-identity")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)
	_, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID,
		systemRoleID(t, db, "user"), operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)
}

// The name for a new staff person at the target school is copied from an
// identity the account already has elsewhere. A child's record is not such an
// identity: copying it would file a child as personnel at another school, and
// afterwards nothing tells that staff row apart from a legitimate one.
func TestIntegration_UpdateAccountTenantRole_RefusesStudentAsNameSource(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	// A child's person record that happens to carry an account.
	student, account := testpkg.CreateTestStudentWithAccount(t, db, "Kind", "Datensatz", "2a")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		_, err := db.ExecContext(ctx, `DELETE FROM users.students WHERE id = ?`, student.ID)
		require.NoError(t, err)
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)
	_, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID,
		systemRoleID(t, db, "user"), operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)
	assert.Contains(t, err.Error(), "unambiguous name",
		"the refusal must come from the name resolution, not from an unrelated failure")

	assertNoPersonAt(t, db, account.ID, accessTargetTenantID)
}

// Two schools carrying two different names is an ambiguity this path is not
// entitled to resolve — picking the lower-numbered school's version is a coin
// toss with someone's name. The change is refused instead.
func TestIntegration_UpdateAccountTenantRole_RefusesAmbiguousNameSource(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-ambiguous-name")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)

	// One identity per school, disagreeing on the name. The partial unique index
	// on (tenant_id, account_id) is why they have to sit at different tenants.
	first := testpkg.CreateTestPerson(t, db, "Anna", "Beispiel")
	linkPersonToAccount(t, db, first.ID, account.ID)
	second := createPersonAtTenant(t, db, ambiguousNameTenantID, "Bea", "Beispiel")
	linkPersonToAccount(t, db, second.ID, account.ID)

	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)
	_, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID,
		systemRoleID(t, db, "user"), operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)
	assert.Contains(t, err.Error(), "unambiguous name",
		"the refusal must come from the name resolution, not from an unrelated failure")

	assertNoPersonAt(t, db, account.ID, accessTargetTenantID)
}

// The ambiguity above must not reach a request that needs no name at all.
//
// A revoke deliberately leaves person, staff and teacher behind, so re-granting
// the same school finds the identity already complete and has nothing to name.
// Resolving the name first anyway meant asking the account's other schools,
// finding two spellings there, and refusing a re-grant on the strength of a
// disagreement that had no bearing on it — the operator could only get past it
// by retyping a name that was already on the record and was never going to be
// written.
func TestIntegration_GrantAccountTenantAccess_ReGrantReusesLocalIdentityDespiteAmbiguityElsewhere(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-regrant-ambiguous")
	testpkg.EnsureAccountTenant(t, db, account.ID, testSchoolID)
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)

	// Two other schools that disagree on the name, so no name can be borrowed.
	first := testpkg.CreateTestPerson(t, db, "Anna", "Beispiel")
	linkPersonToAccount(t, db, first.ID, account.ID)
	second := createPersonAtTenant(t, db, ambiguousNameTenantID, "Bea", "Beispiel")
	linkPersonToAccount(t, db, second.ID, account.ID)

	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)
	adminRoleID := systemRoleID(t, db, "admin")

	// The first grant carries its own name, so the ambiguity never comes up.
	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{
			RoleID:    adminRoleID,
			FirstName: "Carla",
			LastName:  "Beispiel",
		}, operator.ID, testClientIP)
	require.NoError(t, err)

	_, err = service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
	require.NoError(t, err)

	// The re-grant brings no name — and must not need one.
	entries, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: adminRoleID}, operator.ID, testClientIP)
	require.NoError(t, err, "the retained identity answers the question the other schools cannot")

	granted := entryFor(entries, accessTargetTenantID)
	require.NotNil(t, granted)
	assert.Equal(t, authModels.AccountTenantStatusActive, granted.Status)

	// The retained person was reused rather than duplicated, and nothing
	// overwrote the name it already carried.
	var names []struct {
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	err = db.NewSelect().
		ColumnExpr("first_name, last_name").
		TableExpr(`users.persons`).
		Where(`account_id = ?`, account.ID).
		Where(`tenant_id = ?`, accessTargetTenantID).
		Where(`deleted_at IS NULL`).
		Scan(ctx, &names)
	require.NoError(t, err)
	require.Len(t, names, 1, "the partial unique index allows exactly one person per account and school")
	assert.Equal(t, "Carla", names[0].FirstName)
	assert.Equal(t, "Beispiel", names[0].LastName)
}

// A second school for the ambiguity case; kept apart from accessTargetTenantID
// so the disagreement is between two schools that are neither the target.
const ambiguousNameTenantID int64 = 1021002

func createPersonAtTenant(t *testing.T, db *bun.DB, tenantID int64, firstName, lastName string) *userModels.Person {
	t.Helper()
	testpkg.EnsureTestTenant(t, db, tenantID)

	person := &userModels.Person{FirstName: firstName, LastName: lastName}
	person.SetTenantID(tenantID)
	_, err := db.NewInsert().
		Model(person).
		ModelTableExpr(`users.persons`).
		Exec(context.Background())
	require.NoError(t, err)
	return person
}

func assertNoPersonAt(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.persons`).
		Where(`account_id = ?`, accountID).
		Where(`tenant_id = ?`, tenantID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count, "a refused role change must not leave a person behind")
}

func assertAccountActive(t *testing.T, db *bun.DB, accountID int64, want bool) {
	t.Helper()
	var active bool
	err := db.NewSelect().
		ColumnExpr("active").
		TableExpr("auth.accounts").
		Where("id = ?", accountID).
		Scan(context.Background(), &active)
	require.NoError(t, err)
	assert.Equal(t, want, active)
}
