package timetracking_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/config/settingstest"
	"github.com/stretchr/testify/require"
)

func TestScheduleTargetsKeepVersionBoundariesAndHistorySeparate(t *testing.T) {
	t.Parallel()
	first := timezone.NewDate(2026, 3, 23)
	boundary := first.AddDays(7)
	reader := services.NewWorkScheduleTargets(settingstest.Schedules(
		settingstest.ScheduleRow{StaffID: 7, ValidFrom: first, ValidUntil: boundary, RotationLength: 1, DayOfWeek: 0, TargetMinutes: 360},
		settingstest.ScheduleRow{StaffID: 7, ValidFrom: boundary, RotationLength: 1, DayOfWeek: 0, TargetMinutes: 420},
	))
	ctx := context.Background()
	plan, err := reader.TargetsForStaff(ctx, 7, first, boundary)
	require.NoError(t, err)
	require.True(t, plan.HasEntries)
	for _, test := range []struct {
		date    timezone.Date
		minutes int
		matched bool
	}{
		{first.AddDays(-7), 0, false}, {first, 360, true}, {first.AddDays(1), 0, false}, {boundary, 420, true},
	} {
		minutes, matched := plan.DailyTarget(nil, test.date)
		require.Equal(t, test.minutes, minutes)
		require.Equal(t, test.matched, matched)
	}
	batch, err := reader.TargetsByStaff(ctx, []int64{7}, first, boundary)
	require.NoError(t, err)
	require.Len(t, batch, 1)
	minutes, matched := batch[7].DailyTarget(nil, boundary)
	require.Equal(t, 420, minutes)
	require.True(t, matched)
	emptyReader := services.NewWorkScheduleTargets(settingstest.Schedules())
	empty, err := emptyReader.TargetsForStaff(ctx, 7, first, boundary)
	require.NoError(t, err)
	require.False(t, empty.HasEntries)
	history, err := emptyReader.HasScheduleHistory(ctx, 7)
	require.NoError(t, err)
	require.True(t, history)
}
