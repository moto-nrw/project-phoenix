package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Days without holiday care (#4702): a statutory holiday of the host school's
// federal state or a closing day of the host school is never bookable. The
// reads mark such days on each week when the calendar is bound; every write
// that adds or moves a week, or books one anew, requires the calendar. A
// whole week with such a day is refused; a week by days leaves out only the
// day (#6447).

// holidayNoCareDays reads the days without care of the weeks' host schools in
// the span of the weeks.
func holidayNoCareDays(ctx context.Context, source enrollment.HolidaySchoolCalendar, weeks []holidayWeekSpan) (map[int64][]enrollment.HolidayNoCareDay, error) {
	if len(weeks) == 0 {
		return map[int64][]enrollment.HolidayNoCareDay{}, nil
	}
	if source == nil {
		return nil, enrollment.ErrHolidaySchoolCalendarUnavailable
	}
	schools := make([]int64, 0, len(weeks))
	from, to := weeks[0].StartsOn, weeks[0].EndsOn
	for _, week := range weeks {
		if !slices.Contains(schools, week.HostSchoolID) {
			schools = append(schools, week.HostSchoolID)
		}
		if week.StartsOn.Before(from) {
			from = week.StartsOn
		}
		if week.EndsOn.After(to) {
			to = week.EndsOn
		}
	}
	slices.Sort(schools)
	days, err := source.HolidayNoCareDays(ctx, schools, from, to)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read the host schools' days without care: %w", err)
	}
	return days, nil
}

// holidayWeekSpan is the part of a week the check needs.
type holidayWeekSpan struct {
	StartsOn     calendar.Date
	EndsOn       calendar.Date
	HostSchoolID int64
}

// noCareDaysOfWeek are the days of days that fall into the week.
func noCareDaysOfWeek(week holidayWeekSpan, days map[int64][]enrollment.HolidayNoCareDay) []enrollment.HolidayNoCareDay {
	result := []enrollment.HolidayNoCareDay{}
	for _, day := range days[week.HostSchoolID] {
		if !day.Date.Before(week.StartsOn) && !day.Date.After(week.EndsOn) {
			result = append(result, day)
		}
	}
	return result
}

// withHolidayNoCareDays marks the days without care on every week of period.
// Without a bound calendar the period stays as read.
func withHolidayNoCareDays(ctx context.Context, source enrollment.HolidaySchoolCalendar, period enrollment.HolidayPeriod) (enrollment.HolidayPeriod, error) {
	if source == nil || len(period.Weeks) == 0 {
		return period, nil
	}
	return markHolidayNoCareDays(ctx, source, period)
}

// markHolidayNoCareDays marks the days without care on every week of period
// and requires the calendar.
func markHolidayNoCareDays(ctx context.Context, source enrollment.HolidaySchoolCalendar, period enrollment.HolidayPeriod) (enrollment.HolidayPeriod, error) {
	spans := make([]holidayWeekSpan, 0, len(period.Weeks))
	for _, week := range period.Weeks {
		spans = append(spans, holidayWeekSpan{StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolID: week.HostSchoolID})
	}
	days, err := holidayNoCareDays(ctx, source, spans)
	if err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	weeks := slices.Clone(period.Weeks)
	for i := range weeks {
		weeks[i].NoCareDays = noCareDaysOfWeek(spans[i], days)
	}
	period.Weeks = weeks
	return period, nil
}

