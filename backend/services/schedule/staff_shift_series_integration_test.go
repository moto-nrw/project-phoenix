package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type seriesTestEnv struct {
	db     *bun.DB
	scope  testpkg.TenantScope
	staff  *usersModels.Staff
	repos  *repositories.Factory
	series scheduleSvc.StaffShiftSeriesService
	shifts scheduleSvc.StaffShiftService
}

func setupSeriesTest(t *testing.T) *seriesTestEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	scope := testpkg.NewTenantScope(t, db)
	staff := testpkg.CreateTestStaffForTenant(t, db, scope.TenantID, "Serie", fmt.Sprintf("Dienstplan-%d", scope.TenantID))
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	env := &seriesTestEnv{
		db:     db,
		scope:  scope,
		staff:  staff,
		repos:  repoFactory,
		series: serviceFactory.StaffShiftSeries,
		shifts: serviceFactory.StaffShifts,
	}
	return env
}

// createPeriod inserts a calendar period spanning [start, end] for the test
// tenant. cycleLength > 1 sets a week A/B cycle anchored at anchor.
func (e *seriesTestEnv) createPeriod(t *testing.T, start, end timezone.Date, cycleLength int, anchor *timezone.Date) int64 {
	t.Helper()
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Serie-Periode-%d-%d", e.scope.TenantID, time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeCustom,
		StartDate:       start,
		EndDate:         end,
		WeekCycleLength: cycleLength,
		WeekCycleAnchor: anchor,
		IsActive:        true,
	}
	period.SetTenantID(e.scope.TenantID)
	require.NoError(t, e.repos.CalendarPeriod.Create(e.scope.Context(), period))
	return period.ID
}

// inTx runs fn inside a tenant transaction, the same execution context the
// HTTP handlers provide (the advisory lock requires a transaction).
func (e *seriesTestEnv) inTx(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, tenant.WithTenantTx(context.Background(), e.db, e.scope.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	}))
}

func (e *seriesTestEnv) inTxExpectErr(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithTenantTx(context.Background(), e.db, e.scope.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	})
}

func seriesClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

func allWeekdays() []int16 { return []int16{1, 2, 3, 4, 5, 6, 7} }

func (e *seriesTestEnv) buildSeries(t *testing.T, periodID int64, validFrom timezone.Date, validUntil *timezone.Date, weekPattern int) *scheduleModels.StaffShiftSeries {
	t.Helper()
	series := &scheduleModels.StaffShiftSeries{
		StaffID:          e.staff.ID,
		Weekdays:         allWeekdays(),
		StartTime:        seriesClock(t, "09:00"),
		EndTime:          seriesClock(t, "10:00"),
		BreakMinutes:     0,
		CalendarPeriodID: periodID,
		WeekPattern:      weekPattern,
		ValidFrom:        validFrom,
		CreatedBy:        e.staff.ID,
		ValidUntil:       validUntil,
	}
	series.SetTenantID(e.scope.TenantID)
	return series
}

func (e *seriesTestEnv) shiftsInRange(t *testing.T, from, to timezone.Date) []*scheduleModels.StaffShift {
	t.Helper()
	rows, err := e.repos.StaffShift.FindByStaffAndDateRange(e.scope.Context(), e.staff.ID, from, to)
	require.NoError(t, err)
	return rows
}

func TestStaffShiftSeries_CreateMaterializesFromTomorrow(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodStart := today.AddDays(-7)
	periodEnd := today.AddDays(20)
	periodID := env.createPeriod(t, periodStart, periodEnd, 1, nil)

	series := env.buildSeries(t, periodID, periodStart, nil, scheduleModels.WeekPatternEvery)
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.CreateSeries(ctx, series)
		return err
	})

	// valid_from lies in the past; materialization must start tomorrow and
	// never touch today or earlier (#1837 AC / past boundary).
	tomorrow := today.AddDays(1)
	expected := tomorrow.DaysUntil(periodEnd) + 1
	assert.Equal(t, expected, result.Created)
	assert.Empty(t, result.SkippedDates)

	rows := env.shiftsInRange(t, periodStart, periodEnd)
	require.Len(t, rows, expected)
	for _, row := range rows {
		assert.True(t, row.Date.After(today), "no materialized row may lie on or before today: %s", row.Date)
		require.NotNil(t, row.SeriesID)
		assert.Equal(t, result.Series.ID, *row.SeriesID)
		assert.False(t, row.Detached)
		assert.Equal(t, "09:00", timezone.NormalizeWallClock(row.StartTime).Format("15:04"))
	}
}

