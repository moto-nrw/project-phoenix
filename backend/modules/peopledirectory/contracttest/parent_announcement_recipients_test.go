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
// #6988: a letter goes to class 1a, then the children swap classes. The
// letter keeps the child it reached: statistics, recipient list, feed and
// read/confirm follow that child's guardians, and the guardians of the child
// who joined 1a get nothing. Who may read for the child stays live. A
// retraction drops the record and the class of today counts again.
func TestParentAnnouncementRecipients_RecordedAtPublication(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	leaver := testpkg.CreateTestParentGuardianChain(t, db) // child in 1a
	joiner := testpkg.CreateTestParentGuardianChain(t, db)
	moveStudentToClass(t, db, joiner.StudentID, "2b")

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	scope := api.ParentStoreFeedScopeValue(api.ParentStoreFeedScopeInput{TenantIDs: []int64{leaver.TenantID}})
	inFeed := func(accountID, announcementID int64) bool {
		t.Helper()
		feed, err := testpkg.TenantQuery2(t, ctx, db, repo.ListFeedForAccount, accountID, scope)
		require.NoError(t, err)
		for _, item := range feed {
			if item.ID == announcementID {
				return true
			}
		}
		return false
	}

	letter := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, leaver.AccountID, leaver.TenantID,
		"Elternbrief 1a", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("1a")},
		}))
	// The publication records the children it reaches at this moment.
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.AnnouncementRecordRecipients, leaver.TenantID, letter.ID))

	// The children swap classes after the letter went out.
	moveStudentToClass(t, db, leaver.StudentID, "2b")
	moveStudentToClass(t, db, joiner.StudentID, "1a")

	stats, err := testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.TargetCount, "the letter still counts the parent it reached")
	assert.True(t, stats.Recorded, "the counts say they follow the recorded children")
	recipients, err := testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, recipients, 1, "no parent joins or leaves the recipient list")
	assert.Equal(t, leaver.AccountID, recipients[0].AccountID)

	// The parent who received it can still read and confirm it.
	matched, err := testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, leaver.TenantID, letter.ID, leaver.AccountID)
	require.NoError(t, err)
	assert.True(t, matched, "the guardian of a recorded child keeps the letter")
	assert.True(t, inFeed(leaver.AccountID, letter.ID), "the letter stays in the feed of the child who left 1a")
	ackApplied, err := testpkg.TenantQuery4(t, ctx, db, repo.MarkAcknowledged, leaver.TenantID, letter.ID, leaver.AccountID, *letter.PublishedAt)
	require.NoError(t, err)
	require.True(t, ackApplied)
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.AcknowledgedCount, "the confirmation of the recorded parent counts")

	// The parent of the child who joined 1a never got the letter.
	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, leaver.TenantID, letter.ID, joiner.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "a child who joined later is not reached")
	assert.False(t, inFeed(joiner.AccountID, letter.ID), "the new class does not get the old letter")

	// Who may read for the recorded child stays live: without an active
	// membership the parent drops out everywhere.
	_, err = db.NewUpdate().
		TableExpr("auth.account_tenants").
		Set("status = ?", "inactive").
		Where("account_id = ? AND tenant_id = ?", leaver.AccountID, leaver.TenantID).
		Exec(context.Background())
	require.NoError(t, err)
	matched, err = testpkg.TenantQuery3(t, ctx, db, repo.AccountMatchesAnnouncement, leaver.TenantID, letter.ID, leaver.AccountID)
	require.NoError(t, err)
	assert.False(t, matched, "access is still decided by the live relationship")
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TargetCount, "a parent without access is not counted")

	// A retraction drops the record; the draft shows the class of today.
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.SetPublished, letter.ID, (*time.Time)(nil)))
	stats, err = testpkg.TenantQuery2(t, ctx, db, repo.Stats, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	assert.False(t, stats.Recorded, "a draft counts the audience of today")
	recipients, err = testpkg.TenantQuery2(t, ctx, db, repo.AudienceRecipients, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, recipients, 1)
	assert.Equal(t, joiner.AccountID, recipients[0].AccountID, "an unrecorded announcement reaches the class of today")
}

// TestParentAnnouncementRecipients_LetterStatusKeepsTheRecordedChildren
// covers the Elternbrief status, which counts children: after a class change
// it keeps the child the letter went to and does not add the newcomer.
func TestParentAnnouncementRecipients_LetterStatusKeepsTheRecordedChildren(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	leaver := testpkg.CreateTestParentGuardianChain(t, db) // child in 1a
	joiner := testpkg.CreateTestParentGuardianChain(t, db)
	moveStudentToClass(t, db, joiner.StudentID, "2b")

	repo := api.NewPeopleRepositorySuiteAnnouncements(testpkg.NewDatabase(t, db))
	ctx := tenantCtx(t)
	letter := api.ParentStorePublishedAnnouncement(parentStoreFixtureChecks(t), ctx, func() time.Time {
		return databaseTimestamp(t, db)
	}, repo, leaver.AccountID, leaver.TenantID,
		"Elternbrief 1a", api.ParentStoreTargetValues([]api.ParentStoreTargetInput{
			{TargetType: userModels.AnnouncementTargetClass, TargetRefText: strp("1a")},
		}))
	require.NoError(t, testpkg.TenantCommand2(t, ctx, db, repo.AnnouncementRecordRecipients, leaver.TenantID, letter.ID))

	moveStudentToClass(t, db, leaver.StudentID, "2b")
	moveStudentToClass(t, db, joiner.StudentID, "1a")

	children, err := testpkg.TenantQuery2(t, ctx, db, repo.LetterChildStatuses, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, children, 1, "the letter keeps exactly the child it reached")
	assert.Equal(t, leaver.StudentID, children[0].StudentID)
	assert.True(t, children[0].CanConfirm, "its guardian can still confirm")
	reminded, err := testpkg.TenantQuery2(t, ctx, db, repo.UnacknowledgedReminderRecipients, leaver.TenantID, letter.ID)
	require.NoError(t, err)
	require.Len(t, reminded, 1, "a reminder goes to the recorded child's guardian only")
	assert.Equal(t, leaver.AccountID, reminded[0].AccountID)
}
