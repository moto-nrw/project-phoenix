package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestMealPlanAllergensAcceptOnlyTheMainAllergens(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	tenantID := testpkg.Tenant(t)

	// A dish saved before the marking existed stays valid with an empty list.
	var marked int
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.meal_plan_entries (tenant_id, date, position, dish)
		VALUES (?, '2026-09-07', 0, 'Nudeln') RETURNING cardinality(allergens)`, tenantID).Scan(ctx, &marked))
	require.Zero(t, marked)

	_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_plan_entries (tenant_id, date, position, dish, allergens)
		VALUES (?, '2026-09-07', 1, 'Fischstäbchen', ARRAY['fish', 'gluten', 'eggs']::TEXT[])`, tenantID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_plan_entries (tenant_id, date, position, dish, allergens)
		VALUES (?, '2026-09-07', 2, 'Erdbeeren', ARRAY['strawberry']::TEXT[])`, tenantID)
	require.ErrorContains(t, err, "chk_meal_plan_entries_allergens")
}
