package behavior_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func calendarFeedHash(accountID int64, candidate int) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("calendar-feed-%d-%d", accountID, candidate))))
}

func TestNativeStaffFeedConcurrentCreationKeepsOneWinner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	feeds, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "feed-race")
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
			hash, writeErr := feeds.EnsureStaffCalendarFeedToken(ctx, account.ID, testpkg.Tenant(t), calendarFeedHash(account.ID, candidate))
			results <- outcome{hash: hash, err: writeErr}
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
		require.Equal(t, winner, result.hash, "every competing creation must observe the persisted winner")
	}
	owner, found, err := feeds.FindStaffCalendarFeedOwner(ctx, winner)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, account.ID, owner.AccountID)
	require.Equal(t, testpkg.Tenant(t), owner.TenantID)
}

func TestNativeStaffFeedScopeRotationAndAmbientRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	feeds, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "feed-scope")
	ctx := testpkg.Ctx(t)
	home := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	testpkg.MapAccountToTenant(t, db, account.ID, other)
	homeHash, otherHash := calendarFeedHash(account.ID, 0), calendarFeedHash(account.ID, 1)
	stored, err := feeds.EnsureStaffCalendarFeedToken(ctx, account.ID, home, homeHash)
	require.NoError(t, err)
	require.Equal(t, homeHash, stored)
	stored, err = feeds.EnsureStaffCalendarFeedToken(ctx, account.ID, other, otherHash)
	require.NoError(t, err)
	require.Equal(t, otherHash, stored)
	rollback := errors.New("rollback staff feed rotation")
	rotatedHash := calendarFeedHash(account.ID, 2)
	err = tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		updated, rotateErr := feeds.RotateStaffCalendarFeedToken(txCtx, account.ID, home, rotatedHash)
		require.NoError(t, rotateErr)
		require.True(t, updated)
		_, found, readErr := feeds.FindStaffCalendarFeedOwner(txCtx, homeHash)
		require.NoError(t, readErr)
		require.False(t, found, "rotation immediately invalidates the previous hash")
		owner, found, readErr := feeds.FindStaffCalendarFeedOwner(txCtx, rotatedHash)
		require.NoError(t, readErr)
		require.True(t, found)
		require.Equal(t, home, owner.TenantID)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	for school, hash := range map[int64]string{home: homeHash, other: otherHash} {
		owner, found, readErr := feeds.FindStaffCalendarFeedOwner(ctx, hash)
		require.NoError(t, readErr)
		require.True(t, found)
		require.Equal(t, school, owner.TenantID, "rollback and foreign-school isolation preserve both original hashes")
	}
	_, err = db.NewRaw("UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?", account.ID, home).Exec(ctx)
	require.NoError(t, err)
	_, found, err := feeds.FindStaffCalendarFeedOwner(ctx, homeHash)
	require.NoError(t, err)
	require.False(t, found)
	stored, err = feeds.EnsureStaffCalendarFeedToken(ctx, account.ID, home, rotatedHash)
	require.NoError(t, err)
	require.Empty(t, stored, "inactive mappings cannot create feed capabilities")
	updated, err := feeds.RotateStaffCalendarFeedToken(ctx, account.ID, home, rotatedHash)
	require.NoError(t, err)
	require.False(t, updated, "inactive mappings cannot rotate feed capabilities")
}
