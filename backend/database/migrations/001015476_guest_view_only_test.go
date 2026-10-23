package migrations

// Coverage for 1.15.476 (ADR 0089): the system role guest only views. It
// holds exactly students:read, groups:read, rooms:read and activities:read,
// no other role holds students:read, a school's own role named guest stays
// untouched, and Down restores the permissions of migration 1.7.0.

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuestViewOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	guest := systemRolesNamed(t, db, "guest")
	require.Len(t, guest, 1)
	guestID := guest[0].ID
	viewOnly := []string{"activities:read", "groups:read", "rooms:read", "students:read"}
	assertViewOnly := func(t *testing.T) {
		t.Helper()
		assert.Equal(t, viewOnly, rolePermissionNames(t, db, guestID))
		assert.Equal(t, guestViewOnlyText, systemRolesNamed(t, db, "guest")[0].Description)
		var holders []string
		require.NoError(t, db.NewRaw(`SELECT r.name FROM auth.role_permissions rp
			JOIN auth.roles r ON r.id = rp.role_id JOIN auth.permissions p ON p.id = rp.permission_id
			WHERE p.name = 'students:read' ORDER BY r.name`).Scan(ctx, &holders))
		assert.Equal(t, []string{"guest"}, holders, "no other role receives students:read")
	}
	assertViewOnly(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)
	var ownID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.roles (tenant_id, name, description, is_system, base_role)
		VALUES (?, 'guest', 'eigene Rolle', FALSE, 'user') RETURNING id`, tenantID).Scan(ctx, &ownID))
	_, err := db.NewRaw(`INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT ?, id FROM auth.permissions WHERE name = 'users:read'`, ownID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, guestViewOnlyUp(ctx, db), "the migration is idempotent")
	assertViewOnly(t)
	assert.Equal(t, []string{"users:read"}, rolePermissionNames(t, db, ownID), "a school's own role stays untouched")

	require.NoError(t, guestViewOnlyDown(ctx, db))
	assert.Equal(t, []string{"activities:create", "activities:read", "rooms:read", "users:list", "users:read"}, rolePermissionNames(t, db, guestID))
	assert.Equal(t, guestPreviousText, systemRolesNamed(t, db, "guest")[0].Description)
	var permissions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.permissions WHERE name = 'students:read'`).Scan(ctx, &permissions))
	assert.Zero(t, permissions, "Down removes students:read")

	require.NoError(t, guestViewOnlyUp(ctx, db))
	assertViewOnly(t)
}
