package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/require"
)

// sessionValidationStub answers the validation capability with a fixed
// error. Any other method panics through the nil embedded interface.
type sessionValidationStub struct {
	AccountSessions

	err error
}

func (s *sessionValidationStub) ValidateSessionTokens(context.Context, string, string, string) (identityaccess.SessionClaims, error) {
	return identityaccess.SessionClaims{}, s.err
}

func (s *sessionValidationStub) RefreshTokenWithAudit(context.Context, string, string, string, string) (string, string, error) {
	return "", "", s.err
}

// #5399: the BFF reads the answer as the session family's state. A refused
// token ends the session (401); a family that could not be read (503) must
// not, or a database hiccup would sign everyone out.
func TestValidateSession_SeparatesRefusalFromUnavailable(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"revoked family", identityaccess.ErrInvalidToken, http.StatusUnauthorized},
		{"family not readable", errors.New("connection refused"), http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := &Resource{Sessions: &sessionValidationStub{err: tc.err}}
			req := httptest.NewRequest(http.MethodPost, "/auth/session/validate",
				strings.NewReader(`{"access_token":"a","portal":"tenant"}`))
			rr := httptest.NewRecorder()

			rs.validateSession(rr, req)

			require.Equal(t, tc.status, rr.Code, rr.Body.String())
		})
	}
}

// Validation answers a refused session as the refresh does (one
// classification, identityaccess.SessionOutcomeOf): an unbooked module stays
// 403 module.not_booked, so the BFF keeps the session and explains the
// module instead of signing the user out. Only a read that could not run
// differs: validation 503, refresh 500.
func TestValidateSession_AnswersRefusalsAsTheRefresh(t *testing.T) {
	t.Parallel()

	notBooked := &identityaccess.PortalNotBookedError{Module: "eltern_app", ModuleLabel: "Eltern-App"}
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"module not booked", &identityaccess.AuthenticationError{Op: "validate session", Err: notBooked}, http.StatusForbidden, "module.not_booked"},
		{"inactive school", &identityaccess.AuthenticationError{Op: "validate tenant access", Err: identityaccess.ErrTenantNotFound}, http.StatusUnauthorized, "general.permission"},
		{"revoked role", &identityaccess.AuthenticationError{Op: "validate session", Err: identityaccess.ErrTenantAccessDenied}, http.StatusUnauthorized, "general.permission"},
		{"caterer left", identityaccess.ErrAccountNotCaterer, http.StatusUnauthorized, "general.permission"},
		{"second factor unreadable", identityaccess.ErrMFAStatusUnavailable, http.StatusServiceUnavailable, "identity.mfa_status_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := &Resource{Sessions: &sessionValidationStub{err: tc.err}}
			validated := httptest.NewRecorder()
			rs.validateSession(validated, httptest.NewRequest(http.MethodPost, "/auth/session/validate",
				strings.NewReader(`{"access_token":"a","portal":"parent"}`)))
			refreshed := httptest.NewRecorder()
			rs.refreshToken(refreshed, httptest.NewRequest(http.MethodPost, "/auth/refresh", nil))

			for route, rr := range map[string]*httptest.ResponseRecorder{"validate": validated, "refresh": refreshed} {
				require.Equal(t, tc.status, rr.Code, "%s: %s", route, rr.Body.String())
				var body struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), route)
				require.Equal(t, tc.code, body.Code, route)
			}
		})
	}
}
