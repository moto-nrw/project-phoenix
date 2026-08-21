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
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// countInTenantTx runs a count inside a tenant transaction. The unread counts
// carry no tenant_id filter of their own and rely on RLS to narrow them; with
// a plain tenant context they see the posts of the whole clone, which used to
// stay invisible only because per-row teardowns removed every other test's
// rows (#2419).
func countInTenantTx(t *testing.T, db *bun.DB, fn func(ctx context.Context) (int, error)) int {
	t.Helper()
	var count int
	require.NoError(t, tenant.WithTenantTx(context.Background(), db, testpkg.Tenant(t),
		func(txCtx context.Context, _ bun.Tx) error {
			var err error
			count, err = fn(txCtx)
			return err
		}))
	return count
}

func TestPostReadRepository_MarkViewed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewPostReadRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-mark-%d", time.Now().UnixNano()))

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")

	t.Run("marks post as viewed for first time", func(t *testing.T) {
		err := repo.MarkViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		isViewed, err := repo.IsViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.True(t, isViewed)
	})

	t.Run("updates viewed timestamp on subsequent views", func(t *testing.T) {
		err := repo.MarkViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		var firstViewTime time.Time
		err = db.NewSelect().
			TableExpr("suggestions.post_reads").
			Column("viewed_at").
			Where("account_id = ? AND post_id = ?", account.ID, post.ID).
			Scan(ctx, &firstViewTime)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		err = repo.MarkViewed(ctx, account.ID, post.ID, "user")
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

		err := repo.MarkViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		err = repo.MarkViewed(ctx, account2.ID, post.ID, "user")
		require.NoError(t, err)

		isViewed1, err := repo.IsViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.True(t, isViewed1)

		isViewed2, err := repo.IsViewed(ctx, account2.ID, post.ID, "user")
		require.NoError(t, err)
		assert.True(t, isViewed2)
	})
}

func TestPostReadRepository_IsViewed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewPostReadRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-isviewed-%d", time.Now().UnixNano()))

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")

	t.Run("returns false when operator never viewed post", func(t *testing.T) {
		isViewed, err := repo.IsViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.False(t, isViewed)
	})

	t.Run("returns true after operator viewed post", func(t *testing.T) {
		err := repo.MarkViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		isViewed, err := repo.IsViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)
		assert.True(t, isViewed)
	})

	t.Run("returns false for non-existent post", func(t *testing.T) {
		isViewed, err := repo.IsViewed(ctx, account.ID, 999999999, "user")
		require.NoError(t, err)
		assert.False(t, isViewed)
	})

	t.Run("returns false for different operator", func(t *testing.T) {
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op2-%d", time.Now().UnixNano()))

		err := repo.MarkViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		isViewed, err := repo.IsViewed(ctx, account2.ID, post.ID, "user")
		require.NoError(t, err)
		assert.False(t, isViewed)
	})
}

// Each subtest owns its tenant and counts inside a tenant transaction, so
// the count sees its own rows only (#2419).
func TestPostReadRepository_CountUnviewed(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewPostReadRepository(db)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-count-%d", time.Now().UnixNano()))

	t.Run("returns 0 when no posts exist", func(t *testing.T) {
		testpkg.OwnTenant(t)
		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnviewed(ctx, account.ID, "user")
		})
		assert.Equal(t, 0, count)
	})

	t.Run("counts all posts when operator never viewed any", func(t *testing.T) {
		testpkg.OwnTenant(t)
		testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnviewed(ctx, account.ID, "user")
		})
		assert.GreaterOrEqual(t, count, 2)
	})

	t.Run("excludes viewed posts from count", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		post1 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post1 %d", time.Now().UnixNano()), "Desc1")
		testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post2 %d", time.Now().UnixNano()), "Desc2")
		testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post3 %d", time.Now().UnixNano()), "Desc3")

		countBefore := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnviewed(ctx, account.ID, "user")
		})

		require.NoError(t, repo.MarkViewed(ctx, account.ID, post1.ID, "user"))

		countAfter := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnviewed(ctx, account.ID, "user")
		})

		assert.Equal(t, countBefore-1, countAfter)
	})

	t.Run("handles different operators independently", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		account2 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op2-%d", time.Now().UnixNano()))

		post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Desc")

		err := repo.MarkViewed(ctx, account.ID, post.ID, "user")
		require.NoError(t, err)

		count1, err := repo.CountUnviewed(ctx, account.ID, "user")
		require.NoError(t, err)

		count2, err := repo.CountUnviewed(ctx, account2.ID, "user")
		require.NoError(t, err)

		assert.NotEqual(t, count1, count2)
	})

	t.Run("returns 0 after viewing all posts", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		account3 := testpkg.CreateTestAccount(t, db, fmt.Sprintf("post-read-op3-%d", time.Now().UnixNano()))

		post1 := testpkg.CreateTestPost(t, db, account3.ID, fmt.Sprintf("P1 %d", time.Now().UnixNano()), "D1")
		post2 := testpkg.CreateTestPost(t, db, account3.ID, fmt.Sprintf("P2 %d", time.Now().UnixNano()), "D2")

		err := repo.MarkViewed(ctx, account3.ID, post1.ID, "user")
		require.NoError(t, err)

		err = repo.MarkViewed(ctx, account3.ID, post2.ID, "user")
		require.NoError(t, err)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnviewed(ctx, account3.ID, "user")
		})
		assert.Equal(t, 0, count)
	})

	t.Run("excludes hidden posts for operators", func(t *testing.T) {
		ctx := testpkg.OwnCtx(t)
		visiblePost := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Visible %d", time.Now().UnixNano()), "Desc")
		hiddenPost := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Hidden %d", time.Now().UnixNano()), "Desc")

		_, err := db.NewUpdate().
			TableExpr("suggestions.posts").
			Set("is_hidden = TRUE").
			Where("id = ?", hiddenPost.ID).
			Exec(ctx)
		require.NoError(t, err)

		err = repo.MarkViewed(ctx, account.ID, visiblePost.ID, "operator")
		require.NoError(t, err)

		count := countInTenantTx(t, db, func(ctx context.Context) (int, error) {
			return repo.CountUnviewed(ctx, account.ID, "operator")
		})
		assert.Equal(t, 0, count)
	})
}
