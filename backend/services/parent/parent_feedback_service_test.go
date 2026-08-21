package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	suggestionsModels "github.com/moto-nrw/project-phoenix/models/suggestions"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	suggestionsService "github.com/moto-nrw/project-phoenix/services/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildFeedbackService wires the parent service with the real suggestions
// service behind it, which is what the feedback board delegates to.
func buildFeedbackService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: repos.ParentChild,
		Suggestions: suggestionsService.NewService(suggestionsService.ServiceConfig{
			PostRepo:        repos.SuggestionPost,
			VoteRepo:        repos.SuggestionVote,
			CommentRepo:     repos.SuggestionComment,
			CommentReadRepo: repos.SuggestionCommentRead,
			DB:              db,
			Logger:          slog.Default(),
		}),
		DB:     db,
		Logger: slog.Default(),
	})
	return svc, db
}

// TestParentFeedbackRequiresOwnSchool: the school a guardian addresses is not
// taken on trust from the client. Without this check a parent could post onto
// (and read) the board of any school whose id they guessed.
func TestParentFeedbackRequiresOwnSchool(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)

	ctx := context.Background()

	_, err := svc.CreateFeedback(ctx, chain.AccountID, foreignTenant, "Titel", "Text")
	require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)

	_, err = svc.ListFeedback(ctx, chain.AccountID, foreignTenant, "newest")
	require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)
}

// TestParentFeedbackRoundTrip covers the guardian's own board end to end: post,
// read back pseudonymously, comment, and see the reply counted.
func TestParentFeedbackRoundTrip(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	created, err := svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID,
		"Push-Nachrichten", "Damit ich Krankmeldungen schneller sehe.")
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, "Elternteil", created.AuthorName,
		"the guardian's real name must never come back out of the service")
	assert.Equal(t, chain.AccountID, created.AuthorID)

	posts, err := svc.ListFeedback(ctx, chain.AccountID, chain.TenantID, "newest")
	require.NoError(t, err)
	require.NotEmpty(t, posts)
	assert.Equal(t, created.ID, posts[0].ID)

	require.NoError(t, svc.CreateFeedbackComment(ctx, chain.AccountID, chain.TenantID,
		created.ID, "Ergänzung: am liebsten auch per Mail."))

	comments, err := svc.ListFeedbackComments(ctx, chain.AccountID, chain.TenantID, created.ID)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "Verfasser", comments[0].AuthorName,
		"a reply from the author of the entry is labelled as such, not as a stranger")
}

// TestParentFeedbackSchoolsFollowChildren: the picker offers exactly the
// schools the guardian actually has a child at.
func TestParentFeedbackSchoolsFollowChildren(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	schools, err := svc.ListFeedbackSchools(context.Background(), chain.AccountID)
	require.NoError(t, err)
	require.Len(t, schools, 1)
	assert.Equal(t, chain.TenantID, schools[0].TenantID)
}

// TestParentFeedbackEntryLifecycle walks one entry through every state a
// guardian can put it in. Read-back after each write matters here because the
// service returns a freshly fetched row rather than the one it sent in.
func TestParentFeedbackEntryLifecycle(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	created, err := svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID,
		"Abholzeiten im Kalender", "Damit ich den Wochenplan im Handy sehe.")
	require.NoError(t, err)
	require.NotNil(t, created)

	fetched, err := svc.GetFeedback(ctx, chain.AccountID, chain.TenantID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Abholzeiten im Kalender", fetched.Title)
	assert.Equal(t, "Elternteil", fetched.AuthorName)

	updated, err := svc.UpdateFeedback(ctx, chain.AccountID, chain.TenantID, created.ID,
		"Abholzeiten im Kalender abonnieren", "Am liebsten als Abo für den Handy-Kalender.")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Abholzeiten im Kalender abonnieren", updated.Title)
	assert.Equal(t, "Am liebsten als Abo für den Handy-Kalender.", updated.Description)

	require.NoError(t, svc.DeleteFeedback(ctx, chain.AccountID, chain.TenantID, created.ID))

	_, err = svc.GetFeedback(ctx, chain.AccountID, chain.TenantID, created.ID)
	require.Error(t, err, "a deleted entry must not be readable again")
}

