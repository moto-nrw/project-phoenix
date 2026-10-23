package presence

import (
	"context"
)

// Settings is the read-only settings contract used by active HTTP handlers.
type Settings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveString(context.Context, string) (string, error)
	HasTenantOverride(context.Context, string) (bool, error)
}

// DisplaySettings supplies resolved display facts; the root binds Settings Platform.
type DisplaySettings interface {
	WebAttendanceEnabledReader() func(context.Context) (bool, error)
	PrefetchStudentDisplaySettings(context.Context) context.Context
	StudentPhotosEnabled(context.Context) (bool, error)
	TrackingIndicatorLabels(context.Context) (context.Context, []string, error)
}
