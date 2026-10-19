package care_test

// Integration tests for the poll (Umfrage, #1371) half of the parent
// announcement feed: the per-child answer write with its audience/deadline
// guards, and the staff-facing evaluation the same repository serves. Uses a
// real guardian->student chain so the per-child authorization and the audience
// resolution exercise the repository SQL, not a mock.

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedPublishedPoll creates a school-wide poll with the given options, publishes
// it, and returns it with its persisted PublishedAt and option rows attached.
func seedPublishedPoll(
	t *testing.T,
	ctx context.Context,
	repo api.ParentAnnouncementFixtures,
	createdBy, tenantID int64,
	deadline *time.Time,
	labels ...string,
) *communication.ParentAnnouncementFixture {
	t.Helper()
	return seedPublishedPollWithTargets(t, ctx, repo, createdBy, tenantID, deadline,
		[]*communication.ParentAnnouncementTargetFixture{{TargetType: "school_all"}},
		labels...)
}

func seedPublishedPollWithTargets(
	t *testing.T,
	ctx context.Context,
	repo api.ParentAnnouncementFixtures,
	createdBy, tenantID int64,
	deadline *time.Time,
	targets []*communication.ParentAnnouncementTargetFixture,
	labels ...string,
) *communication.ParentAnnouncementFixture {
	t.Helper()
	a := &communication.ParentAnnouncementFixture{
		Title:            "Murmelparty",
		Body:             "Kommt Ihr Kind zur Murmelparty?",
		Priority:         "info",
		ResponseType:     "single_choice",
		ResponseDeadline: deadline,
		Active:           true,
		CreatedBy:        createdBy,
	}
	a.TenantID = tenantID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.Create, a))
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), repo.ReplaceTargets, tenantID, a.ID, targets))

	options := make([]*communication.ParentAnnouncementOptionFixture, 0, len(labels))
	for i, label := range labels {
		options = append(options, &communication.ParentAnnouncementOptionFixture{Label: label, Position: i})
	}
	require.NoError(t, testpkg.TenantCommand3(t, ctx, testpkg.SetupTestDB(t), repo.ReplaceOptions, tenantID, a.ID, options))

	now := time.Now().Add(-time.Minute)
	require.NoError(t, testpkg.TenantCommand2(t, ctx, testpkg.SetupTestDB(t), repo.SetPublished, a.ID, &now))
	t.Cleanup(func() { _ = testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.Delete, a.ID) })

	// Re-read so PublishedAt matches the DB value to the microsecond — the
	// service's version guard compares with Equal().
	persisted, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), repo.FindByID, a.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	a.PublishedAt = persisted.PublishedAt
	stored, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), repo.ListOptions, a.ID)
	require.NoError(t, err)
	a.Options = stored
	return a
}

func TestAnnouncementPoll_AnswerAppearsInFeedAndResults(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	poll := seedPublishedPollWithTargets(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, nil,
		[]*communication.ParentAnnouncementTargetFixture{{
			TargetType:  "student",
			TargetRefID: &chain.StudentID,
		}},
		"Ja", "Nein")
	require.Len(t, poll.Options, 2)

	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	// The feed carries the question, its options and the one child this guardian
	// may answer for — unanswered at first.
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	item := findFeedItem(t, feed, poll.ID)
	assert.Equal(t, "single_choice", item.ResponseType)
	require.Len(t, item.Options, 2)
	require.Len(t, item.Children, 1)
	assert.Equal(t, chain.StudentID, item.Children[0].StudentID)
	assert.Empty(t, item.Children[0].SelectedOptions)

	// Answering stamps the child's selection.
	yes := poll.Options[0].ID
	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{yes}, *poll.PublishedAt))

	feed, err = svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	item = findFeedItem(t, feed, poll.ID)
	require.Len(t, item.Children, 1)
	assert.Equal(t, []int64{yes}, item.Children[0].SelectedOptions)

	// The staff evaluation counts children, and the answered child drops out of
	// the reminder audience.
	results, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.PollResults, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, results.ChildCount)
	assert.Equal(t, 1, results.AnsweredCount)
	require.Len(t, results.Options, 2)
	assert.Equal(t, 1, results.Options[0].Count)
	assert.Equal(t, 0, results.Options[1].Count)

	children, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.PollChildren, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, []string{"Ja"}, children[0].AnswerLabels)

	pending, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.UnansweredReminderRecipients, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Empty(t, pending, "an answered child must not be reminded")
}

func TestAnnouncementPoll_CareEndedChildCannotRespond(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, nil, "Ja", "Nein")

	endCareFor(t, db, chain.StudentID)
	err := svc.RespondToAnnouncement(testpkg.WithTestTenantRuntime(t, context.Background()), chain.AccountID, poll.ID, chain.StudentID,
		[]int64{poll.Options[0].ID}, *poll.PublishedAt)
	require.ErrorIs(t, err, parentportal.ErrChildCareEnded)
}

