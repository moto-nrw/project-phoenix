package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestAccountAnonymizationPreservesTransactionAndOtherAccountState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	accounts := setupAuthFactory(t, db).AccountAuthentication()
	account := testpkg.CreateTestAccount(t, db, "anonymize-account")
	other := testpkg.CreateTestAccount(t, db, "anonymize-other")
	ctx := testpkg.Ctx(t)
	originalUsername := fmt.Sprintf("anonymization-%d", account.ID)
	_, err := db.NewRaw("UPDATE auth.accounts SET username = ? WHERE id = ?", originalUsername, account.ID).Exec(ctx)
	require.NoError(t, err)
	placeholder := fmt.Sprintf("deleted-%d@anonymized.local", account.ID)
	rollback := errors.New("rollback account anonymization")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		require.NoError(t, accounts.AnonymizeAccountForDeletion(txCtx, account.ID, placeholder))
		var changed struct {
			Email    string
			Username *string
			Active   bool
		}
		require.NoError(t, tx.NewRaw("SELECT email, username, active FROM auth.accounts WHERE id = ?", account.ID).Scan(txCtx, &changed))
		require.Equal(t, placeholder, changed.Email)
		require.Nil(t, changed.Username, "the identifying username is cleared, not merely hidden in a result")
		require.Equal(t, account.Active, changed.Active, "deactivation remains the workflow's separate step")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	unchanged, err := accounts.FindOwnAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, account.Email, unchanged.Email)
	require.Equal(t, originalUsername, unchanged.Username)

	require.NoError(t, accounts.AnonymizeAccountForDeletion(ctx, account.ID, placeholder))
	require.NoError(t, accounts.AnonymizeAccountForDeletion(ctx, account.ID, placeholder), "retries are idempotent")
	anonymized, err := accounts.FindOwnAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, placeholder, anonymized.Email)
	require.Empty(t, anonymized.Username)
	untouched, err := accounts.FindOwnAccount(ctx, other.ID)
	require.NoError(t, err)
	require.Equal(t, other.Email, untouched.Email)
	require.NoError(t, accounts.AnonymizeAccountForDeletion(ctx, 0, placeholder), "an already absent account remains a no-op")

	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		return accounts.AnonymizeAccountForDeletion(txCtx, account.ID, other.Email)
	})
	require.Error(t, err, "a conflicting placeholder must fail the deletion transaction")
	afterFailure, err := accounts.FindOwnAccount(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, placeholder, afterFailure.Email)
}
