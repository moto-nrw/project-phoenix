package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAccountRoleQueriesScopeAndSystemRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "role-facts")
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	testpkg.MapAccountToTenant(t, db, account.ID, otherSchool)
	first := testpkg.CreateTestRole(t, db, "first-assigned-role")
	second := testpkg.CreateTestRole(t, db, "second-assigned-role")
	foreign := testpkg.CreateTestRoleForTenant(t, db, "foreign-assigned-role", otherSchool)
	_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at) VALUES
		(?, ?, ?, TIMESTAMPTZ '2025-01-02'), (?, ?, ?, TIMESTAMPTZ '2025-01-01'), (?, ?, ?, TIMESTAMPTZ '2025-01-01')`,
		account.ID, first.ID, testpkg.Tenant(t), account.ID, second.ID, testpkg.Tenant(t), account.ID, foreign.ID, otherSchool).Exec(ctx)
	require.NoError(t, err)
	names, err := identity.ListSchoolAccountRoleNames(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, []string{second.Name, first.Name}, names)
	// Even an administrative transaction must not widen this school query.
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = tenant.WithTenantID(txCtx, testpkg.Tenant(t))
		actual, queryErr := identity.ListSchoolAccountRoleNames(txCtx, account.ID)
		require.NoError(t, queryErr)
		require.Equal(t, names, actual)
		return nil
	}))
	_, err = identity.ListSchoolAccountRoleNames(context.Background(), account.ID)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)

	shadow := testpkg.CreateTestRole(t, db, "system-role-shadow")
	_, err = db.NewRaw(`UPDATE auth.roles SET name = 'user' WHERE id = ?`, shadow.ID).Exec(ctx)
	require.NoError(t, err)
	systemID, found, err := identity.FindSystemRoleID(ctx, "USER")
	require.NoError(t, err)
	require.True(t, found)
	var expectedID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.roles WHERE name = 'user' AND is_system = TRUE AND tenant_id IS NULL`).Scan(ctx, &expectedID))
	require.Equal(t, expectedID, systemID)
	_, found, err = identity.FindSystemRoleID(ctx, first.Name)
	require.NoError(t, err)
	require.False(t, found, "a school role is not a platform system role")

	rollback := errors.New("roll back role removal")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		txCtx = tenant.WithTenantID(txCtx, testpkg.Tenant(t))
		_, writeErr := tx.NewRaw(`DELETE FROM auth.account_roles WHERE account_id = ? AND role_id = ?`, account.ID, second.ID).Exec(txCtx)
		require.NoError(t, writeErr)
		actual, queryErr := identity.ListSchoolAccountRoleNames(txCtx, account.ID)
		require.NoError(t, queryErr)
		require.Equal(t, []string{first.Name}, actual)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	after, err := identity.ListSchoolAccountRoleNames(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, names, after)
}

func TestAccountRoleQueriesDatabaseFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	_, _, err = identity.FindSystemRoleID(context.Background(), "user")
	require.ErrorContains(t, err, "database is closed")
	ctx := tenant.WithUnitOfWork(testpkg.TenantContext(testpkg.Tenant(t)), testpkg.TenantRuntime(t, db))
	_, err = identity.ListSchoolAccountRoleNames(ctx, 1)
	require.ErrorContains(t, err, "database is closed")
}
