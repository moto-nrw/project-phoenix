package inbound

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomAbsenceAllowanceIsTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.NewTenantScope(t, db)
	tenantB := testpkg.NewTenantScope(t, db)
	repos, err := repositories.NewWorkforceTestRepositories(db, repositories.NewTestAuditStore(db))
	require.NoError(t, err)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantA.TenantID, "Rena", "Mandant A")
	admin := testpkg.CreateTestStaffForTenant(t, db, tenantA.TenantID, "Lea", "Mandant A")
	svc := repos.StaffAbsenceType
	absenceType, err := svc.CreateAbsenceType(tenantA.Context(), workforce.CreateAbsenceType{
		Name: "Mandantentag", AllowanceEnabled: true, OverrunPolicy: workforce.AbsenceTypeOverrunBlock,
	})
	require.NoError(t, err)
	_, err = svc.SetAllowance(tenantA.Context(), workforce.SetAbsenceTypeAllowance{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 2, Reason: "Anspruch Mandant A", ChangedBy: admin.ID,
	})
	require.NoError(t, err)
	allowances, err := repos.StaffAbsenceTypeAllowance.List(tenantA.Context(), nil)
	require.NoError(t, err)
	require.Len(t, allowances, 1)
	assert.Equal(t, 2.0, allowances[0].EntitledDays)
	changes, err := repos.StaffAbsenceTypeAllowanceChange.List(tenantA.Context(), nil)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	assert.Equal(t, "Anspruch Mandant A", changes[0].Reason)
	assert.Equal(t, admin.ID, changes[0].ChangedBy)
	// Read all rows in B, not just the known A identifiers: neither the claim
	// nor its audit may cross the tenant boundary.
	allowances, err = repos.StaffAbsenceTypeAllowance.List(tenantB.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, allowances, "tenant B must not see tenant A's allowance")
	changes, err = repos.StaffAbsenceTypeAllowanceChange.List(tenantB.Context(), nil)
	require.NoError(t, err)
	assert.Empty(t, changes, "tenant B must not see tenant A's allowance audit")
}
