// Package auth_test — integration tests for the school-portal login (#2207):
// LoginSchoolWithMFAGate, SwitchSchool, the school-scope refresh round-trip,
// and the school-portal role gate on every token-minting path.
package auth_test

import (
	"context"
	"net"
	"testing"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// decodeTokenClaims decodes a minted JWT and returns scope + tenant_id.
func decodeTokenClaims(t *testing.T, token string) (scope string, tenantID int64) {
	t.Helper()
	decoded, err := authjwt.MustNewTokenAuth().JwtAuth.Decode(token)
	require.NoError(t, err, "minted token must decode")
	_ = decoded.Get("scope", &scope)
	var rawTenant float64
	if getErr := decoded.Get("tenant_id", &rawTenant); getErr == nil {
		tenantID = int64(rawTenant)
	} else {
		var intTenant int64
		if getErr := decoded.Get("tenant_id", &intTenant); getErr == nil {
			tenantID = intTenant
		}
	}
	return scope, tenantID
}

// createLehrkraftAccount registers an account, maps it to the tenant, and
// assigns the lehrkraft system role there.
func createLehrkraftAccount(t *testing.T, db *bun.DB, service auth.AuthService, prefix string, tenantID int64) (email string, accountID int64) {
	t.Helper()
	testpkg.EnsureTestTenant(t, db, tenantID)
	email, username := uniqueTestCredentials(prefix)
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)
	return email, account.ID
}

func TestLoginSchool_NoPortalRole_Refused(t *testing.T) {
	// An account that is mapped to a school but holds no school-portal role
	// (here: no role at all) must be refused with the portal sentinel — the
	// handler maps it to 403.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	testpkg.EnsureTestTenant(t, db, tenantID)
	email, username := uniqueTestCredentials("school-no-role")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.Error(t, err)
	assert.Nil(t, result)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
}

func TestLoginSchool_Lehrkraft_IssuesSchoolScopedTokens(t *testing.T) {
	// Happy path: lehrkraft system role at one school → token pair with
	// scope=school and the school pinned as tenant_id (school tokens are
	// tenant-bound, unlike parent tokens).
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	email, accountID := createLehrkraftAccount(t, db, service, "school-happy", tenantID)
	defer testpkg.CleanupAuthFixtures(t, db, accountID)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusAuthenticated, result.Status)
	require.NotEmpty(t, result.AccessToken)
	require.NotEmpty(t, result.RefreshToken)

	scope, tokenTenantID := decodeTokenClaims(t, result.AccessToken)
	assert.Equal(t, tenant.ScopeSchool, scope, "access token must carry the school scope")
	assert.Equal(t, tenantID, tokenTenantID, "school token must be pinned to the resolved school")

	refreshScope, refreshTenantID := decodeTokenClaims(t, result.RefreshToken)
	assert.Equal(t, tenant.ScopeSchool, refreshScope, "refresh token must carry the school scope for the refresh round-trip")
	assert.Equal(t, tenantID, refreshTenantID)
}

func TestLoginSchool_WrongPassword_Refused(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	email, accountID := createLehrkraftAccount(t, db, service, "school-wrong-pw", tenantID)
	defer testpkg.CleanupAuthFixtures(t, db, accountID)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, "Definitely-Wrong-1%", "", "", "")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestRefreshToken_SchoolScope_Preserved(t *testing.T) {
	// A school-scope refresh token must round-trip as a school token — a
	// silent demotion to tenant scope would fail SchoolMiddleware on the
	// next request and dead-end the portal session.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	email, accountID := createLehrkraftAccount(t, db, service, "school-refresh", tenantID)
	defer testpkg.CleanupAuthFixtures(t, db, accountID)

	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err)

	accessToken, refreshToken, err := service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "")
	require.NoError(t, err, "school-scope refresh must succeed while the role exists")
	require.NotEmpty(t, refreshToken)

	scope, tokenTenantID := decodeTokenClaims(t, accessToken)
	assert.Equal(t, tenant.ScopeSchool, scope, "refreshed access token must stay school-scope")
	assert.Equal(t, tenantID, tokenTenantID)
}

