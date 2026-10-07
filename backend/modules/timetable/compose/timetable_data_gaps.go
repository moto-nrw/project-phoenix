package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ListUnderstaffedInstances loads the window's planned and running blocks
// with one read, their staff rows with a second, and returns the
// understaffed ones with their counts. Completed and cancelled blocks are
// history and never reported.
func (d *timetableData) ListUnderstaffedInstances(ctx context.Context, from, to timezone.Date) ([]timetable.UnderstaffedInstance, error) {
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
	rowsByInstance := indexInstanceStaffRows(staffRows)
	gaps := make([]timetable.UnderstaffedInstance, 0)
	for _, inst := range candidates {
		rows := rowsByInstance[inst.ID]
		if timetable.IsUnderstaffedWithMinimum(staffingRowsOf(rows), dutyMinimumStaff(inst)) {
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

// dutyMinimumStaff is the staffing floor of a duty (#3822): its occurrence
// pin, otherwise the template's „Benötigtes Personal“. Other blocks have none;
// their gap rule stays the absence rule.
func dutyMinimumStaff(inst *scheduleModels.ActivityInstance) int {
	if inst == nil || inst.TemplateType != timetable.GroupTypeDuty {
		return 0
	}
	override := inst.RequiredStaff
	if override == nil {
		override = inst.TemplateRequiredStaff
	}
	return timetable.EffectiveRequiredStaff(override, 0, 1)
}

// staffingRowsOf maps staff rows onto the owner's staffing vocabulary.
func staffingRowsOf(rows []*scheduleModels.InstanceStaff) []timetable.InstanceStaff {
	staffing := make([]timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			staffing = append(staffing, timetable.InstanceStaff{IsAbsent: row.IsAbsent, IsSubstitute: row.IsSubstitute})
		}
	}
	return staffing
}
