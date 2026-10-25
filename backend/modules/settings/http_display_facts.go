package settings

import (
	"context"
)

// DisplaySettingsSource is the read capability used by the display projections.
type DisplaySettingsSource interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveString(context.Context, string) (string, error)
	HasTenantOverride(context.Context, string) (bool, error)
}

// DisplaySettings projects native settings into HTTP display facts.
type DisplaySettings struct{ source DisplaySettingsSource }

func NewDisplaySettings(source DisplaySettingsSource) DisplaySettings {
	return DisplaySettings{source: source}
}

func (s DisplaySettings) PrefetchStudentDisplaySettings(ctx context.Context) context.Context {
	return PrefetchStudentDisplaySettings(ctx, s.source)
}

func (s DisplaySettings) StudentPhotosEnabled(ctx context.Context) (bool, error) {
	return StudentPhotosEnabled(ctx, s.source)
}

func (s DisplaySettings) TrackingIndicatorLabels(ctx context.Context) (context.Context, []string, error) {
	return TrackingIndicatorLabels(ctx, s.source)
}
