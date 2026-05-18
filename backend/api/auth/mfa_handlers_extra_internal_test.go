package auth

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

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// stubMFAService is a configurable MFAService for handler-level tests.
// Methods default to "no error / zero value"; tests set the functions
// they need. The MFAService interface is large — we embed nothing and
// implement every method so calling an unset path returns the zero
// value (rather than panicking with a nil-interface dereference,
// which would mask test misconfiguration).
type stubMFAService struct {
	verifyChallengeFn     func(ctx context.Context, challengeToken, code string) (*authService.VerifiedChallenge, error)
	resendChallengeFn     func(ctx context.Context, challengeToken string, ip net.IP) error
	enrollFn              func(ctx context.Context, accountID int64) error
	verifyCodeFn          func(ctx context.Context, accountID int64, code string) error
	startChallengeFn      func(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	listTrustedDevicesFn  func(ctx context.Context, accountID int64) ([]*authModels.MFATrustedDevice, error)
	revokeTrustedDeviceFn func(ctx context.Context, accountID, deviceID int64) error
}

func (s *stubMFAService) IsRequired(context.Context, *authModels.Account, int64) (bool, error) {
	return false, nil
}
func (s *stubMFAService) HasEnrollment(context.Context, int64) (bool, error) {
	return false, nil
}
func (s *stubMFAService) StartChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	if s.startChallengeFn != nil {
		return s.startChallengeFn(ctx, accountID, tenantID, scope, ip)
	}
	return "challenge", nil
}
func (s *stubMFAService) VerifyChallenge(ctx context.Context, challengeToken, code string) (*authService.VerifiedChallenge, error) {
	if s.verifyChallengeFn != nil {
		return s.verifyChallengeFn(ctx, challengeToken, code)
	}
	return nil, errors.New("not implemented")
}
func (s *stubMFAService) ResendChallenge(ctx context.Context, challengeToken string, ip net.IP) error {
	if s.resendChallengeFn != nil {
		return s.resendChallengeFn(ctx, challengeToken, ip)
	}
	return nil
}
func (s *stubMFAService) VerifyCodeForAccount(ctx context.Context, accountID int64, code string) error {
	if s.verifyCodeFn != nil {
		return s.verifyCodeFn(ctx, accountID, code)
	}
	return nil
}
func (s *stubMFAService) Enroll(ctx context.Context, accountID int64) error {
	if s.enrollFn != nil {
		return s.enrollFn(ctx, accountID)
	}
	return nil
}
func (s *stubMFAService) Disable(context.Context, int64) error { return nil }
func (s *stubMFAService) IssueTrustedDevice(context.Context, int64, int64, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (s *stubMFAService) VerifyTrustedDevice(context.Context, int64, int64, string) (bool, error) {
	return false, nil
}
func (s *stubMFAService) ListTrustedDevices(ctx context.Context, accountID int64) ([]*authModels.MFATrustedDevice, error) {
	if s.listTrustedDevicesFn != nil {
		return s.listTrustedDevicesFn(ctx, accountID)
	}
	return nil, nil
}
func (s *stubMFAService) RevokeTrustedDevice(ctx context.Context, accountID, deviceID int64) error {
	if s.revokeTrustedDeviceFn != nil {
		return s.revokeTrustedDeviceFn(ctx, accountID, deviceID)
	}
	return nil
}
func (s *stubMFAService) IsTrustedDeviceEnabled(context.Context, int64) bool { return true }
func (s *stubMFAService) TrustedDeviceDays(context.Context, int64) int       { return 90 }
func (s *stubMFAService) GetMFAOverride(context.Context, int64) (string, error) {
	return "", nil
}
func (s *stubMFAService) SetMFAOverride(context.Context, int64, int64, string, string, []string) error {
	return nil
}
func (s *stubMFAService) OperatorSetMFAOverride(context.Context, int64, int64, int64, string, string) error {
	return nil
}
func (s *stubMFAService) AdminDisable(context.Context, int64, int64, string, []string) error {
	return nil
}
func (s *stubMFAService) OperatorAdminDisable(context.Context, int64, int64, int64, string) error {
	return nil
}
func (s *stubMFAService) MaskEmailForChallenge(string) string { return "" }
func (s *stubMFAService) CleanupExpiredChallenges(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

// Sanity: stubMFAService must satisfy authService.MFAService at compile
// time, otherwise the embedded handler can't take it.
var _ authService.MFAService = (*stubMFAService)(nil)

// --- helpers -----------------------------------------------------------

func jsonReq(t *testing.T, method, path string, body any) *http.Request {
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

func withClaims(r *http.Request, accountID int) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxClaims, jwt.AppClaims{ID: accountID})
	return r.WithContext(ctx)
}

// --- tests -------------------------------------------------------------

func TestMFAVerify_InvalidJSONReturns400(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := httptest.NewRequest(http.MethodPost, "/mfa/verify", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAVerify_BindRejectsMissingFields(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{}) // empty fields fail Bind
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAVerify_ServiceErrorMapsTo401(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		verifyChallengeFn: func(context.Context, string, string) (*authService.VerifiedChallenge, error) {
			return nil, authService.ErrMFACodeInvalid
		},
	}}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{
		ChallengeToken: "tok",
		Code:           "123456",
	})
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAVerify_ServiceErrorMapsTo429ForLockout(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		verifyChallengeFn: func(context.Context, string, string) (*authService.VerifiedChallenge, error) {
			return nil, authService.ErrMFALocked
		},
	}}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{
		ChallengeToken: "tok",
		Code:           "123456",
	})
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMFAResend_Success_Returns204(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		resendChallengeFn: func(context.Context, string, net.IP) error { return nil },
	}}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMFAResend_BindRejectsEmptyToken(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{}) // empty token fails Bind
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAResend_ServiceErrorPropagates(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		resendChallengeFn: func(context.Context, string, net.IP) error {
			return authService.ErrMFARateLimited
		},
	}}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMFAEnrollStart_RequiresAccountClaim(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil) // no claims in context
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"missing account claim must fail closed, not silently enroll account 0")
}

