package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataCleanupSwitchMigrationResetsStoredSwitchOffs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	switchedOff := testpkg.Tenant(t)
	switchedOn, _ := testpkg.CreateTestTenant(t, db)
	testpkg.OwnTenantRows(t, db, switchedOn)

	_, err := db.ExecContext(ctx, `
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.data_cleanup_enabled', 'false'::jsonb), (?, 'gdpr.data_cleanup_enabled', 'true'::jsonb)`,
		switchedOff, switchedOn)
	require.NoError(t, err)

	require.NoError(t, dataCleanupSwitchOperatorOnlyUp(ctx, db))

	var remaining []bool
	require.NoError(t, db.NewRaw(`
		SELECT (value #>> '{}')::boolean FROM config.setting_values
		WHERE setting_key = 'gdpr.data_cleanup_enabled' AND tenant_id IN (?, ?)`, switchedOff, switchedOn).
		Scan(ctx, &remaining))
	assert.Equal(t, []bool{true}, remaining, "only the stored off is removed")

	var audited int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id = ? AND setting_key = 'gdpr.data_cleanup_enabled' AND action = 'reset' AND old_value = 'false'::jsonb`,
		switchedOff).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the reset is recorded in the settings audit")
}
