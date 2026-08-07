package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// loginGateStub satisfies authService.AuthService by embedding the
// interface — only LoginWithMFAGate is implemented; any other method
// reached by the handler under test will panic with a nil-interface
// dereference, which is exactly the signal we want for "the handler
// drifted away from the planned shape".
type loginGateStub struct {
	authService.AuthService
	result          *authService.LoginResult
	err             error
	gotEmail        string
	gotPassword     string
	gotIPAddress    string
	gotUserAgent    string
	gotTenantSlug   string
	gotTrustedToken string
}

// Compile-time assertion the stub satisfies the interface.
var _ authService.AuthService = (*loginGateStub)(nil)

func (s *loginGateStub) LoginWithMFAGate(
	_ context.Context,
	email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string,
) (*authService.LoginResult, error) {
	s.gotEmail = email
	s.gotPassword = password
	s.gotIPAddress = ipAddress
	s.gotUserAgent = userAgent
	s.gotTenantSlug = tenantSlug
	s.gotTrustedToken = trustedDeviceCookie
	return s.result, s.err
}

func newLoginRequest(t *testing.T, email, password string) *http.Request {
	t.Helper()
	body, err := json.Marshal(LoginRequest{Email: email, Password: password})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestLogin_MFARequiredResponseShape checks the new discriminated JSON shape
// when the gate decides the user needs a second factor.
func TestLogin_MFARequiredResponseShape(t *testing.T) {
	rs := &Resource{
		AuthService: &loginGateStub{
			result: &authService.LoginResult{
				Status:         authService.LoginStatusMFARequired,
				ChallengeToken: "fake.challenge.jwt",
				MaskedEmail:    "j***@example.com",
			},
		},
	}

	rr := httptest.NewRecorder()
	rs.login(rr, newLoginRequest(t, "jane@example.com", "Test1234%"))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp LoginResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "mfa_required", resp.Status)
	assert.Equal(t, "fake.challenge.jwt", resp.ChallengeToken)
	assert.Equal(t, "j***@example.com", resp.MaskedEmail)
	assert.Empty(t, resp.AccessToken, "tokens must NOT leak in mfa_required response")
	assert.Empty(t, resp.RefreshToken, "tokens must NOT leak in mfa_required response")
}

// TestLogin_AuthenticatedResponseShape is the regression guard for the
// non-MFA path: regular logins still return access + refresh tokens.
func TestLogin_AuthenticatedResponseShape(t *testing.T) {
	rs := &Resource{
		AuthService: &loginGateStub{
			result: &authService.LoginResult{
				Status:       authService.LoginStatusAuthenticated,
				AccessToken:  "access.tok",
				RefreshToken: "refresh.tok",
			},
		},
	}

	rr := httptest.NewRecorder()
	rs.login(rr, newLoginRequest(t, "jane@example.com", "Test1234%"))

	require.Equal(t, http.StatusOK, rr.Code)
	var resp LoginResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "authenticated", resp.Status)
	assert.Equal(t, "access.tok", resp.AccessToken)
	assert.Equal(t, "refresh.tok", resp.RefreshToken)
	assert.Empty(t, resp.ChallengeToken)
	assert.False(t, resp.MFAEnrollmentRequired)
}

// TestLogin_EnrollmentRequiredEmitsScopedToken covers the new
// post-#1430 contract: when MFA is required but the account has no
// credential yet, the response carries status=mfa_enrollment_required
// plus an enrollment-scoped JWT in `access_token` and NO `refresh_token`.
// The previous "full session + flag" shape was the bypass the review
// flagged as critical.
func TestLogin_EnrollmentRequiredEmitsScopedToken(t *testing.T) {
	rs := &Resource{
		AuthService: &loginGateStub{
			result: &authService.LoginResult{
				Status:                authService.LoginStatusMFAEnrollmentRequired,
				AccessToken:           "enrollment.scoped.tok",
				MaskedEmail:           "x***@y.de",
				MFAEnrollmentRequired: true,
			},
		},
	}

	rr := httptest.NewRecorder()
	rs.login(rr, newLoginRequest(t, "x@y.de", "Test1234%"))

	var resp LoginResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "mfa_enrollment_required", resp.Status)
	assert.Equal(t, "enrollment.scoped.tok", resp.AccessToken)
	assert.Empty(t, resp.RefreshToken,
		"no refresh token is issued before MFA enrollment — closes the pre-MFA-bypass hole")
	assert.True(t, resp.MFAEnrollmentRequired)
}

