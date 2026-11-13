package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	configRepository "github.com/moto-ogs/project-phoenix/modules/settings/internal/adapters/configpostgres"
	config "github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A setting change commits only with its audit entry (#6848). The audit
// INSERT fails here in SQL, which aborts the transaction: a swallowed error
// would let COMMIT end as a silent ROLLBACK and report a save that never
// happened. Both the standalone command and a request transaction must fail.
func TestSettingChangeFailsWhenTheAuditInsertFails(t *testing.T) {
	t.Parallel()
	// The CHECK constraint is schema DDL, so it gets a clone of its own.
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.ContextForTenant(context.Background(), tenantID)

	registry := config.NewRegistry()
	const key = "test.audit_rejected"
	registerTestSetting(registry, key, config.FieldText, "default")
	persistence := testpkg.ConfigRuntime(db)
	values := configRepository.NewSettingValueRepository(persistence)
	audits := configRepository.NewSettingAuditRepository(persistence)
	service := NewSettingsService(values, audits, nil, testpkg.SettingsRuntime(t, db), slog.Default(), registry)

	stored := func() json.RawMessage {
		t.Helper()
		var rows []json.RawMessage
		err := db.NewSelect().TableExpr("config.setting_values").Column("value").
			Where("tenant_id = ? AND setting_key = ?", tenantID, key).Scan(ctx, &rows)
		require.NoError(t, err)
		if len(rows) == 0 {
			return nil
		}
		return rows[0]
	}
	const rejected = "reject_setting_audit_test"
	_, err := db.ExecContext(ctx, `ALTER TABLE config.setting_audit ADD CONSTRAINT `+rejected+` CHECK (setting_key <> ?) NOT VALID`, key)
	require.NoError(t, err)

	t.Run("set", func(t *testing.T) {
		err := service.SetValue(ctx, key, "standalone", nil, nil)
		require.ErrorContains(t, err, rejected, "a change whose audit cannot be written fails")
		assert.Nil(t, stored(), "the value rolls back with its audit entry")

		err = testpkg.WithinTenantContext(t, ctx, db, tenantID, func(txCtx context.Context) error {
			return service.SetValue(txCtx, key, "in request", nil, nil)
		})
		require.ErrorContains(t, err, rejected, "the request transaction sees the failed audit")
		assert.Nil(t, stored(), "the request transaction stores nothing")
	})

	t.Run("reset", func(t *testing.T) {
		// Seeded past the service: its own write would need an audit entry.
		_, err := db.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, ?, '"kept"')`, tenantID, key)
		require.NoError(t, err)

		err = service.ResetValue(ctx, key, nil, nil)
		require.ErrorContains(t, err, rejected, "a reset whose audit cannot be written fails")
		assert.JSONEq(t, `"kept"`, string(stored()), "the override survives the failed reset")

		err = testpkg.WithinTenantContext(t, ctx, db, tenantID, func(txCtx context.Context) error {
			return service.ResetValue(txCtx, key, nil, nil)
		})
		require.ErrorContains(t, err, rejected, "the request transaction sees the failed audit")
		assert.JSONEq(t, `"kept"`, string(stored()), "the request transaction keeps the override")
	})
}
