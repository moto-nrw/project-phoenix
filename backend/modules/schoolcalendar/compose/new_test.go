package compose

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type observationLog struct {
	mu   sync.Mutex
	seen []Observation
}

func (l *observationLog) record(observation Observation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, observation)
}

func (l *observationLog) operations() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	names := make([]string, 0, len(l.seen))
	for _, observation := range l.seen {
		names = append(names, observation.Operation)
	}
	return names
}

func buildModule(t *testing.T, db *bun.DB, observations ...func(Observation)) *schoolcalendar.Module {
	t.Helper()
	observe := func(Observation) {}
	if len(observations) > 0 {
		observe = observations[0]
	}
	module, err := New(Dependencies{DB: db, Observe: observe})
	require.NoError(t, err)
	return module
}

func otherTenantContext(t *testing.T, db *bun.DB) (context.Context, int64) {
	t.Helper()
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID), otherTenantID
}

func periodFields(name string, start, end string, active bool) schoolcalendar.CalendarPeriodFields {
	return schoolcalendar.CalendarPeriodFields{
		Name: name, PeriodType: schoolcalendar.PeriodTypeCustom, StartDate: start, EndDate: end,
		WeekCycleLength: 1, IsActive: active,
	}
}

func createPeriod(t *testing.T, ctx context.Context, module *schoolcalendar.Module, fields schoolcalendar.CalendarPeriodFields) schoolcalendar.CalendarPeriod {
	t.Helper()
	period, err := module.CreateCalendarPeriod(ctx, schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: fields})
	require.NoError(t, err)
	return period
}

func createClosingDay(t *testing.T, ctx context.Context, module *schoolcalendar.Module, start, end, reason string) schoolcalendar.ClosingDay {
	t.Helper()
	day, err := module.CreateClosingDay(ctx, schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: start, EndDate: end, Reason: reason,
	}})
	require.NoError(t, err)
	return day
}

func idsOf(periods []schoolcalendar.CalendarPeriod) []int64 {
	ids := make([]int64, 0, len(periods))
	for _, period := range periods {
		ids = append(ids, period.ID)
	}
	return ids
}

func TestNewRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := New(Dependencies{})
	require.Error(t, err)
}

// --- calendar periods ---

func TestModuleRunsTheCalendarPeriodLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	created := createPeriod(t, ctx, module, schoolcalendar.CalendarPeriodFields{
		Name: "Schuljahr 2030/2031", PeriodType: schoolcalendar.PeriodTypeSchoolYear,
		StartDate: "2030-08-01", EndDate: "2031-07-31", WeekCycleLength: 2, WeekCycleAnchor: "2030-08-05", IsActive: true,
	})
	assert.Positive(t, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.TenantID)
	assert.Equal(t, "2030-08-01", created.StartDate, "DATE columns round-trip without a timezone shift")
	assert.Equal(t, "2030-08-05", created.WeekCycleAnchor)
	assert.False(t, created.CreatedAt.IsZero())

	found, err := module.FindCalendarPeriod(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, found)

	updated, err := module.UpdateCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: created.ID, CalendarPeriodFields: schoolcalendar.CalendarPeriodFields{
		Name: "Schuljahr 2030/2031", PeriodType: schoolcalendar.PeriodTypeSemester,
		StartDate: "2030-08-01", EndDate: "2031-01-31", WeekCycleLength: 1, IsActive: false,
	}})
	require.NoError(t, err)
	assert.Equal(t, schoolcalendar.PeriodTypeSemester, updated.PeriodType)
	assert.Equal(t, "2031-01-31", updated.EndDate)
	assert.Empty(t, updated.WeekCycleAnchor, "an update clears the anchor when the cycle is gone")
	assert.False(t, updated.IsActive)
	assert.False(t, updated.UpdatedAt.Before(created.UpdatedAt))

	listed, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	require.NoError(t, module.DeleteCalendarPeriod(ctx, created.ID))
	_, err = module.FindCalendarPeriod(ctx, created.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	require.ErrorIs(t, module.DeleteCalendarPeriod(ctx, created.ID), schoolcalendar.ErrCalendarPeriodNotFound)
	_, err = module.UpdateCalendarPeriod(ctx, schoolcalendar.UpdateCalendarPeriod{ID: created.ID, CalendarPeriodFields: periodFields("Gone", "2030-08-01", "2031-07-31", false)})
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)

	assert.Equal(t, []string{
		"create_calendar_period", "find_calendar_period", "update_calendar_period", "list_calendar_periods",
		"delete_calendar_period", "find_calendar_period", "delete_calendar_period", "update_calendar_period",
	}, log.operations())
}

