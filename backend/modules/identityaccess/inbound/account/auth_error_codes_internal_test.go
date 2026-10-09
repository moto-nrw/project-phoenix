package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The login, code and reset forms show a text per code (#2517). These tests
// pin the code next to the unchanged status of each refusal.

type wireError struct {
	Code   string `json:"code"`
	Errors []struct {
		Field string `json:"field"`
	} `json:"errors"`
}

func decodeWireError(t *testing.T, rr *httptest.ResponseRecorder) wireError {
	t.Helper()
	var body wireError
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func TestLoginRefusalsCarryCodes(t *testing.T) {
	t.Parallel()

	wrap := func(err error) error { return &identityaccess.AuthenticationError{Op: "login", Err: err} }
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"wrong password", wrap(identityaccess.ErrInvalidCredentials), http.StatusUnauthorized, "identity.invalid_credentials"},
		{"unknown address", wrap(identityaccess.ErrAccountNotFound), http.StatusUnauthorized, "identity.invalid_credentials"},
		{"no access to this school", wrap(identityaccess.ErrTenantAccessDenied), http.StatusUnauthorized, "identity.invalid_credentials"},
		{"account switched off", wrap(identityaccess.ErrAccountInactive), http.StatusUnauthorized, "identity.session_account_inactive"},
		{"too many code requests", wrap(identityaccess.ErrMFARateLimited), http.StatusTooManyRequests, "identity.mfa_blocked"},
		{"locked after wrong codes", wrap(identityaccess.ErrMFALocked), http.StatusTooManyRequests, "identity.mfa_blocked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			(&Resource{}).handleLoginError(rr, httptest.NewRequest(http.MethodPost, "/auth/login", nil), tc.err)
			assert.Equal(t, tc.status, rr.Code)
			assert.Equal(t, tc.code, decodeWireError(t, rr).Code)
		})
	}
}

func TestMFARefusalsCarryCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  error
		code string
	}{
		{identityaccess.ErrMFAChallengeTokenInvalid, "identity.mfa_code_invalid"},
		{identityaccess.ErrMFACodeInvalid, "identity.mfa_code_invalid"},
		{identityaccess.ErrMFAUnsupportedScope, "identity.mfa_code_invalid"},
		{identityaccess.ErrMFALocked, "identity.mfa_blocked"},
		{identityaccess.ErrMFARateLimited, "identity.mfa_blocked"},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			mapMFAError(rr, httptest.NewRequest(http.MethodPost, "/", nil), tc.err)
			assert.Equal(t, tc.code, decodeWireError(t, rr).Code)
		})
	}
}

func TestPasskeyLoginRefusalsShareOneCode(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		identityaccess.ErrInvalidCredentials,
		identityaccess.ErrPasskeySessionInvalid,
		identityaccess.ErrAccountNotFound,
		identityaccess.ErrTenantAccessDenied,
	} {
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			mapPasskeyLoginError(rr, httptest.NewRequest(http.MethodPost, "/", nil), err)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Equal(t, "identity.passkey_login_failed", decodeWireError(t, rr).Code)
		})
	}
}

func TestInvitationRefusalsCarryCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err    error
		status int
		code   string
	}{
		{identityaccess.ErrInvitationNotFound, http.StatusNotFound, "identity.invitation_not_found"},
		{identityaccess.ErrInvitationExpired, http.StatusGone, "identity.invitation_expired"},
		{identityaccess.ErrInvitationUsed, http.StatusGone, "identity.invitation_expired"},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			require.True(t, renderInvitationError(rr, httptest.NewRequest(http.MethodGet, "/", nil), tc.err))
			assert.Equal(t, tc.status, rr.Code)
			assert.Equal(t, tc.code, decodeWireError(t, rr).Code)
		})
	}
}

func TestInvitationWeakPasswordMarksThePasswordField(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	require.True(t, renderAcceptError(rr, httptest.NewRequest(http.MethodPost, "/", nil), identityaccess.ErrPasswordTooWeak))
	body := decodeWireError(t, rr)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, "identity.password_too_weak", body.Code)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "password", body.Errors[0].Field)
}

func TestPasswordResetRateLimitCarriesCode(t *testing.T) {
	t.Parallel()

	rr := httptest.NewRecorder()
	limited := &identityaccess.PasswordResetRateLimitError{Attempts: 3, RetryAt: time.Now().Add(time.Hour)}
	require.True(t, renderPasswordResetRateLimit(rr, httptest.NewRequest(http.MethodPost, "/", nil), limited))
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
	assert.Equal(t, "identity.password_reset_rate_limited", decodeWireError(t, rr).Code)
}

func TestRegistrationRefusalsCarryCodes(t *testing.T) {
	t.Parallel()

	wrap := func(err error) error { return &identityaccess.AuthenticationError{Op: "register", Err: err} }
	cases := []struct {
		name  string
		err   error
		code  string
		field string
	}{
		// The staff form offers to link the existing account on this code.
		{"address already registered", wrap(identityaccess.ErrEmailAlreadyExists), "identity.email_already_exists", "email"},
		{"password too weak", wrap(identityaccess.ErrPasswordTooWeak), "identity.password_too_weak", "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			(&Resource{}).handleRegistrationError(rr, httptest.NewRequest(http.MethodPost, "/auth/register", nil), tc.err)
			body := decodeWireError(t, rr)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Equal(t, tc.code, body.Code)
			require.Len(t, body.Errors, 1)
			assert.Equal(t, tc.field, body.Errors[0].Field)
		})
	}
}
