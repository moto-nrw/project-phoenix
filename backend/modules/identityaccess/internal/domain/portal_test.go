package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

// The middleware and the token codec read the portals from tokenprimitives,
// the session flows from this table; both must name the same portals with the
// same claims, token shapes and MFA token scopes.
func TestPortalTableMatchesTheTokenGates(t *testing.T) {
	t.Parallel()
	shapes := map[Shape]tokenprimitives.Shape{
		ShapeIgnored: tokenprimitives.Ignored, ShapeOptional: tokenprimitives.Optional,
		ShapeRequired: tokenprimitives.Required, ShapeForbidden: tokenprimitives.Forbidden,
	}
	for _, portal := range portals {
		gate, ok := tokenprimitives.PortalOfClaim(portal.Claim)
		require.True(t, ok, portal.Name)
		assert.Equal(t, tokenprimitives.PortalClaims{
			Name: portal.Name, Claim: portal.Claim, LegacyClaim: portal.LegacyClaim,
			Tenant: shapes[portal.Tenant], Org: shapes[portal.Org],
			Challenge: portal.Challenge, Enrollment: portal.Enrollment,
		}, gate, portal.Name)
	}
}

// The public challenge scopes are the portals with a challenge of their own.
func TestPublicChallengeScopesNameChallengePortals(t *testing.T) {
	t.Parallel()
	public := []string{
		identityaccess.MFAChallengeScopeTenant, identityaccess.MFAChallengeScopeSchool, identityaccess.MFAChallengeScopeOrg,
		identityaccess.MFAChallengeScopeCaterer, identityaccess.MFAChallengeScopePlatform,
	}
	var challengePortals []string
	for _, portal := range portals {
		if portal.Challenge {
			challengePortals = append(challengePortals, portal.Name)
		}
	}
	assert.ElementsMatch(t, challengePortals, public)
}

func TestPortalOfClaimResolvesTheLegacyTenantClaim(t *testing.T) {
	t.Parallel()
	for scope, want := range map[string]string{"": "tenant", "tenant": "tenant", "school": "school", "parent": "parent", "caterer": "caterer", "org": "org", "platform": "platform"} {
		portal, ok := PortalOfClaim(scope)
		require.True(t, ok, scope)
		assert.Equal(t, want, portal.Name, scope)
	}
	for _, scope := range []string{"unknown", "kiosk", "Tenant"} {
		_, ok := PortalOfClaim(scope)
		assert.False(t, ok, scope)
	}
}

func TestPersistedPortalScope(t *testing.T) {
	t.Parallel()
	for scope, want := range map[string]string{
		"": "tenant", "tenant": "tenant", "school": "school", "parent": "parent", "caterer": "caterer", "org": "org",
		"platform": PortalScopeUnknown, "unexpected-new-scope": PortalScopeUnknown,
	} {
		assert.Equal(t, want, PersistedPortalScope(scope), scope)
	}
}

func TestSessionScopeMatchesTheNamedPortalOnly(t *testing.T) {
	t.Parallel()
	assert.True(t, SessionScopeMatches("", "tenant"))
	assert.True(t, SessionScopeMatches("tenant", "tenant"))
	assert.True(t, SessionScopeMatches("platform", "platform"))
	assert.True(t, SessionScopeMatches("org", "org"))
	assert.False(t, SessionScopeMatches("", "school"))
	assert.False(t, SessionScopeMatches("school", "tenant"))
	assert.False(t, SessionScopeMatches("caterer", "parent"))
	assert.False(t, SessionScopeMatches("unknown", "unknown"))
	assert.False(t, SessionScopeMatches("", ""))
}

func TestPortalShapes(t *testing.T) {
	t.Parallel()
	assert.True(t, PortalOrg.Holds(0, 7))
	assert.False(t, PortalOrg.Holds(0, 0), "a Träger session names its Träger")
	assert.False(t, PortalOrg.Holds(100, 7), "a Träger session names no school (ADR 0084)")
	assert.True(t, PortalTenant.Holds(100, 7), "an Absprung session names both")
	assert.False(t, PortalTenant.Holds(0, 0))
	assert.False(t, PortalSchool.Holds(0, 0))
	assert.True(t, PortalParent.Holds(0, 0))
	assert.True(t, PortalPlatform.Holds(0, 0))
}

func TestPushPortalsOf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{PushPortalParent}, PushPortalsOf("parent"))
	assert.Equal(t, []string{PushPortalSchool}, PushPortalsOf("school"))
	assert.Equal(t, []string{PushPortalStaff}, PushPortalsOf("tenant"))
	assert.Equal(t, []string{PushPortalStaff}, PushPortalsOf("org"))
	assert.Empty(t, PushPortalsOf("caterer"), "the caterer portal has no push messages")
	assert.ElementsMatch(t, []string{PushPortalStaff, PushPortalParent, PushPortalSchool}, PushPortalsOf(PortalScopeUnknown))
	assert.ElementsMatch(t, []string{PushPortalStaff, PushPortalParent, PushPortalSchool}, PushPortalsOf(""))
}

func TestMFARuleOf(t *testing.T) {
	t.Parallel()
	for scope, want := range map[string]MFARule{
		"tenant": MFASchoolPolicy, "school": MFASchoolPolicy, PortalScopeUnknown: MFASchoolPolicy, "": MFASchoolPolicy,
		"org": MFAAlways, "caterer": MFAAccountChoice, "parent": MFANone,
	} {
		assert.Equal(t, want, MFARuleOf(scope), scope)
	}
}

func TestAccountMFAChallengeScope(t *testing.T) {
	t.Parallel()
	for scope, want := range map[string]bool{"tenant": true, "school": true, "org": true, "caterer": true, "platform": false, "parent": false, "": false} {
		assert.Equal(t, want, AccountMFAChallengeScope(scope), scope)
	}
}
