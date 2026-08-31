package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// mealPlanSettings answers only the meal-plan toggle (and its resolve error, if
// any); the related-accounts invite mode defaults to disabled, matching the
// prior mealSettings behavior.
func mealPlanSettings(enabled bool, resolveErr error) parentSettingsStub {
	return parentSettingsStub{
		boolValues: map[string]bool{configModels.KeyMealPlanEnabled: enabled},
		boolErr:    resolveErr,
		stringValues: map[string]string{
			configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
		},
	}
}

func buildMealPlanService(t *testing.T, db *bun.DB, settings parentSettingsStub) parentService.Service {
	t.Helper()
	repos := repositories.NewFactory(db)
	enabled := settings.boolValues[configModels.KeyMealPlanEnabled]
	mealPlan := &fakeMealPlan{available: enabled, err: settings.boolErr, entries: []mealplanModule.Entry{
		{Date: mealplanModule.Date("2026-08-24"), Position: 0, Dish: "Spaghetti"},
		{Date: mealplanModule.Date("2026-08-24"), Position: 1, Dish: "Salat"},
	}}
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		MealPlan:      mealPlan,
		Settings:      settings,
		Broadcaster:   testpkg.NewRecordingBroadcaster(),
		DB:            db,
		Logger:        slog.Default(),
		Now:           func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, timezone.Berlin) },
	})
}

type fakeMealPlan struct {
	available bool
	err       error
	entries   []mealplanModule.Entry
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

func TestMealPlanWeek_ReturnsCurrentWeekEntries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := timezone.NewDate(2026, 8, 24)

	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	rows, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, currentMonday)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Spaghetti", rows[0].Dish)
	assert.Equal(t, "Salat", rows[1].Dish)
}

func TestMealPlanWeek_AllowsNextWeek(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := timezone.NewDate(2026, 8, 24)
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

	currentMonday := timezone.NewDate(2026, 8, 24)
	svc := buildMealPlanService(t, db, mealPlanSettings(false, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, currentMonday)
	require.ErrorIs(t, err, parentService.ErrMealPlanDisabled)
}

// TestMealPlanWeek_PastWeekOutOfRange asserts parents cannot reach a staff draft
// for a past week by supplying a crafted week_start.
func TestMealPlanWeek_PastWeekOutOfRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := timezone.NewDate(2026, 8, 24)
	lastWeek := currentMonday.AddDays(-7)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, lastWeek)
	require.ErrorIs(t, err, parentService.ErrMealPlanWeekOutOfRange)
}

func TestMealPlanWeek_FarFutureWeekOutOfRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := timezone.NewDate(2026, 8, 24)
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

	currentMonday := timezone.NewDate(2026, 8, 24)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID, currentMonday)
	require.Error(t, err)
	assert.NotErrorIs(t, err, parentService.ErrMealPlanDisabled)
}

func TestMealPlanWeek_SettingErrorPropagates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday := timezone.NewDate(2026, 8, 24)
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
