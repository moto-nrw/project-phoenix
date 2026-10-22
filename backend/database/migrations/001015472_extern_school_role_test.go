package migrations

// Coverage for the 1.15.472 backfill (#4573): every live school ends up with
// exactly one „Extern“ role with schedules:read, a school that already has a
// role of that name keeps it untouched, and a rerun changes nothing.

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type externRoleRow struct {
	ID          int64   `bun:"id"`
	Name        string  `bun:"name"`
	Description string  `bun:"description"`
	IsSystem    bool    `bun:"is_system"`
	BaseRole    *string `bun:"base_role"`
}

func externRolesOf(t *testing.T, db *testpkg.DB, tenantID int64) []externRoleRow {
	t.Helper()
	var rows []externRoleRow
	require.NoError(t, db.NewRaw(`
		SELECT id, name, description, is_system, base_role FROM auth.roles
		WHERE tenant_id = ? AND LOWER(TRIM(name)) = 'extern' ORDER BY id`, tenantID).Scan(context.Background(), &rows))
	return rows
}

func externRolePermissionNames(t *testing.T, db *testpkg.DB, roleID int64) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.NewRaw(`
		SELECT p.name FROM auth.role_permissions rp JOIN auth.permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = ? ORDER BY p.name`, roleID).Scan(context.Background(), &names))
	return names
}

func TestExternSchoolRoleBackfill(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	withoutRole := testpkg.UniqueTestTenantID(t)
	withRole := testpkg.UniqueTestTenantID(t)
	deleted := testpkg.UniqueTestTenantID(t)
	for _, tenantID := range []int64{withoutRole, withRole, deleted} {
		testpkg.EnsureTestTenant(t, db, tenantID)
		testpkg.OwnTenantRows(t, db, tenantID)
	}
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, deleted).Exec(ctx)
	require.NoError(t, err)

	// The school already created its own „ Extern “ role with another
	// permission: the backfill must neither duplicate nor change it.
	var existingID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (tenant_id, name, description, is_system, base_role)
		VALUES (?, ' Extern ', 'eigene Rolle', FALSE, 'user') RETURNING id`, withRole).Scan(ctx, &existingID))
	_, err = db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ?, id FROM auth.permissions WHERE name = 'groups:read'`, existingID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, externSchoolRoleUp(ctx, db))

	created := externRolesOf(t, db, withoutRole)
	require.Len(t, created, 1, "a school without the role gets exactly one")
	assert.Equal(t, "extern", created[0].Name, "stored like every custom role, in lower case")
	assert.Equal(t, externRoleDescription, created[0].Description)
	assert.False(t, created[0].IsSystem, "the school must be able to edit the role")
	require.NotNil(t, created[0].BaseRole)
	assert.Equal(t, "user", *created[0].BaseRole)
	assert.Equal(t, []string{"schedules:read"}, externRolePermissionNames(t, db, created[0].ID))

	kept := externRolesOf(t, db, withRole)
	require.Len(t, kept, 1, "an existing role of that name is not duplicated")
	assert.Equal(t, existingID, kept[0].ID)
	assert.Equal(t, "eigene Rolle", kept[0].Description)
	assert.Equal(t, []string{"groups:read"}, externRolePermissionNames(t, db, existingID), "the existing role keeps its permissions")

	assert.Empty(t, externRolesOf(t, db, deleted), "a deleted school gets no role")

	var before int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.roles`).Scan(ctx, &before))
	require.NoError(t, externSchoolRoleUp(ctx, db), "the backfill is idempotent")
	var after int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.roles`).Scan(ctx, &after))
	assert.Equal(t, before, after, "a rerun inserts nothing")
	assert.Equal(t, []string{"schedules:read"}, externRolePermissionNames(t, db, created[0].ID))
	assert.Equal(t, []string{"groups:read"}, externRolePermissionNames(t, db, existingID))
}

func TestExternSchoolRoleDownKeepsTheRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	require.NoError(t, externSchoolRoleUp(ctx, db))
	require.NoError(t, externSchoolRoleDown(ctx, db))
	assert.Len(t, externRolesOf(t, db, tenantID), 1)
}
