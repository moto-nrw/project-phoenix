package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentCareRequestFieldSettingsPreserveEffectiveMessagingBehavior(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	enabledTenant, _ := testpkg.CreateTestTenant(t, db)
	disabledTenant, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() {
		_, _ = db.NewRaw(`
			DELETE FROM config.setting_audit
			WHERE tenant_id IN (?, ?)
				AND setting_key LIKE 'operations.parent_care_%_request_enabled';
			DELETE FROM config.setting_values
			WHERE tenant_id IN (?, ?)
				AND setting_key LIKE 'operations.parent_care_%_request_enabled';
		`, enabledTenant, disabledTenant, enabledTenant, disabledTenant).Exec(context.Background())
	})
	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'operations.parent_notes_enabled', 'false'::jsonb)
	`, disabledTenant).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, parentCareRequestFieldSettingsUp(ctx, db))

	type row struct {
		TenantID int64  `bun:"tenant_id"`
		Key      string `bun:"setting_key"`
		Enabled  bool   `bun:"enabled"`
	}
	var rows []row
	require.NoError(t, db.NewRaw(`
		SELECT tenant_id, setting_key, (value #>> '{}')::boolean AS enabled
		FROM config.setting_values
		WHERE tenant_id IN (?, ?)
			AND setting_key LIKE 'operations.parent_care_%_request_enabled'
		ORDER BY tenant_id, setting_key
	`, enabledTenant, disabledTenant).Scan(ctx, &rows))
	require.Len(t, rows, 4)
	for _, got := range rows {
		assert.Equal(t, got.TenantID == enabledTenant, got.Enabled, got.Key)
	}

	var auditCount int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_audit
		WHERE tenant_id IN (?, ?)
			AND setting_key LIKE 'operations.parent_care_%_request_enabled'
			AND action = 'set'
	`, enabledTenant, disabledTenant).Scan(ctx, &auditCount))
	assert.Equal(t, 4, auditCount)
}
