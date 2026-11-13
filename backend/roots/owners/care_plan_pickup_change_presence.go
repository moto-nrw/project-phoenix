package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
)

type PickupChangePresence interface {
	LockStudentAttendance(context.Context, int64) error
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}
