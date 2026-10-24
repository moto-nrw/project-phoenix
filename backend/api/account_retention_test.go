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

	unused := testpkg.CreateTestAccount(t, admin, "erase-unused")
	testpkg.UnclaimTestAccount(t, admin, unused.ID)
	_, needed := testpkg.CreateTestPersonWithAccount(t, admin, "Bleibt", "Konto")

	var erased int
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		erased, err = identity.EraseUnusedAccounts(adminCtx)
		return err
	}))
	assert.Positive(t, erased)

	var emails []string
	require.NoError(t, admin.NewRaw(`SELECT email FROM auth.accounts WHERE id IN (?, ?) ORDER BY id`, unused.ID, needed.ID).
		Scan(context.Background(), &emails))
	assert.Equal(t, []string{fmt.Sprintf("geloescht-%d@example.invalid", unused.ID), needed.Email}, emails)
}