// TestParentFeedbackVoting: the score the board sorts by is recalculated in the
// database, so each step asserts the value that comes back out rather than the
// one the client sent.
func TestParentFeedbackVoting(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	post, err := svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID,
		"Krankmeldung per Knopfdruck", "Zwei Klicks statt Anruf im Sekretariat.")
	require.NoError(t, err)
	require.NotNil(t, post)

	voted, err := svc.VoteFeedback(ctx, chain.AccountID, chain.TenantID, post.ID, "up")
	require.NoError(t, err)
	require.NotNil(t, voted)
	assert.Equal(t, 1, voted.Score)
	assert.Equal(t, 1, voted.Upvotes)
	require.NotNil(t, voted.UserVote)
	assert.Equal(t, "up", *voted.UserVote)

	// Changing the direction replaces the vote instead of adding a second one.
	changed, err := svc.VoteFeedback(ctx, chain.AccountID, chain.TenantID, post.ID, "down")
	require.NoError(t, err)
	require.NotNil(t, changed)
	assert.Equal(t, -1, changed.Score)
	assert.Equal(t, 0, changed.Upvotes)
	assert.Equal(t, 1, changed.Downvotes)

	withdrawn, err := svc.RemoveFeedbackVote(ctx, chain.AccountID, chain.TenantID, post.ID)
	require.NoError(t, err)
	require.NotNil(t, withdrawn)
	assert.Equal(t, 0, withdrawn.Score)
	assert.Nil(t, withdrawn.UserVote)

	_, err = svc.VoteFeedback(ctx, chain.AccountID, chain.TenantID, post.ID, "sideways")
	require.Error(t, err, "only up and down are votes")
}

// TestParentFeedbackThreadReadState covers the badge path: a reply is unread
// until the guardian opens the thread, and the sidebar count has to drop with it.
func TestParentFeedbackThreadReadState(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	post, err := svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID,
		"Mensa-Plan als Aushang", "Bitte auch für die nächste Woche.")
	require.NoError(t, err)
	require.NotNil(t, post)

	require.NoError(t, svc.CreateFeedbackComment(ctx, chain.AccountID, chain.TenantID,
		post.ID, "Ergänzung: bitte mit Allergenen."))

	unreadBefore, err := svc.FeedbackUnreadCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 0, unreadBefore, "a guardian's own comment is not a product-team reply")

	withReply, err := svc.GetFeedback(ctx, chain.AccountID, chain.TenantID, post.ID)
	require.NoError(t, err)
	require.NotNil(t, withReply)
	assert.Equal(t, 1, withReply.CommentCount)
	assert.Equal(t, 0, withReply.UnreadCount)

	operatorReply := &suggestionsModels.Comment{
		PostID:     post.ID,
		AuthorID:   chain.AccountID,
		AuthorType: suggestionsModels.AuthorTypeOperator,
		Content:    "Danke, wir schauen uns das an.",
	}
	operatorReply.SetTenantID(chain.TenantID)
	_, err = db.NewInsert().
		Model(operatorReply).
		ModelTableExpr("suggestions.comments").
		Exec(ctx)
	require.NoError(t, err)

	unreadBefore, err = svc.FeedbackUnreadCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, unreadBefore, "a product-team reply must show up in the badge")

	withReply, err = svc.GetFeedback(ctx, chain.AccountID, chain.TenantID, post.ID)
	require.NoError(t, err)
	require.NotNil(t, withReply)
	assert.Equal(t, 2, withReply.CommentCount)
	assert.Equal(t, 1, withReply.UnreadCount)

	require.NoError(t, svc.MarkFeedbackCommentsRead(ctx, chain.AccountID, chain.TenantID, post.ID))

	read, err := svc.GetFeedback(ctx, chain.AccountID, chain.TenantID, post.ID)
	require.NoError(t, err)
	require.NotNil(t, read)
	assert.Equal(t, 0, read.UnreadCount, "opening the thread clears its marker")

	unreadAfter, err := svc.FeedbackUnreadCount(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, unreadBefore-1, unreadAfter,
		"marking one thread read must clear exactly that thread's replies")

	comments, err := svc.ListFeedbackComments(ctx, chain.AccountID, chain.TenantID, post.ID)
	require.NoError(t, err)
	require.Len(t, comments, 2)

	var ownCommentID int64
	for _, comment := range comments {
		if comment.AuthorType == suggestionsModels.AuthorTypeParent {
			ownCommentID = comment.ID
			break
		}
	}
	require.NotZero(t, ownCommentID)

	otherPost, err := svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID,
		"Andere Rückmeldung", "Ein anderer Thread.")
	require.NoError(t, err)
	require.Error(t, svc.DeleteFeedbackComment(ctx, chain.AccountID, chain.TenantID, otherPost.ID, ownCommentID),
		"a comment must not be deletable through a different thread URL")

	require.NoError(t, svc.DeleteFeedbackComment(ctx, chain.AccountID, chain.TenantID, post.ID, ownCommentID))

	remaining, err := svc.ListFeedbackComments(ctx, chain.AccountID, chain.TenantID, post.ID)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, suggestionsModels.AuthorTypeOperator, remaining[0].AuthorType)
}

