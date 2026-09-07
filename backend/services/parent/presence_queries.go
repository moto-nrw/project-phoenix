package parent

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// AttendanceReader excludes visit and room data from the parents portal.
type AttendanceReader interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	HasAttendance(context.Context, studentpresence.AttendanceFilter) (bool, error)
}
