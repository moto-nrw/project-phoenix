package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffBalanceAdjustmentTenantFKsRejectCrossTenantReferences(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	deciderA := testpkg.CreateTestStaffForTenant(t, db, tenantA, "Tenant", "A")
	staffB := testpkg.CreateTestStaffForTenant(t, db, tenantB, "Tenant", "B")
	t.Cleanup(func() {
		testpkg.CleanupStaffFixtures(t, db, deciderA.ID, staffB.ID)
		_, err := db.Exec("DELETE FROM platform.schools WHERE id IN (?, ?)", tenantA, tenantB)
		require.NoError(t, err)
		_, err = db.Exec("DELETE FROM platform.organizations WHERE id IN (?, ?)", tenantA, tenantB)
		require.NoError(t, err)
	})

	_, err := db.NewRaw(`
		INSERT INTO active.staff_balance_adjustments (
			tenant_id, staff_id, type, minutes_delta, effective_date,
			note, decided_by, decided_at
		)
		VALUES (?, ?, 'payout', -60, ?, 'cross-tenant target', ?, ?)
	`,
		tenantA,
		staffB.ID,
		timezone.TodayDate(),
		deciderA.ID,
		time.Now(),
	).Exec(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fk_sba_staff_tenant")

	_, err = db.NewRaw(`
		INSERT INTO active.staff_balance_adjustments (
			tenant_id, staff_id, type, minutes_delta, effective_date,
			note, decided_by, decided_at
		)
		VALUES (?, ?, 'payout', -60, ?, 'cross-tenant decider', ?, ?)
	`,
		tenantA,
		deciderA.ID,
		timezone.TodayDate(),
		staffB.ID,
		time.Now(),
	).Exec(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fk_sba_decided_by_tenant")
}
