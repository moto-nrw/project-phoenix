package parent_test

// Integration tests for the poll (Umfrage, #1371) half of the parent
// announcement feed: the per-child answer write with its audience/deadline
// guards, and the staff-facing evaluation the same repository serves. Uses a
// real guardian->student chain so the per-child authorization and the audience
// resolution exercise the repository SQL, not a mock.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// seedPublishedPoll creates a school-wide poll with the given options, publishes
// it, and returns it with its persisted PublishedAt and option rows attached.
func seedPublishedPoll(
	t *testing.T,
	ctx context.Context,
	repo usersModels.ParentAnnouncementRepository,
	createdBy, tenantID int64,
	deadline *time.Time,
	labels ...string,
) *usersModels.ParentAnnouncement {
	t.Helper()
	return seedPublishedPollWithTargets(t, ctx, repo, createdBy, tenantID, deadline,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}},
		labels...)
}

func seedPublishedPollWithTargets(
	t *testing.T,
	ctx context.Context,
	repo usersModels.ParentAnnouncementRepository,
	createdBy, tenantID int64,
	deadline *time.Time,
	targets []*usersModels.ParentAnnouncementTarget,
	labels ...string,
) *usersModels.ParentAnnouncement {
	t.Helper()
	a := &usersModels.ParentAnnouncement{
		Title:            "Murmelparty",
		Body:             "Kommt Ihr Kind zur Murmelparty?",
		Priority:         usersModels.ParentAnnouncementPriorityInfo,
		ResponseType:     usersModels.ParentAnnouncementResponseSingleChoice,
		ResponseDeadline: deadline,
		Active:           true,
		CreatedBy:        createdBy,
	}
	a.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID, targets))

	options := make([]*usersModels.ParentAnnouncementOption, 0, len(labels))
	for i, label := range labels {
		options = append(options, &usersModels.ParentAnnouncementOption{Label: label, Position: i})
	}
	require.NoError(t, repo.ReplaceOptions(ctx, tenantID, a.ID, options))

	now := time.Now()
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	t.Cleanup(func() { _ = repo.Delete(ctx, a.ID) })

	// Re-read so PublishedAt matches the DB value to the microsecond — the
	// service's version guard compares with Equal().
	persisted, err := repo.FindByID(ctx, a.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	a.PublishedAt = persisted.PublishedAt
	stored, err := repo.ListOptions(ctx, a.ID)
	require.NoError(t, err)
	a.Options = stored
	return a
}

func TestAnnouncementPoll_AnswerAppearsInFeedAndResults(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	poll := seedPublishedPollWithTargets(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, nil,
		[]*usersModels.ParentAnnouncementTarget{{
			TargetType:  usersModels.AnnouncementTargetStudent,
			TargetRefID: &chain.StudentID,
		}},
		"Ja", "Nein")
	require.Len(t, poll.Options, 2)

	ctx := context.Background()

	// The feed carries the question, its options and the one child this guardian
	// may answer for — unanswered at first.
	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	item := findFeedItem(t, feed, poll.ID)
	assert.Equal(t, usersModels.ParentAnnouncementResponseSingleChoice, item.ResponseType)
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
	results, err := repos.ParentAnnouncement.PollResults(seedCtx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, results.ChildCount)
	assert.Equal(t, 1, results.AnsweredCount)
	require.Len(t, results.Options, 2)
	assert.Equal(t, 1, results.Options[0].Count)
	assert.Equal(t, 0, results.Options[1].Count)

	children, err := repos.ParentAnnouncement.PollChildren(seedCtx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.Equal(t, []string{"Ja"}, children[0].AnswerLabels)

	pending, err := repos.ParentAnnouncement.UnansweredReminderRecipients(seedCtx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Empty(t, pending, "an answered child must not be reminded")
}

func TestAnnouncementPoll_CareEndedChildCannotRespond(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, nil, "Ja", "Nein")

	endCareFor(t, db, chain.StudentID)
	err := svc.RespondToAnnouncement(context.Background(), chain.AccountID, poll.ID, chain.StudentID,
		[]int64{poll.Options[0].ID}, *poll.PublishedAt)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)
}

