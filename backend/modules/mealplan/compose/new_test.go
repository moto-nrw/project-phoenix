package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var testBerlin = time.FixedZone("Europe/Berlin", 2*60*60)

type testParticipantRow struct {
	StudentID   int64  `bun:"student_id"`
	FirstName   string `bun:"first_name"`
	LastName    string `bun:"last_name"`
	SchoolClass string `bun:"school_class"`
}

// openCalendar is a school calendar without closing days or holidays.
func openCalendar(context.Context, string, string) (map[string]bool, map[string]bool, error) {
	return map[string]bool{}, map[string]bool{}, nil
}

// fixedCalendar is a school calendar with the given closed and holiday days.
func fixedCalendar(closed, holiday []string) CalendarFinder {
	return func(_ context.Context, from, to string) (map[string]bool, map[string]bool, error) {
		closedDays, holidayDays := map[string]bool{}, map[string]bool{}
		for _, day := range closed {
			if day >= from && day <= to {
				closedDays[day] = true
			}
		}
		for _, day := range holiday {
			if day >= from && day <= to {
				holidayDays[day] = true
			}
		}
		return closedDays, holidayDays, nil
	}
}

func testParticipantFinder(ctx context.Context, date string) ([]ParticipantCandidate, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("test participant finder: transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("test participant finder: unsupported transaction %T", transaction)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var rows []testParticipantRow
	err = tx.NewSelect().Model(&rows).
		ModelTableExpr(`users.student_profiles AS "student"`).
		Join(`JOIN users.student_school_memberships AS "membership" ON "membership".student_profile_id = "student".id AND "membership".tenant_id = "student".tenant_id AND "membership".deleted_at IS NULL`).
		ColumnExpr(`"student".id AS student_id, "person".first_name, "person".last_name, "membership".school_class`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id`).
		Where(`"student".tenant_id = ?`, tenantID.Int64()).
		Where(`"membership".status = 'active'`).
		Where(`("membership".enrolled_from IS NULL OR "membership".enrolled_from <= ?)`, date).
		Where(`("membership".enrolled_until IS NULL OR "membership".enrolled_until >= ?)`, date).
		OrderExpr(`"membership".school_class, "person".last_name, "person".first_name, "student".id`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ParticipantCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, ParticipantCandidate(row))
	}
	return result, nil
}

func dailyParticipantIDs(list mealplan.DailyList) []int64 {
	ids := make([]int64, 0, len(list.Participants))
	for _, participant := range list.Participants {
		ids = append(ids, participant.StudentID)
	}
	return ids
}

type testStatusDayRow struct {
	bun.BaseModel `bun:"table:student_status_days,alias:student_status_day"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id"`
	StudentID     int64     `bun:"student_id"`
	Date          string    `bun:"date"`
	Status        string    `bun:"status"`
	ReportedAt    time.Time `bun:"reported_at"`
	Source        string    `bun:"source"`
}

func createSickStatusDay(t *testing.T, db *bun.DB, tenantID, studentID int64, date string, reportedAt time.Time) *testStatusDayRow {
	t.Helper()
	row := &testStatusDayRow{TenantID: tenantID, StudentID: studentID, Date: date, Status: "sick", ReportedAt: reportedAt, Source: "manual"}
	_, err := db.NewInsert().Model(row).ModelTableExpr(`active.student_status_days`).Exec(context.Background())
	require.NoError(t, err)
	return row
}

type testMealPlanSettings struct {
	mealPlanEnabled         bool
	mealRegistrationEnabled bool
	cutoff                  string
	// ordering replaces the default single-deadline rules built from cutoff.
	ordering *OrderingSettings
	err      error
}

func (s testMealPlanSettings) MealPlanEnabled(context.Context) (bool, error) {
	return s.mealPlanEnabled, s.err
}

func (s testMealPlanSettings) MealRegistrationEnabled(context.Context) (bool, error) {
	return s.mealRegistrationEnabled, s.err
}

func (s testMealPlanSettings) MealOrderingSettings(context.Context) (OrderingSettings, error) {
	if s.ordering != nil {
		return *s.ordering, s.err
	}
	return singleDeadlineSettings(s.cutoff), s.err
}

// singleDeadlineSettings are the registry defaults with the given clock time:
// one deadline on the meal day, as before #3873.
func singleDeadlineSettings(clock string) OrderingSettings {
	return OrderingSettings{
		Model: "ongoing_single", RegistrationKind: "days_before", RegistrationWeekday: "thursday", RegistrationTime: clock,
		CancellationKind: "days_before", CancellationWeekday: "thursday", CancellationTime: "09:00",
		AnnualChanges: "parents_cancel", SickAfterDeadline: "stays_ordered", OrderRhythm: "daily", OrderDetail: "by_class",
	}
}

func enabledTestSettings(settingErr error) testMealPlanSettings {
	return testMealPlanSettings{mealPlanEnabled: true, mealRegistrationEnabled: true, cutoff: "09:00", err: settingErr}
}

func buildModule(t *testing.T, db *bun.DB, enabled bool, settingErr error) *mealplan.Module {
	t.Helper()
	return buildModuleAt(t, db, enabled, settingErr, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
}

func buildModuleAt(t *testing.T, db *bun.DB, enabled bool, settingErr error, now time.Time) *mealplan.Module {
	t.Helper()
	module, err := New(Dependencies{
		DB:           db,
		Settings:     testMealPlanSettings{mealPlanEnabled: enabled, mealRegistrationEnabled: enabled, cutoff: "09:00", err: settingErr},
		Observe:      func(Observation) {},
		Now:          func() time.Time { return now },
		Participants: testParticipantFinder,
		Calendar:     openCalendar,
	})
	require.NoError(t, err)
	return module
}

func mustDate(t *testing.T, value string) mealplan.Date {
	t.Helper()
	date, err := mealplan.ParseDate(value)
	require.NoError(t, err)
	return date
}

func TestModulePersistsReplacesAndClearsOneTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	ctx := testpkg.Ctx(t)
	date := mustDate(t, "2026-09-07")

	require.NoError(t, module.ReplaceDay(ctx, mealplan.ReplaceDay{Date: date, Dishes: []mealplan.Dish{
		{Dish: "Menü 1"}, {Dish: "Menü 2"},
	}}))
	entries, err := module.Week(ctx, date)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "Menü 1", entries[0].Dish)
	assert.Equal(t, 0, entries[0].Position)
	assert.Equal(t, "Menü 2", entries[1].Dish)
	assert.Equal(t, 1, entries[1].Position)

	require.NoError(t, module.ReplaceDay(ctx, mealplan.ReplaceDay{Date: date, Dishes: []mealplan.Dish{{Dish: "Auflauf"}}}))
	entries, err = module.Week(ctx, date)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Auflauf", entries[0].Dish)

	require.NoError(t, module.ClearDay(ctx, date))
	entries, err = module.Week(ctx, date)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestModuleRLSHidesAnotherTenantsPlan(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	date := mustDate(t, "2026-09-14")
	require.NoError(t, module.ReplaceDay(testpkg.Ctx(t), mealplan.ReplaceDay{Date: date, Dishes: []mealplan.Dish{{Dish: "Tenant A"}}}))

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherContext := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), otherTenantID)
	entries, err := module.Week(otherContext, date)
	require.NoError(t, err)
	assert.Empty(t, entries)

	require.NoError(t, module.ReplaceDay(otherContext, mealplan.ReplaceDay{Date: date, Dishes: []mealplan.Dish{{Dish: "Tenant B"}}}))
	require.NoError(t, module.ClearDay(testpkg.Ctx(t), date))
	entries, err = module.Week(otherContext, date)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Tenant B", entries[0].Dish)
}

func TestModuleReplaceRollsBackWithOuterTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	ctx := testpkg.Ctx(t)
	date := mustDate(t, "2026-09-21")
	wantErr := errors.New("abort command")

	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, module.ReplaceDay(txCtx, mealplan.ReplaceDay{Date: date, Dishes: []mealplan.Dish{{Dish: "Nicht speichern"}}}))
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	entries, err := module.Week(ctx, date)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestModuleKeepsSettingsErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	wantErr := errors.New("settings unavailable")
	module := buildModule(t, db, false, wantErr)

	_, err := module.Week(testpkg.Ctx(t), "2026-09-07")
	require.ErrorIs(t, err, wantErr)
	assert.NotErrorIs(t, err, mealplan.ErrDisabled)
}

func TestModuleKeepsPersistenceErrorsVisible(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	missingTenantID := testpkg.UniqueTestTenantID(t)
	ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), missingTenantID)

	err := module.ReplaceDay(ctx, mealplan.ReplaceDay{
		Date: mustDate(t, "2026-10-05"), Dishes: []mealplan.Dish{{Dish: "Suppe"}},
	})

	require.ErrorContains(t, err, "meal plan postgres: insert day")
	assert.NotErrorIs(t, err, mealplan.ErrDisabled)
}

func TestModuleReportsPersistenceQueryAndRowCounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observations []Observation
	module, err := New(Dependencies{
		DB:           db,
		Settings:     enabledTestSettings(nil),
		Observe:      func(observation Observation) { observations = append(observations, observation) },
		Now:          func() time.Time { return time.Date(2026, 9, 28, 8, 0, 0, 0, testBerlin) },
		Participants: testParticipantFinder,
		Calendar:     openCalendar,
	})
	require.NoError(t, err)

	require.NoError(t, module.ReplaceDay(testpkg.Ctx(t), mealplan.ReplaceDay{
		Date: mustDate(t, "2026-09-28"), Dishes: []mealplan.Dish{{Dish: "Eintopf"}},
	}))
	require.Len(t, observations, 1)
	assert.EqualValues(t, 2, observations[0].Stats.Queries)
	assert.EqualValues(t, 1, observations[0].Stats.Rows)
	assert.Positive(t, observations[0].Stats.StatementDuration)
}