func TestRefreshToken_SchoolScope_RoleRevoked_Rejected(t *testing.T) {
	// Revoking the lehrkraft role must cut the session at the next refresh:
	// the refresh path re-verifies the school-portal role instead of
	// trusting the (still unexpired) refresh token.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	email, accountID := createLehrkraftAccount(t, db, service, "school-revoked", tenantID)
	defer testpkg.CleanupAuthFixtures(t, db, accountID)

	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err)

	_, err = db.NewDelete().
		Table("auth.account_roles").
		Where("account_id = ?", accountID).
		Exec(context.Background())
	require.NoError(t, err)

	_, _, err = service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "")
	require.Error(t, err, "refresh must be rejected once the school-portal role is gone")
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrTenantAccessDenied)
}

func TestSwitchSchool_PortalRoleRequiredAtTarget(t *testing.T) {
	// A Lehrkraft mapped to two schools can switch only where the portal
	// role exists: mapping alone (school B without lehrkraft role) is not
	// enough.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantA int64 = 42
	const tenantB int64 = 43
	email, accountID := createLehrkraftAccount(t, db, service, "school-switch", tenantA)
	defer testpkg.CleanupAuthFixtures(t, db, accountID)
	_ = email

	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.MapAccountToTenant(t, db, accountID, tenantB)

	t.Run("mapping without portal role at target is refused", func(t *testing.T) {
		_, _, err := service.SwitchSchool(context.Background(), accountID, "t43")
		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
	})

	t.Run("portal role at target switches the pinned school", func(t *testing.T) {
		testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantB)

		accessToken, refreshToken, err := service.SwitchSchool(context.Background(), accountID, "t43")
		require.NoError(t, err)
		require.NotEmpty(t, refreshToken)

		scope, tokenTenantID := decodeTokenClaims(t, accessToken)
		assert.Equal(t, tenant.ScopeSchool, scope)
		assert.Equal(t, tenantB, tokenTenantID)
	})
}

func TestLoginSchool_MFARequiredEnrolled_StartsSchoolScopedChallenge(t *testing.T) {
	// With MFA required and enrolled, the school login must hand out a
	// challenge started with the SCHOOL challenge scope — that is what
	// keeps the challenge redeemable only at the school verify endpoint.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	email, accountID := createLehrkraftAccount(t, db, service, "school-mfa", tenantID)
	defer testpkg.CleanupAuthFixtures(t, db, accountID)

	var startedScope string
	var startedTenantID int64
	service.SetMFAService(&authtest.MFAServiceMock{
		IsRequiredFn:    func(context.Context, *authModels.Account, int64) (bool, error) { return true, nil },
		HasEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
		StartChallengeFn: func(_ context.Context, _, challengeTenantID int64, scope string, _ net.IP) (string, error) {
			startedScope = scope
			startedTenantID = challengeTenantID
			return "school-challenge-token", nil
		},
	})
	defer service.SetMFAService(nil)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusMFARequired, result.Status)
	assert.Equal(t, "school-challenge-token", result.ChallengeToken)
	assert.Equal(t, authjwt.MFAChallengeScopeSchool, startedScope,
		"school login must start the challenge with the school scope")
	assert.Equal(t, tenantID, startedTenantID)
}

func TestIssueSchoolTokens_NoPortalRole_Refused(t *testing.T) {
	// The school MFA verify path must not mint school tokens for an account
	// whose portal role disappeared between challenge start and verify.
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	service := setupAuthService(t, db)

	const tenantID int64 = 42
	testpkg.EnsureTestTenant(t, db, tenantID)
	email, username := uniqueTestCredentials("school-issue-no-role")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	_, _, err = service.IssueSchoolTokensForAuthenticatedAccount(context.Background(), account.ID, tenantID, "", "")

	require.Error(t, err)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
}
