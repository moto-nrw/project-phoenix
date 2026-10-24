package care

import (
	"context"
	"time"
)

// Attendance contains only the school-stay times visible to a guardian.
// Device, staff, room and movement facts never cross this port.
type Attendance struct {
	CheckInTime  time.Time
	CheckOutTime *time.Time
}

// AttendanceFilter selects a child's day or the school's attendance window.
// Nil StudentIDs means the school-wide existence check, not child access.
type AttendanceFilter struct {
	StudentIDs []int64
	FromDate   string
	UntilDate  string
}

// AttendanceReader excludes visit and room data from the parents portal.
type AttendanceReader interface {
	ListAttendance(context.Context, AttendanceFilter) ([]Attendance, error)
	HasAttendance(context.Context, AttendanceFilter) (bool, error)
}
