package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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
	a.PublishedAt = &now // reflect the persisted version so callers can pass it to MarkRead/MarkAcknowledged
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

	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt))
	unreadAfter, err := repo.CountUnreadForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	assert.Equal(t, unreadBefore-1, unreadAfter, "reading one announcement drops unread by one")

	// --- Acknowledge stamps the read row ---
	require.NoError(t, repo.MarkAcknowledged(ctx, chain.TenantID, classMatch.ID, chain.AccountID, *classMatch.PublishedAt))
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
	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt))
	recipients, err = repo.AudienceRecipients(ctx, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt = findRecipient(recipients)
	require.NotNil(t, rcpt)
	assert.NotNil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	require.NoError(t, repo.MarkAcknowledged(ctx, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt))
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

// TestParentAnnouncementUpdate_AtomicAndClearsReads verifies the draft Update:
// it refuses a published row (WHERE published_at IS NULL, so a concurrent
// publish can never be silently reverted), and editing a draft clears any
// read/ack state left over from a previous publication (the correction path).
func TestParentAnnouncementUpdate_AtomicAndClearsReads(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()

	a := &usersModels.ParentAnnouncement{
		Title: "Entwurf", Body: "x", Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	}
	a.SetTenantID(chain.TenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, chain.TenantID, a.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}))
	t.Cleanup(func() { _ = repo.Delete(ctx, a.ID) })

	// Publish, then the guardian reads + acknowledges.
	now := time.Now()
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, a.ID, chain.AccountID, now))
	require.NoError(t, repo.MarkAcknowledged(ctx, chain.TenantID, a.ID, chain.AccountID, now))
	stats, err := repo.Stats(ctx, chain.TenantID, a.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.ReadCount, 1)
	require.GreaterOrEqual(t, stats.AcknowledgedCount, 1)

	// A published row is immutable at the write layer: Update matches no row.
	a.Title = "Heimlich geaendert"
	require.ErrorIs(t, repo.Update(ctx, a), usersModels.ErrAnnouncementPublished)
	got, err := repo.FindByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "Entwurf", got.Title, "published title must not be overwritten")

	// Retract -> edit: the update succeeds and clears the stale reads/acks.
	require.NoError(t, repo.SetPublished(ctx, a.ID, nil))
	a.Title = "Korrigiert"
	require.NoError(t, repo.Update(ctx, a))
	got, err = repo.FindByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "Korrigiert", got.Title)
	assert.Nil(t, got.PublishedAt, "edit must not resurrect published_at")

	stats, err = repo.Stats(ctx, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.ReadCount, "editing a draft clears stale read state")
	assert.Equal(t, 0, stats.AcknowledgedCount, "editing a draft clears stale ack state")
}

// TestParentAnnouncementDelete verifies the staff delete path: the generic
// base.Repository.Delete builds an aliased table expression + WHERE, and the
// model's BeforeAppendModel must leave DeleteQuery untouched so the alias
// survives (a bare-table override would produce a WHERE referencing a missing
// alias and fail). Targets + reads cascade in the DB.
func TestParentAnnouncementDelete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()

	a := &usersModels.ParentAnnouncement{
		Title: "Zu löschen", Body: "x", Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	}
	a.SetTenantID(chain.TenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, chain.TenantID, a.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}))

	require.NoError(t, repo.Delete(ctx, a.ID), "delete must not fail on a missing alias")
	got, err := repo.FindByID(ctx, a.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()

	a := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Versioniert", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})
	stale := a.PublishedAt.Add(-time.Hour) // a version the announcement never had

	// A stale published_at records nothing (no error — it is simply a no-op).
	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, a.ID, chain.AccountID, stale))
	require.NoError(t, repo.MarkAcknowledged(ctx, chain.TenantID, a.ID, chain.AccountID, stale))
	stats, err := repo.Stats(ctx, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.ReadCount, "stale-version read must not stamp")
	assert.Equal(t, 0, stats.AcknowledgedCount, "stale-version ack must not stamp")

	// The current version still stamps.
	require.NoError(t, repo.MarkRead(ctx, chain.TenantID, a.ID, chain.AccountID, *a.PublishedAt))
	stats, err = repo.Stats(ctx, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.ReadCount, "current-version read stamps")
}

