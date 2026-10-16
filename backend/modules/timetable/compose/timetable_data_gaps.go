package compose

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListUnderstaffedInstances loads the window's planned and running blocks
// with one read, their staff rows with a second, and returns the
// understaffed ones with their counts. Completed and cancelled blocks are
// history and never reported.
func (d *timetableData) ListUnderstaffedInstances(ctx context.Context, from, to calendar.Date) ([]timetable.UnderstaffedInstance, error) {
	instances, err := d.deps.Instances.FindByTenantAndDateRange(ctx, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	candidates := make([]*scheduleModels.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == scheduleModels.InstanceStatusPlanned || inst.Status == scheduleModels.InstanceStatusActive {
			candidates = append(candidates, inst)
		}
	}
	staffRows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, retainedInstanceIDs(candidates))
	if err != nil {
		return nil, err
	}
	rowsByInstance := application.IndexInstanceStaffRows(staffRows)
	gaps := make([]timetable.UnderstaffedInstance, 0)
	for _, inst := range candidates {
		rows := rowsByInstance[inst.ID]
		if timetable.IsUnderstaffedWithMinimum(application.StaffingRowsOf(rows), application.DutyMinimumStaff(inst)) {
			gaps = append(gaps, understaffedInstanceOf(inst, rows))
		}
	}
	return gaps, nil
}

// understaffedInstanceOf counts an understaffed block's staff rows.
func understaffedInstanceOf(inst *scheduleModels.ActivityInstance, rows []*scheduleModels.InstanceStaff) timetable.UnderstaffedInstance {
	gap := timetable.UnderstaffedInstance{Instance: ScheduledInstanceOf(inst), AssignedStaffCount: len(rows)}
	for _, row := range rows {
		if row.IsAbsent {
			gap.AbsentStaffCount++
		} else {
			gap.PresentStaffCount++
		}
		if !row.IsSubstitute {
			gap.PlannedStaffCount++
		}
	}
	return gap
}
