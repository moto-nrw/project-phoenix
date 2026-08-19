// Package auth_test — integration tests for the school-portal login (#2207):
// LoginSchoolWithMFAGate, SwitchSchool, the school-scope refresh round-trip,
// and the school-portal role gate on every token-minting path.
package auth_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The pool is closed via t.Cleanup rather than defer throughout this file:
// deferred closes run BEFORE the fixture cleanups registered later, so every
// teardown query hit an already-closed pool and left its schools, accounts and
// audit rows behind in the shared test database.

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

// newSchoolTenant creates a school nobody else in the suite shares and tears
// it down again. These tests flip `active` and stamp `deleted_at` on the
// school row itself, so a literal tenant id (EnsureTestTenant's ON CONFLICT
// DO NOTHING) would have them mutating rows a parallel package is reading.
func newSchoolTenant(t *testing.T, db *bun.DB) (tenantID int64, subdomain string) {
	t.Helper()
	tenantID, subdomain = testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })
	return tenantID, subdomain
}

// newSchoolAccount registers an account and maps it to the tenant, without a
// school-portal role — the starting point for the "mapped but no portal role"
// gates.
func newSchoolAccount(t *testing.T, db *bun.DB, service auth.AuthService, prefix string, tenantID int64) (email string, accountID int64) {
	t.Helper()
	email, username := uniqueTestCredentials(prefix)
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	return email, account.ID
}

// createLehrkraftAccount registers an account, maps it to the tenant, and
// assigns the lehrkraft system role there.
func createLehrkraftAccount(t *testing.T, db *bun.DB, service auth.AuthService, prefix string, tenantID int64) (email string, accountID int64) {
	t.Helper()
	email, accountID = newSchoolAccount(t, db, service, prefix, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantID)
	return email, accountID
}

// missingAccountID is an id no fixture ever produces — used for the
// "account vanished between challenge and exchange" gates.
const missingAccountID int64 = 999999999

// switchIP / switchUserAgent stand in for the request metadata the handler
// threads into SwitchSchool. They must be non-empty: generateAndLogTokens
// skips the audit write entirely when the IP is empty, so passing "" here
// would make the audit assertions below vacuous.
const (
	switchIP        = "203.0.113.10"
	switchUserAgent = "school-portal-test-agent"
)

// requireAuthEventEventually waits for an audit.auth_events row of the given
// type for the account AT the tenant — the school flows must attribute their
// events to the school they actually acted on, not to the account's oldest
// mapping. logAuthEvent writes from a detached goroutine, hence the polling.
func requireAuthEventEventually(t *testing.T, db *bun.DB, accountID, tenantID int64, eventType string) {
	t.Helper()
	require.Eventually(t, func() bool {
		count, err := db.NewSelect().
			TableExpr("audit.auth_events").
			Where("account_id = ?", accountID).
			Where("tenant_id = ?", tenantID).
			Where("event_type = ?", eventType).
			Count(context.Background())
		return err == nil && count > 0
	}, 5*time.Second, 25*time.Millisecond,
		"expected a %q audit event for account %d at tenant %d", eventType, accountID, tenantID)
}

// setAccountActive flips auth.accounts.active for the deactivation gates.
func setAccountActive(t *testing.T, db *bun.DB, accountID int64, active bool) {
	t.Helper()
	_, err := db.Exec("UPDATE auth.accounts SET active = ? WHERE id = ?", active, accountID)
	require.NoError(t, err)
}

// setAccountTenantStatus flips auth.account_tenants.status — how membership
// in a school is actually revoked (the mapping row survives, its status does
// not).
func setAccountTenantStatus(t *testing.T, db *bun.DB, accountID, tenantID int64, status string) {
	t.Helper()
	_, err := db.Exec(
		"UPDATE auth.account_tenants SET status = ? WHERE account_id = ? AND tenant_id = ?",
		status, accountID, tenantID)
	require.NoError(t, err)
}

// setSchoolActive flips platform.schools.active — the operator switch that
// must cut every school-portal token mint.
func setSchoolActive(t *testing.T, db *bun.DB, tenantID int64, active bool) {
	t.Helper()
	_, err := db.Exec("UPDATE platform.schools SET active = ? WHERE id = ?", active, tenantID)
	require.NoError(t, err)
}

