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

// createTestVote creates a vote directly in the database for testing.
func createTestVote(t *testing.T, db *bun.DB, postID, voterID int64, direction string) *suggestions.Vote {
	t.Helper()

	vote := &suggestions.Vote{
		PostID:    postID,
		VoterID:   voterID,
		Direction: direction,
	}

	vote.SetTenantID(1)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 5*time.Second)
	defer cancel()

	_, err := db.NewInsert().
		Model(vote).
		ModelTableExpr("suggestions.votes").
		Returning("*").
		Exec(ctx)
	require.NoError(t, err, "Failed to create test vote")

	return vote
}

// cleanupPosts removes test posts and their votes.
func cleanupPosts(t *testing.T, db *bun.DB, postIDs ...int64) {
	t.Helper()
	if len(postIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 5*time.Second)
	defer cancel()

	// Delete votes first (FK constraint)
	_, err := db.NewDelete().
		TableExpr("suggestions.votes").
		Where("post_id IN (?)", bun.List(postIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup votes: %v", err)
	}

	_, err = db.NewDelete().
		TableExpr("suggestions.posts").
		Where("id IN (?)", bun.List(postIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup posts: %v", err)
	}
}

func TestPostRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-create")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	t.Run("creates post successfully", func(t *testing.T) {
		post := &suggestions.Post{
			Title:       fmt.Sprintf("Test Post %d", time.Now().UnixNano()),
			Description: "Test description for create",
			AuthorID:    account.ID,
			Status:      suggestions.StatusOpen,
		}

		err := repo.Create(ctx, post)
		require.NoError(t, err)
		assert.Greater(t, post.ID, int64(0))
		defer cleanupPosts(t, db, post.ID)
	})

	t.Run("rejects nil post", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
	})

	t.Run("rejects invalid post", func(t *testing.T) {
		post := &suggestions.Post{
			Title:       "",
			Description: "Desc",
			AuthorID:    account.ID,
			Status:      suggestions.StatusOpen,
		}
		err := repo.Create(ctx, post)
		require.Error(t, err)
	})
}

func TestPostRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-findbyid")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("FindByID %d", time.Now().UnixNano()), "Find test")
	defer cleanupPosts(t, db, post.ID)

	t.Run("finds existing post", func(t *testing.T) {
		found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, post.ID, found.ID)
		assert.Equal(t, post.Title, found.Title)
	})

	t.Run("returns nil for non-existent post", func(t *testing.T) {
		found, err := repo.FindByID(ctx, 999999999, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestPostRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-update")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Update %d", time.Now().UnixNano()), "Original desc")
	defer cleanupPosts(t, db, post.ID)

	t.Run("updates post successfully", func(t *testing.T) {
		post.Title = "Updated Title"
		post.Description = "Updated description"

		err := repo.Update(ctx, post)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", found.Title)
		assert.Equal(t, "Updated description", found.Description)
	})

	t.Run("rejects nil post", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
	})

	t.Run("does not overwrite status or hidden state", func(t *testing.T) {
		post.Status = suggestions.StatusDone
		post.IsHidden = true
		_, err := db.NewUpdate().
			Model(post).
			ModelTableExpr("suggestions.posts").
			Column("status", "is_hidden").
			Where("id = ?", post.ID).
			Exec(ctx)
		require.NoError(t, err)

		post.Title = "Title only update"
		post.Description = "Description only update"
		post.Status = suggestions.StatusOpen
		post.IsHidden = false

		err = repo.Update(ctx, post)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		assert.Equal(t, "Title only update", found.Title)
		assert.Equal(t, "Description only update", found.Description)
		assert.Equal(t, suggestions.StatusDone, found.Status)
		assert.True(t, found.IsHidden)
	})
}

func TestPostRepository_UpdateStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-update-status")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("UpdateStatus %d", time.Now().UnixNano()), "Original desc")
	defer cleanupPosts(t, db, post.ID)

	err := repo.UpdateStatus(ctx, post.ID, suggestions.StatusDone)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
	require.NoError(t, err)
	assert.Equal(t, suggestions.StatusDone, found.Status)
	assert.False(t, found.IsHidden)
}

