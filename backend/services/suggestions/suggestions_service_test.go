package suggestions_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	svc "github.com/moto-nrw/project-phoenix/services/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupService creates a suggestions service backed by real repositories and test DB.
func setupService(t *testing.T, db *bun.DB) svc.Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	return svc.NewService(repoFactory.SuggestionPost, repoFactory.SuggestionVote, db)
}

// createTestPost inserts a post directly for test setup.
func createTestPost(t *testing.T, db *bun.DB, accountID int64, title, desc string) *suggestions.Post {
	t.Helper()

	post := &suggestions.Post{
		Title:       title,
		Description: desc,
		AuthorID:    accountID,
		Status:      suggestions.StatusOpen,
		Score:       0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewInsert().
		Model(post).
		ModelTableExpr("suggestions.posts").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err)

	return post
}

// cleanupPosts removes test posts and their votes.
func cleanupPosts(t *testing.T, db *bun.DB, postIDs ...int64) {
	t.Helper()
	if len(postIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = db.NewDelete().
		TableExpr("suggestions.votes").
		Where("post_id IN (?)", bun.In(postIDs)).
		Exec(ctx)

	_, _ = db.NewDelete().
		TableExpr("suggestions.posts").
		Where("id IN (?)", bun.In(postIDs)).
		Exec(ctx)
}

// ============================================================================
// CreatePost Tests
// ============================================================================

func TestCreatePost_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-create")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Service Create %d", time.Now().UnixNano()),
		Description: "Created via service",
		AuthorID:    account.ID,
	}

	err := service.CreatePost(ctx, post)
	require.NoError(t, err)
	assert.Greater(t, post.ID, int64(0))
	defer cleanupPosts(t, db, post.ID)

	// Verify status and score are forced
	assert.Equal(t, suggestions.StatusOpen, post.Status)
	assert.Equal(t, 0, post.Score)
}

func TestCreatePost_NilPost(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	err := service.CreatePost(context.Background(), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrInvalidData))
}

func TestCreatePost_ValidationError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	post := &suggestions.Post{
		Title:       "",
		Description: "Missing title",
		AuthorID:    1,
	}

	err := service.CreatePost(context.Background(), post)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrInvalidData))
}

func TestCreatePost_ForcesOpenStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-force-open")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := &suggestions.Post{
		Title:       fmt.Sprintf("Force Open %d", time.Now().UnixNano()),
		Description: "Should be open",
		AuthorID:    account.ID,
		Status:      suggestions.StatusDone, // Try to set done
		Score:       99,                     // Try to set high score
	}

	err := service.CreatePost(ctx, post)
	require.NoError(t, err)
	defer cleanupPosts(t, db, post.ID)

	assert.Equal(t, suggestions.StatusOpen, post.Status)
	assert.Equal(t, 0, post.Score)
}

// ============================================================================
// GetPost Tests
// ============================================================================

func TestGetPost_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-get")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Get %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	found, err := service.GetPost(ctx, post.ID, account.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, post.ID, found.ID)
}

func TestGetPost_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	_, err := service.GetPost(context.Background(), 999999999, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrPostNotFound))
}

// ============================================================================
// UpdatePost Tests
// ============================================================================

func TestUpdatePost_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-update")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Update %d", time.Now().UnixNano()), "Original")
	defer cleanupPosts(t, db, post.ID)

	update := &suggestions.Post{
		Title:       "Updated Title",
		Description: "Updated Description",
	}
	update.ID = post.ID

	err := service.UpdatePost(ctx, update, account.ID)
	require.NoError(t, err)

	found, err := service.GetPost(ctx, post.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", found.Title)
	assert.Equal(t, "Updated Description", found.Description)
}

func TestUpdatePost_NilPost(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	err := service.UpdatePost(context.Background(), nil, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrInvalidData))
}

func TestUpdatePost_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	update := &suggestions.Post{
		Title:       "Title",
		Description: "Desc",
	}
	update.ID = 999999999

	err := service.UpdatePost(context.Background(), update, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrPostNotFound))
}