// TestLogin_PassesTrustedDeviceCookie ensures the handler reads the
// trust-device cookie off the request and forwards it to the gate so the
// gate can skip MFA for previously-trusted browsers.
func TestLogin_PassesTrustedDeviceCookie(t *testing.T) {
	stub := &loginGateStub{
		result: &authService.LoginResult{
			Status:       authService.LoginStatusAuthenticated,
			AccessToken:  "a",
			RefreshToken: "r",
		},
	}
	rs := &Resource{AuthService: stub}

	req := newLoginRequest(t, "j@example.com", "Test1234%")
	req.AddCookie(&http.Cookie{Name: trustedDeviceCookieName, Value: "trust.cookie.value"})

	rs.login(httptest.NewRecorder(), req)

	assert.Equal(t, "trust.cookie.value", stub.gotTrustedToken,
		"login handler must forward the trust-device cookie to the auth service")
	assert.Equal(t, "j@example.com", stub.gotEmail)
}

// TestLogin_ServiceErrorMapping confirms that AuthError → HTTP-status
// mapping survived the handler refactor (the helper was extracted so the
// MFA path could share it).
func TestLogin_ServiceErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"invalid creds", &authService.AuthError{Op: "login", Err: authService.ErrInvalidCredentials}, http.StatusUnauthorized},
		{"account inactive", &authService.AuthError{Op: "login", Err: authService.ErrAccountInactive}, http.StatusUnauthorized},
		{"tenant not found", &authService.AuthError{Op: "login", Err: authService.ErrTenantNotFound}, http.StatusNotFound},
		// Item #3: an MFA gate that couldn't resolve required/enrolled
		// (settings or credentials lookup hit a non-not-found error) must
		// surface as 503 so the frontend can retry, and so a settings DB
		// outage cannot silently downgrade MFA to "off" for this login.
		{"mfa status unavailable", &authService.AuthError{Op: "check mfa required", Err: authService.ErrMFAStatusUnavailable}, http.StatusServiceUnavailable},
		{"unknown server error", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &Resource{
				AuthService: &loginGateStub{err: tc.err},
			}
			rr := httptest.NewRecorder()
			rs.login(rr, newLoginRequest(t, "x@y.de", "Test1234%"))
			assert.Equal(t, tc.status, rr.Code)
		})
	}
}

// TestLogin_GuardianOnlyReturnsParentPortalCode pins the wire contract the
// tenant login page branches on. The frontend must NOT match on the English
// message text, so the stable code has to be in the body — without it a
// guardian-only account is indistinguishable from any other 403 and the user
// gets a generic "login failed" that reads like a password problem.
//
// Safe to be this specific: LoginWithAudit only reaches
// ErrParentMustUseParentPortal after validateLoginCredentials accepted the
// password, so the caller already owns the account.
func TestLogin_GuardianOnlyReturnsParentPortalCode(t *testing.T) {
	rs := &Resource{
		AuthService: &loginGateStub{
			err: &authService.AuthError{
				Op:  "login",
				Err: authService.ErrParentMustUseParentPortal,
			},
		},
	}

	rr := httptest.NewRecorder()
	rs.login(rr, newLoginRequest(t, "parent@example.com", "Test1234%"))

	assert.Equal(t, http.StatusForbidden, rr.Code)

	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Code   string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "use_parent_portal", body.Code)
}
