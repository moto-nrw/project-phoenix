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

// TestMealPermissionsKeepEveryRoleWhereItWas pins #3873: the meal
// permissions land exactly where the old checks granted access, and Down
// removes them again.
func TestMealPermissionsKeepEveryRoleWhereItWas(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, mealPermissionsDown(ctx, db))

	reader := createRoleWith(t, db, "meal-test-reader", "config:read")
	kitchen := createRoleWith(t, db, "meal-test-kitchen", "config:read", "users:read")
	editor := createRoleWith(t, db, "meal-test-editor", "config:update")
	unrelated := createRoleWith(t, db, "meal-test-unrelated", "users:read")
	account := testpkg.CreateTestAccount(t, db, "meal-permission-direct")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	_, err := db.NewRaw(`
		INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
		SELECT ?, ?, id, TRUE FROM auth.permissions WHERE name = 'config:read'
	`, tenantID, account.ID).Exec(ctx)
	require.NoError(t, err)

	// The two old permissions from different sources: config:read from a
	// role, users:read granted directly.
	mixed := testpkg.CreateTestAccount(t, db, "meal-permission-mixed")
	_, err = db.NewRaw(`INSERT INTO auth.account_roles (tenant_id, account_id, role_id) VALUES (?, ?, ?)`, tenantID, mixed.ID, reader).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`
		INSERT INTO auth.account_permissions (tenant_id, account_id, permission_id, granted)
		SELECT ?, ?, id, TRUE FROM auth.permissions WHERE name = 'users:read'
	`, tenantID, mixed.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, mealPermissionsUp(ctx, db))
	require.NoError(t, mealPermissionsUp(ctx, db), "the migration can run twice")

	assert.Equal(t, []string{"meals:read"}, mealPermissionsOfRole(t, db, reader))
	assert.Equal(t, []string{"meals:orders", "meals:read"}, mealPermissionsOfRole(t, db, kitchen))
	assert.Equal(t, []string{"meals:manage"}, mealPermissionsOfRole(t, db, editor))
	assert.Empty(t, mealPermissionsOfRole(t, db, unrelated), "users:read alone opened no meal page")

	var direct []string
	require.NoError(t, db.NewRaw(`
		SELECT p.name FROM auth.account_permissions AS ap
		JOIN auth.permissions AS p ON p.id = ap.permission_id
		WHERE ap.account_id = ? AND ap.tenant_id = ? AND p.resource = 'meals' AND ap.granted
		ORDER BY p.name
	`, account.ID, tenantID).Scan(ctx, &direct))
	assert.Equal(t, []string{"meals:read"}, direct, "a direct grant keeps its school")

	var mixedDirect []string
	require.NoError(t, db.NewRaw(`
		SELECT p.name FROM auth.account_permissions AS ap
		JOIN auth.permissions AS p ON p.id = ap.permission_id
		WHERE ap.account_id = ? AND ap.tenant_id = ? AND p.resource = 'meals' AND ap.granted
		ORDER BY p.name
	`, mixed.ID, tenantID).Scan(ctx, &mixedDirect))
	assert.Equal(t, []string{"meals:orders"}, mixedDirect, "role and direct grant together opened the kitchen list")

	require.NoError(t, mealPermissionsDown(ctx, db))
	assert.Empty(t, mealPermissionsOfRole(t, db, kitchen))
	require.NoError(t, mealPermissionsUp(ctx, db))
}

func createRoleWith(t *testing.T, db *bun.DB, name string, permissions ...string) int64 {
	t.Helper()
	ctx := context.Background()
	var roleID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (name, description) VALUES (?, 'test') RETURNING id`, fmt.Sprintf("%s-%d", name, testpkg.UniqueTestTenantID(t))).Scan(ctx, &roleID))
	for _, permission := range permissions {
		_, err := db.NewRaw(`
			INSERT INTO auth.role_permissions (role_id, permission_id)
			SELECT ?, id FROM auth.permissions WHERE name = ?
		`, roleID, permission).Exec(ctx)
		require.NoError(t, err)
	}
	return roleID
}

func mealPermissionsOfRole(t *testing.T, db *bun.DB, roleID int64) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.NewRaw(`
		SELECT p.name FROM auth.role_permissions AS rp
		JOIN auth.permissions AS p ON p.id = rp.permission_id
		WHERE rp.role_id = ? AND p.resource = 'meals'
		ORDER BY p.name
	`, roleID).Scan(context.Background(), &names))
	return names
}
