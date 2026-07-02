package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// publishedAnnouncement creates an announcement with the given targets, publishes
// it, and registers cleanup. created_by is the chain account (a real auth.account).
func publishedAnnouncement(
	t *testing.T,
	ctx context.Context,
	repo usersModels.ParentAnnouncementRepository,
	createdBy, tenantID int64,
	title string,
	targets []*usersModels.ParentAnnouncementTarget,
) *usersModels.ParentAnnouncement {
	t.Helper()
	a := &usersModels.ParentAnnouncement{
		Title:     title,
		Body:      "Testtext",
		Priority:  usersModels.ParentAnnouncementPriorityInfo,
		Active:    true,
		CreatedBy: createdBy,
	}
	a.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID, targets))
	now := time.Now()
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	t.Cleanup(func() { _ = repo.Delete(ctx, a.ID) })
	return a
}

// TestParentAnnouncementAudience exercises the audience resolver against a real
// guardian->student link: school-wide and class targets reach the linked
// guardian; a non-matching class does not; drafts stay out of the feed; and
// read state flows through the feed + unread count.
func TestParentAnnouncementAudience(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx() // tenant 1
	tenantIDs := []int64{chain.TenantID}

	schoolWide := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Schulweit", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})

	classMatch := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Klasse 1a", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetClass, TargetRefText: strp("1a")},
		})

	classMiss := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Klasse 9z", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetClass, TargetRefText: strp("9z")},
		})

	// A draft (never published) must never reach the feed.
	draft := &usersModels.ParentAnnouncement{
		Title: "Entwurf", Body: "x", Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	}
	draft.SetTenantID(chain.TenantID)
	require.NoError(t, repo.Create(ctx, draft))
	require.NoError(t, repo.ReplaceTargets(ctx, chain.TenantID, draft.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}))
	t.Cleanup(func() { _ = repo.Delete(ctx, draft.ID) })

	// --- AccountMatchesAnnouncement ---
	matched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, schoolWide.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "school-wide should reach the linked guardian")

	matched, err = repo.AccountMatchesAnnouncement(ctx, chain.TenantID, classMatch.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "class 1a should reach a 1a student's guardian")

	matched, err = repo.AccountMatchesAnnouncement(ctx, chain.TenantID, classMiss.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "class 9z must NOT reach a 1a student's guardian")

	// --- CountAudience ---
	count, err := repo.CountAudience(ctx, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "school-wide audience includes the linked guardian")

	missCount, err := repo.CountAudience(ctx, chain.TenantID, classMiss.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, missCount, "class 9z reaches nobody in this fixture")

	// --- Feed: published reachable announcements, drafts excluded ---
	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	ids := map[int64]bool{}
	for _, item := range feed {
		ids[item.ID] = true
	}
	assert.True(t, ids[schoolWide.ID], "feed includes school-wide")
	assert.True(t, ids[classMatch.ID], "feed includes class 1a")
	assert.False(t, ids[classMiss.ID], "feed excludes non-matching class")
	assert.False(t, ids[draft.ID], "feed excludes drafts")

	// --- Unread count drops after a read ---
	unreadBefore, err := repo.CountUnreadForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	require.GreaterOrEqual(t, unreadBefore, 2)

	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, schoolWide.ID, chain.AccountID))
	unreadAfter, err := repo.CountUnreadForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	assert.Equal(t, unreadBefore-1, unreadAfter, "reading one announcement drops unread by one")

	// --- Acknowledge stamps the read row ---
	require.NoError(t, repo.MarkAcknowledged(ctx, chain.TenantID, classMatch.ID, chain.AccountID))
	stats, err := repo.Stats(ctx, chain.TenantID, classMatch.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TargetCount, 1)
	assert.GreaterOrEqual(t, stats.ReadCount, 1)
	assert.GreaterOrEqual(t, stats.AcknowledgedCount, 1)
}

// TestParentAnnouncementAudienceRecipients exercises the per-person recipient
// list: the linked guardian appears with pending state, transitions to
// read/acknowledged as the read row is stamped, and a non-matching target
// yields no recipients.
func TestParentAnnouncementAudienceRecipients(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()

	schoolWide := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Schulweit Empfänger", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})
	classMiss := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Klasse 9z Empfänger", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetClass, TargetRefText: strp("9z")},
		})

	findRecipient := func(list []*usersModels.AnnouncementRecipientStatus) *usersModels.AnnouncementRecipientStatus {
		for _, r := range list {
			if r.AccountID == chain.AccountID {
				return r
			}
		}
		return nil
	}

	// Pending before any read.
	recipients, err := repo.AudienceRecipients(ctx, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt := findRecipient(recipients)
	require.NotNil(t, rcpt, "school-wide recipients include the linked guardian account")
	assert.Nil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	// Read stamps read_at, acknowledge stamps acknowledged_at.
	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, schoolWide.ID, chain.AccountID))
	recipients, err = repo.AudienceRecipients(ctx, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt = findRecipient(recipients)
	require.NotNil(t, rcpt)
	assert.NotNil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	require.NoError(t, repo.MarkAcknowledged(ctx, chain.TenantID, schoolWide.ID, chain.AccountID))
	recipients, err = repo.AudienceRecipients(ctx, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt = findRecipient(recipients)
	require.NotNil(t, rcpt)
	assert.NotNil(t, rcpt.AcknowledgedAt)

	// A target that reaches nobody yields no row for this account.
	missRecipients, err := repo.AudienceRecipients(ctx, chain.TenantID, classMiss.ID)
	require.NoError(t, err)
	assert.Nil(t, findRecipient(missRecipients), "class 9z reaches no recipient in this fixture")
}

func strp(s string) *string { return &s }
