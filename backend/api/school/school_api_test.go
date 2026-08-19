// Router-level tests for the school portal (#2207): the token matrix that
// pins the scope isolation (school tokens work ONLY on /school/*, every
// other scope is refused there), the school login handler's portal-role
// gate, and the scope binding of the school MFA surface.
package school_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/classday"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/api/school"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const testPassword = "Test1234%" //nolint:gosec // test credential

// setupSchoolTest wires the API stack and hands out a school nobody else in
// the suite shares. Each SetupAPITest call opens its OWN connection pool, so
// the close is registered first and therefore runs last — after every fixture
// cleanup that still needs the handle.
func setupSchoolTest(t *testing.T) (*bun.DB, *services.Factory, int64, string) {
	t.Helper()

	db, factory := testutil.SetupAPITest(t)

	tenantID, subdomain := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })

	return db, factory, tenantID, subdomain
}

// newSchoolRouter builds the school portal router, optionally with a stubbed
// MFA service (pass nil for the real one from the factory).
func newSchoolRouter(db *bun.DB, factory *services.Factory, mfa authService.MFAService) http.Handler {
	classDayResource := classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil)
	if mfa == nil {
		return school.NewResource(factory.Auth, factory.MFA, classDayResource).Router()
	}
	return school.NewResource(factory.Auth, mfa, classDayResource).Router()
}

// registerLehrkraft creates an account at the tenant carrying the lehrkraft
// system role — the only role that opens the school portal today.
func registerLehrkraft(t *testing.T, db *bun.DB, factory *services.Factory, tenantID int64, prefix string) (email string, accountID int64) {
	t.Helper()

	unique := time.Now().UnixNano()
	email = fmt.Sprintf("%s-%d@test.local", prefix, unique)
	account, err := factory.Auth.Register(testpkg.TenantContext(tenantID), email, fmt.Sprintf("%s-%d", prefix, unique), testPassword, nil, 0)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)
	return email, account.ID
}

