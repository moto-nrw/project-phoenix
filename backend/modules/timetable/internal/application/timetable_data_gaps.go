package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (d *timetableData) ListUnderstaffedInstances(ctx context.Context, from, to calendar.Date) ([]timetable.UnderstaffedInstance, error) {
	instances, err := instancesBetween(ctx, d.deps.Instances, calendar.Date(from), calendar.Date(to))
	if err != nil {
		return nil, err
	}
	candidates := make([]*scheduleModels.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == scheduleModels.InstanceStatusPlanned || inst.Status == scheduleModels.InstanceStatusActive {
			candidates = append(candidates, inst)
		}
	}
	staffRows, err := staffOfInstances(ctx, d.deps.InstanceStaff, retainedInstanceIDs(candidates))
	if err != nil {
		return nil, err
	}
	rowsByInstance := IndexInstanceStaffRows(staffRows)
	gaps := make([]timetable.UnderstaffedInstance, 0)
	for _, inst := range candidates {
		rows := rowsByInstance[inst.ID]
		if timetable.IsUnderstaffedWithMinimum(StaffingRowsOf(rows), DutyMinimumStaff(inst)) {
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
