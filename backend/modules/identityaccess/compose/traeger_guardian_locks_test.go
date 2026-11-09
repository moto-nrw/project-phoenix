package compose

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTraegerGuardianAccountLocksRequireTheMatchingTransactionAndLiveMembership(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithTenantRuntime(t, t.Context(), db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	otherOrg := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Anderer Träger", Slug: fmt.Sprintf("account-locks-%d", chain.TenantID),
	}).ID
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, otherOrg, "Andere Schule")
	foreign := testpkg.CreateTestAccount(t, db, "foreign-release-lock")
	testpkg.UnclaimTestAccount(t, db, foreign.ID)
	testpkg.EnsureAccountTenant(t, db, foreign.ID, otherSchool)
	erased := testpkg.CreateTestAccount(t, db, "erased-release-lock")
	testpkg.EnsureAccountTenant(t, db, erased.ID, chain.TenantID)
	_, err := db.ExecContext(ctx, `UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, erased.ID)
	require.NoError(t, err)

	access, err := NewGuardianStudentAccess(db, func(Observation) {})
	require.NoError(t, err)
	lock := access.LockLiveTraegerGuardianAccounts
	_, err = lock(ctx, organization, []int64{chain.AccountID})
	require.Error(t, err, "no lock outside a Träger transaction")
	require.NoError(t, testpkg.WithinTenantTransaction(ctx, db, chain.TenantID, func(txCtx context.Context, _ testpkg.Tx) error {
		_, err := lock(txCtx, organization, []int64{chain.AccountID})
		require.Error(t, err, "a school transaction cannot acquire Träger account locks")
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		_, err := lock(txCtx, otherOrg, []int64{foreign.ID})
		require.Error(t, err, "the context cannot lock for another Träger")
		raw, _ := testpkg.TransactionFromContext(txCtx)
		tx := raw.(testpkg.Tx)
		var outside []int64
		require.NoError(t, tx.NewRaw(`SELECT * FROM auth.lock_live_traeger_guardian_accounts(?, ARRAY[?]::bigint[])`,
			otherOrg, foreign.ID).Scan(txCtx, &outside))
		require.Empty(t, outside, "the database capability itself rejects a different Träger")
		live, err := lock(txCtx, organization, []int64{foreign.ID, erased.ID, chain.AccountID, chain.AccountID})
		require.NoError(t, err)
		require.Equal(t, []int64{chain.AccountID}, live, "only the live candidate of this Träger, once")
		live, err = lock(txCtx, organization, nil)
		require.NoError(t, err)
		require.Empty(t, live)
		return nil
	}))
	_, err = db.ExecContext(ctx, `UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ?`, chain.AccountID)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		live, err := lock(txCtx, organization, []int64{chain.AccountID})
		require.NoError(t, err)
		require.Empty(t, live, "a candidate whose membership ended is omitted")
		return nil
	}))
}
