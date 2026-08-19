package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// completeMFAExchangeStub is a narrow AuthService stub for the verify
// success-path tests. It only implements IssueTokensForAuthenticatedAccount;
// any other method panics so accidental drift in the handler shape gets
// caught loudly. (Distinct from loginGateStub to avoid coupling its
// recorder fields to these tests — and to keep the no-test-modifications
// guarantee on the existing login tests.)
type completeMFAExchangeStub struct {
	authService.AuthService
	access  string
	refresh string
	err     error

	gotAccountID int64
	gotTenantID  int64
	gotIP        string
	gotUA        string
}

var _ authService.AuthService = (*completeMFAExchangeStub)(nil)

func (s *completeMFAExchangeStub) IssueTokensForAuthenticatedAccount(
	_ context.Context,
	accountID, tenantID int64,
	ipAddress, userAgent string,
) (string, string, error) {
	s.gotAccountID = accountID
	s.gotTenantID = tenantID
	s.gotIP = ipAddress
	s.gotUA = userAgent
	return s.access, s.refresh, s.err
}

// trustedDeviceMFAStub extends the standard stubMFAService with controllable
// IssueTrustedDevice + VerifyChallenge funcs so the verify success path
// (which both call into) is exercisable end-to-end.
type trustedDeviceMFAStub struct {
	stubMFAService
	issueCookie    string
	issueExpiresAt time.Time
	issueErr       error
	verifyResult   *authService.VerifiedChallenge
	verifyErr      error

	gotIssueAccountID int64
	gotIssueTenantID  int64
	gotIssueUA        string
	gotIssueIP        net.IP
}

func (s *trustedDeviceMFAStub) VerifyChallenge(_ context.Context, _, _ string) (*authService.VerifiedChallenge, error) {
	return s.verifyResult, s.verifyErr
}

func (s *trustedDeviceMFAStub) IssueTrustedDevice(_ context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	s.gotIssueAccountID = accountID
	s.gotIssueTenantID = tenantID
	s.gotIssueUA = userAgent
	s.gotIssueIP = ip
	return s.issueCookie, s.issueExpiresAt, s.issueErr
}

func verifyRequest(t *testing.T, body MFAVerifyRequest) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "test-ua")
	r.Header.Set("X-Real-IP", "203.0.113.10")
	r.RemoteAddr = "203.0.113.10:54321"
	return r
}

func TestMFAVerify_SuccessReturnsTokenPair(t *testing.T) {
	t.Parallel()

	auth := &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{
			AccountID: 4242,
			Scope:     "tenant",
			TenantID:  70010001,
		},
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp TokenResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "access-tok", resp.AccessToken)
	assert.Equal(t, "refresh-tok", resp.RefreshToken)

	assert.Equal(t, int64(4242), auth.gotAccountID,
		"verified.AccountID must flow through to IssueTokens unchanged")
	assert.Equal(t, int64(70010001), auth.gotTenantID)
	assert.Equal(t, "203.0.113.10", auth.gotIP)
	assert.Equal(t, "test-ua", auth.gotUA)

	// remember_device defaulted to false → no Set-Cookie write
	assert.Empty(t, rr.Result().Cookies(),
		"trusted-device cookie must only be set when the client opts in")
}

func TestMFAVerify_RememberDeviceIssuesTrustedCookie(t *testing.T) {
	t.Parallel()

	auth := &completeMFAExchangeStub{access: "a", refresh: "r"}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{
			AccountID: 4242,
			TenantID:  70010001,
		},
		issueCookie:    "td.cookie.value",
		issueExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
		RememberDevice: true,
	}))

	require.Equal(t, http.StatusOK, rr.Code)
	cookies := rr.Result().Cookies()
	require.Len(t, cookies, 1, "remember_device=true must issue exactly one trusted-device cookie")
	c := cookies[0]
	assert.Equal(t, trustedDeviceCookieName, c.Name)
	assert.Equal(t, "td.cookie.value", c.Value)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, "/", c.Path)
	assert.Greater(t, c.MaxAge, 0, "MaxAge must reflect the issuer's expiry")

	assert.Equal(t, int64(4242), mfa.gotIssueAccountID)
	assert.Equal(t, int64(70010001), mfa.gotIssueTenantID)
	assert.Equal(t, "test-ua", mfa.gotIssueUA)
	require.NotNil(t, mfa.gotIssueIP)
	assert.Equal(t, "203.0.113.10", mfa.gotIssueIP.String())
}

func TestMFAVerify_RememberDeviceFailureDoesNotBreakLogin(t *testing.T) {
	t.Parallel()

	// When IssueTrustedDevice errors the handler must still issue the
	// access/refresh pair — losing the cookie is a UX regression, not a
	// security one, so the login must succeed.
	auth := &completeMFAExchangeStub{access: "a", refresh: "r"}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{AccountID: 1, TenantID: testpkg.Tenant(t)},
		issueErr:     errors.New("cookie store down"),
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
		RememberDevice: true,
	}))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Result().Cookies(),
		"cookie issuance failure must NOT produce a Set-Cookie with garbage")
	var resp TokenResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.NotEmpty(t, resp.AccessToken)
}

func TestMFAVerify_RememberDeviceEmptyCookieSkipsHeader(t *testing.T) {
	t.Parallel()

	// Tenants can disable trusted devices via the settings registry; the
	// service signals this by returning an empty cookie value. The handler
	// must not write a Set-Cookie at all in that case — otherwise the
	// browser stores a useless empty cookie that masks the disabled state.
	auth := &completeMFAExchangeStub{access: "a", refresh: "r"}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{AccountID: 1, TenantID: testpkg.Tenant(t)},
		issueCookie:  "", // tenant has trusted devices disabled
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
		RememberDevice: true,
	}))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Result().Cookies(),
		"empty cookie value must skip Set-Cookie entirely")
}

func TestMFAVerify_IssueTokensFailureSurfacesAsUnauthorized(t *testing.T) {
	t.Parallel()

	// IssueTokensForAuthenticatedAccount wraps the underlying repo error in
	// an *authService.AuthError. The handler must translate
	// ErrAccountInactive into a 401, not a 500, to match the rest of the
	// login surface.
	auth := &completeMFAExchangeStub{err: &authService.AuthError{
		Err: authService.ErrAccountInactive,
	}}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{AccountID: 1, TenantID: testpkg.Tenant(t)},
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"inactive accounts at the token-issuance step must collapse back to 401")
}

func TestMFAVerify_IssueTokensFailureForAccountNotFoundReturns401(t *testing.T) {
	t.Parallel()

	auth := &completeMFAExchangeStub{err: &authService.AuthError{
		Err: authService.ErrAccountNotFound,
	}}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{AccountID: 1, TenantID: testpkg.Tenant(t)},
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAVerify_IssueTokensUnknownErrorMapsTo500(t *testing.T) {
	t.Parallel()

	auth := &completeMFAExchangeStub{err: errors.New("kafka is on fire")}
	mfa := &trustedDeviceMFAStub{
		verifyResult: &authService.VerifiedChallenge{AccountID: 1, TenantID: testpkg.Tenant(t)},
	}
	rs := &Resource{AuthService: auth, MFAService: mfa}

	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, verifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	assert.Equal(t, http.StatusInternalServerError, rr.Code,
		"non-AuthError failures must fall through to 500")
}

// Ensure the embedded stub still satisfies authService.MFAService.
var _ authService.MFAService = (*trustedDeviceMFAStub)(nil)

// authModels is imported only to keep the stubMFAService field types
// resolvable; suppress the unused-import warning when the field set
// shrinks.
var _ = authModels.MFATrustedDevice{}
