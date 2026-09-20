package behavior_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffAccountQueriesScopeAndTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	school := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	account := testpkg.CreateTestAccount(t, db, "staff-query")
	inactive := testpkg.CreateTestAccount(t, db, "staff-query-inactive")
	suspended := testpkg.CreateTestAccount(t, db, "staff-query-suspended")
	for _, id := range []int64{account.ID, inactive.ID, suspended.ID} {
		testpkg.MapAccountToTenant(t, db, id, school)
	}
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	_, err = db.NewRaw("UPDATE auth.accounts SET active = FALSE WHERE id = ?", inactive.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", suspended.ID, school).Exec(ctx)
	require.NoError(t, err)
	ids := []int64{account.ID, inactive.ID, suspended.ID}
	active, err := identity.ListActiveAccountIDsForTenant(ctx, school, ids)
	require.NoError(t, err)
	require.Equal(t, []int64{account.ID}, active)
	emails, err := identity.ListAccountEmails(ctx, ids)
	require.NoError(t, err)
	require.Equal(t, map[int64]string{account.ID: account.Email, inactive.ID: inactive.Email, suspended.ID: suspended.Email}, emails)

	role := testpkg.CreateTestRole(t, db, "staff-query-role")
	_, err = db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
		account.ID, role.ID, school, account.ID, role.ID, other).Exec(ctx)
	require.NoError(t, err)
	matches, err := identity.CountRoleNameMatchesByAccountIDs(ctx, ids, []string{" " + strings.ToUpper(role.Name) + " "})
	require.NoError(t, err)
	require.Equal(t, map[int64]int{account.ID: 2}, matches, "without an RLS transaction the historical projection retains cross-school assignment duplicates")

	systemID, found, err := identity.FindSystemRoleID(ctx, "user")
	require.NoError(t, err)
	require.True(t, found)
	shadow := testpkg.CreateTestRole(t, db, "staff-query-shadow")
	_, err = db.NewRaw("UPDATE auth.roles SET name = 'user' WHERE id = ?", shadow.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?), (?, ?, ?)",
		account.ID, systemID, other, suspended.ID, shadow.ID, school).Exec(ctx)
	require.NoError(t, err)
	systemAccounts, err := identity.ListAccountIDsWithSystemRoleNames(ctx, ids, []string{" USER "}, school)
	require.NoError(t, err)
	require.Empty(t, systemAccounts, "neither a foreign assignment nor a custom role with the same name qualifies")
	systemAccounts, err = identity.ListAccountIDsWithSystemRoleNames(ctx, ids, []string{" USER "}, other)
	require.NoError(t, err)
	require.Equal(t, []int64{account.ID}, systemAccounts)

	roleGrant := testpkg.CreateTestPermission(t, db, "staff-query-role", "staff-role", "read")
	directGrant := testpkg.CreateTestPermission(t, db, "staff-query-direct", "staff-direct", "read")
	foreignGrant := testpkg.CreateTestPermission(t, db, "staff-query-foreign", "staff-foreign", "read")
	_, err = db.NewRaw("INSERT INTO auth.role_permissions (role_id, permission_id) VALUES (?, ?)", role.ID, roleGrant.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted) VALUES (?, ?, ?, FALSE), (?, ?, ?, TRUE), (?, ?, ?, TRUE)",
		account.ID, roleGrant.ID, school, account.ID, directGrant.ID, school, account.ID, foreignGrant.ID, other).Exec(ctx)
	require.NoError(t, err)
	names, err := identity.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, []int64{account.ID}, school)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{roleGrant.Resource + ":read", directGrant.Resource + ":read"}, names[account.ID],
		"staff projection keeps role grants despite a direct denial, and excludes foreign direct grants")

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		counts, queryErr := identity.CountRoleNameMatchesByAccountIDs(txCtx, ids, []string{role.Name})
		require.NoError(t, queryErr)
		require.Equal(t, map[int64]int{account.ID: 1}, counts, "role counts retain the ambient RLS restriction")
		foreignActive, queryErr := identity.ListActiveAccountIDsForTenant(txCtx, other, ids)
		require.NoError(t, queryErr)
		require.Equal(t, []int64{account.ID}, foreignActive, "the global account mapping uses its explicit school predicate")
		foreignRoles, queryErr := identity.ListAccountIDsWithSystemRoleNames(txCtx, ids, []string{"user"}, other)
		require.NoError(t, queryErr)
		require.Empty(t, foreignRoles)
		foreignNames, queryErr := identity.FindEffectivePermissionNamesByAccountIDsForTenant(txCtx, ids, other)
		require.NoError(t, queryErr)
		require.Empty(t, foreignNames)
		return nil
	}))

	rollback := errors.New("roll back staff query changes")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, writeErr := tx.NewRaw("UPDATE auth.accounts SET active = FALSE, email = ? WHERE id = ?", "transactional-"+account.Email, account.ID).Exec(txCtx)
		require.NoError(t, writeErr)
		_, writeErr = tx.NewRaw("DELETE FROM auth.account_roles WHERE account_id = ?", account.ID).Exec(txCtx)
		require.NoError(t, writeErr)
		active, queryErr := identity.ListActiveAccountIDsForTenant(txCtx, school, ids)
		require.NoError(t, queryErr)
		require.Empty(t, active)
		emails, queryErr := identity.ListAccountEmails(txCtx, []int64{account.ID})
		require.NoError(t, queryErr)
		require.Equal(t, "transactional-"+account.Email, emails[account.ID])
		counts, queryErr := identity.CountRoleNameMatchesByAccountIDs(txCtx, ids, []string{role.Name})
		require.NoError(t, queryErr)
		require.Empty(t, counts)
		accounts, queryErr := identity.ListAccountIDsWithSystemRoleNames(txCtx, ids, []string{"user"}, other)
		require.NoError(t, queryErr)
		require.Empty(t, accounts)
		names, queryErr := identity.FindEffectivePermissionNamesByAccountIDsForTenant(txCtx, []int64{account.ID}, school)
		require.NoError(t, queryErr)
		require.Equal(t, []string{directGrant.Resource + ":read"}, names[account.ID])
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	active, err = identity.ListActiveAccountIDsForTenant(ctx, school, ids)
	require.NoError(t, err)
	require.Equal(t, []int64{account.ID}, active)
	matches, err = identity.CountRoleNameMatchesByAccountIDs(ctx, ids, []string{role.Name})
	require.NoError(t, err)
	require.Equal(t, map[int64]int{account.ID: 2}, matches)
}

