package common

import (
	"context"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings"
)

// PrefetchStudentDisplaySettings shares the overview and photo visibility reads.
func PrefetchStudentDisplaySettings(ctx context.Context, settings any) context.Context {
	return PrefetchSettings(ctx, settings, configModel.KeyOperationalOverviewScope, configModel.KeyStudentPhotosEnabled)
}

// StudentPhotosEnabled preserves the HTTP photo-visibility default and logging.
func StudentPhotosEnabled(ctx context.Context, settings interface {
	ResolveBool(context.Context, string) (bool, error)
	HasTenantOverride(context.Context, string) (bool, error)
}, logger *slog.Logger) bool {
	return configModel.ResolveBoolOrDefault(ctx, settings, configModel.KeyStudentPhotosEnabled, false, logger)
}
