package operator

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type operatorPasskeyServiceStub struct {
	beginLoginOrigin      string
	finishLoginReq        platformSvc.OperatorPasskeyLoginFinishRequest
	enrollmentOperatorID  int64
	beginRegistrationReq  platformSvc.OperatorPasskeyRegistrationStartRequest
	finishRegistrationReq platformSvc.OperatorPasskeyRegistrationFinishRequest
	listOperatorID        int64
	revokeOperatorID      int64
	revokeCredentialID    int64
	beginLoginErr         error
	finishLoginErr        error
	startEnrollmentErr    error
	beginRegistrationErr  error
	finishRegistrationErr error
	listErr               error
	revokeErr             error
}

func (s *operatorPasskeyServiceStub) StartEnrollmentChallenge(_ context.Context, operatorID int64, _ net.IP) (*platformSvc.OperatorPasskeyEnrollmentChallenge, error) {
	s.enrollmentOperatorID = operatorID
	if s.startEnrollmentErr != nil {
		return nil, s.startEnrollmentErr
	}
	return &platformSvc.OperatorPasskeyEnrollmentChallenge{ChallengeToken: "operator-challenge-token", MaskedEmail: "o***@example.test"}, nil
}

func (s *operatorPasskeyServiceStub) BeginRegistration(_ context.Context, req platformSvc.OperatorPasskeyRegistrationStartRequest) (*authSvc.PasskeyCredentialCreation, error) {
	s.beginRegistrationReq = req
	if s.beginRegistrationErr != nil {
		return nil, s.beginRegistrationErr
	}
	return &authSvc.PasskeyCredentialCreation{SessionID: "operator-registration", Options: map[string]string{"challenge": "register"}}, nil
}

func (s *operatorPasskeyServiceStub) FinishRegistration(_ context.Context, req platformSvc.OperatorPasskeyRegistrationFinishRequest) (*authSvc.PasskeyCredentialSummary, error) {
	s.finishRegistrationReq = req
	if s.finishRegistrationErr != nil {
		return nil, s.finishRegistrationErr
	}
	return &authSvc.PasskeyCredentialSummary{ID: "9", Name: "Admin laptop", CreatedAt: time.Unix(1, 0).UTC()}, nil
}

func (s *operatorPasskeyServiceStub) BeginLogin(_ context.Context, expectedOrigin string) (*authSvc.PasskeyCredentialAssertion, error) {
	s.beginLoginOrigin = expectedOrigin
	if s.beginLoginErr != nil {
		return nil, s.beginLoginErr
	}
	return &authSvc.PasskeyCredentialAssertion{SessionID: "operator-login", Options: map[string]string{"challenge": "login"}}, nil
}

func (s *operatorPasskeyServiceStub) FinishLogin(_ context.Context, req platformSvc.OperatorPasskeyLoginFinishRequest) (*authSvc.PasskeyLoginResult, error) {
	s.finishLoginReq = req
	if s.finishLoginErr != nil {
		return nil, s.finishLoginErr
	}
	return &authSvc.PasskeyLoginResult{AccessToken: "operator-access", RefreshToken: "operator-refresh"}, nil
}

func (s *operatorPasskeyServiceStub) ListCredentials(_ context.Context, operatorID int64) ([]authSvc.PasskeyCredentialSummary, error) {
	s.listOperatorID = operatorID
	if s.listErr != nil {
		return nil, s.listErr
	}
	return []authSvc.PasskeyCredentialSummary{{ID: "10", Name: "Phone", CreatedAt: time.Unix(2, 0).UTC()}}, nil
}

func (s *operatorPasskeyServiceStub) RevokeCredential(_ context.Context, operatorID, credentialID int64) error {
	s.revokeOperatorID = operatorID
	s.revokeCredentialID = credentialID
	return s.revokeErr
}

