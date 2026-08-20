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

// TestAccountRepository_IncrementPINAttempts_AtomicUnderRace proves the fix
// for issue #586. The model previously did a read-modify-write:
//
//	account := repo.FindByID(ctx, id)
//	account.IncrementPINAttempts()            // mutate in memory
//	repo.Update(ctx, account)                 // write whole row
//
// Two concurrent failed PIN entries both read the same starting count and
// both wrote the same incremented value, so only one of N attempts counted —
// letting an attacker burn up to 2x the threshold before lockout. The atomic
// single-statement UPDATE now increments exactly once per call.
//
// This mirrors the MFA atomic race test: N goroutines call
// IncrementPINAttempts in parallel; post-fix the persisted row must show
// exactly N attempts and the N return values must be the unique set {1..N}.
func TestAccountRepository_IncrementPINAttempts_AtomicUnderRace(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	acc := testpkg.CreateTestAccount(t, db, "pin-atomic-counter")
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
			res, err := repo.IncrementPINAttempts(context.Background(), acc.ID, threshold, lockout)
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

	persisted, err := repo.FindByID(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, concurrency, persisted.PINAttempts,
		"persisted pin_attempts must equal goroutine count — anything less means a race-loser was silently dropped")
	require.NotNil(t, persisted.PINLockedUntil, "lockout must be set after exceeding threshold")
	assert.True(t, persisted.PINLockedUntil.After(time.Now()),
		"lockout window must be in the future")
}

// TestAccountRepository_ResetPINAttempts_ClearsCounterAndLock proves the
// successful-verify path. After ResetPINAttempts the counter is 0 and the lock
// is cleared.
func TestAccountRepository_ResetPINAttempts_ClearsCounterAndLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	acc := testpkg.CreateTestAccount(t, db, "pin-atomic-reset")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	repo := repositories.NewFactory(db).Account

	for i := 0; i < 6; i++ {
		_, err := repo.IncrementPINAttempts(context.Background(), acc.ID, 5, 15*time.Minute)
		require.NoError(t, err)
	}

	require.NoError(t, repo.ResetPINAttempts(context.Background(), acc.ID))

	persisted, err := repo.FindByID(context.Background(), acc.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, persisted.PINAttempts, "reset must zero the counter")
	assert.Nil(t, persisted.PINLockedUntil, "reset must clear the lock timestamp")
}
