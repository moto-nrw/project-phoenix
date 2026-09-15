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
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
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
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
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
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
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
	assert.Equal(t, scheduleModels.Date(storedEnd), *result.Series.ValidUntil)
	assert.Empty(t, env.shiftsInRange(t, storedEnd, today.AddDays(28)))
}

func TestStaffShiftSeries_SplitRejectsValidityBeyondCalendarPeriod(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
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
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
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
	assert.Equal(t, scheduleModels.Date(downstreamFrom), *result.Series.ValidUntil)
}

func TestStaffShiftSeries_SplitRejectsWhenNextSegmentLeavesNoOccurrence(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, today.AddDays(-1), periodEnd, 1, nil)
	effective := today.AddDays(1)
	nextFrom := effective.AddDays(1)
	weekday := int16((int(nextFrom.Weekday())+6)%7 + 1)
	series := env.buildSeries(t, periodID, today.AddDays(-1), nil, scheduleModels.WeekPatternEvery)
	series.Weekdays = []int16{weekday}
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: nextFrom,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: effective,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	assert.Contains(t, err.Error(), "no occurrences left to change")
	assert.NotEmpty(t, env.shiftsInRange(t, nextFrom, nextFrom),
		"the rejected edit must leave the later segment's shift intact")
}

func TestStaffShiftSeries_SplitRejectsSupersededSegment(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)
	effective := today.AddDays(1)

	oldEnd := effective
	series := env.buildSeries(t, periodID, today.AddDays(-7), &oldEnd, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		return env.repos.StaffShiftSeries.Create(ctx, series)
	})

	rootID := series.ID
	successor := env.buildSeries(t, periodID, effective, nil, scheduleModels.WeekPatternEvery)
	successor.SeriesRootID = &rootID
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, successor)
		return err
	})

	newEnd := today.AddDays(15)
	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: effective,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ValidUntil:    &newEnd,
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	assert.Contains(t, err.Error(), "already been superseded")
}

// A series whose last planned day has arrived was permanently uneditable: the
// effective date is clamped to tomorrow, and the segment check compared that
// against the STORED end. Extending "Gültig bis" is the one edit that has to
// work there, otherwise "Serie bearbeiten" cannot edit the series at all.
func TestStaffShiftSeries_SplitExtendsSeriesEndingToday(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	// valid_until is exclusive: the last day carrying a shift is today.
	storedEnd := today.AddDays(1)
	series := env.buildSeries(t, periodID, today.AddDays(-7), &storedEnd, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		return env.repos.StaffShiftSeries.Create(ctx, series)
	})

	newEnd := today.AddDays(15)
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID: series.ID,
			// The planner opened an occurrence in the past; the split clamps it
			// to tomorrow rather than rejecting the edit.
			EffectiveDate: today.AddDays(-3),
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ValidUntil:    &newEnd,
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	require.NotNil(t, result.Series)
	assert.Equal(t, scheduleModels.Date(today.AddDays(1)), result.Series.ValidFrom)
	require.NotNil(t, result.Series.ValidUntil)
	assert.Equal(t, scheduleModels.Date(newEnd), *result.Series.ValidUntil)
	assert.NotEmpty(t, env.shiftsInRange(t, today.AddDays(1), today.AddDays(14)),
		"the extended segment must materialize shifts")
	assert.Empty(t, env.shiftsInRange(t, newEnd, periodEnd),
		"nothing may be planned past the new end date")
}

// The same series without an extended end has no day left to change. That is
// still a 400, but it must say so instead of passing for a generic input error.
func TestStaffShiftSeries_SplitRejectsWhenNoOccurrenceRemains(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(28), 1, nil)

	storedEnd := today.AddDays(1)
	series := env.buildSeries(t, periodID, today.AddDays(-7), &storedEnd, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		return env.repos.StaffShiftSeries.Create(ctx, series)
	})

	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: today.AddDays(-3),
			StartTime:     seriesClock(t, "13:00"),
			EndTime:       seriesClock(t, "15:00"),
			BreakMinutes:  0,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	assert.Contains(t, err.Error(), "no occurrences left to change")
}

func TestStaffShiftSeries_SplitRejectsExtensionWithoutRecurrenceOccurrence(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.NewDate(2026, 8, 24)
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(28), 1, nil)
	effective := today.AddDays(1)
	newEnd := effective.AddDays(2)

	// The two included dates are effective and effective+1. Select the next
	// weekday so an extension exists by date but cannot produce a shift.
	weekday := int16((int(newEnd.Weekday())+6)%7 + 1)
	storedEnd := effective
	series := env.buildSeries(t, periodID, today.AddDays(-7), &storedEnd, scheduleModels.WeekPatternEvery)
	series.Weekdays = []int16{weekday}
	env.inTx(t, func(ctx context.Context) error {
		return env.repos.StaffShiftSeries.Create(ctx, series)
	})

	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: effective,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ValidUntil:    &newEnd,
			ValidUntilSet: true,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	assert.Contains(t, err.Error(), "selected weekdays and week pattern")
}