func TestStaffShiftSeries_WeekPatternARespectsCycle(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	// Anchor on the Monday of the current week so week parity is stable.
	isoToday := (int(today.Weekday()) + 6) % 7 // 0 = Monday
	anchor := today.AddDays(-isoToday)
	periodStart := anchor
	periodEnd := today.AddDays(28)
	periodID := env.createPeriod(t, periodStart, periodEnd, 2, &anchor)

	series := env.buildSeries(t, periodID, periodStart, nil, scheduleModels.WeekPatternA)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	rows := env.shiftsInRange(t, periodStart, periodEnd)
	require.NotEmpty(t, rows)
	for _, row := range rows {
		weeks := anchor.DaysUntil(row.Date) / 7
		assert.Equal(t, 0, weeks%2, "week A series materialized into a week-B week: %s", row.Date)
	}
}

func TestStaffShiftSeries_WeekPatternRequiresCycle(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(20), 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternA)
	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
}

func TestStaffShiftSeries_CreateRejectsBadReferences(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(20), 1, nil)

	t.Run("unknown calendar period", func(t *testing.T) {
		series := env.buildSeries(t, periodID+999999, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
		err := env.inTxExpectErr(t, func(ctx context.Context) error {
			_, err := env.series.CreateSeries(ctx, series)
			return err
		})
		require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	})

	t.Run("valid_from outside period", func(t *testing.T) {
		series := env.buildSeries(t, periodID, today.AddDays(40), nil, scheduleModels.WeekPatternEvery)
		err := env.inTxExpectErr(t, func(ctx context.Context) error {
			_, err := env.series.CreateSeries(ctx, series)
			return err
		})
		require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	})

	t.Run("unknown staff", func(t *testing.T) {
		series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
		series.StaffID = series.StaffID + 999999
		err := env.inTxExpectErr(t, func(ctx context.Context) error {
			_, err := env.series.CreateSeries(ctx, series)
			return err
		})
		require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)
	})
}

func TestStaffShiftSeries_SplitOutsideSegmentRejected(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(20), 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})
	// Cap the series first, then try to split at/after the cap: the
	// effective date no longer lies inside the segment.
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.EndSeries(ctx, series.ID, today.AddDays(3))
		return err
	})
	err := env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: today.AddDays(5),
			StartTime:     seriesClock(t, "12:00"),
			EndTime:       seriesClock(t, "13:00"),
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesInvalid)

	// Unknown series id maps to not-found.
	err = env.inTxExpectErr(t, func(ctx context.Context) error {
		_, err := env.series.EndSeries(ctx, series.ID+999999, today.AddDays(3))
		return err
	})
	require.ErrorIs(t, err, scheduleSvc.ErrSeriesNotFound)
}

func TestStaffShiftSeries_CollisionSkipsAndReports(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)
	periodEnd := today.AddDays(10)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	standalone := &scheduleModels.StaffShift{
		StaffID:   env.staff.ID,
		Date:      tomorrow,
		StartTime: seriesClock(t, "08:00"),
		EndTime:   seriesClock(t, "16:00"),
		CreatedBy: env.staff.ID,
	}
	standalone.SetTenantID(env.scope.TenantID)
	require.NoError(t, env.repos.StaffShift.Create(env.scope.Context(), standalone))

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.CreateSeries(ctx, series)
		return err
	})

	require.Len(t, result.SkippedDates, 1)
	assert.Equal(t, tomorrow, result.SkippedDates[0])
	assert.Equal(t, tomorrow.AddDays(1).DaysUntil(periodEnd)+1, result.Created)

	// The pre-existing standalone shift stays untouched and stays the only
	// row on the collision day.
	rows := env.shiftsInRange(t, tomorrow, tomorrow)
	require.Len(t, rows, 1)
	assert.Equal(t, standalone.ID, rows[0].ID)
	assert.Nil(t, rows[0].SeriesID)
}

