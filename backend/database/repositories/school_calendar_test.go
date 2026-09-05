package repositories_test

// The calendar period, closing day and dateframe repositories are adapters
// over the School Calendar owner (#2666). The tests below pin what the
// composition keeps of the legacy contracts: validation messages, the
// not-found shape callers classify on, nil results for unknown names,
// idempotent deletes, the race-free bootstrap insert, the date-typed overlap
// finders, and the usage counts served by the planning owner. This test role
// may only reach the models through fixtures, so new rows are copies of a
// fixture row and dates come from the test-support date helper.

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarPeriodAdapter_KeepsTheLegacyContracts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).CalendarPeriod
	ctx := testpkg.Ctx(t)

	fixture := testpkg.CreateTestCalendarPeriod(t, db, "Vorlage", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
	created := *fixture
	created.ID = 0
	created.Name = "Schuljahr 2030/2031"
	created.IsActive = true
	anchor := testpkg.Date(2030, time.August, 5)
	created.WeekCycleLength = 2
	created.WeekCycleAnchor = &anchor
	require.NoError(t, repo.Create(ctx, &created))
	assert.Positive(t, created.ID)
	assert.NotEqual(t, fixture.ID, created.ID)
	assert.Equal(t, testpkg.Tenant(t), created.GetTenantID(), "the owner stamps the transaction tenant back onto the model")
	assert.False(t, created.CreatedAt.IsZero())

	t.Run("FindByID returns the persisted row", func(t *testing.T) {
		found, err := repo.FindByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, testpkg.Date(2030, time.August, 1), found.StartDate, "DATE columns round-trip without a timezone shift")
		require.NotNil(t, found.WeekCycleAnchor)
		assert.Equal(t, anchor, *found.WeekCycleAnchor)
	})

	t.Run("FindByID not found", func(t *testing.T) {
		_, err := repo.FindByID(ctx, missingID(created.ID))
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err), "callers classify period lookups with sql.ErrNoRows")
	})

	t.Run("FindByName returns nil without error for an unknown name", func(t *testing.T) {
		found, err := repo.FindByName(ctx, "unbekannt")
		require.NoError(t, err)
		assert.Nil(t, found)

		found, err = repo.FindByName(ctx, created.Name)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
	})

	t.Run("Create keeps the validation messages", func(t *testing.T) {
		require.EqualError(t, repo.Create(ctx, nil), "CalendarPeriod cannot be nil or zero value")
		invalid := *fixture
		invalid.ID = 0
		invalid.Name = ""
		require.EqualError(t, repo.Create(ctx, &invalid), "name is required")
		backwards := *fixture
		backwards.ID = 0
		backwards.Name = "Rückwärts"
		backwards.StartDate, backwards.EndDate = backwards.EndDate, backwards.StartDate
		require.EqualError(t, repo.Create(ctx, &backwards), "end_date must be after start_date")
		noAnchor := *fixture
		noAnchor.ID = 0
		noAnchor.Name = "Ohne Anker"
		noAnchor.WeekCycleLength = 2
		require.EqualError(t, repo.Create(ctx, &noAnchor), "week_cycle_anchor is required when week_cycle_length > 1")
	})

	t.Run("Update writes and refuses invalid rows", func(t *testing.T) {
		created.PeriodType = "semester"
		created.IsActive = false
		require.NoError(t, repo.Update(ctx, &created))
		updated, err := repo.FindByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "semester", updated.PeriodType)
		assert.False(t, updated.IsActive)

		require.EqualError(t, repo.Update(ctx, nil), "CalendarPeriod cannot be nil or zero value")
		broken := created
		broken.Name = ""
		require.EqualError(t, repo.Update(ctx, &broken), "name is required")
		gone := created
		gone.ID = missingID(created.ID)
		err = repo.Update(ctx, &gone)
		require.Error(t, err)
		assert.True(t, testpkg.IsNotFoundError(err))
	})

	t.Run("List serves the unfiltered listing", func(t *testing.T) {
		periods, err := repo.FindByTenantID(ctx)
		require.NoError(t, err)
		require.Len(t, periods, 2)
		assert.Equal(t, fixture.ID, periods[0].ID, "ordered by start date, then id")
		assert.Equal(t, created.ID, periods[1].ID)
	})

	t.Run("Delete is idempotent", func(t *testing.T) {
		require.NoError(t, repo.Delete(ctx, created.ID))
		require.NoError(t, repo.Delete(ctx, created.ID))
		_, err := repo.FindByID(ctx, created.ID)
		assert.True(t, testpkg.IsNotFoundError(err))
	})
}

