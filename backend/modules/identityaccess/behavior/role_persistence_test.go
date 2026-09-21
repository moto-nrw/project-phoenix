package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRolePersistenceScopeOrderingAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	otherCtx := tenant.WithTenantID(ctx, other)
	baseRole := " user "
	first, err := identity.CreateRole(ctx, "Primary", "original", &baseRole)
	require.NoError(t, err)
	require.Equal(t, "primary", first.Name)
	require.Equal(t, "user", *first.BaseRole)
	require.Equal(t, tenantID, *first.TenantID)
	require.False(t, first.CreatedAt.IsZero())
	second := testpkg.CreateTestRole(t, db, "secondary")
	foreign := testpkg.CreateTestRoleForTenant(t, db, "primary", other)

	roles, err := identity.ListRoles(ctx, identityaccess.RoleFilter{Name: "PRIMARY"})
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.Equal(t, first.ID, roles[0].ID)
	_, err = identity.GetRole(ctx, foreign.ID)
	require.ErrorIs(t, err, identityaccess.ErrRoleNotFound)
	system, err := identity.GetRole(ctx, adminRoleID(t, db))
	require.NoError(t, err)
	require.True(t, system.IsSystem)
	require.Nil(t, system.TenantID)

	account := testpkg.CreateTestAccount(t, db, "ordered-role-owner")
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at) VALUES
 (?, ?, ?, TIMESTAMPTZ '2025-01-02'), (?, ?, ?, TIMESTAMPTZ '2025-01-02'), (?, ?, ?, TIMESTAMPTZ '2025-01-01')`,
		account.ID, first.ID, tenantID, account.ID, second.ID, tenantID, account.ID, foreign.ID, other).Exec(ctx)
	require.NoError(t, err)
	roles, err = identity.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 2)
	require.Equal(t, first.ID, roles[0].ID, "assignment id breaks equal creation timestamps")
	require.Equal(t, second.ID, roles[1].ID)
	names, err := identity.GetAccountRoleNames(ctx, []int64{account.ID})
	require.NoError(t, err)
	require.Equal(t, first.Name, names[account.ID])

	changed := first
	changed.Name, changed.Description = "Renamed", "transactional change"
	rollback := errors.New("roll back role changes")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = tenant.WithTenantID(txCtx, tenantID)
		require.NoError(t, identity.UpdateRole(txCtx, changed))
		current, readErr := identity.GetRole(txCtx, first.ID)
		require.NoError(t, readErr)
		require.Equal(t, "renamed", current.Name)
		require.NoError(t, identity.DeleteRole(txCtx, first.ID))
		currentRoles, readErr := identity.GetAccountRoles(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Len(t, currentRoles, 1)
		require.Equal(t, second.ID, currentRoles[0].ID)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	current, err := identity.GetRole(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, first.Name, current.Name)
	require.Equal(t, first.Description, current.Description)
	roles, err = identity.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 2)
	require.ErrorIs(t, identity.UpdateRole(otherCtx, changed), identityaccess.ErrRoleNotFound)
	require.ErrorIs(t, identity.DeleteRole(otherCtx, first.ID), identityaccess.ErrRoleNotFound)

	require.NoError(t, identity.DeleteRole(ctx, first.ID))
	roles, err = identity.GetAccountRoles(otherCtx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.Equal(t, foreign.ID, roles[0].ID, "deleting a local role preserves foreign assignments")
}

func TestRolePersistenceDatabaseFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	require.NoError(t, db.Close())
	_, err := identity.GetRole(testpkg.Ctx(t), 1)
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.ListRoles(testpkg.Ctx(t), identityaccess.RoleFilter{})
	require.ErrorContains(t, err, "database is closed")
	avatars, err := identity.GetAccountAvatars(testpkg.Ctx(t), []int64{1})
	require.Nil(t, avatars)
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.GetAccountRoleNames(testpkg.Ctx(t), []int64{1})
	require.ErrorContains(t, err, "database is closed")
}

func TestRolePersistenceAccountAvatars(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)
	withAvatar := testpkg.CreateTestAccount(t, db, "role-avatar")
	secondAvatar := testpkg.CreateTestAccount(t, db, "role-second-avatar")
	emptyAvatar := testpkg.CreateTestAccount(t, db, "role-empty-avatar")
	nullAvatar := testpkg.CreateTestAccount(t, db, "role-null-avatar")
	_, err := db.NewRaw("UPDATE auth.accounts SET avatar = ? WHERE id = ?", "avatars/staff.png", withAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = ? WHERE id = ?", "avatars/second.png", secondAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = '' WHERE id = ?", emptyAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = NULL WHERE id = ?", nullAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	avatars, err := identity.GetAccountAvatars(ctx, []int64{withAvatar.ID, secondAvatar.ID, emptyAvatar.ID, nullAvatar.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]string{withAvatar.ID: "avatars/staff.png", secondAvatar.ID: "avatars/second.png"}, avatars)
	avatars, err = identity.GetAccountAvatars(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, avatars)
	require.Empty(t, avatars)
}

func TestRolePersistenceBatchRoleNames(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)

	t.Run("returns role names for multiple accounts", func(t *testing.T) {
		account1 := testpkg.CreateTestAccount(t, db, "batch_role_1")
		account2 := testpkg.CreateTestAccount(t, db, "batch_role_2")
		role1 := testpkg.CreateTestRole(t, db, fmt.Sprintf("BatchAdmin_%d", time.Now().UnixNano()))
		role2 := testpkg.CreateTestRole(t, db, fmt.Sprintf("BatchUser_%d", time.Now().UnixNano()))

		// Assign roles
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
			account1.ID, role1.ID, testpkg.Tenant(t), account2.ID, role2.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		result, err := repo.GetAccountRoleNames(ctx, []int64{account1.ID, account2.ID})
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, role1.Name, result[account1.ID])
		require.Equal(t, role2.Name, result[account2.ID])
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		result, err := repo.GetAccountRoleNames(ctx, []int64{})
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("returns empty map for accounts with no roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "batch_norole")

		result, err := repo.GetAccountRoleNames(ctx, []int64{account.ID})
		require.NoError(t, err)
		require.Empty(t, result)
	})

	t.Run("keeps first role when account has multiple roles", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "batch_multi")
		role1 := testpkg.CreateTestRole(t, db, fmt.Sprintf("MultiFirst_%d", time.Now().UnixNano()))
		role2 := testpkg.CreateTestRole(t, db, fmt.Sprintf("MultiSecond_%d", time.Now().UnixNano()))

		// Assign two roles — first inserted should be returned (ORDER BY created_at, id ASC).
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role1.ID, testpkg.Tenant(t))
		require.NoError(t, err)
		_, err = db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role2.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		result, err := repo.GetAccountRoleNames(ctx, []int64{account.ID})
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, role1.Name, result[account.ID])
	})

	t.Run("respects tenant scoping", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "batch_tenant")
		role := testpkg.CreateTestRole(t, db, fmt.Sprintf("TenantRole_%d", time.Now().UnixNano()))

		// Assign role to tenant 1
		_, err := db.ExecContext(ctx,
			"INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)",
			account.ID, role.ID, testpkg.Tenant(t))
		require.NoError(t, err)

		// Query with tenant 1 context — should find it
		result, err := repo.GetAccountRoleNames(ctx, []int64{account.ID})
		require.NoError(t, err)
		require.Equal(t, role.Name, result[account.ID])

		// Query from another tenant — should not find it
		ctx2 := testpkg.TenantContext(testpkg.UniqueTestTenantID(t))
		result2, err := repo.GetAccountRoleNames(ctx2, []int64{account.ID})
		require.NoError(t, err)
		require.Empty(t, result2)
	})
}
