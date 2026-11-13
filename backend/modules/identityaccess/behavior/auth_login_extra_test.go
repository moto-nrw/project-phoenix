package behavior_test

// IssueTokensForPasskey mints the tenant session of a passkey sign-in:
// it completes the login without re-validating the password, because the
// assertion proved the identity.

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestIssueTokensForPasskey_HappyPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	regCtx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("issue-tokens-happy")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)

	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	access, refresh, err := service.IssueTokensForPasskey(
		context.Background(), account.ID, tenantID, "127.0.0.1", "test-agent")
	require.NoError(t, err)
	assert.NotEmpty(t, access, "access token must be issued")
	assert.NotEmpty(t, refresh, "refresh token must be issued")
}

func TestIssueTokensForPasskey_UnknownAccount_ReturnsAccountNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// An account ID far above the high-water mark — FindByID returns sql.ErrNoRows.
	_, _, err := service.IssueTokensForPasskey(
		context.Background(), 9876543210, 0, "127.0.0.1", "test-agent")
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrAccountNotFound,
		"unknown account must surface as ErrAccountNotFound, not a generic 500")
}

func TestIssueTokensForPasskey_InactiveAccount_Rejected(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	regCtx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("issue-tokens-inactive")
	account, err := service.Register(regCtx, email, username, testPassword, nil, 0)
	require.NoError(t, err)

	// Deactivate the account — IssueTokens... must refuse to mint a session.
	_, err = db.ExecContext(context.Background(),
		`UPDATE auth.accounts SET active = false WHERE id = ?`, account.ID)
	require.NoError(t, err)

	_, _, err = service.IssueTokensForPasskey(
		context.Background(), account.ID, tenantID, "127.0.0.1", "test-agent")
	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrAccountInactive,
		"inactive accounts must not be issued a session even with MFA proof")
}