func TestLoginSchool_NoPortalRole_Refused(t *testing.T) {
	t.Parallel()

	// An account that is mapped to a school but holds no school-portal role
	// (here: no role at all) must be refused with the portal sentinel — the
	// handler maps it to 403.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := newSchoolAccount(t, db, service, "school-no-role", tenantID)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.Error(t, err)
	assert.Nil(t, result)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
}

func TestLoginSchool_Lehrkraft_IssuesSchoolScopedTokens(t *testing.T) {
	t.Parallel()

	// Happy path: lehrkraft system role at one school → token pair with
	// scope=school and the school pinned as tenant_id (school tokens are
	// tenant-bound, unlike parent tokens).
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-happy", tenantID)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-wrong-pw", tenantID)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, "Definitely-Wrong-1%", "", "", "")

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestRefreshToken_SchoolScope_Preserved(t *testing.T) {
	t.Parallel()

	// A school-scope refresh token must round-trip as a school token — a
	// silent demotion to tenant scope would fail SchoolMiddleware on the
	// next request and dead-end the portal session.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-refresh", tenantID)

	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err)

	accessToken, refreshToken, err := service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "")
	require.NoError(t, err, "school-scope refresh must succeed while the role exists")
	require.NotEmpty(t, refreshToken)

	scope, tokenTenantID := decodeTokenClaims(t, accessToken)
	assert.Equal(t, tenant.ScopeSchool, scope, "refreshed access token must stay school-scope")
	assert.Equal(t, tenantID, tokenTenantID)
}

func TestRefreshToken_SchoolScopeWithoutTenant_RejectedBeforeRotation(t *testing.T) {
	t.Parallel()

	// A school-scope refresh token that names no school is refused, and the
	// refusal must land BEFORE the rotation. Judged only afterwards (as it
	// was), the rotation had already resolved the account's default mapping,
	// minted a successor for a school this session never proved access to, and
	// burned the presented token on the way to the error.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-refresh-no-tenant", tenantID)

	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err)

	// Same session, same DB token row — only the tenant claim is stripped.
	tokenAuth := authjwt.MustNewTokenAuth()
	decoded, err := tokenAuth.JwtAuth.Decode(login.RefreshToken)
	require.NoError(t, err)
	var sessionToken string
	require.NoError(t, decoded.Get("token", &sessionToken))
	tenantlessRefresh, err := tokenAuth.CreateRefreshJWT(authjwt.RefreshClaims{
		ID:    int(accountID),
		Token: sessionToken,
		Scope: tenant.ScopeSchool,
	})
	require.NoError(t, err)

	_, _, err = service.RefreshTokenWithAudit(context.Background(), tenantlessRefresh, "", "")
	require.Error(t, err, "a school token without a school must not refresh")
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrTenantAccessDenied)

	// The session itself is untouched: nothing rotated, so its own refresh
	// token still works and still carries the school.
	accessToken, _, err := service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "")
	require.NoError(t, err, "the rejected refresh must not have consumed the session's token")
	scope, tokenTenantID := decodeTokenClaims(t, accessToken)
	assert.Equal(t, tenant.ScopeSchool, scope)
	assert.Equal(t, tenantID, tokenTenantID)
}

func TestRefreshToken_SchoolScope_RoleRevoked_Rejected(t *testing.T) {
	t.Parallel()

	// Revoking the lehrkraft role must cut the session at the next refresh:
	// the refresh path re-verifies the school-portal role instead of
	// trusting the (still unexpired) refresh token.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-revoked", tenantID)

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
	t.Parallel()

	// A Lehrkraft mapped to two schools can switch only where the portal
	// role exists: mapping alone (school B without lehrkraft role) is not
	// enough.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantA, _ := newSchoolTenant(t, db)
	tenantB, slugB := newSchoolTenant(t, db)
	_, accountID := createLehrkraftAccount(t, db, service, "school-switch", tenantA)

	testpkg.MapAccountToTenant(t, db, accountID, tenantB)

	t.Run("mapping without portal role at target is refused", func(t *testing.T) {
		_, _, err := service.SwitchSchool(context.Background(), accountID, slugB, switchIP, switchUserAgent)
		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
	})

	t.Run("portal role at target switches the pinned school", func(t *testing.T) {
		testpkg.AssignLehrkraftSystemRole(t, db, accountID, tenantB)

		accessToken, refreshToken, err := service.SwitchSchool(context.Background(), accountID, slugB, switchIP, switchUserAgent)
		require.NoError(t, err)
		require.NotEmpty(t, refreshToken)

		scope, tokenTenantID := decodeTokenClaims(t, accessToken)
		assert.Equal(t, tenant.ScopeSchool, scope)
		assert.Equal(t, tenantB, tokenTenantID)

		// The switch must leave a trace at the TARGET school. Without the
		// request IP reaching generateAndLogTokens this event is silently
		// never written — a school switch that nothing records.
		requireAuthEventEventually(t, db, accountID, tenantB, audit.EventTypeTenantSwitch)
	})
}

