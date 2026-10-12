package operator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refusingSessions refuses every token issue with err; the other operator
// capabilities are not reached by the exchange.
type refusingSessions struct {
	OperatorAccess
	err error
}

func (s refusingSessions) IssueTokensForAuthenticatedOperator(context.Context, int64, string, string) (string, string, error) {
	return "", "", s.err
}

// The token exchange after a correct second factor names each refusal by
// its identity code (#2520): a vanished operator is a refused credential
// like in the tenant exchange, not an expired session.
func TestOperatorMFAExchangeRefusalsCarryIdentityCodes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"operator not found", ErrOperatorNotFound, http.StatusUnauthorized, "identity.invalid_credentials"},
		{"operator inactive", ErrOperatorInactive, http.StatusForbidden, "identity.account_inactive"},
		{"issue failure", errors.New("boom"), http.StatusInternalServerError, "general.server"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := &MFAResource{authService: refusingSessions{err: tc.err}}
			recorder := httptest.NewRecorder()
			rs.completeMFAExchange(recorder, httptest.NewRequest(http.MethodPost, "/operator/auth/mfa/verify", nil), 7, false)

			var body struct {
				Status int    `json:"status"`
				Code   string `json:"code"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
			assert.Equal(t, tc.status, recorder.Code)
			assert.Equal(t, tc.status, body.Status)
			assert.Equal(t, tc.code, body.Code)
		})
	}
}
