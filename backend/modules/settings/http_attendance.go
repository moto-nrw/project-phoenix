package settings

import "context"

// WebAttendanceEnabledReader binds the native setting key to a request-tenant read.
// A missing source remains missing so the HTTP adapter fails closed.
func WebAttendanceEnabledReader(source interface {
	ResolveBool(context.Context, string) (bool, error)
}) func(context.Context) (bool, error) {
	if source == nil {
		return nil
	}
	return func(ctx context.Context) (bool, error) {
		return source.ResolveBool(ctx, KeyAttendanceWebEnabled)
	}
}

// WebAttendanceEnabledReader supplies the attendance fact through the existing
// root-bound settings projection, without exposing native keys to consumers.
func (s DisplaySettings) WebAttendanceEnabledReader() func(context.Context) (bool, error) {
	return WebAttendanceEnabledReader(s.source)
}
