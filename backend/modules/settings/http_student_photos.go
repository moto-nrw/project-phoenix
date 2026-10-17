package settings

import (
	"context"
	"log/slog"
)

// PrefetchStudentDisplaySettings shares the overview and photo visibility reads.
func PrefetchStudentDisplaySettings(ctx context.Context, settings any) context.Context {
	return PrefetchSettings(ctx, settings, KeyOperationalOverviewScope, KeyStudentPhotosEnabled)
}

// StudentPhotosEnabled preserves the HTTP photo-visibility default and logging.
func StudentPhotosEnabled(ctx context.Context, settings interface {
	ResolveBool(context.Context, string) (bool, error)
	HasTenantOverride(context.Context, string) (bool, error)
}, logger *slog.Logger) bool {
	return ResolveBoolOrDefault(ctx, settings, KeyStudentPhotosEnabled, false, logger)
}
