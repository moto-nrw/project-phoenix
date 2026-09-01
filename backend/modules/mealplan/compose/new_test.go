package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

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
	module, err := New(Dependencies{
		DB:       db,
		Settings: testMealPlanSettings{mealPlanEnabled: enabled, mealRegistrationEnabled: enabled, cutoff: "09:00", err: settingErr},
		Observe:  func(Observation) {},
		Now:      func() time.Time { return time.Date(2026, 9, 7, 8, 0, 0, 0, timezone.Berlin) },
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
		DB:       db,
		Settings: enabledTestSettings(nil),
		Observe:  func(observation Observation) { observations = append(observations, observation) },
		Now:      func() time.Time { return time.Date(2026, 9, 28, 8, 0, 0, 0, timezone.Berlin) },
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
		Observe: func(Observation) {},
		Now:     func() time.Time { return time.Date(2026, 9, 7, 9, 1, 0, 0, timezone.Berlin) },
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
		Observe: func(Observation) {},
		Now:     func() time.Time { return time.Date(2026, 9, 7, 10, 0, 0, 0, timezone.Berlin) },
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
		Observe: func(Observation) {},
		Now:     func() time.Time { return time.Date(2026, 9, 7, 10, 0, 0, 0, timezone.Berlin) },
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
	status := testpkg.CreateTestStudentStatusDay(t, db, student.ID, timezone.Date("2026-09-07"), "sick")
	_, err := db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`reported_at = ?`, time.Date(2026, 9, 7, 8, 30, 0, 0, timezone.Berlin)).
		WherePK().Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Empty(t, list.Participants)
}

func TestDailyListIncludesChildWhenSicknessIsDeletedBeforeCutoff(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db, true, nil)
	student := testpkg.CreateTestStudent(t, db, "Nora", "Korrektur", "1c")
	account := testpkg.CreateTestAccount(t, db, "meal-sick-deleted-before-cutoff")
	ctx := testpkg.Ctx(t)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{
		StudentID: student.ID, GuardianAccountID: account.ID, Date: "2026-09-07", Participating: true,
	}))
	status := testpkg.CreateTestStudentStatusDay(t, db, student.ID, timezone.Date("2026-09-07"), "sick")
	_, err := db.NewDelete().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		WherePK().Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	require.Len(t, list.Participants, 1)
	assert.Equal(t, student.ID, list.Participants[0].StudentID)
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
	status := &activeModels.StudentStatusDay{
		StudentID: student.ID, Date: timezone.Date("2026-09-07"), Status: activeModels.StudentStatusDaySick,
		ReportedAt: time.Date(2026, 9, 7, 9, 1, 0, 0, timezone.Berlin), Source: activeModels.StudentStatusSourceManual,
	}
	status.SetTenantID(student.GetTenantID())
	_, err := db.NewInsert().Model(status).ModelTableExpr(`active.student_status_days`).Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	require.Len(t, list.Participants, 1)
	assert.Equal(t, student.ID, list.Participants[0].StudentID)
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
	status := testpkg.CreateTestStudentStatusDay(t, db, student.ID, timezone.Date("2026-09-07"), "sick")
	_, err := db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`reported_at = ?`, time.Date(2026, 9, 7, 8, 30, 0, 0, timezone.Berlin)).
		Set(`cleared_at = ?`, time.Date(2026, 9, 7, 9, 30, 0, 0, timezone.Berlin)).
		WherePK().Exec(context.Background())
	require.NoError(t, err)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Empty(t, list.Participants)

	_, err = db.NewUpdate().Model(status).
		ModelTableExpr(`active.student_status_days AS "student_status_day"`).
		Set(`reported_at = ?`, time.Date(2026, 9, 7, 10, 30, 0, 0, timezone.Berlin)).
		Set(`cleared_at = NULL`).
		WherePK().Exec(context.Background())
	require.NoError(t, err)
	list, err = module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	assert.Empty(t, list.Participants)
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
	changedAt := time.Date(2026, 9, 7, 8, 30, 0, 0, timezone.Berlin)
	_, err := db.NewRaw(`
		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		VALUES (?, ?, ?, ?, ?, NULL)
	`, student.GetTenantID(), student.ID, timezone.Date("2026-09-07"), changedAt, changedAt).Exec(context.Background())
	require.NoError(t, err)
	_, err = db.NewRaw(`
		INSERT INTO schedule.meal_sickness_status_history
			(tenant_id, student_id, date, changed_at, reported_at, cleared_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, student.GetTenantID(), student.ID, timezone.Date("2026-09-07"), changedAt, changedAt, changedAt).Exec(context.Background())
	require.NoError(t, err)

	plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-07")
	require.NoError(t, err)
	require.Len(t, plan.Days, 1)
	assert.True(t, plan.Days[0].Participating)
	assert.Equal(t, mealplan.ParticipationOverride, plan.Days[0].Source)

	list, err := module.DailyList(ctx, "2026-09-07")
	require.NoError(t, err)
	require.Len(t, list.Participants, 1)
	assert.Equal(t, student.ID, list.Participants[0].StudentID)
}
