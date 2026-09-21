package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// parentPickupRequestSetting is models/config.KeyParentCarePickupRequestEnabled.
// The fixtures may not import the settings owner's domain (the architecture
// policy keeps test support off it), so the key is spelled out here; a rename
// there fails the test that drives it, next to this line.
const parentPickupRequestSetting = "operations.parent_care_pickup_request_enabled"

// EnableParentPickupTimeRequests switches on the school setting a parent's
// pickup-time request needs, so a test can drive that request (#3468).
func EnableParentPickupTimeRequests(tb testing.TB, db *bun.DB, tenantID int64) {
	tb.Helper()
	_, err := db.NewRaw(`INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, 'true'::jsonb) ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
		tenantID, parentPickupRequestSetting).Exec(context.Background())
	require.NoError(tb, err)
}
