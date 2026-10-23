package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamChatMigrationResetsOnlyOptInOffs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	optInOff := testpkg.Tenant(t)
	deliberateOff, _ := testpkg.CreateTestTenant(t, db)
	optInOn, _ := testpkg.CreateTestTenant(t, db)
	testpkg.OwnTenantRows(t, db, deliberateOff, optInOn)

	// The updated_at trigger fires on UPDATE only, so an insert keeps the
	// given time: two values from the opt-in time, one written after it.
	_, err := db.ExecContext(ctx, `
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_at)
		VALUES (?, 'operations.staff_messaging_enabled', 'false'::jsonb, '2026-09-10T08:00:00Z'),
		       (?, 'operations.staff_messaging_enabled', 'false'::jsonb, '2026-09-24T08:00:00Z'),
		       (?, 'operations.staff_messaging_enabled', 'true'::jsonb, '2026-09-10T08:00:00Z')`,
		optInOff, deliberateOff, optInOn)
	require.NoError(t, err)

	require.NoError(t, teamChatOptInOffsUp(ctx, db))

	var remaining []int64
	require.NoError(t, db.NewRaw(`
		SELECT tenant_id FROM config.setting_values
		WHERE setting_key = 'operations.staff_messaging_enabled' AND tenant_id IN (?, ?, ?)
		ORDER BY tenant_id`, optInOff, deliberateOff, optInOn).
		Scan(ctx, &remaining))
	assert.ElementsMatch(t, []int64{deliberateOff, optInOn}, remaining,
		"only the off from the opt-in time is removed")

	var audited int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id = ? AND setting_key = 'operations.staff_messaging_enabled'
		  AND action = 'reset' AND old_value = 'false'::jsonb AND changed_by IS NULL`,
		optInOff).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the reset is recorded in the settings audit")
}