// TestParentFeedbackRejectsUnusableIdentifiers: every entry point takes the
// account and school id from the request, so a missing one has to be refused
// before any transaction opens — an unscoped query is the leak this prevents.
func TestParentFeedbackRejectsUnusableIdentifiers(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	t.Run("missing school", func(t *testing.T) {
		_, err := svc.ListFeedback(ctx, chain.AccountID, 0, "score")
		require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)
	})

	t.Run("missing account", func(t *testing.T) {
		_, err := svc.ListFeedback(ctx, 0, chain.TenantID, "score")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account_id")
	})

	t.Run("missing account on the school picker", func(t *testing.T) {
		_, err := svc.ListFeedbackSchools(ctx, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account_id")
	})

	t.Run("missing account on the badge", func(t *testing.T) {
		_, err := svc.FeedbackUnreadCount(ctx, 0)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account_id")
	})

	t.Run("foreign school on a write", func(t *testing.T) {
		foreignTenant := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, foreignTenant)

		err := svc.CreateFeedbackComment(ctx, chain.AccountID, foreignTenant, 1, "Hallo")
		require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)

		err = svc.MarkFeedbackCommentsRead(ctx, chain.AccountID, foreignTenant, 1)
		require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)
	})
}

// TestParentFeedbackWithoutSuggestionsWiring: the board is optional wiring, and
// a missing dependency is a startup fault. It must surface as one instead of
// panicking in a request.
func TestParentFeedbackWithoutSuggestionsWiring(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: repos.ParentChild,
		DB:        db,
		Logger:    slog.Default(),
	})

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	_, err := svc.ListFeedback(ctx, chain.AccountID, chain.TenantID, "score")
	require.ErrorIs(t, err, parentService.ErrFeedbackNotWired)

	_, err = svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID, "Titel", "Text")
	require.ErrorIs(t, err, parentService.ErrFeedbackNotWired)

	_, err = svc.FeedbackUnreadCount(ctx, chain.AccountID)
	require.ErrorIs(t, err, parentService.ErrFeedbackNotWired)

	// The picker is derived from the guardian's children, not from the board, so
	// it keeps working even without the suggestions service behind it.
	schools, err := svc.ListFeedbackSchools(ctx, chain.AccountID)
	require.NoError(t, err)
	assert.Len(t, schools, 1)
}