func TestParticipationResolvesRegularDaysAndOneDayOverride(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Mia", "Muster", "2a")
	account := testpkg.CreateTestAccount(t, db, "meal-participation")
	ctx := testpkg.Ctx(t)

	effectiveFrom, err := module.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{
		StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Wednesday},
	})
	require.NoError(t, err)
	assert.Equal(t, mealplan.Date("2026-09-07"), effectiveFrom)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-08", Participating: true,
	}))

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	require.Len(t, plan.Days, 5)
	assert.Equal(t, []mealplan.Weekday{mealplan.Monday, mealplan.Wednesday}, plan.Weekdays)
	assert.True(t, plan.Days[0].Participating)
	assert.Equal(t, mealplan.ParticipationRegular, plan.Days[0].Source)
	assert.True(t, plan.Days[1].Participating)
	assert.Equal(t, mealplan.ParticipationOverride, plan.Days[1].Source)
	assert.False(t, plan.Days[3].Participating)
}

func TestParticipationRejectsSameDayChangeAfterKitchenCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{
		DB: db, Settings: enabledTestSettings(nil),
		Observe:      func(Observation) {},
		Now:          func() time.Time { return time.Date(2026, 9, 7, 9, 1, 0, 0, testBerlin) },
		Participants: testParticipantFinder,
		Calendar:     openCalendar,
	})
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Noah", "Beispiel", "3b")
	account := testpkg.CreateTestAccount(t, db, "meal-cutoff")

	err = module.SetParticipationForDay(testpkg.Ctx(t), mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)
}

func TestRegistrationUsesSeparateToggleAndTenantCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Toni", "Frist", "4a")
	account := testpkg.CreateTestAccount(t, db, "meal-custom-cutoff")

	disabledModule, err := New(Dependencies{
		DB: db,
		Settings: testMealPlanSettings{
			mealPlanEnabled: true, mealRegistrationEnabled: false, cutoff: "10:30",
		},
		Observe:      func(Observation) {},
		Now:          func() time.Time { return time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin) },
		Participants: testParticipantFinder,
		Calendar:     openCalendar,
	})
	require.NoError(t, err)
	available, err := disabledModule.RegistrationAvailable(testpkg.Ctx(t))
	require.NoError(t, err)
	assert.False(t, available)
	err = disabledModule.SetParticipationForDay(testpkg.Ctx(t), mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	})
	require.ErrorIs(t, err, mealplan.ErrRegistrationDisabled)

	enabledModule, err := New(Dependencies{
		DB: db,
		Settings: testMealPlanSettings{
			mealPlanEnabled: true, mealRegistrationEnabled: true, cutoff: "10:30",
		},
		Observe:      func(Observation) {},
		Now:          func() time.Time { return time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin) },
		Participants: testParticipantFinder,
		Calendar:     openCalendar,
	})
	require.NoError(t, err)
	require.NoError(t, enabledModule.SetParticipationForDay(testpkg.Ctx(t), mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	}))
}

func TestDailyListOmitsConfirmedSicknessReportedBeforeCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Lina", "Küche", "1a")
	account := testpkg.CreateTestAccount(t, db, "meal-sick")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	}))
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-07", time.Date(2026, 9, 7, 8, 30, 0, 0, testBerlin))

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.NotContains(t, dailyParticipantIDs(list), student.ID)
}

func TestDailyListIncludesChildWhenSicknessIsDeletedBeforeCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var databaseNow time.Time
	require.NoError(t, db.NewRaw(`SELECT clock_timestamp()`).Scan(context.Background(), &databaseNow))
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	testDay := databaseNow.In(berlin).AddDate(0, 0, 1)
	for testDay.Weekday() == time.Saturday || testDay.Weekday() == time.Sunday {
		testDay = testDay.AddDate(0, 0, 1)
	}
	date := testDay.Format("2006-01-02")
	moduleNow := time.Date(testDay.Year(), testDay.Month(), testDay.Day(), 8, 0, 0, 0, berlin)
	module := buildModuleAt(t, db, true, nil, moduleNow)
	student := testpkg.CreateTestStudent(t, db, "Nora", "Korrektur", "1c")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-deleted-before-cutoff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: mealplan.Date(date), Participating: true,
	}))
	status := createSickStatusDay(t, db, student.TenantID, student.ID, date, databaseNow)
	_, err = db.NewDelete().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		WherePK().Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, mealplan.Date(date))
	require.NoError(t, err)
	assert.Contains(t, dailyParticipantIDs(list), student.ID)
}

