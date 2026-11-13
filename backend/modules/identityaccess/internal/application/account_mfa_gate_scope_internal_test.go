package application

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

type challengeScopeRecorder struct {
	identityaccess.AccountMFA
	scopes []string
}

func (r *challengeScopeRecorder) StartMFAChallenge(_ context.Context, _, _ int64, scope string, _ net.IP, _ int64) (string, error) {
	r.scopes = append(r.scopes, scope)
	return "challenge", nil
}

// The login names the portal; the gate starts that portal's challenge and
// refuses a portal without one instead of falling back to the tenant scope.
func TestCapabilityGateStartsTheChallengeOfTheNamedPortal(t *testing.T) {
	t.Parallel()

	for _, scope := range []string{"tenant", "school", "org", "caterer"} {
		recorder := &challengeScopeRecorder{}
		_, err := CapabilityGate{Capability: recorder}.StartChallenge(context.Background(), 1, 2, scope, "127.0.0.1", 0)
		require.NoError(t, err, scope)
		assert.Equal(t, []string{scope}, recorder.scopes, scope)
	}
	for _, scope := range []string{"parent", "", "unknown", "nonsense"} {
		recorder := &challengeScopeRecorder{}
		_, err := CapabilityGate{Capability: recorder}.StartChallenge(context.Background(), 1, 2, scope, "127.0.0.1", 0)
		require.ErrorIs(t, err, identityaccess.ErrMFAUnsupportedScope, scope)
		assert.Empty(t, recorder.scopes, scope)
	}
}