// TestParentFeedbackSchoolsDedupeAndSort: the picker is derived from the
// guardian's children, and a guardian typically has several. Two children at one
// school must collapse into one entry, and the order has to be stable by name —
// otherwise the picker reshuffles between requests.
func TestParentFeedbackSchoolsDedupeAndSort(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	musterstadtTenantID := testpkg.UniqueTestTenantID(t)
	altdorfTenantID := testpkg.UniqueTestTenantID(t)
	repo := &stubChildRepo{
		result: []*parentModels.ChildSummary{
			{StudentID: 5001, TenantID: musterstadtTenantID, FirstName: "Lara", SchoolName: "GS Musterstadt"},
			{StudentID: 5002, TenantID: altdorfTenantID, FirstName: "Ben", SchoolName: "GS Altdorf"},
			{StudentID: 5003, TenantID: musterstadtTenantID, FirstName: "Mia", SchoolName: "GS Musterstadt"},
			nil,
		},
	}

	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: repo,
		DB:        db,
		Logger:    slog.Default(),
	})

	schools, err := svc.ListFeedbackSchools(context.Background(), 4321)
	require.NoError(t, err)
	require.Len(t, schools, 2, "siblings at the same school must not appear twice")
	assert.Equal(t, "GS Altdorf", schools[0].SchoolName)
	assert.Equal(t, int64(altdorfTenantID), schools[0].TenantID)
	assert.Equal(t, "GS Musterstadt", schools[1].SchoolName)
	assert.Equal(t, int64(musterstadtTenantID), schools[1].TenantID)
}

// TestParentFeedbackRejectsEmptyEntry: validation lives in the suggestions
// service, so the parent path has to surface its error rather than swallow it
// and report a created entry that never landed.
func TestParentFeedbackRejectsEmptyEntry(t *testing.T) {
	t.Parallel()

	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()

	_, err := svc.CreateFeedback(ctx, chain.AccountID, chain.TenantID, "   ", "Ohne Titel")
	require.Error(t, err, "an entry without a title must not be stored")

	_, err = svc.UpdateFeedback(ctx, chain.AccountID, chain.TenantID, 999999,
		"Titel", "Ein Beitrag, den es nicht gibt")
	require.Error(t, err, "editing a nonexistent entry must fail instead of creating one")
}

// TestParentFeedbackChildLookupFailurePropagates: the school check is what
// authorizes the whole board. If resolving the guardian's children fails, the
// request must fail too — treating an error as "no children" would be a
// fail-open authorization bug.
func TestParentFeedbackChildLookupFailurePropagates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	lookupErr := errors.New("children unavailable")
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: &stubChildRepo{err: lookupErr},
		Suggestions: suggestionsService.NewService(suggestionsService.ServiceConfig{
			PostRepo:        repos.SuggestionPost,
			VoteRepo:        repos.SuggestionVote,
			CommentRepo:     repos.SuggestionComment,
			CommentReadRepo: repos.SuggestionCommentRead,
			DB:              db,
			Logger:          slog.Default(),
		}),
		DB:     db,
		Logger: slog.Default(),
	})

	ctx := context.Background()

	_, err := svc.ListFeedback(ctx, 4321, 8801, "score")
	require.ErrorIs(t, err, lookupErr)
	assert.NotErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed,
		"a lookup failure must not be reported as a denied school")

	_, err = svc.ListFeedbackSchools(ctx, 4321)
	require.ErrorIs(t, err, lookupErr)

	_, err = svc.FeedbackUnreadCount(ctx, 4321)
	require.ErrorIs(t, err, lookupErr)
}

// TestParentFeedbackUnreadCountRejectsUnusableSchool: the badge iterates the
// schools it derived itself. A child without a school id would open an unscoped
// transaction, so the helper has to refuse instead.
func TestParentFeedbackUnreadCountRejectsUnusableSchool(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: &stubChildRepo{
			result: []*parentModels.ChildSummary{
				{StudentID: 5001, TenantID: 0, FirstName: "Lara", SchoolName: "Ohne Schule"},
			},
		},
		Suggestions: suggestionsService.NewService(suggestionsService.ServiceConfig{
			PostRepo:        repos.SuggestionPost,
			VoteRepo:        repos.SuggestionVote,
			CommentRepo:     repos.SuggestionComment,
			CommentReadRepo: repos.SuggestionCommentRead,
			DB:              db,
			Logger:          slog.Default(),
		}),
		DB:     db,
		Logger: slog.Default(),
	})

	_, err := svc.FeedbackUnreadCount(context.Background(), 4321)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unread count")
}
