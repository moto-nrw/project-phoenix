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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-read-upsert-%d", time.Now().UnixNano()))

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-read-get-%d", time.Now().UnixNano()))

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentReadRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-unread-%d", time.Now().UnixNano()))

	t.Run("returns 0 when no comments exist", func(t *testing.T) {
		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		})
		assert.Equal(t, 0, count)
	})

	t.Run("counts all comments when user never read", func(t *testing.T) {
		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
		testpkg.CreateTestComment(t, db, post.ID, account.ID, "Comment 1", suggestions.AuthorTypeUser)
		testpkg.CreateTestComment(t, db, post.ID, account.ID, "Comment 2", suggestions.AuthorTypeUser)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		})
		assert.Equal(t, 2, count)
	})

	t.Run("counts only comments after last read time", func(t *testing.T) {
		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
		testpkg.CreateTestComment(t, db, post.ID, account.ID, "Comment 1", suggestions.AuthorTypeUser)

		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		testpkg.CreateTestComment(t, db, post.ID, account.ID, "Comment 2", suggestions.AuthorTypeUser)
		testpkg.CreateTestComment(t, db, post.ID, account.ID, "Comment 3", suggestions.AuthorTypeUser)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		})
		assert.Equal(t, 2, count)
	})

	t.Run("excludes soft-deleted comments", func(t *testing.T) {
		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
		comment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "To be deleted", suggestions.AuthorTypeUser)

		commentRepo := repoSuggestions.NewCommentRepository(db)
		err := commentRepo.Delete(ctx, comment.ID)
		require.NoError(t, err)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnreadByPost(ctx, account.ID, post.ID, "user")
		})
		assert.Equal(t, 0, count)
	})

	t.Run("returns 0 after reading all comments", func(t *testing.T) {
		newPost := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("New %d", time.Now().UnixNano()), "Desc")

		testpkg.CreateTestComment(t, db, newPost.ID, account.ID, "Comment", suggestions.AuthorTypeUser)

		err := repo.Upsert(ctx, account.ID, newPost.ID, "user")
		require.NoError(t, err)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnreadByPost(ctx, account.ID, newPost.ID, "user")
		})
		assert.Equal(t, 0, count)
	})
}

// Each subtest owns its tenant and counts inside a tenant transaction, so
// the count sees its own rows only (#2419).
func TestCommentReadRepository_CountTotalUnread(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentReadRepository(db)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-total-unread-%d", time.Now().UnixNano()))

	t.Run("returns 0 when no comments exist", func(t *testing.T) {
		testpkg.OwnTenant(t)
		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "user")
		})
		assert.Equal(t, 0, count)
	})

	t.Run("counts unread comments across multiple posts", func(t *testing.T) {
		testpkg.OwnTenant(t)
		post1 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		post2 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")

		testpkg.CreateTestComment(t, db, post1.ID, account.ID, "Comment on post1", suggestions.AuthorTypeUser)
		testpkg.CreateTestComment(t, db, post2.ID, account.ID, "Comment on post2", suggestions.AuthorTypeUser)
		testpkg.CreateTestComment(t, db, post2.ID, account.ID, "Another on post2", suggestions.AuthorTypeUser)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "user")
		})
		assert.Equal(t, 3, count)
	})

	t.Run("respects last read time per post", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		post1 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		post2 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")

		testpkg.CreateTestComment(t, db, post1.ID, account.ID, "Comment on post1", suggestions.AuthorTypeUser)

		err := repo.Upsert(ctx, account.ID, post1.ID, "user")
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		testpkg.CreateTestComment(t, db, post1.ID, account.ID, "New on post1", suggestions.AuthorTypeUser)
		testpkg.CreateTestComment(t, db, post2.ID, account.ID, "Comment on post2", suggestions.AuthorTypeUser)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "user")
		})
		assert.Equal(t, 2, count)
	})

	t.Run("excludes soft-deleted comments", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")

		comment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "To be deleted", suggestions.AuthorTypeUser)

		countBefore := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "user")
		})

		commentRepo := repoSuggestions.NewCommentRepository(db)
		require.NoError(t, commentRepo.Delete(ctx, comment.ID))

		countAfter := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "user")
		})

		assert.Equal(t, countBefore-1, countAfter)
	})

	t.Run("handles different users independently", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-user2-%d", time.Now().UnixNano()))

		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")

		testpkg.CreateTestComment(t, db, post.ID, account.ID, "Comment", suggestions.AuthorTypeUser)

		err := repo.Upsert(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		count1 := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "user")
		})
		assert.Equal(t, 0, count1)

		count2 := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account2.ID, "user")
		})
		assert.Equal(t, 1, count2)
	})

	t.Run("excludes comments on hidden posts for operators", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		visiblePost := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Visible %d", time.Now().UnixNano()), "Desc")
		hiddenPost := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Hidden %d", time.Now().UnixNano()), "Desc")

		testpkg.CreateTestComment(t, db, visiblePost.ID, account.ID, "Visible comment", suggestions.AuthorTypeUser)
		testpkg.CreateTestComment(t, db, hiddenPost.ID, account.ID, "Hidden comment", suggestions.AuthorTypeUser)

		_, err := db.NewUpdate().
			TableExpr("suggestions.posts").
			Set("is_hidden = TRUE").
			Where("id = ?", hiddenPost.ID).
			Exec(ctx)
		require.NoError(t, err)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountTotalUnread(ctx, account.ID, "operator")
		})
		assert.Equal(t, 1, count)

		hiddenPostCount, err := repo.CountUnreadByPost(ctx, account.ID, hiddenPost.ID, "operator")
		require.NoError(t, err)
		assert.Equal(t, 0, hiddenPostCount)
	})
}
