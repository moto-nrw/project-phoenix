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
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
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

func TestMealPermissionsMoveStoredLeadDaysWithoutReplacingExplicitValues(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := context.Background()
	legacyTenant, _ := testpkg.CreateTestTenant(t, db)
	explicitTenant, _ := testpkg.CreateTestTenant(t, db)
	untouchedTenant, _ := testpkg.CreateTestTenant(t, db)
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES
			(?, 'operations.meal_registration_cutoff_lead_days', '2'::jsonb),
			(?, 'operations.meal_registration_cutoff_lead_days', '3'::jsonb),
			(?, 'operations.meal_registration_lead_days', '0'::jsonb)
	`, legacyTenant, explicitTenant, explicitTenant).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, mealPermissionsUp(ctx, db))
	require.NoError(t, mealPermissionsUp(ctx, db), "a rerun preserves the migrated values")
	values := func(key string) map[int64]string {
		t.Helper()
		var rows []struct {
			TenantID int64
			Value    string
		}
		require.NoError(t, db.NewRaw(`
			SELECT tenant_id, value #>> '{}' AS value FROM config.setting_values
			WHERE setting_key = ? AND tenant_id IN (?, ?, ?)
		`, key, legacyTenant, explicitTenant, untouchedTenant).Scan(ctx, &rows))
		result := make(map[int64]string, len(rows))
		for _, row := range rows {
			result[row.TenantID] = row.Value
		}
		return result
	}
	assert.Equal(t, map[int64]string{legacyTenant: "4", explicitTenant: "0"}, values("operations.meal_registration_lead_days"),
		"two working days become four calendar days; the explicit value wins")
	assert.Empty(t, values("operations.meal_registration_cutoff_lead_days"))

	require.NoError(t, mealPermissionsDown(ctx, db))
	assert.Equal(t, map[int64]string{legacyTenant: "2", explicitTenant: "0"}, values("operations.meal_registration_cutoff_lead_days"))
	assert.Empty(t, values("operations.meal_registration_lead_days"))
}

// The cutoffs of #3164 carry over into the ordering rules: a weekday of the
// previous week, a separate cancellation cutoff with its lead days, and an
// explicit ordering rule wins.
func TestMealPermissionsAdoptTheCutoffSettingsOfDevelopment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := context.Background()
	weekly, _ := testpkg.CreateTestTenant(t, db)
	separate, _ := testpkg.CreateTestTenant(t, db)
	explicit, _ := testpkg.CreateTestTenant(t, db)
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES
			(?, 'operations.meal_registration_cutoff_weekday', '3'::jsonb),
			(?, 'operations.meal_cancellation_cutoff_separate', 'true'::jsonb),
			(?, 'operations.meal_cancellation_cutoff_lead_days', '1'::jsonb),
			(?, 'operations.meal_registration_cutoff_weekday', '0'::jsonb),
			(?, 'operations.meal_cancellation_cutoff_separate', 'true'::jsonb),
			(?, 'operations.meal_registration_model', '"annual"'::jsonb)
	`, weekly, separate, separate, separate, explicit, explicit).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, mealPermissionsUp(ctx, db))
	require.NoError(t, mealPermissionsUp(ctx, db), "a rerun preserves the adopted values")
	settings := func(tenantID int64) map[string]string {
		t.Helper()
		var rows []struct {
			SettingKey string
			Value      string
		}
		require.NoError(t, db.NewRaw(`
			SELECT setting_key, value #>> '{}' AS value FROM config.setting_values
			WHERE tenant_id = ? AND setting_key LIKE 'operations.meal_%'
		`, tenantID).Scan(ctx, &rows))
		result := make(map[string]string, len(rows))
		for _, row := range rows {
			result[row.SettingKey] = row.Value
		}
		return result
	}
	assert.Equal(t, map[string]string{
		"operations.meal_registration_deadline_kind":    "previous_week",
		"operations.meal_registration_deadline_weekday": "wednesday",
	}, settings(weekly))
	assert.Equal(t, map[string]string{
		"operations.meal_registration_model":         "ongoing_split",
		"operations.meal_cancellation_deadline_kind": "days_before",
		"operations.meal_cancellation_lead_days":     "3",
	}, settings(separate), "a weekday of 0 was no weekly cutoff; one working day is three calendar days")
	assert.Equal(t, map[string]string{
		"operations.meal_registration_model":         "annual",
		"operations.meal_cancellation_deadline_kind": "days_before",
	}, settings(explicit), "the explicit model wins")

	require.NoError(t, mealPermissionsDown(ctx, db))
	assert.Equal(t, "3", settings(weekly)["operations.meal_registration_cutoff_weekday"])
	assert.Equal(t, "true", settings(separate)["operations.meal_cancellation_cutoff_separate"])
	assert.Equal(t, "1", settings(separate)["operations.meal_cancellation_cutoff_lead_days"])
}

// Development counted lead days as working days (#4866), the ordering rules
// count calendar days. Up stores the longest span N working days cover
// across weekends, N + 2*ceil(N/5), so no deadline moves later without a
// holiday; Down restores exactly N.
func TestMealPermissionsConvertWorkingLeadDaysToCalendarDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := context.Background()
	conversions := map[string]string{"0": "0", "1": "3", "2": "4", "5": "7", "6": "10"}
	tenants := make(map[int64]string, len(conversions))
	for working := range conversions {
		tenantID, _ := testpkg.CreateTestTenant(t, db)
		tenants[tenantID] = working
		_, err := db.NewRaw(`
			INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES
				(?, 'operations.meal_registration_cutoff_lead_days', ?::jsonb),
				(?, 'operations.meal_cancellation_cutoff_separate', 'true'::jsonb),
				(?, 'operations.meal_cancellation_cutoff_lead_days', ?::jsonb)
		`, tenantID, working, tenantID, tenantID, working).Exec(ctx)
		require.NoError(t, err)
	}
	stored := func(tenantID int64, key string) string {
		t.Helper()
		var value string
		require.NoError(t, db.NewRaw(`
			SELECT value #>> '{}' FROM config.setting_values WHERE tenant_id = ? AND setting_key = ?
		`, tenantID, key).Scan(ctx, &value))
		return value
	}

	require.NoError(t, mealPermissionsUp(ctx, db))
	require.NoError(t, mealPermissionsUp(ctx, db), "a rerun converts nothing twice")
	for tenantID, working := range tenants {
		assert.Equal(t, conversions[working], stored(tenantID, "operations.meal_registration_lead_days"), "registration lead of %s working days", working)
		assert.Equal(t, conversions[working], stored(tenantID, "operations.meal_cancellation_lead_days"), "cancellation lead of %s working days", working)
	}

	require.NoError(t, mealPermissionsDown(ctx, db))
	for tenantID, working := range tenants {
		assert.Equal(t, working, stored(tenantID, "operations.meal_registration_cutoff_lead_days"), "Down restores %s working days", working)
		assert.Equal(t, working, stored(tenantID, "operations.meal_cancellation_cutoff_lead_days"), "Down restores %s working days", working)
	}
	require.NoError(t, mealPermissionsUp(ctx, db))
}