func TestStaffAccountQueriesDatabaseFailuresAndEmptyInputs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	ctx := testpkg.Ctx(t)
	school := testpkg.Tenant(t)
	_, err = identity.ListActiveAccountIDsForTenant(ctx, school, []int64{1})
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, []int64{1}, school)
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.CountRoleNameMatchesByAccountIDs(ctx, []int64{1}, []string{"user"})
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.ListAccountIDsWithSystemRoleNames(ctx, []int64{1}, []string{"user"}, school)
	require.ErrorContains(t, err, "database is closed")
	_, err = identity.ListAccountEmails(ctx, []int64{1})
	require.ErrorContains(t, err, "database is closed")
	active, err := identity.ListActiveAccountIDsForTenant(ctx, 0, []int64{1})
	require.NoError(t, err)
	require.Empty(t, active)
	names, err := identity.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, []int64{1}, 0)
	require.NoError(t, err)
	require.Empty(t, names)
	counts, err := identity.CountRoleNameMatchesByAccountIDs(ctx, []int64{1}, nil)
	require.NoError(t, err)
	require.Empty(t, counts)
	system, err := identity.ListAccountIDsWithSystemRoleNames(ctx, nil, []string{"user"}, school)
	require.NoError(t, err)
	require.Empty(t, system)
	emails, err := identity.ListAccountEmails(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, emails)
}
