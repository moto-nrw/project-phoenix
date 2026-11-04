package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestSurveyErasureAfterMealsRepairsLateSurveyMigration replays the order of
// a database that applied 1.15.605 first: the late 1.15.568 reinstalls
// development's erasure functions without the meal tables, and 1.15.608
// restores a plan with the meal and the survey tables under which a whole
// school erasure succeeds.
func TestSurveyErasureAfterMealsRepairsLateSurveyMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	// Roll back the holiday guests first (#4666): their tables and foreign
	// keys extend the erasure functions the older migrations restore.
	rollBackHolidayGuests(t, db)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := t.Context()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	erasureExec(t, db, `INSERT INTO schedule.meal_orders (tenant_id, period_start, period_end, deadline_at) VALUES (?, '2026-11-02', '2026-11-02', '2026-11-02 09:00+01')`, school)
	planNames := func() []string {
		t.Helper()
		names := make([]string, 0)
		for _, entry := range readErasurePlan(t, db) {
			names = append(names, entry.Name)
		}
		return names
	}
	complete := planNames()
	require.Contains(t, complete, "schedule.meal_orders")
	require.Contains(t, complete, "users.surveys")

	// The erasure step of 1.15.568, run after 1.15.605.
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := installSchoolErasureFunctions(ctx, tx); err != nil {
			return err
		}
		return installSchoolErasurePreview(ctx, tx)
	}))
	require.NotContains(t, planNames(), "schedule.meal_orders", "the late survey migration drops the meal tables")

	require.NoError(t, surveyErasureAfterMealsUp(ctx, db))
	assert.Equal(t, complete, planNames())
	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	require.Equal(t, "db_done", result.Status, "the repaired functions erase the school: %v", result.ErrorCode)
	var orders int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM schedule.meal_orders WHERE tenant_id = ?`, school).Scan(ctx, &orders))
	assert.Zero(t, orders)
}
