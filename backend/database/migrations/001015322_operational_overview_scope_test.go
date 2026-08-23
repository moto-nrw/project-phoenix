package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOperationalOverviewScopeDerivesFromExistingRules pins the promise the
// migration makes to schools already running: nobody loses the access they
// had. Open care was the broader of the two old rules, so it wins over the
// admin flag; a school on neither stays on the restrictive default and gets
// no row at all.
func TestOperationalOverviewScopeDerivesFromExistingRules(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	openCareTenant, _ := testpkg.CreateTestTenant(t, db)
	adminOverviewTenant, _ := testpkg.CreateTestTenant(t, db)
	bothTenant, _ := testpkg.CreateTestTenant(t, db)
	untouchedTenant, _ := testpkg.CreateTestTenant(t, db)
	tenants := []int64{openCareTenant, adminOverviewTenant, bothTenant, untouchedTenant}

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES
			(?, 'operations.group_mode', '"open_care"'::jsonb),
			(?, 'operations.admin_supervision_overview', 'true'::jsonb),
			(?, 'operations.group_mode', '"open_care"'::jsonb),
			(?, 'operations.admin_supervision_overview', 'true'::jsonb),
			(?, 'operations.group_mode', '"fixed_groups"'::jsonb)
	`, openCareTenant, adminOverviewTenant, bothTenant, bothTenant, untouchedTenant).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, operationalOverviewScopeUp(ctx, db))

	scopeOf := func(tenantID int64) string {
		var scope []string
		require.NoError(t, db.NewRaw(`
			SELECT value #>> '{}' FROM config.setting_values
			WHERE tenant_id = ? AND setting_key = 'operations.operational_overview_scope'
		`, tenantID).Scan(ctx, &scope))
		if len(scope) == 0 {
			return ""
		}
		return scope[0]
	}

	assert.Equal(t, "all_staff", scopeOf(openCareTenant), "open care kept its school-wide access")
	assert.Equal(t, "admins", scopeOf(adminOverviewTenant), "the admin flag became the admins scope")
	assert.Equal(t, "all_staff", scopeOf(bothTenant), "the broader of the two rules wins")
	assert.Empty(t, scopeOf(untouchedTenant), "a school on neither rule keeps the registry default")

	// The retired key must leave nothing behind: a row without a definition
	// is an orphan the settings UI can neither show nor reset.
	var leftovers int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_values
		WHERE setting_key = 'operations.admin_supervision_overview'
	`).Scan(ctx, &leftovers))
	assert.Zero(t, leftovers)

	// The organisational group mode is untouched — it describes how the
	// school organises children, and the migration only reads it.
	var groupModes int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_values
		WHERE tenant_id IN (?, ?, ?) AND setting_key = 'operations.group_mode'
	`, openCareTenant, bothTenant, untouchedTenant).Scan(ctx, &groupModes))
	assert.Equal(t, 3, groupModes)

	var auditCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id IN (?, ?, ?, ?)
			AND setting_key = 'operations.operational_overview_scope'
			AND action = 'set'
	`, tenants[0], tenants[1], tenants[2], tenants[3]).Scan(ctx, &auditCount))
	assert.Equal(t, 3, auditCount, "every derived value is recorded in the audit trail")
}