func TestLoginSchool_MFARequiredEnrolled_StartsSchoolScopedChallenge(t *testing.T) {
	t.Parallel()

	// With MFA required and enrolled, the school login must hand out a
	// challenge started with the SCHOOL challenge scope — that is what
	// keeps the challenge redeemable only at the school verify endpoint.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-mfa", tenantID)

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
	t.Parallel()

	// The school MFA verify path must not mint school tokens for an account
	// whose portal role disappeared between challenge start and verify.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	_, accountID := newSchoolAccount(t, db, service, "school-issue-no-role", tenantID)

	_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(context.Background(), accountID, tenantID, "", "")

	require.Error(t, err)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
}

func TestLoginSchool_InactiveSchool_Refused(t *testing.T) {
	t.Parallel()

	// An operator deactivating a school must cut its Lehrkräfte's school
	// logins: the portal-tenant finder skips inactive schools, so an
	// account whose only lehrkraft school is deactivated is refused.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-inactive", tenantID)

	setSchoolActive(t, db, tenantID, false)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.Error(t, err)
	assert.Nil(t, result)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
}

func TestLoginSchool_DeadOldestSchool_PicksNextValidSchool(t *testing.T) {
	t.Parallel()

	// A soft-deleted school in the OLDEST mapping must not shadow a valid
	// second school: the finder skips dead schools instead of pinning the
	// login to them.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	// Oldest mapping first: the dead school.
	deadTenantID, _ := newSchoolTenant(t, db)
	liveTenantID, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-dead-first", deadTenantID)
	testpkg.MapAccountToTenant(t, db, accountID, liveTenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, liveTenantID)

	_, err := db.Exec("UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?", deadTenantID)
	require.NoError(t, err)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusAuthenticated, result.Status)
	_, tokenTenantID := decodeTokenClaims(t, result.AccessToken)
	assert.Equal(t, liveTenantID, tokenTenantID,
		"login must pin the surviving school, not fail on the dead oldest mapping")
}

func TestLoginSchool_TrustedDevice_SkipsChallenge(t *testing.T) {
	t.Parallel()

	// A trusted-device cookie proven for (account, school) replaces the
	// second factor: the login must mint the token pair directly instead
	// of starting another email-code challenge.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-trusted", tenantID)

	var verifiedCookie string
	var verifiedTenantID int64
	service.SetMFAService(&authtest.MFAServiceMock{
		IsRequiredFn:    func(context.Context, *authModels.Account, int64) (bool, error) { return true, nil },
		HasEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
		VerifyTrustedDeviceFn: func(_ context.Context, _, cookieTenantID int64, cookie string) (bool, error) {
			verifiedCookie = cookie
			verifiedTenantID = cookieTenantID
			return true, nil
		},
		StartChallengeFn: func(context.Context, int64, int64, string, net.IP) (string, error) {
			t.Error("a verified trusted device must not start another challenge")
			return "", nil
		},
	})
	defer service.SetMFAService(nil)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "trusted-cookie")

	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusAuthenticated, result.Status)
	require.NotEmpty(t, result.AccessToken)
	assert.Equal(t, "trusted-cookie", verifiedCookie)
	assert.Equal(t, tenantID, verifiedTenantID, "the trusted device must be checked against the resolved school")

	scope, tokenTenantID := decodeTokenClaims(t, result.AccessToken)
	assert.Equal(t, tenant.ScopeSchool, scope)
	assert.Equal(t, tenantID, tokenTenantID)
}

