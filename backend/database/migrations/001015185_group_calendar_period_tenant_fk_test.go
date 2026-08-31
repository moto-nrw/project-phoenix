// Verifies the tenant-safe FK installed by migration 1.15.185.
package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupCalendarPeriodTenantFKRejectsCrossTenantReference(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.OwnTenantRows(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.OwnTenantRows(t, db, tenantB)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, err := db.NewDelete().TableExpr("schedule.calendar_periods").Where("tenant_id IN (?)", testpkg.DBList([]int64{tenantA, tenantB})).Exec(cleanupCtx)
		require.NoError(t, err)
		// School + organization stay: their IDs come from the test band, so
		// they are not leftovers (#2419).
	})

	periodA := insertCalendarPeriodForTenantFKTest(t, db, tenantA, "A")
	periodB := insertCalendarPeriodForTenantFKTest(t, db, tenantB, "B")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantA, "Tenant-period-FK")

	require.NoError(t, setGroupCalendarPeriodForTenantFKTest(db, tenantA, group.ID, periodA))
	err := setGroupCalendarPeriodForTenantFKTest(db, tenantA, group.ID, periodB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fk_activities_groups_calendar_period_tenant")
}

func insertCalendarPeriodForTenantFKTest(t *testing.T, db *testpkg.DB, tenantID int64, suffix string) int64 {
	t.Helper()
	var periodID int64
	err := db.NewRaw(`
		INSERT INTO schedule.calendar_periods (
			tenant_id, name, period_type, start_date, end_date,
			week_cycle_length, is_active
		)
		VALUES (?, ?, 'custom', '2026-08-01', '2027-07-31', 1, TRUE)
		RETURNING id
	`, tenantID, fmt.Sprintf("Tenant-FK-%s-%d", suffix, tenantID)).Scan(context.Background(), &periodID)
	require.NoError(t, err)
	return periodID
}

func setGroupCalendarPeriodForTenantFKTest(db *testpkg.DB, tenantID, groupID, periodID int64) error {
	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET calendar_period_id = ?
		WHERE tenant_id = ? AND id = ?
	`, periodID, tenantID, groupID).Exec(context.Background())
	return err
}