func TestModuleFiltersCalendarPeriodsByNameTypeActivityAndOverlap(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	active := createPeriod(t, ctx, module, periodFields("Aktiv", "2030-08-01", "2031-07-31", true))
	inactive := createPeriod(t, ctx, module, periodFields("Inaktiv", "2030-08-01", "2031-07-31", false))
	adjacent := createPeriod(t, ctx, module, periodFields("Angrenzend", "2031-08-01", "2032-07-31", true))
	holiday := createPeriod(t, ctx, module, schoolcalendar.CalendarPeriodFields{
		Name: "Herbstferien", PeriodType: schoolcalendar.PeriodTypeHoliday, StartDate: "2030-10-14", EndDate: "2030-10-25", WeekCycleLength: 1, IsActive: true,
	})

	all, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	require.NoError(t, err)
	assert.Equal(t, []int64{active.ID, inactive.ID, holiday.ID, adjacent.ID}, idsOf(all), "ordered by start date, then id")

	byName, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{Name: "Inaktiv"})
	require.NoError(t, err)
	assert.Equal(t, []int64{inactive.ID}, idsOf(byName), "the name match is exact")

	byType, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{PeriodType: schoolcalendar.PeriodTypeHoliday})
	require.NoError(t, err)
	assert.Equal(t, []int64{holiday.ID}, idsOf(byType))

	overlapping, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, OverlappingFrom: "2030-09-01", OverlappingTo: "2030-10-01",
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{active.ID}, idsOf(overlapping), "inactive periods never overlap")

	boundary, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, OverlappingFrom: "2031-07-31", OverlappingTo: "2031-07-31", ExcludeID: adjacent.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{active.ID}, idsOf(boundary), "the inclusive boundary day counts as overlap")

	excluded, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, OverlappingFrom: "2031-08-01", OverlappingTo: "2032-07-31", ExcludeID: adjacent.ID,
	})
	require.NoError(t, err)
	assert.Empty(t, excluded, "adjacent ranges do not overlap and the period itself is excluded")

	byIDs, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{IDs: []int64{adjacent.ID, active.ID}})
	require.NoError(t, err)
	assert.Equal(t, []int64{active.ID, adjacent.ID}, idsOf(byIDs))

	none, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{IDs: []int64{}})
	require.NoError(t, err)
	assert.Empty(t, none, "an empty ID set lists nothing instead of everything")

	_, err = module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{OverlappingFrom: "2030-09-01"})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidCalendarPeriod, "a half window is rejected before any SQL")
}

func TestModuleCreatesTheCalendarPeriodOnlyOncePerName(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	input := schoolcalendar.CreateCalendarPeriod{CalendarPeriodFields: schoolcalendar.CalendarPeriodFields{
		Name: "Schuljahr 2030/2031", PeriodType: schoolcalendar.PeriodTypeSchoolYear,
		StartDate: "2030-08-01", EndDate: "2031-07-31", WeekCycleLength: 1, IsActive: true,
	}}

	first, created, err := module.CreateCalendarPeriodIfAbsent(ctx, input)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Positive(t, first.ID)

	// The retry is a clean no-op: no error, no second row, and the
	// transaction stays usable.
	second, created, err := module.CreateCalendarPeriodIfAbsent(ctx, input)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Zero(t, second.ID)

	listed, err := module.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{Name: input.Name})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, first.ID, listed[0].ID)

	// A plain create of the same name reports the collision with its cause.
	_, err = module.CreateCalendarPeriod(ctx, input)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNameConflict)
	assert.Contains(t, err.Error(), "unique_calendar_period_name", "the constraint stays visible in the chain: %v", err)
	assert.Equal(t, "calendar_period_name_conflict", schoolcalendar.ErrorCode(err))

	// Uniqueness is per tenant, not global.
	otherCtx, _ := otherTenantContext(t, db)
	_, created, err = module.CreateCalendarPeriodIfAbsent(otherCtx, input)
	require.NoError(t, err)
	assert.True(t, created)
}

