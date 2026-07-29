package parent_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	suggestionsService "github.com/moto-nrw/project-phoenix/services/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildFeedbackService wires the parent service with the real suggestions
// service behind it, which is what the feedback board delegates to.
func buildFeedbackService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
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
	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	foreignTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenant)
	defer testpkg.CleanupTenantTestData(t, db, foreignTenant)

	ctx := context.Background()

	_, err := svc.CreateFeedback(ctx, chain.AccountID, foreignTenant, "Titel", "Text")
	require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)

	_, err = svc.ListFeedback(ctx, chain.AccountID, foreignTenant, "newest")
	require.ErrorIs(t, err, parentService.ErrFeedbackSchoolNotAllowed)
}

// TestParentFeedbackRoundTrip covers the guardian's own board end to end: post,
// read back pseudonymously, comment, and see the reply counted.
func TestParentFeedbackRoundTrip(t *testing.T) {
	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
	defer testpkg.CleanupTenantTestData(t, db, chain.TenantID)

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
	svc, db := buildFeedbackService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	schools, err := svc.ListFeedbackSchools(context.Background(), chain.AccountID)
	require.NoError(t, err)
	require.Len(t, schools, 1)
	assert.Equal(t, chain.TenantID, schools[0].TenantID)
}
