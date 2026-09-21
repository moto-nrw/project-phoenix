package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGuardianSeedPreservesExistingInactiveAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("seed-reuse-%d@test.local", testpkg.Tenant(t)), "Existing-seed-test-9!")
	_, err = db.NewRaw("UPDATE auth.accounts SET active = FALSE WHERE id = ?", account.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", account.ID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	id, reused, err := identity.SeedGuardianAccount(ctx, "  "+strings.ToUpper(account.Email)+"  ", "different-prehashed-password")
	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, account.ID, id)
	var stored struct {
		Active       bool
		PasswordHash string
	}
	require.NoError(t, db.NewRaw("SELECT active, password_hash FROM auth.accounts WHERE id = ?", id).Scan(ctx, &stored))
	require.False(t, stored.Active, "seeding must not reactivate the global account")
	require.Equal(t, *account.PasswordHash, stored.PasswordHash)
	active, err := testpkg.ActiveAccountTenantExists(ctx, db, id, testpkg.Tenant(t))
	require.NoError(t, err)
	require.True(t, active, "the school mapping is independently reactivated")

	id, reused, err = identity.SeedGuardianAccount(context.Background(), account.Email, *account.PasswordHash)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
	require.Zero(t, id)
	require.False(t, reused)
}

func TestGuardianSeedRejectsMalformedEmail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	email := fmt.Sprintf("not-an-email-%d", testpkg.Tenant(t))
	id, reused, err := identity.SeedGuardianAccount(ctx, email, "prehashed-password")
	require.ErrorContains(t, err, "invalid email format")
	require.Zero(t, id)
	require.False(t, reused)
	var count int
	require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM auth.accounts WHERE email = ?", email).Scan(ctx, &count))
	require.Zero(t, count, "invalid input must not create an account or its access records")
}