func TestStaffShiftSeries_EditDetachesAndDeleteRecordsException(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(14), 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	editDate := today.AddDays(3)
	deleteDate := today.AddDays(4)
	editRow := env.shiftsInRange(t, editDate, editDate)[0]
	deleteRow := env.shiftsInRange(t, deleteDate, deleteDate)[0]

	// "Nur diese Woche" edit through the EXISTING single-shift path detaches
	// the row automatically.
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
	detached := env.shiftsInRange(t, editDate, editDate)[0]
	assert.True(t, detached.Detached)
	require.NotNil(t, detached.SeriesID)
	assert.Equal(t, series.ID, *detached.SeriesID)
	require.NotNil(t, detached.SeriesOccurrenceDate)
	assert.Equal(t, editDate, *detached.SeriesOccurrenceDate)

	// Deleting a series row through the EXISTING delete path records a series
	// exception so re-plans never regenerate the occurrence.
	env.inTx(t, func(ctx context.Context) error {
		return env.shifts.DeleteShift(ctx, deleteRow.ID)
	})
	assert.Empty(t, env.shiftsInRange(t, deleteDate, deleteDate))
	exceptionDates, err := env.repos.StaffShiftSeriesException.FindDatesBySeriesID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.Len(t, exceptionDates, 1)
	assert.Equal(t, deleteDate, exceptionDates[0])
}

func TestStaffShiftSeries_SplitTodayUpdatesOccurrenceAndReplansTomorrow(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(14), 1, nil)
	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	seriesID := series.ID
	occurrenceDate := today
	todayShift := &scheduleModels.StaffShift{
		StaffID:              env.staff.ID,
		Date:                 today,
		StartTime:            series.StartTime,
		EndTime:              series.EndTime,
		BreakMinutes:         series.BreakMinutes,
		SeriesID:             &seriesID,
		SeriesOccurrenceDate: &occurrenceDate,
		CreatedBy:            env.staff.ID,
	}
	todayShift.SetTenantID(env.scope.TenantID)
	env.inTx(t, func(ctx context.Context) error {
		return env.repos.StaffShift.Create(ctx, todayShift)
	})

	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:          series.ID,
			EffectiveDate:     today,
			OccurrenceShiftID: todayShift.ID,
			StartTime:         seriesClock(t, "09:00"),
			EndTime:           seriesClock(t, "15:00"),
			BreakMinutes:      30,
			ActorStaffID:      env.staff.ID,
		})
		return err
	})

	todayRows := env.shiftsInRange(t, today, today)
	require.Len(t, todayRows, 1)
	assert.Equal(t, todayShift.ID, todayRows[0].ID, "today must retain its concrete identity")
	assert.True(t, todayRows[0].Detached)
	assert.Equal(t, "09:00", timezone.NormalizeWallClock(todayRows[0].StartTime).Format("15:04"))
	assert.Equal(t, "15:00", timezone.NormalizeWallClock(todayRows[0].EndTime).Format("15:04"))
	assert.Equal(t, 30, todayRows[0].BreakMinutes)
	require.NotNil(t, todayRows[0].SeriesID)
	assert.Equal(t, result.Series.ID, *todayRows[0].SeriesID,
		"the retained occurrence must open the active successor for another permanent edit")
	require.NotNil(t, result.Series.RetainedOccurrenceShiftID)
	assert.Equal(t, todayShift.ID, *result.Series.RetainedOccurrenceShiftID)

	tomorrowRows := env.shiftsInRange(t, today.AddDays(1), today.AddDays(1))
	require.Len(t, tomorrowRows, 1)
	assert.Equal(t, result.Series.ID, *tomorrowRows[0].SeriesID)
	assert.False(t, tomorrowRows[0].Detached)
	assert.Equal(t, "09:00", timezone.NormalizeWallClock(tomorrowRows[0].StartTime).Format("15:04"))

	var secondResult *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		secondResult, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:          result.Series.ID,
			EffectiveDate:     today,
			OccurrenceShiftID: todayShift.ID,
			StartTime:         seriesClock(t, "10:00"),
			EndTime:           seriesClock(t, "16:00"),
			BreakMinutes:      30,
			ActorStaffID:      env.staff.ID,
		})
		return err
	})
	require.NotNil(t, secondResult)
	todayRows = env.shiftsInRange(t, today, today)
	require.Len(t, todayRows, 1)
	require.NotNil(t, todayRows[0].SeriesID)
	assert.Equal(t, secondResult.Series.ID, *todayRows[0].SeriesID)
	assert.Equal(t, "10:00", timezone.NormalizeWallClock(todayRows[0].StartTime).Format("15:04"))
}

