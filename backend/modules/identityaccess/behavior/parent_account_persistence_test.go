package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestNativeParentAccountRejectsInvalidEmail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	accounts := newMFATestModule(t, db).Auth
	ctx := testpkg.Ctx(t)
	parentAccountTx(t, db, ctx, testpkg.Tenant(t), func(txCtx context.Context) {
		for _, email := range []string{"", "not-an-email"} {
			_, err := accounts.CreateParentAccount(txCtx, email, "", testPassword)
			require.Error(t, err, "parent creation must preserve the retained model's email validation")
		}
		account, err := accounts.CreateParentAccount(txCtx, "parent-validation@example.com", "parent-validation", testPassword)
		require.NoError(t, err)
		for _, email := range []string{"", "not-an-email"} {
			invalid := account
			invalid.Email = email
			require.Error(t, accounts.UpdateParentAccount(txCtx, invalid))
		}
		stored, err := accounts.GetParentAccountByID(txCtx, account.ID)
		require.NoError(t, err)
		require.Equal(t, account.Email, stored.Email, "rejected updates must preserve the address")
		listed, err := accounts.ListParentAccounts(txCtx, identityaccess.ParentAccountFilter{})
		require.NoError(t, err)
		require.Len(t, listed, 1, "invalid creation must leave no row behind")
	})
}

func TestNativeParentAccountTenantAndCredentialPersistence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	accounts := newMFATestModule(t, db).Auth
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	foreignCtx := testpkg.ContextForTenant(ctx, other)
	email := fmt.Sprintf("parent-native-%d@example.com", home)
	var account identityaccess.ParentAccount
	parentAccountTx(t, db, ctx, home, func(txCtx context.Context) {
		var err error
		account, err = accounts.CreateParentAccount(txCtx, strings.ToUpper(email), "parent-native", testPassword)
		require.NoError(t, err)
		require.Equal(t, email, account.Email)
		byEmail, err := accounts.GetParentAccountByEmail(txCtx, strings.ToUpper(email))
		require.NoError(t, err)
		require.Equal(t, account.ID, byEmail.ID)
	})
	var originalHash string
	require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts_parents WHERE id = ?", account.ID).Scan(ctx, &originalHash))
	require.NotEmpty(t, originalHash)

	parentAccountTx(t, db, foreignCtx, other, func(txCtx context.Context) {
		_, err := accounts.GetParentAccountByID(txCtx, account.ID)
		require.ErrorIs(t, err, identityaccess.ErrParentAccountNotFound)
		account.Username = "changed-parent"
		require.ErrorIs(t, accounts.UpdateParentAccount(txCtx, account), identityaccess.ErrParentAccountNotFound)
		require.ErrorIs(t, accounts.DeactivateParentAccount(txCtx, account.ID), identityaccess.ErrParentAccountNotFound)
		foreign, err := accounts.ListParentAccounts(txCtx, identityaccess.ParentAccountFilter{})
		require.NoError(t, err)
		require.Empty(t, foreign)
	})

	rollback := errors.New("rollback parent-account edit")
	err := testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = testpkg.ContextForTenant(txCtx, home)
		require.NoError(t, accounts.UpdateParentAccount(txCtx, account))
		inside, readErr := accounts.GetParentAccountByID(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Equal(t, account.Username, inside.Username)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	parentAccountTx(t, db, ctx, home, func(txCtx context.Context) {
		stored, err := accounts.GetParentAccountByID(txCtx, account.ID)
		require.NoError(t, err)
		require.Equal(t, "parent-native", stored.Username)

		account.Email = strings.ToUpper(fmt.Sprintf("parent-updated-%d@example.com", home))
		account.Username = ""
		account.Active = false
		require.NoError(t, accounts.UpdateParentAccount(txCtx, account))
	})
	var keptHash string
	require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts_parents WHERE id = ?", account.ID).Scan(ctx, &keptHash))
	require.Equal(t, originalHash, keptHash, "profile updates must not replace the password")
	inactive := false
	parentAccountTx(t, db, ctx, home, func(txCtx context.Context) {
		listed, err := accounts.ListParentAccounts(txCtx, identityaccess.ParentAccountFilter{Email: account.Email, Active: &inactive})
		require.NoError(t, err)
		require.Len(t, listed, 1)
		require.Equal(t, strings.ToLower(account.Email), listed[0].Email)
		require.Empty(t, listed[0].Username)
		require.NoError(t, accounts.ActivateParentAccount(txCtx, account.ID))
		listed, err = accounts.ListParentAccounts(txCtx, identityaccess.ParentAccountFilter{Active: &inactive})
		require.NoError(t, err)
		require.Empty(t, listed)
	})
}

func parentAccountTx(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64, fn func(context.Context)) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		fn(txCtx)
		return nil
	}))
}
