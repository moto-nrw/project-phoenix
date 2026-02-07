package suggestions_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repoSuggestions "github.com/moto-nrw/project-phoenix/database/repositories/suggestions"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// createTestComment creates a comment directly in the database for testing.
func createTestComment(t *testing.T, db *bun.DB, postID, authorID int64, content string, isInternal bool, authorType string) *suggestions.Comment {
	t.Helper()

	comment := &suggestions.Comment{
		PostID:     postID,
		AuthorID:   authorID,
		AuthorType: authorType,
		Content:    content,
		IsInternal: isInternal,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewInsert().
		Model(comment).
		ModelTableExpr("suggestions.comments").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err, "Failed to create test comment")

	return comment
}

// createTestOperator creates an operator account for testing.
func createTestOperator(t *testing.T, db *bun.DB, displayName string) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type operator struct {
		ID           int64  `bun:"id,pk,autoincrement"`
		Email        string `bun:"email"`
		DisplayName  string `bun:"display_name"`
		PasswordHash string `bun:"password_hash"`
		Active       bool   `bun:"active"`
	}

	email := fmt.Sprintf("%s-%d@test.com",
		strings.ReplaceAll(strings.ToLower(displayName), " ", ""),
		time.Now().UnixNano())
	op := &operator{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: "dummy-hash",
		Active:       true,
	}
	_, err := db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Returning("id").
		Exec(ctx)
	require.NoError(t, err, "Failed to create test operator")

	return op.ID
}

// cleanupComments removes test comments.
func cleanupComments(t *testing.T, db *bun.DB, commentIDs ...int64) {
	t.Helper()
	if len(commentIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr("suggestions.comments").
		Where("id IN (?)", bun.In(commentIDs)).
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr("platform.operators").
		Where("id IN (?)", bun.In(operatorIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup operators: %v", err)
	}
}

func TestCommentRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-create-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("creates comment successfully", func(t *testing.T) {
		comment := &suggestions.Comment{
			PostID:     post.ID,
			AuthorID:   account.ID,
			AuthorType: suggestions.AuthorTypeUser,
			Content:    "Test comment content",
			IsInternal: false,
		}

		err := repo.Create(ctx, comment)
		require.NoError(t, err)
		assert.Greater(t, comment.ID, int64(0))
		assert.NotZero(t, comment.CreatedAt)
		defer cleanupComments(t, db, comment.ID)
	})

	t.Run("creates internal operator comment", func(t *testing.T) {
		operatorID := createTestOperator(t, db, "Test Operator")
		defer cleanupOperators(t, db, operatorID)

		comment := &suggestions.Comment{
			PostID:     post.ID,
			AuthorID:   operatorID,
			AuthorType: suggestions.AuthorTypeOperator,
			Content:    "Internal operator note",
			IsInternal: true,
		}

		err := repo.Create(ctx, comment)
		require.NoError(t, err)
		assert.Greater(t, comment.ID, int64(0))
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
			IsInternal: false,
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
			IsInternal: false,
		}
		err := repo.Create(ctx, comment)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "post ID is required")
	})

	t.Run("rejects internal comment from non-operator", func(t *testing.T) {
		comment := &suggestions.Comment{
			PostID:     post.ID,
			AuthorID:   account.ID,
			AuthorType: suggestions.AuthorTypeUser,
			Content:    "Trying to create internal comment",
			IsInternal: true,
		}
		err := repo.Create(ctx, comment)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "only operators can create internal comments")
	})
}

func TestCommentRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-findbyid-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	comment := createTestComment(t, db, post.ID, account.ID, "Find me!", false, suggestions.AuthorTypeUser)
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
		deletedComment := createTestComment(t, db, post.ID, account.ID, "To be deleted", false, suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, deletedComment.ID)

		err := repo.Delete(ctx, deletedComment.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, deletedComment.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestCommentRepository_FindByPostID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := context.Background()

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

	operatorID := createTestOperator(t, db, "Test Operator")
	defer cleanupOperators(t, db, operatorID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	comment1 := createTestComment(t, db, post.ID, account.ID, "First comment", false, suggestions.AuthorTypeUser)
	comment2 := createTestComment(t, db, post.ID, account.ID, "Second comment", false, suggestions.AuthorTypeUser)
	internalComment := createTestComment(t, db, post.ID, operatorID, "Internal note", true, suggestions.AuthorTypeOperator)
	defer cleanupComments(t, db, comment1.ID, comment2.ID, internalComment.ID)

	t.Run("finds all non-internal comments", func(t *testing.T) {
		comments, err := repo.FindByPostID(ctx, post.ID, false)
		require.NoError(t, err)
		assert.Len(t, comments, 2)

		assert.Equal(t, comment1.ID, comments[0].ID)
		assert.Equal(t, comment2.ID, comments[1].ID)

		assert.NotEmpty(t, comments[0].AuthorName)
		assert.Contains(t, comments[0].AuthorName, "Comment")
	})

	t.Run("includes internal comments when requested", func(t *testing.T) {
		comments, err := repo.FindByPostID(ctx, post.ID, true)
		require.NoError(t, err)
		assert.Len(t, comments, 3)

		var foundInternal bool
		for _, c := range comments {
			if c.ID == internalComment.ID {
				foundInternal = true
				assert.True(t, c.IsInternal)
				assert.Equal(t, "Test Operator", c.AuthorName)
			}
		}
		assert.True(t, foundInternal)
	})

	t.Run("returns empty slice for post with no comments", func(t *testing.T) {
		emptyPost := createTestPost(t, db, account.ID, fmt.Sprintf("Empty %d", time.Now().UnixNano()), "No comments")
		defer cleanupPosts(t, db, emptyPost.ID)

		comments, err := repo.FindByPostID(ctx, emptyPost.ID, false)
		require.NoError(t, err)
		assert.Empty(t, comments)
	})

	t.Run("excludes soft-deleted comments", func(t *testing.T) {
		deletedComment := createTestComment(t, db, post.ID, account.ID, "Will be deleted", false, suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, deletedComment.ID)

		err := repo.Delete(ctx, deletedComment.ID)
		require.NoError(t, err)

		comments, err := repo.FindByPostID(ctx, post.ID, false)
		require.NoError(t, err)

		for _, c := range comments {
			assert.NotEqual(t, deletedComment.ID, c.ID)
		}
	})
}

func TestCommentRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-delete-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("soft-deletes comment successfully", func(t *testing.T) {
		comment := createTestComment(t, db, post.ID, account.ID, "To be deleted", false, suggestions.AuthorTypeUser)
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
		comment := createTestComment(t, db, post.ID, account.ID, "Already deleted", false, suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, comment.ID)

		err := repo.Delete(ctx, comment.ID)
		require.NoError(t, err)

		err = repo.Delete(ctx, comment.ID)
		require.NoError(t, err)
	})
}

func TestCommentRepository_CountByPostID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewCommentRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("comment-count-%d", time.Now().UnixNano()))
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Post %d", time.Now().UnixNano()), "Description")
	defer cleanupPosts(t, db, post.ID)

	t.Run("returns 0 for post with no comments", func(t *testing.T) {
		emptyPost := createTestPost(t, db, account.ID, fmt.Sprintf("Empty %d", time.Now().UnixNano()), "No comments")
		defer cleanupPosts(t, db, emptyPost.ID)

		count, err := repo.CountByPostID(ctx, emptyPost.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("counts all non-deleted comments", func(t *testing.T) {
		comment1 := createTestComment(t, db, post.ID, account.ID, "Comment 1", false, suggestions.AuthorTypeUser)
		comment2 := createTestComment(t, db, post.ID, account.ID, "Comment 2", false, suggestions.AuthorTypeUser)
		operatorID := createTestOperator(t, db, "Operator")
		defer cleanupOperators(t, db, operatorID)
		comment3 := createTestComment(t, db, post.ID, operatorID, "Comment 3", true, suggestions.AuthorTypeOperator)
		defer cleanupComments(t, db, comment1.ID, comment2.ID, comment3.ID)

		count, err := repo.CountByPostID(ctx, post.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("excludes soft-deleted comments from count", func(t *testing.T) {
		deletedComment := createTestComment(t, db, post.ID, account.ID, "To be deleted", false, suggestions.AuthorTypeUser)
		defer cleanupComments(t, db, deletedComment.ID)

		countBefore, err := repo.CountByPostID(ctx, post.ID)
		require.NoError(t, err)

		err = repo.Delete(ctx, deletedComment.ID)
		require.NoError(t, err)

		countAfter, err := repo.CountByPostID(ctx, post.ID)
		require.NoError(t, err)

		assert.Equal(t, countBefore-1, countAfter)
	})
}
