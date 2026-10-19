package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// PresenceQuery is the subset of the Student Presence capability the
// dashboard reads.
type PresenceQuery interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}

type PresenceReader struct{ Presence PresenceQuery }

func (r PresenceReader) ListOpenVisits(ctx context.Context) ([]domain.PresentVisit, error) {
	visits, err := r.Presence.ListVisits(ctx, studentpresence.VisitFilter{OpenOnly: true})
	if err != nil {
		return nil, err
	}
	result := make([]domain.PresentVisit, 0, len(visits))
	for _, visit := range visits {
		if visit.ExitTime != nil {
			continue
		}
		result = append(result, domain.PresentVisit{ActiveGroupID: visit.ActiveGroupID, StudentID: visit.StudentID})
	}
	return result, nil
}

func (r PresenceReader) ListPresentStudents(ctx context.Context, today time.Time) ([]int64, error) {
	day := today.Format(time.DateOnly)
	attendance, err := r.Presence.ListAttendance(ctx, studentpresence.AttendanceFilter{FromDate: day, UntilDate: day})
	if err != nil {
		return nil, err
	}
	present := make([]int64, 0, len(attendance))
	for _, entry := range attendance {
		if entry.CheckOutTime == nil {
			present = append(present, entry.StudentID)
		}
	}
	return present, nil
}
