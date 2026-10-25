package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMealCombinedRolesKeepKitchenAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	school := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	testpkg.EnsureTestTenant(t, db, other)
	reader := createRoleWith(t, db, "combined-reader", "config:read")
	users := createRoleWith(t, db, "combined-users", "users:read")
	combined := testpkg.CreateTestAccount(t, db, "combined-meal-roles")
	partial := testpkg.CreateTestAccount(t, db, "partial-meal-role")
	assign := func(account, tenant, role int64) {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.account_roles (account_id, tenant_id, role_id) VALUES (?, ?, ?)`, account, tenant, role)
		require.NoError(t, err)
	}
	assign(combined.ID, school, reader)
	assign(combined.ID, school, users)
	assign(partial.ID, school, reader)
	assign(combined.ID, other, reader)
	direct := testpkg.CreateTestAccount(t, db, "retired-direct-meal-role")
	assign(direct.ID, school, reader)
	_, err := db.ExecContext(ctx, `INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
		SELECT ?, ?, id, TRUE FROM auth.permissions WHERE name = 'users:read'`, school, direct.ID)
	require.NoError(t, err)

	require.NoError(t, mealPermissionsUp(ctx, db))
	require.NoError(t, mealCombinedRolePermissionsUp(ctx, db))
	require.NoError(t, mealCombinedRolePermissionsUp(ctx, db), "rerunning must not duplicate grants")
	permissions := func(account, tenant int64) []string {
		var names []string
		require.NoError(t, withPhoenixTenantTx(t, db, tenant, func(ctx context.Context, tx testpkg.Tx) error {
			return tx.NewRaw(`SELECT DISTINCT p.resource || ':' || p.action
				FROM auth.account_roles ar
				JOIN auth.role_permissions rp ON rp.role_id = ar.role_id
				JOIN auth.permissions p ON p.id = rp.permission_id
				WHERE ar.account_id = ? AND ar.tenant_id = ?`, account, tenant).Scan(ctx, &names)
		}))
		return names
	}
	assert.Contains(t, permissions(combined.ID, school), "meals:orders", "session permissions must include the migrated kitchen access")
	assert.NotContains(t, permissions(partial.ID, school), "meals:orders", "one old permission alone must not gain kitchen access")
	assert.NotContains(t, permissions(combined.ID, other), "meals:orders", "the combined access must stay in its school")
	assert.NotContains(t, permissions(direct.ID, school), "meals:orders", "retired direct grants must not become effective again")
	assert.Equal(t, []string{"meals:read"}, mealPermissionsOfRole(t, db, reader), "other members of the original role must not gain access")
	assert.Empty(t, mealPermissionsOfRole(t, db, users))
	var assignments int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_roles WHERE account_id = ? AND tenant_id = ?`, combined.ID, school).Scan(ctx, &assignments))
	assert.Equal(t, 3, assignments, "rerunning creates exactly one supplemental assignment")
}

func TestMealCombinedRolesPreservePortalScopeAndRollBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	school := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, school)
	reader := createRoleWith(t, db, "school-meal-reader", "config:read")
	users := createRoleWith(t, db, "school-meal-users", "users:read")
	_, err := db.ExecContext(ctx, `UPDATE auth.roles SET school_portal = TRUE WHERE id IN (?, ?)`, reader, users)
	require.NoError(t, err)
	empty := createRoleWith(t, db, "empty-ogs-role")
	accounts := []int64{}
	for _, mixed := range []bool{false, true} {
		account := testpkg.CreateTestAccount(t, db, "school-meal-combined")
		accounts = append(accounts, account.ID)
		_, err = db.ExecContext(ctx, `INSERT INTO auth.account_roles (account_id, tenant_id, role_id) VALUES (?, ?, ?), (?, ?, ?)`, account.ID, school, reader, account.ID, school, users)
		require.NoError(t, err)
		if mixed {
			_, err = db.ExecContext(ctx, `INSERT INTO auth.account_roles (account_id, tenant_id, role_id) VALUES (?, ?, ?)`, account.ID, school, empty)
			require.NoError(t, err)
		}
	}
	local := testpkg.CreateTestRoleForTenant(t, db, "meal-orders-migrated", school)
	require.NoError(t, mealPermissionsUp(ctx, db))
	require.NoError(t, mealCombinedRolePermissionsUp(ctx, db))
	for i, account := range accounts {
		var schoolOnly bool
		require.NoError(t, db.NewRaw(`SELECT bool_and(r.school_portal) FROM auth.account_roles ar
			JOIN auth.roles r ON r.id = ar.role_id WHERE ar.account_id = ? AND ar.tenant_id = ?`, account, school).Scan(ctx, &schoolOnly))
		assert.Equal(t, i == 0, schoolOnly, "a role without permissions still determines portal access")
		var granted []string
		require.NoError(t, db.NewRaw(`SELECT p.name FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id
			JOIN auth.role_permissions rp ON rp.role_id = r.id JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE ar.account_id = ? AND ar.tenant_id = ? AND r.name LIKE 'meal-orders-migrated%'`, account, school).Scan(ctx, &granted))
		assert.Equal(t, []string{"meals:orders"}, granted, "the supplemental role grants exactly the missing permission")
	}
	require.NoError(t, mealCombinedRolePermissionsDown(ctx, db))
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.roles WHERE name LIKE 'meal-orders-migrated%' AND tenant_id IS NULL`).Scan(ctx, &remaining))
	assert.Zero(t, remaining)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.roles WHERE id = ?`, local.ID).Scan(ctx, &remaining))
	assert.Equal(t, 1, remaining, "a school's role of the same name stays untouched")
	require.NoError(t, mealCombinedRolePermissionsUp(ctx, db), "the forward migration works again after rollback")
}

func TestMealCombinedRolesRejectIncompatibleExistingRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	role := createRoleWith(t, db, "migration-role-collision", "config:update")
	_, err := db.ExecContext(ctx, `UPDATE auth.roles SET name = 'meal-orders-migrated' WHERE id = ?`, role)
	require.NoError(t, err)
	require.ErrorContains(t, mealCombinedRolePermissionsUp(ctx, db), "incompatible permissions or portal scope")
	var names []string
	require.NoError(t, db.NewRaw(`SELECT p.name FROM auth.role_permissions rp JOIN auth.permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = ?`, role).Scan(ctx, &names))
	assert.Equal(t, []string{"config:update"}, names, "an existing role must never receive broader permissions through its name")
}
