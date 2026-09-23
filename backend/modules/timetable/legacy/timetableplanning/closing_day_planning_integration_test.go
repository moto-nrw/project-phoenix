package timetableplanning_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Closing days and statutory holidays (#3594). makeScenario builds a Monday
// series in its own tenant; the factory clock stands on Monday 2026-08-24 and
// the tenant keeps the default federal state DE-NW, whose Easter Monday
// 2026-04-06 is the holiday under test.

func insertScenarioClosingDay(t *testing.T, s *scenarioSetup, start, end timezone.Date) {
	t.Helper()
	row := &scheduleModels.ClosingDay{
		StartDate: scheduleModels.Date(start),
		EndDate:   scheduleModels.Date(end),
		Reason:    "Herbstferien",
	}
	row.TenantID = s.tenantID
	_, err := s.db.NewInsert().Model(row).ModelTableExpr(`schedule.closing_days`).Exec(s.ctx)
	require.NoError(t, err)
}

func setSeriesIncludesClosingDays(t *testing.T, s *scenarioSetup, include bool) {
	t.Helper()
	_, err := s.db.NewUpdate().Table("activities.groups").
		Set("include_closing_days = ?", include).
		Where("id = ?", s.template.ID).Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
}

func setInstanceStatus(t *testing.T, s *scenarioSetup, instanceID int64, status string) {
	t.Helper()
	_, err := s.db.NewUpdate().Table("schedule.activity_instances").
		Set("status = ?", status).
		Where("id = ?", instanceID).Where("tenant_id = ?", s.tenantID).
		Exec(s.ctx)
	require.NoError(t, err)
}

func countCancelledExceptions(t *testing.T, db *bun.DB, s *scenarioSetup, date timezone.Date) int {
	t.Helper()
	count, err := db.NewSelect().Table("schedule.activity_exceptions").
		Where("activity_group_id = ?", s.template.ID).
		Where("exception_date = ?", date).
		Where("exception_type = ?", scheduleModels.ActivityExceptionCancelled).
		Where("tenant_id = ?", s.tenantID).
		Count(s.ctx)
	require.NoError(t, err)
	return count
}

func TestMaterializeForTenant_SkipsHolidaysAndClosingDays(t *testing.T) {
	t.Parallel()

	easterMonday := timezone.NewDate(2026, time.April, 6)
	closingMonday := timezone.NewDate(2026, time.April, 13)
	regularMonday := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, regularMonday)
	insertScenarioClosingDay(t, s, closingMonday, closingMonday.AddDays(4))

	result, err := s.factory.Materialization.MaterializeForTenant(s.ctx, easterMonday, regularMonday, timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)

	assert.Equal(t, 1, result.InstancesCreated, "only the regular Monday is planned")
	assert.Equal(t, 1, result.CandidatesSkippedHoliday)
	assert.Equal(t, 1, result.CandidatesSkippedClosingDay)
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, easterMonday))
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, closingMonday))
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, regularMonday), 1)
}

func TestMaterializeForTenant_SeriesIncludingClosingDaysStillSkipsHolidays(t *testing.T) {
	t.Parallel()

	easterMonday := timezone.NewDate(2026, time.April, 6)
	closingMonday := timezone.NewDate(2026, time.April, 13)
	regularMonday := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, regularMonday)
	// A holiday inside the closure: the holiday wins even for holiday care.
	insertScenarioClosingDay(t, s, easterMonday, closingMonday)
	setSeriesIncludesClosingDays(t, s, true)

	result, err := s.factory.Materialization.MaterializeForTenant(s.ctx, easterMonday, regularMonday, timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)

	assert.Equal(t, 2, result.InstancesCreated, "closing day and regular Monday are planned")
	assert.Equal(t, 1, result.CandidatesSkippedHoliday)
	assert.Zero(t, result.CandidatesSkippedClosingDay)
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, easterMonday))
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, closingMonday), 1)
}