func TestPostRepository_UpdateHidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-update-hidden")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("UpdateHidden %d", time.Now().UnixNano()), "Original desc")
	defer cleanupPosts(t, db, post.ID)

	err := repo.UpdateHidden(ctx, post.ID, true)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
	require.NoError(t, err)
	assert.True(t, found.IsHidden)
	assert.Equal(t, suggestions.StatusOpen, found.Status)
}

func TestPostRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-delete")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("Delete %d", time.Now().UnixNano()), "To be deleted")
	// No defer cleanup needed since we're testing deletion

	t.Run("deletes post successfully", func(t *testing.T) {
		err := repo.Delete(ctx, post.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestPostRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-list")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	// Create a person linked to the account for author_name resolution
	person := testpkg.CreateTestPerson(t, db, "List", "Tester")
	defer testpkg.CleanupTableRecords(t, db, "users.persons", person.ID)

	// Link person to account
	_, err := db.NewUpdate().
		TableExpr("users.persons").
		Set("account_id = ?", account.ID).
		Where("id = ?", person.ID).
		Exec(ctx)
	require.NoError(t, err)

	ts := time.Now().UnixNano()
	post1 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("List A %d", ts), "Description A")
	post2 := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("List B %d", ts), "Description B")
	defer cleanupPosts(t, db, post1.ID, post2.ID)

	t.Run("lists posts sorted by score", func(t *testing.T) {
		posts, err := repo.List(ctx, account.ID, suggestions.ReaderTypeUser, "score", "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(posts), 2)
	})

	t.Run("lists posts sorted by newest", func(t *testing.T) {
		posts, err := repo.List(ctx, account.ID, suggestions.ReaderTypeUser, "newest", "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(posts), 2)
	})

	t.Run("lists posts sorted by status", func(t *testing.T) {
		posts, err := repo.List(ctx, account.ID, suggestions.ReaderTypeUser, "status", "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(posts), 2)
	})

	t.Run("resolves author name", func(t *testing.T) {
		posts, err := repo.List(ctx, account.ID, suggestions.ReaderTypeUser, "score", "")
		require.NoError(t, err)

		// Find our test posts
		for _, p := range posts {
			if p.ID == post1.ID {
				assert.NotEmpty(t, p.AuthorName)
				assert.Contains(t, p.AuthorName, "List")
				return
			}
		}
		t.Fatal("test post not found in list results")
	})
}

// TestPostRepository_ListOperatorAuthorJoinIsScoped pins the operator listing
// against duplicated posts. The operator reads through phoenix_admin, which
// bypasses RLS, so an unscoped author join matches every users.persons row the
// author's account owns — one per school, plus retained soft-deleted records —
// and returns the post once per match. users.persons is unique on
// (tenant_id, account_id) only for live rows, so both shapes are reachable.
func TestPostRepository_ListOperatorAuthorJoinIsScoped(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)

	const otherTenantID int64 = 99678
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	account := testpkg.CreateTestAccount(t, db, "suggestions-operator-join")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	// The author's person record in the post's own tenant — the only one the
	// operator board may resolve the display name from.
	homePerson := testpkg.CreateTestPersonForTenant(t, db, 1, "Operator", "Home")
	// The same human at a second school, and a soft-deleted leftover in the
	// post's own tenant. Both used to multiply the post.
	otherPerson := testpkg.CreateTestPersonForTenant(t, db, otherTenantID, "Operator", "Elsewhere")
	removedPerson := testpkg.CreateTestPersonForTenant(t, db, 1, "Operator", "Removed")
	defer testpkg.CleanupTableRecords(t, db, "users.persons",
		homePerson.ID, otherPerson.ID, removedPerson.ID)

	ctx := context.Background()

	// Soft-delete first: the partial unique index forbids two live person rows
	// for the same account within one tenant.
	_, err := db.NewUpdate().
		TableExpr("users.persons").
		Set("deleted_at = ?", time.Now()).
		Where("id = ?", removedPerson.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = db.NewUpdate().
		TableExpr("users.persons").
		Set("account_id = ?", account.ID).
		Where("id IN (?)", bun.List([]int64{homePerson.ID, otherPerson.ID, removedPerson.ID})).
		Exec(ctx)
	require.NoError(t, err)

	ts := time.Now().UnixNano()
	staffPost := testpkg.CreateTestPost(t, db, account.ID,
		fmt.Sprintf("Operator Join Staff %d", ts), "Staff description")
	parentPost := testpkg.CreateTestPost(t, db, account.ID,
		fmt.Sprintf("Operator Join Parent %d", ts), "Parent description")
	defer cleanupPosts(t, db, staffPost.ID, parentPost.ID)

	_, err = db.NewUpdate().
		TableExpr("suggestions.posts").
		Set("author_type = ?", suggestions.PostAuthorParent).
		Where("id = ?", parentPost.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Operator context carries no tenant, so no tenant filter narrows the join.
	posts, err := repo.List(ctx, account.ID, suggestions.ReaderTypeOperator, "newest", "")
	require.NoError(t, err)

	matches := func(id int64) []*suggestions.Post {
		var found []*suggestions.Post
		for _, p := range posts {
			if p.ID == id {
				found = append(found, p)
			}
		}
		return found
	}

	staffMatches := matches(staffPost.ID)
	require.Len(t, staffMatches, 1, "staff post must appear exactly once for the operator")
	assert.Equal(t, "Operator H.", staffMatches[0].AuthorName,
		"name must resolve from the person record in the post's own tenant")

	parentMatches := matches(parentPost.ID)
	require.Len(t, parentMatches, 1, "parent post must appear exactly once for the operator")
	assert.Equal(t, "Elternteil", parentMatches[0].AuthorName)
}

func TestPostRepository_FindByIDWithVote(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	voteRepo := repoSuggestions.NewVoteRepository(db)
	ctx := testpkg.TenantContext(1)

	account := testpkg.CreateTestAccount(t, db, "suggestions-findwithvote")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := testpkg.CreateTestPost(t, db, account.ID, fmt.Sprintf("WithVote %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	t.Run("returns nil user_vote when no vote exists", func(t *testing.T) {
		found, err := repo.FindByIDWithVote(ctx, post.ID, account.ID, suggestions.ReaderTypeUser)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Nil(t, found.UserVote)
	})

	t.Run("returns user_vote when vote exists", func(t *testing.T) {
		vote := &suggestions.Vote{
			PostID:    post.ID,
			VoterID:   account.ID,
			Direction: suggestions.DirectionUp,
		}
		err := voteRepo.Upsert(ctx, vote)
		require.NoError(t, err)

		found, err := repo.FindByIDWithVote(ctx, post.ID, account.ID, suggestions.ReaderTypeUser)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.NotNil(t, found.UserVote)
		assert.Equal(t, suggestions.DirectionUp, *found.UserVote)
	})

	t.Run("returns nil for non-existent post", func(t *testing.T) {
		found, err := repo.FindByIDWithVote(ctx, 999999999, account.ID, suggestions.ReaderTypeUser)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestPostRepository_RecalculateScore(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewPostRepository(db)
	ctx := testpkg.TenantContext(1)

	account1 := testpkg.CreateTestAccount(t, db, "suggestions-score1")
	account2 := testpkg.CreateTestAccount(t, db, "suggestions-score2")
	account3 := testpkg.CreateTestAccount(t, db, "suggestions-score3")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account1.ID, account2.ID, account3.ID)

	post := testpkg.CreateTestPost(t, db, account1.ID, fmt.Sprintf("Score %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	t.Run("score is 0 with no votes", func(t *testing.T) {
		err := repo.RecalculateScore(ctx, post.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		assert.Equal(t, 0, found.Score)
	})

	t.Run("score reflects upvotes minus downvotes", func(t *testing.T) {
		// 2 upvotes, 1 downvote = score 1
		createTestVote(t, db, post.ID, account1.ID, suggestions.DirectionUp)
		createTestVote(t, db, post.ID, account2.ID, suggestions.DirectionUp)
		createTestVote(t, db, post.ID, account3.ID, suggestions.DirectionDown)

		err := repo.RecalculateScore(ctx, post.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, post.ID, suggestions.ReaderTypeOperator)
		require.NoError(t, err)
		assert.Equal(t, 1, found.Score)
	})
}
