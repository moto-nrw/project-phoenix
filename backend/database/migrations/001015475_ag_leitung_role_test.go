package migrations

// Coverage for 1.15.475 (ADR 0083): the platform system role ag-leitung
// exists once with base role user and schedules:read only, a rerun changes
// nothing, a school's own role of the same name stays untouched, and Down
// removes the role with its grants and assignments.

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type systemRoleRow struct {
	ID          int64   `bun:"id"`
	Description string  `bun:"description"`
	BaseRole    *string `bun:"base_role"`
}

func systemRolesNamed(t *testing.T, db *testpkg.DB, name string) []systemRoleRow {
	t.Helper()
	var rows []systemRoleRow
	require.NoError(t, db.NewRaw(`SELECT id, description, base_role FROM auth.roles
		WHERE name = ? AND tenant_id IS NULL AND is_system ORDER BY id`, name).Scan(context.Background(), &rows))
	return rows
}

func rolePermissionNames(t *testing.T, db *testpkg.DB, roleID int64) []string {
	t.Helper()
	var names []string
	require.NoError(t, db.NewRaw(`SELECT p.name FROM auth.role_permissions rp
		JOIN auth.permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = ? ORDER BY p.name`, roleID).Scan(context.Background(), &names))
	return names
}

func TestAGLeitungRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	assertRole := func(t *testing.T) int64 {
		t.Helper()
		roles := systemRolesNamed(t, db, agLeitungRoleName)
		require.Len(t, roles, 1)
		assert.Equal(t, agLeitungRoleText, roles[0].Description)
		require.NotNil(t, roles[0].BaseRole)
		assert.Equal(t, "user", *roles[0].BaseRole, "base role user keeps the person plannable")
		assert.Equal(t, []string{"schedules:read"}, rolePermissionNames(t, db, roles[0].ID))
		return roles[0].ID
	}
	assertRole(t)

	require.NoError(t, agLeitungRoleUp(ctx, db), "the migration is idempotent")
	roleID := assertRole(t)

	// A school's own role of that name is not the system role.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	var ownID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (tenant_id, name, description, is_system, base_role)
		VALUES (?, 'ag-leitung', 'eigene Rolle', FALSE, 'user') RETURNING id`, tenantID).Scan(ctx, &ownID))
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "AG", "Leitung")
	_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`,
		account.ID, roleID, tenantID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, agLeitungRoleDown(ctx, db))
	assert.Empty(t, systemRolesNamed(t, db, agLeitungRoleName))
	var assigned, own int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.account_roles WHERE role_id = ?`, roleID).Scan(ctx, &assigned))
	assert.Zero(t, assigned, "Down removes the assignments with the role")
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.roles WHERE id = ?`, ownID).Scan(ctx, &own))
	assert.Equal(t, 1, own, "a school's own role of the same name stays")

	require.NoError(t, agLeitungRoleUp(ctx, db))
	assertRole(t)
}
