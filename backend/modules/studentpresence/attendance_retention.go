package studentpresence

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AttendanceRetention deletes a school's daily attendance once its retention
// window has passed (#3957): the nightly cleanup deletes every attendance row
// dated before the window, counted from the care day.
type AttendanceRetention interface {
	DeleteExpiredAttendance(ctx context.Context, retentionDays int) (int, error)
}

// attendancePurge is the engine's deletion behind AttendanceRetention.
type attendancePurge interface {
	DeleteAttendanceBefore(ctx context.Context, cutoff calendar.Date) (int, error)
}

// ErrAttendanceRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrAttendanceRetentionWindow = errors.New("attendance retention window must be positive")

// DeleteExpiredAttendance deletes the attendance dated before the first
// Berlin calendar day a window of retentionDays keeps.
func (m *Module) DeleteExpiredAttendance(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", ErrAttendanceRetentionWindow, retentionDays)
	}
	return m.DeleteAttendanceBefore(ctx, calendar.TodayDate().AddDays(-retentionDays))
}

// DeleteAttendanceBefore deletes the attendance dated before cutoff.
func (m *Module) DeleteAttendanceBefore(ctx context.Context, cutoff calendar.Date) (int, error) {
	return m.engine.DeleteAttendanceBefore(ctx, cutoff)
}