func TestSchoolPortalTokenMatrix(t *testing.T) {
	db, factory, tenantID, _ := setupSchoolTest(t)

	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "School", fmt.Sprintf("Matrix-%d", time.Now().UnixNano()))
	className := fmt.Sprintf("sm%d", time.Now().UnixNano()%100000)
	assignment := testpkg.CreateTestClassTeacherForTenant(t, db, tenantID, staff.ID, className)
	t.Cleanup(func() {
		tenantCtx := testpkg.TenantContext(tenantID)
		_, _ = db.NewDelete().TableExpr("education.class_teachers").Where("id = ?", assignment.ID).Exec(tenantCtx)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	classDayResource := classday.NewResource(factory.EnrollmentReport, factory.UserContext, db, nil)
	schoolRouter := school.NewResource(factory.Auth, factory.MFA, classDayResource).Router()
	tenantClassDayRouter := classDayResource.Router()

	schoolClaims := jwt.AppClaims{
		ID: int(account.ID), Sub: account.Email,
		Roles: []string{"lehrkraft"}, TenantID: tenantID,
		Scope: tenant.ScopeSchool,
	}

	// School token on the school class-day surface → 200.
	req := httptest.NewRequest(http.MethodGet, "/class-day/classes", nil)
	rec := testutil.ExecuteWithAuthPermissions(t, schoolRouter, req, schoolClaims, []string{"class_day:read"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), className)

	// School token on the TENANT class-day mount → 401. Same handlers, but
	// the tenant mantle refuses the school scope.
	req = httptest.NewRequest(http.MethodGet, "/classes", nil)
	rec = testutil.ExecuteWithAuthPermissions(t, tenantClassDayRouter, req, schoolClaims, []string{"class_day:read"})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

	// Every non-school scope on the school surface → 401, permission or not.
	foreignScopes := []struct {
		name   string
		claims jwt.AppClaims
	}{
		{"tenant scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeTenant}},
		{"org scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"admin"}, TenantID: tenantID, OrgID: tenantID, Scope: tenant.ScopeOrg}},
		{"platform scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"operator"}, Scope: tenant.ScopePlatform}},
		{"parent scope", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"guardian"}, Scope: tenant.ScopeParent}},
		{"school scope without tenant binding", jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, Scope: tenant.ScopeSchool}},
	}
	for _, tc := range foreignScopes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/class-day/classes", nil)
			rec := testutil.ExecuteWithAuthPermissions(t, schoolRouter, req, tc.claims, []string{"class_day:read"})
			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		})
	}

	// The protected school auth surface follows the same matrix: a tenant
	// token cannot switch schools.
	req = httptest.NewRequest(http.MethodPost, "/auth/switch-school", strings.NewReader(`{"tenant_slug":"t1"}`))
	req.Header.Set("Content-Type", "application/json")
	tenantClaims := jwt.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeTenant}
	rec = testutil.ExecuteWithAuthPermissions(t, schoolRouter, req, tenantClaims, []string{"class_day:read"})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestSchoolLoginHandler_PortalRoleGate(t *testing.T) {
	db, factory, tenantID, _ := setupSchoolTest(t)
	schoolRouter := newSchoolRouter(db, factory, nil)

	unique := time.Now().UnixNano()
	email := fmt.Sprintf("school-login-%d@test.local", unique)
	account, err := factory.Auth.Register(testpkg.TenantContext(tenantID), email, fmt.Sprintf("school-login-%d", unique), testPassword, nil, 0)
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	loginBody := fmt.Sprintf(`{"email":%q,"password":%q}`, email, testPassword)

	// Mapped, but no school-portal role → 403 with the stable portal code.
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "no_school_portal_role")

	// With the lehrkraft system role the same credentials authenticate and
	// the response carries the token pair.
	testpkg.AssignLehrkraftSystemRole(t, db, account.ID, tenantID)

	req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"authenticated"`)
	assert.Contains(t, rec.Body.String(), "access_token")

	// Wrong password stays a masked 401.
	req = httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(fmt.Sprintf(`{"email":%q,"password":"Wrong-1234%%"}`, email)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "invalid_credentials")
}

func TestSchoolMFAEndpoints_RejectForeignScopes(t *testing.T) {
	// The whole school MFA lifecycle must be scope-tight: a challenge or
	// enrollment token minted for the tenant portal is refused at every
	// school MFA endpoint — verify, resend (its code budget must not be
	// burnable through this surface), and both enroll steps.
	db, factory, tenantID, _ := setupSchoolTest(t)
	schoolRouter := newSchoolRouter(db, factory, nil)

	_, accountID := registerLehrkraft(t, db, factory, tenantID, "school-foreign-scope")

	tokenAuth := jwt.MustNewTokenAuth()
	tenantChallenge, err := tokenAuth.CreateMFAChallengeJWT(jwt.MFAChallengeClaims{
		AccountID: accountID,
		Scope:     jwt.MFAChallengeScopeTenant,
		TenantID:  tenantID,
	}, 5*time.Minute)
	require.NoError(t, err)
	tenantEnrollment, err := tokenAuth.CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
		AccountID: accountID,
		Scope:     jwt.MFAEnrollmentScopeTenant,
		TenantID:  tenantID,
	}, 5*time.Minute)
	require.NoError(t, err)

	t.Run("tenant challenge on school verify", func(t *testing.T) {
		body := fmt.Sprintf(`{"challenge_token":%q,"code":"123456"}`, tenantChallenge)
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		schoolRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})

	t.Run("tenant challenge on school resend", func(t *testing.T) {
		body := fmt.Sprintf(`{"challenge_token":%q}`, tenantChallenge)
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/resend", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		schoolRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})

	t.Run("tenant enrollment token on school enroll start", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/enroll/start", nil)
		req.Header.Set("Authorization", "Bearer "+tenantEnrollment)
		rec := httptest.NewRecorder()
		schoolRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})

	t.Run("tenant enrollment token on school enroll confirm", func(t *testing.T) {
		body := fmt.Sprintf(`{"challenge_token":%q,"code":"123456"}`, tenantChallenge)
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/enroll/confirm", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tenantEnrollment)
		rec := httptest.NewRecorder()
		schoolRouter.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	})
}

// TestSchoolMFAVerify_MintsSchoolSession pins the second half of the MFA
// login: once the emailed code is proven, the school surface must exchange
// the challenge for a SCHOOL-scope session — and it must ask the MFA
// service for the school scope while doing it.
func TestSchoolMFAVerify_MintsSchoolSession(t *testing.T) {
	db, factory, tenantID, _ := setupSchoolTest(t)

	_, accountID := registerLehrkraft(t, db, factory, tenantID, "school-mfa-verify")

	var requestedScope string
	mfa := &authtest.MFAServiceMock{
		VerifyChallengeForScopeFn: func(_ context.Context, _, _, scope string) (*authService.VerifiedChallenge, error) {
			requestedScope = scope
			return &authService.VerifiedChallenge{AccountID: accountID, Scope: scope, TenantID: tenantID}, nil
		},
	}
	schoolRouter := newSchoolRouter(db, factory, mfa)

	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify",
		strings.NewReader(`{"challenge_token":"school-challenge","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, jwt.MFAChallengeScopeSchool, requestedScope,
		"the school verify endpoint must redeem the challenge for the school scope only")

	var tokens school.TokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))
	require.NotEmpty(t, tokens.AccessToken)
	require.NotEmpty(t, tokens.RefreshToken)

	decoded, err := jwt.MustNewTokenAuth().JwtAuth.Decode(tokens.AccessToken)
	require.NoError(t, err)
	var scope string
	require.NoError(t, decoded.Get("scope", &scope))
	assert.Equal(t, tenant.ScopeSchool, scope, "the MFA exchange must mint a school-scope session")
}

