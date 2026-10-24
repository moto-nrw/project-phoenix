package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ManualPartialAbsenceReader interface {
	ManualPartialAbsenceDates(context.Context, int64, calendar.Date, calendar.Date) ([]calendar.Date, error)
}
