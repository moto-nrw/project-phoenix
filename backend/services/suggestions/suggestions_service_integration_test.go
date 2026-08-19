package suggestions_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repoSuggestions "github.com/moto-nrw/project-phoenix/database/repositories/suggestions"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	suggestionsService "github.com/moto-nrw/project-phoenix/services/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testAccount holds account and person IDs for integration tests.
type testAccount struct {
	AccountID int64
	PersonID  int64
}

// setupIntegrationService creates a real service backed by the test database.
// Returns the DB (for direct queries), the service, and a test account.
func setupIntegrationService(t *testing.T) (*bun.DB, suggestionsService.Service, *testAccount) {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	postRepo := repoSuggestions.NewPostRepository(db)
	voteRepo := repoSuggestions.NewVoteRepository(db)
	commentRepo := repoSuggestions.NewCommentRepository(db)
	commentReadRepo := repoSuggestions.NewCommentReadRepository(db)

	svc := suggestionsService.NewService(suggestionsService.ServiceConfig{
		PostRepo:        postRepo,
		VoteRepo:        voteRepo,
		CommentRepo:     commentRepo,
		CommentReadRepo: commentReadRepo,
		DB:              db,
	})

	account := testpkg.CreateTestAccount(t, db, "svc-vote-test")
	person := testpkg.CreateTestPersonWithAccountID(t, db, "Vote", "Tester", account.ID)

	t.Cleanup(func() {
		cleanupAllSuggestionData(t, db)
		testpkg.CleanupTableRecords(t, db, "users.persons", person.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	return db, svc, &testAccount{AccountID: account.ID, PersonID: person.ID}
}

// createExtraAccount creates an additional account+person for multi-voter tests.
func createExtraAccount(t *testing.T, db *bun.DB, prefix string) *testAccount {
	t.Helper()

	account := testpkg.CreateTestAccount(t, db, prefix)
	person := testpkg.CreateTestPersonWithAccountID(t, db, prefix, "Voter", account.ID)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "users.persons", person.ID)
		testpkg.CleanupAuthFixtures(t, db, account.ID)
	})

	return &testAccount{AccountID: account.ID, PersonID: person.ID}
}

// cleanupAllSuggestionData removes all test votes and posts from the suggestions schema.
func cleanupAllSuggestionData(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 5*time.Second)
	defer cancel()

	// Delete votes first (FK)
	_, err := db.NewDelete().
		TableExpr("suggestions.votes").
		Where("TRUE").
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup suggestions.votes: %v", err)
	}

	_, err = db.NewDelete().
		TableExpr("suggestions.posts").
		Where("TRUE").
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup suggestions.posts: %v", err)
	}
}

func TestIntegration_Vote_Success(t *testing.T) {
	_, svc, acct := setupIntegrationService(t)
	ctx := testpkg.TenantContext(1)

	// Create a post via the service
	post := &suggestions.Post{
		Title:       fmt.Sprintf("Vote Success %d", time.Now().UnixNano()),
		Description: "Test post for voting",
		AuthorID:    acct.AccountID,
	}
	require.NoError(t, svc.CreatePost(ctx, post))

	// Vote up
	result, err := svc.Vote(ctx, post.ID, acct.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.Score)
	assert.Equal(t, 1, result.Upvotes)
	assert.Equal(t, 0, result.Downvotes)
	require.NotNil(t, result.UserVote)
	assert.Equal(t, suggestions.DirectionUp, *result.UserVote)
}

func TestIntegration_Vote_ChangeDirection(t *testing.T) {
	_, svc, acct := setupIntegrationService(t)
	ctx := testpkg.TenantContext(1)

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Vote Change %d", time.Now().UnixNano()),
		Description: "Test post for changing vote direction",
		AuthorID:    acct.AccountID,
	}
	require.NoError(t, svc.CreatePost(ctx, post))

	// Vote up first
	_, err := svc.Vote(ctx, post.ID, acct.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)

	// Change to down (upsert should overwrite, not insert duplicate)
	result, err := svc.Vote(ctx, post.ID, acct.AccountID, suggestions.DirectionDown)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, -1, result.Score)
	assert.Equal(t, 0, result.Upvotes)
	assert.Equal(t, 1, result.Downvotes)
	require.NotNil(t, result.UserVote)
	assert.Equal(t, suggestions.DirectionDown, *result.UserVote)
}

func TestIntegration_Vote_MultipleVoters(t *testing.T) {
	db, svc, acct1 := setupIntegrationService(t)
	ctx := testpkg.TenantContext(1)

	acct2 := createExtraAccount(t, db, "voter2")
	acct3 := createExtraAccount(t, db, "voter3")

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Multi Voter %d", time.Now().UnixNano()),
		Description: "Test post for multiple voters",
		AuthorID:    acct1.AccountID,
	}
	require.NoError(t, svc.CreatePost(ctx, post))

	// Account1 votes up
	_, err := svc.Vote(ctx, post.ID, acct1.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)

	// Account2 votes up
	_, err = svc.Vote(ctx, post.ID, acct2.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)

	// Account3 votes down
	result, err := svc.Vote(ctx, post.ID, acct3.AccountID, suggestions.DirectionDown)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 2 up - 1 down = score 1
	assert.Equal(t, 1, result.Score)
	assert.Equal(t, 2, result.Upvotes)
	assert.Equal(t, 1, result.Downvotes)
}

func TestIntegration_RemoveVote_Success(t *testing.T) {
	_, svc, acct := setupIntegrationService(t)
	ctx := testpkg.TenantContext(1)

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Remove Vote %d", time.Now().UnixNano()),
		Description: "Test post for removing vote",
		AuthorID:    acct.AccountID,
	}
	require.NoError(t, svc.CreatePost(ctx, post))

	// Vote up
	_, err := svc.Vote(ctx, post.ID, acct.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)

	// Remove vote
	result, err := svc.RemoveVote(ctx, post.ID, acct.AccountID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.Score)
	assert.Equal(t, 0, result.Upvotes)
	assert.Equal(t, 0, result.Downvotes)
	assert.Nil(t, result.UserVote)
}

func TestIntegration_RemoveVote_NoExistingVote(t *testing.T) {
	_, svc, acct := setupIntegrationService(t)
	ctx := testpkg.TenantContext(1)

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Remove No Vote %d", time.Now().UnixNano()),
		Description: "Test post for removing non-existent vote",
		AuthorID:    acct.AccountID,
	}
	require.NoError(t, svc.CreatePost(ctx, post))

	// Remove vote when none exists — should not error
	result, err := svc.RemoveVote(ctx, post.ID, acct.AccountID)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.Score)
	assert.Nil(t, result.UserVote)
}

func TestIntegration_Vote_Atomicity(t *testing.T) {
	db, svc, acct1 := setupIntegrationService(t)
	ctx := testpkg.TenantContext(1)

	acct2 := createExtraAccount(t, db, "atomic-voter")

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Atomicity %d", time.Now().UnixNano()),
		Description: "Test post for atomicity",
		AuthorID:    acct1.AccountID,
	}
	require.NoError(t, svc.CreatePost(ctx, post))

	// Two accounts vote up sequentially (verifies upsert+recalculate both commit)
	_, err := svc.Vote(ctx, post.ID, acct1.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)

	result, err := svc.Vote(ctx, post.ID, acct2.AccountID, suggestions.DirectionUp)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Both votes should be counted (no lost updates from partial transaction commits)
	assert.Equal(t, 2, result.Score)
	assert.Equal(t, 2, result.Upvotes)
	assert.Equal(t, 0, result.Downvotes)
}
