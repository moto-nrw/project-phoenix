package parentauth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/inbound/parentauth"
)

// stubSignIn stands up the parents sign-in the composition root binds; the
// second factor's management is not reached by these routes.
type stubSignIn struct {
	login  func(identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error)
	verify func(identityaccess.ParentMFAVerification) (identityaccess.ParentSession, error)
}

func (s stubSignIn) LoginParent(_ context.Context, request identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
	return s.login(request)
}

func (s stubSignIn) VerifyParentMFA(_ context.Context, verification identityaccess.ParentMFAVerification) (identityaccess.ParentSession, error) {
	return s.verify(verification)
}

func (stubSignIn) ResendParentMFA(context.Context, string, net.IP) (string, error) {
	return "", errors.New("stub: resend not expected")
}

type unusedSecondFactor struct {
	identityaccess.ParentSecondFactor
}

type signInRecorder struct{ emails []string }

func (r *signInRecorder) after(_ context.Context, email string) { r.emails = append(r.emails, email) }

func router(signIn stubSignIn, recorder *signInRecorder) chi.Router {
	r := chi.NewRouter()
	parentauth.NewResource(identityaccess.NewParentAccess(signIn, unusedSecondFactor{})).MountSignIn(r, recorder.after)
	return r
}

func post(t *testing.T, r http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

type refusal struct {
	Code    string            `json:"code"`
	Details map[string]string `json:"details"`
}

func refusalOf(t *testing.T, rr *httptest.ResponseRecorder) refusal {
	t.Helper()
	var body refusal
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func refusing(err error) stubSignIn {
	return stubSignIn{login: func(identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
		return identityaccess.ParentLoginResult{}, err
	}}
}

var credentials = map[string]string{"email": " Parent@Example.com ", "password": "correct-horse-battery"}

// A sign-in without a second factor answers with the session pair in the
// envelope the parents BFF unwraps, and hands the normalized address on.
func TestParentLoginWithoutSecondFactorReturnsTheSession(t *testing.T) {
	t.Parallel()
	recorder := &signInRecorder{}
	rr := post(t, router(stubSignIn{login: func(request identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
		assert.Equal(t, "parent@example.com", request.Email)
		assert.Equal(t, "correct-horse-battery", request.Password)
		return identityaccess.ParentLoginResult{Status: identityaccess.LoginStatusAuthenticated, AccessToken: "access-tok", RefreshToken: "refresh-tok"}, nil
	}}, recorder), "/login", credentials)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var resp struct {
		Data struct {
			Status       string `json:"status"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "authenticated", resp.Data.Status)
	assert.Equal(t, "access-tok", resp.Data.AccessToken)
	assert.Equal(t, "refresh-tok", resp.Data.RefreshToken)
	assert.Equal(t, []string{"parent@example.com"}, recorder.emails)
}

// A parent with the second factor gets the challenge and no session; the
// sign-in is not complete, so nothing is claimed yet.
func TestParentLoginWithSecondFactorAnswersTheChallenge(t *testing.T) {
	t.Parallel()
	recorder := &signInRecorder{}
	rr := post(t, router(stubSignIn{login: func(identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
		return identityaccess.ParentLoginResult{Status: identityaccess.LoginStatusMFARequired, ChallengeToken: "challenge", MaskedEmail: "p***@example.com", TrustedDeviceDays: 90}, nil
	}}, recorder), "/login", credentials)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "mfa_required", resp.Data["status"])
	assert.Equal(t, "challenge", resp.Data["challenge_token"])
	assert.Equal(t, true, resp.Data["trusted_device_enabled"])
	assert.EqualValues(t, 90, resp.Data["trusted_device_days"])
	assert.NotContains(t, resp.Data, "access_token")
	assert.Empty(t, recorder.emails)
}

// The redeemed code completes the sign-in: the pair, the remember-device
// cookie, and the address handed on.
func TestParentMFAVerifyCompletesTheSignIn(t *testing.T) {
	t.Parallel()
	recorder := &signInRecorder{}
	rr := post(t, router(stubSignIn{verify: func(verification identityaccess.ParentMFAVerification) (identityaccess.ParentSession, error) {
		assert.True(t, verification.RememberDevice)
		return identityaccess.ParentSession{AccessToken: "a", RefreshToken: "r", Email: "parent@example.com", TrustedDeviceCookie: "signed"}, nil
	}}, recorder), "/mfa/verify", map[string]any{"challenge_token": "challenge", "code": "123456", "remember_device": true})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Header().Get("Set-Cookie"), "mfa_trust_device=signed")
	assert.Equal(t, []string{"parent@example.com"}, recorder.emails)
}

// The parents login page branches on these codes, so each refusal of the
// owner keeps its answer.
func TestParentLoginRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		cause  error
		status int
		code   string
	}{
		{"wrong password", identityaccess.ErrInvalidCredentials, http.StatusUnauthorized, "care.invalid_credentials"},
		// An unknown address answers like a wrong password; a distinct
		// answer would tell a caller which addresses exist.
		{"unknown address", identityaccess.ErrAccountNotFound, http.StatusUnauthorized, "care.invalid_credentials"},
		{"account switched off", identityaccess.ErrAccountInactive, http.StatusUnauthorized, "identity.session_account_inactive"},
		// A staff account at the parents login; the page masks it but keeps
		// the hint to the staff login.
		{"no guardian", identityaccess.ErrAccountNoGuardianRole, http.StatusForbidden, "care.not_a_guardian"},
		{"code blocked", identityaccess.ErrMFALocked, http.StatusTooManyRequests, "identity.mfa_blocked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := post(t, router(refusing(&identityaccess.AuthenticationError{Op: "login", Err: tc.cause}), &signInRecorder{}), "/login", credentials)
			assert.Equal(t, tc.status, rr.Code)
			assert.Equal(t, tc.code, refusalOf(t, rr).Code)
		})
	}
}

// None of the guardian's schools booked Eltern-App (#4895): the page explains
// the missing module.
func TestParentLoginWithoutTheModuleNamesTheModule(t *testing.T) {
	t.Parallel()
	rr := post(t, router(refusing(&identityaccess.PortalNotBookedError{Module: "eltern_app", ModuleLabel: "Eltern-App"}), &signInRecorder{}), "/login", credentials)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	body := refusalOf(t, rr)
	assert.Equal(t, "module.not_booked", body.Code)
	assert.Equal(t, "eltern_app", body.Details["module"])
	assert.Equal(t, "Eltern-App", body.Details["module_label"])
}

// A 400 keeps the user on the form with a correction hint, a 500 surfaces a
// generic failure; neither may masquerade as the other.
func TestParentLoginInputAndServerFaults(t *testing.T) {
	t.Parallel()
	never := stubSignIn{login: func(identityaccess.ParentLoginRequest) (identityaccess.ParentLoginResult, error) {
		t.Fatal("the sign-in must not run for an invalid request body")
		return identityaccess.ParentLoginResult{}, nil
	}}
	rr := post(t, router(never, &signInRecorder{}), "/login", map[string]string{"email": "not-an-email", "password": "whatever"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = post(t, router(refusing(errors.New("database exploded")), &signInRecorder{}), "/login", credentials)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

type refusingDevices struct {
	identityaccess.ParentSecondFactor
	err error
}

func (d refusingDevices) ListParentTrustedDevices(context.Context, int64) ([]identityaccess.AccountTrustedDevice, error) {
	return nil, d.err
}

func (d refusingDevices) RevokeParentTrustedDevice(context.Context, int64, int64) error { return d.err }

// The remembered devices answer a refusal like the other settings routes of
// the second factor: before the routes shared their table, an account that
// ended or a module without the factor answered 500 here.
func TestParentTrustedDevicesAnswerLikeTheSecondFactorSettings(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err    error
		status int
	}{
		"factor not composed": {identityaccess.ErrAccountMFAUnavailable, http.StatusServiceUnavailable},
		"account ended":       {identityaccess.ErrAccountInactive, http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := chi.NewRouter()
			parentauth.NewResource(identityaccess.NewParentAccess(stubSignIn{}, refusingDevices{err: tc.err})).MountSession(r)
			list := httptest.NewRecorder()
			r.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/me/mfa/trusted-devices", nil))
			assert.Equal(t, tc.status, list.Code, list.Body.String())
			revoke := httptest.NewRecorder()
			r.ServeHTTP(revoke, httptest.NewRequest(http.MethodDelete, "/me/mfa/trusted-devices/7", nil))
			assert.Equal(t, tc.status, revoke.Code, revoke.Body.String())
		})
	}
}