// TestSchoolMFAVerify_MembershipRevoked_Returns401 pins the response shape
// for the race the mint guard catches: the challenge was legitimate, but the
// account was removed from the school before it was redeemed. That is a
// terminal "you no longer belong here", not a server fault — a 500 would tell
// the frontend to retry forever.
func TestSchoolMFAVerify_MembershipRevoked_Returns401(t *testing.T) {
	db, factory, tenantID, _ := setupSchoolTest(t)

	_, accountID := registerLehrkraft(t, db, factory, tenantID, "school-mfa-revoked")

	_, err := db.ExecContext(context.Background(),
		"UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?",
		accountID, tenantID)
	require.NoError(t, err)

	mfa := &authtest.MFAServiceMock{
		VerifyChallengeForScopeFn: func(_ context.Context, _, _, scope string) (*authService.VerifiedChallenge, error) {
			return &authService.VerifiedChallenge{AccountID: accountID, Scope: scope, TenantID: tenantID}, nil
		},
	}
	schoolRouter := newSchoolRouter(db, factory, mfa)

	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify",
		strings.NewReader(`{"challenge_token":"school-challenge","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

// TestSchoolMFAResend_ForwardsSchoolScope pins that the resend endpoint
// hands the school scope down to the service — that check is what keeps a
// foreign-portal challenge's code budget unburnable from here.
func TestSchoolMFAResend_ForwardsSchoolScope(t *testing.T) {
	db, factory, _, _ := setupSchoolTest(t)

	var requestedScope string
	mfa := &authtest.MFAServiceMock{
		ResendChallengeForScopeFn: func(_ context.Context, _ string, _ net.IP, scope string) (string, error) {
			requestedScope = scope
			return "renewed-school-challenge", nil
		},
	}
	schoolRouter := newSchoolRouter(db, factory, mfa)

	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/resend",
		strings.NewReader(`{"challenge_token":"school-challenge"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, jwt.MFAChallengeScopeSchool, requestedScope)

	var renewed common.MFAResendResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &renewed))
	assert.Equal(t, "renewed-school-challenge", renewed.ChallengeToken)
}

