package auth_test

// IssueTokensForAuthenticatedAccount is the post-MFA-verify token mint path —
// completes login without re-validating the password (MFA proved identity).
// It's 0% covered today because none of the existing tests exercise the
// "skip credential check" branch.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestIssueTokensForAuthenticatedAccount_HappyPath(t *testing.T) {
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

	access, refresh, err := service.IssueTokensForAuthenticatedAccount(
		context.Background(), account.ID, tenantID, "127.0.0.1", "test-agent")
	require.NoError(t, err)
	assert.NotEmpty(t, access, "access token must be issued")
	assert.NotEmpty(t, refresh, "refresh token must be issued")
}

func TestIssueTokensForAuthenticatedAccount_UnknownAccount_ReturnsAccountNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// An account ID far above the high-water mark — FindByID returns sql.ErrNoRows.
	_, _, err := service.IssueTokensForAuthenticatedAccount(
		context.Background(), 9876543210, 0, "127.0.0.1", "test-agent")
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrAccountNotFound,
		"unknown account must surface as ErrAccountNotFound, not a generic 500")
}

func TestIssueTokensForAuthenticatedAccount_InactiveAccount_Rejected(t *testing.T) {
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

	_, _, err = service.IssueTokensForAuthenticatedAccount(
		context.Background(), account.ID, tenantID, "127.0.0.1", "test-agent")
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrAccountInactive,
		"inactive accounts must not be issued a session even with MFA proof")
}