// The lost-edit probe replays the materializer's rules: a day the series skips
// expects nothing, a closing day of a series that includes closing days
// expects its occurrence. Neither reports an untouched series as edited.
func TestDetectEditedInWindow_FollowsClosingDaySkip(t *testing.T) {
	t.Parallel()

	closingMonday := timezone.NewDate(2026, time.April, 13)
	regularMonday := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, regularMonday)
	insertScenarioClosingDay(t, s, closingMonday, closingMonday)

	detect := func() []timetableplanning.EditedOccurrence {
		t.Helper()
		var edited []timetableplanning.EditedOccurrence
		err := tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var err error
			edited, err = s.factory.Materialization.DetectEditedInWindow(txCtx, s.template.ID, closingMonday, regularMonday, false)
			return err
		})
		require.NoError(t, err)
		return edited
	}

	_, err := s.factory.Materialization.MaterializeForTenant(s.ctx, closingMonday, regularMonday, timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Empty(t, detect(), "skipping the closing day is not an edit")

	setSeriesIncludesClosingDays(t, s, true)
	_, err = s.factory.Materialization.MaterializeForTenant(s.ctx, closingMonday, regularMonday, timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)
	require.Len(t, listInstancesForDate(t, s.db, s.template.ID, closingMonday), 1)
	assert.Empty(t, detect(), "the closing-day occurrence of holiday care is expected")
}

// An occurrence planned before its day became a closing day (Wissingen: the
// school year was materialized before the autumn holidays were entered) is
// not an edit, so changing the series raises no lost-edits warning for it.
func TestDetectEditedInWindow_OccurrencePlannedBeforeClosureIsNoEdit(t *testing.T) {
	t.Parallel()

	closingMonday := timezone.NewDate(2026, time.April, 13)
	regularMonday := timezone.NewDate(2026, time.April, 20)
	s := makeScenario(t, activitiesModels.WeekdayMonday, closingMonday)

	_, err := s.factory.Materialization.MaterializeForTenant(s.ctx, closingMonday, regularMonday, timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)
	require.Len(t, listInstancesForDate(t, s.db, s.template.ID, closingMonday), 1)
	insertScenarioClosingDay(t, s, closingMonday, closingMonday)

	var edited []timetableplanning.EditedOccurrence
	err = tenant.WithTenantTx(s.ctx, s.db, s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		edited, err = s.factory.Materialization.DetectEditedInWindow(txCtx, s.template.ID, closingMonday, regularMonday, false)
		return err
	})
	require.NoError(t, err)
	assert.Empty(t, edited)
}

// bulkCancelScenario plans six Mondays around the factory's today
// (2026-08-24): one in the past, today, and four ahead.
func bulkCancelScenario(t *testing.T) (*scenarioSetup, []timezone.Date) {
	t.Helper()
	first := timezone.NewDate(2026, time.August, 17)
	s := makeScenario(t, activitiesModels.WeekdayMonday, first)
	// Tenant transactions follow the test onto its own database when it opted
	// into one (the query budget below); otherwise this is the package runtime.
	s.ctx = testpkg.WithTestTenantRuntime(t, s.ctx)
	mondays := make([]timezone.Date, 0, 6)
	for week := range 6 {
		mondays = append(mondays, first.AddDays(7*week))
	}
	result, err := s.factory.Materialization.MaterializeForTenant(s.ctx, mondays[0], mondays[5], timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)
	require.Equal(t, 6, result.InstancesCreated)
	return s, mondays
}

