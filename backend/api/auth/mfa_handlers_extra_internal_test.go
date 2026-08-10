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

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// stubMFAService is a configurable MFAService for handler-level tests. It
// wraps the shared authtest.MFAServiceMock; construct it via
// newStubMFAService and mutate the Fn fields a test needs to override.
type stubMFAService struct {
	*authtest.MFAServiceMock
}

// newStubMFAService returns a stubMFAService pre-seeded with the "safe
// default" behaviors these handler tests have always relied on when they
// don't override a specific Fn (e.g. AccountBelongsToTenant defaulting to
// true, trusted devices being enabled). Tests mutate the returned Fn
// fields directly before handing the stub to a Resource.
func newStubMFAService() *stubMFAService {
	return &stubMFAService{MFAServiceMock: &authtest.MFAServiceMock{
		AccountBelongsToTenantFn: func(context.Context, int64, int64) (bool, error) {
			return true, nil
		},
		StartChallengeFn: func(context.Context, int64, int64, string, net.IP) (string, error) {
			return "challenge", nil
		},
		VerifyChallengeFn: func(context.Context, string, string) (*authService.VerifiedChallenge, error) {
			return nil, errors.New("not implemented")
		},
		ResendChallengeFn: func(context.Context, string, net.IP) (string, error) {
			return "renewed-token", nil
		},
		IsTrustedDeviceEnabledFn: func(context.Context, int64) bool { return true },
		TrustedDeviceDaysFn:      func(context.Context, int64) int { return 90 },
		GetTenantMFAOverrideFn: func(context.Context, int64, int64) (string, error) {
			return authService.MFAAdminOverrideNone, nil
		},
		GetGlobalMFAOverrideFn: func(context.Context, int64) (string, error) {
			return authService.MFAAdminOverrideNone, nil
		},
	}}
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

// withEnrollmentClaims primes the request context with an
// MFAEnrollmentClaims value the same way MFAEnrollmentAuthenticator would
// after parsing an enrollment JWT. Used by the enroll-handler tests
// because those handlers now read CtxEnrollmentClaims (not CtxClaims) —
// the change introduced by Item #1 of the #1430 review to close the
// pre-enrollment session bypass.
func withEnrollmentClaims(r *http.Request, accountID int64, tenantID int64) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxEnrollmentClaims, jwt.MFAEnrollmentClaims{
		AccountID:            accountID,
		Scope:                jwt.MFAEnrollmentScopeTenant,
		TenantID:             tenantID,
		MFAEnrollmentPending: true,
	})
	return r.WithContext(ctx)
}

// --- tests -------------------------------------------------------------

