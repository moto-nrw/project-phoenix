package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completeOperatorMFAAuthStub satisfies OperatorAccess via
// interface-embedding — only IssueTokensForAuthenticatedOperator is
// implemented; any other call path panics so accidental drift in MFAResource
// surfaces loudly. Distinct from the existing operator-auth login stubs so
// it can be tuned independently without touching the no-test-modifications
// boundary.
type completeOperatorMFAAuthStub struct {
	OperatorAccess
	access  string
	refresh string
	err     error

	gotOperatorID int64
	gotIP         string
	gotUA         string
}

var _ OperatorAccess = (*completeOperatorMFAAuthStub)(nil)

func (s *completeOperatorMFAAuthStub) IssueTokensForAuthenticatedOperator(
	_ context.Context,
	operatorID int64,
	ipAddress, userAgent string,
) (string, string, error) {
	s.gotOperatorID = operatorID
	s.gotIP = ipAddress
	s.gotUA = userAgent
	return s.access, s.refresh, s.err
}

// operatorTrustedDeviceStub extends stubOperatorMFAServiceExtra with a
// controllable VerifyOperatorMFAChallenge + IssueOperatorTrustedDevice so we can drive the
// happy-path through completeMFAExchange + issueTrustedDeviceCookie.
type operatorTrustedDeviceStub struct {
	stubOperatorMFAServiceExtra
	verifyResult   int64
	verifyErr      error
	issueCookie    string
	issueExpiresAt time.Time
	issueErr       error

	gotIssueOperatorID int64
	gotIssueUA         string
	gotIssueIP         net.IP
}

func (s *operatorTrustedDeviceStub) VerifyOperatorMFAChallenge(_ context.Context, _, _ string) (int64, error) {
	return s.verifyResult, s.verifyErr
}

func (s *operatorTrustedDeviceStub) IssueOperatorTrustedDevice(_ context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	s.gotIssueOperatorID = operatorID
	s.gotIssueUA = userAgent
	s.gotIssueIP = ip
	return s.issueCookie, s.issueExpiresAt, s.issueErr
}

func (s *operatorTrustedDeviceStub) OperatorTrustedDeviceDays() int { return 90 }

var _ identityoperator.OperatorMFA = (*operatorTrustedDeviceStub)(nil)

func opVerifyRequest(t *testing.T, body MFAVerifyRequest) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/operator/auth/mfa/verify", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "operator-ua")
	r.Header.Set("X-Forwarded-For", "203.0.113.99")
	r.RemoteAddr = "203.0.113.99:54321"
	return r
}

func TestOperatorMFAVerify_SuccessReturnsTokenPair(t *testing.T) {
	t.Parallel()

	auth := &completeOperatorMFAAuthStub{access: "op-access", refresh: "op-refresh"}
	mfa := &operatorTrustedDeviceStub{
		verifyResult: 4242,
	}
	rs := &MFAResource{authService: auth, mfaService: mfa}

	rr := httptest.NewRecorder()
	rs.Verify(rr, opVerifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(4242), auth.gotOperatorID)
	assert.Equal(t, "203.0.113.99", auth.gotIP)
	assert.Equal(t, "operator-ua", auth.gotUA)

	assert.Empty(t, rr.Result().Cookies(),
		"remember_device defaulted false must not produce a Set-Cookie")
}

func TestOperatorMFAVerify_RememberDeviceIssuesCookie(t *testing.T) {
	t.Parallel()

	auth := &completeOperatorMFAAuthStub{access: "a", refresh: "r"}
	mfa := &operatorTrustedDeviceStub{
		verifyResult:   4242,
		issueCookie:    "op.td.cookie",
		issueExpiresAt: time.Now().Add(90 * 24 * time.Hour),
	}
	rs := &MFAResource{authService: auth, mfaService: mfa}

	rr := httptest.NewRecorder()
	rs.Verify(rr, opVerifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
		RememberDevice: true,
	}))

	require.Equal(t, http.StatusOK, rr.Code)
	cookies := rr.Result().Cookies()
	require.Len(t, cookies, 1, "remember_device=true must issue exactly one trusted-device cookie")
	c := cookies[0]
	assert.Equal(t, trustedDeviceCookieName, c.Name)
	assert.Equal(t, "op.td.cookie", c.Value)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, "/", c.Path)
	assert.Greater(t, c.MaxAge, 0)

	assert.Equal(t, int64(4242), mfa.gotIssueOperatorID)
	assert.Equal(t, "operator-ua", mfa.gotIssueUA)
	require.NotNil(t, mfa.gotIssueIP)
	assert.Equal(t, "203.0.113.99", mfa.gotIssueIP.String())
}

func TestOperatorMFAVerify_RememberDeviceFailureDoesNotBreakLogin(t *testing.T) {
	t.Parallel()

	auth := &completeOperatorMFAAuthStub{access: "a", refresh: "r"}
	mfa := &operatorTrustedDeviceStub{
		verifyResult: 1,
		issueErr:     errors.New("cookie store down"),
	}
	rs := &MFAResource{authService: auth, mfaService: mfa}

	rr := httptest.NewRecorder()
	rs.Verify(rr, opVerifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
		RememberDevice: true,
	}))

	require.Equal(t, http.StatusOK, rr.Code,
		"a cookie-issuance failure is a UX regression, not a security one — login must still succeed")
	assert.Empty(t, rr.Result().Cookies(),
		"failed IssueTrustedDevice must NOT produce a Set-Cookie with garbage")
}

func TestOperatorMFAVerify_InactiveOperatorReturns403(t *testing.T) {
	t.Parallel()

	auth := &completeOperatorMFAAuthStub{err: identityoperator.ErrOperatorInactive}
	mfa := &operatorTrustedDeviceStub{
		verifyResult: 1,
	}
	rs := &MFAResource{authService: auth, mfaService: mfa}

	rr := httptest.NewRecorder()
	rs.Verify(rr, opVerifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	assert.Equal(t, http.StatusForbidden, rr.Code,
		"inactive operators surface as 403, matching the rest of the operator surface")
}

func TestOperatorMFAVerify_NotFoundReturns401(t *testing.T) {
	t.Parallel()

	auth := &completeOperatorMFAAuthStub{err: identityoperator.ErrOperatorNotFound}
	mfa := &operatorTrustedDeviceStub{
		verifyResult: 1,
	}
	rs := &MFAResource{authService: auth, mfaService: mfa}

	rr := httptest.NewRecorder()
	rs.Verify(rr, opVerifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOperatorMFAVerify_IssueTokensUnknownErrorMapsTo500(t *testing.T) {
	t.Parallel()

	auth := &completeOperatorMFAAuthStub{err: errors.New("kafka is on fire")}
	mfa := &operatorTrustedDeviceStub{
		verifyResult: 1,
	}
	rs := &MFAResource{authService: auth, mfaService: mfa}

	rr := httptest.NewRecorder()
	rs.Verify(rr, opVerifyRequest(t, MFAVerifyRequest{
		ChallengeToken: "challenge.tok",
		Code:           "123456",
	}))

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