func TestModuleTenantIsolationHidesAnotherTenantsCalendar(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	otherCtx, otherTenantID := otherTenantContext(t, db)

	period := createPeriod(t, ctx, module, periodFields("Eigen", "2030-08-01", "2031-07-31", true))
	foreign := createPeriod(t, otherCtx, module, periodFields("Fremd", "2030-08-01", "2031-07-31", true))
	assert.Equal(t, otherTenantID, foreign.TenantID, "inserts always use the transaction tenant")
	day := createClosingDay(t, ctx, module, "2030-11-04", "2030-11-08", "Pädagogische Woche")
	foreignDay := createClosingDay(t, otherCtx, module, "2030-11-04", "2030-11-08", "Fremde Woche")

	_, err := module.FindCalendarPeriod(otherCtx, period.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	overlapping, err := module.ListCalendarPeriods(otherCtx, schoolcalendar.CalendarPeriodFilter{ActiveOnly: true, OverlappingFrom: "2030-09-01", OverlappingTo: "2030-10-01"})
	require.NoError(t, err)
	assert.Equal(t, []int64{foreign.ID}, idsOf(overlapping))
	_, err = module.UpdateCalendarPeriod(otherCtx, schoolcalendar.UpdateCalendarPeriod{ID: period.ID, CalendarPeriodFields: periodFields("Gekapert", "2030-08-01", "2031-07-31", true)})
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	require.ErrorIs(t, module.DeleteCalendarPeriod(otherCtx, period.ID), schoolcalendar.ErrCalendarPeriodNotFound)

	_, err = module.FindClosingDay(otherCtx, day.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrClosingDayNotFound)
	foreignDays, err := module.ListClosingDays(otherCtx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2030-11-01", OverlappingTo: "2030-11-30"})
	require.NoError(t, err)
	require.Len(t, foreignDays, 1)
	assert.Equal(t, foreignDay.ID, foreignDays[0].ID)
	require.ErrorIs(t, module.DeleteClosingDay(otherCtx, day.ID), schoolcalendar.ErrClosingDayNotFound)

	stillThere, err := module.FindCalendarPeriod(ctx, period.ID)
	require.NoError(t, err)
	assert.Equal(t, "Eigen", stillThere.Name)
	own, err := module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{})
	require.NoError(t, err)
	require.Len(t, own, 1)
	assert.Equal(t, day.ID, own[0].ID)
}

func TestModuleWritesRollBackWithTheOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	wantErr := errors.New("abort outer transaction")

	var periodID, dayID, dateframeID int64
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		period := createPeriod(t, txCtx, module, periodFields("Rolled back", "2030-08-01", "2031-07-31", true))
		periodID = period.ID
		day := createClosingDay(t, txCtx, module, "2030-11-04", "2030-11-08", "Rolled back")
		dayID = day.ID
		dateframe, err := module.CreateDateframe(txCtx, schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{
			StartDate: time.Date(2030, 8, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2030, 8, 31, 0, 0, 0, 0, time.UTC), Name: "Rolled back",
		}})
		require.NoError(t, err)
		dateframeID = dateframe.ID
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = module.FindCalendarPeriod(ctx, periodID)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	_, err = module.FindClosingDay(ctx, dayID)
	require.ErrorIs(t, err, schoolcalendar.ErrClosingDayNotFound)
	_, err = module.FindDateframe(ctx, dateframeID)
	require.ErrorIs(t, err, schoolcalendar.ErrDateframeNotFound)

	// The same write succeeds on retry once the caller commits.
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		period := createPeriod(t, txCtx, module, periodFields("Rolled back", "2030-08-01", "2031-07-31", true))
		periodID = period.ID
		return nil
	})
	require.NoError(t, err)
	found, err := module.FindCalendarPeriod(ctx, periodID)
	require.NoError(t, err)
	assert.Equal(t, "Rolled back", found.Name)
}

func TestModuleReadsOnTheSharedConnectionWithoutTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	period := createPeriod(t, ctx, module, periodFields("Shared", "2030-08-01", "2031-07-31", true))

	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		found, err := module.FindCalendarPeriod(txCtx, period.ID)
		require.NoError(t, err)
		assert.Equal(t, period.ID, found.ID)
		return nil
	})
	require.NoError(t, err)
}

func TestModuleObservesCalendarOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	log := &observationLog{}
	module := buildModule(t, db, log.record)
	ctx := testpkg.Ctx(t)

	_, err := module.FindCalendarPeriod(ctx, 9_223_372_036_854_775_000)
	require.ErrorIs(t, err, schoolcalendar.ErrCalendarPeriodNotFound)
	require.Len(t, log.seen, 1)
	assert.Equal(t, "find_calendar_period", log.seen[0].Operation)
	assert.Equal(t, "not_found", schoolcalendar.ErrorCode(log.seen[0].Err), "observations carry the public error")
	assert.EqualValues(t, 1, log.seen[0].Stats.Queries)
	assert.Zero(t, log.seen[0].Stats.Rows)

	createPeriod(t, ctx, module, periodFields("Observed", "2030-08-01", "2031-07-31", true))
	require.Len(t, log.seen, 2)
	assert.Equal(t, "create_calendar_period", log.seen[1].Operation)
	assert.Equal(t, "none", schoolcalendar.ErrorCode(log.seen[1].Err))
	assert.EqualValues(t, 1, log.seen[1].Stats.Rows)
	assert.Positive(t, log.seen[1].Stats.StatementDuration)
}

