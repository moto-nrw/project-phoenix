package behavior_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestNativeParentFeedConcurrentCreationKeepsOneWinner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	feeds, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "parent-feed-race")
	ctx := testpkg.Ctx(t)
	type outcome struct {
		hash string
		err  error
	}
	results := make(chan outcome, 8)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for candidate := range 8 {
		workers.Go(func() {
			<-start
			hash, writeErr := feeds.EnsureParentCalendarFeedToken(ctx, account.ID, calendarFeedHash(account.ID, candidate))
			results <- outcome{hash, writeErr}
		})
	}
	close(start)
	workers.Wait()
	close(results)
	var winner string
	for result := range results {
		require.NoError(t, result.err)
		require.NotEmpty(t, result.hash)
		if winner == "" {
			winner = result.hash
		}
		require.Equal(t, winner, result.hash)
	}
	owner, found, err := feeds.FindParentCalendarFeedOwner(ctx, winner)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, account.ID, owner.ID)
	require.Equal(t, account.Email, owner.Email)
	require.True(t, owner.Active)
	for candidate := range 8 {
		hash := calendarFeedHash(account.ID, candidate)
		_, found, err := feeds.FindParentCalendarFeedOwner(ctx, hash)
		require.NoError(t, err)
		require.Equal(t, hash == winner, found, "losing candidates must never resolve")
	}
}

func TestNativeParentFeedRotationUsesAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	feeds, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "parent-feed-rollback")
	ctx := testpkg.Ctx(t)
	first, second := calendarFeedHash(account.ID, 1), calendarFeedHash(account.ID, 2)
	require.NoError(t, feeds.RotateParentCalendarFeedToken(ctx, account.ID, first))
	rollback := errors.New("rollback parent feed rotation")
	err = testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, tx bun.Tx) error {
		require.NoError(t, feeds.RotateParentCalendarFeedToken(txCtx, account.ID, second))
		_, found, readErr := feeds.FindParentCalendarFeedOwner(txCtx, first)
		require.NoError(t, readErr)
		require.False(t, found)
		_, updateErr := tx.NewRaw("UPDATE auth.accounts SET active = FALSE WHERE id = ?", account.ID).Exec(txCtx)
		require.NoError(t, updateErr)
		owner, found, readErr := feeds.FindParentCalendarFeedOwner(txCtx, second)
		require.NoError(t, readErr)
		require.True(t, found)
		require.False(t, owner.Active, "Calendar receives current activation facts before authorizing a feed")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	owner, found, err := feeds.FindParentCalendarFeedAccount(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, owner.Active)
	require.Equal(t, first, owner.TokenHash)
	for _, hash := range []string{"", second} {
		_, found, err := feeds.FindParentCalendarFeedOwner(ctx, hash)
		require.NoError(t, err)
		require.False(t, found)
	}
	_, found, err = feeds.FindParentCalendarFeedAccount(ctx, 0)
	require.NoError(t, err)
	require.False(t, found)
	missing, err := feeds.EnsureParentCalendarFeedToken(ctx, 0, second)
	require.NoError(t, err)
	require.Empty(t, missing)
}

func TestNativeParentFeedIgnoresErasedAccounts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	feeds, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "parent-feed-erased")
	ctx := testpkg.Ctx(t)
	first, second := calendarFeedHash(account.ID, 1), calendarFeedHash(account.ID, 2)
	require.NoError(t, feeds.RotateParentCalendarFeedToken(ctx, account.ID, first))
	_, err = db.NewRaw(`UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, account.ID).Exec(ctx)
	require.NoError(t, err)

	_, found, err := feeds.FindParentCalendarFeedAccount(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = feeds.FindParentCalendarFeedOwner(ctx, first)
	require.NoError(t, err)
	require.False(t, found)
	ensured, err := feeds.EnsureParentCalendarFeedToken(ctx, account.ID, second)
	require.NoError(t, err)
	require.Empty(t, ensured)
	require.NoError(t, feeds.RotateParentCalendarFeedToken(ctx, account.ID, second))

	var stored string
	require.NoError(t, db.NewRaw(`SELECT calendar_feed_token FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &stored))
	require.Equal(t, first, stored)
}
