package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type pinCounter struct {
	Attempts    int        `bun:"pin_attempts"`
	LockedUntil *time.Time `bun:"pin_locked_until"`
}

func readPINCounter(t *testing.T, db *bun.DB, accountID int64) pinCounter {
	t.Helper()
	var counter pinCounter
	require.NoError(t, db.NewRaw(`SELECT pin_attempts, pin_locked_until FROM auth.accounts WHERE id = ?`, accountID).
		Scan(context.Background(), &counter))
	return counter
}

// Moved from the retired AccountRepository.IncrementPINAttempts race test
// (#586, #3225). The model once did a read-modify-write, so two concurrent
// failed PIN entries both read the same count and wrote the same increment,
// and only one of N attempts counted. The single-statement UPDATE must count
// every concurrent failure.
func TestIncrementPINAttemptsIsAtomicUnderRace(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "pin-atomic-counter")
	store := lockingStore(db, testpkg.Tenant(t))

	const (
		concurrency = 12
		threshold   = 5
	)
	lockedUntil := time.Now().Add(15 * time.Minute)

	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			stats, err := store.IncrementPINAttempts(context.Background(), account.ID, threshold, lockedUntil)
			if err == nil && stats.Rows != 1 {
				err = assert.AnError
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err, "every increment must update exactly the account row")
	}

	persisted := readPINCounter(t, db, account.ID)
	assert.Equal(t, concurrency, persisted.Attempts,
		"persisted pin_attempts must equal the goroutine count; anything less means a race loser was dropped")
	require.NotNil(t, persisted.LockedUntil, "lockout must be set after reaching the threshold")
	assert.True(t, persisted.LockedUntil.After(time.Now()), "lockout window must be in the future")
}

// The lock deadline is only written once the post-increment count reaches
// the threshold.
func TestIncrementPINAttemptsLocksOnlyAtTheThreshold(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "pin-atomic-threshold")
	store := lockingStore(db, testpkg.Tenant(t))
	lockedUntil := time.Now().Add(15 * time.Minute)

	for range 2 {
		_, err := store.IncrementPINAttempts(context.Background(), account.ID, 3, lockedUntil)
		require.NoError(t, err)
	}
	before := readPINCounter(t, db, account.ID)
	assert.Equal(t, 2, before.Attempts)
	assert.Nil(t, before.LockedUntil, "below the threshold the account stays unlocked")

	_, err := store.IncrementPINAttempts(context.Background(), account.ID, 3, lockedUntil)
	require.NoError(t, err)
	after := readPINCounter(t, db, account.ID)
	assert.Equal(t, 3, after.Attempts)
	require.NotNil(t, after.LockedUntil)
	assert.WithinDuration(t, lockedUntil, *after.LockedUntil, time.Second)
}

// The successful-verify path: after ResetPINAttempts the counter is 0 and the
// lock is cleared.
func TestResetPINAttemptsClearsCounterAndLock(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "pin-atomic-reset")
	store := lockingStore(db, testpkg.Tenant(t))

	for range 6 {
		_, err := store.IncrementPINAttempts(context.Background(), account.ID, 5, time.Now().Add(15*time.Minute))
		require.NoError(t, err)
	}
	_, err := store.ResetPINAttempts(context.Background(), account.ID)
	require.NoError(t, err)

	persisted := readPINCounter(t, db, account.ID)
	assert.Equal(t, 0, persisted.Attempts, "reset must zero the counter")
	assert.Nil(t, persisted.LockedUntil, "reset must clear the lock timestamp")
}
