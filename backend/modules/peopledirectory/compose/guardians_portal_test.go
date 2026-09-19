package compose

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentsWithPortalGuardianNeedsAccountAndPortalAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	// A guardian with an account and portal access.
	_, _, reachable, _ := guardianRows(t, db, tenantID, "primary_guardian")
	// An account alone is not enough: pickup_only carries no portal access.
	_, _, pickupOnly, _ := guardianRows(t, db, tenantID, "pickup_only")
	// Portal access without an account: the guardian was never invited.
	noAccount := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Nora", "Directory", "2a")
	uninvited := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Uwe", "Directory", "directory-uninvited")
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, noAccount.ID, uninvited.ID, "primary_guardian")
	// No guardian at all.
	alone := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Omar", "Directory", "3a")

	result, err := module.StudentsWithPortalGuardian(ctx, []int64{reachable, pickupOnly, noAccount.ID, alone.ID, reachable, 0})
	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{reachable: true}, result, "children without a portal guardian are absent")

	empty, err := module.StudentsWithPortalGuardian(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestStudentsWithPortalGuardianStaysInsideTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	_, _, foreign, _ := guardianRows(t, db, otherTenant, "primary_guardian")

	result, err := module.StudentsWithPortalGuardian(testpkg.Ctx(t), []int64{foreign})
	require.NoError(t, err)
	assert.Empty(t, result, "another school's child is not visible")
}
