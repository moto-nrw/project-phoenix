package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleNFCSwitchCarriesEverySchoolsNFCState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	nfcOn := testpkg.Tenant(t)
	nfcOff, _ := testpkg.CreateTestTenant(t, db)
	noValue, _ := testpkg.CreateTestTenant(t, db)
	alreadyMoved, _ := testpkg.CreateTestTenant(t, db)
	testpkg.OwnTenantRows(t, db, nfcOff, noValue, alreadyMoved)

	_, err := db.ExecContext(ctx, `
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'attendance.nfc_enabled', 'true'::jsonb),
		       (?, 'attendance.nfc_enabled', 'false'::jsonb),
		       (?, 'module.nfc_anwesenheit', 'true'::jsonb)`,
		nfcOn, nfcOff, alreadyMoved)
	require.NoError(t, err)

	require.NoError(t, moduleNFCSwitchUp(ctx, db))

	moduleValue := func(tenantID int64) string {
		var value string
		require.NoError(t, db.NewRaw(`
			SELECT value::text FROM config.setting_values
			WHERE tenant_id = ? AND setting_key = 'module.nfc_anwesenheit'`, tenantID).
			Scan(ctx, &value))
		return value
	}
	assert.Equal(t, "true", moduleValue(nfcOn), "a school with NFC keeps NFC")
	assert.Equal(t, "false", moduleValue(nfcOff), "a school that switched NFC off keeps it off")
	assert.Equal(t, "false", moduleValue(noValue),
		"a school without a value had the old default off and keeps it")
	assert.Equal(t, "true", moduleValue(alreadyMoved), "an existing module value stays")

	var oldValues int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM config.setting_values
		WHERE setting_key = 'attendance.nfc_enabled' AND tenant_id IN (?, ?, ?, ?)`,
		nfcOn, nfcOff, noValue, alreadyMoved).Scan(ctx, &oldValues))
	assert.Zero(t, oldValues, "the replaced setting has no values left")

	var audited []struct {
		TenantID int64  `bun:"tenant_id"`
		Key      string `bun:"setting_key"`
		Action   string `bun:"action"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT tenant_id, setting_key, action FROM config.setting_audit
		WHERE tenant_id IN (?, ?, ?, ?) AND changed_by IS NULL
		  AND setting_key IN ('attendance.nfc_enabled', 'module.nfc_anwesenheit')
		ORDER BY tenant_id, setting_key`,
		nfcOn, nfcOff, noValue, alreadyMoved).Scan(ctx, &audited))
	type entry struct {
		tenantID int64
		key      string
		action   string
	}
	got := make([]entry, len(audited))
	for i, row := range audited {
		got[i] = entry{row.TenantID, row.Key, row.Action}
	}
	assert.ElementsMatch(t, []entry{
		{nfcOn, "attendance.nfc_enabled", "delete"},
		{nfcOn, "module.nfc_anwesenheit", "set"},
		{nfcOff, "attendance.nfc_enabled", "delete"},
		{nfcOff, "module.nfc_anwesenheit", "set"},
		{noValue, "module.nfc_anwesenheit", "set"},
	}, got, "each carried and each deleted value is in the settings audit")
}
