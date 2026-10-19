package compose

import (
	"context"
	"fmt"
	"testing"

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
	rolesBySchool := map[int64]int64{}
	for _, id := range []int64{schoolA, schoolB} {
		role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("scope-%d", id), id)
		rolesBySchool[id] = role.ID
		_, err := db.ExecContext(context.Background(), `INSERT INTO auth.account_roles(account_id,role_id,tenant_id) VALUES (?,?,?)`, account.ID, role.ID, id)
		require.NoError(t, err)
	}
	store := newStore(db)
	base := testpkg.WithTenantRuntime(t, context.Background(), db)
	_, _, err := store.ListActiveTenantIDs(base, account.ID)
	require.ErrorContains(t, err, "tenant")
	_, _, err = store.ListActiveTenantIDs(testpkg.ContextForTenant(base, schoolA), account.ID)
	require.ErrorContains(t, err, "transaction is required")
	for _, id := range []int64{schoolA, schoolB} {
		require.NoError(t, testpkg.WithinTenantTransaction(base, db, id, func(ctx context.Context, _ testpkg.Tx) error {
			rows, _, err := store.ListAccountRolesAtTenant(ctx, account.ID, id, false)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, rolesBySchool[id], rows[0].RoleID)
			foreign := schoolA
			if id == schoolA {
				foreign = schoolB
			}
			other, _, err := store.ListAccountRolesAtTenant(ctx, account.ID, foreign, false)
			require.NoError(t, err)
			require.Empty(t, other, "school-scoped role reads cannot select another school")

			// The explicit role/permission API cannot use school 0 as all schools.
			_, _, err = store.ListAccountRolesAtTenant(ctx, account.ID, 0, false)
			require.Error(t, err)
			_, _, err = store.ListAccountPermissionsAtTenant(ctx, account.ID, 0)
			require.Error(t, err)
			return nil
		}))
	}
	require.NoError(t, testpkg.WithinAdminTransaction(base, func(ctx context.Context) error {
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
	required := testpkg.WithinCurrentTenant(testpkg.WithTenantRuntime(t, context.Background(), db), func(context.Context) error { t.Fatal("missing tenant reached callback"); return nil })
	require.Error(t, required)
	for _, scope := range []string{"parent", "school", "org", "unexpected"} {
		called := false
		err := runtime.RunInTx(testpkg.ContextForScope(context.Background(), scope), func(context.Context) error { called = true; return nil })
		require.ErrorIs(t, err, required, scope)
		require.False(t, called, scope)
	}
}
