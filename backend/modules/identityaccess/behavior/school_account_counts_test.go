package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSchoolAccountCountsDeduplicateWithinBoundedGroups(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	first, _ := testpkg.CreateTestTenant(t, db)
	second, _ := testpkg.CreateTestTenant(t, db)
	outside, _ := testpkg.CreateTestTenant(t, db)
	shared := testpkg.CreateTestAccount(t, db, "count-shared")
	secondOnly := testpkg.CreateTestAccount(t, db, "count-second")
	departed := testpkg.CreateTestAccount(t, db, "count-departed")
	for _, school := range []int64{first, second, outside} {
		testpkg.EnsureAccountTenant(t, db, shared.ID, school)
	}
	testpkg.EnsureAccountTenant(t, db, secondOnly.ID, second)
	testpkg.EnsureAccountTenant(t, db, departed.ID, first)
	_, err = db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", departed.ID, first).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET active = FALSE WHERE id = ?", shared.ID).Exec(ctx)
	require.NoError(t, err)
	groups := map[int64][]int64{0: {first, second, first}, 1: {first}, 2: {second}, 3: nil, 4: {0}}
	want := map[int64]int{0: 2, 1: 1, 2: 2, 3: 0, 4: 0}
	check := func(readCtx context.Context, expected map[int64]int) {
		t.Helper()
		got, readErr := identity.CountActiveAccountsBySchoolGroups(readCtx, groups)
		require.NoError(t, readErr)
		require.Equal(t, expected, got)
	}
	check(ctx, want)
	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.TenantContext(first), db, first, func(txCtx context.Context) error {
		check(txCtx, want) // Memberships have no RLS; explicit school bounds remain decisive.
		return nil
	}))
	rollback := errors.New("rollback count membership change")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, updateErr := tx.NewRaw("UPDATE auth.account_tenants SET status = 'pending' WHERE account_id = ? AND tenant_id = ?", shared.ID, first).Exec(txCtx)
		require.NoError(t, updateErr)
		check(txCtx, map[int64]int{0: 2, 1: 0, 2: 2, 3: 0, 4: 0})
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	check(ctx, want)
	empty, err := identity.CountActiveAccountsBySchoolGroups(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
