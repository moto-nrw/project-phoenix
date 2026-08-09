package auth

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignRoleToAccount_RejectsLehrkraftForCaregiverProfile(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupInternalAuthService(t, db)
	ctx := testpkg.TenantContext(1)

	// Live caregiver profile (person → staff → teacher) linked to the
	// account at this school — the state a role swap must never strand.
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, "Guard", "Betreuung")
	t.Cleanup(func() {
		testpkg.CleanupTeacherFixtures(t, db, teacher.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)

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
	defer func() { _ = db.Close() }()

	service := setupInternalAuthService(t, db)
	ctx := testpkg.TenantContext(1)

	// No person/staff/teacher chain: a plain account may become Lehrkraft.
	account := testpkg.CreateTestAccount(t, db, "lehrkraft-plain")
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.EnsureAccountTenant(t, db, account.ID, 1)

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

func TestRoleManagement_PersistsRoleChangesWithoutTokenRevocation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)

	t.Run("AssignRoleToAccount persists created mapping", func(t *testing.T) {
		service := setupInternalAuthService(t, db)

		account := testpkg.CreateTestAccount(t, db, "assign-rollback")
		t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
		testpkg.EnsureAccountTenant(t, db, account.ID, 1)

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
		testpkg.EnsureAccountTenant(t, db, account.ID, 1)

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
