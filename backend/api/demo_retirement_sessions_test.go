package api

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationtenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetiredDemoSchoolAcceptsNoSessionOfIt pins #6305: once the hourly demo
// run hid a demo school, no session of that school is accepted any more.
// Every request checks the family an access token names (#5194), not the
// school; a family the run leaves behind therefore opens the hidden school
// until its access token expires, and lets a switch start from it. The
// person the visitor added to the school is no visitor account, so the
// visitor erasure does not touch them; a session of the same person at
// another school stays.
func TestRetiredDemoSchoolAcceptsNoSessionOfIt(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	identity, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	demoID, _ := testpkg.CreateTestTenant(t, db)
	otherID, otherSlug := testpkg.CreateTestTenant(t, db)
	visitor := testpkg.CreateTestAccount(t, db, "demo-retire-visitor")
	testpkg.MapAccountToTenant(t, db, visitor.ID, demoID)
	colleague := testpkg.CreateTestAccount(t, db, "demo-retire-colleague")
	testpkg.MapAccountToTenant(t, db, colleague.ID, demoID)
	testpkg.MapAccountToTenant(t, db, colleague.ID, otherID)

	type session struct{ access, refresh string }
	issue := func(accountID, schoolID int64) session {
		access, refresh, issueErr := identity.IssueTokensForPasskey(ctx, accountID, schoolID, "", "")
		require.NoError(t, issueErr)
		return session{access: access, refresh: refresh}
	}
	visitorAtDemo := issue(visitor.ID, demoID)
	colleagueAtDemo := issue(colleague.ID, demoID)
	colleagueElsewhere := issue(colleague.ID, otherID)
	accessOf := func(accountID int64, s session) identityaccess.AccessSession {
		return identityaccess.AccessSession{AccountID: accountID, FamilyID: accessTokenFamily(t, s.access)}
	}
	require.NoError(t, identity.VerifyAccessSession(ctx, accessOf(colleague.ID, colleagueAtDemo)))

	// The visitor's demo access expired: no access names the order, so the
	// run hides its school.
	queue, err := organizationtenancyCompose.NewDemoSchoolQueue(db)
	require.NoError(t, err)
	schools, err := organizationtenancyCompose.NewDemoSchools(db)
	require.NoError(t, err)
	var slug string
	require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(ctx context.Context) error {
		var orderErr error
		slug, orderErr = organizationtenancyCompose.NewDemoSchoolOrders(strings.NewReader("k3m9xp"), 300).
			OrderDemoSchool(ctx, "OGS Nord", "Anna Lena", "von Berg")
		return orderErr
	}))
	_, err = queue.ClaimDemoSchoolOrder(ctx)
	require.NoError(t, err)
	require.NoError(t, schools.RememberDemoSchool(ctx, slug, organizationtenancy.DemoSchoolState{
		SchoolID: demoID, SeedJSON: []byte(`{}`), VisitorAccountID: visitor.ID,
	}))
	require.NoError(t, queue.FinishDemoSchoolOrder(ctx, slug, visitor.ID, 0))

	// The retirement starts no school after its budget (200 ms), the first
	// one included: a run that met a stalled database hides nothing and
	// leaves the school to the next run (#6698).
	hidden := 0
	for run := 0; run < 10 && hidden == 0; run++ {
		hidden, _, err = queue.RetireDemoSchools(ctx, nil)
		require.NoError(t, err)
	}
	require.Equal(t, 1, hidden)

	for name, s := range map[string]struct {
		accountID int64
		session   session
	}{
		"visitor":   {visitor.ID, visitorAtDemo},
		"colleague": {colleague.ID, colleagueAtDemo},
	} {
		assert.ErrorIs(t, identity.VerifyAccessSession(ctx, accessOf(s.accountID, s.session)), identityaccess.ErrInvalidToken,
			"%s: the access token of the hidden school opens nothing", name)
		_, _, refreshErr := identity.RefreshTokenWithAudit(ctx, s.session.refresh, "", "", "")
		assert.Error(t, refreshErr, "%s: the refresh token of the hidden school renews nothing", name)
	}
	_, _, err = identity.SwitchTenant(ctx, colleague.ID, otherSlug, accessOf(colleague.ID, colleagueAtDemo).FamilyID, "", "")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "no switch starts from a session of the hidden school")

	require.NoError(t, identity.VerifyAccessSession(ctx, accessOf(colleague.ID, colleagueElsewhere)),
		"the colleague's session at another school stays")
	_, _, err = identity.RefreshTokenWithAudit(ctx, colleagueElsewhere.refresh, "", "", "")
	require.NoError(t, err)
}
