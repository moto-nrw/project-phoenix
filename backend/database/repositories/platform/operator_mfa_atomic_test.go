package platform_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOperatorRepository_IncrementMFAAttempts_AtomicUnderRace mirrors the
// tenant-side test for #1430 review item #6: N concurrent operator MFA
// failures must produce N counted attempts, not 1. Pre-fix the
// read-modify-write at the service layer let racing goroutines all see
// the same starting count and overwrite each other.
func TestOperatorRepository_IncrementMFAAttempts_AtomicUnderRace(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	op := createTestOperatorForMFAAtomic(t, db, "mfa-atomic-counter")
	t.Cleanup(func() { cleanupTestOperatorForMFAAtomic(t, db, op.ID) })

	repo := repositories.NewFactory(db).Operator

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
			res, err := repo.IncrementMFAAttempts(context.Background(), op.ID, threshold, lockout)
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
		require.NoError(t, err)
	}

	seen := make(map[int]bool, concurrency)
	for n := range results {
		assert.False(t, seen[n], "each operator increment must observe a unique post-update count — duplicate=%d means the UPDATE wasn't atomic", n)
		seen[n] = true
	}
	assert.Len(t, seen, concurrency)

	persisted, err := repo.FindByID(context.Background(), op.ID)
	require.NoError(t, err)
	assert.Equal(t, concurrency, persisted.MFAAttempts,
		"persisted mfa_attempts must equal goroutine count — any drop means a race-loser was silently lost")
	require.NotNil(t, persisted.MFALockedUntil)
	assert.True(t, persisted.MFALockedUntil.After(time.Now()))
}

func TestOperatorRepository_ResetMFAAttempts_ClearsCounterAndLock(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	op := createTestOperatorForMFAAtomic(t, db, "mfa-atomic-reset")
	t.Cleanup(func() { cleanupTestOperatorForMFAAtomic(t, db, op.ID) })

	repo := repositories.NewFactory(db).Operator

	for i := 0; i < 6; i++ {
		_, err := repo.IncrementMFAAttempts(context.Background(), op.ID, 5, 15*time.Minute)
		require.NoError(t, err)
	}

	require.NoError(t, repo.ResetMFAAttempts(context.Background(), op.ID))

	persisted, err := repo.FindByID(context.Background(), op.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, persisted.MFAAttempts)
	assert.Nil(t, persisted.MFALockedUntil)
}

func createTestOperatorForMFAAtomic(t *testing.T, db *bun.DB, slug string) *platformModels.Operator {
	t.Helper()
	op := &platformModels.Operator{
		Email:        fmt.Sprintf("op-mfa-atomic-%s-%d@test.local", slug, time.Now().UnixNano()),
		DisplayName:  "MFA Atomic Test Operator",
		PasswordHash: "$argon2id$placeholder-mfa-atomic",
		Active:       true,
	}
	_, err := db.NewInsert().
		Model(op).
		ModelTableExpr("platform.operators").
		Exec(context.Background())
	require.NoError(t, err)
	return op
}

func cleanupTestOperatorForMFAAtomic(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	_, _ = db.NewDelete().
		Model((*platformModels.Operator)(nil)).
		ModelTableExpr("platform.operators").
		Where("id = ?", operatorID).
		Exec(context.Background())
}
