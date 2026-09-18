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

	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type operatorPasskeyServiceStub struct {
	beginLoginOrigin      string
	finishLoginReq        identityoperator.OperatorPasskeyLoginFinish
	enrollmentOperatorID  int64
	beginRegistrationReq  identityoperator.OperatorPasskeyRegistrationStart
	finishRegistrationReq identityoperator.OperatorPasskeyRegistrationFinish
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

func (s *operatorPasskeyServiceStub) StartOperatorPasskeyEnrollment(_ context.Context, operatorID int64, _ net.IP) (identityoperator.PasskeyEnrollmentChallenge, error) {
	s.enrollmentOperatorID = operatorID
	if s.startEnrollmentErr != nil {
		return identityoperator.PasskeyEnrollmentChallenge{}, s.startEnrollmentErr
	}
	return identityoperator.PasskeyEnrollmentChallenge{ChallengeToken: "operator-challenge-token", MaskedEmail: "o***@example.test"}, nil
}

func (s *operatorPasskeyServiceStub) BeginOperatorPasskeyRegistration(_ context.Context, req identityoperator.OperatorPasskeyRegistrationStart) (identityoperator.PasskeyCeremonyOptions, error) {
	s.beginRegistrationReq = req
	if s.beginRegistrationErr != nil {
		return identityoperator.PasskeyCeremonyOptions{}, s.beginRegistrationErr
	}
	return identityoperator.PasskeyCeremonyOptions{SessionID: "operator-registration", Options: map[string]string{"challenge": "register"}}, nil
}

func (s *operatorPasskeyServiceStub) FinishOperatorPasskeyRegistration(_ context.Context, req identityoperator.OperatorPasskeyRegistrationFinish) (identityoperator.PasskeyCredentialSummary, error) {
	s.finishRegistrationReq = req
	if s.finishRegistrationErr != nil {
		return identityoperator.PasskeyCredentialSummary{}, s.finishRegistrationErr
	}
	return identityoperator.PasskeyCredentialSummary{ID: "9", Name: "Admin laptop", CreatedAt: time.Unix(1, 0).UTC()}, nil
}

func (s *operatorPasskeyServiceStub) BeginOperatorPasskeyLogin(_ context.Context, expectedOrigin string) (identityoperator.PasskeyCeremonyOptions, error) {
	s.beginLoginOrigin = expectedOrigin
	if s.beginLoginErr != nil {
		return identityoperator.PasskeyCeremonyOptions{}, s.beginLoginErr
	}
	return identityoperator.PasskeyCeremonyOptions{SessionID: "operator-login", Options: map[string]string{"challenge": "login"}}, nil
}

func (s *operatorPasskeyServiceStub) FinishOperatorPasskeyLogin(_ context.Context, req identityoperator.OperatorPasskeyLoginFinish) (identityoperator.PasskeyLoginResult, error) {
	s.finishLoginReq = req
	if s.finishLoginErr != nil {
		return identityoperator.PasskeyLoginResult{}, s.finishLoginErr
	}
	return identityoperator.PasskeyLoginResult{AccessToken: "operator-access", RefreshToken: "operator-refresh"}, nil
}

func (s *operatorPasskeyServiceStub) ListOperatorPasskeyCredentials(_ context.Context, operatorID int64) ([]identityoperator.PasskeyCredentialSummary, error) {
	s.listOperatorID = operatorID
	if s.listErr != nil {
		return nil, s.listErr
	}
	return []identityoperator.PasskeyCredentialSummary{{ID: "10", Name: "Phone", CreatedAt: time.Unix(2, 0).UTC()}}, nil
}

func (s *operatorPasskeyServiceStub) RevokeOperatorPasskeyCredential(_ context.Context, operatorID, credentialID int64) error {
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
	mapOperatorPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), identityoperator.ErrPasskeyNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	mapOperatorPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestOperatorPasskeyStoreFailuresAreNotClientErrors pins the wire outcome
// of the passkey completions (#2724): a refused ceremony or a missing
// passkey stays a client error, a store failure behind them is a server
// error.
func TestOperatorPasskeyStoreFailuresAreNotClientErrors(t *testing.T) {
	t.Parallel()

	storeDown := errors.New("database error during consume operator passkey session: connection reset")
	claims := jwt.AppClaims{ID: 41}
	loginVerify := func(rs *Resource, w http.ResponseWriter) {
		rs.PasskeyLoginVerify(w, operatorPasskeyJSONRequest("/auth/passkeys/login/verify", `{"session_id":"s","response":{"id":"a"}}`))
	}
	registerVerify := func(rs *Resource, w http.ResponseWriter) {
		req := operatorPasskeyJSONRequest("/auth/passkeys/register/verify", `{"session_id":"s","response":{"id":"a"}}`)
		rs.PasskeyRegisterVerify(w, withOperatorPasskeyClaims(req, claims))
	}
	revoke := func(rs *Resource, w http.ResponseWriter) {
		req := withOperatorPasskeyClaims(httptest.NewRequest(http.MethodDelete, "/auth/passkeys/7", nil), claims)
		rs.PasskeyRevoke(w, withOperatorPasskeyRouteParam(req, "passkeyId", "7"))
	}
	tests := []struct {
		name     string
		svc      *operatorPasskeyServiceStub
		call     func(*Resource, http.ResponseWriter)
		wantCode int
	}{
		{"login with a spent ceremony", &operatorPasskeyServiceStub{finishLoginErr: identityoperator.ErrPasskeySessionInvalid}, loginVerify, http.StatusUnauthorized},
		{"login with wrong credentials", &operatorPasskeyServiceStub{finishLoginErr: identityoperator.ErrOperatorInvalidCredentials}, loginVerify, http.StatusUnauthorized},
		{"registration for an inactive operator", &operatorPasskeyServiceStub{finishRegistrationErr: identityoperator.ErrOperatorInactive}, registerVerify, http.StatusForbidden},
		{"login with a store failure", &operatorPasskeyServiceStub{finishLoginErr: storeDown}, loginVerify, http.StatusInternalServerError},
		{"registration with a spent ceremony", &operatorPasskeyServiceStub{finishRegistrationErr: identityoperator.ErrPasskeySessionInvalid}, registerVerify, http.StatusUnauthorized},
		{"registration with a store failure", &operatorPasskeyServiceStub{finishRegistrationErr: storeDown}, registerVerify, http.StatusInternalServerError},
		{"revoke of a missing passkey", &operatorPasskeyServiceStub{revokeErr: identityoperator.ErrPasskeyNotFound}, revoke, http.StatusNotFound},
		{"revoke with a store failure", &operatorPasskeyServiceStub{revokeErr: storeDown}, revoke, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.call(&Resource{passkeyService: tt.svc}, w)
			assert.Equal(t, tt.wantCode, w.Code)
			assert.NotContains(t, w.Body.String(), "connection reset", "store details stay out of the response")
		})
	}
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