// --- closing days ---

func TestModuleRunsTheClosingDayLifecycleAndRangeLookup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)

	summer := createClosingDay(t, ctx, module, "2030-07-22", "2030-08-09", "Sommerschließung")
	single := createClosingDay(t, ctx, module, "2031-02-17", "2031-02-17", "Rosenmontag")
	assert.Equal(t, single.StartDate, single.EndDate)
	assert.Equal(t, testpkg.Tenant(t), summer.TenantID)

	found, err := module.FindClosingDay(ctx, summer.ID)
	require.NoError(t, err)
	assert.Equal(t, summer, found)

	updated, err := module.UpdateClosingDay(ctx, schoolcalendar.UpdateClosingDay{ID: summer.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2030-07-22", EndDate: "2030-08-16", Reason: "Sommerschließung verlängert",
	}})
	require.NoError(t, err)
	assert.Equal(t, "2030-08-16", updated.EndDate)

	inside, err := module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2030-07-29", OverlappingTo: "2030-08-02"})
	require.NoError(t, err)
	require.Len(t, inside, 1)
	assert.Equal(t, summer.ID, inside[0].ID)

	edge, err := module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2030-08-16", OverlappingTo: "2030-08-31"})
	require.NoError(t, err)
	require.Len(t, edge, 1, "the inclusive end date matches the window start")

	outside, err := module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: "2030-08-17", OverlappingTo: "2030-08-31"})
	require.NoError(t, err)
	assert.Empty(t, outside)

	all, err := module.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{})
	require.NoError(t, err)
	assert.Equal(t, []int64{summer.ID, single.ID}, []int64{all[0].ID, all[1].ID}, "ordered by start date")

	require.NoError(t, module.DeleteClosingDay(ctx, summer.ID))
	_, err = module.FindClosingDay(ctx, summer.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrClosingDayNotFound)
	require.ErrorIs(t, module.DeleteClosingDay(ctx, summer.ID), schoolcalendar.ErrClosingDayNotFound)
}

// --- dateframes ---

func TestModuleListsDateframesWithNamePatternPaginationAndSorting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	create := func(name string, start, end time.Time) schoolcalendar.Dateframe {
		dateframe, err := module.CreateDateframe(ctx, schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{
			StartDate: start, EndDate: end, Name: name,
		}})
		require.NoError(t, err)
		return dateframe
	}
	january := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)
	alpha := create("Listing alpha", january, january.AddDate(0, 0, 1))
	beta := create("Listing beta", january.AddDate(0, 1, 0), january.AddDate(0, 1, 1))
	other := create("Unrelated", january.AddDate(0, 2, 0), january.AddDate(0, 2, 1))

	page, err := module.ListDateframes(ctx, schoolcalendar.DateframeFilter{
		NamePattern: "Listing %", Limit: 1, Sort: []schoolcalendar.DateframeSort{{Field: schoolcalendar.DateframeSortName, Descending: true}},
	})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, beta.ID, page[0].ID)

	exact, err := module.ListDateframes(ctx, schoolcalendar.DateframeFilter{Name: "listing ALPHA"})
	require.NoError(t, err)
	require.Len(t, exact, 1, "the exact name match folds case")
	assert.Equal(t, alpha.ID, exact[0].ID)

	contains := january.AddDate(0, 2, 0)
	containing, err := module.ListDateframes(ctx, schoolcalendar.DateframeFilter{Contains: &contains})
	require.NoError(t, err)
	require.Len(t, containing, 1)
	assert.Equal(t, other.ID, containing[0].ID)

	from, to := january.AddDate(0, 0, 1), january.AddDate(0, 1, 0)
	overlapping, err := module.ListDateframes(ctx, schoolcalendar.DateframeFilter{OverlappingFrom: &from, OverlappingTo: &to})
	require.NoError(t, err)
	assert.Equal(t, []int64{alpha.ID, beta.ID}, []int64{overlapping[0].ID, overlapping[1].ID}, "inclusive window, ordered by id")

	_, err = module.ListDateframes(ctx, schoolcalendar.DateframeFilter{Sort: []schoolcalendar.DateframeSort{{Field: "description"}}})
	require.ErrorIs(t, err, schoolcalendar.ErrInvalidDateframe, "sortable columns are a closed set")

	require.NoError(t, module.DeleteDateframe(ctx, other.ID))
	_, err = module.FindDateframe(ctx, other.ID)
	require.ErrorIs(t, err, schoolcalendar.ErrDateframeNotFound)
}
