package account

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
