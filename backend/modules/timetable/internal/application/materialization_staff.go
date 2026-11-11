package application

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
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

// lockMaterializationStaff takes the staff row of every supervisor the run
// copies, ascending and before its first occurrence row (#6702). It plans
// the candidates the run creates with the run's own rules (kept slot, not
// yet existing) and their valid supervisors; the copy itself then only
// re-reads held rows. Two runs, or a run and a writer that locks the same
// people, never hold one person each while they wait for the other's.
func (s *materializationService) lockMaterializationStaff(
	ctx context.Context,
	templates []*activityContract.Group,
	schedules map[int64][]*activityContract.Schedule,
	people map[int64]templateRoster,
	world *materializationWorld,
) error {
	var staff []int64
	planned := make(map[existingKey]struct{})
	for _, tmpl := range templates {
		staff = world.plannedSupervisorStaff(staff, planned, tmpl, schedules[tmpl.ID], people[tmpl.ID].supervisors)
	}
	if err := lockStaffRows(ctx, s.staffRepo, staff); err != nil {
		return &ports.ScheduleError{Op: "materialize for tenant: lock supervisor staff", Err: err}
	}
	return nil
}

// plannedSupervisorStaff appends the supervisors of every occurrence of the
// template the run creates: the same walk, slot rules and existing-slot
// index as materializeTemplate, without writing. planned collects the slots
// the walk already counted, as the run marks them in its index.
func (w *materializationWorld) plannedSupervisorStaff(staff []int64, planned map[existingKey]struct{}, tmpl *activityContract.Group, schedules []*activityContract.Schedule, supervisors []*activityContract.SupervisorPlanned) []int64 {
	for date := w.from; !date.After(w.to); date = date.AddDays(1) {
		if isWeekend(date) && !w.weekendFollowsFriday {
			continue
		}
		weekday := planWeekday(date, w.weekendFollowsFriday)
		for _, sch := range schedules {
			if sch.Weekday != weekday {
				continue
			}
			if period, ok := w.plannedSlot(planned, tmpl, sch, date); ok {
				staff = appendValidSupervisors(staff, supervisors, date, period.ID, weekday)
			}
		}
	}
	return staff
}

// plannedSlot reports whether the run creates the candidate: its slot is
// kept, does not exist yet and was not counted before.
func (w *materializationWorld) plannedSlot(planned map[existingKey]struct{}, tmpl *activityContract.Group, sch *activityContract.Schedule, date calendar.Date) (*schedule.CalendarPeriod, bool) {
	exc := w.exceptionIdx[exceptionKey{tmpl.ID, date}]
	effective, period, skip := candidateSlot(tmpl, sch, date, w.periods, w.days, exc, w.timeframeByID, slog.New(slog.DiscardHandler))
	if skip != candidateKept {
		return nil, false
	}
	key := existingKey{ActivityGroupID: tmpl.ID, Date: date, StartTime: FormatTimeOfDay(effective.StartTime)}
	if _, exists := w.existingIdx[key]; exists {
		return nil, false
	}
	if _, counted := planned[key]; counted {
		return nil, false
	}
	planned[key] = struct{}{}
	return period, true
}

func appendValidSupervisors(staff []int64, supervisors []*activityContract.SupervisorPlanned, date calendar.Date, periodID int64, weekday int) []int64 {
	for _, supervisor := range supervisors {
		if isSupervisorValidOn(supervisor, date, periodID, weekday) {
			staff = append(staff, supervisor.StaffID)
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
