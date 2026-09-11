package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// guardianRows seeds one guardian with an account and one link in the given
// tenant and returns (accountID, guardianID, studentID, linkID).
func guardianRows(t *testing.T, db *bun.DB, tenantID int64, role string) (int64, int64, int64, int64) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "directory-guardian")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Sabine", "Directory", "directory-guardian")
	_, err := db.NewUpdate().TableExpr("users.guardian_profiles").
		Set("account_id = ?", account.ID).Set("has_account = TRUE").
		Where("id = ?", guardian.ID).Exec(context.Background())
	require.NoError(t, err)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Felix", "Directory", "1a")
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, guardian.ID, role)
	return account.ID, guardian.ID, student.ID, link.ID
}

func TestGuardianDirectoryReadsLinksAndProfilesOfTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	accountID, guardianID, studentID, linkID := guardianRows(t, db, testpkg.Tenant(t), "primary_guardian")
	unlinked := testpkg.CreateTestGuardianProfileNamed(t, db, "Nobody", "Linked", "directory-unlinked")

	links, err := module.ListGuardianLinksByAccount(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, linkID, links[0].ID)
	assert.Equal(t, studentID, links[0].StudentID)
	assert.Equal(t, guardianID, links[0].GuardianProfileID)
	assert.Equal(t, testpkg.Tenant(t), links[0].TenantID)
	assert.True(t, links[0].HasPermission(peopledirectory.GuardianPermissionPortalAccess), "the primary guardian preset grants portal access")
	assert.True(t, links[0].IsPrimary || links[0].CanPickup, "the link flags travel with the row")

	guardians, err := module.ListGuardiansByAccount(ctx, []int64{accountID, 0})
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	assert.Equal(t, guardianID, guardians[0].ID)
	require.NotNil(t, guardians[0].AccountID)
	assert.Equal(t, accountID, *guardians[0].AccountID)
	assert.Equal(t, "Sabine", guardians[0].FirstName)

	byID, err := module.ListGuardiansByID(ctx, []int64{guardianID, unlinked.ID, guardianID})
	require.NoError(t, err)
	assert.Len(t, byID, 2)

	counts, err := module.CountGuardianLinks(ctx, []int64{guardianID, unlinked.ID})
	require.NoError(t, err)
	assert.Equal(t, map[int64]int{guardianID: 1}, counts, "a guardian without links is absent from the count")
}

func TestGuardianDirectoryRestrictedRolesGrantNoPortalAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	accountID, _, _, _ := guardianRows(t, db, testpkg.Tenant(t), "pickup_only")

	links, err := module.ListGuardianLinksByAccount(testpkg.Ctx(t), accountID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Empty(t, links[0].Permissions, "a pickup-only contact holds no parents-portal permission")
	assert.Equal(t, "pickup_only", links[0].GuardianRole)
}

func TestGuardianDirectoryTenantIsolationAndAdminReads(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	otherCtx, otherTenantID := otherTenantContext(t, db)
	accountID, guardianID, _, _ := guardianRows(t, db, testpkg.Tenant(t), "primary_guardian")
	otherGuardian := testpkg.CreateTestGuardianProfileForTenant(t, db, otherTenantID, "Other", "School", "directory-other")
	_, err := db.NewUpdate().TableExpr("users.guardian_profiles").
		Set("account_id = ?", accountID).Set("has_account = TRUE").
		Where("id = ?", otherGuardian.ID).Exec(context.Background())
	require.NoError(t, err)
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Other", "Child", "2b")
	testpkg.CreateTestStudentGuardianLinkForTenant(t, db, otherTenantID, otherStudent.ID, otherGuardian.ID, "co_guardian")

	// Inside the other tenant only its own link is visible.
	links, err := module.ListGuardianLinksByAccount(otherCtx, accountID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, otherTenantID, links[0].TenantID)

	guardians, err := module.ListGuardiansByID(otherCtx, []int64{guardianID, otherGuardian.ID})
	require.NoError(t, err)
	require.Len(t, guardians, 1)
	assert.Equal(t, otherGuardian.ID, guardians[0].ID)

	// An admin transaction spans every tenant the account is linked at, in
	// tenant order; that is what the parents portal reads.
	require.NoError(t, tenant.WithinAdmin(testpkg.Ctx(t), func(adminCtx context.Context) error {
		links, err := module.ListGuardianLinksByAccount(adminCtx, accountID)
		require.NoError(t, err)
		tenants := make([]int64, 0, len(links))
		for _, link := range links {
			tenants = append(tenants, link.TenantID)
		}
		assert.ElementsMatch(t, []int64{testpkg.Tenant(t), otherTenantID}, tenants)
		assert.True(t, tenants[0] <= tenants[1], "links are ordered by tenant")
		return nil
	}))
}

func TestGuardianDirectoryObservesProviderOperations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var observed []string
	module := buildModule(t, db, func(observation Observation) {
		observed = append(observed, observation.Operation+":"+peopledirectory.ErrorCode(observation.Err))
	})

	_, err := module.FindGuardian(testpkg.Ctx(t), 1)
	require.ErrorIs(t, err, peopledirectory.ErrGuardianProviderUnbound, "the compose graph binds no guardian provider on its own")
	assert.Contains(t, observed, "find_guardian:unbound")
}
