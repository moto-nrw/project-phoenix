package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// IndexInstanceStaffRows groups staff rows by their instance.
func IndexInstanceStaffRows(rows []*scheduleModels.InstanceStaff) map[int64][]*scheduleModels.InstanceStaff {
	byInstance := make(map[int64][]*scheduleModels.InstanceStaff)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

// StaffingRowsOf maps staff rows onto the owner's staffing vocabulary.
func StaffingRowsOf(rows []*scheduleModels.InstanceStaff) []timetable.InstanceStaff {
	staffing := make([]timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			staffing = append(staffing, timetable.InstanceStaff{IsAbsent: row.IsAbsent, IsSubstitute: row.IsSubstitute})
		}
	}
	return staffing
}

// DutyMinimumStaff is the staffing floor of a duty (#3822): its occurrence
// pin, otherwise the template's „Benötigtes Personal“. Other blocks have none;
// their gap rule stays the absence rule.
func DutyMinimumStaff(inst *scheduleModels.ActivityInstance) int {
	if inst == nil || inst.TemplateType != timetable.GroupTypeDuty {
		return 0
	}
	override := inst.RequiredStaff
	if override == nil {
		override = inst.TemplateRequiredStaff
	}
	return timetable.EffectiveRequiredStaff(override, 0, 1)
}
