package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The role administration moved to Identity & Access (#3314). These tests
// drive its public contract composed the way the serving root composes it:
// the Lehrkraft guards in both directions (#1772), the persisted role
// changes and the account batch lookups.

func TestAssignRoleToAccount_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	// Live caregiver profile (person → staff → teacher) linked to the
	// account at this school — the state a role swap must never strand.
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Guard", "Betreuung")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	var lehrkraftRoleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = 'lehrkraft'").
		Where("is_system = true").
		Where("tenant_id IS NULL").
		Scan(context.Background(), &lehrkraftRoleID),
		"lehrkraft system role must exist in the test schema")

	// The tenant RBAC endpoint (POST /auth/accounts/{id}/roles/{roleId}) is
	// the last path that could put the Lehrkraft role on a caregiver
	// account — the role administration must reject it like the operator and
	// invitation flows do (#1772).
	err := admin.AssignRoleToAccount(ctx, account.ID, lehrkraftRoleID)
	require.ErrorIs(t, err, identityaccess.ErrRoleLehrkraftCaregiverProfile)

	roles, err := admin.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestAssignRoleToAccount_AllowsLehrkraftWithoutCaregiverProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	// No person/staff/teacher chain: a plain account may become Lehrkraft.
	account := testpkg.CreateTestAccount(t, db, "lehrkraft-plain")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	var lehrkraftRoleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = 'lehrkraft'").
		Where("is_system = true").
		Where("tenant_id IS NULL").
		Scan(context.Background(), &lehrkraftRoleID),
		"lehrkraft system role must exist in the test schema")

	require.NoError(t, admin.AssignRoleToAccount(ctx, account.ID, lehrkraftRoleID))

	roles, err := admin.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, lehrkraftRoleID, roles[0].ID)
}

// lehrkraftSystemRoleID resolves the platform Lehrkraft role the migrations
// seed. Shared by the guard tests in both directions.
func lehrkraftRoleID(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = 'lehrkraft'").
		Where("is_system = true").
		Where("tenant_id IS NULL").
		Scan(context.Background(), &roleID),
		"lehrkraft system role must exist in the test schema")
	return roleID
}

// caregiverSystemRoleID resolves the "user" system role — the caregiver tier
// that reads through users.teachers.
func caregiverRoleID(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = 'user'").
		Where("is_system = true").
		Where("tenant_id IS NULL").
		Scan(context.Background(), &roleID),
		"user system role must exist in the test schema")
	return roleID
}

func adminRoleID(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	var roleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("LOWER(name) = 'admin'").
		Where("is_system = true").
		Where("tenant_id IS NULL").
		Scan(context.Background(), &roleID),
		"admin system role must exist in the test schema")
	return roleID
}

func TestAssignRoleToAccount_RejectsNonLehrkraftRoleForLehrkraftAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "lehrkraft-to-admin")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	require.NoError(t, admin.AssignRoleToAccount(ctx, account.ID, lehrkraftRoleID(t, db)))

	err := admin.AssignRoleToAccount(ctx, account.ID, adminRoleID(t, db))
	require.ErrorIs(t, err, identityaccess.ErrLehrkraftRoleImmutable)

	roles, err := admin.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, lehrkraftRoleID(t, db), roles[0].ID)
}

func TestReplaceAccountRole_RejectsReplacingLehrkraftAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "replace-lehrkraft")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	require.NoError(t, admin.AssignRoleToAccount(ctx, account.ID, lehrkraftRoleID(t, db)))

	err := admin.ReplaceAccountRole(ctx, account.ID, adminRoleID(t, db))
	require.ErrorIs(t, err, identityaccess.ErrLehrkraftRoleImmutable)

	roles, err := admin.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, lehrkraftRoleID(t, db), roles[0].ID)
}

func TestAssignRoleToAccount_RejectsCaregiverRoleForLehrkraftWithoutProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	// A Lehrkraft account by construction: no person → staff → teacher chain.
	account := testpkg.CreateTestAccount(t, db, "lehrkraft-to-caregiver")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	require.NoError(t, admin.AssignRoleToAccount(ctx, account.ID, lehrkraftRoleID(t, db)))

	// Lehrkraft accounts cannot be changed through the tenant RBAC endpoint,
	// even when the target role would otherwise require no identity change.
	err := admin.AssignRoleToAccount(ctx, account.ID, caregiverRoleID(t, db))
	require.ErrorIs(t, err, identityaccess.ErrLehrkraftRoleImmutable)

	roles, err := admin.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1, "the refused assignment must not have been written")
	assert.Equal(t, lehrkraftRoleID(t, db), roles[0].ID)
}

