package mealplan_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/mealplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// cleanupMealPlan removes every meal plan row for the tenant in the given
// (inclusive) date range so the test leaves no residue.
func cleanupMealPlan(t *testing.T, repo mealplan.MealPlanEntryRepository, ctx context.Context, from, to timezone.Date) {
	t.Helper()
	for d := from; !d.After(to); d = d.AddDays(1) {
		_ = repo.DeleteByDate(ctx, d)
	}
}

func TestMealPlanRepository_ReplaceFindDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).MealPlanEntry
	ctx := testpkg.Ctx(t)

	monday := timezone.NewDate(2026, time.July, 6) // a Monday
	friday := monday.AddDays(4)
	defer cleanupMealPlan(t, repo, ctx, monday, friday)

	note := "vegetarisch"
	// Two dishes on Monday, one on Wednesday.
	require.NoError(t, repo.ReplaceDay(ctx, monday, []*mealplan.MealPlanEntry{
		{Date: monday, Position: 0, Dish: "Menü 1", Note: &note},
		{Date: monday, Position: 1, Dish: "Menü 2"},
	}))
	wed := monday.AddDays(2)
	require.NoError(t, repo.ReplaceDay(ctx, wed, []*mealplan.MealPlanEntry{
		{Date: wed, Position: 0, Dish: "Suppe"},
	}))

	// FindByDateRange returns rows ordered by (date, position).
	rows, err := repo.FindByDateRange(ctx, monday, friday)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "Menü 1", rows[0].Dish)
	assert.Equal(t, 0, rows[0].Position)
	require.NotNil(t, rows[0].Note)
	assert.Equal(t, "vegetarisch", *rows[0].Note)
	assert.Equal(t, "Menü 2", rows[1].Dish)
	assert.Equal(t, 1, rows[1].Position)
	assert.Nil(t, rows[1].Note)
	assert.Equal(t, "Suppe", rows[2].Dish)
	assert.Equal(t, testpkg.Tenant(t), rows[0].TenantID, "ReplaceDay must stamp tenant_id from context")

	// ReplaceDay is a full replace: fewer dishes overwrite the day.
	require.NoError(t, repo.ReplaceDay(ctx, monday, []*mealplan.MealPlanEntry{
		{Date: monday, Position: 0, Dish: "Auflauf"},
	}))
	rows, err = repo.FindByDateRange(ctx, monday, monday)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Auflauf", rows[0].Dish)

	// An empty slice clears the day entirely.
	require.NoError(t, repo.ReplaceDay(ctx, monday, nil))
	rows, err = repo.FindByDateRange(ctx, monday, monday)
	require.NoError(t, err)
	assert.Empty(t, rows)

	// DeleteByDate removes the remaining Wednesday entry; deleting an empty day
	// is a no-op.
	require.NoError(t, repo.DeleteByDate(ctx, wed))
	require.NoError(t, repo.DeleteByDate(ctx, wed))
	rows, err = repo.FindByDateRange(ctx, monday, friday)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// TestMealPlanRepository_DateRangeExcludesOutsideWeek proves FindByDateRange is
// an inclusive [start, end] filter: entries the day before start and the day
// after end are not returned.
func TestMealPlanRepository_DateRangeExcludesOutsideWeek(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).MealPlanEntry
	ctx := testpkg.Ctx(t)

	monday := timezone.NewDate(2026, time.August, 3) // Monday
	friday := monday.AddDays(4)
	before := monday.AddDays(-1)
	after := friday.AddDays(1)
	defer cleanupMealPlan(t, repo, ctx, before, after)

	for _, d := range []timezone.Date{before, monday, friday, after} {
		require.NoError(t, repo.ReplaceDay(ctx, d, []*mealplan.MealPlanEntry{
			{Date: d, Position: 0, Dish: "x"},
		}))
	}

	rows, err := repo.FindByDateRange(ctx, monday, friday)
	require.NoError(t, err)
	require.Len(t, rows, 2, "only Monday and Friday fall inside the inclusive range")
	assert.True(t, rows[0].Date == monday)
	assert.True(t, rows[1].Date == friday)
}

func TestMealPlanRepository_ReplaceDayZeroDateRejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).MealPlanEntry
	ctx := testpkg.Ctx(t)

	err := repo.ReplaceDay(ctx, timezone.Date{}, []*mealplan.MealPlanEntry{{Dish: "x"}})
	require.Error(t, err)
}

// TestMealPlanRepository_TenantIsolation proves the repository never returns
// another tenant's rows: an entry written under tenant 1 is invisible to a
// context scoped to a different tenant.
func TestMealPlanRepository_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).MealPlanEntry
	tenant1 := testpkg.Ctx(t)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	tenant2 := tenant.WithTenantID(context.Background(), otherTenantID)

	day := timezone.NewDate(2026, time.September, 7)
	defer cleanupMealPlan(t, repo, tenant1, day, day)
	defer cleanupMealPlan(t, repo, tenant2, day, day)

	require.NoError(t, repo.ReplaceDay(tenant1, day, []*mealplan.MealPlanEntry{
		{Date: day, Position: 0, Dish: "Tenant1 Dish"},
	}))

	// Tenant 2 sees nothing.
	rows, err := repo.FindByDateRange(tenant2, day, day)
	require.NoError(t, err)
	assert.Empty(t, rows, "tenant 2 must not see tenant 1's meal plan")

	// Tenant 1 still sees its own row.
	rows, err = repo.FindByDateRange(tenant1, day, day)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Tenant1 Dish", rows[0].Dish)
}
