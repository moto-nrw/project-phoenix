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

// A refresh session is stored under its portal's name when the portal
// persists sessions; every other scope, also a future one, lands in the
// legacy bucket.
func TestPersistedPortalScopeFollowsThePortal(t *testing.T) {
	t.Parallel()
	for _, portal := range portals {
		want := PortalScopeUnknown
		if portal.Persisted {
			want = portal.Name
		}
		assert.Equal(t, want, PersistedPortalScope(portal.Claim), portal.Name)
	}
	assert.Equal(t, PortalTenant.Name, PersistedPortalScope(PortalTenant.LegacyClaim), "the legacy tenant claim")
	assert.Equal(t, PortalScopeUnknown, PersistedPortalScope("unexpected-new-scope"))
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

// A session holds subscriptions only in its portal's push portal; a legacy
// row may belong to any of them.
func TestPushPortalsOfFollowsThePortal(t *testing.T) {
	t.Parallel()
	for _, portal := range portals {
		if !portal.Persisted {
			continue
		}
		if portal.Push == "" {
			assert.Empty(t, PushPortalsOf(portal.Name), portal.Name)
			continue
		}
		assert.Equal(t, []string{portal.Push}, PushPortalsOf(portal.Name), portal.Name)
	}
	all := AllPushPortals()
	assert.ElementsMatch(t, []string{PushPortalStaff, PushPortalParent, PushPortalSchool}, all, "each push portal once")
	assert.Equal(t, all, PushPortalsOf(PortalScopeUnknown))
	assert.Equal(t, all, PushPortalsOf(""))
}

// The MFA rule of a stored session is its portal's; a legacy row follows the
// school policy.
func TestMFARuleOfFollowsThePortal(t *testing.T) {
	t.Parallel()
	for _, portal := range portals {
		if portal.Persisted {
			assert.Equal(t, portal.MFA, MFARuleOf(portal.Name), portal.Name)
		}
	}
	for _, legacy := range []string{PortalScopeUnknown, "", PortalPlatform.Name, "kiosk"} {
		assert.Equal(t, MFASchoolPolicy, MFARuleOf(legacy), legacy)
	}
}

// An account challenge starts only for a portal with a challenge whose
// sessions it mints; the operator keeps its own.
func TestAccountMFAChallengeScopeRefusesOtherPortals(t *testing.T) {
	t.Parallel()
	for _, portal := range portals {
		assert.Equal(t, portal.Challenge && portal.Persisted, AccountMFAChallengeScope(portal.Name), portal.Name)
	}
	assert.False(t, AccountMFAChallengeScope(PortalPlatform.Name))
	assert.False(t, AccountMFAChallengeScope(PortalParent.Name))
	assert.False(t, AccountMFAChallengeScope(""))
	assert.False(t, AccountMFAChallengeScope(PortalScopeUnknown))
}

// The three representations of a Träger school session (refresh claims,
// stored row, access token) resolve to the same kind; a staff token that
// names its school's Träger is none.
func TestTraegerSchoolIsRecognisedInEveryRepresentation(t *testing.T) {
	t.Parallel()
	refresh, ok := RefreshClaims{Scope: PortalTenant.Claim, TenantID: 10, OrgID: 7}.SessionKind()
	require.True(t, ok)
	access, ok := SessionClaims{Scope: PortalTenant.Claim, TenantID: 10, OrgID: 7, ViaTraeger: true}.SessionKind()
	require.True(t, ok)
	row := AccountSession{PortalScope: PortalTenant.Name, TenantID: 10, OrganizationID: 7}.SessionKind()
	assert.Equal(t, SessionKindTraegerSchool, refresh)
	assert.Equal(t, SessionKindTraegerSchool, access)
	assert.Equal(t, SessionKindTraegerSchool, row)

	staff, ok := SessionClaims{Scope: PortalTenant.Claim, TenantID: 10, OrgID: 7}.SessionKind()
	require.True(t, ok)
	assert.Equal(t, SessionKind(PortalTenant.Name), staff, "every staff token names its school's Träger")
	traeger, ok := RefreshClaims{Scope: PortalOrg.Claim, OrgID: 7}.SessionKind()
	require.True(t, ok)
	assert.Equal(t, SessionKind(PortalOrg.Name), traeger)
	_, ok = RefreshClaims{Scope: PortalPlatform.Claim}.SessionKind()
	assert.False(t, ok, "operator sessions follow their own rules")
	_, ok = RefreshClaims{Scope: "kiosk"}.SessionKind()
	assert.False(t, ok)
}

// Only the Träger-Portal keeps its sessions without a school; the sign-ins
// of the Träger-Portal and the caterer name none.
func TestSchoolFactsFollowTheTokenShape(t *testing.T) {
	t.Parallel()
	assert.False(t, AccountSession{PortalScope: PortalOrg.Name}.KeptAtSchool())
	for _, scope := range []string{PortalTenant.Name, PortalSchool.Name, PortalParent.Name, PortalCaterer.Name, PortalScopeUnknown} {
		assert.True(t, AccountSession{PortalScope: scope}.KeptAtSchool(), scope)
	}
	for _, portal := range []Portal{PortalOrg, PortalCaterer} {
		assert.True(t, portal.Schoolless(), portal.Name)
	}
	for _, portal := range []Portal{PortalTenant, PortalSchool} {
		assert.False(t, portal.Schoolless(), portal.Name)
	}
}
