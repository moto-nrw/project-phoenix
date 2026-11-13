package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	userModels "github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/rows"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// recipientRecorder is the publish-time write the staff publication runs
// (#6988); the parent repository suite exposes it on its concrete store.
type recipientRecorder interface {
	AnnouncementRecordRecipients(ctx context.Context, id int64, accountIDs []int64) error
}

func moveStudentToClass(t *testing.T, db *bun.DB, studentID int64, class string) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("school_class = ?", class).
		Where("student_profile_id = ?", studentID).
		Exec(context.Background())
	require.NoError(t, err)
}

// TestParentAnnouncementRecipients_RecordedAtPublication is the scenario of
// #6988: a letter goes to class 1a, then a child changes class. Statistics and
// the recipient list keep the parent the letter went to and do not add the
// parent of the child who joined; that parent still finds the letter in the
// feed, and so does the parent who left the class. A retraction drops the
// record and the audience of today counts again.
func TestParentAnnouncementRecipients_RecordedAtPublication(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	leaver := testpkg.CreateTestParentGuardianChain(t, db) // child in 1a
	joiner := testpkg.CreateTestParentGuardianChain(t, db)
	moveStudentToClass(t, db, joiner.StudentID, "2b")

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	recorder, ok := repo.(recipientRecorder)
	require.True(t, ok, "the announcement store records recipients")
	ctx := tenantCtx(t)
	scope := api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: []int64{leaver.TenantID}})

	letter := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, leaver.AccountID, leaver.TenantID,
		"Elternbrief 1a", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("1a")},
		}))

	// The publication records whom it reached at that moment.
	reached, err := testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, reached, 1)
	require.Equal(t, leaver.AccountID, reached[0].AccountID)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, recorder.AnnouncementRecordRecipients, letter.ID, []int64{reached[0].AccountID}))

	// The children swap classes after the letter went out.
	moveStudentToClass(t, db, leaver.StudentID, "2b")
	moveStudentToClass(t, db, joiner.StudentID, "1a")

	stats, err := testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TargetCount, "the letter still counts the one parent it reached")
	assert.True(t, stats.Recorded, "the counts say they refer to the recorded recipients")
	recipients, err := testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, recipients, 1, "no parent joins or leaves the recipient list")
	assert.Equal(t, leaver.AccountID, recipients[0].AccountID, "the recipient list keeps the parent the letter went to")
	assert.Equal(t, "Sabine", recipients[0].FirstName, "a recorded recipient keeps their name")

	// The parent who received it can still read and confirm it.
	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, leaver.TenantID, letter.ID, leaver.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "a recorded recipient with portal access keeps the letter")
	feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, leaver.AccountID, scope)
	require.NoError(t, err)
	inFeed := false
	for _, item := range feed {
		inFeed = inFeed || item.ID == letter.ID
	}
	assert.True(t, inFeed, "the recorded recipient still finds the letter in the feed")
	ackApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkAcknowledged, leaver.TenantID, letter.ID, leaver.AccountID, *letter.PublishedAt)
	require.NoError(t, err)
	require.True(t, ackApplied)
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.AcknowledgedCount, "the confirmation of the recorded recipient counts")

	// The parent of the child who joined sees the letter, as the class does today.
	feed, err = testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, joiner.AccountID, scope)
	require.NoError(t, err)
	inFeed = false
	for _, item := range feed {
		inFeed = inFeed || item.ID == letter.ID
	}
	assert.True(t, inFeed, "the class of today still sees the letter")

	// Without an active membership the recorded parent drops out everywhere.
	_, err = db.NewUpdate().
		TableExpr("auth.account_tenants").
		Set("status = ?", "inactive").
		Where("account_id = ? AND tenant_id = ?", leaver.AccountID, leaver.TenantID).
		Exec(context.Background())
	require.NoError(t, err)
	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, leaver.TenantID, letter.ID, leaver.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "membership is still required")
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TargetCount, "a parent who left the school is no longer counted")

	// A retraction drops the record; the draft shows the audience of today.
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.SetPublished, letter.ID, (*time.Time)(nil)))
	var recorded bool
	require.NoError(t, db.NewRaw(`SELECT recipient_account_ids IS NOT NULL FROM users.parent_announcements WHERE id = ?`, letter.ID).Scan(context.Background(), &recorded))
	assert.False(t, recorded, "the retraction clears the recorded recipients")
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.False(t, stats.Recorded, "a draft counts the audience of today")
	assert.Equal(t, 1, stats.TargetCount)
	recipients, err = testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, recipients, 1)
	assert.Equal(t, joiner.AccountID, recipients[0].AccountID, "an unrecorded announcement reaches the class of today")
}
