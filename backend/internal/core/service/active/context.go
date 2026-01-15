package active

import "context"

type contextKey string

const attendanceAutoSyncKey contextKey = "active:autoSyncAttendance"

// WithAttendanceAutoSync marks the context so EndVisit will also sync daily attendance.
func WithAttendanceAutoSync(ctx context.Context) context.Context {
	return context.WithValue(ctx, attendanceAutoSyncKey, true)
}

func shouldAutoSyncAttendance(ctx context.Context) bool {
	enabled, ok := ctx.Value(attendanceAutoSyncKey).(bool)
	return ok && enabled
}
