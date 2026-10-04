package planning_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	planning "github.com/moto-nrw/project-phoenix/modules/workforce/internal/planning"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unificationDayCalendar is the School Calendar with the statutory holidays
// pinned to the Tag der Deutschen Einheit: resolving the federal state is the
// calendar owner's own concern, the closing days and Ferien periods read here
// are real rows.
type unificationDayCalendar struct {
	planning.SeriesCalendarReads
}

func setupBreakSeriesTest(t *testing.T) *seriesTestEnv {
	t.Helper()
	// The package support returns its shared pool; this file also inserts its
	// calendar rows directly, so it must initialize that pool itself.
	testpkg.SetupTestDB(t)
	return setupSeriesTest(t)
}

func (unificationDayCalendar) TenantHolidayDates(_ context.Context, from, to string) (map[string]bool, error) {
	if from <= "2026-10-03" && "2026-10-03" <= to {
		return map[string]bool{"2026-10-03": true}, nil
	}
	return map[string]bool{}, nil
}

// breakSeriesService is the series service the root composes (#3820): the
// School Calendar answers holidays, closing days and Ferien periods.
func (e *seriesTestEnv) breakSeriesService() planning.StaffShiftSeriesService {
	calendar := e.repos.SchoolCalendar()
	today := timezone.CalendarDateClock(func() time.Time {
		return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	})
	return planning.NewStaffShiftSeriesService(
		e.seriesRows, e.exceptionRows, e.shiftRows, e.repos.Staff, planning.SchoolCalendarPeriods(calendar),
		nil, workforceCompose.NewStaffShiftLock(e.db), slog.Default(), e.shifts,
		planning.WithStaffShiftSeriesToday(today),
		planning.WithStaffShiftSeriesNonWorkingDays(planning.SchoolCalendarNonWorkingDays(unificationDayCalendar{SeriesCalendarReads: calendar})),
	)
}

func (e *seriesTestEnv) createBreakSeries(t *testing.T, series *planning.StaffShiftSeries) *planning.SeriesResult {
	t.Helper()
	var result *planning.SeriesResult
	e.inTx(t, func(ctx context.Context) error {
		var err error
		result, err = e.breakSeriesService().CreateSeries(ctx, series)
		return err
	})
	return result
}

func (e *seriesTestEnv) createClosingDay(t *testing.T, start, end timezone.Date, reason string) {
	t.Helper()

	row := &schedule.ClosingDay{
		StartDate: schedule.Date(start),
		EndDate:   schedule.Date(end),
		Reason:    reason,
	}
	row.TenantID = e.scope.TenantID
	_, err := e.db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.closing_days`).
		Exec(e.scope.Context())
	require.NoError(t, err)
}

func (e *seriesTestEnv) createHolidayPeriod(t *testing.T, name string, start, end timezone.Date) *schedule.CalendarPeriod {
	t.Helper()

	row := &schedule.CalendarPeriod{
		Name:            name,
		PeriodType:      schedule.PeriodTypeHoliday,
		StartDate:       schedule.Date(start),
		EndDate:         schedule.Date(end),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	row.TenantID = e.scope.TenantID
	_, err := e.db.NewInsert().
		Model(row).
		ModelTableExpr(`schedule.calendar_periods`).
		Exec(e.scope.Context())
	require.NoError(t, err)
	return row
}

// The window 2026-09-28 … 2026-10-11 holds a closing day (Thu 1 Oct), the
// Tag der Deutschen Einheit (Sat 3 Oct) and the Herbstferien, which start on
// 5 Oct and reach past the window's end.
func TestStaffShiftSeries_SkipsSchoolBreaksUnlessIncluded(t *testing.T) {
	t.Parallel()

	periodStart, periodEnd := timezone.NewDate(2026, 9, 28), timezone.NewDate(2026, 10, 11)
	tests := []struct {
		name    string
		include bool
		want    []string
		skipped int
	}{
		{
			name:    "default leaves holidays, closing days and Ferien free",
			want:    []string{"2026-09-28", "2026-09-29", "2026-09-30", "2026-10-02", "2026-10-04"},
			skipped: 9,
		},
		{
			name: "opt-in plans Ferien and closing days but no holiday", include: true,
			want: []string{
				"2026-09-28", "2026-09-29", "2026-09-30", "2026-10-01", "2026-10-02", "2026-10-04",
				"2026-10-05", "2026-10-06", "2026-10-07", "2026-10-08", "2026-10-09", "2026-10-10", "2026-10-11",
			},
			skipped: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := setupBreakSeriesTest(t)
			closingDay := timezone.Date("2026-10-01")
			ferienStart, ferienEnd := timezone.Date("2026-10-05"), timezone.Date("2026-10-17")
			env.createClosingDay(t, closingDay, closingDay, "Pädagogischer Tag")
			env.createHolidayPeriod(t, fmt.Sprintf("Herbstferien-%d", env.scope.TenantID), ferienStart, ferienEnd)
			periodID := env.createPeriod(t, periodStart, periodEnd, 1, nil)
			series := env.buildSeries(t, periodID, periodStart, nil, planning.WeekPatternEvery)
			series.IncludeSchoolBreaks = tc.include

			result := env.createBreakSeries(t, series)

			planned := make([]string, 0, len(tc.want))
			for _, row := range env.shiftsInRange(t, periodStart, periodEnd) {
				planned = append(planned, row.Date.String())
			}
			assert.ElementsMatch(t, tc.want, planned)
			assert.Equal(t, tc.skipped, result.SkippedNonWorkingDays)

			stored, err := env.seriesRows.FindByID(env.scope.Context(), result.Series.ID)
			require.NoError(t, err)
			assert.Equal(t, tc.include, stored.IncludeSchoolBreaks, "the opt-in round-trips through the store")
		})
	}
}

// A series planned over a Ferien period exists for the holiday care, so that
// period is never a break for it.
func TestStaffShiftSeries_FerienSeriesKeepsItsOwnPeriod(t *testing.T) {
	t.Parallel()

	env := setupBreakSeriesTest(t)
	start, end := timezone.NewDate(2026, 10, 5), timezone.NewDate(2026, 10, 9)
	period := env.createHolidayPeriod(t, fmt.Sprintf("Ferienbetreuung-%d", env.scope.TenantID), start, end)
	series := env.buildSeries(t, period.ID, start, nil, planning.WeekPatternEvery)

	result := env.createBreakSeries(t, series)
	assert.Equal(t, 5, result.Created)
	assert.Zero(t, result.SkippedNonWorkingDays)
}
