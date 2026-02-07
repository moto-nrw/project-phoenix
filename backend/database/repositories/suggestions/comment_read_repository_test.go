package suggestions_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repoSuggestions "github.com/moto-nrw/project-phoenix/database/repositories/suggestions"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestCommentReadRepository_Upsert(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-read-upsert-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("creates new comment read record", func(t *testing.T) {
		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		lastRead, err := repo.GetLastReadAt(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		require.NotNil(t, lastRead)
		assert.WithinDuration(t, time.Now(), *lastRead, 5*time.Second)
	})

	t.Run("updates existing comment read record", func(t *testing.T) {
		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		firstRead, err := repo.GetLastReadAt(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		require.NotNil(t, firstRead)

		time.Sleep(100 * time.Millisecond)

		err = repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		secondRead, err := repo.GetLastReadAt(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		require.NotNil(t, secondRead)

		assert.True(t, secondRead.After(*firstRead), "second read should be after first read")
	})

	t.Run("handles multiple users reading same post", func(t *testing.T) {
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-read-user2-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account2.ID)

		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		err = repo.Upsert(ctx, account2.ID, post.ID, "user")
		require.NoError(t, err)

		lastRead1, err := repo.GetLastReadAt(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		require.NotNil(t, lastRead1)

		lastRead2, err := repo.GetLastReadAt(ctx, account2.ID, post.ID, "user")
		require.NoError(t, err)
		require.NotNil(t, lastRead2)
	})
}

func TestCommentReadRepository_GetLastReadAt(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-read-get-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("returns nil when user never read comments", func(t *testing.T) {
		lastRead, err := repo.GetLastReadAt(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.Nil(t, lastRead)
	})

	t.Run("returns timestamp after user reads comments", func(t *testing.T) {
		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		lastRead, err := repo.GetLastReadAt(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		require.NotNil(t, lastRead)
		assert.WithinDuration(t, time.Now(), *lastRead, 5*time.Second)
	})

	t.Run("returns nil for non-existent post", func(t *testing.T) {
		lastRead, err := repo.GetLastReadAt(ctx, account.ID, 999999999, "user")
		require.NoError(t, err)
		assert.Nil(t, lastRead)
	})
}

func TestCommentReadRepository_CountUnreadByPost(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-unread-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("returns 0 when no comments exist", func(t *testing.T) {
		count, err := repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts all comments when user never read", func(t *testing.T) {
		comment1 := createTestComment(t, db, post.ID, account.ID, "Comment 1", suggestions.AuthorTypeUser)
		comment2 := createTestComment(t, db, post.ID, account.ID, "Comment 2", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment1.ID, comment2.ID)

		count, err := repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("counts only comments after last read time", func(t *testing.T) {
		comment1 := createTestComment(t, db, post.ID, account.ID, "Comment 1", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment1.ID)

		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		comment2 := createTestComment(t, db, post.ID, account.ID, "Comment 2", suggestions.AuthorTypeUser)
		comment3 := createTestComment(t, db, post.ID, account.ID, "Comment 3", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment2.ID, comment3.ID)

		count, err := repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("excludes soft-deleted comments", func(t *testing.T) {
		comment := createTestComment(t, db, post.ID, account.ID, "To be deleted", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		commentRepo := repoSuggestions.NewCommentRepository(db)
		err := commentRepo.Delete(ctx, comment.ID)
		require.NoError(t, err)

		count, err := repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("returns 0 after reading all comments", func(t *testing.T) {
		newPost := createTestPost(t, db, account.ID, fmt.Sprintf("New %d", time.Now().UnixNano()), "Desc")
		defer cleanupPosts(t, db, newPost.ID)

		comment := createTestComment(t, db, newPost.ID, account.ID, "Comment", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		err := repo.Upsert(ctx, account.ID, newPost.ID, "user")
		require.NoError(t, err)

		count, err := repo.CountUnreadByPost(ctx, account.ID, newPost.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestCommentReadRepository_CountTotalUnread(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-total-unread-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	t.Run("returns 0 when no comments exist", func(t *testing.T) {
		count, err := repo.CountTotalUnread(ctx, account.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts unread comments across multiple posts", func(t *testing.T) {
		post1 := createTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		post2 := createTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")
		defer cleanupPosts(t, db, post1.ID, post2.ID)

		comment1 := createTestComment(t, db, post1.ID, account.ID, "Comment on post1", suggestions.AuthorTypeUser)
		comment2 := createTestComment(t, db, post2.ID, account.ID, "Comment on post2", suggestions.AuthorTypeUser)
		comment3 := createTestComment(t, db, post2.ID, account.ID, "Another on post2", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment1.ID, comment2.ID, comment3.ID)

		count, err := repo.CountTotalUnread(ctx, account.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("respects last read time per post", func(t *testing.T) {
		post1 := createTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		post2 := createTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")
		defer cleanupPosts(t, db, post1.ID, post2.ID)

		comment1 := createTestComment(t, db, post1.ID, account.ID, "Comment on post1", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment1.ID)

		err := repo.Upsert(ctx, account.ID, post1.ID, "user")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		comment2 := createTestComment(t, db, post1.ID, account.ID, "New on post1", suggestions.AuthorTypeUser)
		comment3 := createTestComment(t, db, post2.ID, account.ID, "Comment on post2", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment2.ID, comment3.ID)

		count, err := repo.CountTotalUnread(ctx, account.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("excludes soft-deleted comments", func(t *testing.T) {
		post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
		defer cleanupPosts(t, db, post.ID)

		comment := createTestComment(t, db, post.ID, account.ID, "To be deleted", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		countBefore, err := repo.CountTotalUnread(ctx, account.ID, "user")
		require.NoError(t, err)

		commentRepo := repoSuggestions.NewCommentRepository(db)
		err = commentRepo.Delete(ctx, comment.ID)
		require.NoError(t, err)

		countAfter, err := repo.CountTotalUnread(ctx, account.ID, "user")
		require.NoError(t, err)

		assert.Equal(t, countBefore-1, countAfter)
	})

	t.Run("handles different users independently", func(t *testing.T) {
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-user2-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account2.ID)

		post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
		defer cleanupPosts(t, db, post.ID)

		comment := createTestComment(t, db, post.ID, account.ID, "Comment", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		count1, err := repo.CountTotalUnread(ctx, account.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 0, count1)

		count2, err := repo.CountTotalUnread(ctx, account2.ID, "user")
		require.NoError(t, err)
		assert.Equal(t, 1, count2)
	})
}
