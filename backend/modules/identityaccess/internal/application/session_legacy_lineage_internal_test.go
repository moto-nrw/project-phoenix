package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCapLegacyLineage pins the transition of #5422: every family without a
// lineage origin keeps its current expiry as its deadline, whatever its
// portal; a family that names an origin renews as before.
func TestCapLegacyLineage(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{domain.PortalScopeTenant, domain.PortalScopeSchool, domain.PortalScopeParent, domain.PortalScopeCaterer, domain.PortalScopeOrg, domain.PortalScopeUnknown} {
		expiry := time.Now().Add(time.Hour)
		session := domain.AccountSession{AccountID: 1, FamilyID: "family", PortalScope: scope, Expiry: expiry}

		require.True(t, capLegacyLineage(&session), scope)
		require.NotNil(t, session.FamilyExpiryCap, scope)
		assert.True(t, session.FamilyExpiryCap.Equal(expiry), "%s: the family keeps its current expiry as its deadline", scope)
		require.NotNil(t, session.LineageDeadline, scope)
		assert.True(t, session.LineageDeadline.Equal(expiry), "%s: the lineage carries the deadline to later switches", scope)
		assert.Empty(t, session.SwitchedFromFamilyID, "%s: a legacy family stays unlinked", scope)
	}
	for _, origin := range []string{"family", "origin"} {
		session := domain.AccountSession{AccountID: 1, FamilyID: "family", SwitchedFromFamilyID: origin, PortalScope: domain.PortalScopeTenant, Expiry: time.Now().Add(time.Hour)}
		assert.False(t, capLegacyLineage(&session), origin)
		assert.Nil(t, session.FamilyExpiryCap, "a family the new code issued renews as before")
		assert.Nil(t, session.LineageDeadline)
	}
}

// TestCapLegacyLineage_KeepsAnEarlierDeadline leaves a retired family's
// shorter deadline in place.
func TestCapLegacyLineage_KeepsAnEarlierDeadline(t *testing.T) {
	t.Parallel()
	retired := time.Now().Add(time.Minute)
	session := domain.AccountSession{AccountID: 1, FamilyID: "family", PortalScope: domain.PortalScopeTenant, Expiry: retired.Add(time.Hour), FamilyExpiryCap: &retired}

	require.True(t, capLegacyLineage(&session))

	assert.True(t, session.FamilyExpiryCap.Equal(retired))
	assert.True(t, session.LineageDeadline.Equal(retired))
}

// TestLegacyDeadline pins what a switch hands down: the lineage deadline of
// a linked family, the cap or expiry of a legacy one, and nothing for a new
// family whose cap is only its retirement grace.
func TestLegacyDeadline(t *testing.T) {
	t.Parallel()
	expiry := time.Now().Add(time.Hour)
	grace := time.Now().Add(5 * time.Minute)
	deadline := time.Now().Add(30 * time.Minute)
	assert.Nil(t, legacyDeadline(domain.AccountSession{SwitchedFromFamilyID: "own", Expiry: expiry, FamilyExpiryCap: &grace}), "a retirement grace is not handed down")
	got := legacyDeadline(domain.AccountSession{SwitchedFromFamilyID: "origin", Expiry: expiry, FamilyExpiryCap: &grace, LineageDeadline: &deadline})
	require.NotNil(t, got)
	assert.True(t, got.Equal(deadline), "a linked family hands down its lineage deadline")
	got = legacyDeadline(domain.AccountSession{Expiry: expiry, FamilyExpiryCap: &grace})
	require.NotNil(t, got)
	assert.True(t, got.Equal(grace), "a legacy family hands down its earlier cap")
	got = legacyDeadline(domain.AccountSession{Expiry: expiry})
	require.NotNil(t, got)
	assert.True(t, got.Equal(expiry), "a legacy family without cap hands down its expiry")
}
