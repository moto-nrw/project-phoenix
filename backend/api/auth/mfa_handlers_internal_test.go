package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// TestMFAVerifyRequest_BindRejectsBadInputs covers the request-binding
// validations the handlers depend on so wrong-shape payloads fail before
// the service is touched.
func TestMFAVerifyRequest_BindRejectsBadInputs(t *testing.T) {
	tests := []struct {
		name string
		req  MFAVerifyRequest
	}{
		{name: "empty challenge token", req: MFAVerifyRequest{Code: "123456"}},
		{name: "empty code", req: MFAVerifyRequest{ChallengeToken: "abc.def.ghi"}},
		{name: "wrong code length", req: MFAVerifyRequest{ChallengeToken: "abc.def.ghi", Code: "123"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Bind(nil)
			assert.Error(t, err)
		})
	}
}

func TestMFAVerifyRequest_BindAcceptsHappyPath(t *testing.T) {
	req := MFAVerifyRequest{ChallengeToken: "  abc.def.ghi  ", Code: "  482157  "}
	require.NoError(t, req.Bind(nil))
	assert.Equal(t, "abc.def.ghi", req.ChallengeToken, "expected trimming")
	assert.Equal(t, "482157", req.Code)
}

func TestMFAEnrollConfirmRequest_BindEnforcesCodeLength(t *testing.T) {
	short := MFAEnrollConfirmRequest{Code: "12345"}
	assert.Error(t, short.Bind(nil), "five-digit code must be rejected")

	long := MFAEnrollConfirmRequest{Code: "1234567"}
	assert.Error(t, long.Bind(nil), "seven-digit code must be rejected")

	ok := MFAEnrollConfirmRequest{Code: "  123456  "}
	require.NoError(t, ok.Bind(nil))
	assert.Equal(t, "123456", ok.Code)
}

// TestMFAHandlers_ServiceUnavailableWhenNotWired ensures every public-facing
// endpoint fails closed (503) when MFAService is nil. A misconfigured
// deployment must never silently fall through to a partial-feature state.
func TestMFAHandlers_ServiceUnavailableWhenNotWired(t *testing.T) {
	rs := &Resource{}

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		body any
	}{
		{"verify", rs.mfaVerify, MFAVerifyRequest{ChallengeToken: "x", Code: "123456"}},
		{"resend", rs.mfaResend, MFAResendRequest{ChallengeToken: "x"}},
		{"enroll-start", rs.mfaEnrollStart, nil},
		{"enroll-confirm", rs.mfaEnrollConfirm, MFAEnrollConfirmRequest{Code: "123456"}},
		{"trusted-devices-list", rs.mfaListTrustedDevices, nil},
		{"trusted-devices-revoke", rs.mfaRevokeTrustedDevice, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			if tc.body != nil {
				var err error
				body, err = json.Marshal(tc.body)
				require.NoError(t, err)
			}
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			tc.fn(rr, req)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "expected 503 when MFAService is nil")
		})
	}
}

// TestMapMFAError_StatusCodes locks in the error-to-status mapping so future
// changes to the service-error set surface visibly.
func TestMapMFAError_StatusCodes(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{authService.ErrMFAChallengeTokenInvalid, http.StatusUnauthorized},
		{authService.ErrMFACodeInvalid, http.StatusUnauthorized},
		{authService.ErrMFALocked, http.StatusTooManyRequests},
		{authService.ErrMFARateLimited, http.StatusTooManyRequests},
		{authService.ErrMFANotEnrolled, http.StatusForbidden},
		{authService.ErrMFAAlreadyEnrolled, http.StatusConflict},
		{authService.ErrMFAPermissionDenied, http.StatusForbidden},
		{authService.ErrMFAInvalidOverride, http.StatusBadRequest},
		{authService.ErrMFAUnsupportedScope, http.StatusUnauthorized},
		// Transient: the service could not read the MFA status or the
		// rate-limit counter and failed closed. A 500 here would tell the
		// frontend the resend endpoint is broken instead of "retry".
		{authService.ErrMFAStatusUnavailable, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			mapMFAError(rr, req, tc.err)
			assert.Equal(t, tc.status, rr.Code)
		})
	}
}
