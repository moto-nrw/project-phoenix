package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareDaySource supplies plan facts and the dated participation boundary.
// Neither read derives a care-day verdict or consults live attendance.
type CareDaySource interface {
	LoadCareDayFacts(context.Context, []int64, calendar.Date, calendar.Date) (map[int64]map[calendar.Date]domain.CareDayFacts, error)
	ParticipatingStudentIDsByDate(context.Context, []int64, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error)
}