func TestDailyListKeepsRegistrationWhenSicknessIsReportedAfterCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Emil", "Spätmeldung", "1b")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-after-cutoff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	}))
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-07", time.Date(2026, 9, 7, 9, 1, 0, 0, testBerlin))

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Contains(t, dailyParticipantIDs(list), student.ID)
}

func TestDailyListKeepsCutoffStateWhenSicknessIsClearedAfterCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Ada", "Spätkorrektur", "2c")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-cleared-after-cutoff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	}))
	status := createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-07", time.Date(2026, 9, 7, 8, 30, 0, 0, testBerlin))
	_, err := db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`cleared_at = ?`, time.Date(2026, 9, 7, 9, 30, 0, 0, testBerlin)).
		WherePK().Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.NotContains(t, dailyParticipantIDs(list), student.ID)

	_, err = db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`reported_at = ?`, time.Date(2026, 9, 7, 10, 30, 0, 0, testBerlin)).
		Set(`cleared_at = NULL`).
		WherePK().Exec(context.Background())
	require.NoError(t, err)
	list, err = module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.NotContains(t, dailyParticipantIDs(list), student.ID)
}

func TestDailyListOmitsChildWhenClearedSicknessIsReopenedBeforeCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModuleAt(t, db, true, nil, time.Date(2099, 9, 7, 8, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Mila", "Wiedereröffnet", "2d")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-reopened-before-cutoff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2099-09-07", Participating: true,
	}))
	reportedAt := time.Date(2000, 1, 3, 8, 0, 0, 0, testBerlin)
	status := createSickStatusDay(t, db, student.TenantID, student.ID, "2099-09-07", reportedAt)
	_, err := db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`cleared_at = ?`, time.Date(2000, 1, 3, 8, 30, 0, 0, testBerlin)).
		WherePK().Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`cleared_at = NULL`).
		WherePK().Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, "2099-09-07")
	require.NoError(t, err)
	assert.NotContains(t, dailyParticipantIDs(list), student.ID)
}

