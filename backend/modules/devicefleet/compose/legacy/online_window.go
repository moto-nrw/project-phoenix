package legacy

import (
	"context"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// SettingsResolver is the subset of the settings service the online window
// needs. It is declared here so the owner never imports the settings package.
type SettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// NewOnlineWindowResolver reads the per-tenant
// iot.device_online_window_minutes setting. A missing override, a
// non-positive value, or a lookup failure yields zero, and the owner then
// applies its own default.
func NewOnlineWindowResolver(settings SettingsResolver, logger *slog.Logger) func(context.Context) time.Duration {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context) time.Duration {
		minutes := configService.ResolveIntOrDefault(ctx, settings, configModel.KeyDeviceOnlineWindowMinutes, 0, logger)
		if minutes <= 0 {
			return 0
		}
		return time.Duration(minutes) * time.Minute
	}
}
