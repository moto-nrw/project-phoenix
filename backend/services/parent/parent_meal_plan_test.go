package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	mealplanModels "github.com/moto-nrw/project-phoenix/models/mealplan"
	mealplanService "github.com/moto-nrw/project-phoenix/services/mealplan"
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
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		MealPlanRepo:  repos.MealPlanEntry,
		Settings:      settings,
		Broadcaster:   testpkg.NewRecordingBroadcaster(),
		DB:            db,
		Logger:        slog.Default(),
	})
}

// seedMealPlanWeek writes dishes for the Monday of the given week under tenant 1.
func seedMealPlanWeek(t *testing.T, db *bun.DB, monday timezone.Date) {
	t.Helper()
	repo := repositories.NewFactory(db).MealPlanEntry
	ctx := testpkg.Ctx(t)
	require.NoError(t, repo.ReplaceDay(ctx, monday, []*mealplanModels.MealPlanEntry{
		{Date: monday, Position: 0, Dish: "Spaghetti"},
		{Date: monday, Position: 1, Dish: "Salat"},
	}))
	t.Cleanup(func() { _ = repo.DeleteByDate(ctx, monday) })
}

func TestMealPlanWeek_ReturnsCurrentWeekEntries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
	seedMealPlanWeek(t, db, currentMonday)

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

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
	nextMonday := currentMonday.AddDays(7)
	seedMealPlanWeek(t, db, nextMonday)

	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	rows, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, nextMonday)
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestMealPlanWeek_DisabledReturnsSentinel(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
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

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
	lastWeek := currentMonday.AddDays(-7)
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, lastWeek)
	require.ErrorIs(t, err, parentService.ErrMealPlanWeekOutOfRange)
}

func TestMealPlanWeek_FarFutureWeekOutOfRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
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

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
	svc := buildMealPlanService(t, db, mealPlanSettings(true, nil))
	_, err := svc.MealPlanWeek(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID, currentMonday)
	require.Error(t, err)
	assert.NotErrorIs(t, err, parentService.ErrMealPlanDisabled)
}

func TestMealPlanWeek_SettingErrorPropagates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
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
