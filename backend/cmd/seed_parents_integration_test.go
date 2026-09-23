// CLI integration tests stay internal: the architecture policy forbids
// module-behavior-test packages from importing the root CLI package.
package cmd

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSeedParentAccounts(t *testing.T) {
	t.Parallel()
	for _, reuse := range []bool{false, true} {
		name := "create"
		if reuse {
			name = "reuse"
		}
		t.Run(name, func(t *testing.T) {
			db := testpkg.SetupIsolatedTestDB(t)
			ctx := testpkg.Ctx(t)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			email := chain.Email
			if !reuse {
				email = "new-" + email
			}
			var oldHash string
			require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts WHERE id = ?", chain.AccountID).Scan(ctx, &oldHash))
			_, err := db.ExecContext(ctx,
				"UPDATE users.guardian_profiles SET account_id = NULL, has_account = FALSE, email = ? WHERE id = ?",
				email, chain.GuardianProfileID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, "UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
				chain.AccountID, chain.TenantID)
			require.NoError(t, err)

			require.NoError(t, seedParentAccounts(context.Background(), db, 1, "Seed-parent-test-9!"))
			var accountID int64
			require.NoError(t, db.NewRaw("SELECT account_id FROM users.guardian_profiles WHERE id = ?", chain.GuardianProfileID).Scan(ctx, &accountID))
			testpkg.OwnTestAccount(t, db, accountID)
			var hash string
			require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts WHERE id = ?", accountID).Scan(ctx, &hash))
			if reuse {
				require.Equal(t, chain.AccountID, accountID)
				require.Equal(t, oldHash, hash, "existing credentials are untouched")
			} else {
				require.NotEqual(t, chain.AccountID, accountID)
				require.NotEmpty(t, hash)
			}
			var status string
			require.NoError(t, db.NewRaw("SELECT status FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?",
				accountID, chain.TenantID).Scan(ctx, &status))
			require.Equal(t, "active", status)
			var assignments int
			require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id
				WHERE ar.account_id = ? AND ar.tenant_id = ? AND r.name = 'guardian'`, accountID, chain.TenantID).Scan(ctx, &assignments))
			require.Equal(t, 1, assignments)
			require.NoError(t, seedParentAccounts(context.Background(), db, 1, "Different-seed-test-9!"))
			var afterHash string
			require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts WHERE id = ?", accountID).Scan(ctx, &afterHash))
			require.Equal(t, hash, afterHash)
		})
	}
}

func TestSeedParentAccountsRollsBackMissingRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	email := "rollback-" + chain.Email
	_, err := db.ExecContext(ctx,
		"UPDATE users.guardian_profiles SET account_id = NULL, has_account = FALSE, email = ? WHERE id = ?",
		email, chain.GuardianProfileID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "UPDATE auth.roles SET name = 'seed-test-missing-guardian' WHERE name = 'guardian'")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, restoreErr := db.ExecContext(context.Background(), "UPDATE auth.roles SET name = 'guardian' WHERE name = 'seed-test-missing-guardian'")
		require.NoError(t, restoreErr)
	})
	require.ErrorContains(t, seedParentAccounts(context.Background(), db, 1, "Seed-rollback-test-9!"), "grant guardian access")
	var accounts int
	require.NoError(t, db.NewRaw("SELECT COUNT(*) FROM auth.accounts WHERE email = ?", email).Scan(ctx, &accounts))
	require.Zero(t, accounts)
	var linked bool
	require.NoError(t, db.NewRaw("SELECT account_id IS NOT NULL OR has_account FROM users.guardian_profiles WHERE id = ?", chain.GuardianProfileID).Scan(ctx, &linked))
	require.False(t, linked)
}
