package config

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// ResolvePresenceMode returns the tenant's resolved presence mode
// ("detailed" | "binary"), defaulting to "detailed" whenever the setting is
// missing, empty, or cannot be resolved. Use this helper everywhere instead of
// calling svc.ResolveString(ctx, KeyPresenceMode) directly — it centralises the
// default and the error-to-default fallback so every consumer behaves the same.
//
// Call inside tenant middleware (ctx carries tenant). For device auth, scheduler
// per-tenant loops, or any caller without a tenant transaction in context, use
// ResolvePresenceModeForTenant.
func ResolvePresenceMode(ctx context.Context, svc SettingsService, logger *slog.Logger) string {
	if svc == nil {
		return configModel.PresenceModeDetailed
	}
	val, err := svc.ResolveString(ctx, configModel.KeyPresenceMode)
	if err != nil {
		if logger != nil {
			logger.Warn("presence_mode resolve failed, using default",
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

// ResolvePresenceModeForTenant is the out-of-tenant-middleware variant of
// ResolvePresenceMode. It wraps its own tenant transaction via
// ResolveStringForTenant, suitable for device auth (IoT handshake) and
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

// IsBinaryMode reports whether a resolved mode string equals the binary mode.
// Accepts the output of ResolvePresenceMode / ResolvePresenceModeForTenant.
func IsBinaryMode(mode string) bool {
	return mode == configModel.PresenceModeBinary
}