// A portal-visible child without a guardian who may answer remains visible to
// staff, but cannot make completion or reminders look permanently outstanding.
func TestAnnouncementPoll_IneligibleChildIsNotOutstanding(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	_, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	poll := seedPublishedPollWithTargets(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, nil,
		[]*usersModels.ParentAnnouncementTarget{{
			TargetType:  usersModels.AnnouncementTargetStudent,
			TargetRefID: &chain.StudentID,
		}},
		"Ja", "Nein")
	_, err := db.NewUpdate().
		TableExpr("users.students_guardians").
		Set("permissions = ?", `{"parent_portal.access": true}`).
		Where("student_id = ?", chain.StudentID).
		Where("guardian_profile_id = ?", chain.GuardianProfileID).
		Exec(context.Background())
	require.NoError(t, err)

	results, err := repos.ParentAnnouncement.PollResults(seedCtx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, results.TargetChildCount)
	assert.Zero(t, results.ChildCount)
	assert.Zero(t, results.AnsweredCount)

	children, err := repos.ParentAnnouncement.PollChildren(seedCtx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	assert.False(t, children[0].CanAnswer)

	pending, err := repos.ParentAnnouncement.UnansweredReminderRecipients(seedCtx, chain.TenantID, poll.ID)
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

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, nil, "Ja", "Nein")
	ctx := context.Background()

	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[0].ID}, *poll.PublishedAt))
	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[1].ID}, *poll.PublishedAt))

	results, err := repos.ParentAnnouncement.PollResults(seedCtx, chain.TenantID, poll.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, results.AnsweredCount)
	assert.Equal(t, 0, results.Options[0].Count, "the first answer must be replaced")
	assert.Equal(t, 1, results.Options[1].Count)

	// An empty selection withdraws the answer entirely.
	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, nil, *poll.PublishedAt))
	results, err = repos.ParentAnnouncement.PollResults(seedCtx, chain.TenantID, poll.ID)
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

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	future := time.Now().Add(time.Hour)
	poll := seedPublishedPoll(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, &future, "Ja", "Nein")

	// Move the deadline into the past with a direct write: the service refuses to
	// SET one there, but a poll closing while a parent has the page open is
	// exactly the state this guard exists for.
	past := time.Now().Add(-time.Minute)
	_, err := db.NewUpdate().
		TableExpr("users.parent_announcements").
		Set("response_deadline = ?", past).
		Where("id = ?", poll.ID).
		Exec(context.Background())
	require.NoError(t, err)

	respondErr := svc.RespondToAnnouncement(context.Background(), chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[0].ID}, *poll.PublishedAt)
	assert.ErrorIs(t, respondErr, parentService.ErrPollClosed)
}

// A guardian may not answer for a child that is not theirs.
func TestAnnouncementPoll_ForeignChildIsRejected(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestStudent(t, db, "Mila", "Fremd", "2b")

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, nil, "Ja", "Nein")

	err := svc.RespondToAnnouncement(context.Background(), chain.AccountID, poll.ID, other.ID, []int64{poll.Options[0].ID}, *poll.PublishedAt)
	assert.ErrorIs(t, err, parentService.ErrChildNotAnswerable)
}

// A plain Mitteilung accepts no answers at all.
func TestAnnouncementPoll_PlainAnnouncementRejectsAnswer(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	ann := seedPublishedAnnouncement(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, false)

	err := svc.RespondToAnnouncement(context.Background(), chain.AccountID, ann.ID, chain.StudentID, nil, *ann.PublishedAt)
	assert.ErrorIs(t, err, parentService.ErrAnnouncementNotAPoll)
}

// The unread badge keeps counting an open poll the guardian has already read but
// not answered — that is the whole point of the reminder.
func TestAnnouncementPoll_UnansweredPollStaysInUnreadCount(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), chain.TenantID)
	poll := seedPublishedPoll(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID, nil, "Ja", "Nein")
	ctx := context.Background()

	require.NoError(t, svc.MarkAnnouncementRead(ctx, chain.AccountID, poll.ID, *poll.PublishedAt))
	count, err := svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "a read but unanswered poll must stay outstanding")

	require.NoError(t, svc.RespondToAnnouncement(ctx, chain.AccountID, poll.ID, chain.StudentID, []int64{poll.Options[0].ID}, *poll.PublishedAt))
	count, err = svc.UnreadAnnouncementCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "answering clears the outstanding state")
}

func findFeedItem(t *testing.T, feed []*usersModels.AnnouncementFeedItem, id int64) *usersModels.AnnouncementFeedItem {
	t.Helper()
	for _, item := range feed {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("announcement %d not found in feed", id)
	return nil
}