func TestStaffShiftSeries_MoveConsumesOriginalDateBeforeRematerialization(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	originalDate := today.AddDays(1)
	for originalDate.Weekday() != time.Monday {
		originalDate = originalDate.AddDays(1)
	}
	periodID := env.createPeriod(t, today, originalDate.AddDays(21), 1, nil)
	series := env.buildSeries(t, periodID, today, nil, scheduleModels.WeekPatternEvery)
	series.Weekdays = []int16{1}
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	originalRows := env.shiftsInRange(t, originalDate, originalDate)
	require.Len(t, originalRows, 1)
	original := originalRows[0]
	movedDate := originalDate.AddDays(1)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.shifts.MoveShift(ctx, scheduleSvc.MoveShiftInput{
			ShiftID:       original.ID,
			SourceStaffID: original.StaffID,
			TargetStaffID: original.StaffID,
			Date:          movedDate,
			StartTime:     original.StartTime,
			EndTime:       original.EndTime,
			BreakMinutes:  original.BreakMinutes,
			ShiftTypeID:   original.ShiftTypeID,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	assert.Empty(t, env.shiftsInRange(t, originalDate, originalDate))
	movedRows := env.shiftsInRange(t, movedDate, movedDate)
	require.Len(t, movedRows, 1)
	assert.Equal(t, original.ID, movedRows[0].ID)
	assert.True(t, movedRows[0].Detached)

	// Splitting re-plans the successor segment. The exception written for the
	// vacated Monday must prevent that original occurrence from coming back.
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: originalDate,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ShiftTypeID:   series.ShiftTypeID,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})

	assert.Empty(t, env.shiftsInRange(t, originalDate, originalDate),
		"re-plan must not recreate the vacated series occurrence")
	after := env.shiftsInRange(t, movedDate, movedDate)
	require.Len(t, after, 1)
	assert.Equal(t, original.ID, after[0].ID)
}

