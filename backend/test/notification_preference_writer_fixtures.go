//go:build testhooks

package test

import (
	"context"

	"github.com/uptrace/bun"
)

// ReadNotificationPreferenceWriterSnapshot reads the full settings, audit and
// consent fixture state in the tenant transaction supplied by the test.
func ReadNotificationPreferenceWriterSnapshot(ctx context.Context, db bun.IDB, tenantID int64) (string, error) {
	var state string
	err := db.NewRaw(`SELECT jsonb_build_object(
		'values', (SELECT COALESCE(jsonb_agg(to_jsonb(v) ORDER BY v.id), '[]'::jsonb) FROM config.setting_values v WHERE tenant_id = ?),
		'audit', (SELECT COALESCE(jsonb_agg(to_jsonb(a) ORDER BY a.id), '[]'::jsonb) FROM config.setting_audit a WHERE tenant_id = ?),
		'consent', (SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id), '[]'::jsonb) FROM users.notification_preferences p WHERE tenant_id = ?),
		'default_isolation', current_setting('default_transaction_isolation'))::text`, tenantID, tenantID, tenantID).Scan(ctx, &state)
	return state, err
}
