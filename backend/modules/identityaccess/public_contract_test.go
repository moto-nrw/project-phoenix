package identityaccess_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPublicIdentityContractListsOnlyActiveAccountsOfItsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewIdentityAccessForTests(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	var query identityaccess.SchoolMembershipQuery = module
	own := testpkg.CreateTestAccount(t, db, "identity-contract")
	var otherID int64
	t.Run("foreign school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		otherID = testpkg.CreateTestAccount(t, db, "identity-foreign").ID
	})
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		ids, err := query.ListActiveSchoolAccountIDs(ctx)
		require.NoError(t, err)
		require.Contains(t, ids, own.ID)
		require.NotContains(t, ids, otherID)
		active, err := query.HasActiveSchoolMembership(ctx, otherID)
		require.NoError(t, err)
		require.False(t, active)
		return nil
	}))
}
