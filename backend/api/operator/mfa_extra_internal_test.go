package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// stubOperatorMFAServiceExtra implements OperatorMFAService with
// optional per-method overrides. Defaults return zero/nil so unset
// branches don't panic on the way to the assertion.
type stubOperatorMFAServiceExtra struct {
	verifyChallengeFn func(ctx context.Context, challengeToken, code string) (*platformSvc.OperatorVerifiedChallenge, error)
	resendChallengeFn func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	startChallengeFn  func(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	verifyCodeFn      func(ctx context.Context, operatorID int64, code string) error
	enrollFn          func(ctx context.Context, operatorID int64) error
	listDevicesFn     func(ctx context.Context, operatorID int64) ([]*platformModels.OperatorMFATrustedDevice, error)
}

func (s *stubOperatorMFAServiceExtra) HasEnrollment(context.Context, int64) (bool, error) {
	return false, nil
}

func (s *stubOperatorMFAServiceExtra) StartChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	if s.startChallengeFn != nil {
		return s.startChallengeFn(ctx, operatorID, ip)
	}
	return "challenge", nil
}

func (s *stubOperatorMFAServiceExtra) VerifyChallenge(ctx context.Context, challengeToken, code string) (*platformSvc.OperatorVerifiedChallenge, error) {
	if s.verifyChallengeFn != nil {
		return s.verifyChallengeFn(ctx, challengeToken, code)
	}
	return nil, errors.New("not implemented")
}

func (s *stubOperatorMFAServiceExtra) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if s.resendChallengeFn != nil {
		return s.resendChallengeFn(ctx, challengeToken, ip)
	}
	return "renewed-token", nil
}

func (s *stubOperatorMFAServiceExtra) VerifyCodeForOperator(ctx context.Context, operatorID int64, code string) error {
	if s.verifyCodeFn != nil {
		return s.verifyCodeFn(ctx, operatorID, code)
	}
	return nil
}

func (s *stubOperatorMFAServiceExtra) Enroll(ctx context.Context, operatorID int64) error {
	if s.enrollFn != nil {
		return s.enrollFn(ctx, operatorID)
	}
	return nil
}

func (s *stubOperatorMFAServiceExtra) Disable(context.Context, int64) error { return nil }

func (s *stubOperatorMFAServiceExtra) IssueTrustedDevice(context.Context, int64, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (s *stubOperatorMFAServiceExtra) VerifyTrustedDevice(context.Context, int64, string) (bool, error) {
	return false, nil
}

func (s *stubOperatorMFAServiceExtra) ListTrustedDevices(ctx context.Context, operatorID int64) ([]*platformModels.OperatorMFATrustedDevice, error) {
	if s.listDevicesFn != nil {
		return s.listDevicesFn(ctx, operatorID)
	}
	return nil, nil
}

func (s *stubOperatorMFAServiceExtra) RevokeTrustedDevice(context.Context, int64, int64) error {
	return nil
}

var _ platformSvc.OperatorMFAService = (*stubOperatorMFAServiceExtra)(nil)

// --- helpers -----------------------------------------------------------

func opJSONReq(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var rdr *bytes.Reader
	if body == nil {
		rdr = bytes.NewReader([]byte("{}"))
	} else {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rdr)
	r.Header.Set("Content-Type", "application/json")
	return r
}

func opWithClaims(r *http.Request, operatorID int) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxClaims, jwt.AppClaims{ID: operatorID})
	return r.WithContext(ctx)
}

// opWithEnrollmentClaims primes the request context the same way
// MFAEnrollmentAuthenticator would after parsing an enrollment JWT. Used
// by the enroll-handler tests because those handlers now require an
// enrollment-scoped claim (Item #1 of the #1430 review).
func opWithEnrollmentClaims(r *http.Request, operatorID int64) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxEnrollmentClaims, jwt.MFAEnrollmentClaims{
		AccountID:            operatorID,
		Scope:                jwt.MFAEnrollmentScopePlatform,
		MFAEnrollmentPending: true,
	})
	return r.WithContext(ctx)
}

// --- tests -------------------------------------------------------------

