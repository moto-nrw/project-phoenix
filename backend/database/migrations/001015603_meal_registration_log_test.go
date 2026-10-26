package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMealRegistrationLogGrantsNoRoleAndOnlyGrows pins #3873: meals:register
// exists for no role, seeing the lists opens no change, the log can only be
// appended to, and Down removes both again.
func TestMealRegistrationLogGrantsNoRoleAndOnlyGrows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := context.Background()
	kitchen := createRoleWith(t, db, "meal-log-kitchen", "meals:orders", "meals:read")
	require.NoError(t, mealRegistrationLogDown(ctx, db))
	require.NoError(t, mealRegistrationLogUp(ctx, db))

	var holders int
	require.NoError(t, db.NewRaw(`
		SELECT count(*) FROM auth.role_permissions AS rp
		JOIN auth.permissions AS p ON p.id = rp.permission_id
		WHERE p.name = 'meals:register'
	`).Scan(ctx, &holders))
	assert.Zero(t, holders, "no role registers children until the school grants it")
	assert.Equal(t, []string{"meals:orders", "meals:read"}, mealPermissionsOfRole(t, db, kitchen))

	var canInsert, canUpdate, canDelete bool
	require.NoError(t, db.NewRaw(`
		SELECT has_table_privilege('phoenix_tenant', 'schedule.meal_registration_changes', 'INSERT'),
			has_table_privilege('phoenix_tenant', 'schedule.meal_registration_changes', 'UPDATE'),
			has_table_privilege('phoenix_tenant', 'schedule.meal_registration_changes', 'DELETE')
	`).Scan(ctx, &canInsert, &canUpdate, &canDelete))
	assert.True(t, canInsert)
	assert.False(t, canUpdate, "the log is never rewritten")
	assert.False(t, canDelete, "the log goes only with the child")

	for _, table := range []string{"schedule.meal_order_days", "schedule.meal_order_lines", "schedule.meal_order_changes"} {
		var rewritable bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', ?, 'UPDATE') OR has_table_privilege('phoenix_tenant', ?, 'DELETE')`, table, table).Scan(ctx, &rewritable))
		assert.False(t, rewritable, table)
	}

	require.NoError(t, mealRegistrationLogDown(ctx, db))
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.permissions WHERE name = 'meals:register'`).Scan(ctx, &remaining))
	assert.Zero(t, remaining)
	require.NoError(t, mealRegistrationLogUp(ctx, db))
}
