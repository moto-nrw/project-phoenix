package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestNativeParentAccountRejectsInvalidEmail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	accounts := newMFATestModule(t, db).Auth
	ctx := testpkg.Ctx(t)
	for _, email := range []string{"", "not-an-email"} {
		_, err := accounts.CreateParentAccount(ctx, email, "", testPassword)
		require.Error(t, err, "parent creation must preserve the retained model's email validation")
	}
	account, err := accounts.CreateParentAccount(ctx, "parent-validation@example.com", "parent-validation", testPassword)
	require.NoError(t, err)
	for _, email := range []string{"", "not-an-email"} {
		invalid := account
		invalid.Email = email
		require.Error(t, accounts.UpdateParentAccount(ctx, invalid))
	}
	stored, err := accounts.GetParentAccountByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, account.Email, stored.Email, "rejected updates must preserve the address")
	listed, err := accounts.ListParentAccounts(ctx, identityaccess.ParentAccountFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1, "invalid creation must leave no row behind")
}

func TestNativeParentAccountTenantAndCredentialPersistence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	accounts := newMFATestModule(t, db).Auth
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	foreignCtx := tenant.WithTenantID(ctx, other)
	email := fmt.Sprintf("parent-native-%d@example.com", home)
	account, err := accounts.CreateParentAccount(ctx, strings.ToUpper(email), "parent-native", testPassword)
	require.NoError(t, err)
	require.Equal(t, email, account.Email)
	byEmail, err := accounts.GetParentAccountByEmail(ctx, strings.ToUpper(email))
	require.NoError(t, err)
	require.Equal(t, account.ID, byEmail.ID)
	var originalHash string
	require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts_parents WHERE id = ?", account.ID).Scan(ctx, &originalHash))
	require.NotEmpty(t, originalHash)

	_, err = accounts.GetParentAccountByID(foreignCtx, account.ID)
	require.ErrorIs(t, err, identityaccess.ErrParentAccountNotFound)
	account.Username = "changed-parent"
	require.ErrorIs(t, accounts.UpdateParentAccount(foreignCtx, account), identityaccess.ErrParentAccountNotFound)
	require.ErrorIs(t, accounts.DeactivateParentAccount(foreignCtx, account.ID), identityaccess.ErrParentAccountNotFound)
	foreign, err := accounts.ListParentAccounts(foreignCtx, identityaccess.ParentAccountFilter{})
	require.NoError(t, err)
	require.Empty(t, foreign)

	rollback := errors.New("rollback parent-account edit")
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		txCtx = tenant.WithTenantID(txCtx, home)
		require.NoError(t, accounts.UpdateParentAccount(txCtx, account))
		inside, readErr := accounts.GetParentAccountByID(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Equal(t, account.Username, inside.Username)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	stored, err := accounts.GetParentAccountByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "parent-native", stored.Username)

	account.Email = strings.ToUpper(fmt.Sprintf("parent-updated-%d@example.com", home))
	account.Username = ""
	account.Active = false
	require.NoError(t, accounts.UpdateParentAccount(ctx, account))
	var keptHash string
	require.NoError(t, db.NewRaw("SELECT password_hash FROM auth.accounts_parents WHERE id = ?", account.ID).Scan(ctx, &keptHash))
	require.Equal(t, originalHash, keptHash, "profile updates must not replace the password")
	inactive := false
	listed, err := accounts.ListParentAccounts(ctx, identityaccess.ParentAccountFilter{Email: account.Email, Active: &inactive})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, strings.ToLower(account.Email), listed[0].Email)
	require.Empty(t, listed[0].Username)
	require.NoError(t, accounts.ActivateParentAccount(ctx, account.ID))
	listed, err = accounts.ListParentAccounts(ctx, identityaccess.ParentAccountFilter{Active: &inactive})
	require.NoError(t, err)
	require.Empty(t, listed)
}
