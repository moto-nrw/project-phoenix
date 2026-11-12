package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// HolidayClosureCalendar owns revision checks and no-care date resolution.
type HolidayClosureCalendar struct {
	reads         ports.HolidayClosureCalendarReads
	holidays      ports.HolidayClosureHolidays
	defaultRegion string
}

// NewHolidayClosureCalendar binds calendar reads, local holidays and the registry default.
func NewHolidayClosureCalendar(reads ports.HolidayClosureCalendarReads, holidays ports.HolidayClosureHolidays, defaultRegion string) *HolidayClosureCalendar {
	if reads == nil || holidays == nil {
		panic("holiday closure calendar: calendar reads and holidays are required")
	}
	return &HolidayClosureCalendar{reads: reads, holidays: holidays, defaultRegion: defaultRegion}
}

// HolidayClosureNoCareDays validates the calendar and computes no-care dates in one school batch.
func (c HolidayClosureCalendar) HolidayClosureNoCareDays(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date, revisions map[int64]int64) (map[int64][]schoolcalendar.HolidayClosureNoCareDay, error) {
	schools, err := c.reads.HolidayClosureCalendars(ctx, organizationID, schoolIDs, from, to)
	if err != nil {
		return nil, err
	}
	for schoolID := range revisions {
		if _, ok := schools[schoolID]; !ok {
			return nil, schoolcalendar.ErrHolidayClosureNoticeNotFound
		}
	}
	result := make(map[int64][]schoolcalendar.HolidayClosureNoCareDay, len(schools))
	holidays := map[string][]schoolcalendar.HolidayClosureNoCareDay{}
	for schoolID, school := range schools {
		if revision, ok := revisions[schoolID]; ok && school.Revision != revision {
			return nil, schoolcalendar.ErrHolidayClosureCalendarChanged
		}
		region, err := c.region(school.FederalState)
		if err != nil {
			return nil, err
		}
		if _, ok := holidays[region]; !ok {
			if holidays[region], err = c.holidayDays(ctx, region, from, to); err != nil {
				return nil, err
			}
		}
		result[schoolID] = mergeClosureNoCareDays(holidays[region], closingNoCareDays(school.ClosingDays, from, to))
	}
	return result, nil
}

// holidayDays are the statutory holidays of a federal state in [from, to].
func (c HolidayClosureCalendar) holidayDays(ctx context.Context, region string, from, to calendar.Date) ([]schoolcalendar.HolidayClosureNoCareDay, error) {
	if !c.holidays.ValidHolidayRegion(region) {
		return nil, fmt.Errorf("%w: unsupported region %q", schoolcalendar.ErrFederalStateUnavailable, region)
	}
	listed, err := c.holidays.ListHolidays(ctx, region, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	days := make([]schoolcalendar.HolidayClosureNoCareDay, 0, len(listed))
	for _, holiday := range listed {
		day, err := calendar.ParseDate(holiday.Date)
		if err != nil {
			return nil, fmt.Errorf("holiday closure calendar: %w", err)
		}
		days = append(days, schoolcalendar.HolidayClosureNoCareDay{Date: day, Kind: schoolcalendar.HolidayClosureNoCareHoliday, Name: holiday.Name})
	}
	return days, nil
}

// closingNoCareDays are the days in [from, to] of a school's closing days,
// each naming its source.
func closingNoCareDays(closingDays []schoolcalendar.HolidayClosureCalendarDay, from, to calendar.Date) []schoolcalendar.HolidayClosureNoCareDay {
	var closing []schoolcalendar.HolidayClosureNoCareDay
	for _, closingDay := range closingDays {
		for day := max(closingDay.StartsOn, from); !day.After(min(closingDay.EndsOn, to)); day = day.AddDays(1) {
			closing = append(closing, schoolcalendar.HolidayClosureNoCareDay{Date: day, Kind: schoolcalendar.HolidayClosureNoCareClosingDay, Name: closingDay.Reason,
				ClosingDays: []schoolcalendar.HolidayClosureSource{{ID: closingDay.SourceID, Traeger: closingDay.Traeger}}})
		}
	}
	return closing
}

// region uses the stored state or the bound registry default, never an env value.
func (c HolidayClosureCalendar) region(stored string) (string, error) {
	if stored != "" {
		return stored, nil
	}
	if c.defaultRegion == "" {
		return "", fmt.Errorf("%w: no registry default", schoolcalendar.ErrFederalStateUnavailable)
	}
	return c.defaultRegion, nil
}

func mergeClosureNoCareDays(holidays, closing []schoolcalendar.HolidayClosureNoCareDay) []schoolcalendar.HolidayClosureNoCareDay {
	merged := append(slices.Clone(holidays), closing...)
	slices.SortStableFunc(merged, func(a, b schoolcalendar.HolidayClosureNoCareDay) int { return a.Date.Compare(b.Date) })
	result := make([]schoolcalendar.HolidayClosureNoCareDay, 0, len(merged))
	for _, day := range merged {
		if n := len(result); n > 0 && result[n-1].Date == day.Date {
			result[n-1].ClosingDays = append(result[n-1].ClosingDays, day.ClosingDays...)
			continue
		}
		day.ClosingDays = slices.Clone(day.ClosingDays)
		result = append(result, day)
	}
	return result
}
