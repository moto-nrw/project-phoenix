package contracttest_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// publishedAnnouncement creates an announcement with the given targets, publishes
// it, and registers cleanup. created_by is the chain account (a real auth.account).

// reflect the persisted version so callers can pass it to MarkRead/MarkAcknowledged

func databaseTimestamp(t *testing.T, db *bun.DB) time.Time {
	t.Helper()
	var now time.Time
	require.NoError(t, db.NewSelect().ColumnExpr("CURRENT_TIMESTAMP").Scan(context.Background(), &now))
	return now
}

// TestParentAnnouncementAudience exercises the audience resolver against a real
// guardian->student link: school-wide and class targets reach the linked
// guardian; a non-matching class does not; drafts stay out of the feed; and
// read state flows through the feed + unread count.
func TestParentAnnouncementAudience(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t) // tenant 1
	tenantIDs := []int64{chain.TenantID}

	schoolWide := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Schulweit", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetSchoolAll},
		}))

	classMatch := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Klasse 1a", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("1a")},
		}))

	classMiss := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Klasse 9z", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("9z")},
		}))

	// A draft (never published) must never reach the feed.
	draft := api.ParentStoreAnnouncementValue(api.ParentStoreAnnouncementInput{
		Title: "Entwurf", Body: "x", Priority: userModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	})
	draft.TenantID = chain.TenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, draft))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, repo.ReplaceTargets, chain.TenantID, draft.ID, api.ParentStoreTargetValues([]api.ParentStoreTargetInput{{TargetType: userModels.AnnouncementTargetSchoolAll}})))
	t.Cleanup(func() { _ = testpkg.TenantCommand1(t, ctx, db, repo.Delete, draft.ID) })

	// --- AccountMatchesAnnouncement ---
	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, schoolWide.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "school-wide should reach the linked guardian")

	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, classMatch.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "class 1a should reach a 1a student's guardian")

	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, classMiss.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "class 9z must NOT reach a 1a student's guardian")

	// --- CountAudience ---
	count, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "school-wide audience includes the linked guardian")

	missCount, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, classMiss.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, missCount, "class 9z reaches nobody in this fixture")

	// --- Feed: published reachable announcements, drafts excluded ---
	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: tenantIDs}))
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
	unreadBefore, err := testpkg.TenantQuery2(t, ctx, db, repo.CountUnreadForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: tenantIDs}))
	require.NoError(t, err)
	require.GreaterOrEqual(t, unreadBefore, 2)

	readApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkRead, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt)
	require.NoError(t, err)
	assert.True(t, readApplied, "reading at the live version applies")
	unreadAfter, err := testpkg.TenantQuery2(t, ctx, db, repo.CountUnreadForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: tenantIDs}))
	require.NoError(t, err)
	assert.Equal(t, unreadBefore-1, unreadAfter, "reading one announcement drops unread by one")

	// --- Acknowledge stamps the read row ---
	ackApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkAcknowledged, chain.TenantID, classMatch.ID, chain.AccountID, *classMatch.PublishedAt)
	require.NoError(t, err)
	assert.True(t, ackApplied, "acknowledging at the live version applies")
	stats, err := testpkg.TenantQuery2(t, ctx, db, repo.Stats, chain.TenantID, classMatch.ID)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	_, err := db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", "en").
		Where("account_id = ?", chain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	schoolWide := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Schulweit Empfänger", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetSchoolAll},
		}))
	classMiss := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Klasse 9z Empfänger", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("9z")},
		}))

	// Pending before any read.
	recipients, err := testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt := api.ParentStoreRecipientByAccount(recipients, chain.AccountID)
	require.NotNil(t, rcpt, "school-wide recipients include the linked guardian account")
	assert.Equal(t, "en", rcpt.PortalLocale)
	assert.Nil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	// Read stamps read_at, acknowledge stamps acknowledged_at.
	readApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkRead, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt)
	require.NoError(t, err)
	assert.True(t, readApplied)
	recipients, err = testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt = api.ParentStoreRecipientByAccount(recipients, chain.AccountID)
	require.NotNil(t, rcpt)
	assert.NotNil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	ackApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkAcknowledged, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt)
	require.NoError(t, err)
	assert.True(t, ackApplied)
	recipients, err = testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt = api.ParentStoreRecipientByAccount(recipients, chain.AccountID)
	require.NotNil(t, rcpt)
	assert.NotNil(t, rcpt.AcknowledgedAt)

	// A target that reaches nobody yields no row for this account.
	missRecipients, err := testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, chain.TenantID, classMiss.ID)
	require.NoError(t, err)
	assert.Nil(t, api.ParentStoreRecipientByAccount(missRecipients, chain.AccountID), "class 9z reaches no recipient in this fixture")
}

