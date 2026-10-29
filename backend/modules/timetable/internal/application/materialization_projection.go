package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ProjectOccurrences returns, per template, the dates in [from, to] on which
// materialization creates an occurrence, without writing anything (#5140).
// It loads the per-date rule inputs the materializer loads
// (loadOccurrenceRules: holidays and closing days, exceptions, timeframes,
// the weekend rule) plus the active periods, and asks
// candidateSlot, the materializer's own per-date rule, for every schedule
// and date. Period selection, the A/B anchor of the selected period and the
// skipped days are therefore the materializer's by construction. Unknown or
// archived templates are absent. A constant number of reads.
func (s *materializationService) ProjectOccurrences(ctx context.Context, templateIDs []int64, from, to calendar.Date) (timetable.OccurrenceProjection, error) {
	projection := timetable.OccurrenceProjection{Dates: map[int64][]calendar.Date{}, ClosedDays: map[calendar.Date]bool{}}
	if from.After(to) {
		return projection, nil
	}
	if days := from.DaysUntil(to) + 1; days > timetable.MaxOccurrenceProjectionDays {
		return projection, &ports.ScheduleError{Op: "project occurrences", Err: fmt.Errorf("window exceeds %d days", timetable.MaxOccurrenceProjectionDays)}
	}
	periods, err := s.periodRepo.FindActiveByTenantID(ctx)
	if err != nil {
		return projection, &ports.ScheduleError{Op: "project occurrences: load periods", Err: err}
	}
	world, err := s.loadOccurrenceRules(ctx, from, to, periods)
	if err != nil {
		return projection, err
	}
	projection.ClosedDays = world.closedDays()
	if len(templateIDs) == 0 {
		return projection, nil
	}
	templates, byTemplate, err := s.projectedTemplates(ctx, templateIDs)
	if err != nil {
		return projection, err
	}
	for _, tmpl := range templates {
		if !tmpl.IsTemplate || tmpl.ArchivedAt != nil {
			continue
		}
		if dates := world.occurrenceDates(tmpl, byTemplate[tmpl.ID]); len(dates) > 0 {
			projection.Dates[tmpl.ID] = dates
		}
	}
	return projection, nil
}

// projectedTemplates loads the templates and their schedules in two reads.
func (s *materializationService) projectedTemplates(ctx context.Context, templateIDs []int64) ([]*activityContract.Group, map[int64][]*activityContract.Schedule, error) {
	templates, err := listLegacyGroups(ctx, s.groupRepo, timetable.GroupFilter{IDs: templateIDs}, "project occurrences: load templates")
	if err != nil {
		return nil, nil, &ports.ScheduleError{Op: "project occurrences: load templates", Err: err}
	}
	schedules, err := groupsSchedules(ctx, s.scheduleRepo, templateIDs)
	if err != nil {
		return nil, nil, &ports.ScheduleError{Op: "project occurrences: load schedules", Err: err}
	}
	byTemplate := make(map[int64][]*activityContract.Schedule, len(templateIDs))
	for _, sch := range schedules {
		byTemplate[sch.ActivityGroupID] = append(byTemplate[sch.ActivityGroupID], sch)
	}
	return templates, byTemplate, nil
}

// closedDays are the window's holidays and closing days.
func (w *materializationWorld) closedDays() map[calendar.Date]bool {
	closed := map[calendar.Date]bool{}
	for date := w.from; !date.After(w.to); date = date.AddDays(1) {
		if w.days.Skip(date.String(), false) != timetable.SkipNone {
			closed[date] = true
		}
	}
	return closed
}

// occurrenceDates replays materializeTemplate's date loop for one template
// and keeps the dates at least one schedule is materialized on.
func (w *materializationWorld) occurrenceDates(tmpl *activityContract.Group, schedules []*activityContract.Schedule) []calendar.Date {
	dates := make([]calendar.Date, 0)
	for date := w.from; !date.After(w.to); date = date.AddDays(1) {
		if isWeekend(date) && !w.weekendFollowsFriday {
			continue
		}
		isoWd := planWeekday(date, w.weekendFollowsFriday)
		for _, sch := range schedules {
			if sch.Weekday != isoWd {
				continue
			}
			exc := w.exceptionIdx[exceptionKey{tmpl.ID, date}]
			if _, _, skip := candidateSlot(tmpl, sch, date, w.periods, w.days, exc, w.timeframeByID, nil); skip == candidateKept {
				dates = append(dates, date)
				break
			}
		}
	}
	return dates
}
