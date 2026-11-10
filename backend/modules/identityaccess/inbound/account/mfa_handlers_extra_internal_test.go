package account

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

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/portalauth"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// stubMFAService is a configurable MFAService for handler-level tests. It
// wraps the shared MFAServiceMock; construct it via
// newStubMFAService and mutate the Fn fields a test needs to override.
type stubMFAService struct {
	*MFAServiceMock
}

// newStubMFAService returns a stubMFAService pre-seeded with the "safe
// default" behaviors these handler tests have always relied on when they
// don't override a specific Fn (e.g. AccountBelongsToTenant defaulting to
// true, trusted devices being enabled). Tests mutate the returned Fn
// fields directly before handing the stub to a Resource.
func newStubMFAService() *stubMFAService {
	return &stubMFAService{MFAServiceMock: &MFAServiceMock{
		AccountBelongsToSchoolFn: func(context.Context, int64, int64) (bool, error) {
			return true, nil
		},
		StartMFAChallengeFn: func(context.Context, int64, int64, string, net.IP) (string, error) {
			return "challenge", nil
		},
		VerifyMFAChallengeFn: func(context.Context, string, string) (identityaccess.VerifiedMFAChallenge, error) {
			return identityaccess.VerifiedMFAChallenge{}, errors.New("not implemented")
		},
		ResendMFAChallengeFn: func(context.Context, string, net.IP) (string, error) {
			return "renewed-token", nil
		},
		IsTrustedDeviceEnabledFn: func(context.Context, int64) bool { return true },
		TrustedDeviceDaysFn:      func(context.Context, int64) int { return 90 },
		GetTenantMFAOverrideFn: func(context.Context, int64, int64) (string, error) {
			return identityaccess.MFAAdminOverrideNone, nil
		},
		GetGlobalMFAOverrideFn: func(context.Context, int64) (string, error) {
			return identityaccess.MFAAdminOverrideNone, nil
		},
	}}
}

// Sanity: stubMFAService must satisfy identityaccess.AccountMFA at compile
// time, otherwise the embedded handler can't take it.
var _ identityaccess.AccountMFA = (*stubMFAService)(nil)

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
	ctx := testutil.WithAuthenticatedContext(r.Context(), testutil.Claims{ID: accountID}, nil)
	return r.WithContext(ctx)
}

// withEnrollmentClaims primes the request context with an
// MFAEnrollmentClaims value the same way MFAEnrollmentAuthenticator would
// after parsing an enrollment JWT. Used by the enroll-handler tests
// because those handlers now read CtxEnrollmentClaims (not CtxClaims) —
// the change introduced by Item #1 of the #1430 review to close the
// pre-enrollment session bypass.
func withEnrollmentClaims(r *http.Request, accountID int64, tenantID int64) *http.Request {
	ctx := testutil.WithEnrollmentClaims(r.Context(), testutil.MFAEnrollmentClaims{
		AccountID:            accountID,
		Scope:                tokenprimitives.PortalTenant,
		TenantID:             tenantID,
		MFAEnrollmentPending: true,
	})
	return r.WithContext(ctx)
}

// --- tests -------------------------------------------------------------

