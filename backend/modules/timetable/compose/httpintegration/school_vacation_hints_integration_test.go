package httpintegration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The vacation hint (#5345) over real Regeltermine: a Monday series in the
// NRW autumn vacation 2026-10-17 to 2026-10-31, materialized for the four
// Mondays 12.10. to 02.11. Two of them (19.10. and 26.10.) lie in the
// vacation.

var (
	autumnStart = calendar.NewDate(2026, time.October, 17)
	autumnEnd   = calendar.NewDate(2026, time.October, 31)
	autumnFirst = calendar.NewDate(2026, time.October, 19)
	autumnLast  = calendar.NewDate(2026, time.October, 26)
)

// nrwDays is the real School Calendar with the school's federal state fixed
// to NRW: the test calendar binds no settings.
type nrwDays struct{ schoolcalendar.Calendar }

func (d nrwDays) TenantHolidayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	return d.HolidayDates(ctx, "DE-NW", from, to)
}

// autumnVacation is the School Calendar's vacation list of an NRW school.
type autumnVacation struct {
	vacations []schoolcalendar.SchoolVacation
}

func (v *autumnVacation) TenantSchoolVacations(context.Context, string, string) (schoolcalendar.SchoolVacationList, error) {
	return schoolcalendar.SchoolVacationList{
		Region:    schoolcalendar.HolidayRegion{Code: "DE-NW", Name: "Nordrhein-Westfalen"},
		Vacations: v.vacations,
	}, nil
}

type vacationHintScenario struct {
	*scenarioSetup
	vacation *autumnVacation
	hints    func(today calendar.Date) []timetable.SchoolVacationHint
	query    func(ctx context.Context, today calendar.Date) []timetable.SchoolVacationHint
}

func makeVacationHintScenario(t *testing.T) *vacationHintScenario {
	t.Helper()
	s := makeScenario(t, activityContract.WeekdayMonday, calendar.NewDate(2026, time.October, 12))
	result, err := s.materialization.MaterializeForTenant(s.ctx, calendar.NewDate(2026, time.October, 12), calendar.NewDate(2026, time.November, 2), timetable.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 4, result.InstancesCreated)

	rows := api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db)).Timetable
	days := nrwDays{Calendar: api.MustNewSchoolCalendar(testpkg.MustDatabase(s.db))}
	vacation := &autumnVacation{vacations: []schoolcalendar.SchoolVacation{{
		Name: "Herbstferien", StartDate: autumnStart.String(), EndDate: autumnEnd.String(),
	}}}
	scenario := &vacationHintScenario{scenarioSetup: s, vacation: vacation}
	scenario.query = func(ctx context.Context, today calendar.Date) []timetable.SchoolVacationHint {
		t.Helper()
		query, err := compose.NewSchoolVacationHints(rows, days, vacation, func() string { return today.String() })
		require.NoError(t, err)
		var hints []timetable.SchoolVacationHint
		require.NoError(t, tenant.WithTenantTx(ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			hints, err = query.SchoolVacationHints(txCtx)
			return err
		}))
		return hints
	}
	scenario.hints = func(today calendar.Date) []timetable.SchoolVacationHint {
		return scenario.query(s.ctx, today)
	}
	return scenario
}

func hintCount(hints []timetable.SchoolVacationHint) int {
	if len(hints) == 0 {
		return 0
	}
	return hints[0].Count
}

func TestSchoolVacationHints_CountFilledAndEmptyRegeltermine(t *testing.T) {
	t.Parallel()
	s := makeVacationHintScenario(t)
	first := listInstancesForDate(t, s.db, s.template.ID, autumnFirst)
	require.Len(t, first, 1)
	// The first vacation Monday keeps no child: an empty Termin counts too.
	_, err := s.db.NewDelete().TableExpr("schedule.instance_students").
		Where("instance_id = ?", first[0].ID).Where("tenant_id = ?", s.tenantID).Exec(s.ctx)
	require.NoError(t, err)

	hints := s.hints(calendar.NewDate(2026, time.September, 1))

	require.Len(t, hints, 1)
	assert.Equal(t, timetable.SchoolVacationHint{
		Name: "Herbstferien", StartDate: "2026-10-17", EndDate: "2026-10-31", CancelFrom: "2026-10-17", Count: 2,
	}, hints[0], "the empty and the filled Monday in the vacation; not the Mondays around it")
}

