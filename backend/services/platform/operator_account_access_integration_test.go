// Integration tests for operator-led school access management (issue #1021).
package platform_test

import (
	"context"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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
	testpkg.CleanupRoleRecords(t, db, roleID)
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
// test school, plus the second school the tests grant access to. The returned
// func must be deferred BEFORE db.Close(), so it cannot use t.Cleanup.
func setupAccessTestAccount(t *testing.T, db *bun.DB) (*authModels.Account, func()) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "access-target")
	testpkg.EnsureAccountTenant(t, db, account.ID, testSchoolID)
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)

	person := testpkg.CreateTestPerson(t, db, "Zugriff", "Testperson")
	linkPersonToAccount(t, db, person.ID, account.ID)

	return account, func() {
		cleanupAccessFixtures(t, db, account.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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

func TestIntegration_GrantAccountTenantAccess_CustomUserBaseDoesNotCreateCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	assert.Zero(t, teacherCount, "custom user-base roles must not create a caregiver profile")
}

func TestIntegration_GrantAccountTenantAccess_RejectsDuplicate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	// A custom role that exists only at the ORIGINAL school.
	foreignRole := testpkg.CreateTestRoleForTenant(t, db, "zugriff-fremdrolle", testSchoolID)
	defer testpkg.CleanupRoleRecords(t, db, foreignRole.ID)

	_, err := service.GrantAccountTenantAccess(ctx, account.ID, accessTargetTenantID,
		platformSvc.GrantAccountTenantAccessRequest{RoleID: foreignRole.ID}, operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid, "a role of another school must not be assignable")
}

func TestIntegration_UpdateAccountTenantRole_ReplacesAdminKeepsCaregiver(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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

func TestIntegration_UpdateAccountTenantRole_RequiresExistingAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	for _, roleName := range []string{authModels.BaseRoleGuardian, "teacher"} {
		t.Run(roleName, func(t *testing.T) {
			db := testpkg.SetupTestDB(t)
			defer func() { _ = db.Close() }()

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
					testpkg.CleanupRoleRecords(t, db, legacyRole.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-single-school")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.RevokeAccountTenantAccess(ctx, account.ID, accessTargetTenantID, operator.ID, testClientIP)
	require.NoError(t, err)

	assertAccountActive(t, db, account.ID, false)
}

func TestIntegration_ListAccountTenantAccess_UnknownAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)

	_, err := service.ListAccountTenantAccess(context.Background(), 0)

	var notFound *platformSvc.AccountNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_ListAccountTenantAccess_ReturnsSchoolsWithRoles(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)

	_, err := service.ListAccountTenantAccess(context.Background(), 99999999)

	var notFound *platformSvc.AccountNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_GrantAccountTenantAccess_RequiresNamesWithoutPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	// An account that carries no person anywhere: there is no name to copy, so
	// the operator has to supply one.
	account := testpkg.CreateTestAccount(t, db, "access-nameless")
	testpkg.EnsureAccountTenant(t, db, account.ID, testSchoolID)
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-reactivate")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.RevokeAccountTenantAccess(context.Background(), account.ID, 99999999, operator.ID, testClientIP)

	var notFound *platformSvc.SchoolNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_RevokeAccountTenantAccess_WithoutExistingAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	account, cleanupAccount := setupAccessTestAccount(t, db)
	defer cleanupAccount()
	operator := testpkg.CreateTestOperator(t, db)

	_, err := service.RevokeAccountTenantAccess(context.Background(), account.ID, accessTargetTenantID, operator.ID, testClientIP)

	var notFound *platformSvc.AccountTenantAccessNotFoundError
	require.ErrorAs(t, err, &notFound)
}

func TestIntegration_UpdateAccountTenantRole_ToCaregiverCreatesLocalIdentity(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "access-no-person")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	source := testpkg.CreateTestPerson(t, db, "Bestehend", "Betreuung")
	linkPersonToAccount(t, db, source.ID, account.ID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := buildProvisioningService(t, db)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "access-no-caregiver-identity")
	testpkg.EnsureTestTenant(t, db, accessTargetTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, accessTargetTenantID)
	defer func() {
		cleanupAccessFixtures(t, db, account.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	}()

	operator := testpkg.CreateTestOperator(t, db)
	_, err := service.UpdateAccountTenantRole(ctx, account.ID, accessTargetTenantID,
		systemRoleID(t, db, "user"), operator.ID, testClientIP)

	var invalid *platformSvc.InvalidDataError
	require.ErrorAs(t, err, &invalid)
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
