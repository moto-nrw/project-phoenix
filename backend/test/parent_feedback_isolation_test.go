// Parent feedback board isolation (#1678).
//
// The parent portal's feedback board and the school's staff suggestion board
// share suggestions.posts. What keeps them apart is not a WHERE clause but the
// RLS policy from migration 1.15.239, which matches author_type against
// app.current_actor_type. These tests run through real phoenix_tenant
// transactions (tenant.WithTenantTx / tenant.WithParentTx) because that is the
// only place the guarantee actually exists — a repository-level test with a
// plain context would pass even if the policy were missing.
package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repoSuggestions "github.com/moto-nrw/project-phoenix/database/repositories/suggestions"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// createBoardPost writes one post through the given actor transaction.
func createBoardPost(tb testing.TB, db *bun.DB, tenantID, accountID int64, authorType, title string) *suggestions.Post {
	tb.Helper()
	repo := repoSuggestions.NewPostRepository(db)
	post := &suggestions.Post{
		Title:       title,
		Description: "isolation fixture",
		AuthorID:    accountID,
		AuthorType:  authorType,
		Status:      suggestions.StatusOpen,
	}
	runner := tenant.WithTenantTx
	if authorType == suggestions.PostAuthorParent {
		runner = tenant.WithParentTx
	}
	err := runner(context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return repo.Create(txCtx, post)
	})
	require.NoError(tb, err, "creating a %s post must succeed in its own actor transaction", authorType)
	require.NotZero(tb, post.ID)
	return post
}

// TestParentFeedbackHiddenFromStaffBoard is the core guarantee of #1678: what a
// guardian writes to the product team must never be readable by the school.
func TestParentFeedbackHiddenFromStaffBoard(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantID := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantID)
	defer CleanupTenantTestData(t, db, tenantID)

	account := CreateTestAccount(t, db, "feedback-isolation@example.test")
	defer CleanupAccount(t, db, account.ID)
	EnsureAccountTenant(t, db, account.ID, tenantID)

	parentPost := createBoardPost(t, db, tenantID, account.ID, suggestions.PostAuthorParent, "Elternsicht auf die App")
	staffPost := createBoardPost(t, db, tenantID, account.ID, suggestions.PostAuthorStaff, "Wunsch aus dem Team")

	repo := repoSuggestions.NewPostRepository(db)

	t.Run("staff transaction cannot list or fetch parent feedback", func(t *testing.T) {
		err := tenant.WithTenantTx(context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			posts, err := repo.List(txCtx, account.ID, suggestions.ReaderTypeUser, "newest", "")
			require.NoError(t, err)
			for _, post := range posts {
				assert.NotEqual(t, parentPost.ID, post.ID,
					"parent feedback leaked onto the school's suggestion board")
				assert.Equal(t, suggestions.PostAuthorStaff, post.AuthorType)
			}

			found, err := repo.FindByID(txCtx, parentPost.ID, suggestions.ReaderTypeUser)
			require.NoError(t, err)
			assert.Nil(t, found, "staff must not be able to fetch a parent post by id")
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("parent transaction cannot list or fetch staff suggestions", func(t *testing.T) {
		err := tenant.WithParentTx(context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			posts, err := repo.List(txCtx, account.ID, suggestions.ReaderTypeParent, "newest", "")
			require.NoError(t, err)
			require.NotEmpty(t, posts, "the parent board must still show the guardian's own entry")
			for _, post := range posts {
				assert.NotEqual(t, staffPost.ID, post.ID,
					"staff suggestions leaked onto the parent feedback board")
				assert.Equal(t, suggestions.PostAuthorParent, post.AuthorType)
			}

			found, err := repo.FindByID(txCtx, staffPost.ID, suggestions.ReaderTypeParent)
			require.NoError(t, err)
			assert.Nil(t, found, "parents must not be able to fetch a staff post by id")
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("parent names never leave the database", func(t *testing.T) {
		err := tenant.WithParentTx(context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			post, err := repo.FindByIDWithVote(txCtx, parentPost.ID, account.ID, suggestions.ReaderTypeParent)
			require.NoError(t, err)
			require.NotNil(t, post)
			assert.Equal(t, "Elternteil", post.AuthorName,
				"a guardian's real name must be masked before it leaves SQL")
			return nil
		})
		require.NoError(t, err)
	})
}

// TestParentPostRejectedInStaffTransaction pins the fail-closed direction: a
// parent-typed row written through the wrong transaction is refused by the
// policy's WITH CHECK instead of silently landing on the school's board.
func TestParentPostRejectedInStaffTransaction(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantID := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantID)
	defer CleanupTenantTestData(t, db, tenantID)

	account := CreateTestAccount(t, db, "feedback-wrong-tx@example.test")
	defer CleanupAccount(t, db, account.ID)
	EnsureAccountTenant(t, db, account.ID, tenantID)

	repo := repoSuggestions.NewPostRepository(db)
	post := &suggestions.Post{
		Title:       "Falscher Kanal",
		Description: "written through a staff transaction",
		AuthorID:    account.ID,
		AuthorType:  suggestions.PostAuthorParent,
		Status:      suggestions.StatusOpen,
	}

	err := tenant.WithTenantTx(context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return repo.Create(txCtx, post)
	})
	require.Error(t, err, "the database must refuse a parent post written on the staff side")
}

// TestNestedActorMismatchRejected guards the transaction helper itself: reusing
// a staff transaction for parent work would run against the wrong board, so the
// mismatch has to fail loudly rather than degrade silently.
func TestNestedActorMismatchRejected(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantID := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantID)
	defer CleanupTenantTestData(t, db, tenantID)

	err := tenant.WithTenantTx(context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return tenant.WithParentTx(txCtx, db, tenantID, func(context.Context, bun.Tx) error {
			t.Fatal("nested parent transaction must not run inside a staff transaction")
			return nil
		})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatched actor")
}
