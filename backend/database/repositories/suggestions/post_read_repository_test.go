package suggestions_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repoSuggestions "github.com/moto-nrw/project-phoenix/database/repositories/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPostReadRepository_MarkViewed(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-mark-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("marks post as viewed for first time", func(t *testing.T) {
		err := repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		isViewed, err := repo.IsViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)
		assert.True(t, isViewed)
	})

	t.Run("updates viewed timestamp on subsequent views", func(t *testing.T) {
		err := repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		var firstViewTime time.Time
		err = db.NewSelect().
			TableExpr("suggestions.post_reads").
			Column("viewed_at").
			Where("account_id = ? AND post_id = ?", account.ID, post.ID).
			Scan(ctx, &firstViewTime)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		var secondViewTime time.Time
		err = db.NewSelect().
			TableExpr("suggestions.post_reads").
			Column("viewed_at").
			Where("account_id = ? AND post_id = ?", account.ID, post.ID).
			Scan(ctx, &secondViewTime)
		require.NoError(t, err)

		assert.True(t, secondViewTime.After(firstViewTime))
	})

	t.Run("handles multiple operators viewing same post", func(t *testing.T) {
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op2-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account2.ID)

		err := repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		err = repo.MarkViewed(ctx, account2.ID, post.ID)
		require.NoError(t, err)

		isViewed1, err := repo.IsViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)
		assert.True(t, isViewed1)

		isViewed2, err := repo.IsViewed(ctx, account2.ID, post.ID)
		require.NoError(t, err)
		assert.True(t, isViewed2)
	})
}

func TestPostReadRepository_IsViewed(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-isviewed-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("returns false when operator never viewed post", func(t *testing.T) {
		isViewed, err := repo.IsViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)
		assert.False(t, isViewed)
	})

	t.Run("returns true after operator viewed post", func(t *testing.T) {
		err := repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		isViewed, err := repo.IsViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)
		assert.True(t, isViewed)
	})

	t.Run("returns false for non-existent post", func(t *testing.T) {
		isViewed, err := repo.IsViewed(ctx, account.ID, 999999999)
		require.NoError(t, err)
		assert.False(t, isViewed)
	})

	t.Run("returns false for different operator", func(t *testing.T) {
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op2-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account2.ID)

		err := repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		isViewed, err := repo.IsViewed(ctx, account2.ID, post.ID)
		require.NoError(t, err)
		assert.False(t, isViewed)
	})
}

func TestPostReadRepository_CountUnviewed(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostReadRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-count-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	t.Run("returns 0 when no posts exist", func(t *testing.T) {
		count, err := repo.CountUnviewed(ctx, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts all posts when operator never viewed any", func(t *testing.T) {
		post1 := createTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		post2 := createTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")
		defer cleanupPosts(t, db, post1.ID, post2.ID)

		count, err := repo.CountUnviewed(ctx, account.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("excludes viewed posts from count", func(t *testing.T) {
		post1 := createTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		post2 := createTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")
		post3 := createTestPost(t, db, account.ID, fmt.Sprintf("Post3 %d", time.Now().UnixNano()), "Desc3")
		defer cleanupPosts(t, db, post1.ID, post2.ID, post3.ID)

		countBefore, err := repo.CountUnviewed(ctx, account.ID)
		require.NoError(t, err)

		err = repo.MarkViewed(ctx, account.ID, post1.ID)
		require.NoError(t, err)

		countAfter, err := repo.CountUnviewed(ctx, account.ID)
		require.NoError(t, err)

		assert.Equal(t, countBefore-1, countAfter)
	})

	t.Run("handles different operators independently", func(t *testing.T) {
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op2-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account2.ID)

		post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")
		defer cleanupPosts(t, db, post.ID)

		err := repo.MarkViewed(ctx, account.ID, post.ID)
		require.NoError(t, err)

		count1, err := repo.CountUnviewed(ctx, account.ID)
		require.NoError(t, err)

		count2, err := repo.CountUnviewed(ctx, account2.ID)
		require.NoError(t, err)

		assert.NotEqual(t, count1, count2)
	})

	t.Run("returns 0 after viewing all posts", func(t *testing.T) {
		account3 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op3-%d", time.Now().UnixNano()))
		defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account3.ID)

		post1 := createTestPost(t, db, account3.ID, fmt.Sprintf("P1 %d", time.Now().UnixNano()), "D1")
		post2 := createTestPost(t, db, account3.ID, fmt.Sprintf("P2 %d", time.Now().UnixNano()), "D2")
		defer cleanupPosts(t, db, post1.ID, post2.ID)

		err := repo.MarkViewed(ctx, account3.ID, post1.ID)
		require.NoError(t, err)

		err = repo.MarkViewed(ctx, account3.ID, post2.ID)
		require.NoError(t, err)

		count, err := repo.CountUnviewed(ctx, account3.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}
