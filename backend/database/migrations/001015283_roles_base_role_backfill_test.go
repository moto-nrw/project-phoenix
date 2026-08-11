package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The tier is derived from whether the role may manage users, by the same
// standard CanGrantRole applies, and CanGrantRole asks
// authorize.HasPermission, which matches wildcards. A role granted users:* or
// *:manage may manage users just as much as one granted users:manage, so the
// backfill has to classify all three as admin roles. Reading the permission
// name literally would file the wildcard ones as staff and hand out a tier the
// running code disagrees with.
func TestRolesBaseRoleBackfillMatchesWildcardPermissions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)

	testpkg.EnsureTestTenant(t, db, tenantID)
	defer testpkg.CleanupTenantTestData(t, db, tenantID)

	wildcardResource := ensureBackfillPermission(t, db, "users:*", "users", "*")
	wildcardAction := ensureBackfillPermission(t, db, "*:manage", "*", "manage")
	literal := ensureBackfillPermission(t, db, "users:manage", "users", "manage")
	unrelated := ensureBackfillPermission(t, db, "groups:read", "groups", "read")

	byWildcardResource := createIdentityRepairRole(t, db, tenantID, "backfill-users-star", nil)
	byWildcardAction := createIdentityRepairRole(t, db, tenantID, "backfill-star-manage", nil)
	byLiteral := createIdentityRepairRole(t, db, tenantID, "backfill-literal", nil)
	withoutUserManagement := createIdentityRepairRole(t, db, tenantID, "backfill-plain", nil)
	alreadyClassified := createIdentityRepairRole(t, db, tenantID, "backfill-known", strPtrValue("user"))
	defer cleanupIdentityRepairRoles(t, db,
		byWildcardResource, byWildcardAction, byLiteral, withoutUserManagement, alreadyClassified)

	grantBackfillPermission(t, db, byWildcardResource, wildcardResource)
	grantBackfillPermission(t, db, byWildcardAction, wildcardAction)
	grantBackfillPermission(t, db, byLiteral, literal)
	grantBackfillPermission(t, db, withoutUserManagement, unrelated)
	// A role that already carries its tier keeps it, even when its permissions
	// would suggest otherwise.
	grantBackfillPermission(t, db, alreadyClassified, literal)

	require.NoError(t, rolesBaseRoleBackfillUp(ctx, db))
	require.NoError(t, rolesBaseRoleBackfillUp(ctx, db), "backfill must be idempotent")

	assert.Equal(t, "admin", baseRoleOf(t, db, byWildcardResource), "users:* grants users:manage")
	assert.Equal(t, "admin", baseRoleOf(t, db, byWildcardAction), "*:manage grants users:manage")
	assert.Equal(t, "admin", baseRoleOf(t, db, byLiteral))
	assert.Equal(t, "user", baseRoleOf(t, db, withoutUserManagement))
	assert.Equal(t, "user", baseRoleOf(t, db, alreadyClassified), "an existing tier must not be overwritten")
}

// ensureBackfillPermission returns the id of the named permission, creating it
// when the catalog does not carry it yet. Permissions are global rows, so a
// pre-existing one is left in place rather than cleaned up.
func ensureBackfillPermission(t *testing.T, db *bun.DB, name, resource, action string) int64 {
	t.Helper()
	ctx := context.Background()

	var ids []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.permissions WHERE name = ?`, name).Scan(ctx, &ids))
	if len(ids) > 0 {
		return ids[0]
	}

	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO auth.permissions (name, description, resource, action, created_at, updated_at)
		VALUES (?, 'Backfill-Testrecht', ?, ?, NOW(), NOW())
		RETURNING id`, name, resource, action).Scan(ctx, &id))
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.role_permissions WHERE permission_id = ?`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.permissions WHERE id = ?`, id)
	})
	return id
}

func grantBackfillPermission(t *testing.T, db *bun.DB, roleID, permissionID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO auth.role_permissions (role_id, permission_id, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW())
		ON CONFLICT DO NOTHING`, roleID, permissionID)
	require.NoError(t, err)
}

func baseRoleOf(t *testing.T, db *bun.DB, roleID int64) string {
	t.Helper()
	var values []string
	require.NoError(t, db.NewRaw(
		`SELECT COALESCE(base_role, '') FROM auth.roles WHERE id = ?`, roleID,
	).Scan(context.Background(), &values))
	require.Len(t, values, 1)
	return values[0]
}