// TestParentAnnouncementUpdate_AtomicAndClearsReads verifies the draft Update:
// it refuses a published row (WHERE published_at IS NULL, so a concurrent
// publish can never be silently reverted), and editing a draft clears any
// read/ack state left over from a previous publication (the correction path).
func TestParentAnnouncementUpdate_AtomicAndClearsReads(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)

	a := api.ParentStoreAnnouncementValue(api.ParentStoreAnnouncementInput{
		Title: "Entwurf", Body: "x", Priority: userModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	})
	a.TenantID = chain.TenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, a))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, repo.ReplaceTargets, chain.TenantID, a.ID, api.ParentStoreTargetValues([]api.ParentStoreTargetInput{{TargetType: userModels.AnnouncementTargetSchoolAll}})))
	t.Cleanup(func() { _ = testpkg.TenantCommand1(t, ctx, db, repo.Delete, a.ID) })

	// Publish, then the guardian reads + acknowledges.
	now := databaseTimestamp(t, db)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.SetPublished, a.ID, &now))
	readApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkRead, chain.TenantID, a.ID, chain.AccountID, now)
	require.NoError(t, err)
	assert.True(t, readApplied)
	ackApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkAcknowledged, chain.TenantID, a.ID, chain.AccountID, now)
	require.NoError(t, err)
	assert.True(t, ackApplied)
	stats, err := testpkg.TenantQuery2(t, ctx, db, repo.Stats, chain.TenantID, a.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.ReadCount, 1)
	require.GreaterOrEqual(t, stats.AcknowledgedCount, 1)

	// A published row is immutable at the write layer: Update matches no row.
	a.Title = "Heimlich geaendert"
	require.ErrorIs(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, a), api.ParentStorePublishedConflict())
	got, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "Entwurf", got.Title, "published title must not be overwritten")

	// Retract -> edit: the update succeeds and clears the stale reads/acks.
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.SetPublished, a.ID, nil))
	a.Title = "Korrigiert"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Update, a))
	got, err = testpkg.TenantQuery1(t, ctx, db, repo.FindByID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "Korrigiert", got.Title)
	assert.Nil(t, got.PublishedAt, "edit must not resurrect published_at")

	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.ReadCount, "editing a draft clears stale read state")
	assert.Equal(t, 0, stats.AcknowledgedCount, "editing a draft clears stale ack state")
}

// TestParentAnnouncementReplaceTargets_RefusesPublished verifies the target
// swap is guarded like the content Update: once an announcement is published,
// ReplaceTargets locks the row, sees published_at != NULL and returns
// ErrAnnouncementPublished without touching the target rows. This closes the
// window where a publish (which enqueues the opt-in e-mails against the current
// targets) could race a target edit and desynchronize the e-mailed audience
// from the live feed/stats audience.
func TestParentAnnouncementReplaceTargets_RefusesPublished(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)

	a := api.ParentStoreAnnouncementValue(api.ParentStoreAnnouncementInput{
		Title: "Entwurf", Body: "x", Priority: userModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	})
	a.TenantID = chain.TenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, a))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, repo.ReplaceTargets, chain.TenantID, a.ID, api.ParentStoreTargetValues([]api.ParentStoreTargetInput{{TargetType: userModels.AnnouncementTargetSchoolAll}})))
	t.Cleanup(func() { _ = testpkg.TenantCommand1(t, ctx, db, repo.Delete, a.ID) })

	// Publish it, then attempt to swap the audience to a single student.
	now := databaseTimestamp(t, db)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.SetPublished, a.ID, &now))
	studentID := chain.StudentID
	err := testpkg.TenantCommand3(t, ctx, db, repo.ReplaceTargets, chain.TenantID, a.ID, api.ParentStoreTargetValues([]api.ParentStoreTargetInput{{TargetType: userModels.AnnouncementTargetStudent, TargetRefID: &studentID}}))
	require.ErrorIs(t, err, api.ParentStorePublishedConflict(),
		"a published announcement's targets must not be replaceable")

	// The original school_all target must survive the rejected swap.
	targets, err := testpkg.TenantQuery1(t, ctx, db, repo.ListTargets, a.ID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, userModels.AnnouncementTargetSchoolAll, targets[0].TargetType,
		"rejected swap must leave the published audience untouched")
}

