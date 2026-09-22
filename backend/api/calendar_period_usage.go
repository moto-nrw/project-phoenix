package api

import (
	"context"

	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// periodUsageSource is the planning owners' one-statement usage read.
type periodUsageSource interface {
	Usage(context.Context) (map[int64]timetableCompose.CalendarPeriodUsage, error)
}

// calendarPeriodUsage serves the timetable surface's period usage counts
// from the two planning owners' one-statement reads (#3124).
type calendarPeriodUsage struct {
	usage periodUsageSource
}

func (u calendarPeriodUsage) UsageCounts(ctx context.Context) (map[int64]timetableAPI.CalendarPeriodUsageCounts, error) {
	values, err := u.usage.Usage(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]timetableAPI.CalendarPeriodUsageCounts, len(values))
	for id, value := range values {
		result[id] = timetableAPI.CalendarPeriodUsageCounts(value)
	}
	return result, nil
}
