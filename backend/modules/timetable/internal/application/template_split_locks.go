package application

import (
	"context"
	"errors"
	"slices"

	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Lock old and successor people together before capping any supervisor or
// writing the successor. The later deviation snapshot is too late: it could
// wait for A while holding B or a supervisor row offboarding needs from A.
func (s *TemplateSplitService) lockSplitStaff(ctx context.Context, in TemplateSplitInput, supervisors []*activityContract.SupervisorPlanned, until *calendar.Date) error {
	from := in.EffectiveDate.String()
	spontaneous := false
	instances, err := listLegacyInstances(ctx, s.deps.InstanceRepo.Reads, ports.LegacyInstanceFilter{
		ActivityGroupID: &in.TemplateID, FromDate: &from, Status: "planned", IsSpontaneous: &spontaneous,
	}, "split template: load old occurrences")
	if err != nil {
		return err
	}
	rows, err := staffOfInstances(ctx, s.deps.InstanceStaff, instanceRowIDs(instances))
	if err != nil {
		return err
	}
	staffIDs := make([]int64, 0, len(rows)+len(supervisors))
	for _, row := range rows {
		staffIDs = append(staffIDs, row.StaffID)
	}
	for _, row := range supervisors {
		staffIDs = append(staffIDs, row.StaffID)
	}
	for _, row := range buildSuccessorStaffRows(in, supervisors, until) {
		staffIDs = append(staffIDs, row.StaffID)
	}
	// The split then materializes its window across every template: its
	// supervisors join the same ascending batch (#6702).
	materialized, err := s.liveMaterializationStaff(ctx, in)
	if err != nil {
		return err
	}
	staffIDs = append(staffIDs, materialized...)
	if err := lockStaffRows(ctx, s.deps.InstanceStaff, staffIDs); err != nil {
		return &ports.ScheduleError{Op: "split template: lock staff", Err: err}
	}
	return nil
}

// liveMaterializationStaff lists the live people the split's
// materialization window can plan; without a window it plans nobody.
func (s *TemplateSplitService) liveMaterializationStaff(ctx context.Context, in TemplateSplitInput) ([]int64, error) {
	from, to, ok := splitMaterializationWindow(in)
	if !ok {
		return nil, nil
	}
	// A materialization without the roster (a test double) locks nobody;
	// the real one would refuse a later lower person in its own pre-lock.
	roster, ok := s.deps.Materialization.(materializationStaffRoster)
	if !ok {
		return nil, nil
	}
	candidates, err := roster.staffForMaterialization(ctx, from, to)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	live, err := s.deps.Directories.StaffPersons(ctx, candidates, false)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "split template: load materialization staff", Err: err}
	}
	ids := make([]int64, 0, len(live))
	for id := range live {
		ids = append(ids, id)
	}
	return ids, nil
}

// validateSplitCatalogue pins the successor's category and a changed
// planning track, catalogue rows of the lock order, before the split's days
// and staff rows (#6702).
func (s *TemplateSplitService) validateSplitCatalogue(ctx context.Context, in TemplateSplitInput) error {
	if err := validateAssignableCategory(ctx, s.deps.CategoryRepo, in.CategoryID, "split template: validate category"); err != nil {
		return err
	}
	if !in.PlanningTrackIDProvided {
		return nil
	}
	old, err := s.loadTemplate(ctx, in.TemplateID)
	if err != nil {
		return err
	}
	if samePlanningTrackID(in.PlanningTrackID, old.PlanningTrackID) {
		return nil
	}
	return validateAssignablePlanningTrack(ctx, s.deps.PlanningTracks, in.PlanningTrackID, old.PlanningTrackID)
}

// lockSplitDays runs under recurrence, BEFORE any split writes. The mandatory
// order is recurrence (when needed) -> all affected days ascending -> rows.
// In particular, the predecessor offering resync deletes instance_students
// from today on, including occurrences before the cut and beyond the new
// materialization window. Locking only that window, or locking after resync,
// deadlocks against bulk/single cancellation holding a day and waiting on a
// participant (#5378). Recurrence keeps other planners from moving/creating
// occurrences while this read determines the complete set of days.
func (s *TemplateSplitService) lockSplitDays(ctx context.Context, in TemplateSplitInput, tenantID int64) error {
	days, err := s.splitWriteDays(ctx, in)
	if err != nil || len(days) == 0 {
		return err
	}
	if s.deps.Deviations == nil {
		return &ports.ScheduleError{Op: "split template: lock deviation days", Err: errors.New("deviation preservation is not configured")}
	}
	for _, day := range days {
		if err := s.deps.Deviations.LockDeviationDays(ctx, tenantID, day, day); err != nil {
			return &ports.ScheduleError{Op: "split template: lock deviation days", Err: err}
		}
	}
	return nil
}

func (s *TemplateSplitService) splitWriteDays(ctx context.Context, in TemplateSplitInput) ([]calendar.Date, error) {
	from := s.deps.Today().String()
	spontaneous := false
	instances, err := listLegacyInstances(ctx, s.deps.InstanceRepo.Reads, ports.LegacyInstanceFilter{
		ActivityGroupID: &in.TemplateID, FromDate: &from, Status: "planned", IsSpontaneous: &spontaneous,
	}, "split template: load affected days")
	if err != nil {
		return nil, err
	}
	unique := make(map[calendar.Date]struct{}, len(instances))
	for _, instance := range instances {
		unique[instance.Date] = struct{}{}
	}
	if from, to, ok := splitMaterializationWindow(in); ok {
		for day := from; !day.After(to); day = day.AddDays(1) {
			unique[day] = struct{}{}
		}
	}
	days := make([]calendar.Date, 0, len(unique))
	for day := range unique {
		days = append(days, day)
	}
	slices.Sort(days)
	return days, nil
}
