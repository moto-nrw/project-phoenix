package auth

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
	"github.com/moto-nrw/project-phoenix/models/base"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type passkeyServiceStub struct {
	beginLoginReq         authService.PasskeyLoginStartRequest
	finishLoginReq        authService.PasskeyLoginFinishRequest
	beginRegistrationReq  authService.PasskeyRegistrationStartRequest
	finishRegistrationReq authService.PasskeyRegistrationFinishRequest
	enrollmentAccountID   int64
	enrollmentTenantID    int64
	listAccountID         int64
	revokeAccountID       int64
	revokeCredentialID    int64
	beginLoginErr         error
	finishLoginErr        error
	beginRegistrationErr  error
	finishRegistrationErr error
	startEnrollmentErr    error
	listErr               error
	revokeErr             error
}

func (s *passkeyServiceStub) StartEnrollmentChallenge(_ context.Context, accountID, tenantID int64, _ net.IP) (*authService.PasskeyEnrollmentChallenge, error) {
	s.enrollmentAccountID = accountID
	s.enrollmentTenantID = tenantID
	if s.startEnrollmentErr != nil {
		return nil, s.startEnrollmentErr
	}
	return &authService.PasskeyEnrollmentChallenge{ChallengeToken: "challenge-token", MaskedEmail: "m***@example.test"}, nil
}

func (s *passkeyServiceStub) BeginRegistration(_ context.Context, req authService.PasskeyRegistrationStartRequest) (*authService.PasskeyCredentialCreation, error) {
	s.beginRegistrationReq = req
	if s.beginRegistrationErr != nil {
		return nil, s.beginRegistrationErr
	}
	return &authService.PasskeyCredentialCreation{SessionID: "registration-session", Options: map[string]string{"challenge": "register"}}, nil
}

func (s *passkeyServiceStub) FinishRegistration(_ context.Context, req authService.PasskeyRegistrationFinishRequest) (*authService.PasskeyCredentialSummary, error) {
	s.finishRegistrationReq = req
	if s.finishRegistrationErr != nil {
		return nil, s.finishRegistrationErr
	}
	return &authService.PasskeyCredentialSummary{ID: "7", Name: "Laptop", CreatedAt: time.Unix(1, 0).UTC()}, nil
}

func (s *passkeyServiceStub) BeginLogin(_ context.Context, req authService.PasskeyLoginStartRequest) (*authService.PasskeyCredentialAssertion, error) {
	s.beginLoginReq = req
	if s.beginLoginErr != nil {
		return nil, s.beginLoginErr
	}
	return &authService.PasskeyCredentialAssertion{SessionID: "login-session", Options: map[string]string{"challenge": "login"}}, nil
}