func TestStaffShiftSeries_RepeatedMoveKeepsOriginalOccurrenceIdentity(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	originalMonday := today.AddDays(1)
	for originalMonday.Weekday() != time.Monday {
		originalMonday = originalMonday.AddDays(1)
	}
	periodID := env.createPeriod(t, today, originalMonday.AddDays(21), 1, nil)
	series := env.buildSeries(t, periodID, today, nil, scheduleModels.WeekPatternEvery)
	series.Weekdays = []int16{1, 2}
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	mondayRows := env.shiftsInRange(t, originalMonday, originalMonday)
	require.Len(t, mondayRows, 1)
	moved := mondayRows[0]
	tuesday := originalMonday.AddDays(1)
	wednesday := originalMonday.AddDays(2)

	// Move Monday's occurrence onto a second, non-overlapping Tuesday slot.
	// The genuine Tuesday occurrence must remain independently owned by Tuesday.
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.shifts.MoveShift(ctx, scheduleSvc.MoveShiftInput{
			ShiftID:       moved.ID,
			SourceStaffID: moved.StaffID,
			TargetStaffID: moved.StaffID,
			Date:          tuesday,
			StartTime:     seriesClock(t, "11:00"),
			EndTime:       seriesClock(t, "12:00"),
			BreakMinutes:  moved.BreakMinutes,
			ShiftTypeID:   moved.ShiftTypeID,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	tuesdayRows := env.shiftsInRange(t, tuesday, tuesday)
	require.Len(t, tuesdayRows, 2)

	var movedOnTuesday *scheduleModels.StaffShift
	for _, row := range tuesdayRows {
		if row.ID == moved.ID {
			movedOnTuesday = row
			break
		}
	}
	require.NotNil(t, movedOnTuesday)
	require.NotNil(t, movedOnTuesday.SeriesOccurrenceDate)
	assert.Equal(t, originalMonday, *movedOnTuesday.SeriesOccurrenceDate)

	// Moving the same detached row again must record the same Monday exception
	// idempotently, never an exception for the genuine Tuesday occurrence.
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.shifts.MoveShift(ctx, scheduleSvc.MoveShiftInput{
			ShiftID:       movedOnTuesday.ID,
			SourceStaffID: movedOnTuesday.StaffID,
			TargetStaffID: movedOnTuesday.StaffID,
			Date:          wednesday,
			StartTime:     movedOnTuesday.StartTime,
			EndTime:       movedOnTuesday.EndTime,
			BreakMinutes:  movedOnTuesday.BreakMinutes,
			ShiftTypeID:   movedOnTuesday.ShiftTypeID,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	exceptionDates, err := env.repos.StaffShiftSeriesException.FindDatesBySeriesID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.Equal(t, []timezone.Date{originalMonday}, exceptionDates)

	// A split forces re-materialization. Monday stays consumed, Tuesday is
	// regenerated, and the moved row survives on Wednesday as Monday's deviation.
	var splitResult *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		splitResult, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: originalMonday,
			StartTime:     series.StartTime,
			EndTime:       series.EndTime,
			BreakMinutes:  series.BreakMinutes,
			ShiftTypeID:   series.ShiftTypeID,
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	require.NotNil(t, splitResult)
	assert.Empty(t, env.shiftsInRange(t, originalMonday, originalMonday))
	require.Len(t, env.shiftsInRange(t, tuesday, tuesday), 1,
		"the genuine Tuesday occurrence must survive re-materialization")
	wednesdayRows := env.shiftsInRange(t, wednesday, wednesday)
	require.Len(t, wednesdayRows, 1)
	assert.Equal(t, moved.ID, wednesdayRows[0].ID)
	require.NotNil(t, wednesdayRows[0].SeriesOccurrenceDate)
	assert.Equal(t, originalMonday, *wednesdayRows[0].SeriesOccurrenceDate)
}

func TestStaffShiftSeries_SplitPreservesDeviationsOnSuccessor(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(14)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	// Deviations after the future split point: one detached edit, one removed
	// occurrence.
	editDate := today.AddDays(5)
	deleteDate := today.AddDays(6)
	editRow := env.shiftsInRange(t, editDate, editDate)[0]
	deleteRow := env.shiftsInRange(t, deleteDate, deleteDate)[0]
	env.inTx(t, func(ctx context.Context) error {
		updated := &scheduleModels.StaffShift{
			StaffID:   editRow.StaffID,
			Date:      editRow.Date,
			StartTime: seriesClock(t, "14:00"),
			EndTime:   seriesClock(t, "15:00"),
			CreatedBy: editRow.CreatedBy,
		}
		updated.ID = editRow.ID
		_, err := env.shifts.UpdateShift(ctx, updated)
		return err
	})
	env.inTx(t, func(ctx context.Context) error {
		return env.shifts.DeleteShift(ctx, deleteRow.ID)
	})

	effective := today.AddDays(3)
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		// Weekdays and WeekPattern stay nil: the successor must inherit them
		// from the predecessor (the UI edits only times here).
		result, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: effective,
			StartTime:     seriesClock(t, "12:00"),
			EndTime:       seriesClock(t, "13:00"),
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	successorID := result.Series.ID
	require.NotEqual(t, series.ID, successorID)

	// Old series is capped at the effective date (exclusive).
	oldSeries, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.NotNil(t, oldSeries.ValidUntil)
	assert.Equal(t, effective, *oldSeries.ValidUntil)
	// Successor carries the lineage root.
	successor, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), successorID)
	require.NoError(t, err)
	require.NotNil(t, successor.SeriesRootID)
	assert.Equal(t, series.ID, *successor.SeriesRootID)

	// Rows before the effective date keep the old series and old time.
	before := env.shiftsInRange(t, today.AddDays(1), effective.AddDays(-1))
	require.NotEmpty(t, before)
	for _, row := range before {
		require.NotNil(t, row.SeriesID)
		assert.Equal(t, series.ID, *row.SeriesID)
		assert.Equal(t, "09:00", timezone.NormalizeWallClock(row.StartTime).Format("15:04"))
	}

	// The detached edit survives the split, re-pointed to the successor.
	detachedRows := env.shiftsInRange(t, editDate, editDate)
	require.Len(t, detachedRows, 1)
	assert.True(t, detachedRows[0].Detached)
	require.NotNil(t, detachedRows[0].SeriesID)
	assert.Equal(t, successorID, *detachedRows[0].SeriesID)
	assert.Equal(t, "14:00", timezone.NormalizeWallClock(detachedRows[0].StartTime).Format("15:04"))

	// The removed occurrence stays removed: the exception moved to the
	// successor and the split did not regenerate the date.
	assert.Empty(t, env.shiftsInRange(t, deleteDate, deleteDate))
	successorExceptions, err := env.repos.StaffShiftSeriesException.FindDatesBySeriesID(env.scope.Context(), successorID)
	require.NoError(t, err)
	require.Len(t, successorExceptions, 1)
	assert.Equal(t, deleteDate, successorExceptions[0])

	// Regenerated rows from the effective date carry the successor's time.
	after := env.shiftsInRange(t, effective, periodEnd)
	require.NotEmpty(t, after)
	for _, row := range after {
		if row.Date == editDate {
			continue
		}
		require.NotNil(t, row.SeriesID)
		assert.Equal(t, successorID, *row.SeriesID, "row on %s must belong to the successor", row.Date)
		assert.Equal(t, "12:00", timezone.NormalizeWallClock(row.StartTime).Format("15:04"))
	}
}

func TestStaffShiftSeries_EndSeriesKeepsDetachedAndPast(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(14)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	detachDate := today.AddDays(8)
	detachRow := env.shiftsInRange(t, detachDate, detachDate)[0]
	env.inTx(t, func(ctx context.Context) error {
		updated := &scheduleModels.StaffShift{
			StaffID:   detachRow.StaffID,
			Date:      detachRow.Date,
			StartTime: seriesClock(t, "13:00"),
			EndTime:   seriesClock(t, "14:00"),
			CreatedBy: detachRow.CreatedBy,
		}
		updated.ID = detachRow.ID
		_, err := env.shifts.UpdateShift(ctx, updated)
		return err
	})

	endFrom := today.AddDays(5)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.EndSeries(ctx, series.ID, endFrom)
		return err
	})

	// Rows before the end date stay; regenerable rows from the end date are
	// gone; the detached row survives.
	kept := env.shiftsInRange(t, today.AddDays(1), endFrom.AddDays(-1))
	assert.Len(t, kept, today.AddDays(1).DaysUntil(endFrom.AddDays(-1))+1)
	after := env.shiftsInRange(t, endFrom, periodEnd)
	require.Len(t, after, 1)
	assert.Equal(t, detachRow.ID, after[0].ID)
	assert.True(t, after[0].Detached)

	ended, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.NotNil(t, ended.ValidUntil)
	assert.Equal(t, endFrom, *ended.ValidUntil)
}