func TestCalendarPeriodAdapter_CreateIfAbsentIsRaceFreePerTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).CalendarPeriod
	ctx := testpkg.Ctx(t)
	fixture := testpkg.CreateTestCalendarPeriod(t, db, "Vorlage", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))

	first := *fixture
	first.ID = 0
	first.Name = "Bootstrap"
	created, err := repo.CreateIfAbsent(ctx, &first)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Positive(t, first.ID)

	second := *fixture
	second.ID = 0
	second.Name = "Bootstrap"
	createdAgain, err := repo.CreateIfAbsent(ctx, &second)
	require.NoError(t, err, "the conflict is a clean no-op, not an error")
	assert.False(t, createdAgain)
	assert.Zero(t, second.ID, "a skipped insert leaves the model untouched")

	found, err := repo.FindByName(ctx, "Bootstrap")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, first.ID, found.ID)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	other := *fixture
	other.ID = 0
	other.Name = "Bootstrap"
	other.SetTenantID(0)
	createdElsewhere, err := repo.CreateIfAbsent(testpkg.TenantContext(otherTenantID), &other)
	require.NoError(t, err)
	assert.True(t, createdElsewhere, "uniqueness is per tenant, not global")
	assert.Equal(t, otherTenantID, other.GetTenantID())

	_, err = repo.CreateIfAbsent(ctx, nil)
	require.EqualError(t, err, "calendar period cannot be nil")
	invalid := *fixture
	invalid.ID = 0
	invalid.Name = ""
	_, err = repo.CreateIfAbsent(ctx, &invalid)
	require.EqualError(t, err, "name is required")

	// A plain Create of the taken name reports the collision instead of a
	// bare driver error; the constraint stays visible in the chain.
	duplicate := *fixture
	duplicate.ID = 0
	duplicate.Name = "Bootstrap"
	err = repo.Create(ctx, &duplicate)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "calendar period name already exists")
	assert.Contains(t, err.Error(), "unique_calendar_period_name")
}

func TestCalendarPeriodAdapter_OverlapFindersKeepTheDateSemantics(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).CalendarPeriod
	ctx := testpkg.Ctx(t)

	active := testpkg.CreateTestCalendarPeriod(t, db, "Aktiv", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
	testpkg.SetCalendarPeriodActive(t, db, active, true)
	inactive := testpkg.CreateTestCalendarPeriod(t, db, "Inaktiv", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
	adjacent := testpkg.CreateTestCalendarPeriod(t, db, "Angrenzend", testpkg.Date(2031, time.August, 1), testpkg.Date(2032, time.July, 31))
	testpkg.SetCalendarPeriodActive(t, db, adjacent, true)
	semester := testpkg.CreateTestCalendarPeriod(t, db, "Halbjahr", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.January, 31))
	semester.PeriodType = "semester"
	semester.IsActive = true
	require.NoError(t, repo.Update(ctx, semester))

	overlapping, err := repo.FindActiveOverlapping(ctx, testpkg.Date(2030, time.September, 1), testpkg.Date(2030, time.October, 1), 0)
	require.NoError(t, err)
	require.Len(t, overlapping, 2, "active periods of every type, inactive ones never")
	assert.Equal(t, active.ID, overlapping[0].ID)
	assert.Equal(t, semester.ID, overlapping[1].ID)
	for _, period := range overlapping {
		assert.NotEqual(t, inactive.ID, period.ID)
	}

	boundary, err := repo.FindActiveOverlapping(ctx, testpkg.Date(2031, time.July, 31), testpkg.Date(2031, time.July, 31), adjacent.ID)
	require.NoError(t, err)
	require.Len(t, boundary, 1, "the inclusive boundary day counts as overlap")
	assert.Equal(t, active.ID, boundary[0].ID)

	self, err := repo.FindActiveOverlapping(ctx, adjacent.StartDate, adjacent.EndDate, adjacent.ID)
	require.NoError(t, err)
	assert.Empty(t, self, "adjacent ranges do not overlap and the period itself is excluded")

	byType, err := repo.FindActiveOverlappingByType(ctx, "semester", testpkg.Date(2030, time.September, 1), testpkg.Date(2030, time.October, 1), 0)
	require.NoError(t, err)
	require.Len(t, byType, 1)
	assert.Equal(t, semester.ID, byType[0].ID)

	activeOnly, err := repo.FindActiveByTenantID(ctx)
	require.NoError(t, err)
	require.Len(t, activeOnly, 3)
	assert.Equal(t, []int64{active.ID, semester.ID, adjacent.ID}, []int64{activeOnly[0].ID, activeOnly[1].ID, activeOnly[2].ID}, "ordered by start date, then id")

	all, err := repo.FindByTenantID(ctx)
	require.NoError(t, err)
	assert.Len(t, all, 4)
}

