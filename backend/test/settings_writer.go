package test

import "context"

// SettingsWriter is the settings service a route suite toggles tenant settings
// with: it writes and resets one key of the request tenant.
type SettingsWriter interface {
	SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error
	ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error
}
