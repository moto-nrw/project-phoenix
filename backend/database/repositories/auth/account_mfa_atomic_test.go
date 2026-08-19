package auth_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestAccountRepository_IncrementMFAAttempts_AtomicUnderRace proves the
// fix for #1430 review item #6. Pre-fix the service did:
//
//	account := repo.FindByID(ctx, id)
//	account.IncrementMFAAttempts()            // mutate in memory
//	repo.Update(ctx, account)                 // write whole row
//
// Two concurrent failed verifies both read the same starting count and
// both wrote the same incremented value — only one of N attempts
// counted, letting an attacker burn up to 2x the threshold before
// lockout. The repository method now uses an atomic single-statement
// UPDATE so each call increments exactly once.
//
// This test fires N goroutines in parallel, each calling
// IncrementMFAAttempts. Post-fix the database row must show exactly N
// attempts, and the N return values from the racers must be the unique
// set {1..N}.
func TestAccountRepository_IncrementMFAAttempts_AtomicUnderRace(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-atomic-counter")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	repo := repositories.NewFactory(db).Account

	const (
		concurrency = 12
		threshold   = 5
		lockout     = 15 * time.Minute
	)

	results := make(chan int, concurrency)
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			res, err := repo.IncrementMFAAttempts(context.Background(), acc.ID, threshold, lockout)
			if err != nil {
				errs <- err
				return
			}
			results <- res.Attempts
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err, "no goroutine should error")
	}

	seen := make(map[int]bool, concurrency)
	for n := range results {
		assert.False(t, seen[n], "each goroutine must observe a unique post-update count — duplicate=%d means the UPDATE wasn't atomic", n)
		seen[n] = true
	}
	assert.Len(t, seen, concurrency, "all %d goroutines must report distinct counters from 1..%d", concurrency, concurrency)

	// Sanity-check the persisted row matches the highest count any
	// goroutine saw. The lockout window must be set (count > threshold).
	persisted, err := repo.FindByID(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, concurrency, persisted.MFAAttempts,
		"persisted mfa_attempts must equal goroutine count — anything less means a race-loser was silently dropped")
	require.NotNil(t, persisted.MFALockedUntil, "lockout must be set after exceeding threshold")
	assert.True(t, persisted.MFALockedUntil.After(time.Now()),
		"lockout window must be in the future")
}

// TestAccountRepository_ResetMFAAttempts_ClearsCounterAndLock proves the
// successful-verify path. After ResetMFAAttempts the row's counter is 0
// and the lock cleared.
func TestAccountRepository_ResetMFAAttempts_ClearsCounterAndLock(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-atomic-reset")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	repo := repositories.NewFactory(db).Account

	// Drive the counter past the threshold so a lock is in place.
	for i := 0; i < 6; i++ {
		_, err := repo.IncrementMFAAttempts(context.Background(), acc.ID, 5, 15*time.Minute)
		require.NoError(t, err)
	}

	require.NoError(t, repo.ResetMFAAttempts(context.Background(), acc.ID))

	persisted, err := repo.FindByID(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, persisted.MFAAttempts, "reset must zero the counter")
	assert.Nil(t, persisted.MFALockedUntil, "reset must clear the lock timestamp")
}
