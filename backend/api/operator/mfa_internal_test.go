package operator

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

// TestOperatorMFAVerifyRequest_BindRejectsBadInputs covers the request-binding
// validations the handlers depend on so wrong-shape payloads fail before the
// service is touched.
func TestOperatorMFAVerifyRequest_BindRejectsBadInputs(t *testing.T) {
	t.Parallel()

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

func TestOperatorMFAVerifyRequest_BindAcceptsHappyPath(t *testing.T) {
	t.Parallel()

	req := MFAVerifyRequest{ChallengeToken: "  abc.def.ghi  ", Code: "  482157  "}
	require.NoError(t, req.Bind(nil))
	assert.Equal(t, "abc.def.ghi", req.ChallengeToken, "expected trimming")
	assert.Equal(t, "482157", req.Code)
}

func TestOperatorMFAResendRequest_BindRejectsEmpty(t *testing.T) {
	t.Parallel()

	req := MFAResendRequest{}
	assert.Error(t, req.Bind(nil))
}

func TestOperatorMFAEnrollConfirmRequest_BindEnforcesCodeLength(t *testing.T) {
	t.Parallel()

	short := MFAEnrollConfirmRequest{Code: "12345"}
	assert.Error(t, short.Bind(nil), "five-digit code must be rejected")

	long := MFAEnrollConfirmRequest{Code: "1234567"}
	assert.Error(t, long.Bind(nil), "seven-digit code must be rejected")

	ok := MFAEnrollConfirmRequest{Code: "  123456  "}
	require.NoError(t, ok.Bind(nil))
	assert.Equal(t, "123456", ok.Code)
}

// TestOperatorMFAHandlers_ServiceUnavailableWhenNotWired ensures every
// endpoint fails closed (503) when no OperatorMFAService is wired into the
// resource. A deployment that's missing the service must never silently fall
// through to a partial-feature state.
func TestOperatorMFAHandlers_ServiceUnavailableWhenNotWired(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{} // no mfaService, no authService, no tokenAuth

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		body any
	}{
		{"verify", rs.Verify, MFAVerifyRequest{ChallengeToken: "x", Code: "123456"}},
		{"resend", rs.Resend, MFAResendRequest{ChallengeToken: "x"}},
		{"enroll-start", rs.EnrollStart, nil},
		{"enroll-confirm", rs.EnrollConfirm, MFAEnrollConfirmRequest{Code: "123456"}},
		{"trusted-devices-list", rs.ListTrustedDevices, nil},
		{"trusted-devices-revoke", rs.RevokeTrustedDevice, nil},
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

// TestMapOperatorMFAError_StatusCodes locks in the error-to-status mapping so
// future changes to the operator-mfa service-error set surface visibly. The
// errors are aliased to authService.ErrMFA* — those identities are the
// shared contract between tenant and operator paths.
func TestMapOperatorMFAError_StatusCodes(t *testing.T) {
	t.Parallel()

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
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			mapOperatorMFAError(rr, req, tc.err)
			assert.Equal(t, tc.status, rr.Code)
		})
	}
}