// TestSchoolMFAStatusUnavailable_Returns503 pins the transient-failure
// mapping on the school MFA surface. The service fails closed when it cannot
// read the MFA status or the code rate-limit counter; answering that with a
// 500 tells the client the endpoint is broken and stops the retry that would
// have worked a second later.
func TestSchoolMFAStatusUnavailable_Returns503(t *testing.T) {
	db, factory, tenantID, _ := setupSchoolTest(t)

	_, accountID := registerLehrkraft(t, db, factory, tenantID, "school-mfa-unavailable")

	enrollmentToken, err := jwt.MustNewTokenAuth().CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
		AccountID: accountID,
		Scope:     jwt.MFAEnrollmentScopeSchool,
		TenantID:  tenantID,
	}, 5*time.Minute)
	require.NoError(t, err)

	mfa := &authtest.MFAServiceMock{
		ResendChallengeForScopeFn: func(_ context.Context, _ string, _ net.IP, _ string) (string, error) {
			return "", authService.ErrMFAStatusUnavailable
		},
		StartChallengeFn: func(_ context.Context, _, _ int64, _ string, _ net.IP) (string, error) {
			return "", authService.ErrMFAStatusUnavailable
		},
	}
	schoolRouter := newSchoolRouter(db, factory, mfa)

	resend := httptest.NewRequest(http.MethodPost, "/auth/mfa/resend",
		strings.NewReader(`{"challenge_token":"school-challenge"}`))
	resend.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, resend)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())

	enrollStart := httptest.NewRequest(http.MethodPost, "/auth/mfa/enroll/start", nil)
	enrollStart.Header.Set("Authorization", "Bearer "+enrollmentToken)
	rec = httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, enrollStart)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

