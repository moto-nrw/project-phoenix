package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recoveryProofHeader is the HTTP contract of the refresh recovery proof
// (auth/rotation.RecoveryProofHeader) the frontend sends, spelled out so the
// route test does not import the rotation policy.
const recoveryProofHeader = "X-Refresh-Recovery-Proof"

// refreshRecoveryProofStub records what the refresh route hands the session
// capability. Any other method panics through the nil embedded interface.
type refreshRecoveryProofStub struct {
	AccountSessions

	gotRefreshToken  string
	gotRecoveryProof string
}

func (s *refreshRecoveryProofStub) RefreshTokenWithAudit(_ context.Context, refreshToken, recoveryProof, _, _ string) (string, string, error) {
	s.gotRefreshToken = refreshToken
	s.gotRecoveryProof = recoveryProof
	return "access", "refresh", nil
}

// The recovery proof is an input of the refresh capability: the route reads
// it from its header and passes it on. Without it, a refresh retried after a
// lost response is treated as a replay and revokes the session family.
func TestRefreshToken_PassesRecoveryProofHeaderToSessions(t *testing.T) {
	t.Parallel()

	sessions := &refreshRecoveryProofStub{}
	rs := &Resource{
		ConstraintViolation: transactionFailures.IsConstraintViolation, Sessions: sessions}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set(recoveryProofHeader, "independent-recovery-secret")
	req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxRefreshToken, "presented-refresh-token"))
	rr := httptest.NewRecorder()

	rs.refreshToken(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "presented-refresh-token", sessions.gotRefreshToken)
	assert.Equal(t, "independent-recovery-secret", sessions.gotRecoveryProof)
}

// The locked policy read can fail after a valid refresh token was accepted.
// This is a retryable dependency failure, not an expired session or a 500.
type unavailableMFARefreshStub struct{ AccountSessions }

func (*unavailableMFARefreshStub) RefreshTokenWithAudit(context.Context, string, string, string, string) (string, string, error) {
	return "", "", &identityaccess.AuthenticationError{Op: "refresh", Err: fmt.Errorf("policy lookup: %w", identityaccess.ErrMFAStatusUnavailable)}
}
func TestRefreshToken_UnavailableMFAPolicyReturns503(t *testing.T) {
	t.Parallel()
	rs := &Resource{Sessions: &unavailableMFARefreshStub{}}
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req = req.WithContext(context.WithValue(req.Context(), sessiontoken.CtxRefreshToken, "presented-refresh-token"))
	rr := httptest.NewRecorder()
	rs.refreshToken(rr, req)
	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
	assert.Equal(t, "identity.mfa_status_unavailable", problem.Code)
}
