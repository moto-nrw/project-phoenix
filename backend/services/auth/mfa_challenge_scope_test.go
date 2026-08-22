// Scope enforcement on the MFA challenge resend (#2207). A challenge is
// bound to the portal it was started for: the school resend endpoint must
// not let a tenant-portal challenge be re-driven — and its 3-codes/15-min
// budget burned — through the school surface, and vice versa.
package auth_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services/auth"
)

// rateLimitWindowStart is the lower bound for the code-issuance counter the
// resend path shares with StartChallenge — wide enough to catch every code
// this test could have produced.
func rateLimitWindowStart() time.Time { return time.Now().Add(-time.Hour) }

func TestMFAService_ResendChallengeForScope_InvalidToken_ReturnsTokenInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newExtraMFAService(t)

	renewed, err := svc.ResendChallengeForScope(
		context.Background(), "not-a-jwt", net.ParseIP("127.0.0.1"), authjwt.MFAChallengeScopeSchool)

	require.Error(t, err)
	assert.Empty(t, renewed, "invalid token must not produce a renewed JWT")
	assert.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid)
}

func TestMFAService_ResendChallengeForScope_ForeignScope_RefusedBeforeSending(t *testing.T) {
	t.Parallel()

	svc, repos, accID := newExtraMFAService(t)
	ctx := context.Background()
	require.NoError(t, svc.Enroll(ctx, accID))

	// A challenge started at the TENANT login…
	token, err := svc.StartChallenge(ctx, accID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, token)

	before, err := repos.MFAEmailChallenge.CountRecentByAccountID(ctx, accID, rateLimitWindowStart())
	require.NoError(t, err)

	// …must not be re-drivable through the school resend endpoint.
	renewed, err := svc.ResendChallengeForScope(ctx, token, net.ParseIP("127.0.0.1"), authjwt.MFAChallengeScopeSchool)

	require.Error(t, err)
	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, auth.ErrMFAUnsupportedScope)

	after, err := repos.MFAEmailChallenge.CountRecentByAccountID(ctx, accID, rateLimitWindowStart())
	require.NoError(t, err)
	assert.Equal(t, before, after, "a refused scope must not consume a code from the rate-limit budget")
}

func TestMFAService_ResendChallengeForScope_SchoolScope_HappyPath(t *testing.T) {
	t.Parallel()

	svc, _, accID := newExtraMFAService(t)
	ctx := context.Background()
	require.NoError(t, svc.Enroll(ctx, accID))

	token, err := svc.StartChallenge(ctx, accID, 0, authjwt.MFAChallengeScopeSchool, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, token)

	renewed, err := svc.ResendChallengeForScope(ctx, token, net.ParseIP("127.0.0.1"), authjwt.MFAChallengeScopeSchool)

	require.NoError(t, err)
	assert.NotEmpty(t, renewed, "a matching scope must return a usable renewed challenge JWT")
}
