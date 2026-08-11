package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A school role invitation left the account with a person and nothing else
// (#2222). The repair must attach the staff record to exactly that person, and
// give a caregiver-tier role its profile too.
func TestRepairSchoolIdentities(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	adminRole := createIdentityRepairRole(t, db, tenantID, "reparatur-leitung", strPtrValue("admin"))
	userRole := createIdentityRepairRole(t, db, tenantID, "reparatur-kraft", strPtrValue("user"))
	legacyRole := createIdentityRepairRole(t, db, tenantID, "reparatur-alt", nil)
	guardianRole := createIdentityRepairRole(t, db, tenantID, "reparatur-sorge", strPtrValue("guardian"))
	defer cleanupIdentityRepairRoles(t, db, adminRole, userRole, legacyRole, guardianRole)

	admin := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Admin", adminRole)
	caregiver := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Kraft", userRole)
	legacy := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Alt", legacyRole)
	guardian := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Sorge", guardianRole)
	defer cleanupBrokenSchoolIdentities(t, db, admin, caregiver, legacy, guardian)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 1, liveStaffCount(t, db, admin.personID), "admin-tier role must get a staff record")
	assert.Equal(t, 1, liveStaffCount(t, db, caregiver.personID), "caregiver-tier role must get a staff record")
	assert.Equal(t, 1, liveStaffCount(t, db, legacy.personID), "unknown tier counts as personnel")
	assert.Equal(t, 0, liveStaffCount(t, db, guardian.personID), "guardians are not personnel")

	assert.Equal(t, 0, liveTeacherCount(t, db, admin.personID), "an admin-tier role runs without a caregiver profile")
	assert.Equal(t, 1, liveTeacherCount(t, db, caregiver.personID), "a caregiver-tier role needs its profile")
	assert.Equal(t, 0, liveTeacherCount(t, db, legacy.personID))
}

// An account whose access was revoked keeps its person row on purpose, but must
// not be silently re-staffed by the repair.
func TestRepairSchoolIdentitiesSkipsInactiveAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	role := createIdentityRepairRole(t, db, tenantID, "reparatur-inaktiv", strPtrValue("admin"))
	defer cleanupIdentityRepairRoles(t, db, role)

	revoked := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Inaktiv", role)
	defer cleanupBrokenSchoolIdentities(t, db, revoked)

	_, err := db.ExecContext(ctx, `
		UPDATE auth.account_tenants SET status = 'inactive'
		WHERE account_id = ? AND tenant_id = ?`, revoked.accountID, tenantID)
	require.NoError(t, err)

	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	assert.Equal(t, 0, liveStaffCount(t, db, revoked.personID))
}

type brokenSchoolIdentity struct {
	accountID int64
	personID  int64
}

func strPtrValue(v string) *string { return &v }

func createIdentityRepairRole(t *testing.T, db *bun.DB, tenantID int64, name string, baseRole *string) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(`
		INSERT INTO auth.roles (name, description, is_system, tenant_id, base_role, created_at, updated_at)
		VALUES (?, 'Reparatur-Testrolle', FALSE, ?, ?, NOW(), NOW())
		RETURNING id`,
		fmt.Sprintf("%s-%d", name, tenantID), tenantID, baseRole,
	).Scan(context.Background(), &id)
	require.NoError(t, err)
	return id
}

func cleanupIdentityRepairRoles(t *testing.T, db *bun.DB, roleIDs ...int64) {
	t.Helper()
	for _, id := range roleIDs {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.account_roles WHERE role_id = ?`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.roles WHERE id = ?`, id)
	}
}

// createBrokenSchoolIdentity reproduces the state the invitation flow produced:
// an active account at the school, holding a role, with a person record and no
// staff record.
func createBrokenSchoolIdentity(t *testing.T, db *bun.DB, tenantID int64, firstName, lastName string, roleID int64) brokenSchoolIdentity {
	t.Helper()
	ctx := context.Background()

	var accountID int64
	err := db.NewRaw(`
		INSERT INTO auth.accounts (email, username, password_hash, active, created_at, updated_at)
		VALUES (?, ?, 'x', TRUE, NOW(), NOW())
		RETURNING id`,
		fmt.Sprintf("repair-%s-%d@example.com", lastName, tenantID),
		fmt.Sprintf("repair_%s_%d", lastName, tenantID),
	).Scan(ctx, &accountID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
		VALUES (?, ?, 'active', NOW(), NOW(), NOW())`, accountID, tenantID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())`, accountID, roleID, tenantID)
	require.NoError(t, err)

	var personID int64
	err = db.NewRaw(`
		INSERT INTO users.persons (tenant_id, first_name, last_name, account_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		RETURNING id`, tenantID, firstName, lastName, accountID).Scan(ctx, &personID)
	require.NoError(t, err)

	return brokenSchoolIdentity{accountID: accountID, personID: personID}
}

func cleanupBrokenSchoolIdentities(t *testing.T, db *bun.DB, identities ...brokenSchoolIdentity) {
	t.Helper()
	ctx := context.Background()
	for _, identity := range identities {
		_, _ = db.ExecContext(ctx, `
			DELETE FROM users.teachers WHERE staff_id IN (
				SELECT id FROM users.staff WHERE person_id = ?
			)`, identity.personID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users.staff WHERE person_id = ?`, identity.personID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users.persons WHERE id = ?`, identity.personID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ?`, identity.accountID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, identity.accountID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, identity.accountID)
	}
}

func liveStaffCount(t *testing.T, db *bun.DB, personID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.staff AS "s"`).
		Where(`"s".person_id = ?`, personID).
		Where(`"s".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}

func liveTeacherCount(t *testing.T, db *bun.DB, personID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Where(`"s".person_id = ?`, personID).
		Where(`"t".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return count
}