func TestAssignRoleToAccount_RejectsCaregiverRoleForLehrkraftWithProfile(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	// A profile does not make a Lehrkraft role exchange safe. The Lehrkraft
	// account must be offboarded before it receives a different role.
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Wechsel", "Betreuung")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	// Put the account into the Lehrkraft state directly: the service guard
	// itself refuses this combination, and the point here is the state a
	// half-finished operator switch leaves behind.
	_, err := db.NewInsert().
		Model(&map[string]interface{}{
			"account_id": account.ID,
			"role_id":    lehrkraftRoleID(t, db),
			"tenant_id":  testpkg.Tenant(t),
		}).
		TableExpr("auth.account_roles").
		Exec(context.Background())
	require.NoError(t, err)

	err = admin.AssignRoleToAccount(ctx, account.ID, caregiverRoleID(t, db))
	require.ErrorIs(t, err, identityaccess.ErrLehrkraftRoleImmutable)
}

func TestAssignRoleToAccount_AllowsCaregiverRoleForPlainAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	admin := setupAuthFactory(t, db).AccountAuthentication()
	ctx := testpkg.Ctx(t)

	// Negative control for the guard's scope: account provisioning assigns
	// the caregiver role before it creates the teacher record, so a plain
	// account without the Lehrkraft role must stay unaffected.
	account := testpkg.CreateTestAccount(t, db, "plain-caregiver")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	require.NoError(t, admin.AssignRoleToAccount(ctx, account.ID, caregiverRoleID(t, db)))
}

func TestRoleManagement_PersistsRoleChangesWithoutTokenRevocation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	t.Run("AssignRoleToAccount persists created mapping", func(t *testing.T) {
		admin := setupAuthFactory(t, db).AccountAuthentication()

		account := testpkg.CreateTestAccount(t, db, "assign-rollback")
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		role := testpkg.CreateTestRole(t, db, "assign-rollback-role-"+time.Now().Format("150405.000000000"))

		err := admin.AssignRoleToAccount(ctx, account.ID, role.ID)
		require.NoError(t, err)

		roles, err := admin.GetAccountRoles(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, roles, 1)
		assert.Equal(t, role.ID, roles[0].ID)
	})

	t.Run("RemoveRoleFromAccount persists deleted mapping", func(t *testing.T) {
		admin := setupAuthFactory(t, db).AccountAuthentication()

		account := testpkg.CreateTestAccount(t, db, "remove-rollback")
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		role := testpkg.CreateTestRole(t, db, "remove-rollback-role-"+time.Now().Format("150405.000000000"))

		require.NoError(t, admin.AssignRoleToAccount(ctx, account.ID, role.ID))

		err := admin.RemoveRoleFromAccount(ctx, account.ID, role.ID)
		require.NoError(t, err)

		roles, err := admin.GetAccountRoles(ctx, account.ID)
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

func TestRoleAdministration_GetAccountEmails(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	svc := setupAuthService(t, db)
	rbac := roleAdministrationOf(t, svc)
	ctx := testpkg.Ctx(t)

	t.Run("returns emails for valid account IDs", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "svc-emails1")
		account2 := testpkg.CreateTestAccount(t, db, "svc-emails2")

		result, err := rbac.GetAccountEmails(ctx, []int64{account1.ID, account2.ID})
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, account1.Email, result[account1.ID])
		assert.Equal(t, account2.Email, result[account2.ID])
	})

	t.Run("returns empty map for empty slice", func(t *testing.T) {
		result, err := rbac.GetAccountEmails(ctx, []int64{})
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("returns partial results for mixed valid and invalid IDs", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "svc-emailspartial")

		result, err := rbac.GetAccountEmails(ctx, []int64{account.ID, int64(999999)})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, account.Email, result[account.ID])
	})
}
