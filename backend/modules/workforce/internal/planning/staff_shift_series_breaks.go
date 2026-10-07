package planning

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// SeriesNonWorkingDays are the days of one materialization window a series
// may not plan on (#3820). Statutory holidays are always free; breaks (Ferien
// and closing days) only for a series that did not opt in with
// IncludeSchoolBreaks. The zero value skips nothing.
type SeriesNonWorkingDays struct {
	Holidays map[timezone.Date]bool
	Breaks   map[timezone.Date]bool
}

// skips reports whether an occurrence on day is left out.
func (d SeriesNonWorkingDays) skips(day timezone.Date, includeSchoolBreaks bool) bool {
	return d.Holidays[day] || (d.Breaks[day] && !includeSchoolBreaks)
}

// SeriesNonWorkingDayReader answers the non-working days of [from, to] for a
// series bounded by the calendar period seriesPeriodID.
type SeriesNonWorkingDayReader interface {
	SeriesNonWorkingDays(ctx context.Context, from, to timezone.Date, seriesPeriodID int64) (SeriesNonWorkingDays, error)
}

// WithStaffShiftSeriesNonWorkingDays binds the reader the materializer skips
// holidays, Ferien and closing days by. Without it (unit tests, narrow
// compositions) a series skips none, as the timetable materializer does.
func WithStaffShiftSeriesNonWorkingDays(reader SeriesNonWorkingDayReader) StaffShiftSeriesOption {
	return func(s *staffShiftSeriesService) { s.nonWorkingDays = reader }
}

func (s *staffShiftSeriesService) loadNonWorkingDays(ctx context.Context, series *StaffShiftSeries, from, to timezone.Date) (SeriesNonWorkingDays, error) {
	if s.nonWorkingDays == nil {
		return SeriesNonWorkingDays{}, nil
	}
	return s.nonWorkingDays.SeriesNonWorkingDays(ctx, from, to, series.CalendarPeriodID)
}

// SeriesCalendarReads is the School Calendar read behind the non-working
// days: the statutory holidays of the tenant's federal state, the school's
// closing days and its calendar periods. The School Calendar capability
// satisfies it.
type SeriesCalendarReads interface {
	TenantHolidayDates(ctx context.Context, from, to string) (map[string]bool, error)
	ClosingDayDates(ctx context.Context, from, to string) (map[string]bool, error)
	ListCalendarPeriods(ctx context.Context, filter schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
}

// SchoolCalendarNonWorkingDays binds the series' non-working days to the
// School Calendar.
func SchoolCalendarNonWorkingDays(calendar SeriesCalendarReads) SeriesNonWorkingDayReader {
	return schoolCalendarNonWorkingDays{calendar: calendar}
}

type schoolCalendarNonWorkingDays struct{ calendar SeriesCalendarReads }

// SeriesNonWorkingDays reads holidays, closing days and the active Ferien
// periods once per materialization. The series' own calendar period is never
// a break for it: a series planned over a Ferien period exists for that
// holiday care.
func (c schoolCalendarNonWorkingDays) SeriesNonWorkingDays(ctx context.Context, from, to timezone.Date, seriesPeriodID int64) (SeriesNonWorkingDays, error) {
	holidays, err := c.calendar.TenantHolidayDates(ctx, from.String(), to.String())
	if err != nil {
		return SeriesNonWorkingDays{}, fmt.Errorf("load holidays for series: %w", err)
	}
	closing, err := c.calendar.ClosingDayDates(ctx, from.String(), to.String())
	if err != nil {
		return SeriesNonWorkingDays{}, fmt.Errorf("load closing days for series: %w", err)
	}
	ferien, err := c.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		PeriodType: schoolcalendar.PeriodTypeHoliday, ActiveOnly: true,
		OverlappingFrom: from.String(), OverlappingTo: to.String(), ExcludeID: seriesPeriodID,
	})
	if err != nil {
		return SeriesNonWorkingDays{}, fmt.Errorf("load holiday periods for series: %w", err)
	}
	days := SeriesNonWorkingDays{Holidays: dateSet(holidays), Breaks: dateSet(closing)}
	for _, period := range ferien {
		start, end := timezone.Date(period.StartDate), timezone.Date(period.EndDate)
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		for day := start; !day.After(end); day = day.AddDays(1) {
			days.Breaks[day] = true
		}
	}
	return days, nil
}

func dateSet(values map[string]bool) map[timezone.Date]bool {
	result := make(map[timezone.Date]bool, len(values))
	for day, set := range values {
		if set {
			result[timezone.Date(day)] = true
		}
	}
	return result
}
