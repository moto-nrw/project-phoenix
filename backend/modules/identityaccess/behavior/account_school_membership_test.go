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

func TestActiveAccountSchoolIDsPreservesMembershipSemantics(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	pending, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "native-school-memberships")
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	testpkg.MapAccountToTenant(t, db, account.ID, pending)
	_, err = db.NewRaw("UPDATE auth.account_tenants SET status = 'pending' WHERE account_id = ? AND tenant_id = ?", account.ID, pending).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.account_tenants SET created_at = NOW() - INTERVAL '1 day' WHERE account_id = ? AND tenant_id = ?", account.ID, other).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.accounts SET active = FALSE WHERE id = ?", account.ID).Exec(ctx)
	require.NoError(t, err)
	ids, err := query.ListActiveAccountSchoolIDs(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{other, home}, ids, "creation order and mapping status, not account activation, define these facts")
	empty, err := query.ListActiveAccountSchoolIDs(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, empty, "an unknown account must not enumerate another account's memberships")

	rollback := errors.New("rollback membership change")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		_, updateErr := tx.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", account.ID, other).Exec(txCtx)
		require.NoError(t, updateErr)
		inside, readErr := query.ListActiveAccountSchoolIDs(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Equal(t, []int64{home}, inside, "read the caller's uncommitted membership change")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	after, err := query.ListActiveAccountSchoolIDs(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, ids, after)
}

func TestActiveSchoolMembershipsAreBoundedAndUseAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	query, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	account := testpkg.CreateTestAccount(t, db, "bounded-memberships")
	unselected := testpkg.CreateTestAccount(t, db, "unselected-memberships")
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	_, err = db.NewRaw("UPDATE auth.accounts SET active = FALSE WHERE id = ?", account.ID).Exec(ctx)
	require.NoError(t, err)
	read := func(ctx context.Context) map[int64][]int64 {
		t.Helper()
		found, readErr := query.FindActiveSchoolMemberships(ctx, []int64{account.ID}, []int64{home})
		require.NoError(t, readErr)
		return found
	}
	require.Equal(t, map[int64][]int64{account.ID: {home}}, read(ctx), "exclude unselected accounts and schools, without inferring account activation")
	for _, status := range []string{"pending", "inactive"} {
		rollback := errors.New("rollback membership status")
		err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
			_, updateErr := tx.NewRaw("UPDATE auth.account_tenants SET status = ? WHERE account_id = ? AND tenant_id = ?", status, account.ID, home).Exec(txCtx)
			require.NoError(t, updateErr)
			require.Empty(t, read(txCtx), "another school's active membership must not authorize this school")
			return rollback
		})
		require.ErrorIs(t, err, rollback)
		require.Equal(t, map[int64][]int64{account.ID: {home}}, read(ctx))
	}
	for _, selection := range []struct{ accounts, schools []int64 }{
		{nil, []int64{home}}, {[]int64{unselected.ID}, nil},
	} {
		found, readErr := query.FindActiveSchoolMemberships(ctx, selection.accounts, selection.schools)
		require.NoError(t, readErr)
		require.Empty(t, found, "empty bounds must not enumerate memberships")
	}
}
