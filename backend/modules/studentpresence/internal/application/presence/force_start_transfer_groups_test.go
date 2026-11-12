package presence_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestForceStartTransferGroupsPreserveBareRowsAndTenantErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := sessionGroups(api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db)).ActiveGroup)
	source := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	target := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	foreign := testpkg.CreateTestActiveGroupForTenant(t, db, foreignTenant)

	err := testpkg.TenantCommand0(t, ctx, db, func(txCtx context.Context) error {
		for _, pair := range [][2]int64{{target.ID, source.ID}, {source.ID, target.ID}, {source.ID, source.ID}} {
			for _, id := range pair {
				_, err := repo.FindByIDForUpdate(txCtx, id)
				require.NoError(t, err)
			}
			wantSource, err := repo.FindByID(txCtx, pair[0])
			require.NoError(t, err)
			wantTarget, err := repo.FindByID(txCtx, pair[1])
			require.NoError(t, err)
			gotSource, gotTarget, err := repo.FindVisitTransferGroups(txCtx, pair[0], pair[1])
			require.NoError(t, err)
			require.Equal(t, wantSource, gotSource)
			require.Equal(t, wantTarget, gotTarget)
		}
		_, missing := repo.FindByID(txCtx, foreign.ID)
		require.Error(t, missing)
		for _, pair := range [][2]int64{{foreign.ID, target.ID}, {source.ID, foreign.ID}, {foreign.ID, foreign.ID}} {
			gotSource, gotTarget, err := repo.FindVisitTransferGroups(txCtx, pair[0], pair[1])
			require.Nil(t, gotSource)
			require.Nil(t, gotTarget)
			require.Equal(t, missing, err, "preserve the scalar missing-row type and detail")
		}
		return nil
	})
	require.NoError(t, err)

	_, scalarError := repo.FindByID(context.Background(), source.ID)
	require.Error(t, scalarError)
	var first, second *ports.ActiveGroup
	first, second, pairError := repo.FindVisitTransferGroups(context.Background(), source.ID, target.ID)
	require.Nil(t, first)
	require.Nil(t, second)
	require.Equal(t, scalarError, pairError, "missing tenant must retain the scalar error and detail")
}