func TestLoginSchool_MFAStatusUnavailable_FailsClosed(t *testing.T) {
	t.Parallel()

	// Fail-closed gate: when the MFA status cannot be determined the login
	// is refused with the 503 sentinel instead of silently degrading to
	// "MFA not required".
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-mfa-infra", tenantID)
	defer service.SetMFAService(nil)

	mfaErr := errors.New("mfa backend unreachable")

	t.Run("IsRequired lookup fails", func(t *testing.T) {
		service.SetMFAService(&authtest.MFAServiceMock{
			IsRequiredFn: func(context.Context, *authModels.Account, int64) (bool, error) { return false, mfaErr },
		})

		result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

		require.Error(t, err)
		assert.Nil(t, result)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrMFAStatusUnavailable)
	})

	t.Run("enrollment lookup fails", func(t *testing.T) {
		service.SetMFAService(&authtest.MFAServiceMock{
			IsRequiredFn:    func(context.Context, *authModels.Account, int64) (bool, error) { return true, nil },
			HasEnrollmentFn: func(context.Context, int64) (bool, error) { return false, mfaErr },
		})

		result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

		require.Error(t, err)
		assert.Nil(t, result)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrMFAStatusUnavailable)
	})

	t.Run("challenge start fails", func(t *testing.T) {
		service.SetMFAService(&authtest.MFAServiceMock{
			IsRequiredFn:    func(context.Context, *authModels.Account, int64) (bool, error) { return true, nil },
			HasEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
			StartChallengeFn: func(context.Context, int64, int64, string, net.IP) (string, error) {
				return "", auth.ErrMFARateLimited
			},
		})

		result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

		require.Error(t, err)
		assert.Nil(t, result)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrMFARateLimited)
	})
}

func TestIssueSchoolTokens_Lehrkraft_MintsSchoolScopedPair(t *testing.T) {
	t.Parallel()

	// The school MFA verify path mints the session once the second factor
	// is proven — same scope and school binding as the password login.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	_, accountID := createLehrkraftAccount(t, db, service, "school-issue", tenantID)

	accessToken, refreshToken, err := service.IssueSchoolTokensForAuthenticatedAccount(
		context.Background(), accountID, tenantID, "", "")

	require.NoError(t, err)
	require.NotEmpty(t, refreshToken)

	scope, tokenTenantID := decodeTokenClaims(t, accessToken)
	assert.Equal(t, tenant.ScopeSchool, scope)
	assert.Equal(t, tenantID, tokenTenantID)

	refreshScope, refreshTenantID := decodeTokenClaims(t, refreshToken)
	assert.Equal(t, tenant.ScopeSchool, refreshScope)
	assert.Equal(t, tenantID, refreshTenantID)
}

func TestIssueSchoolTokens_AccountStateGates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	_, accountID := createLehrkraftAccount(t, db, service, "school-issue-gates", tenantID)

	t.Run("unknown account", func(t *testing.T) {
		_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(
			context.Background(), missingAccountID, tenantID, "", "")

		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountNotFound)
	})

	t.Run("deactivated account", func(t *testing.T) {
		setAccountActive(t, db, accountID, false)
		defer setAccountActive(t, db, accountID, true)

		_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(
			context.Background(), accountID, tenantID, "", "")

		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountInactive)
	})

	t.Run("deactivated school", func(t *testing.T) {
		// The school going dark between challenge start and verify must
		// cut the exchange — the token would otherwise open a portal the
		// operator just switched off.
		setSchoolActive(t, db, tenantID, false)
		defer setSchoolActive(t, db, tenantID, true)

		_, _, err := service.IssueSchoolTokensForAuthenticatedAccount(
			context.Background(), accountID, tenantID, "", "")

		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrTenantNotFound)
	})
}

func TestSwitchSchool_AccountStateAndSlugGates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, slug := newSchoolTenant(t, db)
	_, accountID := createLehrkraftAccount(t, db, service, "school-switch-gates", tenantID)

	t.Run("unknown account", func(t *testing.T) {
		_, _, err := service.SwitchSchool(context.Background(), missingAccountID, slug, switchIP, switchUserAgent)

		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountNotFound)
	})

	t.Run("deactivated account", func(t *testing.T) {
		setAccountActive(t, db, accountID, false)
		defer setAccountActive(t, db, accountID, true)

		_, _, err := service.SwitchSchool(context.Background(), accountID, slug, switchIP, switchUserAgent)

		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountInactive)
	})

	t.Run("unknown slug", func(t *testing.T) {
		_, _, err := service.SwitchSchool(context.Background(), accountID, "definitely-no-such-school", switchIP, switchUserAgent)

		require.Error(t, err)
		var authErr *auth.AuthError
		require.ErrorAs(t, err, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrTenantNotFound)
	})
}

