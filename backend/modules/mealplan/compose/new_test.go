package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func buildModule(t *testing.T, db *bun.DB, enabled bool, settingErr error) *mealplan.Module {
	t.Helper()
	module, err := New(Dependencies{
		DB: db,
		Settings: SettingsFunc(func(context.Context) (bool, error) {
			return enabled, settingErr
		}),
		Observe: func(Observation) {},
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
		Settings: SettingsFunc(func(context.Context) (bool, error) { return true, nil }),
		Observe:  func(observation Observation) { observations = append(observations, observation) },
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
