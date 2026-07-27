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

// #2028: "Alle Termine der Serie" rewrites the rule in place. The occurrence
// scopes ("Nur diese Woche", "Ab jetzt dauerhaft") keep working unchanged —
// these tests cover what the third scope adds.

func TestStaffShiftSeries_UpdateRewritesRuleAndRebuildsFuture(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodStart := today.AddDays(-7)
	periodEnd := today.AddDays(20)
	periodID := env.createPeriod(t, periodStart, periodEnd, 1, nil)

	series := env.buildSeries(t, periodID, periodStart, nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})
	before := env.shiftsInRange(t, periodStart, periodEnd)
	require.NotEmpty(t, before)

	// Narrow the rule to Mondays and move the window.
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.UpdateSeries(ctx, scheduleSvc.UpdateSeriesInput{
			SeriesID:     series.ID,
			Weekdays:     []int16{1},
			StartTime:    seriesClock(t, "14:00"),
			EndTime:      seriesClock(t, "17:00"),
			BreakMinutes: 15,
			ActorStaffID: env.staff.ID,
		})
		return err
	})
	require.NotNil(t, result)

	rows := env.shiftsInRange(t, periodStart, periodEnd)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		assert.True(t, row.Date.After(today), "the rebuild must not touch today or earlier: %s", row.Date)
		assert.Equal(t, time.Monday, row.Date.Weekday(), "only Mondays remain after the rule change")
		assert.Equal(t, "14:00", timezone.WallClock(row.StartTime).Format("15:04"))
		assert.Equal(t, "17:00", timezone.WallClock(row.EndTime).Format("15:04"))
		assert.Equal(t, 15, row.BreakMinutes)
	}

	// The rule itself changed — no successor segment was created (that is what
	// distinguishes the whole-series edit from a split).
	stored, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, []int16{1}, stored.Weekdays)
	assert.Nil(t, stored.ValidUntil)
	require.NotNil(t, stored.UpdatedBy)
	assert.Equal(t, env.staff.ID, *stored.UpdatedBy)
}

func TestStaffShiftSeries_UpdateKeepsPastAndCancelledOccurrences(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodStart := today.AddDays(-7)
	periodEnd := today.AddDays(20)
	periodID := env.createPeriod(t, periodStart, periodEnd, 1, nil)

	series := env.buildSeries(t, periodID, periodStart, nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	// One occurrence is edited for a single day (detaches it), a later one is
	// cancelled, a third is deleted (records an exception).
	editDate := today.AddDays(2)
	cancelDate := today.AddDays(3)
	deleteDate := today.AddDays(4)
	editRow := env.shiftsInRange(t, editDate, editDate)[0]
	cancelRow := env.shiftsInRange(t, cancelDate, cancelDate)[0]
	deleteRow := env.shiftsInRange(t, deleteDate, deleteDate)[0]

	env.inTx(t, func(ctx context.Context) error {
		updated := &scheduleModels.StaffShift{
			StaffID:   editRow.StaffID,
			Date:      editRow.Date,
			StartTime: seriesClock(t, "11:00"),
			EndTime:   seriesClock(t, "12:00"),
			CreatedBy: editRow.CreatedBy,
		}
		updated.ID = editRow.ID
		_, err := env.shifts.UpdateShift(ctx, updated)
		return err
	})
	cancelReason := "Fortbildung"
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.shifts.ApplyCancellation(ctx, scheduleSvc.CancelShiftInput{
			ShiftID:      cancelRow.ID,
			Cancelled:    true,
			ChangeReason: &cancelReason,
			ActorStaffID: env.staff.ID,
		})
		return err
	})
	env.inTx(t, func(ctx context.Context) error {
		return env.shifts.DeleteShift(ctx, deleteRow.ID)
	})

	// The deviation preview names exactly what the edit will overwrite.
	var deviations *scheduleSvc.SeriesDeviations
	env.inTx(t, func(ctx context.Context) error {
		var err error
		deviations, err = env.series.SeriesDeviations(ctx, series.ID)
		return err
	})
	require.NotNil(t, deviations)
	require.Len(t, deviations.Overwritten, 1)
	assert.Equal(t, editDate, deviations.Overwritten[0].Date)
	assert.Equal(t, scheduleSvc.SeriesDeviationTimeEdit, deviations.Overwritten[0].Kind)
	preservedDates := map[timezone.Date]scheduleSvc.SeriesDeviationKind{}
	for _, d := range deviations.Preserved {
		preservedDates[d.Date] = d.Kind
	}
	assert.Equal(t, scheduleSvc.SeriesDeviationCancelled, preservedDates[cancelDate])
	assert.Equal(t, scheduleSvc.SeriesDeviationRemoved, preservedDates[deleteDate])

	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.UpdateSeries(ctx, scheduleSvc.UpdateSeriesInput{
			SeriesID:     series.ID,
			StartTime:    seriesClock(t, "13:00"),
			EndTime:      seriesClock(t, "15:00"),
			BreakMinutes: 0,
			ActorStaffID: env.staff.ID,
		})
		return err
	})

	// The single-day time change is gone: that day follows the rule again.
	edited := env.shiftsInRange(t, editDate, editDate)
	require.Len(t, edited, 1)
	assert.Equal(t, "13:00", timezone.WallClock(edited[0].StartTime).Format("15:04"))
	assert.False(t, edited[0].Detached)

	// The cancellation survives — dropping it would resurrect the shift and
	// strand any replacement covering its gap.
	cancelled := env.shiftsInRange(t, cancelDate, cancelDate)
	require.Len(t, cancelled, 1)
	assert.True(t, cancelled[0].Cancelled)
	assert.Equal(t, "09:00", timezone.WallClock(cancelled[0].StartTime).Format("15:04"))

	// The deliberately deleted day stays deleted.
	assert.Empty(t, env.shiftsInRange(t, deleteDate, deleteDate))

	// Days that already happened keep their original window.
	past := env.shiftsInRange(t, periodStart, today)
	for _, row := range past {
		assert.Equal(t, "09:00", timezone.WallClock(row.StartTime).Format("15:04"),
			"past shift on %s must stay untouched", row.Date)
	}
}

func TestStaffShiftSeries_UpdateShortensValidityAndDropsLaterShifts(t *testing.T) {
	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-1), today.AddDays(20), 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-1), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	// valid_until is exclusive: the series ends after today+5.
	newEnd := today.AddDays(6)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.UpdateSeries(ctx, scheduleSvc.UpdateSeriesInput{
			SeriesID:      series.ID,
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
	assert.Empty(t, env.shiftsInRange(t, newEnd, today.AddDays(20)))
}