func TestLoginSchool_MFAEnrollmentRequired_MintsSchoolScopedEnrollmentToken(t *testing.T) {
	t.Parallel()

	// MFA required + not enrolled → the enrollment token must carry the
	// SCHOOL enrollment scope. A tenant-scope token here would let the
	// enrollment detour convert a school login into a tenant session.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, _ := createLehrkraftAccount(t, db, service, "school-enroll", tenantID)

	service.SetMFAService(&authtest.MFAServiceMock{
		IsRequiredFn:    func(context.Context, *authModels.Account, int64) (bool, error) { return true, nil },
		HasEnrollmentFn: func(context.Context, int64) (bool, error) { return false, nil },
	})
	defer service.SetMFAService(nil)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusMFAEnrollmentRequired, result.Status)
	require.NotEmpty(t, result.AccessToken)

	claims, err := authjwt.MustNewTokenAuth().ParseMFAEnrollmentJWT(result.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, authjwt.MFAEnrollmentScopeSchool, claims.Scope,
		"school login must mint a school-scope enrollment token")
	assert.Equal(t, tenantID, claims.TenantID)
}

func TestSchoolTokenMints_RevokedMembership_Refused(t *testing.T) {
	t.Parallel()

	// Membership is revoked by flipping auth.account_tenants.status, not by
	// touching roles. Every path that mints a school token must re-read that
	// status: an in-flight MFA challenge, a school switch, and a refresh all
	// used to trust the mapping resolved at password time and kept minting
	// tokens for an account already removed from the school.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, slug := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-revoked-mapping", tenantID)

	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusAuthenticated, login.Status)

	setAccountTenantStatus(t, db, accountID, tenantID, "inactive")

	t.Run("password login", func(t *testing.T) {
		// The portal-tenant finder only walks ACTIVE mappings, so the school
		// disappears entirely and the portal-role sentinel is the answer.
		_, loginErr := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, "", "", "")

		require.Error(t, loginErr)
		var authErr *auth.AuthError
		require.ErrorAs(t, loginErr, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrAccountNoSchoolPortalRole)
	})

	t.Run("mfa verify token issue", func(t *testing.T) {
		_, _, issueErr := service.IssueSchoolTokensForAuthenticatedAccount(
			context.Background(), accountID, tenantID, "", "")

		require.Error(t, issueErr)
		var authErr *auth.AuthError
		require.ErrorAs(t, issueErr, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrTenantAccessDenied)
	})

	t.Run("school switch", func(t *testing.T) {
		_, _, switchErr := service.SwitchSchool(context.Background(), accountID, slug, switchIP, switchUserAgent)

		require.Error(t, switchErr)
		var authErr *auth.AuthError
		require.ErrorAs(t, switchErr, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrTenantAccessDenied)
	})

	t.Run("refresh", func(t *testing.T) {
		_, _, refreshErr := service.RefreshTokenWithAudit(context.Background(), login.RefreshToken, "", "")

		require.Error(t, refreshErr)
		var authErr *auth.AuthError
		require.ErrorAs(t, refreshErr, &authErr)
		assert.ErrorIs(t, authErr.Err, auth.ErrTenantAccessDenied)
	})
}

func TestIssueSchoolTokens_SoftDeletedSchool_Refused(t *testing.T) {
	t.Parallel()

	// A school soft-deleted while `active` stays true must still cut the
	// token mint — the liveness gate checks deleted_at as well, not only the
	// active flag.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	_, accountID := createLehrkraftAccount(t, db, service, "school-soft-deleted", tenantID)

	_, err := db.Exec("UPDATE platform.schools SET deleted_at = NOW(), active = true WHERE id = ?", tenantID)
	require.NoError(t, err)

	_, _, err = service.IssueSchoolTokensForAuthenticatedAccount(
		context.Background(), accountID, tenantID, "", "")

	require.Error(t, err)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrTenantNotFound)
}

