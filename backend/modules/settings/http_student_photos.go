package settings

import (
	"context"
)

// PrefetchStudentDisplaySettings shares the overview and photo visibility reads.
func PrefetchStudentDisplaySettings(ctx context.Context, settings any) context.Context {
	return PrefetchSettings(ctx, settings, KeyOperationalOverviewScope, KeyStudentPhotosEnabled)
}

// StudentPhotosEnabled propagates lookup failures instead of inventing photo policy.
func StudentPhotosEnabled(ctx context.Context, settings interface {
	ResolveBool(context.Context, string) (bool, error)
}) (bool, error) {
	return settings.ResolveBool(ctx, KeyStudentPhotosEnabled)
}
