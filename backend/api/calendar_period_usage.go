package api

import (
	"context"
	"errors"

	timetableAPI "github.com/moto-ogs/project-phoenix/modules/timetable/http"
	timetablePorts "github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// periodUsages are the planning owners' usage counts per calendar period.
type periodUsages = map[int64]timetablePorts.CalendarPeriodUsage

// periodUsageSource is the planning owners' one-statement usage read.
type periodUsageSource interface {
	Usage(context.Context) (periodUsages, error)
}

// periodShiftSeriesSource is Workforce's one-statement count of the
// Dienstplan series per calendar period (#6845).
type periodShiftSeriesSource interface {
	ShiftPlanReferences(context.Context) (workforce.ShiftPlanReferences, error)
}

// calendarPeriodUsage serves the timetable surface's period usage counts
// from the planning owners' one-statement reads (#3124) and Workforce's
// series count, which blocks a deletion (#6845).
type calendarPeriodUsage struct {
	usage       periodUsageSource
	shiftSeries periodShiftSeriesSource
}

func (u calendarPeriodUsage) UsageCounts(ctx context.Context) (map[int64]timetableAPI.CalendarPeriodUsageCounts, error) {
	if u.usage == nil || u.shiftSeries == nil {
		return nil, errors.New("calendar period usage: planning and workforce reads are required")
	}
	values, err := u.usage.Usage(ctx)
	if err != nil {
		return nil, err
	}
	plan, err := u.shiftSeries.ShiftPlanReferences(ctx)
	if err != nil {
		return nil, err
	}
	series := plan.SeriesByCalendarPeriod
	result := make(map[int64]timetableAPI.CalendarPeriodUsageCounts, len(values)+len(series))
	for id, value := range values {
		result[id] = timetableAPI.CalendarPeriodUsageCounts{
			EnrollmentPhases: value.EnrollmentPhases, ActivityGroups: value.ActivityGroups, Schedules: value.Schedules,
			StudentEnrollments: value.StudentEnrollments, Supervisors: value.Supervisors, ActivityInstances: value.ActivityInstances,
		}
	}
	for id, count := range series {
		entry := result[id]
		entry.StaffShiftSeries = count
		result[id] = entry
	}
	return result, nil
}
