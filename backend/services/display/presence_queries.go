package display

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type PresenceReader interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}
