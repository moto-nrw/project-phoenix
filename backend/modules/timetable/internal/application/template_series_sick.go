package application

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// sickDaysOfOccurrences takes the day lock of every occurrence day, ascending,
// then indexes the open sick reports over those days (#6292). The caller holds
// the recurrence gate, so the order is the documented recurrence → days
// ascending, and no planner moves or creates an occurrence meanwhile. The
// sick cascade marks a day's rows under the same lock: a report committing
// meanwhile is either read here or sees the rows the series change adds.
func (s *TemplateService) sickDaysOfOccurrences(ctx context.Context, instances []*scheduleModel.ActivityInstance) (map[staffDay]int64, error) {
	if s.deps.SickReports == nil || len(instances) == 0 {
		return nil, nil
	}
	days := occurrenceDays(instances)
	if s.deps.DayLocks != nil {
		tenantID := tenant.FromContext(ctx)
		for _, day := range days {
			if err := s.deps.DayLocks.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenantID, day)); err != nil {
				return nil, &ports.ScheduleError{Op: "reconcile predecessor staff: lock day " + day.String(), Err: err}
			}
		}
	}
	from, to := days[0], days[len(days)-1]
	reports, err := s.deps.SickReports.OpenSickReports(ctx, from, to)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "reconcile predecessor staff: load sick reports", Err: err}
	}
	return sickDayIndex(reports, from, to), nil
}

// occurrenceDays lists the distinct occurrence dates, ascending.
func occurrenceDays(instances []*scheduleModel.ActivityInstance) []calendar.Date {
	seen := make(map[calendar.Date]bool, len(instances))
	days := make([]calendar.Date, 0, len(instances))
	for _, inst := range instances {
		day := calendar.Date(inst.Date)
		if !seen[day] {
			seen[day] = true
			days = append(days, day)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days
}
