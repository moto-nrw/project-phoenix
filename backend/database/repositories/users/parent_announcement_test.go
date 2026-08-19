package users_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
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
	// Publish 2s in the past: the DB clock can lag the Go clock in the Docker
	// VM, and published_at <= NOW() guards must not see a future timestamp.
	now := time.Now().Add(-2 * time.Second)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t) // tenant 1
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

	readApplied, err := repo.MarkRead(ctx, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt)
	require.NoError(t, err)
	assert.True(t, readApplied, "reading at the live version applies")
	unreadAfter, err := repo.CountUnreadForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	assert.Equal(t, unreadBefore-1, unreadAfter, "reading one announcement drops unread by one")

	// --- Acknowledge stamps the read row ---
	ackApplied, err := repo.MarkAcknowledged(ctx, chain.TenantID, classMatch.ID, chain.AccountID, *classMatch.PublishedAt)
	require.NoError(t, err)
	assert.True(t, ackApplied, "acknowledging at the live version applies")
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)
	_, err := db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", "en").
		Where("account_id = ?", chain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

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
	assert.Equal(t, "en", rcpt.PortalLocale)
	assert.Nil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	// Read stamps read_at, acknowledge stamps acknowledged_at.
	readApplied, err := repo.MarkRead(ctx, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt)
	require.NoError(t, err)
	assert.True(t, readApplied)
	recipients, err = repo.AudienceRecipients(ctx, chain.TenantID, schoolWide.ID)
	require.NoError(t, err)
	rcpt = findRecipient(recipients)
	require.NotNil(t, rcpt)
	assert.NotNil(t, rcpt.ReadAt)
	assert.Nil(t, rcpt.AcknowledgedAt)

	ackApplied, err := repo.MarkAcknowledged(ctx, chain.TenantID, schoolWide.ID, chain.AccountID, *schoolWide.PublishedAt)
	require.NoError(t, err)
	assert.True(t, ackApplied)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

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
	// Publish 2s in the past: the DB clock can lag the Go clock in the Docker
	// VM, and published_at <= NOW() guards must not see a future timestamp.
	now := time.Now().Add(-2 * time.Second)
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	readApplied, err := repo.MarkRead(ctx, chain.TenantID, a.ID, chain.AccountID, now)
	require.NoError(t, err)
	assert.True(t, readApplied)
	ackApplied, err := repo.MarkAcknowledged(ctx, chain.TenantID, a.ID, chain.AccountID, now)
	require.NoError(t, err)
	assert.True(t, ackApplied)
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
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	a := &usersModels.ParentAnnouncement{
		Title: "Entwurf", Body: "x", Priority: usersModels.ParentAnnouncementPriorityInfo,
		Active: true, CreatedBy: chain.AccountID,
	}
	a.SetTenantID(chain.TenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, chain.TenantID, a.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}}))
	t.Cleanup(func() { _ = repo.Delete(ctx, a.ID) })

	// Publish it, then attempt to swap the audience to a single student.
	// Publish 2s in the past: the DB clock can lag the Go clock in the Docker
	// VM, and published_at <= NOW() guards must not see a future timestamp.
	now := time.Now().Add(-2 * time.Second)
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	studentID := chain.StudentID
	err := repo.ReplaceTargets(ctx, chain.TenantID, a.ID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetStudent, TargetRefID: &studentID}})
	require.ErrorIs(t, err, usersModels.ErrAnnouncementPublished,
		"a published announcement's targets must not be replaceable")

	// The original school_all target must survive the rejected swap.
	targets, err := repo.ListTargets(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, targets, 1)
	assert.Equal(t, usersModels.AnnouncementTargetSchoolAll, targets[0].TargetType,
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
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	a := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Versioniert", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})
	stale := a.PublishedAt.Add(-time.Hour) // a version the announcement never had

	// A stale published_at records nothing (no error) and reports the guard miss
	// as not-applied, so the service can surface it as a 409 instead of a silent
	// success.
	staleRead, err := repo.MarkRead(ctx, chain.TenantID, a.ID, chain.AccountID, stale)
	require.NoError(t, err)
	assert.False(t, staleRead, "stale-version read reports not applied")
	staleAck, err := repo.MarkAcknowledged(ctx, chain.TenantID, a.ID, chain.AccountID, stale)
	require.NoError(t, err)
	assert.False(t, staleAck, "stale-version ack reports not applied")
	stats, err := repo.Stats(ctx, chain.TenantID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.ReadCount, "stale-version read must not stamp")
	assert.Equal(t, 0, stats.AcknowledgedCount, "stale-version ack must not stamp")

	// The current version still stamps and reports applied.
	liveRead, err := repo.MarkRead(ctx, chain.TenantID, a.ID, chain.AccountID, *a.PublishedAt)
	require.NoError(t, err)
	assert.True(t, liveRead, "current-version read applies")
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // active mapping, tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // student class "1a", tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db) // tenant 1
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

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

func TestParentAnnouncementAudience_WeekdayScopedEnrollmentMatchesToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, chain.TenantID, "AG-Wochentag")
	today := timezone.TodayDate()
	todayWeekday := int(today.Weekday())
	if todayWeekday == 0 {
		todayWeekday = activitiesModels.WeekdaySunday
	}
	otherWeekday := todayWeekday%activitiesModels.WeekdaySunday + 1
	enrollment := &activitiesModels.StudentEnrollment{
		StudentID:       chain.StudentID,
		ActivityGroupID: group.ID,
		ValidFrom:       today.AddDays(-1),
		Weekday:         &otherWeekday,
	}
	enrollment.SetTenantID(chain.TenantID)
	_, err := db.NewInsert().
		Model(enrollment).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Model((*activitiesModels.StudentEnrollment)(nil)).
			ModelTableExpr("activities.student_enrollments").
			Where("id = ?", enrollment.ID).
			Exec(context.Background())
	})

	announcement := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"AG-Wochentag", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetActivityGroup, TargetRefID: &group.ID},
		})

	matched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, announcement.ID, chain.AccountID)
	require.NoError(t, err)
	assert.False(t, matched)
	count, err := repo.CountAudience(ctx, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Zero(t, count)
	recipients, err := repo.ResolveAudienceEmails(ctx, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Empty(t, recipients)
	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, []int64{chain.TenantID})
	require.NoError(t, err)
	assert.NotContains(t, announcementIDs(feed), announcement.ID)

	_, err = db.NewUpdate().
		Model((*activitiesModels.StudentEnrollment)(nil)).
		ModelTableExpr("activities.student_enrollments").
		Set("weekday = ?", todayWeekday).
		Where("id = ?", enrollment.ID).
		Exec(context.Background())
	require.NoError(t, err)

	matched, err = repo.AccountMatchesAnnouncement(ctx, chain.TenantID, announcement.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched)
	count, err = repo.CountAudience(ctx, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	recipients, err = repo.ResolveAudienceEmails(ctx, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	require.Len(t, recipients, 1)
	assert.Equal(t, chain.AccountID, recipients[0].AccountID)
	feed, err = repo.ListFeedForAccount(ctx, chain.AccountID, []int64{chain.TenantID})
	require.NoError(t, err)
	assert.Contains(t, announcementIDs(feed), announcement.ID)
}

func announcementIDs(items []*usersModels.AnnouncementFeedItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
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
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	bg := context.Background()
	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)
	tenantIDs := []int64{chain.TenantID}

	// A phase to anchor the enrollment request FK.
	phase := &enrollmentModels.Phase{
		Name:                      "Testphase",
		ServiceStartDate:          timezone.NewDate(2026, time.September, 1),
		ServiceEndDate:            timezone.NewDate(2027, time.July, 31),
		CareOverflowMode:          "waitlist",
		CareOfferingSelectionMode: "optional",
	}
	phase.SetTenantID(chain.TenantID)
	_, err := db.NewInsert().Model(phase).ModelTableExpr("enrollment.phases").Exec(bg)
	require.NoError(t, err)
	// defer (not t.Cleanup) so the row is removed while db is still open — the
	// LIFO order deletes child -> request -> phase before CleanupParentGuardianChain
	// and db.Close, keeping tenant 1's open-enrollment set clean for other tests.
	defer func() {
		_, _ = db.NewDelete().Model((*enrollmentModels.Phase)(nil)).
			ModelTableExpr("enrollment.phases").Where("id = ?", phase.ID).Exec(bg)
	}()

	// An UNSTAMPED request (guardian_account_id NULL) whose e-mail matches the
	// chain account — the invite-accept-backfill-failed edge case.
	req := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Sabine",
		GuardianLastName:  "Schneider",
		GuardianEmail:     chain.Email,
		GuardianAccountID: nil,
		StatusToken:       fmt.Sprintf("tok-%d", time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	req.SetTenantID(chain.TenantID)
	_, err = db.NewInsert().Model(req).ModelTableExpr("enrollment.requests").Exec(bg)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewDelete().Model((*enrollmentModels.Request)(nil)).
			ModelTableExpr("enrollment.requests").Where("id = ?", req.ID).Exec(bg)
	}()

	child := &enrollmentModels.RequestChild{
		RequestID:   req.ID,
		FirstName:   "Felix",
		LastName:    "Schneider",
		DateOfBirth: timezone.NewDate(2019, time.March, 3),
		Status:      enrollmentModels.ChildStatusSubmitted,
	}
	child.SetTenantID(chain.TenantID)
	_, err = db.NewInsert().Model(child).ModelTableExpr("enrollment.request_children").Exec(bg)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewDelete().Model((*enrollmentModels.RequestChild)(nil)).
			ModelTableExpr("enrollment.request_children").Where("id = ?", child.ID).Exec(bg)
	}()

	ann := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Offene Anmeldungen", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetPendingEnrollment},
		})
	// Delete the announcement while db is still open (before CleanupParentGuardianChain
	// runs), so the created_by FK does not block the chain account teardown.
	defer func() { _ = repo.Delete(ctx, ann.ID) }()

	// --- AccountMatchesAnnouncement: the e-mail-matched applicant is reached ---
	matched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, chain.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "an unstamped request whose e-mail matches the account must be reached")

	// A different account (no matching request e-mail) must NOT be reached — the
	// fallback resolves the real account, it does not match everyone.
	other := testpkg.CreateTestAccount(t, db, "other-parent")
	defer testpkg.CleanupAccount(t, db, other.ID)
	otherMatched, err := repo.AccountMatchesAnnouncement(ctx, chain.TenantID, ann.ID, other.ID)
	require.NoError(t, err)
	assert.False(t, otherMatched, "an account with no matching enrollment request must NOT be reached")

	// --- Feed + unread badge include the announcement ---
	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	inFeed := false
	for _, item := range feed {
		if item.ID == ann.ID {
			inFeed = true
		}
	}
	assert.True(t, inFeed, "feed includes the pending_enrollment announcement for the e-mail-matched applicant")

	unread, err := repo.CountUnreadForAccount(ctx, chain.AccountID, tenantIDs)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, unread, 1, "unread badge counts the announcement")

	// --- Staff surfaces: audience count, recipients, e-mail, stats ---
	count, err := repo.CountAudience(ctx, chain.TenantID, ann.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "the e-mail-matched applicant counts toward the audience")

	recipients, err := repo.AudienceRecipients(ctx, chain.TenantID, ann.ID)
	require.NoError(t, err)
	foundRcpt := false
	for _, r := range recipients {
		if r.AccountID == chain.AccountID {
			foundRcpt = true
		}
	}
	assert.True(t, foundRcpt, "recipients include the e-mail-matched applicant account")

	emails, err := repo.ResolveAudienceEmails(ctx, chain.TenantID, ann.ID)
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

	stats, err := repo.Stats(ctx, chain.TenantID, ann.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TargetCount, 1, "stats count the e-mail-matched applicant")
}

func strp(s string) *string { return &s }
