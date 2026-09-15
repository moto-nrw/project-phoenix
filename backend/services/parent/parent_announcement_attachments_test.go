package parent_test

// Der Empfängerkreis-Test der Mitteilungs-Anhänge (#2890).
//
// Die Datei-Seite (services/filestore) entscheidet nichts darüber, wer eine
// Datei sehen darf; sie fragt hier nach und macht aus "0" ein 404. Genau
// deshalb liegt der Test hier: was dieses Konto zurückbekommt, IST die
// Zugriffskontrolle des Downloads.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Parallel is safe here, unlike for the feed tests above: every assertion
// names an id this test created, so an announcement another test publishes
// into the shared clone cannot change the answer. The feed tests count what a
// guardian sees in total, which is why they run serially.
func TestGuardianAnnouncementTenant_OnlyForTheAudience(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	// A guardian the announcement reaches gets the announcement's school back,
	// which is the tenant the attachment rows are then read under.
	tenantID, err := svc.GuardianAnnouncementTenant(ctx, chain.AccountID, ann.ID)
	require.NoError(t, err)
	assert.Equal(t, chain.TenantID, tenantID)

	// An account with no guardian link to any addressed child gets 0 — not an
	// error. The file side turns that into the same 404 it gives for an
	// announcement that does not exist, so ids cannot be probed.
	stranger := testpkg.CreateTestAccount(t, db, "fremde.person@example.test")
	tenantID, err = svc.GuardianAnnouncementTenant(ctx, stranger.ID, ann.ID)
	require.NoError(t, err)
	assert.Zero(t, tenantID, "an account outside the audience must not resolve a school")

	// An announcement that does not exist is indistinguishable from one the
	// caller may not see.
	tenantID, err = svc.GuardianAnnouncementTenant(ctx, chain.AccountID, ann.ID+9_000_000)
	require.NoError(t, err)
	assert.Zero(t, tenantID)
}

// Parallel is safe, see above.
func TestGuardianAnnouncementTenant_DraftAndDisabledFeatureHideTheFile(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	// A draft is not live, so its attachments are not reachable either — an
	// attachment must never become the back door to an unpublished message.
	draft := &usersModels.ParentAnnouncement{
		Title:     "Noch nicht raus",
		Body:      "Entwurf.",
		Priority:  usersModels.ParentAnnouncementPriorityInfo,
		Active:    true,
		CreatedBy: chain.AccountID,
	}
	draft.SetTenantID(chain.TenantID)
	require.NoError(t, repos.ParentAnnouncement.Create(seedCtx, draft))
	require.NoError(t, repos.ParentAnnouncement.ReplaceTargets(seedCtx, chain.TenantID, draft.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}))
	t.Cleanup(func() { _ = repos.ParentAnnouncement.Delete(seedCtx, draft.ID) })

	tenantID, err := svc.GuardianAnnouncementTenant(ctx, chain.AccountID, draft.ID)
	require.NoError(t, err)
	assert.Zero(t, tenantID, "a draft's attachments are not reachable")

	// An announcement whose publication moment is still ahead is not live
	// either, so its attachments stay out of reach until it actually goes out.
	scheduled := seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)
	future := time.Now().Add(time.Hour)
	require.NoError(t, repos.ParentAnnouncement.SetPublished(seedCtx, scheduled.ID, &future))

	tenantID, err = svc.GuardianAnnouncementTenant(ctx, chain.AccountID, scheduled.ID)
	require.NoError(t, err)
	assert.Zero(t, tenantID, "an announcement not yet live has no reachable attachments")
}

// Parallel is safe, see above.
func TestGuardianAnnouncementTenant_FeatureOffHidesTheFile(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	// Same setup, but the school has the parent-news feature switched off. The
	// check fails closed: no feature, no file.
	svc, db, repos := buildAnnouncementService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	tenantID, err := svc.GuardianAnnouncementTenant(ctx, chain.AccountID, ann.ID)
	require.NoError(t, err)
	assert.Zero(t, tenantID)
}
