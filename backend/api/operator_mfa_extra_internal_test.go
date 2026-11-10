package api

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

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
)

// stubOperatorMFAServiceExtra implements OperatorMFAService with
// optional per-method overrides. Defaults return zero/nil so unset
// branches don't panic on the way to the assertion.
type stubOperatorMFAServiceExtra struct {
	verifyChallengeFn func(ctx context.Context, challengeToken, code string) (int64, error)
	resendChallengeFn func(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	startChallengeFn  func(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	verifyCodeFn      func(ctx context.Context, operatorID int64, code string) error
	enrollFn          func(ctx context.Context, proof identityaccess.OperatorMFAProof) error
	listDevicesFn     func(ctx context.Context, operatorID int64) ([]identityaccess.OperatorTrustedDevice, error)
}

func (s *stubOperatorMFAServiceExtra) HasOperatorMFAEnrollment(context.Context, int64) (bool, error) {
	return false, nil
}

func (s *stubOperatorMFAServiceExtra) StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP, _ *int64) (string, error) {
	if s.startChallengeFn != nil {
		return s.startChallengeFn(ctx, operatorID, ip)
	}
	return "challenge", nil
}

func (s *stubOperatorMFAServiceExtra) VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (identityaccess.OperatorMFAProof, error) {
	if s.verifyChallengeFn != nil {
		operatorID, err := s.verifyChallengeFn(ctx, challengeToken, code)
		return identityaccess.OperatorMFAProof{OperatorID: operatorID}, err
	}
	return identityaccess.OperatorMFAProof{}, errors.New("not implemented")
}

func (s *stubOperatorMFAServiceExtra) ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	if s.resendChallengeFn != nil {
		return s.resendChallengeFn(ctx, challengeToken, ip)
	}
	return "renewed-token", nil
}

func (s *stubOperatorMFAServiceExtra) VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string, _ *int64) error {
	if s.verifyCodeFn != nil {
		return s.verifyCodeFn(ctx, operatorID, code)
	}
	return nil
}

func (s *stubOperatorMFAServiceExtra) EnrollOperatorMFA(ctx context.Context, proof identityaccess.OperatorMFAProof) error {
	if s.enrollFn != nil {
		return s.enrollFn(ctx, proof)
	}
	return nil
}

func (s *stubOperatorMFAServiceExtra) DisableOperatorMFA(context.Context, int64) error { return nil }

func (s *stubOperatorMFAServiceExtra) IssueOperatorTrustedDevice(context.Context, identityaccess.OperatorMFAProof, string, net.IP) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (s *stubOperatorMFAServiceExtra) VerifyOperatorTrustedDevice(context.Context, int64, string) (bool, error) {
	return false, nil
}

func (s *stubOperatorMFAServiceExtra) ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]identityaccess.OperatorTrustedDevice, error) {
	if s.listDevicesFn != nil {
		return s.listDevicesFn(ctx, operatorID)
	}
	return nil, nil
}

func (s *stubOperatorMFAServiceExtra) RevokeOperatorTrustedDeviceOwned(context.Context, int64, int64) error {
	return nil
}

func (s *stubOperatorMFAServiceExtra) OperatorTrustedDeviceDays() int { return 90 }

var _ identityaccess.OperatorMFAFlows = (*stubOperatorMFAServiceExtra)(nil)

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
	ctx := routetest.WithAuthenticatedContext(r.Context(), routetest.Claims{ID: operatorID}, nil)
	return r.WithContext(ctx)
}

// opWithEnrollmentClaims primes the request context the same way
// MFAEnrollmentAuthenticator would after parsing an enrollment JWT. Used
// by the enroll-handler tests because those handlers now require an
// enrollment-scoped claim (Item #1 of the #1430 review).
func opWithEnrollmentClaims(r *http.Request, operatorID int64) *http.Request {
	ctx := routetest.WithEnrollmentClaims(r.Context(), routetest.MFAEnrollmentClaims{
		AccountID:            operatorID,
		Scope:                tokenprimitives.PortalPlatform,
		MFAEnrollmentPending: true,
	})
	return r.WithContext(ctx)
}

// --- tests -------------------------------------------------------------

func TestOperatorMFAVerify_InvalidJSON_Returns400(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{})

	r := httptest.NewRequest(http.MethodPost, "/operator/mfa/verify", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.Verify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperatorMFAVerify_ServiceErrorMapsTo401(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		verifyChallengeFn: func(context.Context, string, string) (int64, error) {
			return 0, identityaccess.ErrMFACodeInvalid
		},
	})

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/verify",
		identityoperator.MFAVerifyRequest{ChallengeToken: "tok", Code: "123456"})
	rr := httptest.NewRecorder()
	rs.Verify(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// TestOperatorMFAResend_Success_ReturnsRenewedToken mirrors the tenant-side
// shape: operator resend now hands back the renewed challenge JWT so the
// operator login flow can replace its in-flight token.
func TestOperatorMFAResend_Success_ReturnsRenewedToken(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		resendChallengeFn: func(context.Context, string, net.IP) (string, error) { return "renewed-tok", nil },
	})

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/resend",
		identityoperator.MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.Resend(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp identityoperator.MFAResendResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "renewed-tok", resp.ChallengeToken)
}

