package enrollment

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking of days and the bound of care days per child (#4701, decision
// E-f A of 09.11.2026). The Träger-Büro offers a week as a whole or by
// days. A booking of a week by days names its days; a booking of a whole
// week takes every care day of the week. The care days of a week are its
// days from Monday to Friday. A period may bound the care days one child
// books in it, over all its weeks, time models and host schools; days spread
// over several weeks are allowed. The child is the child of its
// registration: a second registration is a child of its own.
//
// The bound counts every booking of the child, placed or waiting: a waiting
// booking that moves up never takes the child over the bound. A lower bound
// later takes no booking away; a rebooking may then keep or lower the days,
// not raise them. Each change of a child's bookings after its submission
// runs under the lock of the period, so two changes of one child never both
// read the old number of days.
//
// The capacity of a week by days holds for each of its days: a booking gets
// its place only when every one of its days has a free place, otherwise all
// of it waits. The waiting list stays in the order the registrations came
// in; a booking whose days are not all free lets a later booking of other
// days pass.

// Booking units of a holiday week.
const (
	HolidayBookingUnitWeek = "week"
	HolidayBookingUnitDay  = "day"
)

// HolidayMaxDaysPerChildMax bounds the bound of care days per child.
const HolidayMaxDaysPerChildMax = 366

var (
	// ErrHolidayChildDayLimit reports bookings of a child with more care
	// days than its period allows.
	ErrHolidayChildDayLimit = errors.New("the child books more care days than the period allows")
	// ErrHolidayBookingUnitBooked reports a change of the booking unit of a
	// week that bookings already name.
	ErrHolidayBookingUnitBooked = errors.New("the booking unit of a booked week cannot change")
)

// HolidayChildDayLimitRefusal names the child, by its position in the
// written registration (0 for the first), the bound of its period and the
// days it would book. It unwraps to ErrHolidayChildDayLimit.
type HolidayChildDayLimitRefusal struct {
	Child int
	Limit int
	Days  int
}

func (r *HolidayChildDayLimitRefusal) Error() string {
	return fmt.Sprintf("child %d: %d care days, the period allows %d: %v", r.Child+1, r.Days, r.Limit, ErrHolidayChildDayLimit)
}

func (r *HolidayChildDayLimitRefusal) Unwrap() error { return ErrHolidayChildDayLimit }

// HolidayDayPlaces is the number of placed bookings on one day of a week by
// days.
type HolidayDayPlaces struct {
	Date   calendar.Date `json:"date"`
	Placed int           `json:"placed"`
}

// HolidayWeekCareDays are the care days of a week: its days from Monday to
// Friday, ascending.
func HolidayWeekCareDays(startsOn, endsOn calendar.Date) []calendar.Date {
	var days []calendar.Date
	if startsOn.IsZero() || endsOn.IsZero() {
		return days
	}
	for day := startsOn; !day.After(endsOn); day = day.AddDays(1) {
		if weekday := day.Weekday(); weekday != time.Saturday && weekday != time.Sunday {
			days = append(days, day)
		}
		if len(days) > 7 {
			break
		}
	}
	return days
}

// HolidayBookedDays are the care days a booking takes: its days, or every
// care day of its week for a booking of the whole week.
func HolidayBookedDays(week HolidayWeek, days []calendar.Date) []calendar.Date {
	if len(days) > 0 {
		return days
	}
	return HolidayWeekCareDays(week.StartsOn, week.EndsOn)
}

// NormalizeHolidayDays sorts the days and drops repetitions.
func NormalizeHolidayDays(days []calendar.Date) []calendar.Date {
	if len(days) == 0 {
		return nil
	}
	sorted := slices.Clone(days)
	slices.SortFunc(sorted, func(a, b calendar.Date) int { return a.Compare(b) })
	return slices.Compact(sorted)
}

// HolidayWeekQueue is a locked week with the bookings that take or wait for
// its places, in the order of the waiting list (#4699, #4701). It serves the
// placement only.
type HolidayWeekQueue struct {
	Capacity *int
	StartsOn calendar.Date
	EndsOn   calendar.Date
	Bookings []HolidayQueuedBooking
}

// HolidayQueuedBooking is one booking of a HolidayWeekQueue. Days are the
// days of a booking of days, empty for the whole week.
type HolidayQueuedBooking struct {
	ID             int64
	RegistrationID int64
	ChildID        int64
	ChildPosition  int
	Placed         bool
	Days           []calendar.Date
}

// holidayWeekdayNames are the German short names of the weekdays, Sunday
// first like time.Weekday.
var holidayWeekdayNames = [7]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}

// HolidayDayNames names booked days in German for mails and exports, such as
// "Mo 06.07., Mi 08.07." (#4701).
func HolidayDayNames(days []calendar.Date) string {
	names := make([]string, len(days))
	for i, day := range days {
		names[i] = holidayWeekdayNames[day.Weekday()] + " " + day.Format("02.01.")
	}
	return strings.Join(names, ", ")
}
