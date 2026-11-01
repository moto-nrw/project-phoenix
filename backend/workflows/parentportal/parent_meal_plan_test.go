package parentportal_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// mealPlanSettings answers only the meal-plan toggle (and its resolve error, if
// any); the related-accounts invite mode defaults to disabled, matching the
// prior mealSettings behavior.
func mealPlanSettings(enabled bool, resolveErr error) parentSettingsStub {
	return parentSettingsStub{
		boolValues: map[string]bool{parentService.KeyMealPlanEnabled: enabled},
		boolErr:    resolveErr,
		stringValues: map[string]string{
			parentService.KeyGuardianParentInviteMode: parentService.ParentInviteModeDisabled,
		},
	}
}

func buildMealPlanService(t *testing.T, db *bun.DB, settings parentSettingsStub, moduleOverride ...*fakeMealPlan) *parentService.Portal {
	t.Helper()
	enabled := settings.boolValues[parentService.KeyMealPlanEnabled]
	mealPlan := &fakeMealPlan{available: enabled, err: settings.boolErr, entries: []mealplanModule.Entry{
		{Date: mealplanModule.Date("2026-08-24"), Position: 0, Dish: "Spaghetti", Allergens: []mealplanModule.Allergen{mealplanModule.AllergenGluten}},
		{Date: mealplanModule.Date("2026-08-24"), Position: 1, Dish: "Salat"},
	}}
	require.LessOrEqual(t, len(moduleOverride), 1)
	if len(moduleOverride) == 1 {
		mealPlan = moduleOverride[0]
	}
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	return parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         owners.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       repos.ParentChildReads(),
		StatusDayRepo:   repos.StudentStatusDay,
		StudentRepo:     owners.NewParentStudentReads(reposPeople.Student),
		MealPlan:        mealPlan,
		Settings:        settings,
		StudentUpdates:  owners.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		Logger:          slog.Default(),
		Now:             func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, calendar.Berlin) },

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
}

type fakeMealPlan struct {
	available        bool
	err              error
	entries          []mealplanModule.Entry
	lastClearCommand *mealplanModule.SetParticipationDay
	lastWeekCommand  *mealplanModule.SetParticipationWeek
}

func availableMealPlan(enabled bool) *fakeMealPlan { return &fakeMealPlan{available: enabled} }

func (f *fakeMealPlan) Available(context.Context) (bool, error) { return f.available, f.err }
func (f *fakeMealPlan) Week(context.Context, mealplanModule.Date) ([]mealplanModule.Entry, error) {
	if f.err != nil {
		return nil, f.err
	}
	if !f.available {
		return nil, mealplanModule.ErrDisabled
	}
	return f.entries, nil
}
func (f *fakeMealPlan) RegistrationAvailable(context.Context) (bool, error) {
	return f.available, f.err
}
func (f *fakeMealPlan) Participation(context.Context, int64, mealplanModule.Date, mealplanModule.Date) (mealplanModule.ParticipationPlan, error) {
	return mealplanModule.ParticipationPlan{}, f.err
}
func (f *fakeMealPlan) ReplaceParticipationSchedule(context.Context, mealplanModule.ReplaceParticipationSchedule) (mealplanModule.Date, error) {
	return "2026-09-07", f.err
}
func (f *fakeMealPlan) SetParticipationForDay(context.Context, mealplanModule.SetParticipationDay) error {
	return f.err
}

func (f *fakeMealPlan) SetParticipationForWeek(_ context.Context, command mealplanModule.SetParticipationWeek) (mealplanModule.WeekChange, error) {
	f.lastWeekCommand = &command
	return mealplanModule.WeekChange{Changed: []mealplanModule.Date{command.WeekOf}}, f.err
}

func (f *fakeMealPlan) ClearParticipationForDay(_ context.Context, command mealplanModule.SetParticipationDay) error {
	f.lastClearCommand = &command
	return f.err
}

func TestMealPlanWeek_ReturnsCurrentWeekEntries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := calendar.NewDate(2026, 8, 24)

	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	rows, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, currentMonday)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Spaghetti", rows[0].Dish)
	assert.Equal(t, []string{"gluten"}, rows[0].Allergens, "the parent sees the dish's allergens")
	assert.Equal(t, "Salat", rows[1].Dish)
	assert.Empty(t, rows[1].Allergens)
}

func TestMealPlanWeek_AllowsNextWeek(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := calendar.NewDate(2026, 8, 24)
	nextMonday := currentMonday.AddDays(7)

	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	rows, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, nextMonday)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestMealPlanWeek_DisabledReturnsSentinel(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := calendar.NewDate(2026, 8, 24)
	svc := buildMealPlanService(t, db, mealPlanSettings(false, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, currentMonday)
	require.ErrorIs(t, err, parentService.ErrMealPlanDisabled)
}

func TestMealParticipationWrites_CareEndedChildIsRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	endCareFor(t, db, chain.StudentID)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err := svc.ReplaceMealParticipationSchedule(ctx, chain.AccountID, chain.StudentID, []parentService.MealWeekday{1})
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)

	err = svc.SetMealParticipationDay(ctx, chain.AccountID, chain.StudentID, calendar.NewDate(2026, 8, 25), true)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)

	err = svc.ClearMealParticipationDay(ctx, chain.AccountID, chain.StudentID, calendar.NewDate(2026, 8, 25))
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)

	_, err = svc.SetMealParticipationWeek(ctx, chain.AccountID, chain.StudentID, calendar.NewDate(2026, 8, 31), false)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)
}

