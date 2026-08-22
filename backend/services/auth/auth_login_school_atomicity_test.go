// Package auth_test — the atomicity half of the school-portal login tests
// (#2207): the authorization behind a school session must be decided in the
// same transaction that writes the token, and a refused refresh must leave
// the caller's refresh token intact.
package auth_test

import (
	"context"
	"net"
	"testing"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const portalBindingJWTSecret = "portal-binding-test-secret-please-ignore"

// The pool is closed via t.Cleanup rather than defer throughout this file:
// deferred closes run BEFORE the fixture cleanups registered later, so every
// teardown query hit an already-closed pool and left its rows behind in the
// shared test database.

// TestRefreshToken_SchoolScope_RoleRevoked_LeavesRefreshTokenUsable pins the
// ordering half of the role re-check. Rejecting a revoked Lehrkraft is not
// enough on its own: while the check ran AFTER the rotation transaction, the
// refusal had already consumed the presented refresh token and destroyed the
// session even when the revocation turned out to be temporary (a role
// reassigned seconds later, an operator undoing a mis-click). Deciding inside
// the rotation rolls the rotation back with it, so the token the client still
// holds keeps working the moment access is restored.
func TestRefreshToken_SchoolScope_RoleRevoked_LeavesRefreshTokenUsable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := context.Background()

	tenantID, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-revoke-atomic", tenantID)

	login, err := service.LoginSchoolWithMFAGate(ctx, email, testPassword, "", "", "")
	require.NoError(t, err)

	_, err = db.NewDelete().
		Table("auth.account_roles").
		Where("account_id = ?", accountID).
		Exec(ctx)
	require.NoError(t, err)

	_, _, err = service.RefreshTokenWithAudit(ctx, login.RefreshToken, "", "")
	require.Error(t, err, "refresh must be rejected while the school-portal role is gone")

	// Restore the role and replay the SAME refresh token. It still works only
	// if the rejected attempt rolled its rotation back.
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantID)

	accessToken, refreshToken, err := service.RefreshTokenWithAudit(ctx, login.RefreshToken, "", "")
	require.NoError(t, err,
		"the refused refresh must not have consumed the token — authorization belongs inside the rotation transaction")
	require.NotEmpty(t, refreshToken)

	scope, tokenTenantID := decodeTokenClaims(t, accessToken)
	assert.Equal(t, "school", scope)
	assert.Equal(t, tenantID, tokenTenantID)
}

// TestRefreshToken_SchoolScope_InactiveSchool_LeavesRefreshTokenUsable is the
// same contract for the school-liveness gate. Deactivating a school used to be
// caught only after the rotation had committed, so the first refresh after the
// switch-off both failed AND burned the token: reactivating the school left
// every session dead anyway.
func TestRefreshToken_SchoolScope_InactiveSchool_LeavesRefreshTokenUsable(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)
	ctx := context.Background()

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-inactive-atomic", tenantID)

	login, err := service.LoginSchoolWithMFAGate(ctx, email, testPassword, "", "", "")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "UPDATE platform.schools SET active = false WHERE id = ?", tenantID)
	require.NoError(t, err)

	_, _, err = service.RefreshTokenWithAudit(ctx, login.RefreshToken, "", "")
	require.Error(t, err, "a deactivated school must not keep minting school sessions")
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrTenantNotFound)

	_, err = db.ExecContext(ctx, "UPDATE platform.schools SET active = true WHERE id = ?", tenantID)
	require.NoError(t, err)

	_, refreshToken, err := service.RefreshTokenWithAudit(ctx, login.RefreshToken, "", "")
	require.NoError(t, err,
		"the refusal must have left the refresh token intact — it is rejected before the rotation, not after")
	require.NotEmpty(t, refreshToken)
}

// TestVerifyCodeForAccount_RefusesForeignPortalChallenge pins the portal
// binding on the JWT-less verify path. It is the one verify without a
// challenge id to look up by, so it resolves the account's newest active
// code — and with the school portal in the picture that code can belong to a
// completely different login. A Lehrkraft finishing tenant-portal MFA
// enrollment in one tab while a school-portal login waits for its code in
// another must not be able to complete the tenant enrollment with the school
// code, which would mint a tenant session off a second factor issued for the
// school portal.
func TestVerifyCodeForAccount_RefusesForeignPortalChallenge(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	repos := repositories.NewFactory(db)
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(portalBindingJWTSecret)
	require.NoError(t, err)
	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: portalBindingJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	account := testpkg.CreateTestAccount(t, db, "mfa-portal-binding")
	accountID := account.ID
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, accountID) })

	tenantID, _ := newSchoolTenant(t, db)
	testpkg.EnsureAccountTenant(t, db, accountID, tenantID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.mfa_email_challenges").
			Where("account_id = ?", accountID).Exec(context.Background())
	})

	// One school-portal challenge in flight, nothing else.
	_, err = svc.StartChallenge(ctx, accountID, tenantID, authjwt.MFAChallengeScopeSchool, net.ParseIP("203.0.113.7"))
	require.NoError(t, err)

	stored, err := repos.MFAEmailChallenge.FindActiveByAccountIDInScope(ctx, accountID, tenantID, authjwt.MFAChallengeScopeSchool)
	require.NoError(t, err)
	require.NotNil(t, stored, "the school challenge must be visible to the school scope")
	assert.Equal(t, authjwt.MFAChallengeScopeSchool, stored.Scope)
	assert.Equal(t, tenantID, stored.TenantID)

	// The tenant surface must not see it at all — not even to compare a code
	// against, so a correct school code cannot be spent here.
	_, err = repos.MFAEmailChallenge.FindActiveByAccountIDInScope(ctx, accountID, tenantID, authjwt.MFAChallengeScopeTenant)
	assert.Error(t, err, "a school-scope code must be invisible to the tenant portal's lookup")

	err = svc.VerifyCodeForAccount(ctx, accountID, tenantID, "000000", authjwt.MFAChallengeScopeTenant)
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid,
		"the tenant enroll-confirm must find no challenge at all while only a school challenge is in flight")

	// The school challenge is still unconsumed — the foreign-portal attempt
	// must not have burned it either.
	stillActive, err := repos.MFAEmailChallenge.FindActiveByAccountIDInScope(ctx, accountID, tenantID, authjwt.MFAChallengeScopeSchool)
	require.NoError(t, err)
	require.NotNil(t, stillActive)
	assert.Equal(t, stored.ID, stillActive.ID)
}
