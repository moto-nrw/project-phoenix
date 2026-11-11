package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// lockCreatedTemplateStaff takes, after the gates and catalogue rows and
// before the template's first row, the staff rows of its complete roster
// (weekday-only people included) and of the live people the caller's
// materialization window plans, in one ascending batch (#6702). The roster
// write and the materialization that follows in the same unit then only
// re-read held rows; without the window people they would lock other
// templates' supervisors after the new template's roster.
func (s *TemplateService) lockCreatedTemplateStaff(ctx context.Context, in CreateTemplateInput) error {
	roster, err := resolveTemplateRoster(in.Weekdays, in.StudentIDs, in.StaffIDs, in.PrimaryStaffID, in.WeekdayAssignments)
	if err != nil {
		return &ports.ScheduleError{Op: "create template: resolve roster", Err: err}
	}
	staffIDs := make([]int64, 0, len(roster.Staff))
	for _, row := range roster.Staff {
		staffIDs = append(staffIDs, row.PersonID)
	}
	windowStaff, err := s.liveWindowStaff(ctx, in)
	if err != nil {
		return err
	}
	if err := lockStaffRows(ctx, s.deps.InstanceStaffRepo, append(staffIDs, windowStaff...)); err != nil {
		return &ports.ScheduleError{Op: "create template: lock staff", Err: err}
	}
	return nil
}

// liveWindowStaff lists the live people the materialization window plans
// over the existing templates; the new template's people are its roster.
func (s *TemplateService) liveWindowStaff(ctx context.Context, in CreateTemplateInput) ([]int64, error) {
	if in.MaterializeFrom == nil || in.MaterializeTo == nil {
		return nil, nil
	}
	roster, ok := s.deps.Materialization.(materializationStaffRoster)
	if !ok {
		return nil, nil
	}
	candidates, err := roster.staffForMaterialization(ctx, *in.MaterializeFrom, *in.MaterializeTo)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	live, err := s.deps.Directories.StaffPersons(ctx, candidates, false)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "create template: load materialization staff", Err: err}
	}
	ids := make([]int64, 0, len(live))
	for id := range live {
		ids = append(ids, id)
	}
	return ids, nil
}
