package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestWeekdayRosterPrimarySupervisorIsScopedByCalendarPeriod(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)

	ctx := context.Background()
	require.NoError(t, rosterWeekdayScopeUp(ctx, db))

	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "RosterPrimaryPeriod")
	staffA := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Primary", "PeriodA")
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Primary", "PeriodB")
	staffC := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Replacement", "PeriodA")
	periodA := insertRosterUniquenessPeriod(t, db, tenantID, "Primary A")
	periodB := insertRosterUniquenessPeriod(t, db, tenantID, "Primary B")

	insertPrimarySupervisor(t, db, tenantID, group.ID, staffA.ID, periodA, 1)
	insertPrimarySupervisor(t, db, tenantID, group.ID, staffB.ID, periodB, 1)

	assert.True(t, supervisorIsPrimary(t, db, tenantID, group.ID, staffA.ID, periodA, 1))
	assert.True(t, supervisorIsPrimary(t, db, tenantID, group.ID, staffB.ID, periodB, 1),
		"a primary in another calendar period must remain set")

	insertPrimarySupervisor(t, db, tenantID, group.ID, staffC.ID, periodA, 1)

	assert.False(t, supervisorIsPrimary(t, db, tenantID, group.ID, staffA.ID, periodA, 1),
		"a replacement in the same period and weekday must clear the old primary")
	assert.True(t, supervisorIsPrimary(t, db, tenantID, group.ID, staffB.ID, periodB, 1),
		"replacing one period's primary must not affect another period")
	assert.True(t, supervisorIsPrimary(t, db, tenantID, group.ID, staffC.ID, periodA, 1))
}

func insertPrimarySupervisor(
	t *testing.T,
	db *bun.DB,
	tenantID, groupID, staffID, periodID int64,
	weekday int,
) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO activities.supervisors (
			tenant_id, staff_id, group_id, is_primary, valid_from, calendar_period_id, weekday
		)
		VALUES (?, ?, ?, true, '2026-01-01', ?, ?)
	`, tenantID, staffID, groupID, periodID, weekday).Exec(context.Background())
	require.NoError(t, err)
}

func supervisorIsPrimary(
	t *testing.T,
	db *bun.DB,
	tenantID, groupID, staffID, periodID int64,
	weekday int,
) bool {
	t.Helper()
	var primary bool
	err := db.NewRaw(`
		SELECT is_primary
		FROM activities.supervisors
		WHERE tenant_id = ?
		  AND group_id = ?
		  AND staff_id = ?
		  AND calendar_period_id = ?
		  AND weekday = ?
		  AND valid_until IS NULL
	`, tenantID, groupID, staffID, periodID, weekday).Scan(context.Background(), &primary)
	require.NoError(t, err)
	return primary
}