// The weekly overview endpoint feeds the Dienstplan grid. Series-backed
// shifts must ride the existing shift rows (series_id/detached are plain
// columns), adding ZERO reads. This test is additive; the 6/13/9 assertions
// in staff_schedule_overview_integration_test.go stay untouched.
func TestStaffScheduleOverview_SeriesFieldsRideExistingReads(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(14), 1, nil)

	series := env.buildSeries(t, periodID, today.AddDays(-7), nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	queryCounter := &overviewQueryCounter{}
	countedDB := env.db.WithQueryHook(queryCounter)
	repos := repositories.NewFactory(countedDB)
	service := scheduleSvc.NewStaffScheduleOverviewService(scheduleSvc.StaffScheduleOverviewDependencies{
		Shifts: repos.StaffShift, ShiftWeeks: repos.StaffShift,
		Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
		Rooms: repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	weekFrom := today.AddDays(1)
	overview, err := service.GetOverview(env.scope.Context(), weekFrom, weekFrom.AddDays(6))
	require.NoError(t, err)
	// A shift-only week (no timetable assignments) resolves in 5 fixed
	// batched reads — below the 7 of a fully populated active week, because
	// the assignment-dependent reads early-return. The point pinned here:
	// series_id/detached ride the existing shift rows and add ZERO reads.
	assert.Equal(t, int64(5), queryCounter.count.Load(),
		"series-backed shifts must not add any overview read")

	require.NotEmpty(t, overview.Shifts)
	for _, shift := range overview.Shifts {
		require.NotNil(t, shift.SeriesID, "series-backed shift must expose its series_id in the overview")
		assert.Equal(t, series.ID, *shift.SeriesID)
		assert.False(t, shift.Detached)
	}
	assert.True(t, overview.DienstplanInUse)
}

// Splitting a series at its very first occurrence must not violate the
// validity constraint: the predecessor collapses to an empty segment
// (valid_until = valid_from) and every occurrence moves to the successor.
func TestStaffShiftSeries_SplitAtFirstOccurrence(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(20)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	validFrom := today.AddDays(7)
	series := env.buildSeries(t, periodID, validFrom, nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	// "Ab jetzt dauerhaft" on the FIRST shift of the series: effective date
	// equals valid_from.
	var result *scheduleSvc.SeriesResult
	env.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = env.series.SplitSeries(ctx, scheduleSvc.SplitSeriesInput{
			SeriesID:      series.ID,
			EffectiveDate: validFrom,
			StartTime:     seriesClock(t, "12:00"),
			EndTime:       seriesClock(t, "13:00"),
			ActorStaffID:  env.staff.ID,
		})
		return err
	})
	successorID := result.Series.ID
	require.NotEqual(t, series.ID, successorID)

	// The predecessor is an empty segment: valid_until clamped to valid_from.
	oldSeries, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.NotNil(t, oldSeries.ValidUntil)
	assert.Equal(t, validFrom, *oldSeries.ValidUntil)

	// Every occurrence belongs to the successor with the new time.
	rows := env.shiftsInRange(t, validFrom, periodEnd)
	require.Len(t, rows, validFrom.DaysUntil(periodEnd)+1)
	for _, row := range rows {
		require.NotNil(t, row.SeriesID)
		assert.Equal(t, successorID, *row.SeriesID)
		assert.Equal(t, "12:00", timezone.NormalizeWallClock(row.StartTime).Format("15:04"))
	}
}

// Ending a series at its very first occurrence must clamp valid_until to
// valid_from (empty segment) instead of violating the validity constraint.
func TestStaffShiftSeries_EndAtFirstOccurrence(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodEnd := today.AddDays(20)
	periodID := env.createPeriod(t, today.AddDays(-7), periodEnd, 1, nil)

	validFrom := today.AddDays(7)
	series := env.buildSeries(t, periodID, validFrom, nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.EndSeries(ctx, series.ID, validFrom)
		return err
	})

	ended, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.NotNil(t, ended.ValidUntil)
	assert.Equal(t, validFrom, *ended.ValidUntil)
	assert.Empty(t, env.shiftsInRange(t, validFrom, periodEnd))
}

// Offboarding caps every series of the staff member at today. A series that
// only starts in the future must clamp to its own valid_from (empty segment),
// not to today — otherwise the cap violates the validity constraint and the
// whole offboarding aborts.
func TestStaffShiftSeries_CapAllByStaffIDClampsFutureSeries(t *testing.T) {
	t.Parallel()

	env := setupSeriesTest(t)
	today := timezone.TodayDate()
	periodID := env.createPeriod(t, today.AddDays(-7), today.AddDays(20), 1, nil)

	validFrom := today.AddDays(7)
	series := env.buildSeries(t, periodID, validFrom, nil, scheduleModels.WeekPatternEvery)
	env.inTx(t, func(ctx context.Context) error {
		_, err := env.series.CreateSeries(ctx, series)
		return err
	})

	capped, err := env.repos.StaffShiftSeries.CapAllByStaffID(env.scope.Context(), env.staff.ID, today)
	require.NoError(t, err)
	assert.Equal(t, int64(1), capped)

	reloaded, err := env.repos.StaffShiftSeries.FindByID(env.scope.Context(), series.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.ValidUntil)
	assert.Equal(t, validFrom, *reloaded.ValidUntil,
		"a future-dated series must clamp to its valid_from, not to today")
}
