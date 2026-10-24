package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The caller's own account is readable without the management predicate,
// because the account holder proves who they are with their session, not
// with a school membership. Every other account is only readable inside the
// predicate — and an account outside it reads as missing, so the boundary
// never reveals that it exists (#3332).
func TestFindOwnAccountIgnoresTheManagementBoundary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	administration := setupAuthFactory(t, db)

	// An account with no membership at the school in context: the
	// administration may not touch it, its holder may still read it.
	account := testpkg.CreateTestAccount(t, db, "own-account-unmapped")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	testpkg.OwnTestAccount(t, db, account.ID)
	ctx := testpkg.Ctx(t)

	own, err := administration.FindOwnAccount(ctx, account.ID)
	require.NoError(t, err)
	assert.Equal(t, account.ID, own.ID)
	assert.Equal(t, account.Email, own.Email)

	_, err = administration.FindManageableAccount(ctx, account.ID)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
}

// The address rule the retained account model applied on every write still
// applies: an update normalizes it, so a capitalized input can never create
// a second spelling of the same address.
func TestUpdateManageableAccountNormalisesTheAddress(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	administration := setupAuthFactory(t, db)
	tenantID := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "update-normalised")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	mixed := fmt.Sprintf("  Updated-%d@Test.Local  ", time.Now().UnixNano())
	require.NoError(t, administration.UpdateManageableAccount(testpkg.Ctx(t), identityaccess.AccountIdentityUpdate{
		AccountID: account.ID, Email: mixed,
	}))

	var stored string
	require.NoError(t, db.NewSelect().
		ColumnExpr("email").
		TableExpr("auth.accounts").
		Where("id = ?", account.ID).
		Scan(context.Background(), &stored))
	assert.Equal(t, strings.ToLower(strings.TrimSpace(mixed)), stored)
}

// An update without an address is refused before it writes: an account row
// with an empty e-mail is one nobody can sign in to, because every lookup
// matches on the address.
func TestUpdateManageableAccountRefusesAnEmptyAddress(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	administration := setupAuthFactory(t, db)
	tenantID := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "update-nameless")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	require.Error(t, administration.UpdateManageableAccount(testpkg.Ctx(t), identityaccess.AccountIdentityUpdate{
		AccountID: account.ID,
	}))

	var stored string
	require.NoError(t, db.NewSelect().
		ColumnExpr("email").
		TableExpr("auth.accounts").
		Where("id = ?", account.ID).
		Scan(context.Background(), &stored))
	assert.Equal(t, account.Email, stored, "a refused update writes nothing")
}

// A password change is authorized by the current password, so a wrong one
// leaves the stored credential exactly as it was.
func TestChangeAccountPasswordKeepsTheCredentialOnAWrongCurrentPassword(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	administration := setupAuthFactory(t, db)

	email := fmt.Sprintf("change-password-%d@test.local", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, db, email, testPassword)

	before := accountPasswordHash(t, db, account.ID)
	require.NotEmpty(t, before)

	err := administration.ChangeAccountPassword(context.Background(), account.ID, "WrongPassword1!", testNewPassword)
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	assert.Equal(t, before, accountPasswordHash(t, db, account.ID))

	require.NoError(t, administration.ChangeAccountPassword(context.Background(), account.ID, testPassword, testNewPassword))
	assert.NotEqual(t, before, accountPasswordHash(t, db, account.ID), "the accepted change replaces the credential")
}

func TestAccountAdministrationIgnoresErasedAccounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	administration := setupAuthFactory(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("erased-administration-%d@test.local", time.Now().UnixNano()), testPassword)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	role := testpkg.CreateTestRoleForTenant(t, db, "erased-administration-role", tenantID)
	_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, account.ID, role.ID, tenantID).Exec(ctx)
	require.NoError(t, err)
	beforeHash := accountPasswordHash(t, db, account.ID)
	_, err = db.NewRaw(`UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, account.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = administration.FindOwnAccount(ctx, account.ID)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	_, err = administration.FindManageableAccount(ctx, account.ID)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	listed, err := administration.ListManageableAccounts(ctx, identityaccess.AccountListFilter{})
	require.NoError(t, err)
	for _, listedAccount := range listed {
		assert.NotEqual(t, account.ID, listedAccount.ID)
	}
	byRole, err := administration.ListManageableAccountsByRole(ctx, role.Name)
	require.NoError(t, err)
	for _, listedAccount := range byRole {
		assert.NotEqual(t, account.ID, listedAccount.ID)
	}

	require.ErrorIs(t, administration.UpdateManageableAccount(ctx, identityaccess.AccountIdentityUpdate{
		AccountID: account.ID, Email: "changed-after-erasure@test.local",
	}), identityaccess.ErrAccountNotFound)
	require.ErrorIs(t, administration.ActivateAccount(ctx, account.ID), identityaccess.ErrAccountNotFound)
	require.ErrorIs(t, administration.DeactivateAccount(ctx, account.ID), identityaccess.ErrAccountNotFound)
	require.ErrorIs(t, administration.ChangeAccountPassword(ctx, account.ID, testPassword, testNewPassword), identityaccess.ErrAccountNotFound)

	var current struct {
		Email        string `bun:"email"`
		Active       bool   `bun:"active"`
		PasswordHash string `bun:"password_hash"`
	}
	require.NoError(t, db.NewRaw(`SELECT email, active, password_hash FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &current))
	assert.Equal(t, account.Email, current.Email)
	assert.Equal(t, account.Active, current.Active)
	assert.Equal(t, beforeHash, current.PasswordHash)
}
