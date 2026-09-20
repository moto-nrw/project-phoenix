package behavior_test

import (
	"context"
	"errors"
	"testing"

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
	_, err = identity.GetAccountRoleNames(testpkg.Ctx(t), []int64{1})
	require.ErrorContains(t, err, "database is closed")
}

func TestRolePersistenceAccountAvatars(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity := roleAdministrationOf(t, setupAuthService(t, db))
	ctx := testpkg.Ctx(t)
	withAvatar := testpkg.CreateTestAccount(t, db, "role-avatar")
	emptyAvatar := testpkg.CreateTestAccount(t, db, "role-empty-avatar")
	nullAvatar := testpkg.CreateTestAccount(t, db, "role-null-avatar")
	_, err := db.NewRaw("UPDATE auth.accounts SET avatar = ? WHERE id = ?", "avatars/staff.png", withAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = '' WHERE id = ?", emptyAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET avatar = NULL WHERE id = ?", nullAvatar.ID).Exec(ctx)
	require.NoError(t, err)
	avatars, err := identity.GetAccountAvatars(ctx, []int64{withAvatar.ID, emptyAvatar.ID, nullAvatar.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]string{withAvatar.ID: "avatars/staff.png"}, avatars)
	avatars, err = identity.GetAccountAvatars(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, avatars)
	require.Empty(t, avatars)
}
