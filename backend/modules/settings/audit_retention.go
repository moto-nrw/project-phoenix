package settings

import "context"

// SettingAuditRetentionYears is the fixed retention of a setting change
// (#4044, ADR 0048): the setting protocol proves who entered a retention
// window or another value, so it stays as long as the deletion log. The
// database functions config.delete_expired_setting_audit and
// config.delete_expired_traeger_setting_audit delete with the same window
// on their own clock (#6844); change both together.
const SettingAuditRetentionYears = 3

// AuditRetention deletes the setting changes of the school in context after
// SettingAuditRetentionYears.
type AuditRetention interface {
	// DeleteExpiredSettingChanges reports the changes it deleted and the
	// window in days.
	DeleteExpiredSettingChanges(ctx context.Context) (deleted, retentionDays int, err error)
}
