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
	administration := setupAuthFactory(t, db).AccountAuthentication()

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
	administration := setupAuthFactory(t, db).AccountAuthentication()
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
	administration := setupAuthFactory(t, db).AccountAuthentication()
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
	administration := setupAuthFactory(t, db).AccountAuthentication()

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
