package auth

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAssignRoleToAccount_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// Live caregiver profile (person → staff → teacher) linked to the
	// account at this school — the state a role swap must never strand.
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Guard", "Betreuung")
	t.Cleanup(func() {
		testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})
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
	// account — the service must reject it like the operator and
	// invitation flows do (#1772).
	err := service.AssignRoleToAccount(ctx, int(account.ID), int(lehrkraftRoleID))
	require.ErrorIs(t, err, ErrRoleLehrkraftCaregiverProfile)

	roles, err := service.GetAccountRoles(ctx, int(account.ID))
	require.NoError(t, err)
	assert.Empty(t, roles)
}

func TestAssignRoleToAccount_AllowsLehrkraftWithoutCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// No person/staff/teacher chain: a plain account may become Lehrkraft.
	account := testpkg.CreateTestAccount(t, db, "lehrkraft-plain")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
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

	require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(lehrkraftRoleID)))

	roles, err := service.GetAccountRoles(ctx, int(account.ID))
	require.NoError(t, err)
	require.Len(t, roles, 1)
	assert.Equal(t, lehrkraftRoleID, roles[0].ID)
}

// lehrkraftSystemRoleID resolves the platform Lehrkraft role the migrations
// seed. Shared by the guard tests in both directions.
func lehrkraftSystemRoleID(t *testing.T, db *bun.DB) int64 {
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
func caregiverSystemRoleID(t *testing.T, db *bun.DB) int64 {
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

func TestAssignRoleToAccount_RejectsCaregiverRoleForLehrkraftWithoutProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// A Lehrkraft account by construction: no person → staff → teacher chain.
	account := testpkg.CreateTestAccount(t, db, "lehrkraft-to-caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(lehrkraftSystemRoleID(t, db))))

	// The reverse of the guard above: swapping to a caregiver role through
	// the tenant RBAC endpoint would grant caregiver permissions to an
	// account with no users.teachers row — no groups, no supervision, an
	// empty caregiver landing page (#1772).
	err := service.AssignRoleToAccount(ctx, int(account.ID), int(caregiverSystemRoleID(t, db)))
	require.ErrorIs(t, err, ErrRoleCaregiverNeedsProfile)

	roles, err := service.GetAccountRoles(ctx, int(account.ID))
	require.NoError(t, err)
	require.Len(t, roles, 1, "the refused assignment must not have been written")
	assert.Equal(t, lehrkraftSystemRoleID(t, db), roles[0].ID)
}

func TestAssignRoleToAccount_AllowsCaregiverRoleOnceProfileExists(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// The operator role change provisions person/staff/teacher BEFORE it
	// assigns the role, so the same swap must pass there.
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Wechsel", "Betreuung")
	t.Cleanup(func() {
		testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	// Put the account into the Lehrkraft state directly: the service guard
	// itself refuses this combination, and the point here is the state a
	// half-finished operator switch leaves behind.
	_, err := db.NewInsert().
		Model(&map[string]interface{}{
			"account_id": account.ID,
			"role_id":    lehrkraftSystemRoleID(t, db),
			"tenant_id":  testpkg.Tenant(t),
		}).
		TableExpr("auth.account_roles").
		Exec(context.Background())
	require.NoError(t, err)

	require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(caregiverSystemRoleID(t, db))))
}

func TestAssignRoleToAccount_AllowsCaregiverRoleForPlainAccount(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// Negative control for the guard's scope: account provisioning assigns
	// the caregiver role before it creates the teacher record, so a plain
	// account without the Lehrkraft role must stay unaffected.
	account := testpkg.CreateTestAccount(t, db, "plain-caregiver")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(caregiverSystemRoleID(t, db))))
}

func TestRoleManagement_PersistsRoleChangesWithoutTokenRevocation(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	t.Run("AssignRoleToAccount persists created mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "assign-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		role := testpkg.CreateTestRole(t, db, "assign-rollback-role-"+time.Now().Format("150405.000000000"))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

		err := service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		require.Len(t, roles, 1)
		assert.Equal(t, role.ID, roles[0].ID)
	})

	t.Run("RemoveRoleFromAccount persists deleted mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "remove-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		role := testpkg.CreateTestRole(t, db, "remove-rollback-role-"+time.Now().Format("150405.000000000"))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "auth.roles", role.ID) })

		require.NoError(t, service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID)))

		err := service.RemoveRoleFromAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Empty(t, roles)
	})
}

// POST /auth/link-to-tenant is the fourth path that can put the Lehrkraft role
// on an account, and the one that was still open (#1772/#2222). Unlike
// /auth/register the account already exists and may already carry an identity at
// this school — a revoke deliberately leaves person → staff → teacher behind —
// so linking it as Lehrkraft would revive a live caregiver profile and its group
// supervisions under a JWT that only holds class_day permissions.
func TestLinkSchoolAccount_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Link", "Betreuung")
	t.Cleanup(func() {
		testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	roleID := lehrkraftSystemRoleID(t, db)
	_, _, err := service.LinkSchoolAccount(context.Background(), account.Email, &roleID, testpkg.Tenant(t),
		&SchoolAccountIdentity{FirstName: "Link", LastName: "Betreuung"})
	require.ErrorIs(t, err, ErrRoleLehrkraftCaregiverProfile)

	// The guard runs inside the link transaction, so the role is not assigned.
	roles, err := service.GetAccountRoles(testpkg.Ctx(t), int(account.ID))
	require.NoError(t, err)
	assert.Empty(t, roles, "a refused link must not assign the role")
}

// The same path with no caregiver profile in the way is the ordinary case and
// must keep working — the guard is about the profile, not about the role.
func TestLinkSchoolAccount_AllowsLehrkraftWithoutCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	service := setupInternalAuthService(t, db)

	account := testpkg.CreateTestAccount(t, db, "link-lehrkraft-plain")
	t.Cleanup(func() { testpkg.CleanupAccountWithIdentity(t, db, account.ID) })

	roleID := lehrkraftSystemRoleID(t, db)
	_, identity, err := service.LinkSchoolAccount(context.Background(), account.Email, &roleID, testpkg.Tenant(t),
		&SchoolAccountIdentity{FirstName: "Link", LastName: "Lehrkraft"})
	require.NoError(t, err)

	// Staff, because Lehrkraft is personnel; no caregiver profile, because it is
	// class_day-read-only by design.
	require.NotNil(t, identity)
	require.NotNil(t, identity.Staff)
	assert.Nil(t, identity.Teacher, "the Lehrkraft role never earns a caregiver profile")
}