func TestCalendarPeriodAdapter_UsageCountsComeFromThePlanningTables(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).CalendarPeriod
	ctx := testpkg.Ctx(t)

	period := testpkg.CreateTestCalendarPeriod(t, db, "Genutzt", testpkg.Date(2030, time.August, 1), testpkg.Date(2031, time.July, 31))
	unused := testpkg.CreateTestCalendarPeriod(t, db, "Ungenutzt", testpkg.Date(2031, time.August, 1), testpkg.Date(2032, time.July, 31))

	usage, err := repo.UsageCounts(ctx)
	require.NoError(t, err)
	assert.Empty(t, usage, "periods without references are omitted")

	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG %d", period.ID))
	_, err = db.NewUpdate().TableExpr("activities.groups").
		Set("calendar_period_id = ?", period.ID).
		Where("id = ?", group.ID).
		Exec(ctx)
	require.NoError(t, err)

	usage, err = repo.UsageCounts(ctx)
	require.NoError(t, err)
	require.Contains(t, usage, period.ID)
	assert.Equal(t, 1, usage[period.ID].ActivityGroups)
	assert.Zero(t, usage[period.ID].EnrollmentPhases)
	assert.NotContains(t, usage, unused.ID)
}

func TestCalendarPeriodAdapter_UsageCountsReturnDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	repo := repositories.NewFactory(db).CalendarPeriod
	ctx := testpkg.Ctx(t)
	require.NoError(t, db.Close())

	usage, err := repo.UsageCounts(ctx)
	require.Error(t, err)
	assert.Nil(t, usage)
	assert.Contains(t, err.Error(), "usage counts")
}

// TestCalendarPeriodFKOnDelete verifies that the three FK columns pointing to
// schedule.calendar_periods are declared ON DELETE SET NULL, so deleting a
// period clears references instead of failing with a constraint violation.
func TestCalendarPeriodFKOnDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	for _, table := range []string{"activities.schedules", "activities.student_enrollments", "activities.supervisors"} {
		t.Run(table, func(t *testing.T) {
			var confdeltype string
			err := db.NewRaw(`
				SELECT c.confdeltype
				FROM pg_constraint c
				JOIN pg_class t ON t.oid = c.conrelid
				JOIN pg_namespace ns ON ns.oid = t.relnamespace
				JOIN pg_class rt ON rt.oid = c.confrelid
				JOIN pg_namespace rns ON rns.oid = rt.relnamespace
				WHERE c.contype = 'f'
				  AND ns.nspname || '.' || t.relname = ?
				  AND rns.nspname = 'schedule'
				  AND rt.relname = 'calendar_periods'
				LIMIT 1
			`, table).Scan(ctx, &confdeltype)
			require.NoError(t, err, "FK to schedule.calendar_periods must exist on %s", table)
			assert.Equal(t, "n", confdeltype, "%s.calendar_period_id must be ON DELETE SET NULL (got %q)", table, confdeltype)
		})
	}
}

