package api

import (
	"context"
	"fmt"
	"testing"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The hourly cleanup erases the accounts no school needs (#4043) in its
// administrative transaction, as phoenix_admin, the only role the function
// grants (TestOnlyTheAdminRoleMayEraseUnusedAccounts). The erasure sweeps
// every school, so the test owns a database clone.
func TestHourlyCleanupErasesUnusedAccounts(t *testing.T) {
	t.Parallel()
	admin := testpkg.SetupIsolatedTestDB(t)
	testpkg.EnsureTestTenant(t, admin, testpkg.Tenant(t))
	db, err := transactionruntime.NewDatabase(admin)
	require.NoError(t, err)
	identity, err := NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, admin))

	// The migration template includes an unused account. Give inherited
	// accounts a school so they cannot consume this run's bounded budget
	// before our unused account, whose position would otherwise be random.
	var inherited []int64
	require.NoError(t, admin.NewRaw(`SELECT id FROM auth.accounts`).Scan(ctx, &inherited))
	for _, accountID := range inherited {
		testpkg.EnsureAccountTenant(t, admin, accountID, testpkg.Tenant(t))
	}

	unused := testpkg.CreateTestAccount(t, admin, "erase-unused")
	testpkg.UnclaimTestAccount(t, admin, unused.ID)
	_, needed := testpkg.CreateTestPersonWithAccount(t, admin, "Bleibt", "Konto")

	var candidates []int64
	require.NoError(t, admin.NewRaw(`SELECT id FROM auth.accounts
		WHERE personal_data_erased_at IS NULL AND auth.account_is_unused(id, email)
		ORDER BY id`).Scan(ctx, &candidates))
	require.Equal(t, []int64{unused.ID}, candidates, "one bounded cleanup must only have the test's unused account to process")

	var erased int
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		erased, err = identity.EraseUnusedAccounts(adminCtx)
		return err
	}))
	assert.Equal(t, 1, erased)

	var emails []string
	require.NoError(t, admin.NewRaw(`SELECT email FROM auth.accounts WHERE id IN (?, ?) ORDER BY id`, unused.ID, needed.ID).
		Scan(context.Background(), &emails))
	assert.Equal(t, []string{fmt.Sprintf("geloescht-%d@example.invalid", unused.ID), needed.Email}, emails)
}
