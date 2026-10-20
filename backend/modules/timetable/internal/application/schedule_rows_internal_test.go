package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// publicTestSchedules serves legacy schedule rows of a unit test through the
// owner's schedule shape.
func publicTestSchedules(rows []*activityContract.Schedule) []timetable.Schedule {
	result := make([]timetable.Schedule, 0, len(rows))
	for _, row := range rows {
		input := publicScheduleInput(row)
		result = append(result, timetable.Schedule{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			Weekday: input.Weekday, TimeframeID: input.TimeframeID, ActivityGroupID: input.ActivityGroupID,
			WeekPattern: input.WeekPattern, CalendarPeriodID: input.CalendarPeriodID, ValidUntil: input.ValidUntil, ValidFrom: input.ValidFrom,
		})
	}
	return result
}
