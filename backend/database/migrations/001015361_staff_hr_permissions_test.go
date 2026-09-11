package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// staffHRPermissionNames are the three permissions #2906 splits out of
// users:update.
var staffHRPermissionNames = []string{
	staffStammdatenPermissionName,
	staffDocumentsPermissionName,
	staffManagePermissionName,
}

// TestStaffHRPermissionsGrantedToAdminOnly pins the point of #2906: the three
// personnel permissions exist, the OGS-Leitung role holds them, and the plain
// `user` (Betreuer) role — which holds users:update for the child-data
// surfaces — does not.
func TestStaffHRPermissionsGrantedToAdminOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// Re-running the migration must change nothing: it is already applied in
	// the package clone, so this asserts idempotency and leaves no rows behind.
	require.NoError(t, addStaffHRPermissions(ctx, db))
	require.NoError(t, addStaffHRPermissions(ctx, db))

	permissionExists := func(name string) bool {
		var count int
		require.NoError(t, db.NewRaw(
			`SELECT COUNT(*) FROM auth.permissions WHERE name = ?`, name,
		).Scan(ctx, &count))
		return count > 0
	}

	roleHolds := func(roleName, permissionName string) bool {
		var count int
		require.NoError(t, db.NewRaw(`
			SELECT COUNT(*)
			FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id
			JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE r.name = ? AND p.name = ?
		`, roleName, permissionName).Scan(ctx, &count))
		return count > 0
	}

	for _, name := range staffHRPermissionNames {
		assert.True(t, permissionExists(name), "permission %s must exist", name)
		assert.True(t, roleHolds("admin", name), "admin must hold %s", name)
		assert.False(t, roleHolds("user", name),
			"the Betreuer role must NOT hold %s (#2906)", name)
	}

	// users:update itself is untouched — it still gates the child-data
	// surfaces the Betreuer role needs.
	assert.True(t, roleHolds("user", "users:update"),
		"users:update must stay on the Betreuer role")
}
