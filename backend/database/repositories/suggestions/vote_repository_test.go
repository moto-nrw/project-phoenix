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

func TestVoteRepository_Upsert(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewVoteRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "vote-upsert")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Vote Upsert %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	t.Run("creates new vote", func(t *testing.T) {
		vote := &suggestions.Vote{
			PostID:    post.ID,
			VoterID:   account.ID,
			Direction: suggestions.DirectionUp,
		}

		err := repo.Upsert(ctx, vote)
		require.NoError(t, err)
		assert.Greater(t, vote.ID, int64(0))
	})

	t.Run("updates existing vote on conflict", func(t *testing.T) {
		vote := &suggestions.Vote{
			PostID:    post.ID,
			VoterID:   account.ID,
			Direction: suggestions.DirectionDown,
		}

		err := repo.Upsert(ctx, vote)
		require.NoError(t, err)

		found, err := repo.FindByPostAndVoter(ctx, post.ID, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, suggestions.DirectionDown, found.Direction)
	})

	t.Run("rejects nil vote", func(t *testing.T) {
		err := repo.Upsert(ctx, nil)
		require.Error(t, err)
	})

	t.Run("rejects invalid vote", func(t *testing.T) {
		vote := &suggestions.Vote{
			PostID:    0,
			VoterID:   account.ID,
			Direction: suggestions.DirectionUp,
		}
		err := repo.Upsert(ctx, vote)
		require.Error(t, err)
	})
}

func TestVoteRepository_DeleteByPostAndVoter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewVoteRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "vote-delete")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Vote Delete %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	createTestVote(t, db, post.ID, account.ID, suggestions.DirectionUp)

	t.Run("deletes existing vote", func(t *testing.T) {
		err := repo.DeleteByPostAndVoter(ctx, post.ID, account.ID)
		require.NoError(t, err)

		found, err := repo.FindByPostAndVoter(ctx, post.ID, account.ID)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("does not error when vote does not exist", func(t *testing.T) {
		err := repo.DeleteByPostAndVoter(ctx, post.ID, 999999999)
		require.NoError(t, err)
	})
}

func TestVoteRepository_FindByPostAndVoter(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repoSuggestions.NewVoteRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "vote-find")
	defer testpkg.CleanupTableRecords(t, db, "auth.accounts", account.ID)

	post := createTestPost(t, db, account.ID, fmt.Sprintf("Vote Find %d", time.Now().UnixNano()), "Desc")
	defer cleanupPosts(t, db, post.ID)

	createTestVote(t, db, post.ID, account.ID, suggestions.DirectionDown)

	t.Run("finds existing vote", func(t *testing.T) {
		found, err := repo.FindByPostAndVoter(ctx, post.ID, account.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, suggestions.DirectionDown, found.Direction)
		assert.Equal(t, post.ID, found.PostID)
		assert.Equal(t, account.ID, found.VoterID)
	})

	t.Run("returns nil for non-existent vote", func(t *testing.T) {
		found, err := repo.FindByPostAndVoter(ctx, post.ID, 999999999)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}
