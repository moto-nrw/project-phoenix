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
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// cleanupComments removes test comments.
func cleanupComments(t *testing.T, db *bun.DB, commentIDs ...int64) {
	t.Helper()
	if len(commentIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr("suggestions.comments").
		Where("id IN (?)", bun.List(commentIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup comments: %v", err)
	}
}

// cleanupOperators removes test operators.
func cleanupOperators(t *testing.T, db *bun.DB, operatorIDs ...int64) {
	t.Helper()
	if len(operatorIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr("platform.operators").
		Where("id IN (?)", bun.List(operatorIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup operators: %v", err)
	}
}

func TestCommentRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-create-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("creates comment successfully", func(t *testing.T) {
		comment := &suggestions.Comment{
			PostID:     post.ID,
			AuthorID:   account.ID,
			AuthorType: suggestions.AuthorTypeUser,
			Content:    "Test comment content",
		}

		err := repo.Create(ctx, comment)
		require.NoError(t, err)
		assert.Greater(t, comment.ID, int64(0))
		assert.NotZero(t, comment.CreatedAt)
		defer cleanupComments(t, db, comment.ID)
	})

	t.Run("rejects nil comment", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects invalid comment - empty content", func(t *testing.T) {
		comment := &suggestions.Comment{
			PostID:     post.ID,
			AuthorID:   account.ID,
			AuthorType: suggestions.AuthorTypeUser,
			Content:    "",
		}
		err := repo.Create(ctx, comment)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "content is required")
	})

	t.Run("rejects invalid comment - missing post ID", func(t *testing.T) {
		comment := &suggestions.Comment{
			PostID:     0,
			AuthorID:   account.ID,
			AuthorType: suggestions.AuthorTypeUser,
			Content:    "Content",
		}
		err := repo.Create(ctx, comment)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "post ID is required")
	})
}

func TestCommentRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-findbyid-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	comment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "Find me!", suggestions.AuthorTypeUser)
	defer cleanupComments(t, db, comment.ID)

	t.Run("finds existing comment", func(t *testing.T) {
		found, err := repo.FindByID(ctx, comment.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, comment.ID, found.ID)
		assert.Equal(t, comment.Content, found.Content)
		assert.Equal(t, comment.PostID, found.PostID)
		assert.Equal(t, comment.AuthorID, found.AuthorID)
	})

	t.Run("returns nil for non-existent comment", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("returns nil for soft-deleted comment", func(t *testing.T) {
		deletedComment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "To be deleted", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, deletedComment.ID)

		err := repo.Delete(ctx, deletedComment.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, deletedComment.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestCommentRepository_FindByPostID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-findbypost-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	person := testpkg.CreateTestPerson(t, db, "Comment", "Author")
	defer testpkg.CleanupTableRecords(t, db, "users.persons", person.ID)

	_, err := db.NewUpdate().
		TableExpr("users.persons").
		Set("account_id = ?", account.ID).
		Where("id = ?", person.ID).
		Exec(ctx)
	require.NoError(t, err)

	operatorID := testpkg.CreateTestOperator(t, db).ID
	defer cleanupOperators(t, db, operatorID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	comment1 := testpkg.CreateTestComment(t, db, post.ID, account.ID, "First comment", suggestions.AuthorTypeUser)
	comment2 := testpkg.CreateTestComment(t, db, post.ID, account.ID, "Second comment", suggestions.AuthorTypeUser)
	internalComment := testpkg.CreateTestComment(t, db, post.ID, operatorID, "Internal note", suggestions.AuthorTypeOperator)
	defer cleanupComments(t, db, comment1.ID, comment2.ID, internalComment.ID)

	t.Run("finds all comments for a post", func(t *testing.T) {
		comments, err := repo.FindByPostID(ctx, post.ID)
		require.NoError(t, err)
		assert.Len(t, comments, 3)

		assert.Equal(t, comment1.ID, comments[0].ID)
		assert.Equal(t, comment2.ID, comments[1].ID)

		assert.NotEmpty(t, comments[0].AuthorName)
		assert.Contains(t, comments[0].AuthorName, "Comment")

		var foundOperatorComment bool
		for _, c := range comments {
			if c.ID == internalComment.ID {
				foundOperatorComment = true
				assert.Equal(t, "Test Operator", c.AuthorName)
			}
		}
		assert.True(t, foundOperatorComment)
	})

	t.Run("returns empty slice for post with no comments", func(t *testing.T) {
		emptyPost := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Empty %d", time.Now().UnixNano()), "No comments")
		defer cleanupPosts(t, db, emptyPost.ID)

		comments, err := repo.FindByPostID(ctx, emptyPost.ID)
		require.NoError(t, err)
		assert.Empty(t, comments)
	})

	t.Run("excludes soft-deleted comments", func(t *testing.T) {
		deletedComment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "Will be deleted", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, deletedComment.ID)

		err := repo.Delete(ctx, deletedComment.ID)
		require.NoError(t, err)

		comments, err := repo.FindByPostID(ctx, post.ID)
		require.NoError(t, err)

		for _, c := range comments {
			assert.NotEqual(t, deletedComment.ID, c.ID)
		}
	})
}

func TestCommentRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-delete-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("soft-deletes comment successfully", func(t *testing.T) {
		comment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "To be deleted", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		err := repo.Delete(ctx, comment.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, comment.ID)
		require.NoError(t, err)
		assert.Nil(t, found)

		var deletedAt *time.Time
		err = db.NewSelect().
			TableExpr("suggestions.comments").
			Column("deleted_at").
			Where("id = ?", comment.ID).
			Scan(ctx, &deletedAt)
		require.NoError(t, err)
		assert.NotNil(t, deletedAt)
	})

	t.Run("no error when deleting non-existent comment", func(t *testing.T) {
		err := repo.Delete(ctx, 999999999)
		require.NoError(t, err)
	})

	t.Run("no error when deleting already deleted comment", func(t *testing.T) {
		comment := testpkg.CreateTestComment(t, db, post.ID, account.ID, "Already deleted", suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		err := repo.Delete(ctx, comment.ID)
		require.NoError(t, err)

		err = repo.Delete(ctx, comment.ID)
		require.NoError(t, err)
	})
}
