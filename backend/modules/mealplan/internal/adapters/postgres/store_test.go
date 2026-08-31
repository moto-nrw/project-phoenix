package postgres

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStoreScopesQueriesToTenantWithoutRLS(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	weekDate, err := domain.ParseDate("2026-09-07")
	require.NoError(t, err)
	clearDate, err := domain.ParseDate("2026-09-08")
	require.NoError(t, err)
	rows := []row{
		{TenantID: tenantID, Date: weekDate, Dish: "Tenant A"},
		{TenantID: otherTenantID, Date: weekDate, Dish: "Tenant B"},
		{TenantID: tenantID, Date: clearDate, Dish: "Tenant A"},
		{TenantID: otherTenantID, Date: clearDate, Dish: "Tenant B"},
	}
	_, err = db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_plan_entries`).Exec(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewDelete().Model((*row)(nil)).
			ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
			Where(`"meal_plan_entry".tenant_id IN (?)`, bun.List([]int64{tenantID, otherTenantID})).
			Where(`"meal_plan_entry".date >= ? AND "meal_plan_entry".date <= ?`, weekDate, clearDate).
			Exec(ctx)
	})

	store := New(func(context.Context) (bun.IDB, int64, error) { return db, tenantID, nil })
	entries, _, err := store.FindWeek(ctx, weekDate, weekDate)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "Tenant A", entries[0].Dish)

	deleted, _, _, err := store.ClearDay(ctx, clearDate)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)

	count, err := db.NewSelect().
		TableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".tenant_id = ?`, otherTenantID).
		Where(`"meal_plan_entry".date = ?`, clearDate).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
