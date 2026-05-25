package config

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

func ResolveOGSGroupsEnabled(ctx context.Context, svc SettingsService, logger *slog.Logger) bool {
	if svc == nil {
		return true
	}
	val, err := svc.ResolveBool(ctx, configModel.KeyOGSGroupsEnabled)
	if err != nil {
		if logger != nil {
			logger.Warn("ogs_groups_enabled resolve failed, using default",
				slog.String("error", err.Error()),
			)
		}
		return true
	}
	return val
}

func ResolveOGSGroupsEnabledForTenant(ctx context.Context, svc SettingsService, tenantID int64, logger *slog.Logger) bool {
	if svc == nil {
		return true
	}
	val, err := svc.ResolveStringForTenant(ctx, tenantID, configModel.KeyOGSGroupsEnabled)
	if err != nil {
		if logger != nil {
			logger.Warn("ogs_groups_enabled resolve failed, using default",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
		return true
	}
	return val != "false"
}