func TestSchoolVacationHints_LeaveOutClosingDaysAndDeliberateCare(t *testing.T) {
	t.Parallel()
	s := makeVacationHintScenario(t)
	today := calendar.NewDate(2026, time.September, 1)

	insertScenarioClosingDay(t, s.scenarioSetup, autumnFirst, autumnFirst)
	assert.Equal(t, 1, hintCount(s.hints(today)), "a closing day already says the OGS is closed")

	setSeriesIncludesClosingDays(t, s.scenarioSetup, true)
	assert.Empty(t, s.hints(today), "a series with holiday care is planned on purpose")
	setSeriesIncludesClosingDays(t, s.scenarioSetup, false)

	_, err := s.db.NewUpdate().Table("schedule.calendar_periods").Set("period_type = ?", schoolcalendar.PeriodTypeHoliday).
		Where("id = ?", s.period.ID).Where("tenant_id = ?", s.tenantID).Exec(s.ctx)
	require.NoError(t, err)
	assert.Empty(t, s.hints(today), "a series in a holiday period the school entered is planned on purpose")
}

func TestSchoolVacationHints_AcknowledgedVacationAndCancelledTermine(t *testing.T) {
	t.Parallel()
	s := makeVacationHintScenario(t)
	today := calendar.NewDate(2026, time.September, 1)

	last := listInstancesForDate(t, s.db, s.template.ID, autumnLast)
	require.Len(t, last, 1)
	setInstanceStatus(t, s.scenarioSetup, last[0].ID, timetable.InstanceStatusCancelled)
	assert.Equal(t, 1, hintCount(s.hints(today)), "a cancelled Termin is no longer planned")

	s.vacation.vacations[0].Acknowledged = true
	assert.Empty(t, s.hints(today), "the school confirmed care in this vacation")
}

func TestSchoolVacationHints_EightWeeksAheadAndWhileRunning(t *testing.T) {
	t.Parallel()
	s := makeVacationHintScenario(t)

	// 2026-08-22 + 56 days is the first vacation day.
	assert.Equal(t, 2, hintCount(s.hints(calendar.NewDate(2026, time.August, 22))))
	assert.Empty(t, s.hints(calendar.NewDate(2026, time.August, 21)), "the vacation starts more than eight weeks ahead")

	running := s.hints(calendar.NewDate(2026, time.October, 20))
	require.Len(t, running, 1)
	assert.Equal(t, "2026-10-20", running[0].CancelFrom)
	assert.Equal(t, 1, running[0].Count, "only the Monday still ahead")
	assert.Empty(t, s.hints(calendar.NewDate(2026, time.November, 1)), "an ended vacation gives no hint")
}

// The hint only reads: a repeated materialization neither changes the
// Termine nor the count, and asking for the hint removes nothing.
func TestSchoolVacationHints_RepeatedMaterializationKeepsTheTermine(t *testing.T) {
	t.Parallel()
	s := makeVacationHintScenario(t)
	today := calendar.NewDate(2026, time.September, 1)
	before := listInstancesForDate(t, s.db, s.template.ID, autumnFirst)
	require.Len(t, before, 1)
	assert.Equal(t, 2, hintCount(s.hints(today)))

	result, err := s.materialization.MaterializeForTenant(s.ctx, calendar.NewDate(2026, time.October, 12), calendar.NewDate(2026, time.November, 2), timetable.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, result.InstancesCreated)

	after := listInstancesForDate(t, s.db, s.template.ID, autumnFirst)
	require.Len(t, after, 1)
	assert.Equal(t, before[0].ID, after[0].ID, "the existing Termin stays the same row")
	assert.Equal(t, 2, hintCount(s.hints(today)))
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, autumnLast), 1)
}

// TestSchoolVacationHintsQueryBudgetStaysFlat proves the hint's fixed read
// scope remains fixed when it returns more vacation windows. The instances,
// series, periods and non-working days are all loaded in batches.
func TestSchoolVacationHintsQueryBudgetStaysFlat(t *testing.T) {
	t.Parallel()
	s := makeVacationHintScenario(t)
	today := calendar.NewDate(2026, time.October, 1)
	result, err := s.materialization.MaterializeForTenant(s.ctx, calendar.NewDate(2026, time.October, 5), calendar.NewDate(2026, time.November, 23), timetable.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 4, result.InstancesCreated)

	windows := func(count int) []schoolcalendar.SchoolVacation {
		result := make([]schoolcalendar.SchoolVacation, 0, count)
		for i := range count {
			date := calendar.NewDate(2026, time.October, 5+i*7)
			result = append(result, schoolcalendar.SchoolVacation{
				Name: fmt.Sprintf("Budget-%d", i), StartDate: date.String(), EndDate: date.String(),
			})
		}
		return result
	}
	counter := testpkg.CaptureQueries(t, s.db)
	run := func(expected int) []string {
		counter.Reset()
		hints := s.query(counter.Context(s.ctx), today)
		require.Len(t, hints, expected)
		return counter.Queries()
	}

	s.vacation.vacations = windows(3)
	small := run(3)
	s.vacation.vacations = windows(8)
	large := run(8)

	require.Len(t, large, len(small), "statement count must not grow with the vacation hints")
	t.Logf("query budget: 3 school vacation hints → %d queries, 8 school vacation hints → %d queries", len(small), len(large))
	testpkg.AssertQueryBudget(t, "modules.timetable.school_vacation_hints.list", large)
}