func TestMealParticipationWrites_RequireManagePermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE auth.guardian_student_access AS a
		SET permissions = '{"parent_portal.access": true}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	date := calendar.NewDate(2026, 8, 25)

	_, err = svc.MealParticipation(ctx, chain.AccountID, chain.StudentID, date, date)
	require.NoError(t, err, "portal access still permits reading meal participation")

	_, err = svc.ReplaceMealParticipationSchedule(ctx, chain.AccountID, chain.StudentID, []parentService.MealWeekday{1})
	require.EqualError(t, err, "parent: meal participation management not permitted")

	err = svc.SetMealParticipationDay(ctx, chain.AccountID, chain.StudentID, date, true)
	require.EqualError(t, err, "parent: meal participation management not permitted")

	err = svc.ClearMealParticipationDay(ctx, chain.AccountID, chain.StudentID, date)
	require.EqualError(t, err, "parent: meal participation management not permitted")

	_, err = svc.SetMealParticipationWeek(ctx, chain.AccountID, chain.StudentID, date, false)
	require.EqualError(t, err, "parent: meal participation management not permitted")

	flags, err := svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, flags.MealRegistrationEnabled)
}

func TestClearMealParticipationDay_PassesGuardianAccountID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	mealPlan := availableMealPlan(true)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil), mealPlan)
	date := calendar.NewDate(2026, 8, 25)

	err := svc.ClearMealParticipationDay(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, date)
	require.NoError(t, err)
	require.NotNil(t, mealPlan.lastClearCommand)
	assert.Equal(t, chain.AccountID, mealPlan.lastClearCommand.GuardianAccountID)
	assert.Equal(t, chain.StudentID, mealPlan.lastClearCommand.StudentID)
	assert.Equal(t, mealplanModule.Date(date.String()), mealPlan.lastClearCommand.Date)
}

// TestMealPlanWeek_PastWeekOutOfRange asserts parents cannot reach a staff draft
// for a past week by supplying a crafted week_start.
func TestMealPlanWeek_PastWeekOutOfRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := calendar.NewDate(2026, 8, 24)
	lastWeek := currentMonday.AddDays(-7)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, lastWeek)
	require.ErrorIs(t, err, parentService.ErrMealPlanWeekOutOfRange)
}

func TestMealPlanWeek_FarFutureWeekOutOfRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := calendar.NewDate(2026, 8, 24)
	weekAfterNext := currentMonday.AddDays(14)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, weekAfterNext)
	require.ErrorIs(t, err, parentService.ErrMealPlanWeekOutOfRange)
}

func TestMealPlanWeek_NotOwnedChildRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")

	currentMonday := calendar.NewDate(2026, 8, 24)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID, currentMonday)
	require.Error(t, err)
	assert.NotErrorIs(t, err, parentService.ErrMealPlanDisabled)
}

func TestMealPlanWeek_SettingErrorPropagates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := calendar.NewDate(2026, 8, 24)
	svc := buildMealPlanService(t, db, mealPlanSettings(false, errors.New("settings down")))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, currentMonday)
	require.Error(t, err)
	assert.NotErrorIs(t, err, parentService.ErrMealPlanDisabled)
}

// TestChildFeatures_ReflectsMealPlanSetting verifies the meal-plan flag on the
// aggregate feature response tracks the tenant setting.
func TestChildFeatures_ReflectsMealPlanSetting(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	svcOn := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	flags, err := svcOn.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.MealPlanEnabled)

	svcOff := buildMealPlanService(t, db, mealPlanSettings(false, nil))
	flags, err = svcOff.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, flags.MealPlanEnabled)
}

func TestMealParticipationRejectsChildWithoutGuardianAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))

	_, err := svc.MealParticipation(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID, calendar.NewDate(2026, 8, 24), calendar.NewDate(2026, 9, 4))
	require.Error(t, err)
	assert.NotErrorIs(t, err, parentService.ErrMealRegistrationDisabled)
}

func TestSetMealParticipationWeek_AnchorsOnMondayWithinRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	mealPlan := availableMealPlan(true)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil), mealPlan)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	// Today is Monday 2026-08-24; the parent reaches four weeks.
	change, err := svc.SetMealParticipationWeek(ctx, chain.AccountID, chain.StudentID, calendar.NewDate(2026, 9, 17), false)
	require.NoError(t, err)
	require.NotNil(t, mealPlan.lastWeekCommand)
	assert.Equal(t, mealplanModule.Date("2026-09-14"), mealPlan.lastWeekCommand.WeekOf)
	assert.Equal(t, chain.AccountID, mealPlan.lastWeekCommand.GuardianAccountID)
	assert.False(t, mealPlan.lastWeekCommand.Participating)
	assert.Equal(t, []string{"2026-09-14"}, change.Changed)

	_, err = svc.SetMealParticipationWeek(ctx, chain.AccountID, chain.StudentID, calendar.NewDate(2026, 9, 21), false)
	require.ErrorIs(t, err, parentService.ErrMealParticipationOutOfRange, "the fifth week is out of reach")
	_, err = svc.SetMealParticipationWeek(ctx, chain.AccountID, chain.StudentID, calendar.NewDate(2026, 8, 21), false)
	require.ErrorIs(t, err, parentService.ErrMealParticipationOutOfRange, "a past week is out of reach")
}
