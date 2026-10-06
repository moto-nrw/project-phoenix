package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
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
		{"wrong password", wrap(identityaccess.ErrInvalidCredentials), http.StatusUnauthorized, common.CodeIdentityInvalidCredentials},
		{"unknown address", wrap(identityaccess.ErrAccountNotFound), http.StatusUnauthorized, common.CodeIdentityInvalidCredentials},
		{"no access to this school", wrap(identityaccess.ErrTenantAccessDenied), http.StatusUnauthorized, common.CodeIdentityInvalidCredentials},
		{"account switched off", wrap(identityaccess.ErrAccountInactive), http.StatusUnauthorized, common.CodeIdentitySessionAccountInactive},
		{"too many code requests", wrap(identityaccess.ErrMFARateLimited), http.StatusTooManyRequests, common.CodeIdentityMfaBlocked},
		{"locked after wrong codes", wrap(identityaccess.ErrMFALocked), http.StatusTooManyRequests, common.CodeIdentityMfaBlocked},
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
		{identityaccess.ErrMFAChallengeTokenInvalid, common.CodeIdentityMfaCodeInvalid},
		{identityaccess.ErrMFACodeInvalid, common.CodeIdentityMfaCodeInvalid},
		{identityaccess.ErrMFAUnsupportedScope, common.CodeIdentityMfaCodeInvalid},
		{identityaccess.ErrMFALocked, common.CodeIdentityMfaBlocked},
		{identityaccess.ErrMFARateLimited, common.CodeIdentityMfaBlocked},
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
			assert.Equal(t, common.CodeIdentityPasskeyLoginFailed, decodeWireError(t, rr).Code)
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
		{identityaccess.ErrInvitationNotFound, http.StatusNotFound, common.CodeIdentityInvitationNotFound},
		{identityaccess.ErrInvitationExpired, http.StatusGone, common.CodeIdentityInvitationExpired},
		{identityaccess.ErrInvitationUsed, http.StatusGone, common.CodeIdentityInvitationExpired},
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
	assert.Equal(t, common.CodeIdentityPasswordTooWeak, body.Code)
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
	assert.Equal(t, common.CodeIdentityPasswordResetRateLimited, decodeWireError(t, rr).Code)
}