// TestParentAnnouncementAudience_InactiveMembershipExcluded verifies that a
// guardian whose auth.account_tenants mapping is no longer active drops out of
// every audience surface, even though the guardian_profiles + students_guardians
// rows (with parent_portal.access) still exist. Membership, not just the
// relationship, is what grants parent-portal access.
func TestParentAnnouncementAudience_InactiveMembershipExcluded(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // active mapping, tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()
	tenantIDs := []int64{chain.TenantID}

	ann := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Schulweit", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})

	// Sanity: while the mapping is active the guardian is in the audience.
	matched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	require.True(t, matched, "sanity: an active guardian is reached")
	count, err := repo.CountAudience(ctx, chain.TenantID, ann.ID)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, 1)

	// Revoke school access: flip the mapping to inactive.
	_, err = db.NewUpdate().
		Model((*authModels.AccountTenant)(nil)).
		ModelTableExpr("auth.account_tenants").
		Set("status = ?", authModels.AccountTenantStatusInactive).
		Where("account_id = ? AND tenant_id = ?", chain.AccountID, chain.TenantID).
		Exec(context.Background())
	require.NoError(t, err)

	matched, err = repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "an inactive membership must not be reached")

	count, err = repo.CountAudience(ctx, chain.TenantID, ann.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "inactive membership excluded from CountAudience")

	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	assert.Empty(t, feed, "inactive membership sees no feed for this tenant")
}

// TestParentAnnouncementAudience_ClassMatchIsCaseInsensitive verifies a class
// target matches a student's school_class case- and whitespace-insensitively,
// the same LOWER(TRIM(...)) rule the canonical FindBySchoolClass uses and that
// ListSchoolClasses (TRIM) feeds the selector with — so staff can never pick a
// visible class that then reaches no guardian.
func TestParentAnnouncementAudience_ClassMatchIsCaseInsensitive(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()

	// Uppercase + padded target text against a lowercase "1a" student class.
	ann := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Klasse 1A", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetClass, TargetRefText: strp(" 1A ")},
		})

	matched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "class target ' 1A ' must reach a '1a' student (case/space-insensitive)")
}

// TestParentAnnouncementAudience_FutureEnrollmentExcluded verifies an
// activity_group target only reaches a student whose enrollment has already
// started: a future valid_from is not yet in the audience, and the guardian
// enters it once the enrollment becomes current.
func TestParentAnnouncementAudience_FutureEnrollmentExcluded(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	chain := testpkg.CreateTestParentGuardianChain(t, db) // tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx()

	group := testpkg.CreateTestActivityGroupForTenant(t, db, chain.TenantID, "AG-Regression")

	// Enroll the chain student with a valid_from a week out (not yet started).
	enr := &activitiesModels.StudentEnrollment{
		StudentID:       chain.StudentID,
		ActivityGroupID: group.ID,
		ValidFrom:       timezone.TodayDate().AddDays(7),
	}
	enr.SetTenantID(chain.TenantID)
	_, err := db.NewInsert().
		Model(enr).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Model((*activitiesModels.StudentEnrollment)(nil)).
			ModelTableExpr("activities.student_enrollments").
			Where("id = ?", enr.ID).
			Exec(context.Background())
	})

	ann := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"AG-Mitteilung", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetActivityGroup, TargetRefID: &group.ID},
		})

	matched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "a not-yet-started (future valid_from) enrollment must not be reached")

	// Advance the enrollment to today: now it is current and reaches the guardian.
	_, err = db.NewUpdate().
		Model((*activitiesModels.StudentEnrollment)(nil)).
		ModelTableExpr("activities.student_enrollments").
		Set("valid_from = ?", timezone.TodayDate()).
		Where("id = ?", enr.ID).
		Exec(context.Background())
	require.NoError(t, err)

	matched, err = repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "a started enrollment reaches the guardian")
}

func strp(s string) *string { return &s }
