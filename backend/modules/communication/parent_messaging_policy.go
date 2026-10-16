package communication

import (
	"context"
	"log/slog"
)

// ParentNotesSettingKey switches the school's parent/OGS messaging on.
const ParentNotesSettingKey = "operations.parent_notes_enabled"

// ParentMessagingSettings resolves the flag outside a tenant transaction.
type ParentMessagingSettings interface {
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

// ParentMessagingEnabledForTenant applies the shared fail-open feature gate.
// This is not authorization: relationship-level permissions remain mandatory.
func ParentMessagingEnabledForTenant(ctx context.Context, settings ParentMessagingSettings, tenantID int64, logger *slog.Logger) bool {
	if logger == nil {
		logger = slog.Default()
	}
	enabled, err := settings.ResolveBoolForTenant(ctx, tenantID, ParentNotesSettingKey)
	if err != nil {
		logger.Warn("parent messaging: resolve enabled for tenant failed, failing open (counting as enabled)",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return enabled
}