// refuseHolidayWeeksWithoutCare refuses a week of input that the Träger-Büro
// adds, moves, gives another host school or another booking unit when it
// contains a day without care. A week booked by days takes such a day: only
// that day is not offered and not bookable (#6447); it is refused only when
// no care day is left. A stored week left as it is stays: it may hold
// bookings made before the day became a closing day, and its booking is
// refused on its own.
func refuseHolidayWeeksWithoutCare(ctx context.Context, source enrollment.HolidaySchoolCalendar, weeks []enrollment.HolidayWeekInput, current enrollment.HolidayPeriod) error {
	stored := map[int64]enrollment.HolidayWeek{}
	for _, week := range current.Weeks {
		stored[week.ID] = week
	}
	var spans []holidayWeekSpan
	var positions []int
	for i, week := range weeks {
		if before, ok := stored[week.ID]; ok && week.ID != 0 && before.StartsOn == week.StartsOn &&
			before.EndsOn == week.EndsOn && before.HostSchoolID == week.HostSchoolID &&
			holidayBookingUnitOf(before.BookingUnit) == holidayBookingUnitOf(week.BookingUnit) {
			continue
		}
		spans = append(spans, holidayWeekSpan{StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolID: week.HostSchoolID})
		positions = append(positions, i)
	}
	days, err := holidayNoCareDays(ctx, source, spans)
	if err != nil {
		return err
	}
	for j, span := range spans {
		found := noCareDaysOfWeek(span, days)
		if len(found) == 0 {
			continue
		}
		week := weeks[positions[j]]
		if holidayBookingUnitOf(week.BookingUnit) == enrollment.HolidayBookingUnitDay &&
			holidayWeekKeepsACareDay(span, found) {
			continue
		}
		return &enrollment.HolidayWeekNoCareDayRefusal{Week: positions[j], WeekID: week.ID, Day: found[0]}
	}
	return nil
}

// holidayBookingUnitOf reads an empty booking unit as a whole week (#4701).
func holidayBookingUnitOf(unit string) string {
	if unit == "" {
		return enrollment.HolidayBookingUnitWeek
	}
	return unit
}

// holidayWeekKeepsACareDay reports whether the week has a care day that is
// none of the days without care.
func holidayWeekKeepsACareDay(week holidayWeekSpan, none []enrollment.HolidayNoCareDay) bool {
	return slices.ContainsFunc(enrollment.HolidayWeekCareDays(week.StartsOn, week.EndsOn), func(day calendar.Date) bool {
		return !slices.ContainsFunc(none, func(n enrollment.HolidayNoCareDay) bool { return n.Date == day })
	})
}

// refuseHolidayBookingsWithoutCare refuses a booking of a week of the marked
// period that contains a day without care, unless kept names it: the child
// had booked that week before.
func refuseHolidayBookingsWithoutCare(bookings []enrollment.HolidayBookingInput, period enrollment.HolidayPeriod, kept []enrollment.HolidayBookingInput) error {
	weeks := map[int64]enrollment.HolidayWeek{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	for i, booking := range bookings {
		week, ok := weeks[booking.WeekID]
		if !ok || len(week.NoCareDays) == 0 {
			continue
		}
		if week.BookingUnit == enrollment.HolidayBookingUnitDay {
			// A booking of days is refused only for a day without care it
			// adds; a day it booked before stays (#4701).
			if day, refused := holidayNewNoCareDay(week, booking, kept); refused {
				return &enrollment.HolidayWeekNoCareDayRefusal{Week: i, WeekID: week.ID, Day: day}
			}
			continue
		}
		if slices.ContainsFunc(kept, func(k enrollment.HolidayBookingInput) bool { return k.WeekID == booking.WeekID }) {
			continue
		}
		return &enrollment.HolidayWeekNoCareDayRefusal{Week: i, WeekID: week.ID, Day: week.NoCareDays[0]}
	}
	return nil
}

// holidayNewNoCareDay names the first day without care of the week that a
// booking of days books and kept, the child's bookings so far, did not.
func holidayNewNoCareDay(week enrollment.HolidayWeek, booking enrollment.HolidayBookingInput, kept []enrollment.HolidayBookingInput) (enrollment.HolidayNoCareDay, bool) {
	var before []calendar.Date
	for _, k := range kept {
		if k.WeekID == booking.WeekID {
			before = k.Days
		}
	}
	for _, day := range week.NoCareDays {
		if slices.Contains(booking.Days, day.Date) && !slices.Contains(before, day.Date) {
			return day, true
		}
	}
	return enrollment.HolidayNoCareDay{}, false
}

// holidayBookableCareDays are the care days of a week by days without the
// days its host school gives no care (#4702).
func holidayBookableCareDays(week enrollment.HolidayWeek) []calendar.Date {
	return slices.DeleteFunc(enrollment.HolidayWeekCareDays(week.StartsOn, week.EndsOn), func(day calendar.Date) bool {
		return slices.ContainsFunc(week.NoCareDays, func(none enrollment.HolidayNoCareDay) bool { return none.Date == day })
	})
}