// TestParentAnnouncementDelete verifies the staff delete path: the generic
// base.Repository.Delete builds an aliased table expression + WHERE, and the
// model's BeforeAppendModel must leave DeleteQuery untouched so the alias
// survives (a bare-table override would produce a WHERE referencing a missing
// alias and fail). Targets + reads cascade in the DB.
func TestParentAnnouncementDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)

	a := api.ParentStoreAnnouncementValue(api.ParentStoreAnnouncementInput{
		Title: "Zu löschen", Body: "x", Priority: userModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	})
	a.TenantID = chain.TenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Create, a))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, db, repo.ReplaceTargets, chain.TenantID, a.ID, api.ParentStoreTargetValues([]api.ParentStoreTargetInput{{TargetType: userModels.AnnouncementTargetSchoolAll}})))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Delete, a.ID), "delete must not fail on a missing alias")
	got, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, a.ID)
	require.NoError(t, err)
	assert.Nil(t, got, "deleted announcement is gone")
}

// TestParentAnnouncementMarkRead_VersionGuard verifies that MarkRead /
// MarkAcknowledged only stamp when the announcement is still live at the
// expected published_at. A request carrying the version the guardian loaded
// before a correction (unpublish -> edit -> republish re-stamps published_at)
// must record nothing against the corrected wording — the guard closes the
// window between the service's authorize phase and the write.
func TestParentAnnouncementMarkRead_VersionGuard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)

	a := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Versioniert", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetSchoolAll},
		}))
	stale := a.PublishedAt.Add(-time.Hour) // a version the announcement never had

	// A stale published_at records nothing (no error) and reports the guard miss
	// as not-applied, so the service can surface it as a 409 instead of a silent
	// success.
	staleRead, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkRead, chain.TenantID, a.ID, chain.AccountID, stale)
	require.NoError(t, err)
	assert.False(t, staleRead, "stale-version read reports not applied")
	staleAck, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkAcknowledged, chain.TenantID, a.ID, chain.AccountID, stale)
	require.NoError(t, err)
	assert.False(t, staleAck, "stale-version ack reports not applied")
	stats, err := testpkg.TenantQuery2(t, ctx, db, repo.Stats, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.ReadCount, "stale-version read must not stamp")
	assert.Equal(t, 0, stats.AcknowledgedCount, "stale-version ack must not stamp")

	// The current version still stamps and reports applied.
	liveRead, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkRead, chain.TenantID, a.ID, chain.AccountID, *a.PublishedAt)
	require.NoError(t, err)
	assert.True(t, liveRead, "current-version read applies")
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.ReadCount, "current-version read stamps")
}

// TestParentAnnouncementAudience_InactiveMembershipExcluded verifies that a
// guardian whose auth.account_tenants mapping is no longer active drops out of
// every audience surface, even though the guardian_profiles + relationship
// rows (with parent_portal.access) still exist. Membership, not just the
// relationship, is what grants parent-portal access.
func TestParentAnnouncementAudience_InactiveMembershipExcluded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // active mapping, tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	tenantIDs := []int64{chain.TenantID}

	ann := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Schulweit", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetSchoolAll},
		}))

	// Sanity: while the mapping is active the guardian is in the audience.
	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	require.True(t, matched, "sanity: an active guardian is reached")
	countBefore, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, ann.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, countBefore, 1)

	// Revoke school access: flip the mapping to inactive.
	_, err = db.NewUpdate().
		Model((*testpkg.AccountTenantFixture)(nil)).
		ModelTableExpr("auth.account_tenants").
		Set("status = ?", "inactive").
		Where("account_id = ? AND tenant_id = ?", chain.AccountID, chain.TenantID).
		Exec(context.Background())
	require.NoError(t, err)

	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "an inactive membership must not be reached")

	count, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, ann.ID)
	require.NoError(t, err)
	assert.Equal(t, countBefore-1, count, "inactive membership excluded from CountAudience")

	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: tenantIDs}))
	require.NoError(t, err)
	assert.Empty(t, feed, "inactive membership sees no feed for this tenant")
}

