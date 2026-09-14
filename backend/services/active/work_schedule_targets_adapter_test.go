package active_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/stretchr/testify/require"
)

type scheduleTargetRecords struct{ rows []*config.StaffWorkSchedule }

func (r scheduleTargetRecords) FindByStaffIDsValidInRange(context.Context, []int64, config.CalendarDate, config.CalendarDate) ([]*config.StaffWorkSchedule, error) {
	return r.rows, nil
}
func (r scheduleTargetRecords) HasScheduleHistory(context.Context, int64) (bool, error) {
	return true, nil
}
func (r scheduleTargetRecords) FindStaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error) {
	return map[int64]bool{7: true}, nil
}

func TestScheduleTargetsKeepVersionBoundariesAndHistorySeparate(t *testing.T) {
	t.Parallel()
	first := timezone.NewDate(2026, 3, 23)
	boundary := first.AddDays(7)
	until := config.CalendarDate(boundary)
	reader := services.NewWorkScheduleTargets(scheduleTargetRecords{rows: []*config.StaffWorkSchedule{
		{StaffID: 7, ValidFrom: config.CalendarDate(first), ValidUntil: &until, RotationLength: 1, DayOfWeek: 0, TargetMinutes: 360},
		{StaffID: 7, ValidFrom: until, RotationLength: 1, DayOfWeek: 0, TargetMinutes: 420},
	}})
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
	emptyReader := services.NewWorkScheduleTargets(scheduleTargetRecords{})
	empty, err := emptyReader.TargetsForStaff(ctx, 7, first, boundary)
	require.NoError(t, err)
	require.False(t, empty.HasEntries)
	history, err := emptyReader.HasScheduleHistory(ctx, 7)
	require.NoError(t, err)
	require.True(t, history)
}