func TestLogout_SchoolSession_AuditedAtTheSessionsSchool(t *testing.T) {
	t.Parallel()

	// /auth/logout is a pre-deauthentication route: nothing has put a tenant
	// in the context by the time the event is written. logAuthEvent then
	// falls back to the account's FIRST active mapping, so a Lehrkraft who
	// teaches at one school and is merely mapped to another had every logout
	// filed under the school they never logged into. The tenant on the token
	// row is what the session actually was.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	// Mapped first (and therefore the oldest mapping — the wrong answer the
	// fallback would produce), with no school-portal role.
	otherTenantID, _ := newSchoolTenant(t, db)
	schoolTenantID, _ := newSchoolTenant(t, db)

	email, accountID := newSchoolAccount(t, db, service, "school-logout", otherTenantID)
	testpkg.MapAccountToTenant(t, db, accountID, schoolTenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, accountID, schoolTenantID)

	login, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, switchIP, switchUserAgent, "")
	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusAuthenticated, login.Status)
	_, tokenTenantID := decodeTokenClaims(t, login.AccessToken)
	require.Equal(t, schoolTenantID, tokenTenantID, "the session must be pinned to the school with the portal role")

	require.NoError(t, service.LogoutWithAudit(context.Background(), login.RefreshToken, switchIP, switchUserAgent))

	requireAuthEventEventually(t, db, accountID, schoolTenantID, audit.EventTypeLogout)

	count, err := db.NewSelect().
		TableExpr("audit.auth_events").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", otherTenantID).
		Where("event_type = ?", audit.EventTypeLogout).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count, "the logout must not be filed under a school the session never belonged to")
}

func TestLoginSchool_MFARequirementAppearingMidLogin_ChallengesInsteadOfMinting(t *testing.T) {
	t.Parallel()

	// The MFA gate runs before the token transaction, so its verdict is a
	// snapshot. Under `required_admins` an admin role granted in that window
	// used to produce a full school session that never saw a second factor:
	// the mint guard re-checked membership and portal role, but not the gate.
	//
	// The race is reproduced deterministically by granting the role from
	// inside the policy lookup — the last thing that happens before the gate
	// decides, using the roles loaded a moment earlier.
	//
	// The grant stays on the PRE-TRANSACTION lookup. The guard's own re-read
	// runs inside the mint transaction, which holds auth.accounts FOR UPDATE;
	// an account_roles INSERT from another connection would block on that row's
	// foreign key there and hang the test instead of exercising it. That is not
	// a test artifact — it is the reason the in-transaction resolver reads on
	// the caller's transaction and never opens a second one.
	db := testpkg.SetupTestDB(t)
	service := setupAuthService(t, db)

	tenantID, _ := newSchoolTenant(t, db)
	email, accountID := createLehrkraftAccount(t, db, service, "school-mfa-race", tenantID)

	// The seeded admin system role, not a fixture one: `required_admins`
	// matches the literal role name, and CreateTestRoleForTenant uniquifies it.
	var adminRoleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", "admin").
		Where("is_system = TRUE").
		Scan(context.Background(), &adminRoleID))

	var startedScope string
	service.SetMFAService(&authtest.MFAServiceMock{
		ResolvePolicyFn: func(_ context.Context, policyAccountID, _ int64) (auth.MFAPolicy, error) {
			assignment := &authModels.AccountRole{AccountID: policyAccountID, RoleID: adminRoleID}
			assignment.SetTenantID(tenantID)
			_, err := db.NewInsert().Model(assignment).ModelTableExpr(`auth.account_roles`).Exec(context.Background())
			require.NoError(t, err)
			return auth.MFAPolicyForMode(configModels.MFAModeRequiredAdmins), nil
		},
		// The guard re-reads the policy inside the mint transaction; the school
		// still says `required_admins`, and this time it meets the admin role.
		ResolvePolicyInTxFn: func(context.Context, int64, int64) (auth.MFAPolicy, error) {
			return auth.MFAPolicyForMode(configModels.MFAModeRequiredAdmins), nil
		},
		HasEnrollmentFn: func(context.Context, int64) (bool, error) { return true, nil },
		StartChallengeFn: func(_ context.Context, _, _ int64, scope string, _ net.IP) (string, error) {
			startedScope = scope
			return "school-race-challenge", nil
		},
	})
	defer service.SetMFAService(nil)

	result, err := service.LoginSchoolWithMFAGate(context.Background(), email, testPassword, switchIP, switchUserAgent, "")

	require.NoError(t, err)
	require.Equal(t, auth.LoginStatusMFARequired, result.Status,
		"the guard must send the login back to the second factor instead of minting")
	assert.Equal(t, "school-race-challenge", result.ChallengeToken)
	assert.Equal(t, authjwt.MFAChallengeScopeSchool, startedScope)
	assert.Empty(t, result.RefreshToken, "no session may be issued for a login that owes a second factor")

	count, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", accountID).
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count, "the aborted mint must leave no refresh token behind")
}