func TestUpdatePost_Forbidden_NotAuthor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	author := testpkg.CreateTestAccount(t, db, "svc-update-author")
	other := testpkg.CreateTestAccount(t, db, "svc-update-other")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, other.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("Forbidden %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	update := &suggestions.Post{
		Title:       "Hacked Title",
		Description: "Hacked Desc",
	}
	update.ID = post.ID

	err := service.UpdatePost(ctx, update, other.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrForbidden))
}

func TestUpdatePost_ValidationError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-update-invalid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("ValError %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	update := &suggestions.Post{
		Title:       "", // Empty title
		Description: "Desc",
	}
	update.ID = post.ID

	err := service.UpdatePost(ctx, update, account.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrInvalidData))
}

// ============================================================================
// DeletePost Tests
// ============================================================================

func TestDeletePost_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-delete")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Delete %d", time.Now().UnixNano()), "Desc")
	// No defer cleanup since we're deleting

	err := service.DeletePost(ctx, post.ID, account.ID)
	require.NoError(t, err)

	_, err = service.GetPost(ctx, post.ID, account.ID)
	assert.True(t, errors.Is(err, svc.ErrPostNotFound))
}

func TestDeletePost_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	err := service.DeletePost(context.Background(), 999999999, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrPostNotFound))
}

func TestDeletePost_Forbidden_NotAuthor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	author := testpkg.CreateTestAccount(t, db, "svc-del-author")
	other := testpkg.CreateTestAccount(t, db, "svc-del-other")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, other.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("DelForbidden %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	err := service.DeletePost(ctx, post.ID, other.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrForbidden))
}

// ============================================================================
// ListPosts Tests
// ============================================================================

func TestListPosts_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "svc-list")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	ts := time.Now().UnixNano()
	post1 := createTestPost(t, db, account.ID, fmt.Sprintf("ListA %d", ts), "Desc A")
	post2 := createTestPost(t, db, account.ID, fmt.Sprintf("ListB %d", ts), "Desc B")
	defer cleanupPosts(t, db, post1.ID, post2.ID)

	posts, err := service.ListPosts(ctx, account.ID, "score")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(posts), 2)
}

// ============================================================================
// Vote Tests
// ============================================================================

func TestVote_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	author := testpkg.CreateTestAccount(t, db, "svc-vote-author")
	voter := testpkg.CreateTestAccount(t, db, "svc-vote-voter")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, voter.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("Vote %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	result, err := service.Vote(ctx, post.ID, voter.ID, suggestions.DirectionUp)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Score)
	assert.Equal(t, 1, result.Upvotes)
	assert.Equal(t, 0, result.Downvotes)
}

func TestVote_ChangeDirection(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	author := testpkg.CreateTestAccount(t, db, "svc-vote-change-author")
	voter := testpkg.CreateTestAccount(t, db, "svc-vote-change-voter")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, voter.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("VoteChange %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	// Vote up first
	_, err := service.Vote(ctx, post.ID, voter.ID, suggestions.DirectionUp)
	require.NoError(t, err)

	// Change to down
	result, err := service.Vote(ctx, post.ID, voter.ID, suggestions.DirectionDown)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, -1, result.Score)
	assert.Equal(t, 0, result.Upvotes)
	assert.Equal(t, 1, result.Downvotes)
}

func TestVote_InvalidDirection(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	_, err := service.Vote(context.Background(), 1, 1, "sideways")
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrInvalidData))
}

func TestVote_PostNotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	_, err := service.Vote(context.Background(), 999999999, 1, suggestions.DirectionUp)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrPostNotFound))
}

// ============================================================================
// RemoveVote Tests
// ============================================================================

func TestRemoveVote_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)
	ctx := context.Background()

	author := testpkg.CreateTestAccount(t, db, "svc-rmvote-author")
	voter := testpkg.CreateTestAccount(t, db, "svc-rmvote-voter")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", author.ID, voter.ID)

	post := createTestPost(t, db, author.ID, fmt.Sprintf("RemoveVote %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	// Vote first
	_, err := service.Vote(ctx, post.ID, voter.ID, suggestions.DirectionUp)
	require.NoError(t, err)

	// Remove vote
	result, err := service.RemoveVote(ctx, post.ID, voter.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Score)
	assert.Equal(t, 0, result.Upvotes)
}

func TestRemoveVote_PostNotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupService(t, db)

	_, err := service.RemoveVote(context.Background(), 999999999, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, svc.ErrPostNotFound))
}
