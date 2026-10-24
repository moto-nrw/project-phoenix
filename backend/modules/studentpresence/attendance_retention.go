package studentpresence

import (
	"context"
	"errors"
)

// AttendanceRetention deletes a school's daily attendance once its retention
// window has passed (#3957): the nightly cleanup deletes every attendance row
// dated before the window, counted from the care day.
type AttendanceRetention interface {
	DeleteExpiredAttendance(ctx context.Context, retentionDays int) (int, error)
}

// ErrAttendanceRetentionWindow refuses a retention window that is not
// positive: nothing is deleted on a wrong period, and the run is caught up
// once the setting is fixed.
var ErrAttendanceRetentionWindow = errors.New("attendance retention window must be positive")