// TestParentAnnouncementAudience_ClassMatchIsCaseInsensitive verifies a class
// target matches a student's school_class case- and whitespace-insensitively,
// the same LOWER(TRIM(...)) rule the canonical FindBySchoolClass uses and that
// ListSchoolClasses (TRIM) feeds the selector with — so staff can never pick a
// visible class that then reaches no guardian.
func TestParentAnnouncementAudience_ClassMatchIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)

	// Uppercase + padded target text against a lowercase "1a" student class.
	ann := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Klasse 1A", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp(" 1A ")},
		}))

	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "class target ' 1A ' must reach a '1a' student (case/space-insensitive)")
}

// TestParentAnnouncementAudience_FutureEnrollmentExcluded verifies an
// activity_group target only reaches a student whose enrollment has already
// started: a future valid_from is not yet in the audience, and the guardian
// enters it once the enrollment becomes current.
func TestParentAnnouncementAudience_FutureEnrollmentExcluded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)

	group := testpkg.CreateTestActivityGroupForTenant(t, db, chain.TenantID, "AG-Regression")

	// Enroll the chain student with a valid_from a week out (not yet started).
	enr := &testpkg.StudentEnrollment{
		StudentID:       chain.StudentID,
		ActivityGroupID: group.ID,
		ValidFrom:       testpkg.ActivityDate(calendar.TodayDate().AddDays(7)),
	}
	enr.TenantID = chain.TenantID
	_, err := db.NewInsert().
		Model(enr).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Exec(context.Background())
	require.NoError(t, err)

	ann := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"AG-Mitteilung", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetActivityGroup, TargetRefID: &group.ID},
		}))

	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "a not-yet-started (future valid_from) enrollment must not be reached")

	// Advance the enrollment to today: now it is current and reaches the guardian.
	_, err = db.NewUpdate().
		Model((*testpkg.StudentEnrollment)(nil)).
		ModelTableExpr("activities.student_enrollments").
		Set("valid_from = ?", calendar.TodayDate()).
		Where("id = ?", enr.ID).
		Exec(context.Background())
	require.NoError(t, err)

	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "a started enrollment reaches the guardian")
}

func TestParentAnnouncementAudience_WeekdayScopedEnrollmentMatchesToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db), func() time.Time {
		return calendar.NewDate(2026, 8, 24).BerlinMidnight()
	})
	ctx := tenantCtx(t)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, chain.TenantID, "AG-Wochentag")
	today := calendar.NewDate(2026, 8, 24)
	todayWeekday := int(today.Weekday())
	if todayWeekday == 0 {
		todayWeekday = testpkg.WeekdaySunday
	}
	otherWeekday := todayWeekday%testpkg.WeekdaySunday + 1
	enrollment := &testpkg.StudentEnrollment{
		StudentID:       chain.StudentID,
		ActivityGroupID: group.ID,
		ValidFrom:       testpkg.ActivityDate(today.AddDays(-1)),
		Weekday:         &otherWeekday,
	}
	enrollment.TenantID = chain.TenantID
	_, err := db.NewInsert().
		Model(enrollment).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Exec(context.Background())
	require.NoError(t, err)

	announcement := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"AG-Wochentag", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetActivityGroup, TargetRefID: &group.ID},
		}))

	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, announcement.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched)
	count, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Zero(t, count)
	recipients, err := testpkg.TenantQuery2(t, ctx, db, repo.ResolveAudienceEmails, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Empty(t, recipients)
	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: []int64{chain.TenantID}}))
	require.NoError(t, err)
	assert.NotContains(t, api.ParentStoreAnnouncementIDs(feed), announcement.ID)

	_, err = db.NewUpdate().
		Model((*testpkg.StudentEnrollment)(nil)).
		ModelTableExpr("activities.student_enrollments").
		Set("weekday = ?", todayWeekday).
		Where("id = ?", enrollment.ID).
		Exec(context.Background())
	require.NoError(t, err)

	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, announcement.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched)
	count, err = testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	recipients, err = testpkg.TenantQuery2(t, ctx, db, repo.ResolveAudienceEmails, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	require.Len(t, recipients, 1)
	assert.Equal(t, chain.AccountID, recipients[0].AccountID)
	feed, err = testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: []int64{chain.TenantID}}))
	require.NoError(t, err)
	assert.Contains(t, api.ParentStoreAnnouncementIDs(feed), announcement.ID)
}