func TestBulkCancelPlanned_CancelsAndRemovesPlannedOccurrences(t *testing.T) {
	t.Parallel()

	s, mondays := bulkCancelScenario(t)
	past, today, active, second, third, outside := mondays[0], mondays[1], mondays[2], mondays[3], mondays[4], mondays[5]
	activeRows := listInstancesForDate(t, s.db, s.template.ID, active)
	require.Len(t, activeRows, 1)
	setInstanceStatus(t, s, activeRows[0].ID, scheduleModels.InstanceStatusActive)

	preview, err := s.factory.Instance.BulkCancelPlanned(s.ctx, past, third, timetableplanning.BulkCancelOptionsForTest(true, false), nil)
	require.NoError(t, err)
	assert.True(t, preview.DryRun)
	assert.Equal(t, 3, preview.Count, "today and the two planned Mondays ahead")
	perDay := make(map[string]int, len(preview.Days))
	days := make([]string, 0, len(preview.Days))
	for _, day := range preview.Days {
		perDay[day.Date] = day.Count
		days = append(days, day.Date)
	}
	assert.Equal(t, []string{today.String(), second.String(), third.String()}, days, "one entry per day, in date order")
	assert.Equal(t, map[string]int{today.String(): 1, second.String(): 1, third.String(): 1}, perDay)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, second), 1, "a dry run changes nothing")

	done, err := s.factory.Instance.BulkCancelPlanned(s.ctx, past, third, timetableplanning.BulkCancelOptionsForTest(false, false), nil)
	require.NoError(t, err)
	assert.False(t, done.DryRun)
	assert.Equal(t, 3, done.Count)

	for _, removed := range []timezone.Date{today, second, third} {
		assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, removed), "removed on %s", removed)
		assert.Equal(t, 1, countCancelledExceptions(t, s.db, s, removed), "slot exception on %s", removed)
	}
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, past), 1, "past occurrences stay")
	kept := listInstancesForDate(t, s.db, s.template.ID, active)
	require.Len(t, kept, 1, "running occurrences stay")
	assert.Equal(t, scheduleModels.InstanceStatusActive, kept[0].Status)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, outside), 1, "occurrences outside the range stay")

	again, err := s.factory.Materialization.MaterializeForTenant(s.ctx, mondays[0], mondays[5], timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)
	assert.Zero(t, again.InstancesCreated, "materialization does not bring cancelled occurrences back")
	assert.Equal(t, 3, again.CandidatesSkippedException)
}

func TestBulkCancelPlanned_KeepsSeriesThatIncludeClosingDays(t *testing.T) {
	t.Parallel()

	s, mondays := bulkCancelScenario(t)
	setSeriesIncludesClosingDays(t, s, true)

	result, err := s.factory.Instance.BulkCancelPlanned(s.ctx, mondays[1], mondays[5], timetableplanning.BulkCancelOptionsForTest(false, false), nil)
	require.NoError(t, err)
	assert.Zero(t, result.Count)
	assert.Empty(t, result.Days)
	assert.Equal(t, 5, result.Kept, "the dialog learns that holiday care stays")
	require.Len(t, result.KeptSeries, 1, "the dialog names the series that stay")
	assert.Equal(t, s.template.Name, result.KeptSeries[0].Name)
	assert.Equal(t, 5, result.KeptSeries[0].Count)
	for _, monday := range mondays[1:] {
		assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, monday), 1)
	}
}

func TestBulkCancelPlanned_IncludeClosingDaySeriesRemovesThem(t *testing.T) {
	t.Parallel()

	s, mondays := bulkCancelScenario(t)
	setSeriesIncludesClosingDays(t, s, true)

	preview, err := s.factory.Instance.BulkCancelPlanned(s.ctx, mondays[1], mondays[5],
		timetableplanning.BulkCancelOptionsForTest(true, true), nil)
	require.NoError(t, err)
	assert.Equal(t, 5, preview.Count, "the recount includes the holiday care series")
	assert.Zero(t, preview.Kept)
	assert.Empty(t, preview.KeptSeries)

	done, err := s.factory.Instance.BulkCancelPlanned(s.ctx, mondays[1], mondays[5],
		timetableplanning.BulkCancelOptionsForTest(false, true), nil)
	require.NoError(t, err)
	assert.Equal(t, 5, done.Count)
	for _, monday := range mondays[1:] {
		assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, monday), "removed on %s", monday)
		assert.Equal(t, 1, countCancelledExceptions(t, s.db, s, monday), "slot exception on %s", monday)
	}
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, mondays[0]), 1, "past occurrences stay")
}

