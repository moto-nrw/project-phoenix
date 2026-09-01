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
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id, "person".first_name, "person".last_name, "student".school_class`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id`).
		Where(`"student".tenant_id = ?`, tenantID.Int64()).
		Where(`"student".status = 'active'`).
		Where(`("student".enrolled_from IS NULL OR "student".enrolled_from <= ?)`, date).
		Where(`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, date).
		OrderExpr(`"student".school_class, "person".last_name, "person".first_name, "student".id`).
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
	err                     error
}

func (s testMealPlanSettings) MealPlanEnabled(context.Context) (bool, error) {
	return s.mealPlanEnabled, s.err
}

func (s testMealPlanSettings) MealRegistrationEnabled(context.Context) (bool, error) {
	return s.mealRegistrationEnabled, s.err
}

func (s testMealPlanSettings) MealRegistrationCutoff(context.Context) (string, error) {
	return s.cutoff, s.err
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
	createSickStatusDay(t, db, student.GetTenantID(), student.ID, "2026-09-07", time.Date(2026, 9, 7, 8, 30, 0, 0, testBerlin))

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
	date := testDay.Format("2006-01-02")
	moduleNow := time.Date(testDay.Year(), testDay.Month(), testDay.Day(), 8, 0, 0, 0, berlin)
	module := buildModuleAt(t, db, true, nil, moduleNow)
	student := testpkg.CreateTestStudent(t, db, "Nora", "Korrektur", "1c")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-deleted-before-cutoff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: mealplan.Date(date), Participating: true,
	}))
	status := createSickStatusDay(t, db, student.GetTenantID(), student.ID, date, databaseNow)
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
	createSickStatusDay(t, db, student.GetTenantID(), student.ID, "2026-09-07", time.Date(2026, 9, 7, 9, 1, 0, 0, testBerlin))

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
	status := createSickStatusDay(t, db, student.GetTenantID(), student.ID, "2026-09-07", time.Date(2026, 9, 7, 8, 30, 0, 0, testBerlin))
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
	`, student.GetTenantID(), student.ID, "2026-09-07", changedAt, changedAt).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`
		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, student.GetTenantID(), student.ID, "2026-09-07", changedAt, changedAt, changedAt).Exec(context.Background())
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