func (s *passkeyServiceStub) FinishLogin(_ context.Context, req authService.PasskeyLoginFinishRequest) (*authService.PasskeyLoginResult, error) {
	s.finishLoginReq = req
	if s.finishLoginErr != nil {
		return nil, s.finishLoginErr
	}
	return &authService.PasskeyLoginResult{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func (s *passkeyServiceStub) ListCredentials(_ context.Context, accountID int64) ([]authService.PasskeyCredentialSummary, error) {
	s.listAccountID = accountID
	if s.listErr != nil {
		return nil, s.listErr
	}
	return []authService.PasskeyCredentialSummary{{ID: "8", Name: "Phone", CreatedAt: time.Unix(2, 0).UTC()}}, nil
}

func (s *passkeyServiceStub) RevokeCredential(_ context.Context, accountID, credentialID int64) error {
	s.revokeAccountID = accountID
	s.revokeCredentialID = credentialID
	return s.revokeErr
}

type passkeySchoolServiceStub struct {
	byID        *platformModel.School
	bySubdomain *platformModel.School
}

func (s passkeySchoolServiceStub) GetSchoolByID(context.Context, int64) (*platformModel.School, error) {
	return s.byID, nil
}

func (s passkeySchoolServiceStub) GetSchoolBySlug(context.Context, string) (*platformModel.School, error) {
	return nil, nil
}

func (s passkeySchoolServiceStub) GetSchoolBySubdomain(context.Context, string) (*platformModel.School, error) {
	return s.bySubdomain, nil
}

func (s passkeySchoolServiceStub) ListPublicSchools(context.Context) ([]platformModel.School, error) {
	return nil, nil
}

func (s passkeySchoolServiceStub) ListActiveSchoolsByAccountID(context.Context, int64) ([]platformModel.School, error) {
	return nil, nil
}

func TestPasskeyLoginHandlers(t *testing.T) {
	svc := &passkeyServiceStub{}
	school := &platformModel.School{
		Model:     base.Model{ID: 42},
		Subdomain: "school-a",
		Active:    true,
	}
	rs := &Resource{
		PasskeyService: svc,
		SchoolService: passkeySchoolServiceStub{
			bySubdomain: school,
		},
	}

	w := httptest.NewRecorder()
	req := passkeyJSONRequest("/passkeys/login/options", `{"tenant_slug":" school-a "}`)
	req.Header.Set(headerMotoFrontendOrigin, "https://school-a.localhost")
	rs.passkeyLoginOptions(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, school.ID, svc.beginLoginReq.TenantID)
	assert.Equal(t, "school-a", svc.beginLoginReq.TenantSubdomain)
	assert.Equal(t, "https://school-a.localhost", svc.beginLoginReq.ExpectedOrigin)
	assert.JSONEq(t, `{"session_id":"login-session","options":{"challenge":"login"}}`, w.Body.String())

	w = httptest.NewRecorder()
	req = passkeyJSONRequest("/passkeys/login/verify", `{"session_id":"login-session","response":{"id":"assertion"}}`)
	req.Header.Set(headerUserAgent, "test-agent")
	req.RemoteAddr = "203.0.113.10:1234"
	rs.passkeyLoginVerify(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "login-session", svc.finishLoginReq.SessionID)
	assert.Equal(t, "test-agent", svc.finishLoginReq.UserAgent)
	assert.JSONEq(t, `{"status":"authenticated","access_token":"access","refresh_token":"refresh"}`, w.Body.String())
}

func TestPasskeyAuthenticatedHandlers(t *testing.T) {
	svc := &passkeyServiceStub{}
	school := &platformModel.School{
		Model:     base.Model{ID: 42},
		Subdomain: "school-a",
		Active:    true,
	}
	rs := &Resource{
		PasskeyService: svc,
		SchoolService: passkeySchoolServiceStub{
			byID: school,
		},
	}
	claims := jwt.AppClaims{ID: 11, TenantID: school.ID}

	w := httptest.NewRecorder()
	rs.passkeyEnrollmentChallenge(w, withPasskeyClaims(passkeyJSONRequest("/passkeys/enrollment/challenge", `{}`), claims))
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, claims.ID, svc.enrollmentAccountID)
	assert.Equal(t, claims.TenantID, svc.enrollmentTenantID)
	assert.JSONEq(t, `{"challenge_token":"challenge-token","masked_email":"m***@example.test"}`, w.Body.String())

	w = httptest.NewRecorder()
	req := withPasskeyClaims(passkeyJSONRequest("/passkeys/register/options", `{"code":"123456","name":" Laptop "}`), claims)
	req.Header.Set("Origin", "https://school-a.localhost")
	rs.passkeyRegisterOptions(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, claims.ID, svc.beginRegistrationReq.AccountID)
	assert.Equal(t, "Laptop", svc.beginRegistrationReq.Name)

	w = httptest.NewRecorder()
	rs.passkeyRegisterVerify(w, withPasskeyClaims(passkeyJSONRequest("/passkeys/register/verify", `{"session_id":"registration-session","name":"Phone","response":{"id":"credential"}}`), claims))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "registration-session", svc.finishRegistrationReq.SessionID)
	assert.JSONEq(t, `{"id":"7","name":"Laptop","created_at":"1970-01-01T00:00:01Z"}`, w.Body.String())

	w = httptest.NewRecorder()
	rs.passkeyList(w, withPasskeyClaims(httptest.NewRequest(http.MethodGet, "/passkeys", nil), claims))
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, claims.ID, svc.listAccountID)
	assert.JSONEq(t, `[{"id":"8","name":"Phone","created_at":"1970-01-01T00:00:02Z"}]`, w.Body.String())

	w = httptest.NewRecorder()
	passkeyID := school.ID + 1
	rs.passkeyRevoke(w, withPasskeyRouteParam(
		withPasskeyClaims(httptest.NewRequest(http.MethodDelete, "/passkeys/"+strconv.FormatInt(passkeyID, 10), nil), claims),
		"passkeyId",
		strconv.FormatInt(passkeyID, 10),
	))
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.EqualValues(t, claims.ID, svc.revokeAccountID)
	assert.Equal(t, passkeyID, svc.revokeCredentialID)
}

