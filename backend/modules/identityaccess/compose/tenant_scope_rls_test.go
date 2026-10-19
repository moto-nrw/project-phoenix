package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestIdentityStoreRequiresTransactionAndIsolatesSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "scope-guard")
	schoolA := testpkg.Tenant(t)
	schoolB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, schoolB)
	testpkg.MapAccountToTenant(t, db, account.ID, schoolA)
	testpkg.MapAccountToTenant(t, db, account.ID, schoolB)
	store := newStore(db)
	base := testpkg.WithTenantRuntime(t, context.Background(), db)
	_, _, err := store.ListActiveTenantIDs(base, account.ID)
	require.ErrorContains(t, err, "tenant")
	_, _, err = store.ListActiveTenantIDs(tenant.WithTenantID(base, schoolA), account.ID)
	require.ErrorContains(t, err, "transaction is required")
	for _, id := range []int64{schoolA, schoolB} {
		require.NoError(t, tenant.WithTenantTx(base, db, id, func(ctx context.Context, _ testpkg.Tx) error {
			ids, _, err := store.ListActiveTenantIDs(ctx, account.ID)
			require.NoError(t, err)
			require.Equal(t, []int64{id}, ids)
			// The explicit role/permission API cannot use school 0 as all schools.
			_, _, err = store.ListAccountRolesAtTenant(ctx, account.ID, 0, false)
			require.Error(t, err)
			_, _, err = store.ListAccountPermissionsAtTenant(ctx, account.ID, 0)
			require.Error(t, err)
			return nil
		}))
	}
	require.NoError(t, tenant.WithinAdmin(base, func(ctx context.Context) error {
		ids, _, err := store.ListActiveTenantIDs(ctx, account.ID)
		require.NoError(t, err)
		require.ElementsMatch(t, []int64{schoolA, schoolB}, ids, "explicit authentication bootstrap can resolve both account schools")
		return nil
	}))
}

func TestIdentityRuntimeDoesNotEscalateTenantlessPortalScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	runtime := tenantRuntime{attach: func(ctx context.Context) context.Context { return testpkg.WithTenantRuntime(t, ctx, db) }, runner: newTransactionRunner()}
	for _, scope := range []string{tenant.ScopeParent, tenant.ScopeSchool, tenant.ScopeOrg, "unexpected"} {
		called := false
		err := runtime.RunInTx(tenant.WithScope(context.Background(), scope), func(context.Context) error { called = true; return nil })
		require.ErrorIs(t, err, tenant.ErrTenantRequired, scope)
		require.False(t, called, scope)
	}
}
