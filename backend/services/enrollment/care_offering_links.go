package enrollment

import (
	"github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking materialization still reads timetable and calendar rows here while
// the link rules it applies belong to the Care Plan catalog (#3559). These
// translations hand the rows to the owner's link vocabulary; they decide
// nothing.

func offeringPhaseOf(phase *capability.Phase) careplan.OfferingPhase {
	return careplan.OfferingPhase{
		ID: phase.ID, Name: phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate), ServiceEnd: calendar.Date(phase.ServiceEndDate),
	}
}

func linkedGroupOf(group *activities.Group) careplan.LinkedGroup {
	return careplan.LinkedGroup{
		ID: group.ID, IsTemplate: group.IsTemplate, Archived: group.ArchivedAt != nil,
		CalendarPeriodID: group.CalendarPeriodID, PlannedRoomID: group.PlannedRoomID,
	}
}

func linkedSchedulesOf(schedules []*activities.Schedule) []careplan.LinkedSchedule {
	result := make([]careplan.LinkedSchedule, 0, len(schedules))
	for _, schedule := range schedules {
		if schedule == nil {
			continue
		}
		result = append(result, careplan.LinkedSchedule{
			GroupID: schedule.ActivityGroupID, Weekday: schedule.Weekday, TimeframeID: schedule.TimeframeID,
			CalendarPeriodID: schedule.CalendarPeriodID, WeekPattern: schedule.WeekPattern,
			ValidFrom: linkedDate(schedule.ValidFrom), ValidUntil: linkedDate(schedule.ValidUntil),
		})
	}
	return result
}

func linkedDate(value *activities.Date) *calendar.Date {
	if value == nil {
		return nil
	}
	date := calendar.Date(*value)
	return &date
}

func linkedPeriodOf(period *scheduleModels.CalendarPeriod) *careplan.LinkedPeriod {
	if period == nil {
		return nil
	}
	linked := &careplan.LinkedPeriod{
		ID: period.ID, StartDate: calendar.Date(period.StartDate), EndDate: calendar.Date(period.EndDate),
		IsActive: period.IsActive, WeekCycleLength: period.WeekCycleLength,
	}
	if period.WeekCycleAnchor != nil {
		linked.WeekCycleAnchor = string(*period.WeekCycleAnchor)
	}
	return linked
}

// phaseWithinTemplatePeriod applies the catalog's phase-within-period rule;
// an unknown phase or an unpinned template passes.
func phaseWithinTemplatePeriod(phase *capability.Phase, period *scheduleModels.CalendarPeriod) error {
	if phase == nil || period == nil {
		return nil
	}
	return careplan.ValidatePhaseWithinPeriod(offeringPhaseOf(phase), linkedPeriodOf(period))
}
