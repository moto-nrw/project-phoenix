package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The database rejects impossible dates even when writes bypass the HTTP
// validator. February 29 is excluded because the carryover date must exist in
// every following year.
func TestAbsenceAllowanceCarryoverConstraintRejectsInvalidCalendarDays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.OwnTenantRows(t, db, tenantID)

	for _, carryoverUntil := range []string{"02-29", "02-30", "02-31", "04-31", "11-31"} {
		_, err := db.NewRaw(`
			INSERT INTO active.staff_absence_types
				(tenant_id, name, base_type, is_active, carryover_until)
			VALUES (?, ?, 'other', TRUE, ?)
		`, tenantID, "Ungültiger Verfall "+carryoverUntil, carryoverUntil).Exec(context.Background())
		require.Error(t, err, "%s must not pass the carryover date constraint", carryoverUntil)
	}

	_, err := db.NewRaw(`
		INSERT INTO active.staff_absence_types
			(tenant_id, name, base_type, is_active, carryover_until)
		VALUES (?, 'Gültiger Verfall', 'other', TRUE, '02-28')
	`, tenantID).Exec(context.Background())
	require.NoError(t, err)
}
