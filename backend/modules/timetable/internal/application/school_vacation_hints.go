package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SchoolVacationDays is the Timetable's port onto the School Calendar's
// days for the vacation hint (#5345): the statutory holidays, the closing
// days and the holiday periods the school entered.
type SchoolVacationDays interface {
	timetable.NonWorkingDayCalendar
	ListCalendarPeriods(ctx context.Context, filter schoolcalendar.CalendarPeriodFilter) ([]schoolcalendar.CalendarPeriod, error)
}

// SchoolVacationReader is the Timetable's port onto the School Calendar's
// vacations of the school's federal state with the school's confirmations.
type SchoolVacationReader interface {
	TenantSchoolVacations(ctx context.Context, from, to string) (schoolcalendar.SchoolVacationList, error)
}

// SchoolVacationHintRows are the Timetable reads of the hint: the planned
// occurrences with their execution state and their series.
type SchoolVacationHintRows interface {
	ports.LegacyInstanceReads
	ListGroups(ctx context.Context, filter timetable.GroupFilter) ([]timetable.Group, error)
}

// SchoolVacationHintService counts the planned Regeltermine in the coming
// vacations of the school's federal state. It only reads.
type SchoolVacationHintService struct {
	rows      SchoolVacationHintRows
	days      SchoolVacationDays
	vacations SchoolVacationReader
	today     func() string
}

var _ timetable.SchoolVacationHintQuery = (*SchoolVacationHintService)(nil)

// NewSchoolVacationHints composes the hint; today names the school's
// calendar day in YYYY-MM-DD, nil means the Berlin day.
func NewSchoolVacationHints(rows SchoolVacationHintRows, days SchoolVacationDays, vacations SchoolVacationReader, today func() string) (*SchoolVacationHintService, error) {
	if rows == nil || days == nil || vacations == nil {
		return nil, errors.New("timetable school vacation hints: all dependencies are required")
	}
	if today == nil {
		today = NewToday()
	}
	return &SchoolVacationHintService{rows: rows, days: days, vacations: vacations, today: today}, nil
}

// SchoolVacationHints loads, in a fixed number of queries, the vacations
// starting within the lookahead, the Regeltermine in them from today on,
// their series, the holiday periods and the non-working days, and selects
// the hints. A school without a usable federal state gets no hint.
func (s *SchoolVacationHintService) SchoolVacationHints(ctx context.Context) ([]timetable.SchoolVacationHint, error) {
	today, err := calendar.ParseDate(s.today())
	if err != nil {
		return nil, fmt.Errorf("school vacation hints: invalid today: %w", err)
	}
	horizon := today.AddDays(timetable.SchoolVacationHintLookaheadDays)
	list, err := s.vacations.TenantSchoolVacations(ctx, today.String(), horizon.String())
	if errors.Is(err, schoolcalendar.ErrFederalStateUnavailable) {
		return []timetable.SchoolVacationHint{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("school vacation hints: load vacations: %w", err)
	}
	windows, last := openVacationWindows(list.Vacations, today.String())
	if len(windows) == 0 {
		return []timetable.SchoolVacationHint{}, nil
	}
	candidates, err := s.candidates(ctx, today.String(), last)
	if err != nil {
		return nil, err
	}
	days, err := timetable.LoadNonWorkingDays(ctx, s.days, today.String(), last)
	if err != nil {
		return nil, fmt.Errorf("school vacation hints: %w", err)
	}
	return timetable.SelectSchoolVacationHints(windows, candidates, days, today.String(), horizon.String()), nil
}

// openVacationWindows keeps the unconfirmed windows and names the last day
// any of them covers.
func openVacationWindows(vacations []schoolcalendar.SchoolVacation, today string) ([]timetable.SchoolVacationWindow, string) {
	windows := make([]timetable.SchoolVacationWindow, 0, len(vacations))
	last := today
	for _, vacation := range vacations {
		if vacation.Acknowledged || vacation.EndDate < today {
			continue
		}
		windows = append(windows, timetable.SchoolVacationWindow{Name: vacation.Name, StartDate: vacation.StartDate, EndDate: vacation.EndDate})
		last = max(last, vacation.EndDate)
	}
	return windows, last
}

// candidates loads the planned Regeltermine of [from, to] with the closing
// day flag of their series and whether their period is a holiday period:
// one instance, one group and one period query.
func (s *SchoolVacationHintService) candidates(ctx context.Context, from, to string) ([]timetable.SchoolVacationCandidate, error) {
	instances, err := s.rows.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{
		FromDate: &from, ToDate: &to, MaterializedPlanned: true,
	})
	if err != nil {
		return nil, fmt.Errorf("school vacation hints: load instances: %w", err)
	}
	if len(instances) == 0 {
		return []timetable.SchoolVacationCandidate{}, nil
	}
	includeClosing, err := s.seriesIncludingClosingDays(ctx, instances)
	if err != nil {
		return nil, err
	}
	holidayPeriods, err := s.holidayPeriodIDs(ctx, from, to)
	if err != nil {
		return nil, err
	}
	candidates := make([]timetable.SchoolVacationCandidate, 0, len(instances))
	for _, instance := range instances {
		candidate := timetable.SchoolVacationCandidate{Date: instance.Date.String()}
		if instance.ActivityGroupID != nil {
			candidate.SeriesIncludesClosingDays = includeClosing[*instance.ActivityGroupID]
		}
		if instance.CalendarPeriodID != nil {
			candidate.InHolidayPeriod = holidayPeriods[*instance.CalendarPeriodID]
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func (s *SchoolVacationHintService) seriesIncludingClosingDays(ctx context.Context, instances []*scheduleModels.ActivityInstance) (map[int64]bool, error) {
	seen := make(map[int64]bool, len(instances))
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance.ActivityGroupID != nil && !seen[*instance.ActivityGroupID] {
			seen[*instance.ActivityGroupID] = true
			ids = append(ids, *instance.ActivityGroupID)
		}
	}
	groups, err := s.rows.ListGroups(ctx, timetable.GroupFilter{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("school vacation hints: load series: %w", err)
	}
	result := make(map[int64]bool, len(groups))
	for _, group := range groups {
		result[group.ID] = group.IncludeClosingDays
	}
	return result, nil
}

// holidayPeriodIDs names the holiday periods the school entered that touch
// [from, to], active or not: a series bound to one is holiday care on
// purpose.
func (s *SchoolVacationHintService) holidayPeriodIDs(ctx context.Context, from, to string) (map[int64]bool, error) {
	periods, err := s.days.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		PeriodType: schoolcalendar.PeriodTypeHoliday, OverlappingFrom: from, OverlappingTo: to,
	})
	if err != nil {
		return nil, fmt.Errorf("school vacation hints: load holiday periods: %w", err)
	}
	result := make(map[int64]bool, len(periods))
	for _, period := range periods {
		result[period.ID] = true
	}
	return result, nil
}
