package compose_test

// Concurrent tests for Workforce's SplitSeries against a sick report of the
// same person (#6583). Invariant R5: the split takes the person's
// staff-shift gate before it replaces the predecessor's segment and
// materializes the successor, and the sick report takes the same gate first
// (R1). Whichever commits first, the other waits for it, and the successor's
// shift on the sick day is cancelled with the report's stamp. The sick day is
// a Tuesday only the successor plans, so no shift row the two share
// serializes them: only the gate does. The tests run the public SplitSeries
// path, not CreateSeries.

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/require"
)

// splitSeriesAtWeekTwo adds Tuesday to the Monday series from the second
// week on and returns the successor.
func (e *sickCascadeEnv) splitSeriesAtWeekTwo(ctx context.Context, seriesID int64) (int64, error) {
	result, err := e.Planning.SplitSeries(ctx, workforce.SplitStaffShiftSeries{
		SeriesID: seriesID, EffectiveDate: stampWeekTwo.String(), Weekdays: []int{1, 2},
		StartTime: clock("08:00"), EndTime: clock("12:00"), ActorStaffID: e.admin.ID,
	})
	return result.SeriesID, err
}

// splitSickTuesday is the second week's Tuesday, which only the successor
// plans.
var splitSickTuesday = stampWeekTwo.AddDays(1)

// assertSplitSickState checks the shifts both orders must leave: the
// predecessor keeps its first Monday, the successor plans the sick Tuesday
// cancelled with the report's stamp and its healthy days planned.
func (e *sickCascadeEnv) assertSplitSickState(t *testing.T, seriesID, successorID, absenceID int64) {
	t.Helper()
	require.NotZero(t, successorID)
	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekOne), "predecessor's Monday")
	assertPlanned(t, e.seriesShift(t, successorID, stampWeekTwo), "successor's healthy Monday")
	assertSickCancelled(t, e.seriesShift(t, successorID, splitSickTuesday), absenceID, "successor's sick Tuesday")
	assertPlanned(t, e.seriesShift(t, successorID, stampWeekThree), "successor's last Monday")
}

// R5, split first: the split holds the gate with its successor's shifts
// uncommitted while a sick report for the new Tuesday is filed. The report
// waits for the gate and then cancels the successor's shift it could not
// see before.
func TestSickReport_WaitsForConcurrentSeriesSplit(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	seriesID := e.createMondaySeries(t, e.createSeriesPeriod(t), "08:00", "12:00")
	var successorID int64
	held := e.holdWrite(t, func(ctx context.Context) (err error) {
		successorID, err = e.splitSeriesAtWeekTwo(ctx, seriesID)
		return err
	})
	var absenceID int64
	report := e.startContender(t, func(ctx context.Context) (err error) {
		absenceID, err = e.createSickReport(ctx, e.subject.ID, splitSickTuesday, splitSickTuesday)
		return err
	})
	e.requireWaitsForHeld(t, held, report, "sick report")

	e.assertSplitSickState(t, seriesID, successorID, absenceID)
}

// R5, report first: the report holds the gate with its absence uncommitted
// while the series is split. The split waits for the gate, then reads the
// committed report and materializes the successor's sick Tuesday cancelled.
func TestSickReport_SeriesSplitWaitsForConcurrentReport(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	seriesID := e.createMondaySeries(t, e.createSeriesPeriod(t), "08:00", "12:00")
	var absenceID int64
	held := e.holdWrite(t, func(ctx context.Context) (err error) {
		absenceID, err = e.createSickReport(ctx, e.subject.ID, splitSickTuesday, splitSickTuesday)
		return err
	})
	var successorID int64
	split := e.startContender(t, func(ctx context.Context) (err error) {
		successorID, err = e.splitSeriesAtWeekTwo(ctx, seriesID)
		return err
	})
	e.requireWaitsForHeld(t, held, split, "series split")

	e.assertSplitSickState(t, seriesID, successorID, absenceID)
}