func TestOperatorMFAVerify_InvalidJSON_Returns400(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{}}

	r := httptest.NewRequest(http.MethodPost, "/operator/mfa/verify", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Verify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperatorMFAVerify_ServiceErrorMapsTo401(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		verifyChallengeFn: func(context.Context, string, string) (*platformSvc.OperatorVerifiedChallenge, error) {
			return nil, authService.ErrMFACodeInvalid
		},
	}}

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/verify",
		MFAVerifyRequest{ChallengeToken: "tok", Code: "123456"})
	rr := httptest.NewRecorder()
	rs.Verify(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestOperatorMFAResend_Success_ReturnsRenewedToken mirrors the tenant-side
// shape: operator resend now hands back the renewed challenge JWT so the
// operator login flow can replace its in-flight token.
func TestOperatorMFAResend_Success_ReturnsRenewedToken(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		resendChallengeFn: func(context.Context, string, net.IP) (string, error) { return "renewed-tok", nil },
	}}

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/resend",
		MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.Resend(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp MFAResendResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "renewed-tok", resp.ChallengeToken)
}

func TestOperatorMFAResend_BindRejectsEmptyToken(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{}}

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/resend", MFAResendRequest{})
	rr := httptest.NewRecorder()
	rs.Resend(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperatorMFAResend_ServiceErrorMapsTo429(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		resendChallengeFn: func(context.Context, string, net.IP) (string, error) {
			return "", authService.ErrMFARateLimited
		},
	}}

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/resend",
		MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.Resend(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestOperatorMFAEnrollStart_RequiresEnrollmentClaim(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{}}

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/start", nil)
	rr := httptest.NewRecorder()
	rs.EnrollStart(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAEnrollStart_Success(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		startChallengeFn: func(context.Context, int64, net.IP) (string, error) { return "ch", nil },
	}}

	r := opWithEnrollmentClaims(opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/start", nil), 42)
	rr := httptest.NewRecorder()
	rs.EnrollStart(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestOperatorMFAEnrollConfirm_RequiresEnrollmentClaim(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{}}

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"})
	rr := httptest.NewRecorder()
	rs.EnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAEnrollConfirm_WrongCodeReturns401(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		verifyCodeFn: func(context.Context, int64, string) error { return authService.ErrMFACodeInvalid },
	}}

	r := opWithEnrollmentClaims(opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "654321"}), 42)
	rr := httptest.NewRecorder()
	rs.EnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAEnrollConfirm_AlreadyEnrolledMintsSession(t *testing.T) {
	t.Parallel()

	// Item #1 of #1430 review (operator side): confirm now mints a full
	// access/refresh pair instead of returning 204, so the enrollment
	// token doesn't outlive enrollment. ErrMFAAlreadyEnrolled is still
	// swallowed so retried confirms succeed.
	rs := &MFAResource{
		mfaService: &stubOperatorMFAServiceExtra{
			verifyCodeFn: func(context.Context, int64, string) error { return nil },
			enrollFn:     func(context.Context, int64) error { return authService.ErrMFAAlreadyEnrolled },
		},
		authService: &completeOperatorMFAAuthStub{access: "op-access", refresh: "op-refresh"},
	}

	r := opWithEnrollmentClaims(opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42)
	rr := httptest.NewRecorder()
	rs.EnrollConfirm(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data    MFATokenResponse `json:"data"`
		Message string           `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "op-access", body.Data.AccessToken)
	assert.Equal(t, "op-refresh", body.Data.RefreshToken)
}

func TestOperatorMFAListTrustedDevices_EmptyOK(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		listDevicesFn: func(context.Context, int64) ([]*platformModels.OperatorMFATrustedDevice, error) {
			return []*platformModels.OperatorMFATrustedDevice{}, nil
		},
	}}

	r := opWithClaims(httptest.NewRequest(http.MethodGet, "/operator/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.ListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOperatorMFAListTrustedDevices_RequiresClaim(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{}}

	r := httptest.NewRequest(http.MethodGet, "/operator/mfa/trusted-devices", nil)
	rr := httptest.NewRecorder()
	rs.ListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAListTrustedDevices_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	rs := &MFAResource{mfaService: &stubOperatorMFAServiceExtra{
		listDevicesFn: func(context.Context, int64) ([]*platformModels.OperatorMFATrustedDevice, error) {
			return nil, errors.New("db down")
		},
	}}

	r := opWithClaims(httptest.NewRequest(http.MethodGet, "/operator/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.ListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
