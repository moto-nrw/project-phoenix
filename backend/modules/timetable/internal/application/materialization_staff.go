package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// materializationStaffRoster lets the replan lock its old and new people in
// one ascending batch. The caller holds recurrence, grade and day locks;
// template rosters cannot change before materialization in the same unit.
type materializationStaffRoster interface {
	staffForMaterialization(context.Context, calendar.Date, calendar.Date) ([]int64, error)
}

// staffForMaterialization uses the engine's candidate and supervisor rules,
// across every template: even a template-scoped replan materializes the
// whole window. Existing occurrences may be deleted by the caller, so their
// presence does not exclude a candidate here.
func (s *materializationService) staffForMaterialization(ctx context.Context, from, to calendar.Date) ([]int64, error) {
	if err := validateMaterializationWindow(from, to); err != nil {
		return nil, err
	}
	periods, templates, done, err := s.loadMaterializationPreconditions(ctx, &timetable.MaterializationResult{})
	if err != nil || done {
		return nil, err
	}
	schedules, err := s.materializationSchedules(ctx, templates)
	if err != nil {
		return nil, err
	}
	ids := scheduledTemplateIDs(templates, schedules)
	if len(ids) == 0 {
		return nil, nil
	}
	values, err := s.supervisorRepo.ListPlannedSupervisors(ctx, timetable.PlannedSupervisorFilter{GroupIDs: ids})
	if err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: load planned supervisors", Err: err}
	}
	supervisors := make(map[int64][]*activityContract.SupervisorPlanned)
	for _, row := range legacyPlannedSupervisors(values) {
		supervisors[row.GroupID] = append(supervisors[row.GroupID], row)
	}
	world, err := s.loadOccurrenceRules(ctx, from, to, periods)
	if err != nil {
		return nil, err
	}
	var staff []int64
	for _, tmpl := range templates {
		staff = append(staff, world.templateStaff(tmpl, schedules[tmpl.ID], supervisors[tmpl.ID])...)
	}
	return staff, nil
}

func (w *materializationWorld) templateStaff(tmpl *activityContract.Group, schedules []*activityContract.Schedule, supervisors []*activityContract.SupervisorPlanned) []int64 {
	var staff []int64
	_, times := w.occurrenceDates(tmpl, schedules)
	for day, clocks := range times {
		for _, clock := range clocks {
			for _, supervisor := range supervisors {
				if isSupervisorValidOn(supervisor, day, clock.PeriodID, planWeekday(day, w.weekendFollowsFriday)) {
					staff = append(staff, supervisor.StaffID)
				}
			}
		}
	}
	return staff
}

// A candidate may already be occupied by immutable history of a retired
// person. Materialization skips that slot; a retired person needs no lock.
func (s *InstanceLifecycleService) liveMaterializationStaff(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	live, err := s.deps.StaffRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "re-plan: load materialization staff", Err: err}
	}
	staff := make([]int64, 0, len(live))
	for id, person := range live {
		if person != nil {
			staff = append(staff, id)
		}
	}
	return staff, nil
}