func TestOperatorPasskeyLoginHandlers(t *testing.T) {
	t.Parallel()

	svc := &operatorPasskeyServiceStub{}
	rs := &Resource{passkeyService: svc}

	w := httptest.NewRecorder()
	req := operatorPasskeyJSONRequest("/auth/passkeys/login/options", `{}`)
	req.Header.Set(headerOperatorFrontendOrigin, "https://operator.localhost")
	rs.PasskeyLoginOptions(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://operator.localhost", svc.beginLoginOrigin)
	assert.JSONEq(t, `{"session_id":"operator-login","options":{"challenge":"login"}}`, w.Body.String())

	w = httptest.NewRecorder()
	req = operatorPasskeyJSONRequest("/auth/passkeys/login/verify", `{"session_id":"operator-login","response":{"id":"assertion"}}`)
	req.Header.Set("User-Agent", "operator-agent")
	req.RemoteAddr = "203.0.113.11:1234"
	rs.PasskeyLoginVerify(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "operator-login", svc.finishLoginReq.SessionID)
	assert.Equal(t, "operator-agent", svc.finishLoginReq.UserAgent)
	assert.JSONEq(t, `{"status":"authenticated","access_token":"operator-access","refresh_token":"operator-refresh"}`, w.Body.String())
}

func TestOperatorPasskeyLoginRoutesArePublic(t *testing.T) {
	t.Parallel()

	tokenAuth, err := jwt.NewTokenAuth()
	require.NoError(t, err)

	svc := &operatorPasskeyServiceStub{}
	router := NewResource(ResourceConfig{
		PasskeyService: svc,
		TokenAuth:      tokenAuth,
	}).Router()

	req := operatorPasskeyJSONRequest("/auth/passkeys/login/options", `{}`)
	req.Header.Set(headerOperatorFrontendOrigin, "https://operator.localhost")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://operator.localhost", svc.beginLoginOrigin)
	assert.JSONEq(t, `{"session_id":"operator-login","options":{"challenge":"login"}}`, w.Body.String())
}

// TestOperatorPasskeyListRouteAcceptsBothSlashForms pins the compatibility
// fix: GET /auth/passkeys (no slash) and GET /auth/passkeys/ (slash) must both
// be routed to PasskeyList. chi has no RedirectSlashes on this router, so the
// two are distinct patterns; an UNregistered path 404s before any middleware
// runs, while a registered protected path reaches the auth middleware and 401s
// without a token. Asserting 401 (not 404) for both forms proves both are
// registered. Regression guard for the route dropped in PR #1690.
func TestOperatorPasskeyListRouteAcceptsBothSlashForms(t *testing.T) {
	t.Parallel()

	tokenAuth, err := jwt.NewTokenAuth()
	require.NoError(t, err)

	router := NewResource(ResourceConfig{
		PasskeyService: &operatorPasskeyServiceStub{},
		TokenAuth:      tokenAuth,
	}).Router()

	for _, path := range []string{"/auth/passkeys", "/auth/passkeys/"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.NotEqual(t, http.StatusNotFound, w.Code,
			"GET %s must be a registered route, not 404", path)
		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"GET %s is protected, so an unauthenticated request must 401", path)
	}
}

func TestOperatorPasskeyAuthenticatedHandlers(t *testing.T) {
	t.Parallel()

	svc := &operatorPasskeyServiceStub{}
	rs := &Resource{passkeyService: svc}
	claims := jwt.AppClaims{ID: 21, Scope: "platform"}

	w := httptest.NewRecorder()
	rs.PasskeyEnrollmentChallenge(w, withOperatorPasskeyClaims(operatorPasskeyJSONRequest("/auth/passkeys/enrollment/challenge", `{}`), claims))
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, claims.ID, svc.enrollmentOperatorID)
	assert.JSONEq(t, `{"challenge_token":"operator-challenge-token","masked_email":"o***@example.test"}`, w.Body.String())

	w = httptest.NewRecorder()
	req := withOperatorPasskeyClaims(operatorPasskeyJSONRequest("/auth/passkeys/register/options", `{"code":"654321","name":" Admin laptop "}`), claims)
	req.Header.Set("Origin", "https://operator.localhost")
	rs.PasskeyRegisterOptions(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, claims.ID, svc.beginRegistrationReq.OperatorID)
	assert.Equal(t, "Admin laptop", svc.beginRegistrationReq.Name)

	w = httptest.NewRecorder()
	rs.PasskeyRegisterVerify(w, withOperatorPasskeyClaims(operatorPasskeyJSONRequest("/auth/passkeys/register/verify", `{"session_id":"operator-registration","name":"Phone","response":{"id":"credential"}}`), claims))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "operator-registration", svc.finishRegistrationReq.SessionID)
	assert.JSONEq(t, `{"id":"9","name":"Admin laptop","created_at":"1970-01-01T00:00:01Z"}`, w.Body.String())

	w = httptest.NewRecorder()
	rs.PasskeyList(w, withOperatorPasskeyClaims(httptest.NewRequest(http.MethodGet, "/auth/passkeys", nil), claims))
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, claims.ID, svc.listOperatorID)
	assert.JSONEq(t, `[{"id":"10","name":"Phone","created_at":"1970-01-01T00:00:02Z"}]`, w.Body.String())

	w = httptest.NewRecorder()
	passkeyID := svc.enrollmentOperatorID + 1
	rs.PasskeyRevoke(w, withOperatorPasskeyRouteParam(
		withOperatorPasskeyClaims(httptest.NewRequest(http.MethodDelete, "/auth/passkeys/"+strconv.FormatInt(passkeyID, 10), nil), claims),
		"passkeyId",
		strconv.FormatInt(passkeyID, 10),
	))
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.EqualValues(t, claims.ID, svc.revokeOperatorID)
	assert.Equal(t, passkeyID, svc.revokeCredentialID)
}

func TestOperatorPasskeyHandlerErrors(t *testing.T) {
	t.Parallel()

	rs := &Resource{}
	w := httptest.NewRecorder()
	rs.PasskeyLoginOptions(w, operatorPasskeyJSONRequest("/auth/passkeys/login/options", `{}`))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	rs.passkeyService = &operatorPasskeyServiceStub{}
	w = httptest.NewRecorder()
	rs.PasskeyLoginOptions(w, operatorPasskeyJSONRequest("/auth/passkeys/login/options", `{}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	mapOperatorPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), authSvc.ErrPasskeyNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	mapOperatorPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func operatorPasskeyJSONRequest(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func withOperatorPasskeyClaims(req *http.Request, claims jwt.AppClaims) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
}

func withOperatorPasskeyRouteParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