func TestOperatorMFAResend_BindRejectsEmptyToken(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{})

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/resend", identityoperator.MFAResendRequest{})
	rr := httptest.NewRecorder()
	rs.Resend(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperatorMFAResend_ServiceErrorMapsTo429(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		resendChallengeFn: func(context.Context, string, net.IP) (string, error) {
			return "", identityaccess.ErrMFARateLimited
		},
	})

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/resend",
		identityoperator.MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.Resend(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestOperatorMFAEnrollStart_RequiresEnrollmentClaim(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{})

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/start", nil)
	rr := httptest.NewRecorder()
	rs.EnrollStart(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAEnrollStart_Success(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		startChallengeFn: func(context.Context, int64, net.IP) (string, error) { return "ch", nil },
	})

	r := opWithEnrollmentClaims(opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/start", nil), 42)
	rr := httptest.NewRecorder()
	rs.EnrollStart(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestOperatorMFAEnrollConfirm_RequiresEnrollmentClaim(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{})

	r := opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		identityoperator.MFAEnrollConfirmRequest{Code: "123456"})
	rr := httptest.NewRecorder()
	rs.EnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAEnrollConfirm_WrongCodeReturns401(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		verifyCodeFn: func(context.Context, int64, string) error { return identityaccess.ErrMFACodeInvalid },
	})

	r := opWithEnrollmentClaims(opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		identityoperator.MFAEnrollConfirmRequest{Code: "654321"}), 42)
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
	rs := identityoperator.NewMFAResource(
		&completeOperatorMFAAuthStub{access: "op-access", refresh: "op-refresh"},
		&stubOperatorMFAServiceExtra{
			verifyCodeFn: func(context.Context, int64, string) error { return nil },
			enrollFn: func(context.Context, identityaccess.OperatorMFAProof) error {
				return identityaccess.ErrMFAAlreadyEnrolled
			},
		},
	)

	r := opWithEnrollmentClaims(opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		identityoperator.MFAEnrollConfirmRequest{Code: "123456"}), 42)
	rr := httptest.NewRecorder()
	rs.EnrollConfirm(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body struct {
		Data    portalauth.TokenResponse `json:"data"`
		Message string                   `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "op-access", body.Data.AccessToken)
	assert.Equal(t, "op-refresh", body.Data.RefreshToken)
}

// TestOperatorMFAEnrollConfirm_StaleCredentialEnrollsAndMintsNothing: the
// confirm hands the enrollment the generation its enrollment token names
// (#6459). When a password change committed since, the enrollment refuses,
// the confirm answers 401 identity.invalid_credentials and mints nothing.
func TestOperatorMFAEnrollConfirm_StaleCredentialEnrollsAndMintsNothing(t *testing.T) {
	t.Parallel()

	var enrolled identityaccess.OperatorMFAProof
	auth := &completeOperatorMFAAuthStub{access: "op-access", refresh: "op-refresh"}
	rs := identityoperator.NewMFAResource(auth, &stubOperatorMFAServiceExtra{
		verifyCodeFn: func(context.Context, int64, string) error { return nil },
		enrollFn: func(_ context.Context, proof identityaccess.OperatorMFAProof) error {
			enrolled = proof
			return identityaccess.ErrOperatorInvalidCredentials
		},
	})

	ctx := routetest.WithEnrollmentClaims(context.Background(), routetest.MFAEnrollmentClaims{
		AccountID:            42,
		Scope:                tokenprimitives.PortalPlatform,
		CredentialGeneration: 7,
		MFAEnrollmentPending: true,
	})
	r := opJSONReq(t, http.MethodPost, "/operator/mfa/enroll/confirm",
		identityoperator.MFAEnrollConfirmRequest{Code: "123456"}).WithContext(ctx)
	rr := httptest.NewRecorder()
	rs.EnrollConfirm(rr, r)

	assert.Equal(t, identityaccess.OperatorMFAProof{OperatorID: 42, CredentialGeneration: 7}, enrolled,
		"the enrollment is held to the generation of the enrollment token")
	routetest.AssertErrorResponse(t, rr, http.StatusUnauthorized)
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
	assert.Equal(t, "identity.invalid_credentials", problem.Code)
	assert.Zero(t, auth.gotOperatorID, "no session is minted")
}

func TestOperatorMFAListTrustedDevices_EmptyOK(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		listDevicesFn: func(context.Context, int64) ([]identityaccess.OperatorTrustedDevice, error) {
			return []identityaccess.OperatorTrustedDevice{}, nil
		},
	})

	r := opWithClaims(httptest.NewRequest(http.MethodGet, "/operator/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.ListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOperatorMFAListTrustedDevices_RequiresClaim(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{})

	r := httptest.NewRequest(http.MethodGet, "/operator/mfa/trusted-devices", nil)
	rr := httptest.NewRecorder()
	rs.ListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAListTrustedDevices_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	rs := identityoperator.NewMFAResource(nil, &stubOperatorMFAServiceExtra{
		listDevicesFn: func(context.Context, int64) ([]identityaccess.OperatorTrustedDevice, error) {
			return nil, errors.New("db down")
		},
	})

	r := opWithClaims(httptest.NewRequest(http.MethodGet, "/operator/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.ListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