func TestPasskeyAuthenticatedHandlersRejectNonTenantAccountClaims(t *testing.T) {
	platformClaims := jwt.AppClaims{ID: 99, Scope: tenant.ScopePlatform}
	zeroTenantClaims := jwt.AppClaims{ID: 99}

	tests := []struct {
		name   string
		claims jwt.AppClaims
		call   func(*Resource, *httptest.ResponseRecorder, jwt.AppClaims)
		assert func(*testing.T, *passkeyServiceStub)
	}{
		{
			name:   "enrollment challenge rejects platform claims",
			claims: platformClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				req := passkeyJSONRequest("/passkeys/enrollment/challenge", `{}`)
				rs.passkeyEnrollmentChallenge(w, withPasskeyClaims(req, claims))
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.enrollmentAccountID)
				assert.Zero(t, svc.enrollmentTenantID)
			},
		},
		{
			name:   "register options rejects platform claims",
			claims: platformClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				req := passkeyJSONRequest("/passkeys/register/options", `{"code":"123456","name":"Laptop"}`)
				req.Header.Set("Origin", "https://school-a.localhost")
				rs.passkeyRegisterOptions(w, withPasskeyClaims(req, claims))
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.beginRegistrationReq.AccountID)
			},
		},
		{
			name:   "register verify rejects platform claims",
			claims: platformClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				req := passkeyJSONRequest("/passkeys/register/verify", `{"session_id":"registration-session","name":"Phone","response":{"id":"credential"}}`)
				rs.passkeyRegisterVerify(w, withPasskeyClaims(req, claims))
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.finishRegistrationReq.AccountID)
			},
		},
		{
			name:   "register verify rejects zero tenant claims",
			claims: zeroTenantClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				req := passkeyJSONRequest("/passkeys/register/verify", `{"session_id":"registration-session","name":"Phone","response":{"id":"credential"}}`)
				rs.passkeyRegisterVerify(w, withPasskeyClaims(req, claims))
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.finishRegistrationReq.AccountID)
			},
		},
		{
			name:   "list rejects platform claims",
			claims: platformClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				rs.passkeyList(w, withPasskeyClaims(httptest.NewRequest(http.MethodGet, "/passkeys", nil), claims))
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.listAccountID)
			},
		},
		{
			name:   "list rejects zero tenant claims",
			claims: zeroTenantClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				rs.passkeyList(w, withPasskeyClaims(httptest.NewRequest(http.MethodGet, "/passkeys", nil), claims))
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.listAccountID)
			},
		},
		{
			name:   "revoke rejects platform claims",
			claims: platformClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				req := withPasskeyRouteParam(
					withPasskeyClaims(httptest.NewRequest(http.MethodDelete, "/passkeys/8", nil), claims),
					"passkeyId",
					"8",
				)
				rs.passkeyRevoke(w, req)
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.revokeAccountID)
				assert.Zero(t, svc.revokeCredentialID)
			},
		},
		{
			name:   "revoke rejects zero tenant claims",
			claims: zeroTenantClaims,
			call: func(rs *Resource, w *httptest.ResponseRecorder, claims jwt.AppClaims) {
				req := withPasskeyRouteParam(
					withPasskeyClaims(httptest.NewRequest(http.MethodDelete, "/passkeys/8", nil), claims),
					"passkeyId",
					"8",
				)
				rs.passkeyRevoke(w, req)
			},
			assert: func(t *testing.T, svc *passkeyServiceStub) {
				assert.Zero(t, svc.revokeAccountID)
				assert.Zero(t, svc.revokeCredentialID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &passkeyServiceStub{}
			rs := &Resource{PasskeyService: svc}
			w := httptest.NewRecorder()

			tt.call(rs, w, tt.claims)

			require.Equal(t, http.StatusUnauthorized, w.Code)
			tt.assert(t, svc)
		})
	}
}

func TestPasskeyHandlerErrors(t *testing.T) {
	rs := &Resource{}
	w := httptest.NewRecorder()
	rs.passkeyLoginOptions(w, passkeyJSONRequest("/passkeys/login/options", `{"tenant_slug":"school-a"}`))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	rs.PasskeyService = &passkeyServiceStub{}
	w = httptest.NewRecorder()
	rs.passkeyLoginOptions(w, passkeyJSONRequest("/passkeys/login/options", `{"tenant_slug":"school-a"}`))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	mapPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), authService.ErrPasskeyNotFound)
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	mapPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), &authService.AuthError{Op: "issue tokens", Err: authService.ErrAccountInactive})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	mapPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), &authService.AuthError{Op: "issue tokens", Err: authService.ErrTenantAccessDenied})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	mapPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), &authService.AuthError{Op: "issue tokens", Err: authService.ErrTenantNotFound})
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = httptest.NewRecorder()
	// Passkey enrollment issues an email challenge, so it inherits the MFA
	// service's fail-closed status/rate-limit errors — those are transient.
	mapPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), authService.ErrMFAStatusUnavailable)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	w = httptest.NewRecorder()
	mapPasskeyError(w, httptest.NewRequest(http.MethodPost, "/", nil), errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func passkeyJSONRequest(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func withPasskeyClaims(req *http.Request, claims jwt.AppClaims) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
}

func withPasskeyRouteParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