// TestParentAnnouncementAudience_PendingEnrollmentEmailFallback verifies a
// pending_enrollment announcement reaches an applicant whose enrollment request
// was never stamped with guardian_account_id but whose guardian_email matches
// the account's e-mail — the same ownership fallback that
// parent.EnrollmentRequestRepository.ListByAccount (and thus newsEnabledTenants)
// applies. Without it the applicant saw the school in the feed's tenant set yet
// no announcement, while staff stats + e-mail counted them symmetrically. All
// four surfaces (feed/unread, count, recipients, e-mail, stats) must agree.
func TestParentAnnouncementAudience_PendingEnrollmentEmailFallback(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // account with a real e-mail, tenant 1

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	tenantIDs := []int64{chain.TenantID}

	// An UNSTAMPED request (guardian_account_id NULL) whose e-mail matches the
	// chain account — the invite-accept-backfill-failed edge case.
	testpkg.CreateTestUnstampedEnrollmentApplicant(t, db, chain.TenantID, chain.Email)

	ann := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, chain.AccountID, chain.TenantID,
		"Offene Anmeldungen", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetPendingEnrollment},
		}))
	// Delete the announcement while db is still open (before CleanupParentGuardianChain
	// runs), so the created_by FK does not block the chain account teardown.
	defer func() { _ = testpkg.TenantCommand1(t, ctx, db, repo.Delete, ann.ID) }()

	// --- AccountMatchesAnnouncement: the e-mail-matched applicant is reached ---
	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "an unstamped request whose e-mail matches the account must be reached")

	// A different account (no matching request e-mail) must NOT be reached — the
	// fallback resolves the real account, it does not match everyone.
	other := testpkg.CreateTestAccount(t, db, "other-parent")
	otherMatched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, chain.TenantID, ann.ID, other.ID)
	require.NoError(t, err)
	assert.False(t, otherMatched, "an account with no matching enrollment request must NOT be reached")

	// --- Feed + unread badge include the announcement ---
	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: tenantIDs}))
	require.NoError(t, err)
	inFeed := false
	for _, item := range feed {
		if item.ID == ann.ID {
			inFeed = true
		}
	}
	assert.True(t, inFeed, "feed includes the pending_enrollment announcement for the e-mail-matched applicant")

	unread, err := testpkg.TenantQuery2(t, ctx, db, repo.CountUnreadForAccount, chain.AccountID, api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: tenantIDs}))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, unread, 1, "unread badge counts the announcement")

	// --- Staff surfaces: audience count, recipients, e-mail, stats ---
	count, err := testpkg.TenantQuery2(t, ctx, db, repo.CountAudience, chain.TenantID, ann.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "the e-mail-matched applicant counts toward the audience")

	recipients, err := testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, chain.TenantID, ann.ID)
	require.NoError(t, err)
	foundRcpt := false
	for _, r := range recipients {
		if r.AccountID == chain.AccountID {
			foundRcpt = true
		}
	}
	assert.True(t, foundRcpt, "recipients include the e-mail-matched applicant account")

	emails, err := testpkg.TenantQuery2(t, ctx, db, repo.ResolveAudienceEmails, chain.TenantID, ann.ID)
	require.NoError(t, err)
	foundEmail := false
	for _, e := range emails {
		if strings.EqualFold(e.Email, chain.Email) {
			assert.Equal(t, chain.AccountID, e.AccountID,
				"the e-mail recipient keeps the account identity needed for consent filtering")
			foundEmail = true
		}
	}
	assert.True(t, foundEmail, "the e-mail send targets the applicant's address")

	stats, err := testpkg.TenantQuery2(t, ctx, db, repo.Stats, chain.TenantID, ann.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TargetCount, 1, "stats count the e-mail-matched applicant")
}

func strp(s string) *string { return &s }

func parentStoreFixtureChecks(t testing.TB) api.ParentStoreFixtureChecks {
	t.Helper()
	return api.ParentStoreFixtureChecks{
		Helper: t.Helper, Cleanup: t.Cleanup,
		NoError: func(err error) { t.Helper(); require.NoError(t, err) },
		Len:     func(value any, length int) { t.Helper(); require.Len(t, value, length) },
	}
}
