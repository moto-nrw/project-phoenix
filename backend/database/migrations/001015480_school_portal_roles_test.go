package migrations

// Coverage for 1.15.480 (#4574): auth.roles.school_portal exists with
// default false, the push view counts marked roles as school roles, a rerun
// changes nothing, and Down restores the view of 1.15.468 without the column.

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchoolPortalRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	state := func(t *testing.T) (columnExists bool, pushView string) {
		t.Helper()
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'auth' AND table_name = 'roles' AND column_name = 'school_portal')`).Scan(ctx, &columnExists))
		require.NoError(t, db.NewRaw(`SELECT pg_get_viewdef('platform.delivery_push_subscriptions'::regclass)`).Scan(ctx, &pushView))
		return columnExists, pushView
	}

	exists, view := state(t)
	require.True(t, exists)
	assert.Contains(t, view, "school_portal")
	var marked int
	// The system role schulverwaltung (1.15.505, #4789) is marked by its own
	// migration; 1.15.480 marks none of the schools' own roles.
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.roles WHERE school_portal AND tenant_id IS NOT NULL`).Scan(ctx, &marked))
	assert.Zero(t, marked, "no existing school role is marked")

	require.NoError(t, schoolPortalRolesUp(ctx, db), "the migration is idempotent")
	exists, view = state(t)
	require.True(t, exists)
	assert.Contains(t, view, "school_portal")

	require.NoError(t, schoolPortalRolesDown(ctx, db))
	exists, view = state(t)
	assert.False(t, exists)
	assert.NotContains(t, view, "school_portal")
	assert.Contains(t, view, "lehrkraft", "Down keeps the system role's school-portal branch")

	require.NoError(t, schoolPortalRolesUp(ctx, db))
	exists, _ = state(t)
	assert.True(t, exists)
}
