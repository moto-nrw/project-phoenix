package api

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configuredSessionTestSecret = "configured-session-auth-test-secret-only"

// The HTTP session runtime the server root mounts and the codec Identity's
// flows sign with come from the same configuration values, so a session one
// of them issues is accepted by the other (ADR 0053 point 6, ADR 0056).
func TestConfiguredSessionAuthVerifiesTheConfiguredCodecsSessions(t *testing.T) {
	t.Parallel()

	sessionAuth, err := NewConfiguredSessionAuth(configuredSessionTestSecret, 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	tokens, err := NewConfiguredSessionTokens(configuredSessionTestSecret, 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)

	accessToken, err := tokens.IssueAccessToken(identityaccess.SessionClaims{
		AccountID: 42, Email: "staff@example.test", Username: "staff", Roles: []string{"staff"}, TenantID: 7, Scope: "tenant",
	})
	require.NoError(t, err)
	claims, err := sessionAuth.ParseAccessJWT(accessToken)
	require.NoError(t, err)
	assert.Equal(t, 42, claims.ID)
	assert.Equal(t, 15*time.Minute, sessionAuth.JwtExpiry)
	assert.Equal(t, 24*time.Hour, sessionAuth.JwtRefreshExpiry)

	other, err := NewConfiguredSessionAuth("a-different-configured-session-secret", 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	_, err = other.ParseAccessJWT(accessToken)
	assert.Error(t, err, "a session signed under another secret must not verify")
}

func TestConfiguredSessionAuthRejectsTheGeneratedSecretMode(t *testing.T) {
	t.Parallel()

	_, err := NewConfiguredSessionAuth("random", 15*time.Minute, 24*time.Hour)
	require.ErrorContains(t, err, "invalid auth JWT configuration")
}
