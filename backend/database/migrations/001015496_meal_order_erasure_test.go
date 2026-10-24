package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #3873: erasing a school removes its meal orders, registration changes,
// caterer invitations and supplies, and keeps the caterer business, its
// accounts and the supply of another school.
func TestMealOrderErasureRemovesTheSchoolsOrdersAndKeepsTheCaterer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	var catererID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES ('Erasure Caterer') RETURNING id`).Scan(t.Context(), &catererID))
	erasureExec(t, db, `INSERT INTO auth.caterer_schools (caterer_id, tenant_id) VALUES (?, ?), (?, ?)`, catererID, schoolID, catererID, otherSchool)
	erasureExec(t, db, `INSERT INTO schedule.meal_orders (tenant_id, period_start, period_end, deadline_at) VALUES (?, '2026-11-02', '2026-11-02', '2026-11-02 09:00+01')`, schoolID)

	var preview []byte
	operator := testpkg.CreateTestOperator(t, db)
	require.NoError(t, db.NewRaw(`SELECT platform.school_erasure_preview(?, ?)::text`, schoolID, operator.ID).Scan(t.Context(), &preview))
	assert.Contains(t, string(preview), `"auth.caterer_schools": 1`)
	assert.Contains(t, string(preview), `"schedule.meal_orders": 1`)

	orderID := beginErasure(t, db, schoolID)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
	var remaining struct{ Caterers, Supplies, Orders int }
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM auth.caterers WHERE id = ?) AS caterers,
		(SELECT count(*) FROM auth.caterer_schools WHERE caterer_id = ?) AS supplies,
		(SELECT count(*) FROM schedule.meal_orders WHERE tenant_id = ?) AS orders`, catererID, catererID, schoolID).Scan(t.Context(), &remaining))
	assert.Equal(t, struct{ Caterers, Supplies, Orders int }{Caterers: 1, Supplies: 1, Orders: 0}, remaining)
}
