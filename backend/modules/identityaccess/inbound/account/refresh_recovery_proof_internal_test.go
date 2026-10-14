package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"

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
	rs := &Resource{Sessions: sessions}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set(recoveryProofHeader, "independent-recovery-secret")
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxRefreshToken, "presented-refresh-token"))
	rr := httptest.NewRecorder()

	rs.refreshToken(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "presented-refresh-token", sessions.gotRefreshToken)
	assert.Equal(t, "independent-recovery-secret", sessions.gotRecoveryProof)
}