func TestParticipationUsesLatestSicknessSnapshotWhenTimestampsMatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Ida", "Gleichzeitig", "3a")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-equal-timestamps")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	}))
	changedAt := time.Date(2026, 9, 7, 8, 30, 0, 0, testBerlin)
	_, err := db.NewRaw(`
		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		VALUES (?, ?, ?, ?, ?, NULL)
	`, student.TenantID, student.ID, "2026-09-07", changedAt, changedAt).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`
		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, student.TenantID, student.ID, "2026-09-07", changedAt, changedAt, changedAt).Exec(context.Background())
	require.NoError(t, err)

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, plan.Days, 1)
	assert.True(t, plan.Days[0].Participating)
	assert.Equal(t, mealplan.ParticipationOverride, plan.Days[0].Source)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Contains(t, dailyParticipantIDs(list), student.ID)
}

func buildModuleWithRules(t *testing.T, db *bun.DB, ordering OrderingSettings, now time.Time) *mealplan.Module {
	t.Helper()
	module, err := New(Dependencies{
		DB:           db,
		Settings:     testMealPlanSettings{mealPlanEnabled: true, mealRegistrationEnabled: true, ordering: &ordering},
		Observe:      func(Observation) {},
		Now:          func() time.Time { return now },
		Participants: testParticipantFinder,
		Calendar:     openCalendar,
	})
	require.NoError(t, err)
	return module
}

// splitSettings: register until Thursday noon of the previous week, cancel
// until 08:00 on the meal day.
func splitSettings() OrderingSettings {
	settings := singleDeadlineSettings("12:00")
	settings.Model = "ongoing_split"
	settings.RegistrationKind, settings.RegistrationWeekday = "previous_week", "thursday"
	settings.CancellationKind, settings.CancellationTime = "days_before", "08:00"
	return settings
}

func TestSplitModelLetsParentsCancelAfterRegistrationClosed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	// Friday 2026-09-11: the registration for the week of 2026-09-14 closed
	// on Thursday noon; cancelling stays open until 08:00 each day.
	module := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 11, 10, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Paula", "Getrennt", "2b")
	account := testpkg.CreateTestAccount(t, db, "meal-split")
	ctx := testpkg.Ctx(t)

	err := module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15", Participating: false}))

	plan, err := module.Participation(ctx, student.ID, "2026-09-14", "2026-09-14")
	require.NoError(t, err)
	require.Len(t, plan.Days, 1)
	day := plan.Days[0]
	assert.False(t, day.CanRegister)
	assert.True(t, day.CanCancel)
	assert.True(t, day.Changeable)
	assert.Equal(t, time.Date(2026, 9, 10, 12, 0, 0, 0, testBerlin).UTC(), day.RegisterUntil.UTC())
	assert.Equal(t, time.Date(2026, 9, 14, 8, 0, 0, 0, testBerlin).UTC(), day.CancelUntil.UTC())
	assert.Equal(t, mealplan.RegistrationModelSplit, plan.Model)
	assert.True(t, plan.ScheduleChangeable)
}

func TestClearingAnOverrideFollowsTheDeadlineOfItsDirection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Karl", "Rücknahme", "3c")
	account := testpkg.CreateTestAccount(t, db, "meal-clear-direction")
	ctx := testpkg.Ctx(t)
	early := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 9, 10, 0, 0, 0, testBerlin))
	_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Tuesday}})
	require.NoError(t, err)
	require.NoError(t, early.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15", Participating: false}))

	// After the registration deadline: dropping the cancellation would
	// register the Tuesday again, which only the registration deadline allows.
	late := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 11, 10, 0, 0, 0, testBerlin))
	err = late.ClearParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15"})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)

	// Dropping a registration on a day the pattern leaves out cancels it,
	// which the later cancellation deadline still allows.
	require.NoError(t, early.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-16", Participating: true}))
	require.NoError(t, late.ClearParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-16"}))
}

func TestAnnualModelStopsParentRegistrationAfterTheDeadline(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("09:00")
	settings.Model, settings.AnnualDeadline = "annual", "2026-08-31"
	settings.CancellationKind, settings.CancellationLeadDays, settings.CancellationTime = "days_before", 1, "18:00"
	student := testpkg.CreateTestStudent(t, db, "Jona", "Schuljahr", "1d")
	account := testpkg.CreateTestAccount(t, db, "meal-annual")
	ctx := testpkg.Ctx(t)

	august := buildModuleWithRules(t, db, settings, time.Date(2026, 8, 28, 10, 0, 0, 0, testBerlin))
	_, err := august.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Thursday}})
	require.NoError(t, err)

	september := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	_, err = september.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Friday}})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)
	err = september.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-11", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)
	require.NoError(t, september.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-10", Participating: false}))

	plan, err := september.Participation(ctx, student.ID, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	assert.False(t, plan.ScheduleChangeable)
	assert.Equal(t, mealplan.RegistrationModelAnnual, plan.Model)
	assert.False(t, plan.Days[3].Participating, "the Thursday is cancelled")

	settings.AnnualChanges = "school_only"
	schoolOnly := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	err = schoolOnly.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-14", Participating: false})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)
}

func TestWeekChangeCancelsOpenDaysAndReportsLockedOnes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("09:00")
	settings.RegistrationLeadDays = 1
	student := testpkg.CreateTestStudent(t, db, "Wanda", "Woche", "4b")
	account := testpkg.CreateTestAccount(t, db, "meal-week")
	ctx := testpkg.Ctx(t)
	early := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 3, 10, 0, 0, 0, testBerlin))
	_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Wednesday, mealplan.Thursday}})
	require.NoError(t, err)

	// Tuesday 2026-09-08, 10:00: Monday is past, Tuesday and Wednesday are
	// past their deadline (the day before, 09:00), Thursday is open and
	// Friday is no meal day of the child.
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	change, err := module.SetParticipationForWeek(ctx, mealplan.SetParticipationWeek{StudentID: student.ID, GuardianAccountID: account.ID, WeekOf: "2026-09-09", Participating: false})
	require.NoError(t, err)
	assert.Equal(t, []mealplan.Date{"2026-09-10"}, change.Changed)
	assert.Equal(t, []mealplan.Date{"2026-09-08", "2026-09-09"}, change.Locked)

	plan, err := module.Participation(ctx, student.ID, "2026-09-10", "2026-09-11")
	require.NoError(t, err)
	assert.Equal(t, mealplan.ParticipationOverride, plan.Days[0].Source)
	assert.False(t, plan.Days[0].Participating)
	assert.Equal(t, mealplan.ParticipationNone, plan.Days[1].Source, "a day the child does not eat gets no exception")

	again, err := module.SetParticipationForWeek(ctx, mealplan.SetParticipationWeek{StudentID: student.ID, GuardianAccountID: account.ID, WeekOf: "2026-09-07", Participating: false})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff, "only locked days are left to cancel")
	assert.Empty(t, again.Changed)

	nextWeek := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	_, err = nextWeek.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: nil})
	require.NoError(t, err)
	empty, err := nextWeek.SetParticipationForWeek(ctx, mealplan.SetParticipationWeek{StudentID: student.ID, GuardianAccountID: account.ID, WeekOf: "2026-09-21", Participating: false})
	require.NoError(t, err, "a week without meals has nothing to cancel")
	assert.Empty(t, empty.Changed)
	assert.Empty(t, empty.Locked)
}

func TestInvalidOrderingSettingsRefuseRegistration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := splitSettings()
	settings.CancellationTime = "acht Uhr"
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))

	_, err := module.Participation(testpkg.Ctx(t), 1, "2026-09-07", "2026-09-11")
	require.ErrorIs(t, err, mealplan.ErrInvalidParticipation)
}

func TestSickCancelledModeCancelsAbsencesReportedAfterTheDeadline(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("09:00")
	settings.SickAfterDeadline = "cancelled"
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Theo", "Spätkrank", "2e")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-cancelled")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true}))
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-07", time.Date(2026, 9, 7, 10, 30, 0, 0, testBerlin))

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.NotContains(t, dailyParticipantIDs(list), student.ID, "a sick note after the deadline still cancels")

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.True(t, plan.SickCancels)
	assert.Equal(t, mealplan.ParticipationSick, plan.Days[0].Source)

	staysOrdered := buildModule(t, db, true, nil)
	plan, err = staysOrdered.Participation(ctx, student.ID, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	assert.False(t, plan.SickCancels)
	assert.True(t, plan.Days[0].Participating, "with the default the meal stays ordered")
}

func buildModuleWithCalendar(t *testing.T, db *bun.DB, calendar CalendarFinder, now time.Time) *mealplan.Module {
	t.Helper()
	module, err := New(Dependencies{
		DB:           db,
		Settings:     enabledTestSettings(nil),
		Observe:      func(Observation) {},
		Now:          func() time.Time { return now },
		Participants: testParticipantFinder,
		Calendar:     calendar,
	})
	require.NoError(t, err)
	return module
}

func createStatusDay(t *testing.T, db *bun.DB, tenantID, studentID int64, date, status string, reportedAt time.Time) {
	t.Helper()
	row := &testStatusDayRow{TenantID: tenantID, StudentID: studentID, Date: date, Status: status, ReportedAt: reportedAt, Source: "manual"}
	_, err := db.NewInsert().Model(row).ModelTableExpr(`active.student_status_days`).Exec(context.Background())
	require.NoError(t, err)
}

// #3873: on a closing day or public holiday nobody eats, and parents cannot
// register for it.
func TestClosedDaysCountNobodyAndStayClosedForParents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModuleWithCalendar(t, db, fixedCalendar([]string{"2026-09-09"}, nil), time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Clara", "Schließtag", "2f")
	account := testpkg.CreateTestAccount(t, db, "meal-closed-day")
	ctx := testpkg.Ctx(t)
	_, err := module.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Wednesday}})
	require.NoError(t, err)

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-11")
	require.NoError(t, err)
	wednesday := plan.Days[2]
	assert.Equal(t, mealplan.ParticipationClosed, wednesday.Source)
	assert.False(t, wednesday.Participating)
	assert.False(t, wednesday.Changeable, "a closed day offers nothing to change")
	assert.True(t, plan.Days[0].Participating, "Monday stays registered")

	err = module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-09", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff)

	list, err := module.DailyList(ctx, "2026-09-09")
	require.NoError(t, err)
	assert.Empty(t, list.Participants)
	assert.True(t, list.Closed, "the list says why it is empty")

	change, err := module.SetParticipationForWeek(ctx, mealplan.SetParticipationWeek{StudentID: student.ID, GuardianAccountID: account.ID, WeekOf: "2026-09-07", Participating: false})
	require.NoError(t, err)
	assert.NotContains(t, change.Changed, mealplan.Date("2026-09-09"))
	assert.NotContains(t, change.Locked, mealplan.Date("2026-09-09"))
}

// #3873: the weekly pattern does not apply in school holidays, but a single
// day registered for holiday care still counts.
func TestHolidaysSuspendTheWeeklyPatternButKeepSingleRegistrations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModuleWithCalendar(t, db, fixedCalendar(nil, []string{"2026-09-07", "2026-09-08"}), time.Date(2026, 9, 4, 8, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Henri", "Ferien", "3d")
	account := testpkg.CreateTestAccount(t, db, "meal-holidays")
	ctx := testpkg.Ctx(t)
	_, err := module.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday, mealplan.Tuesday, mealplan.Wednesday}})
	require.NoError(t, err)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-08", Participating: true}))

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-09")
	require.NoError(t, err)
	assert.False(t, plan.Days[0].Participating, "no pattern in the holidays")
	assert.Equal(t, mealplan.ParticipationNone, plan.Days[0].Source)
	assert.True(t, plan.Days[0].Changeable, "holiday care can still be registered")
	assert.True(t, plan.Days[1].Participating, "holiday care registered for the day")
	assert.True(t, plan.Days[2].Participating, "the pattern applies after the holidays")

	monday, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.NotContains(t, dailyParticipantIDs(monday), student.ID)
	tuesday, err := module.DailyList(ctx, "2026-09-08")
	require.NoError(t, err)
	assert.Contains(t, dailyParticipantIDs(tuesday), student.ID)
}

// #3873: an excused child or one on a class trip does not eat either; the
// parent sees "absent", never the reason, and a cleared status does not hide
// another one of the same day.
func TestExcusedAndClassTripCancelTheMealLikeASickNote(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Ella", "Entschuldigt", "4c")
	account := testpkg.CreateTestAccount(t, db, "meal-excused")
	ctx := testpkg.Ctx(t)
	for _, date := range []mealplan.Date{"2026-09-07", "2026-09-08", "2026-09-09"} {
		require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: date, Participating: true}))
	}
	before := time.Date(2026, 9, 6, 18, 0, 0, 0, testBerlin)
	createStatusDay(t, db, student.TenantID, student.ID, "2026-09-07", "excused", before)
	createStatusDay(t, db, student.TenantID, student.ID, "2026-09-08", "class_trip", before)
	createStatusDay(t, db, student.TenantID, student.ID, "2026-09-09", "excused", before)
	sick := createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-09", before)
	_, err := db.NewUpdate().Model(sick).ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`cleared_at = ?`, before.Add(time.Hour)).WherePK().Exec(context.Background())
	require.NoError(t, err)

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-09")
	require.NoError(t, err)
	for _, day := range plan.Days {
		assert.False(t, day.Participating, day.Date)
		assert.Equal(t, mealplan.ParticipationAbsent, day.Source, day.Date)
	}
	for _, date := range []mealplan.Date{"2026-09-07", "2026-09-08", "2026-09-09"} {
		list, err := module.DailyList(ctx, date)
		require.NoError(t, err)
		assert.NotContains(t, dailyParticipantIDs(list), student.ID, date)
	}
}

func TestDailyListCarriesTheDayDeadlineUnderALeadTime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("12:00")
	settings.RegistrationLeadDays = 3
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))

	list, err := module.DailyList(testpkg.Ctx(t), "2026-09-14")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 9, 11, 12, 0, 0, 0, testBerlin).UTC(), list.Deadline.UTC(), "Monday's deadline is Friday noon")
	assert.Equal(t, "12:00", list.CutoffTime)
}

// Under the split model an absence keeps cancelling until the later
// cancellation deadline, even with the default "stays ordered".
func TestSplitModelCancelsAbsencesUntilTheCancellationDeadline(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 9, 10, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Selma", "Spät", "1e")
	account := testpkg.CreateTestAccount(t, db, "meal-split-absence")
	ctx := testpkg.Ctx(t)
	early := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 3, 10, 0, 0, 0, testBerlin))
	require.NoError(t, early.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-14", Participating: true}))
	require.NoError(t, early.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-15", Participating: true}))
	// Reported after the registration deadline (Thursday 2026-09-10 noon) but
	// before 08:00 on the Monday: cancelled. Reported at 08:30 on the
	// Tuesday: past its cancellation deadline, the meal stays ordered.
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-14", time.Date(2026, 9, 14, 7, 30, 0, 0, testBerlin))
	createSickStatusDay(t, db, student.TenantID, student.ID, "2026-09-15", time.Date(2026, 9, 15, 8, 30, 0, 0, testBerlin))

	plan, err := module.Participation(ctx, student.ID, "2026-09-14", "2026-09-15")
	require.NoError(t, err)
	assert.Equal(t, mealplan.ParticipationSick, plan.Days[0].Source)
	assert.True(t, plan.Days[1].Participating)
}

func TestPreviousWeekDeadlineMovesTheNewPatternToTheWeekAfter(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Pia", "Vorwoche", "2g")
	account := testpkg.CreateTestAccount(t, db, "meal-previous-week")
	ctx := testpkg.Ctx(t)
	before := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 10, 11, 0, 0, 0, testBerlin))
	effectiveFrom, err := before.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday}})
	require.NoError(t, err)
	assert.Equal(t, mealplan.Date("2026-09-14"), effectiveFrom, "before Thursday noon the next week is still open")

	after := buildModuleWithRules(t, db, splitSettings(), time.Date(2026, 9, 10, 13, 0, 0, 0, testBerlin))
	effectiveFrom, err = after.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Tuesday}})
	require.NoError(t, err)
	assert.Equal(t, mealplan.Date("2026-09-21"), effectiveFrom)
}

// Clearing a holiday-care registration would cancel the day, so the
// cancellation deadline decides; clearing a cancellation the holiday makes
// moot changes nothing and stays allowed.
func TestClearingAnOverrideInTheHolidaysFollowsTheHolidayResult(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	holidays := fixedCalendar(nil, []string{"2026-09-08", "2026-09-09"})
	student := testpkg.CreateTestStudent(t, db, "Hugo", "Ferienbetreuung", "3e")
	account := testpkg.CreateTestAccount(t, db, "meal-holiday-clear")
	ctx := testpkg.Ctx(t)
	early := buildModuleWithCalendar(t, db, holidays, time.Date(2026, 9, 4, 8, 0, 0, 0, testBerlin))
	_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Tuesday, mealplan.Wednesday}})
	require.NoError(t, err)
	require.NoError(t, early.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-08", Participating: true}))
	require.NoError(t, early.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-09", Participating: false}))

	late := buildModuleWithCalendar(t, db, holidays, time.Date(2026, 9, 8, 10, 0, 0, 0, testBerlin))
	err = late.ClearParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-08"})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff, "holiday care cannot be dropped after the deadline")

	plan, err := late.Participation(ctx, student.ID, "2026-09-08", "2026-09-09")
	require.NoError(t, err)
	assert.False(t, plan.Days[0].Regular, "the pattern does not apply in the holidays")
	assert.True(t, plan.Days[0].Participating)

	require.NoError(t, late.ClearParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-09"}))
}

// A registered child who is absent still has the meal registered: cancelling
// the week must cancel that day too, so clearing the absence later does not
// bring the meal back.
func TestWeekChangeCancelsRegisteredDaysOfAnAbsentChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Anton", "Abwesend", "4d")
	account := testpkg.CreateTestAccount(t, db, "meal-week-absent")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-10", Participating: true}))
	createStatusDay(t, db, student.TenantID, student.ID, "2026-09-10", "excused", time.Date(2026, 9, 6, 18, 0, 0, 0, testBerlin))

	change, err := module.SetParticipationForWeek(ctx, mealplan.SetParticipationWeek{StudentID: student.ID, GuardianAccountID: account.ID, WeekOf: "2026-09-07", Participating: false})
	require.NoError(t, err)
	assert.Equal(t, []mealplan.Date{"2026-09-10"}, change.Changed)
}

// #3873: the school changes a registration at any time, deadlines and the
// annual registration included, but never a past or closed day.
func TestSchoolChangesIgnoreDeadlinesButNotThePast(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	settings := singleDeadlineSettings("09:00")
	settings.Model, settings.AnnualDeadline, settings.AnnualChanges = "annual", "2026-08-31", "school_only"
	settings.CancellationKind, settings.CancellationTime = "days_before", "09:00"
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 9, 12, 0, 0, 0, testBerlin))
	student := testpkg.CreateTestStudent(t, db, "Sina", "Schule", "1f")
	staff := testpkg.CreateTestAccount(t, db, "meal-school-change")
	ctx := testpkg.Ctx(t)

	require.NoError(t, module.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-09", Participating: true}), "today, after its deadline")
	err := module.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-08", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrInvalidParticipation, "yesterday is over")

	effectiveFrom, err := module.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: student.ID, StaffAccountID: staff.ID, Weekdays: []mealplan.Weekday{mealplan.Thursday}, EffectiveFrom: "2026-09-10"})
	require.NoError(t, err, "after the annual deadline only the school registers")
	assert.Equal(t, mealplan.Date("2026-09-10"), effectiveFrom)
	_, err = module.ReplaceScheduleBySchool(ctx, mealplan.SchoolSchedule{StudentID: student.ID, StaffAccountID: staff.ID, Weekdays: []mealplan.Weekday{mealplan.Friday}, EffectiveFrom: "2026-09-01"})
	require.ErrorIs(t, err, mealplan.ErrInvalidParticipation)

	plan, err := module.Participation(ctx, student.ID, "2026-09-09", "2026-09-10")
	require.NoError(t, err)
	assert.True(t, plan.Days[0].Participating)
	assert.True(t, plan.Days[1].Participating)

	require.NoError(t, module.ClearParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-09"}))
	plan, err = module.Participation(ctx, student.ID, "2026-09-09", "2026-09-09")
	require.NoError(t, err)
	assert.False(t, plan.Days[0].Participating, "the regular days apply again")

	closed := buildModuleWithCalendar(t, db, fixedCalendar([]string{"2026-09-11"}, nil), time.Date(2026, 9, 9, 12, 0, 0, 0, testBerlin))
	err = closed.SetParticipationBySchool(ctx, mealplan.SchoolChange{StudentID: student.ID, StaffAccountID: staff.ID, Date: "2026-09-11", Participating: true})
	require.ErrorIs(t, err, mealplan.ErrParticipationCutoff, "nobody eats on a closed day")
}