func TestMFAVerify_InvalidJSONReturns400(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := httptest.NewRequest(http.MethodPost, "/mfa/verify", strings.NewReader("not-json"))
	r.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAVerify_BindRejectsMissingFields(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{}) // empty fields fail Bind
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAVerify_ServiceErrorMapsTo401(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.VerifyMFAChallengeFn = func(context.Context, string, string) (identityaccess.VerifiedMFAChallenge, error) {
		return identityaccess.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/verify", MFAVerifyRequest{
		ChallengeToken: "tok",
		Code:           "123456",
	})
	rr := httptest.NewRecorder()
	rs.mfaVerify(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAVerify_ServiceErrorMapsTo429ForLockout(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.VerifyMFAChallengeFn = func(context.Context, string, string) (identityaccess.VerifiedMFAChallenge, error) {
		return identityaccess.VerifiedMFAChallenge{}, identityaccess.ErrMFALocked
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

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
	t.Parallel()

	svc := newStubMFAService()
	svc.ResendMFAChallengeFn = func(context.Context, string, net.IP) (string, error) { return "renewed-tok", nil }
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp MFAResendResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "renewed-tok", resp.ChallengeToken)
}

func TestMFAResend_BindRejectsEmptyToken(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{}) // empty token fails Bind
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAResend_ServiceErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.ResendMFAChallengeFn = func(context.Context, string, net.IP) (string, error) {
		return "", identityaccess.ErrMFARateLimited
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := jsonReq(t, http.MethodPost, "/mfa/resend", MFAResendRequest{ChallengeToken: "tok"})
	rr := httptest.NewRecorder()
	rs.mfaResend(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

// The start step answers the challenge token the confirm must present, so
// the confirm redeems exactly the code it mailed (H11).
func TestMFAEnrollStart_ReturnsTheChallengeToken(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.StartMFAChallengeFn = func(_ context.Context, accountID, tenantID int64, scope string, _ net.IP) (string, error) {
		assert.Equal(t, int64(42), accountID)
		assert.Equal(t, int64(70010001), tenantID)
		assert.Equal(t, identityaccess.MFAChallengeScopeTenant, scope)
		return "enroll-challenge", nil
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	var body portalauth.EnrollStartResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "enroll-challenge", body.ChallengeToken)
}

// The enrollment start is bound to the credential generation its token
// names (#6289): the handler hands exactly that generation on.
func TestMFAEnrollStart_PassesTheTokensCredentialGeneration(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	rs := &Resource{ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}
	r := jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil)
	r = r.WithContext(testutil.WithEnrollmentClaims(r.Context(), testutil.MFAEnrollmentClaims{
		AccountID: 42, Scope: tokenprimitives.PortalTenant, TenantID: 70010001,
		CredentialGeneration: 7, MFAEnrollmentPending: true,
	}))
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []int64{7}, svc.StartedCredentialGenerations)
}

// The confirm redeems the presented challenge pinned to the enrollment
// token's account and school; a confirm without a challenge token is
// refused before any code is compared.
func TestMFAEnrollConfirm_RedeemsThePresentedChallengeOfTheEnrollingAccount(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	var redeemed []string
	svc.VerifyMFAChallengeForOwnerFn = func(_ context.Context, challengeToken, code, scope string, accountID, tenantID int64) (identityaccess.VerifiedMFAChallenge, error) {
		redeemed = append(redeemed, challengeToken)
		assert.Equal(t, "123456", code)
		assert.Equal(t, identityaccess.MFAChallengeScopeTenant, scope)
		assert.Equal(t, int64(42), accountID)
		assert.Equal(t, int64(70010001), tenantID)
		return identityaccess.VerifiedMFAChallenge{AccountID: accountID, TenantID: tenantID, Scope: scope}, nil
	}
	svc.VerifyMFACodeForAccountFn = func(context.Context, int64, int64, string, string) error {
		t.Fatal("the confirm must not redeem the newest active code")
		return nil
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation,
		MFAService:          svc,
		Sessions:            &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"},
	}

	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "123456"}), 42, 70010001))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []string{"enroll-challenge"}, redeemed)

	rr = httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		map[string]string{"code": "123456"}), 42, 70010001))
	assert.Equal(t, http.StatusBadRequest, rr.Code, "a confirm without challenge_token is refused")
	assert.Equal(t, []string{"enroll-challenge"}, redeemed, "nothing is redeemed without a challenge token")
}

func TestMFAEnrollStart_StartChallengeErrorMapped(t *testing.T) {
	t.Parallel()

	// mfaEnrollStart is a thin wrapper around StartChallenge — actual
	// enrollment happens in mfaEnrollConfirm. A service-level rate-limit
	// must propagate as 429.
	svc := newStubMFAService()
	svc.StartMFAChallengeFn = func(context.Context, int64, int64, string, net.IP) (string, error) {
		return "", identityaccess.ErrMFARateLimited
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestMFAEnrollConfirm_AlreadyEnrolledMintsSession(t *testing.T) {
	t.Parallel()

	// Item #1 of #1430 review: confirm no longer returns 204 on
	// already-enrolled. It mints a full access/refresh pair so the
	// enrollment-scoped token never sticks around past enrollment.
	// ErrMFAAlreadyEnrolled is still swallowed so a retried confirm
	// (where the credential row was already written) advances to the
	// token-pair branch instead of bubbling up.
	svc := newStubMFAService()
	svc.VerifyMFAChallengeForOwnerFn = func(context.Context, string, string, string, int64, int64) (identityaccess.VerifiedMFAChallenge, error) {
		return identityaccess.VerifiedMFAChallenge{AccountID: 42, TenantID: 70010001, Scope: identityaccess.MFAChallengeScopeTenant}, nil
	}
	svc.EnrollMFAFn = func(context.Context, identityaccess.VerifiedMFAChallenge) error {
		return identityaccess.ErrMFAAlreadyEnrolled
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation,
		MFAService:          svc,
		Sessions:            &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"},
	}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "123456"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body portalauth.TokenResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "access-tok", body.AccessToken)
	assert.Equal(t, "refresh-tok", body.RefreshToken)
}

// TestMFAEnrollConfirm_StaleCredentialRecordsAndMintsNothing: the confirm
// hands the redeemed code's proof, with its credential generation, to the
// enrollment (#6518). When a password change or MFA reset moved the
// generation on since, the enrollment refuses it; the confirm answers the
// sign-in's refusal and mints no session and remembers no device.
func TestMFAEnrollConfirm_StaleCredentialRecordsAndMintsNothing(t *testing.T) {
	t.Parallel()

	proof := identityaccess.VerifiedMFAChallenge{AccountID: 42, TenantID: 70010001, Scope: identityaccess.MFAChallengeScopeTenant, MFAResetGeneration: 7}
	svc := newStubMFAService()
	svc.VerifyMFAChallengeForOwnerFn = func(context.Context, string, string, string, int64, int64) (identityaccess.VerifiedMFAChallenge, error) {
		return proof, nil
	}
	var enrolled []identityaccess.VerifiedMFAChallenge
	svc.EnrollMFAFn = func(_ context.Context, got identityaccess.VerifiedMFAChallenge) error {
		enrolled = append(enrolled, got)
		return identityaccess.ErrInvalidCredentials
	}
	sessions := &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation,
		MFAService:          svc,
		Sessions:            sessions,
	}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "123456", RememberDevice: true}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "identity.invalid_credentials", decodeWireError(t, rr).Code)
	assert.Equal(t, []identityaccess.VerifiedMFAChallenge{proof}, enrolled, "the enrollment holds the redeemed code's generation")
	assert.Zero(t, sessions.issueCalls, "a refused enrollment mints no session")
	assert.Empty(t, rr.Result().Cookies(), "and remembers no device")
}

