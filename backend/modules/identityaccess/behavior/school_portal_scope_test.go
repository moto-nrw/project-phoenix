package behavior_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
)

// scopeRecordingMFA records the challenge scope every school-portal call
// carries. It embeds the capability so the methods the binding does not use
// stay unimplemented rather than silently answering.
type scopeRecordingMFA struct {
	identityaccess.AccountMFA
	scopes []string
}

func (m *scopeRecordingMFA) StartMFAChallenge(_ context.Context, _, _ int64, scope string, _ net.IP) (string, error) {
	m.scopes = append(m.scopes, scope)
	return "challenge", nil
}

func (m *scopeRecordingMFA) VerifyMFAChallengeForScope(_ context.Context, _, _, expectedScope string) (identityaccess.VerifiedMFAChallenge, error) {
	m.scopes = append(m.scopes, expectedScope)
	return identityaccess.VerifiedMFAChallenge{}, nil
}

func (m *scopeRecordingMFA) VerifyMFAChallengeForOwner(_ context.Context, _, _, expectedScope string, _, _ int64) (identityaccess.VerifiedMFAChallenge, error) {
	m.scopes = append(m.scopes, expectedScope)
	return identityaccess.VerifiedMFAChallenge{}, nil
}

func (m *scopeRecordingMFA) ResendMFAChallengeForScope(_ context.Context, _ string, _ net.IP, expectedScope string) (string, error) {
	m.scopes = append(m.scopes, expectedScope)
	return "challenge", nil
}

// The school portal's second factor is pinned to the school challenge scope
// by the composition, not by the handlers: a challenge minted for another
// portal must not verify here. The scope moved into this binding with #3364,
// so it is pinned here instead of at the handler.
func TestSchoolPortalMFAOverAppliesTheSchoolScope(t *testing.T) {
	t.Parallel()

	capability := &scopeRecordingMFA{}
	runtime := services.SchoolPortalMFAOver(capability)
	ctx := context.Background()
	ip := net.ParseIP("203.0.113.7")

	_, err := runtime.StartChallenge(ctx, 42, 7, ip)
	require.NoError(t, err)
	_, err = runtime.VerifyChallenge(ctx, "token", "515151")
	require.NoError(t, err)
	_, err = runtime.VerifyChallengeForOwner(ctx, "token", "515151", 42, 7)
	require.NoError(t, err)
	_, err = runtime.ResendChallenge(ctx, "token", ip)
	require.NoError(t, err)

	assert.Equal(t, []string{
		identityaccess.MFAChallengeScopeSchool,
		identityaccess.MFAChallengeScopeSchool,
		identityaccess.MFAChallengeScopeSchool,
		identityaccess.MFAChallengeScopeSchool,
	}, capability.scopes, "every school-portal challenge call carries the school scope")
}

// A composition without the second factor hands out the zero runtime, which
// the portal answers 503 from; it must not build closures over a nil
// capability.
func TestSchoolPortalMFAOverWithoutCapability(t *testing.T) {
	t.Parallel()

	assert.Nil(t, services.SchoolPortalMFAOver(nil).VerifyChallenge)
}
