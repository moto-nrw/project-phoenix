package users_test

import (
	"context"
	"testing"
	"time"

	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// reminderAnnouncement creates an announcement carrying a reminder at
// reminderAt. published decides whether it is live; expiresAt is optional.
func reminderAnnouncement(
	t *testing.T,
	ctx context.Context,
	db *bun.DB,
	repo usersModels.ParentAnnouncementRepository,
	createdBy, tenantID int64,
	title string,
	reminderAt time.Time,
	published bool,
	expiresAt *time.Time,
) *usersModels.ParentAnnouncement {
	t.Helper()
	text := "Kurz: " + title
	a := &usersModels.ParentAnnouncement{
		Title:        title,
		Body:         "Testtext",
		Priority:     usersModels.ParentAnnouncementPriorityInfo,
		Active:       true,
		CreatedBy:    createdBy,
		ExpiresAt:    expiresAt,
		ReminderAt:   &reminderAt,
		ReminderText: &text,
	}
	a.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID, []*usersModels.ParentAnnouncementTarget{
		{TargetType: usersModels.AnnouncementTargetSchoolAll},
	}))
	if published {
		publishedAt := databaseTimestamp(t, db).Add(-7 * 24 * time.Hour)
		require.NoError(t, repo.SetPublished(ctx, a.ID, &publishedAt))
	}
	t.Cleanup(func() { _ = repo.Delete(ctx, a.ID) })
	return a
}

func idsOf(rows []*usersModels.ParentAnnouncement) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

// TestParentAnnouncementReminderDueScanAndClaim pins the exactly-once contract
// of the scheduled reminder (#3162): the due scan sees only live announcements
// whose unsent reminder fell inside the window, the claim succeeds once, and
// after the claim the reminder can no longer be edited.
func TestParentAnnouncementReminderDueScanAndClaim(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)

	now := databaseTimestamp(t, db)
	window := now.Add(-time.Hour)

	due := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Fällig", now.Add(-10*time.Minute), true, nil)
	draft := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Entwurf", now.Add(-10*time.Minute), false, nil)
	tooOld := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Zu alt", window.Add(-time.Minute), true, nil)
	future := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Später", now.Add(2*time.Hour), true, nil)
	// The expiry CHECK (expires_at > created_at) only accepts a future expiry
	// on insert; age the row afterwards, the way time itself does.
	expiresSoon := now.Add(time.Hour)
	expired := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Abgelaufen", now.Add(-10*time.Minute), true, &expiresSoon)
	_, err := db.NewUpdate().TableExpr("users.parent_announcements").
		Set("created_at = ?", now.Add(-2*time.Hour)).
		Set("expires_at = ?", now.Add(-time.Minute)).
		Where("id = ?", expired.ID).
		Exec(context.Background())
	require.NoError(t, err)

	rows, err := repo.ListDueReminders(ctx, window, now)
	require.NoError(t, err)
	got := idsOf(rows)
	assert.Contains(t, got, due.ID)
	assert.NotContains(t, got, draft.ID, "drafts never fire")
	assert.NotContains(t, got, tooOld.ID, "a reminder older than the window stays unsent")
	assert.NotContains(t, got, future.ID, "not due yet")
	assert.NotContains(t, got, expired.ID, "an expired announcement is invisible, so no reminder")
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Targets, 1, "the due row carries its targets for audience resolution")
	require.NotNil(t, rows[0].ReminderText)
	assert.Equal(t, "Kurz: Fällig", *rows[0].ReminderText)

	// Editing before the claim works, and only rewrites the reminder columns.
	moved := now.Add(30 * time.Minute)
	applied, err := repo.SetReminder(ctx, due.ID, &moved, nil)
	require.NoError(t, err)
	assert.True(t, applied)
	reloaded, err := repo.FindByID(ctx, due.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.ReminderAt)
	assert.WithinDuration(t, moved, *reloaded.ReminderAt, time.Second)
	assert.Nil(t, reloaded.ReminderText, "a nil text clears the wording")
	assert.Equal(t, due.Title, reloaded.Title)
	assert.NotNil(t, reloaded.PublishedAt, "the reminder edit leaves the publication untouched")

	// Move it back into the window and claim it: once, and only once.
	backDue := now.Add(-5 * time.Minute)
	applied, err = repo.SetReminder(ctx, due.ID, &backDue, nil)
	require.NoError(t, err)
	assert.True(t, applied)

	claimed, err := repo.ClaimReminder(ctx, due.ID, now)
	require.NoError(t, err)
	assert.True(t, claimed, "the first claim wins")
	released, err := repo.ReleaseReminderClaim(ctx, due.ID, now)
	require.NoError(t, err)
	assert.True(t, released, "a temporary delivery gate may restore its own claim")
	claimed, err = repo.ClaimReminder(ctx, due.ID, now.Add(time.Second))
	require.NoError(t, err)
	assert.True(t, claimed, "the restored reminder can be retried")
	released, err = repo.ReleaseReminderClaim(ctx, due.ID, now)
	require.NoError(t, err)
	assert.False(t, released, "an older scheduler run must not clear the newer claim")
	claimed, err = repo.ClaimReminder(ctx, due.ID, now)
	require.NoError(t, err)
	assert.False(t, claimed, "a second claim of the same reminder must not send again")

	rows, err = repo.ListDueReminders(ctx, window, now)
	require.NoError(t, err)
	assert.NotContains(t, idsOf(rows), due.ID, "a claimed reminder leaves the due scan")

	applied, err = repo.SetReminder(ctx, due.ID, &moved, nil)
	require.NoError(t, err)
	assert.False(t, applied, "a sent reminder is history and cannot be edited")

	reloaded, err = repo.FindByID(ctx, due.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.ReminderSentAt)

	// A correction starts a new publication cycle. Its former reminder belonged
	// to the withdrawn version, so no part of it may be retained or resent.
	require.NoError(t, repo.SetPublished(ctx, due.ID, nil))
	reloaded, err = repo.FindByID(ctx, due.ID)
	require.NoError(t, err)
	assert.Nil(t, reloaded.ReminderAt)
	assert.Nil(t, reloaded.ReminderText)
	assert.Nil(t, reloaded.ReminderSentAt)
	replanned := now.Add(time.Hour)
	applied, err = repo.SetReminder(ctx, due.ID, &replanned, nil)
	require.NoError(t, err)
	assert.True(t, applied)
	republishedAt := now
	require.NoError(t, repo.SetPublished(ctx, due.ID, &republishedAt))
	claimed, err = repo.ClaimReminder(ctx, due.ID, replanned.Add(time.Minute))
	require.NoError(t, err)
	assert.True(t, claimed)

	// Removing an unsent reminder drops moment and text together.
	applied, err = repo.SetReminder(ctx, future.ID, nil, nil)
	require.NoError(t, err)
	assert.True(t, applied)
	reloaded, err = repo.FindByID(ctx, future.ID)
	require.NoError(t, err)
	assert.Nil(t, reloaded.ReminderAt)
	assert.Nil(t, reloaded.ReminderText)

	// Claims that lose to a retraction match nothing.
	require.NoError(t, repo.SetPublished(ctx, draft.ID, nil))
	claimed, err = repo.ClaimReminder(ctx, draft.ID, now)
	require.NoError(t, err)
	assert.False(t, claimed, "a draft cannot be claimed")
	claimed, err = repo.ClaimReminder(ctx, expired.ID, now)
	require.NoError(t, err)
	assert.False(t, claimed, "an expired announcement cannot be claimed")
}

