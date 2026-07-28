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
