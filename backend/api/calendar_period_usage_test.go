package api

import (
	"context"
	"testing"

	timetableAPI "github.com/moto-ogs/project-phoenix/modules/timetable/http"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

type fixedPeriodUsage periodUsages

func (f fixedPeriodUsage) Usage(context.Context) (periodUsages, error) {
	return periodUsages(f), nil
}

type fixedPeriodShiftSeries map[int64]int

func (f fixedPeriodShiftSeries) ShiftPlanReferences(context.Context) (workforce.ShiftPlanReferences, error) {
	return workforce.ShiftPlanReferences{SeriesByCalendarPeriod: f}, nil
}

// TestCalendarPeriodUsageCountsShiftSeries guards #6845: the deletion
// preview names the Dienstplan series, also of a period nothing else uses.
func TestCalendarPeriodUsageCountsShiftSeries(t *testing.T) {
	t.Parallel()
	usage := calendarPeriodUsage{
		usage:       fixedPeriodUsage{7: {EnrollmentPhases: 1, ActivityGroups: 2}},
		shiftSeries: fixedPeriodShiftSeries{7: 3, 9: 1},
	}

	counts, err := usage.UsageCounts(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[int64]timetableAPI.CalendarPeriodUsageCounts{
		7: {EnrollmentPhases: 1, ActivityGroups: 2, StaffShiftSeries: 3},
		9: {StaffShiftSeries: 1},
	}, counts)

	_, err = calendarPeriodUsage{usage: usage.usage}.UsageCounts(context.Background())
	require.Error(t, err, "missing Workforce wiring must not report a period as unused")
}
