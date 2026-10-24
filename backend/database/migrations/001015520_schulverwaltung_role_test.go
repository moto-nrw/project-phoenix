package migrations

// Coverage for 1.15.520 (#4789): the system role schulverwaltung is a
// school-portal role with every Lehrkraft permission plus
// class_day:all_classes, a rerun changes nothing, a school's own role of the
// same name stays untouched, and Down removes role and permission.

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchulverwaltungRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	permissionsOf := func(t *testing.T, role string) []string {
		t.Helper()
		var names []string
		require.NoError(t, db.NewRaw(`SELECT p.name FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE r.name = ? AND r.tenant_id IS NULL ORDER BY p.name`, role).Scan(ctx, &names))
		return names
	}

	var schoolPortal, isSystem bool
	require.NoError(t, db.NewRaw(`SELECT school_portal, is_system FROM auth.roles
		WHERE name = 'schulverwaltung' AND tenant_id IS NULL`).Scan(ctx, &schoolPortal, &isSystem))
	assert.True(t, schoolPortal, "the role opens moto schule")
	assert.True(t, isSystem)

	granted := permissionsOf(t, "schulverwaltung")
	assert.Contains(t, granted, "class_day:all_classes")
	for _, permission := range permissionsOf(t, "lehrkraft") {
		assert.Contains(t, granted, permission, "the Schulverwaltung holds every Lehrkraft permission")
	}
	assert.NotContains(t, permissionsOf(t, "lehrkraft"), "class_day:all_classes", "a Lehrkraft keeps the assigned classes")

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	own := testpkg.CreateTestRoleForTenant(t, db, "schulverwaltung", tenantID)

	require.NoError(t, schulverwaltungRoleUp(ctx, db), "the migration is idempotent")
	assert.Equal(t, granted, permissionsOf(t, "schulverwaltung"))
	var ownGrants int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.role_permissions WHERE role_id = ?`, own.ID).Scan(ctx, &ownGrants))
	assert.Zero(t, ownGrants, "a school's own role of the same name gets nothing")

	require.NoError(t, schulverwaltungRoleDown(ctx, db))
	var roles, permissions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.roles WHERE name = 'schulverwaltung' AND tenant_id IS NULL`).Scan(ctx, &roles))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.permissions WHERE name = 'class_day:all_classes'`).Scan(ctx, &permissions))
	assert.Zero(t, roles)
	assert.Zero(t, permissions)

	require.NoError(t, schulverwaltungRoleUp(ctx, db))
	assert.Equal(t, granted, permissionsOf(t, "schulverwaltung"))
}
