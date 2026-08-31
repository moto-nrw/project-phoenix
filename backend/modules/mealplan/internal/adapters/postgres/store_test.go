package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestStoreScopesReadsAndDeletesWithoutRLS(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// The test pool is the postgres superuser, so RLS cannot hide a missing
	// application predicate and make this defense-in-depth check pass.
	tenantID := testpkg.Tenant(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	date, err := domain.ParseDate("2026-09-07")
	require.NoError(t, err)

	rows := []row{
		{TenantID: tenantID, Date: date, Position: 0, Dish: "Tenant A"},
		{TenantID: otherTenantID, Date: date, Position: 0, Dish: "Tenant B"},
	}
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_plan_entries`).Exec(context.Background())
	require.NoError(t, err)

	store := New(func(context.Context) (bun.IDB, int64, error) {
		return db, tenantID, nil
	})
	entries, _, err := store.FindWeek(context.Background(), date, date)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Tenant A", entries[0].Dish)

	_, err = store.ClearDay(context.Background(), date)
	require.NoError(t, err)

	var remaining int
	remaining, err = db.NewSelect().Model((*row)(nil)).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".date = ?`, date).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, remaining)

	var survivor row
	err = db.NewSelect().Model(&survivor).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".date = ?`, date).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, otherTenantID, survivor.TenantID)
	assert.Equal(t, "Tenant B", survivor.Dish)
}

func TestMain(m *testing.M) {
	testpkg.PerTestTenants()
	testpkg.Run(m)
}
