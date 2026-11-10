package enrollment

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Days without holiday care (#4702). A day that is a statutory holiday of the
// host school's federal state or a closing day of the host school, the
// school's own or one its Träger planned for it, is never bookable: a week
// containing such a day is not written by the Träger-Büro and not offered to
// parents or the Büro for a new booking. A booking made before the day became
// a closing day stays; only a new booking of the week is refused.

// Kinds of a day without care.
const (
	HolidayNoCareHoliday    = "holiday"
	HolidayNoCareClosingDay = "closing_day"
)

// ErrHolidaySchoolCalendarUnavailable reports a holiday registration
// composed without the host schools' calendar; it never offers or books a
// week then.
var ErrHolidaySchoolCalendarUnavailable = errors.New("the calendar of the host schools is not available")

// ErrHolidayWeekNoCareDay reports a week that contains a day without care at
// its host school.
var ErrHolidayWeekNoCareDay = errors.New("the week contains a day without care at its host school")

// HolidayNoCareDay is a day on which a host school gives no care.
type HolidayNoCareDay struct {
	Date calendar.Date `json:"date"`
	Kind string        `json:"kind"`
	Name string        `json:"name"`
}

// HolidaySchoolCalendar is Enrollment's port onto the days its host schools
// give no care: the statutory holidays of each school's federal state and
// its closing days, ordered by date, for every school in [from, to]. The
// root binds it to School Calendar, outside the caller's Träger transaction.
type HolidaySchoolCalendar interface {
	HolidayNoCareDays(ctx context.Context, schoolIDs []int64, from, to calendar.Date) (map[int64][]HolidayNoCareDay, error)
}

// HolidayWeekNoCareDayRefusal names the week, by its position in the written
// period or booking, and its first day without care. It unwraps to
// ErrHolidayWeekNoCareDay.
type HolidayWeekNoCareDayRefusal struct {
	Week   int
	WeekID int64
	Day    HolidayNoCareDay
}

func (r *HolidayWeekNoCareDayRefusal) Error() string {
	return fmt.Sprintf("week %d: %s is a day without care (%s: %s)", r.Week+1, r.Day.Date, r.Day.Kind, r.Day.Name)
}

func (r *HolidayWeekNoCareDayRefusal) Unwrap() error { return ErrHolidayWeekNoCareDay }