// A portal-visible child without a guardian who may answer remains visible to
// staff, but cannot make completion or reminders look permanently outstanding.
func TestAnnouncementPoll_IneligibleChildIsNotOutstanding(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	_, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	poll := seedPublishedPollWithTargets(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, nil,
		[]*communication.ParentAnnouncementTargetFixture{{
			TargetType:  "student",
			TargetRefID: &chain.StudentID,
		}},
		"Ja", "Nein")
	_, err := db.ExecContext(testpkg.WithTestTenantRuntime(t, context.Background()), `
		UPDATE auth.guardian_student_access AS a SET permissions = '{"parent_portal.access": true}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id AND r.student_id = ? AND r.guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	results, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.PollResults, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, results.TargetChildCount)
	assert.Zero(t, results.ChildCount)
	assert.Zero(t, results.AnsweredCount)

	children, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.PollChildren, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.False(t, children[0].CanAnswer)

	pending, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.UnansweredReminderRecipients, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// Re-answering replaces the previous selection rather than adding a second vote:
// a single-choice poll must never count one child twice.
func TestAnnouncementPoll_ReAnswerReplacesSelection(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, nil, "Ja", "Nein")
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[0].ID}, *poll.PublishedAt))
	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[1].ID}, *poll.PublishedAt))

	results, err := testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.PollResults, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, results.AnsweredCount)
	assert.Equal(t, 0, results.Options[0].Count, "the first answer must be replaced")
	assert.Equal(t, 1, results.Options[1].Count)

	// An empty selection withdraws the answer entirely.
	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, nil, *poll.PublishedAt))
	results, err = testpkg.TenantQuery2(t, seedCtx, db, repos.ParentAnnouncement.PollResults, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, results.AnsweredCount)
}

// The deadline is the answer cut-off, checked in the same statement as the
// write. A closed poll rejects the answer instead of silently dropping it.
func TestAnnouncementPoll_ClosedPollRejectsAnswer(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	future := time.Now().Add(time.Hour)
	poll := seedPublishedPoll(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, &future, "Ja", "Nein")

	// Move the deadline into the past with a direct write: the service refuses to
	// SET one there, but a poll closing while a parent has the page open is
	// exactly the state this guard exists for.
	past := time.Now().Add(-time.Minute)
	_, err := db.NewUpdate().
		TableExpr("users.parent_announcements").
		Set("response_deadline = ?", past).
		Where("id = ?", poll.ID).
		Exec(testpkg.WithTestTenantRuntime(t, context.Background()))
	require.NoError(t, err)

	respondErr := svc.RespondToAnnouncement(testpkg.WithTestTenantRuntime(t, context.Background()), chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[0].ID}, *poll.PublishedAt)
	assert.ErrorIs(t, respondErr, parentportal.ErrPollClosed)
}

// A guardian may not answer for a child that is not theirs.
func TestAnnouncementPoll_ForeignChildIsRejected(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestStudent(t, db, "Mila", "Fremd", "2b")

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, nil, "Ja", "Nein")

	err := svc.RespondToAnnouncement(testpkg.WithTestTenantRuntime(t, context.Background()), chain.AccountID, poll.ID, other.ID, []int64{poll.Options[0].ID}, *poll.PublishedAt)
	assert.ErrorIs(t, err, parentportal.ErrChildNotAnswerable)
}

// A plain Mitteilung accepts no answers at all.
func TestAnnouncementPoll_PlainAnnouncementRejectsAnswer(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, false)

	err := svc.RespondToAnnouncement(testpkg.WithTestTenantRuntime(t, context.Background()), chain.AccountID, ann.ID, chain.StudentID, nil, *ann.PublishedAt)
	assert.ErrorIs(t, err, parentportal.ErrAnnouncementNotAPoll)
}

// The unread badge keeps counting an open poll the guardian has already read but
// not answered — that is the whole point of the reminder.
func TestAnnouncementPoll_UnansweredPollStaysInUnreadCount(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, api.NewParentAnnouncementFixtures(repos.ParentAnnouncement), chain.AccountID, chain.TenantID, nil, "Ja", "Nein")
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())

	require.NoError(t, svc.MarkAnnouncementRead(ctx, chain.AccountID, poll.ID, *poll.PublishedAt))
	count, err := svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a read but unanswered poll must stay outstanding")

	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[0].ID}, *poll.PublishedAt))
	count, err = svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "answering clears the outstanding state")
}

func findFeedItem(t *testing.T, feed []*parentportal.AnnouncementFeedItem, id int64) *parentportal.AnnouncementFeedItem {
	t.Helper()
	for _, item := range feed {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("announcement %d not found in feed", id)
	return nil
}
