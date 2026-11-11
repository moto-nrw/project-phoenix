package schoolcalendar

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrHolidayClosureCalendarChanged rejects preparation against another revision.
var ErrHolidayClosureCalendarChanged = errors.New("holiday closure calendar changed")

// HolidayClosureNoCareDay names a statutory holiday or closing day and its sources.
type HolidayClosureNoCareDay struct {
	Date        calendar.Date
	Kind        string
	Name        string
	ClosingDays []HolidayClosureSource
}

// HolidayClosureSource identifies a school or Träger closing day.
type HolidayClosureSource struct {
	ID      int64
	Traeger bool
}

const (
	HolidayClosureNoCareHoliday    = "holiday"
	HolidayClosureNoCareClosingDay = "closing_day"
)

// HolidayClosureCareCalendar validates revisions and resolves the no-care days
// of current schools in the caller's Träger transaction (ADR 0196).
type HolidayClosureCareCalendar interface {
	HolidayClosureNoCareDays(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date, revisions map[int64]int64) (map[int64][]HolidayClosureNoCareDay, error)
}