func TestParentAnnouncementReminderDueScanBatchLoadsTargets(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)
	now := databaseTimestamp(t, db)
	window := now.Add(-time.Hour)
	addDue := func(title string) {
		reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
			title, now.Add(-10*time.Minute), true, nil)
	}
	readDue := func(counter *testpkg.QueryCounter, want int) []string {
		counter.Reset()
		rows, err := repo.ListDueReminders(counter.Context(ctx), window, now)
		require.NoError(t, err)
		require.Len(t, rows, want)
		return counter.Queries()
	}

	addDue("Erste fällige Erinnerung")
	counter := testpkg.CaptureQueriesForContext(t, db)
	small := readDue(counter, 1)
	addDue("Zweite fällige Erinnerung")
	addDue("Dritte fällige Erinnerung")
	large := readDue(counter, 3)

	assert.Equal(t, len(small), len(large), "due scan reads must not grow with the number of reminders")
	testpkg.AssertQueryBudget(t, "repositories.parent_announcements.due_reminders", large)
}

// TestParentAnnouncementReminderFeedOrderAndTenantIsolation pins what the
// portal sees: a reminded announcement carries its reminder fields, sorts back
// to the top of the feed, keeps the guardian's read state, and a reminder of
// another school is invisible to the due scan of this one.
func TestParentAnnouncementReminderFeedOrderAndTenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repo := repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New())
	ctx := tenantCtx(t)
	now := databaseTimestamp(t, db)

	older := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Älter", now.Add(-10*time.Minute), true, nil)
	newer := publishedAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Neuer", []*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}})

	// The guardian read the older one weeks ago.
	olderRow, err := repo.FindByID(ctx, older.ID)
	require.NoError(t, err)
	live, err := repo.MarkRead(ctx, chain.TenantID, older.ID, chain.AccountID, *olderRow.PublishedAt)
	require.NoError(t, err)
	require.True(t, live)

	scope := usersModels.AnnouncementFeedScope{TenantIDs: []int64{chain.TenantID}}
	feed, err := repo.ListFeedForAccount(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(feed), 2)
	assert.Equal(t, newer.ID, feed[0].ID, "before the reminder the newer publication leads")

	// The reminder goes out AFTER the newer announcement was published.
	sentAt := databaseTimestamp(t, db)
	claimed, err := repo.ClaimReminder(ctx, older.ID, sentAt)
	require.NoError(t, err)
	require.True(t, claimed)

	feed, err = repo.ListFeedForAccount(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(feed), 2)
	assert.Equal(t, older.ID, feed[0].ID, "the reminded announcement is back on top")
	require.NotNil(t, feed[0].ReminderSentAt)
	require.NotNil(t, feed[0].ReminderText)
	assert.NotNil(t, feed[0].ReadAt, "the reminder does not reset the read state")

	unread, err := repo.CountUnreadForAccount(ctx, chain.AccountID, scope)
	require.NoError(t, err)
	assert.Equal(t, 1, unread, "only the never-read announcement counts; the reminder adds nothing")

	// Another school's due scan must not see this tenant's reminder. The
	// claim above already consumed it here, so seed a fresh unsent one first.
	pending := reminderAnnouncement(t, ctx, db, repo, chain.AccountID, chain.TenantID,
		"Offen", now.Add(-10*time.Minute), true, nil)
	rows, err := repo.ListDueReminders(ctx, now.Add(-time.Hour), now)
	require.NoError(t, err)
	require.Contains(t, idsOf(rows), pending.ID)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	rows, err = repo.ListDueReminders(testpkg.TenantContext(otherTenant), now.Add(-time.Hour), now)
	require.NoError(t, err)
	assert.NotContains(t, idsOf(rows), pending.ID)
}
