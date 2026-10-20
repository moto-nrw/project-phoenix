package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type brokenSchoolIdentity struct {
	accountID int64
	personID  int64
}

func strPtrValue(v string) *string { return &v }

func createIdentityRepairRole(t *testing.T, db *testpkg.DB, tenantID int64, name string, baseRole *string) int64 {
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

// createBrokenSchoolIdentity reproduces the state the invitation flow produced:
// an active account at the school, holding a role, with a person record and no
// staff record.
func createBrokenSchoolIdentity(t *testing.T, db *testpkg.DB, tenantID int64, firstName, lastName string, roleID int64) brokenSchoolIdentity {
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

// The repair matches persons, staff and teachers within one school, and the
// statements say so explicitly. What makes an unscoped match unreachable rather
// than merely unlikely is the schema: the composite foreign keys from migration
// 1.15.2 tie each row's tenant to its parent's, so a staff row on another
// school's person — the shape that could otherwise read as "already repaired"
// and hide an account from both the fix and the report — cannot be stored at
// all.
//
// Pinned here because the SQL above leans on it. Drop either constraint and the
// tenant predicates stop being belt-and-braces and start being the only thing
// holding the invariant up, which is worth failing a test over.
func TestRepairSchoolIdentitiesTenantScopingIsEnforcedBySchema(t *testing.T) {
	t.Parallel()
	db := setupPeopleStorageBeforeCutover(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)

	testpkg.OwnTenantRows(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	testpkg.OwnTenantRows(t, db, otherTenantID)

	role := createIdentityRepairRole(t, db, tenantID, "reparatur-fremdschule", strPtrValue("user"))

	holder := createBrokenSchoolIdentity(t, db, tenantID, "Repair", "Fremd", role)

	_, err := db.ExecContext(ctx, `
		INSERT INTO users.staff (tenant_id, person_id, staff_notes, created_at, updated_at)
		VALUES (?, ?, '', NOW(), NOW())`, otherTenantID, holder.personID)
	require.Error(t, err, "a staff row must not outlive the school of the person it belongs to")
	assert.Contains(t, err.Error(), "fk_staff_person_tenant")

	// And with the invariant intact, the repair does what it says on the tin.
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db))
	require.NoError(t, repairSchoolIdentitiesUp(ctx, db), "repair migration must be idempotent")

	assert.Equal(t, 1, liveStaffCountForTenant(t, db, holder.personID, tenantID),
		"the school the account holds its role at gets the staff record")
	assert.Equal(t, 1, liveTeacherCountForTenant(t, db, holder.personID, tenantID),
		"and the caregiver profile lands under that same school")
}

func liveStaffCountForTenant(t *testing.T, db *testpkg.DB, personID, tenantID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.staff AS "s"`).
		Where(`"s".person_id = ?`, personID).
		Where(`"s".tenant_id = ?`, tenantID).
		Where(`"s".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}

func liveTeacherCountForTenant(t *testing.T, db *testpkg.DB, personID, tenantID int64) int {
	t.Helper()
	count, err := db.NewSelect().
		TableExpr(`users.teachers AS "t"`).
		Join(`JOIN users.staff AS "s" ON "s".id = "t".staff_id`).
		Where(`"s".person_id = ?`, personID).
		Where(`"t".tenant_id = ?`, tenantID).
		Where(`"t".deleted_at IS NULL`).
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}