// TestSchoolMFAEnroll_BoundToItsOwnChallenge pins the enrollment leg of the
// MFA flow to the exact challenge it started, not to "the account's newest
// active code". The account-wide lookup is redeemable by any concurrent
// challenge — the same person logging into the tenant portal in another tab
// is enough — so the confirm must go through the scope- and id-bound verify
// and must reject a challenge belonging to another account or school.
func TestSchoolMFAEnroll_BoundToItsOwnChallenge(t *testing.T) {
	db, factory, tenantID, _ := setupSchoolTest(t)

	_, accountID := registerLehrkraft(t, db, factory, tenantID, "school-enroll")

	enrollmentToken, err := jwt.MustNewTokenAuth().CreateMFAEnrollmentJWT(jwt.MFAEnrollmentClaims{
		AccountID: accountID,
		Scope:     jwt.MFAEnrollmentScopeSchool,
		TenantID:  tenantID,
	}, 5*time.Minute)
	require.NoError(t, err)

	// The challenge the enroll/start step handed out. verifiedFor decides
	// which (account, school) the mocked service says the redeemed challenge
	// belonged to.
	//
	// The owner-bound verifier is stubbed EXPLICITLY. MFAServiceMock falls back
	// from VerifyChallengeForOwner through VerifyChallengeForScope, so stubbing
	// only the scope-aware one would let this test stay green against a handler
	// that had quietly dropped the identity binding — the exact regression it
	// exists to catch. ownerBoundVerify records that the handler took the bound
	// path, and the stub asserts the pinned arguments in place.
	var (
		verifiedAccountID  int64
		verifiedTenantID   int64
		requestedScope     string
		requestedToken     string
		pinnedAccountID    int64
		pinnedTenantID     int64
		ownerBoundVerify   bool
		scopeOnlyVerify    bool
		accountWideVerify  bool
		startedScope       string
		startedChallengeID = "school-enroll-challenge"
	)
	mfa := &authtest.MFAServiceMock{
		StartChallengeFn: func(_ context.Context, _, _ int64, scope string, _ net.IP) (string, error) {
			startedScope = scope
			return startedChallengeID, nil
		},
		VerifyChallengeForOwnerFn: func(_ context.Context, challengeToken, _, scope string, accountID, tenantID int64) (*authService.VerifiedChallenge, error) {
			ownerBoundVerify = true
			requestedToken = challengeToken
			requestedScope = scope
			pinnedAccountID, pinnedTenantID = accountID, tenantID
			return &authService.VerifiedChallenge{AccountID: verifiedAccountID, Scope: scope, TenantID: verifiedTenantID}, nil
		},
		VerifyChallengeForScopeFn: func(_ context.Context, _, _, _ string) (*authService.VerifiedChallenge, error) {
			scopeOnlyVerify = true
			return &authService.VerifiedChallenge{AccountID: verifiedAccountID, Scope: jwt.MFAChallengeScopeSchool, TenantID: verifiedTenantID}, nil
		},
		VerifyCodeForAccountFn: func(context.Context, int64, int64, string, string) error {
			accountWideVerify = true
			return nil
		},
		EnrollFn: func(context.Context, int64) error { return nil },
	}
	schoolRouter := newSchoolRouter(db, factory, mfa)

	// enroll/start must hand the challenge token back — the confirm is bound
	// to it, so swallowing it would leave the client nothing to send.
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/enroll/start", nil)
	req.Header.Set("Authorization", "Bearer "+enrollmentToken)
	rec := httptest.NewRecorder()
	schoolRouter.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var started school.EnrollStartResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &started))
	assert.Equal(t, startedChallengeID, started.ChallengeToken)
	assert.Equal(t, jwt.MFAChallengeScopeSchool, startedScope)

	confirm := func(t *testing.T) *httptest.ResponseRecorder {
		t.Helper()
		body := fmt.Sprintf(`{"challenge_token":%q,"code":"123456"}`, started.ChallengeToken)
		req := httptest.NewRequest(http.MethodPost, "/auth/mfa/enroll/confirm", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+enrollmentToken)
		rec := httptest.NewRecorder()
		schoolRouter.ServeHTTP(rec, req)
		return rec
	}

	t.Run("challenge of another account is refused", func(t *testing.T) {
		verifiedAccountID, verifiedTenantID = accountID+1, tenantID
		require.Equal(t, http.StatusUnauthorized, confirm(t).Code)
	})

	t.Run("challenge of another school is refused", func(t *testing.T) {
		verifiedAccountID, verifiedTenantID = accountID, tenantID+1
		require.Equal(t, http.StatusUnauthorized, confirm(t).Code)
	})

	t.Run("its own challenge mints a school session", func(t *testing.T) {
		verifiedAccountID, verifiedTenantID = accountID, tenantID

		rec := confirm(t)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var tokens school.TokenResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))
		require.NotEmpty(t, tokens.AccessToken)

		decoded, err := jwt.MustNewTokenAuth().JwtAuth.Decode(tokens.AccessToken)
		require.NoError(t, err)
		var scope string
		require.NoError(t, decoded.Get("scope", &scope))
		assert.Equal(t, tenant.ScopeSchool, scope)
	})

	assert.True(t, ownerBoundVerify,
		"enroll confirm must redeem the challenge through the owner-bound verifier")
	assert.False(t, scopeOnlyVerify,
		"enroll confirm must not redeem through the scope-only verifier: that one consumes the code before the account/school comparison")
	assert.False(t, accountWideVerify,
		"enroll confirm must not fall back to the account-wide newest-code lookup")
	assert.Equal(t, startedChallengeID, requestedToken,
		"enroll confirm must redeem the exact challenge enroll/start handed out")
	assert.Equal(t, jwt.MFAChallengeScopeSchool, requestedScope,
		"enroll confirm must redeem the challenge for the school scope")
	assert.Equal(t, accountID, pinnedAccountID,
		"the enrollment token's account must be pinned INSIDE the verify, before the code is consumed")
	assert.Equal(t, tenantID, pinnedTenantID,
		"the enrollment token's school must be pinned INSIDE the verify, before the code is consumed")
}
