package config

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// ResolvePresenceModeForTenant returns the tenant's resolved presence mode
// ("detailed" | "binary"), defaulting to "detailed" whenever the setting is
// missing, empty, or cannot be resolved. It wraps its own tenant transaction
// via ResolveStringForTenant, suitable for device auth (IoT handshake) and
// scheduler per-tenant loops.
func ResolvePresenceModeForTenant(ctx context.Context, svc SettingsService, tenantID int64, logger *slog.Logger) string {
	if svc == nil {
		return configModel.PresenceModeDetailed
	}
	val, err := svc.ResolveStringForTenant(ctx, tenantID, configModel.KeyPresenceMode)
	if err != nil {
		if logger != nil {
			logger.Warn("presence_mode resolve failed, using default",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
		return configModel.PresenceModeDetailed
	}
	if val == "" {
		return configModel.PresenceModeDetailed
	}
	return val
}