func TestMFAEnrollStart_Success_Returns204(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		enrollFn:         func(context.Context, int64) error { return nil },
		startChallengeFn: func(context.Context, int64, int64, string, net.IP) (string, error) { return "ch", nil },
	}}

	r := withClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMFAEnrollStart_StartChallengeErrorMapped(t *testing.T) {
	// mfaEnrollStart is a thin wrapper around StartChallenge — actual
	// enrollment happens in mfaEnrollConfirm. A service-level rate-limit
	// must propagate as 429.
	rs := &Resource{MFAService: &stubMFAService{
		startChallengeFn: func(context.Context, int64, int64, string, net.IP) (string, error) {
			return "", authService.ErrMFARateLimited
		},
	}}

	r := withClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMFAEnrollConfirm_AlreadyEnrolledTreatedAsSuccess(t *testing.T) {
	// The handler swallows ErrMFAAlreadyEnrolled so a retried confirm
	// (where the credential row was already written) still completes
	// cleanly with 204.
	rs := &Resource{MFAService: &stubMFAService{
		verifyCodeFn: func(context.Context, int64, string) error { return nil },
		enrollFn:     func(context.Context, int64) error { return authService.ErrMFAAlreadyEnrolled },
	}}

	r := withClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMFAEnrollConfirm_EnrollErrorPropagates(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		verifyCodeFn: func(context.Context, int64, string) error { return nil },
		enrollFn:     func(context.Context, int64) error { return errors.New("db down") },
	}}

	r := withClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestMFAEnrollConfirm_RequiresAccountClaim(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"})
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAEnrollConfirm_BindRejectsShortCode(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := withClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "12"}), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAEnrollConfirm_WrongCodeReturns401(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		verifyCodeFn: func(context.Context, int64, string) error { return authService.ErrMFACodeInvalid },
	}}

	r := withClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "654321"}), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// listTrustedDevices returns an empty slice for an account with no rows.
func TestMFAListTrustedDevices_EmptyOK(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		listTrustedDevicesFn: func(context.Context, int64) ([]*authModels.MFATrustedDevice, error) {
			return []*authModels.MFATrustedDevice{}, nil
		},
	}}

	r := withClaims(httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var devices []any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&devices))
	assert.Empty(t, devices, "no rows means an empty list, not null")
}

func TestMFAListTrustedDevices_RequiresClaim(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{}}

	r := httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAListTrustedDevices_ServiceErrorReturns500(t *testing.T) {
	rs := &Resource{MFAService: &stubMFAService{
		listTrustedDevicesFn: func(context.Context, int64) ([]*authModels.MFATrustedDevice, error) {
			return nil, errors.New("db down")
		},
	}}

	r := withClaims(httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// Quiet linter — render is imported via the production code path; this
// reference keeps the import set tidy if static analysis ever re-evaluates.
var _ = render.JSON