func TestMFAEnrollConfirm_EnrollErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.VerifyMFAChallengeForOwnerFn = func(context.Context, string, string, string, int64, int64) (identityaccess.VerifiedMFAChallenge, error) {
		return identityaccess.VerifiedMFAChallenge{AccountID: 42, TenantID: 70010001, Scope: identityaccess.MFAChallengeScopeTenant}, nil
	}
	svc.EnrollMFAFn = func(context.Context, identityaccess.VerifiedMFAChallenge) error { return errors.New("db down") }
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "123456"}), 42, 70010001)
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
	t.Parallel()

	const tenantID int64 = 70010001
	var verifyTenant, enrollTenant int64
	svc := newStubMFAService()
	svc.VerifyMFAChallengeForOwnerFn = func(ctx context.Context, _, _, _ string, _, _ int64) (identityaccess.VerifiedMFAChallenge, error) {
		verifyTenant = tenant.FromContext(ctx)
		return identityaccess.VerifiedMFAChallenge{AccountID: 42, TenantID: tenantID, Scope: identityaccess.MFAChallengeScopeTenant}, nil
	}
	svc.EnrollMFAFn = func(ctx context.Context, _ identityaccess.VerifiedMFAChallenge) error {
		enrollTenant = tenant.FromContext(ctx)
		return nil
	}
	// The tenant reaches the context through the tenant runtime the root
	// binds (#2736); without one the handler cannot scope the context.
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation,
		MFAService:          svc,
		Sessions:            &completeMFAExchangeStub{access: "access-tok", refresh: "refresh-tok"},
		transactions:        identityaccessCompose.TenantUnitOfWork{},
	}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "123456"}), 42, tenantID)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, tenantID, verifyTenant,
		"VerifyCodeForAccount must run with the enrollment claim's tenant on the context")
	assert.Equal(t, tenantID, enrollTenant,
		"Enroll must run with the enrollment claim's tenant on the context")
}

func TestMFAEnrollConfirm_BindRejectsShortCode(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "12"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMFAEnrollConfirm_WrongCodeReturns401(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.VerifyMFAChallengeForOwnerFn = func(context.Context, string, string, string, int64, int64) (identityaccess.VerifiedMFAChallenge, error) {
		return identityaccess.VerifiedMFAChallenge{}, identityaccess.ErrMFACodeInvalid
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := withEnrollmentClaims(jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "654321"}), 42, 70010001)
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// listTrustedDevices returns an empty slice for an account with no rows.
func TestMFAListTrustedDevices_EmptyOK(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.ListTrustedDevicesFn = func(context.Context, int64, int64) ([]identityaccess.AccountTrustedDevice, error) {
		return []identityaccess.AccountTrustedDevice{}, nil
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := withClaims(httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
	var devices []any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&devices))
	assert.Empty(t, devices, "no rows means an empty list, not null")
}

func TestMFAListTrustedDevices_RequiresClaim(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestMFAListTrustedDevices_ServiceErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := newStubMFAService()
	svc.ListTrustedDevicesFn = func(context.Context, int64, int64) ([]identityaccess.AccountTrustedDevice, error) {
		return nil, errors.New("db down")
	}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: svc}

	r := withClaims(httptest.NewRequest(http.MethodGet, "/mfa/trusted-devices", nil), 42)
	rr := httptest.NewRecorder()
	rs.mfaListTrustedDevices(rr, r)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestMFAEnrollStart_RequiresEnrollmentClaim(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/start", nil) // no enrollment claim
	rr := httptest.NewRecorder()
	rs.mfaEnrollStart(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"missing enrollment claim must fail closed — this is the post-#1430 contract; the handler used to read CtxClaims and a regular session token would have worked")
}

func TestMFAEnrollConfirm_RequiresEnrollmentClaim(t *testing.T) {
	t.Parallel()

	// Without an enrollment claim on the context the handler must fail
	// closed — this also covers the case where a regular AppClaims token
	// (CtxClaims) was set instead. Renamed from RequiresAccountClaim to
	// reflect the post-#1430 shape.
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, MFAService: newStubMFAService()}

	r := jsonReq(t, http.MethodPost, "/mfa/enroll/confirm",
		MFAVerifyRequest{ChallengeToken: "enroll-challenge", Code: "123456"})
	rr := httptest.NewRecorder()
	rs.mfaEnrollConfirm(rr, r)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
