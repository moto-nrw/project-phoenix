package application

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