func TestMFAVerify_InvalidJSONReturns400(t *testing.T) {
	rs := &Resource{MFAService: newStubMFAService()}

	r := httptest.NewRequest(http.MethodPost, "/mfa/verify", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAVerify_BindRejectsMissingFields(t *testing.T) {
	rs := &Resource{MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{}) // empty fields fail Bind
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAVerify_ServiceErrorMapsTo401(t *testing.T) {
	svc := newStubMFAService()
	svc.VerifyChallengeFn = func(context.Context, string, string) (*authService.VerifiedChallenge, error) {
		return nil, authService.ErrMFACodeInvalid
	}
	rs := &Resource{MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{
		ChallengeToken: "tok",
		Code:           "123456",
	})
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAVerify_ServiceErrorMapsTo429ForLockout(t *testing.T) {
	svc := newStubMFAService()
	svc.VerifyChallengeFn = func(context.Context, string, string) (*authService.VerifiedChallenge, error) {
		return nil, authService.ErrMFALocked
	}
	rs := &Resource{MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{
		ChallengeToken: "tok",
		Code:           "123456",
	})
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

// TestMFAResend_Success_ReturnsRenewedToken verifies the post-#1430-round-2
// shape: resend hands the renewed challenge JWT back to the caller so the
// frontend can replace its in-flight token. The previous shape (204 no body)
// produced a dead end where the freshly emailed code couldn't be verified
// once the original JWT expired.
func TestMFAResend_Success_ReturnsRenewedToken(t *testing.T) {
	svc := newStubMFAService()
	svc.ResendChallengeFn = func(context.Context, string, net.IP) (string, error) { return "renewed-tok", nil }
	rs := &Resource{MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp MFAResendResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "renewed-tok", resp.ChallengeToken)
}

func TestMFAResend_BindRejectsEmptyToken(t *testing.T) {
	rs := &Resource{MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{}) // empty token fails Bind
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAResend_ServiceErrorPropagates(t *testing.T) {
	svc := newStubMFAService()
	svc.ResendChallengeFn = func(context.Context, string, net.IP) (string, error) {
		return "", authService.ErrMFARateLimited
	}
	rs := &Resource{MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMFAEnrollStart_RequiresEnrollmentClaim(t *testing.T) {
	rs := &Resource{MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil) // no enrollment claim
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"missing enrollment claim must fail closed — this is the post-#1430 contract; the handler used to read CtxClaims and a regular session token would have worked")
}

func TestMFAEnrollStart_Success_Returns204(t *testing.T) {
	svc := newStubMFAService()
	svc.EnrollFn = func(context.Context, int64) error { return nil }
	svc.StartChallengeFn = func(context.Context, int64, int64, string, net.IP) (string, error) { return "ch", nil }
	rs := &Resource{MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMFAEnrollStart_StartChallengeErrorMapped(t *testing.T) {
	// mfaEnrollStart is a thin wrapper around StartChallenge — actual
	// enrollment happens in mfaEnrollConfirm. A service-level rate-limit
	// must propagate as 429.
	svc := newStubMFAService()
	svc.StartChallengeFn = func(context.Context, int64, int64, string, net.IP) (string, error) {
		return "", authService.ErrMFARateLimited
	}
	rs := &Resource{MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMFAEnrollConfirm_AlreadyEnrolledMintsSession(t *testing.T) {
	// Item #1 of #1430 review: confirm no longer returns 204 on
	// already-enrolled. It mints a full access/refresh pair so the
	// enrollment-scoped token never sticks around past enrollment.
	// ErrMFAAlreadyEnrolled is still swallowed so a retried confirm
	// (where the credential row was already written) advances to the
	// token-pair branch instead of bubbling up.
	svc := newStubMFAService()
	svc.VerifyCodeForAccountFn = func(context.Context, int64, int64, string, string) error { return nil }
	svc.EnrollFn = func(context.Context, int64) error { return authService.ErrMFAAlreadyEnrolled }
	rs := &Resource{
		MFAService:  svc,
		AuthService: &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"},
	}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body TokenResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "access-tok", body.AccessToken)
	assert.Equal(t, "refresh-tok", body.RefreshToken)
}

func TestMFAEnrollConfirm_EnrollErrorPropagates(t *testing.T) {
	svc := newStubMFAService()
	svc.VerifyCodeForAccountFn = func(context.Context, int64, int64, string, string) error { return nil }
	svc.EnrollFn = func(context.Context, int64) error { return errors.New("db down") }
	rs := &Resource{MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestMFAEnrollConfirm_InjectsTenantFromClaims is a regression test for
// #1430 review round 3, finding ①. The enroll routes run under
// MFAEnrollmentAuthenticator, NOT TenantTxMiddleware, so tenant.FromContext
// is 0 when the handler runs. mfaEnrollConfirm must inject the tenant from
// the enrollment claims before calling VerifyCodeForAccount / Enroll, so the
// MFA audit events those emit (mfa_verified / mfa_failed) land in
// audit.auth_events instead of being dropped as "no tenant context".
func TestMFAEnrollConfirm_InjectsTenantFromClaims(t *testing.T) {
	const tenantID int64 = 70010001
	var verifyTenant, enrollTenant int64
	svc := newStubMFAService()
	svc.VerifyCodeForAccountFn = func(ctx context.Context, _, _ int64, _, _ string) error {
		verifyTenant = tenant.FromContext(ctx)
		return nil
	}
	svc.EnrollFn = func(ctx context.Context, _ int64) error {
		enrollTenant = tenant.FromContext(ctx)
		return nil
	}
	rs := &Resource{
		MFAService:  svc,
		AuthService: &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"},
	}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42, tenantID)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, tenantID, verifyTenant,
		"VerifyCodeForAccount must run with the enrollment claim's tenant on the context")
	assert.Equal(t, tenantID, enrollTenant,
		"Enroll must run with the enrollment claim's tenant on the context")
}

func TestMFAEnrollConfirm_RequiresEnrollmentClaim(t *testing.T) {
	// Without an enrollment claim on the context the handler must fail
	// closed — this also covers the case where a regular AppClaims token
	// (CtxClaims) was set instead. Renamed from RequiresAccountClaim to
	// reflect the post-#1430 shape.
	rs := &Resource{MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"})
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// withPlatformEnrollmentClaims primes the request context with a
// platform-scope enrollment claim. Mirrors withEnrollmentClaims but
// flips Scope/TenantID so we can prove the tenant handlers reject
// operator tokens whose AccountID happens to collide with a tenant
// account_id.
func withPlatformEnrollmentClaims(r *http.Request, accountID int64) *http.Request {
	ctx := context.WithValue(r.Context(), jwt.CtxEnrollmentClaims, jwt.MFAEnrollmentClaims{
		AccountID:            accountID,
		Scope:                jwt.MFAEnrollmentScopePlatform,
		TenantID:             0,
		MFAEnrollmentPending: true,
	})
	return r.WithContext(ctx)
}

// TestMFAEnrollStart_RejectsPlatformScopeToken proves the tenant
// enrollment-start endpoint refuses a platform-scope enrollment token
// even when account_id is set. Regression test for the
// cross-scope-enrollment finding in the second #1430 review round.
func TestMFAEnrollStart_RejectsPlatformScopeToken(t *testing.T) {
	svc := newStubMFAService()
	svc.StartChallengeFn = func(context.Context, int64, int64, string, net.IP) (string, error) {
		t.Fatal("StartChallenge must not run on a platform-scope token")
		return "", nil
	}
	rs := &Resource{MFAService: svc}

	r := withPlatformEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"tenant endpoint must reject platform-scope enrollment tokens")
}

// TestMFAEnrollConfirm_RejectsPlatformScopeToken is the confirm-side
// counterpart of TestMFAEnrollStart_RejectsPlatformScopeToken.
func TestMFAEnrollConfirm_RejectsPlatformScopeToken(t *testing.T) {
	svc := newStubMFAService()
	svc.VerifyCodeForAccountFn = func(context.Context, int64, int64, string, string) error {
		t.Fatal("VerifyCodeForAccount must not run on a platform-scope token")
		return nil
	}
	rs := &Resource{MFAService: svc}

	r := withPlatformEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "123456"}), 42)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"tenant endpoint must reject platform-scope enrollment tokens")
}

// TestMFAEnrollStart_RejectsMissingTenantID proves a tenant-scope token
// with TenantID==0 is also refused. (claims.Scope alone is not enough
// — without TenantID the service can't resolve the tenant policy.)
func TestMFAEnrollStart_RejectsMissingTenantID(t *testing.T) {
	svc := newStubMFAService()
	svc.StartChallengeFn = func(context.Context, int64, int64, string, net.IP) (string, error) {
		t.Fatal("StartChallenge must not run without a tenant_id")
		return "", nil
	}
	rs := &Resource{MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil)
	ctx := context.WithValue(r.Context(), jwt.CtxEnrollmentClaims, jwt.MFAEnrollmentClaims{
		AccountID:            42,
		Scope:                jwt.MFAEnrollmentScopeTenant,
		TenantID:             0,
		MFAEnrollmentPending: true,
	})
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r.WithContext(ctx))

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"tenant endpoint must reject tenant-scope tokens without a tenant_id")
}

func TestMFAEnrollConfirm_BindRejectsShortCode(t *testing.T) {
	rs := &Resource{MFAService: newStubMFAService()}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "12"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAEnrollConfirm_WrongCodeReturns401(t *testing.T) {
	svc := newStubMFAService()
	svc.VerifyCodeForAccountFn = func(context.Context, int64, int64, string, string) error { return authService.ErrMFACodeInvalid }
	rs := &Resource{MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAEnrollConfirmRequest{Code: "654321"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// listTrustedDevices returns an empty slice for an account with no rows.
func TestMFAListTrustedDevices_EmptyOK(t *testing.T) {
	svc := newStubMFAService()
	svc.ListTrustedDevicesFn = func(context.Context, int64, int64) ([]*authModels.MFATrustedDevice, error) {
		return []*authModels.MFATrustedDevice{}, nil
	}
	rs := &Resource{MFAService: svc}

	r := withClaims(httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var devices []any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&devices))
	assert.Empty(t, devices, "no rows means an empty list, not null")
}

func TestMFAListTrustedDevices_RequiresClaim(t *testing.T) {
	rs := &Resource{MFAService: newStubMFAService()}

	r := httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAListTrustedDevices_ServiceErrorReturns500(t *testing.T) {
	svc := newStubMFAService()
	svc.ListTrustedDevicesFn = func(context.Context, int64, int64) ([]*authModels.MFATrustedDevice, error) {
		return nil, errors.New("db down")
	}
	rs := &Resource{MFAService: svc}

	r := withClaims(httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// Quiet linter — render is imported via the production code path; this
// reference keeps the import set tidy if static analysis ever re-evaluates.
var _ = render.JSON
