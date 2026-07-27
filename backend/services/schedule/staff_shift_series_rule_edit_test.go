package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #2028: the series editor changes the rule itself — weekdays, rhythm, window,
// validity — and the change applies from the opened occurrence onwards. It
// goes through the existing split, so these tests pin the fields the split
// gained rather than a second write path.

func TestStaffShiftSeries_SplitAppliesNewWeekdaysFromEffectiveDate(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	// A Tuesday-only series.
	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	series.Weekdays = []int16{2}
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	effective := today.AddDays(1)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: effective,
			// The planner adds Friday and keeps everything else.
			Weekdays:     []int16{2, 5},
			StartTime:    series.StartTime,
			EndTime:      series.EndTime,
			BreakMinutes: series.BreakMinutes,
			ActorStaffID: env.staff.ID,
		})
		return err
	})

	rows := env.shiftsInRange(t, effective, periodEnd)
	require.NotEmpty(t, rows)
	sawFriday := false
	for _, row := range rows {
		weekday := row.Date.Weekday()
		assert.Contains(t, []time.Weekday{time.Tuesday, time.Friday}, weekday,
			"only the edited weekdays may remain, got %s on %s", weekday, row.Date)
		if weekday == time.Friday {
			sawFriday = true
		}
	}
	assert.True(t, sawFriday, "the added weekday must be planned")
}

func TestStaffShiftSeries_SplitShortensValidityAndDropsLaterShifts(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, today.AddDays(-1), periodEnd, 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-1), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	// valid_until is exclusive: the last planned day is today+5.
	newEnd := today.AddDays(6)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: today.AddDays(1),
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ValidUntil:    &newEnd,
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	assert.NotEmpty(t, env.shiftsInRange(t, today.AddDays(1), today.AddDays(5)))
	assert.Empty(t, env.shiftsInRange(t, newEnd, periodEnd),
		"nothing may remain after the new end date")
}

func TestStaffShiftSeries_SplitKeepsStoredValidityWhenUnset(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-1), today.AddDays(28), 1, nil)

	storedEnd := today.AddDays(10)
	series := env.buildSeries(t, periodID, today.AddDays(-1), &storedEnd, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	// Editing only the window must not silently drop the stored end date.
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: today.AddDays(1),
			StartTime:     seriesClock(t, "13:00"),
			EndTime:       seriesClock(t, "15:00"),
			BreakMinutes:  0,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.NotNil(t, result.Series)
	require.NotNil(t, result.Series.ValidUntil)
	assert.Equal(t, storedEnd, *result.Series.ValidUntil)
	assert.Empty(t, env.shiftsInRange(t, storedEnd, today.AddDays(28)))
}

func TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(14)
	periodID := env.createPeriod(t, today.AddDays(-1), periodEnd, 1, nil)
	series := env.buildSeries(t, periodID, today.AddDays(-1), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	tooLate := periodEnd.AddDays(2) // valid_until is exclusive; periodEnd+1 is the limit.
	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: today.AddDays(1),
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ValidUntil:    &tooLate,
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
}

func TestStaffShiftSeries_SplitBoundsEarlierSegmentAtNextSuccessor(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, today.AddDays(-1), periodEnd, 1, nil)
	series := env.buildSeries(t, periodID, today.AddDays(-1), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	downstreamFrom := today.AddDays(10)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: downstreamFrom,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	var result *scheduleSvc.SeriesResult
	earlierFrom := today.AddDays(3)
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: earlierFrom,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			// Clearing the end must not extend across the already-created successor.
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	require.NotNil(t, result.Series.ValidUntil)
	assert.Equal(t, downstreamFrom, *result.Series.ValidUntil)
}