func TestBulkCancelPlanned_StaysInsideTheTenant(t *testing.T) {
	t.Parallel()

	own, mondays := bulkCancelScenario(t)
	other, _ := bulkCancelScenario(t)

	result, err := own.factory.Instance.BulkCancelPlanned(own.ctx, mondays[1], mondays[5], timetableplanning.BulkCancelOptionsForTest(false, false), nil)
	require.NoError(t, err)
	assert.Equal(t, 5, result.Count)
	for _, monday := range mondays[1:] {
		assert.Empty(t, listInstancesForDate(t, own.db, own.template.ID, monday))
		assert.Len(t, listInstancesForDate(t, other.db, other.template.ID, monday), 1, "other tenant untouched on %s", monday)
	}
}

func TestBulkCancelPlanned_RejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	s, mondays := bulkCancelScenario(t)
	for name, window := range map[string][2]timezone.Date{
		"reversed": {mondays[3], mondays[1]},
		"too long": {mondays[1], mondays[1].AddDays(366)}, // 367 days

		"same days": {mondays[5].AddDays(1), mondays[5].AddDays(1)},
	} {
		_, err := s.factory.Instance.BulkCancelPlanned(s.ctx, window[0], window[1], timetableplanning.BulkCancelOptionsForTest(true, false), nil)
		if name == "same days" {
			require.NoError(t, err, name)
			continue
		}
		require.ErrorContains(t, err, "invalid bulk cancel range", name)
	}
}

// A re-plan regenerates future occurrences with the skip rule: holiday care
// keeps its closing-day occurrence, a regular series loses it.
func TestReplanWeek_FollowsTheSeriesClosingDayFlag(t *testing.T) {
	t.Parallel()

	first := timezone.NewDate(2026, time.August, 31)
	closing := first.AddDays(7)
	last := first.AddDays(14)
	s := makeScenario(t, activitiesModels.WeekdayMonday, first)
	insertScenarioClosingDay(t, s, closing, closing)
	setSeriesIncludesClosingDays(t, s, true)
	_, err := s.factory.Materialization.MaterializeForTenant(s.ctx, first, last, timetableplanning.MaterializationSourceManual)
	require.NoError(t, err)
	require.Len(t, listInstancesForDate(t, s.db, s.template.ID, closing), 1)

	templateID := s.template.ID
	_, err = s.factory.Instance.ReplanWeek(s.ctx, first, last, &templateID, nil)
	require.NoError(t, err)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, closing), 1, "holiday care stays on the closing day")

	setSeriesIncludesClosingDays(t, s, false)
	_, err = s.factory.Instance.ReplanWeek(s.ctx, first, last, &templateID, nil)
	require.NoError(t, err)
	assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, closing), "a regular series drops the closing day")
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, first), 1)
	assert.Len(t, listInstancesForDate(t, s.db, s.template.ID, last), 1)
}

// The dry run behind the confirmation dialog reads the range with a fixed
// number of statements, however many occurrences it covers (#2940).
func TestBulkCancelPlanned_DryRunQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s, mondays := bulkCancelScenario(t)
	counter := testpkg.CaptureQueries(t, s.db)

	run := func(to timezone.Date, want int) int {
		counter.Reset()
		result, err := s.factory.Instance.BulkCancelPlanned(s.ctx, mondays[1], to, timetableplanning.BulkCancelOptionsForTest(true, false), nil)
		require.NoError(t, err)
		require.Equal(t, want, result.Count)
		return counter.Total()
	}

	small := run(mondays[2], 2)
	large := run(mondays[5], 5)

	t.Logf("query budget: 2 occurrences → %d queries, 5 occurrences → %d queries", small, large)
	assert.Equal(t, small, large, "the dry run must not load per occurrence")
	testpkg.AssertQueryBudget(t, "services.schedule.bulk_cancel_dry_run", counter.Queries())
}