func TestClosingDayAdapter_KeepsTheLegacyContracts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).ClosingDay
	ctx := testpkg.Ctx(t)

	fixture := testpkg.CreateTestClosingDay(t, db, testpkg.Date(2030, time.November, 4), testpkg.Date(2030, time.November, 8), "Pädagogische Woche")
	day := *fixture
	day.ID = 0
	day.StartDate = testpkg.Date(2030, time.December, 23)
	day.EndDate = testpkg.Date(2030, time.December, 31)
	day.Reason = "Weihnachtswoche"
	require.NoError(t, repo.Create(ctx, &day))
	assert.Positive(t, day.ID)
	assert.NotEqual(t, fixture.ID, day.ID)
	assert.Equal(t, testpkg.Tenant(t), day.GetTenantID())

	found, err := repo.FindByID(ctx, day.ID)
	require.NoError(t, err)
	assert.Equal(t, testpkg.Date(2030, time.December, 23), found.StartDate)
	assert.Equal(t, "Weihnachtswoche", found.Reason)

	_, err = repo.FindByID(ctx, missingID(day.ID))
	require.Error(t, err)
	assert.True(t, testpkg.IsNotFoundError(err))

	blank := *fixture
	blank.ID = 0
	blank.Reason = ""
	require.EqualError(t, repo.Create(ctx, &blank), "reason is required")
	require.EqualError(t, repo.Create(ctx, nil), "ClosingDay cannot be nil or zero value")

	day.EndDate = testpkg.Date(2031, time.January, 3)
	require.NoError(t, repo.Update(ctx, &day))
	overlapping, err := repo.FindOverlappingRange(ctx, testpkg.Date(2031, time.January, 3), testpkg.Date(2031, time.January, 31))
	require.NoError(t, err)
	require.Len(t, overlapping, 1, "the inclusive end date matches the window start")
	assert.Equal(t, day.ID, overlapping[0].ID)
	outside, err := repo.FindOverlappingRange(ctx, testpkg.Date(2031, time.January, 4), testpkg.Date(2031, time.January, 31))
	require.NoError(t, err)
	assert.Empty(t, outside)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	foreign, err := repo.FindByTenantID(testpkg.TenantContext(otherTenantID))
	require.NoError(t, err)
	assert.Empty(t, foreign, "the other tenant must not see this tenant's closing days")

	all, err := repo.FindByTenantID(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, fixture.ID, all[0].ID, "ordered by start date")

	require.NoError(t, repo.Delete(ctx, day.ID))
	require.NoError(t, repo.Delete(ctx, day.ID), "delete stays idempotent")
	days, err := repo.FindByTenantID(ctx)
	require.NoError(t, err)
	require.Len(t, days, 1)
	assert.Equal(t, fixture.ID, days[0].ID)
}

func TestDateframeAdapter_KeepsTheLegacyLookups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).Dateframe
	ctx := testpkg.Ctx(t)

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	fixture := testpkg.CreateTestDateframe(t, db, "Vorlage", time.Date(2030, time.January, 6, 0, 0, 0, 0, berlin), time.Date(2030, time.January, 10, 0, 0, 0, 0, berlin))
	dateframe := *fixture
	dateframe.ID = 0
	dateframe.Name = "Projektwoche"
	dateframe.StartDate = time.Date(2030, time.March, 3, 0, 0, 0, 0, berlin)
	dateframe.EndDate = time.Date(2030, time.March, 7, 0, 0, 0, 0, berlin)
	require.NoError(t, repo.Create(ctx, &dateframe))
	assert.Positive(t, dateframe.ID)
	assert.Equal(t, testpkg.Tenant(t), dateframe.GetTenantID())

	byName, err := repo.FindByName(ctx, "projektwoche")
	require.NoError(t, err)
	assert.Equal(t, dateframe.ID, byName.ID, "the name lookup folds case")
	_, err = repo.FindByName(ctx, "unbekannt")
	require.Error(t, err)
	assert.True(t, testpkg.IsNotFoundError(err))

	byDate, err := repo.FindByDate(ctx, time.Date(2030, time.March, 5, 15, 30, 0, 0, berlin))
	require.NoError(t, err)
	require.Len(t, byDate, 1, "the clock is dropped before the containment check")
	assert.Equal(t, dateframe.ID, byDate[0].ID)
	outside, err := repo.FindByDate(ctx, time.Date(2030, time.March, 8, 0, 0, 0, 0, berlin))
	require.NoError(t, err)
	assert.Empty(t, outside)

	overlapping, err := repo.FindOverlapping(ctx, time.Date(2030, time.March, 7, 12, 0, 0, 0, berlin), time.Date(2030, time.March, 31, 0, 0, 0, 0, berlin))
	require.NoError(t, err)
	require.Len(t, overlapping, 1, "the inclusive end day overlaps the window start")

	_, err = repo.FindByID(ctx, missingID(dateframe.ID))
	require.Error(t, err)
	assert.True(t, testpkg.IsNotFoundError(err))

	require.EqualError(t, repo.Create(ctx, nil), "Dateframe cannot be nil or zero value")
	require.NoError(t, repo.Delete(ctx, dateframe.ID))
	require.NoError(t, repo.Delete(ctx, dateframe.ID), "delete stays idempotent")
}

func TestCapacityOccurrencesReadPeriodsThroughTheCalendarOwner(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.Ctx(t)

	period := testpkg.CreateTestCalendarPeriod(t, db, "Kapazität", testpkg.Date(2030, time.September, 2), testpkg.Date(2030, time.September, 6))
	testpkg.SetCalendarPeriodActive(t, db, period, true)

	occurrences, err := factory.ActivityGroup.ListTemplateCapacityOccurrences(ctx, &period.ID, []int64{missingID(period.ID)})
	require.NoError(t, err, "the owner query is bound by the factory; an unknown template yields no rows")
	assert.Empty(t, occurrences)
}
