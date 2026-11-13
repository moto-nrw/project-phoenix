package statistics

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Care days of a report window: weekdays without a public holiday, a
// closing day or a holiday period of the school calendar.

// careDays returns the ordered set of care days in the window and the
// exclusion breakdown.
func (s *service) careDays(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, studentpresence.StatisticsExcludedDays, error) {
	care, _, excluded, err := s.careCalendar(ctx, from, to)
	return care, excluded, err
}

// careCalendar is careDays plus the care weekdays that only a holiday period
// excludes: holiday care is open on them (#5994), unlike on a public holiday
// or a closing day.
func (s *service) careCalendar(ctx context.Context, from, to calendar.Date) (care, vacationOnly map[calendar.Date]bool, excluded studentpresence.StatisticsExcludedDays, err error) {
	holidays, err := s.holidayDates(ctx, from, to)
	if err != nil {
		return nil, nil, excluded, err
	}
	closing, vacation, err := s.calendarDates(ctx, from, to)
	if err != nil {
		return nil, nil, excluded, err
	}
	care, vacationOnly = map[calendar.Date]bool{}, map[calendar.Date]bool{}
	for d := from; !d.After(to); d = d.AddDays(1) {
		// A weekend counts only where it follows Friday's plan (#3921).
		careWeekday, err := calendar.IsCareWeekday(ctx, d)
		if err != nil {
			return nil, nil, excluded, fmt.Errorf("statistics: resolve weekend plan: %w", err)
		}
		if !careWeekday {
			continue
		}
		if countExclusions(&excluded, holidays[d], closing[d], vacation[d]) {
			vacationOnly[d] = vacation[d] && !holidays[d] && !closing[d]
			continue
		}
		care[d] = true
	}
	return care, vacationOnly, excluded, nil
}

// countExclusions adds a weekday to every exclusion bucket it falls into and
// reports whether it is excluded at all; the total counts the union.
func countExclusions(excluded *studentpresence.StatisticsExcludedDays, holiday, closing, vacation bool) bool {
	if holiday {
		excluded.PublicHolidays++
	}
	if closing {
		excluded.ClosingDays++
	}
	if vacation {
		excluded.HolidayPeriods++
	}
	off := holiday || closing || vacation
	if off {
		excluded.Total++
	}
	return off
}

func (s *service) holidayDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	if s.cfg.Holidays == nil {
		return map[calendar.Date]bool{}, nil
	}
	set, err := s.cfg.Holidays.HolidayDates(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load public holidays: %w", err)
	}
	return set, nil
}

// calendarDates reads the closing days and the holiday periods: in one
// statement where the combined read is bound, else one read each.
func (s *service) calendarDates(ctx context.Context, from, to calendar.Date) (closing, vacation map[calendar.Date]bool, err error) {
	if s.cfg.CalendarDays != nil {
		closing, vacation, err = s.cfg.CalendarDays.CareCalendarDays(ctx, from, to)
		if err != nil {
			return nil, nil, fmt.Errorf("load closing days and holiday periods: %w", err)
		}
		return closing, vacation, nil
	}
	if closing, err = s.closingDayDates(ctx, from, to); err != nil {
		return nil, nil, err
	}
	if vacation, err = s.holidayPeriodDates(ctx, from, to); err != nil {
		return nil, nil, err
	}
	return closing, vacation, nil
}

func (s *service) closingDayDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	if s.cfg.ClosingDays == nil {
		return map[calendar.Date]bool{}, nil
	}
	set, err := s.cfg.ClosingDays.ClosingDayDates(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load closing days: %w", err)
	}
	return set, nil
}

func (s *service) holidayPeriodDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	vacation := map[calendar.Date]bool{}
	if s.cfg.Periods == nil {
		return vacation, nil
	}
	periods, err := s.cfg.Periods.StatisticsHolidayPeriods(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load holiday periods: %w", err)
	}
	for _, p := range periods {
		// Clamp to the report window: a period may span years, and only
		// its overlap with [from, to] can ever be a care day.
		start, end := p.StartDate, p.EndDate
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		for d := start; !d.After(end); d = d.AddDays(1) {
			vacation[d] = true
		}
	}
	return vacation, nil
}
