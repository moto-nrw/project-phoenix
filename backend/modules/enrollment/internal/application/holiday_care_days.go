package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Days without holiday care (#4702): a statutory holiday of the host school's
// federal state or a closing day of the host school is never bookable. The
// reads mark such days on each week when the calendar is bound; every write
// that adds or moves a week, or books one anew, requires the calendar and
// refuses a week with such a day.

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
// adds, moves or gives another host school when it contains a day without
// care. A stored week left as it is stays: it may hold bookings made before
// the day became a closing day, and its booking is refused on its own.
func refuseHolidayWeeksWithoutCare(ctx context.Context, source enrollment.HolidaySchoolCalendar, weeks []enrollment.HolidayWeekInput, current enrollment.HolidayPeriod) error {
	stored := map[int64]enrollment.HolidayWeek{}
	for _, week := range current.Weeks {
		stored[week.ID] = week
	}
	var spans []holidayWeekSpan
	var positions []int
	for i, week := range weeks {
		if before, ok := stored[week.ID]; ok && week.ID != 0 && before.StartsOn == week.StartsOn &&
			before.EndsOn == week.EndsOn && before.HostSchoolID == week.HostSchoolID {
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
		if found := noCareDaysOfWeek(span, days); len(found) > 0 {
			return &enrollment.HolidayWeekNoCareDayRefusal{Week: positions[j], WeekID: weeks[positions[j]].ID, Day: found[0]}
		}
	}
	return nil
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
		if slices.ContainsFunc(kept, func(k enrollment.HolidayBookingInput) bool { return k.WeekID == booking.WeekID }) {
			continue
		}
		return &enrollment.HolidayWeekNoCareDayRefusal{Week: i, WeekID: week.ID, Day: week.NoCareDays[0]}
	}
	return nil
}
