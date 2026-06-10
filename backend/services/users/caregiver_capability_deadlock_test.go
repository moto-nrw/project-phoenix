package users

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Tests for the deadlock-retry wrapper around DisableCaregiverCapability
// (issue #1619). pgdriver.Error cannot be constructed outside the driver
// package, so the classifier is exercised against a genuine deadlock
// provoked between two transactions on private scratch tables; the captured
// error then drives the retry-loop tests.

var (
	deadlockProbeOnce sync.Once
	deadlockProbeErr  error
)

// provokeDeadlock forces a real 40P01 by locking two scratch tables in
// opposite order from two transactions. The tables are private to this test
// file, so no other package can contend on them.
func provokeDeadlock(tb testing.TB, db *bun.DB) error {
	tb.Helper()
	ctx := context.Background()

	_, err := db.ExecContext(ctx,
		"CREATE TABLE IF NOT EXISTS users_deadlock_probe_a (id int); CREATE TABLE IF NOT EXISTS users_deadlock_probe_b (id int)")
	require.NoError(tb, err)
	defer func() {
		_, _ = db.ExecContext(context.Background(),
			"DROP TABLE IF EXISTS users_deadlock_probe_a; DROP TABLE IF EXISTS users_deadlock_probe_b")
	}()

	tx1, err := db.BeginTx(ctx, nil)
	require.NoError(tb, err)
	defer func() { _ = tx1.Rollback() }()
	tx2, err := db.BeginTx(ctx, nil)
	require.NoError(tb, err)
	defer func() { _ = tx2.Rollback() }()

	_, err = tx1.ExecContext(ctx, "LOCK TABLE users_deadlock_probe_a IN ACCESS EXCLUSIVE MODE")
	require.NoError(tb, err)
	_, err = tx2.ExecContext(ctx, "LOCK TABLE users_deadlock_probe_b IN ACCESS EXCLUSIVE MODE")
	require.NoError(tb, err)

	errCh := make(chan error, 2)
	go func() {
		_, lockErr := tx1.ExecContext(ctx, "LOCK TABLE users_deadlock_probe_b IN ACCESS EXCLUSIVE MODE")
		errCh <- lockErr
	}()
	go func() {
		_, lockErr := tx2.ExecContext(ctx, "LOCK TABLE users_deadlock_probe_a IN ACCESS EXCLUSIVE MODE")
		errCh <- lockErr
	}()

	first, second := <-errCh, <-errCh
	if first != nil {
		return first
	}
	return second
}

// capturedDeadlockError provokes the deadlock once per test binary and
// returns the captured driver error.
func capturedDeadlockError(t *testing.T, db *bun.DB) error {
	t.Helper()
	deadlockProbeOnce.Do(func() {
		deadlockProbeErr = provokeDeadlock(t, db)
	})
	require.Error(t, deadlockProbeErr, "expected to capture a real deadlock error")
	return deadlockProbeErr
}

func TestIsDeadlockError_ClassifiesRealDeadlock(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	deadlockErr := capturedDeadlockError(t, db)

	assert.True(t, isDeadlockError(deadlockErr), "real 40P01 must be classified as deadlock, got: %v", deadlockErr)
	assert.True(t, isDeadlockError(fmt.Errorf("users.disable caregiver capability: lock tables: %w", deadlockErr)),
		"wrapped 40P01 must still be classified via errors.As")
	assert.False(t, isDeadlockError(errors.New("deadlock detected")), "plain text must not satisfy the typed check")
	assert.False(t, isDeadlockError(nil))
}

func TestRunDisableTxWithDeadlockRetry_RetriesThenSucceeds(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	deadlockErr := capturedDeadlockError(t, db)
	svc := &caregiverCapabilityService{txHandler: modelBase.NewTxHandler(db)}

	attempts := 0
	err := svc.runDisableTxWithDeadlockRetry(context.Background(), func(context.Context, bun.Tx) error {
		attempts++
		if attempts < 3 {
			return deadlockErr
		}
		return nil
	})

	require.NoError(t, err, "the third attempt succeeds, so the operation must succeed")
	assert.Equal(t, 3, attempts)
}

func TestRunDisableTxWithDeadlockRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	deadlockErr := capturedDeadlockError(t, db)
	svc := &caregiverCapabilityService{txHandler: modelBase.NewTxHandler(db)}

	attempts := 0
	err := svc.runDisableTxWithDeadlockRetry(context.Background(), func(context.Context, bun.Tx) error {
		attempts++
		return deadlockErr
	})

	require.Error(t, err)
	assert.True(t, isDeadlockError(err), "the final deadlock error must surface unchanged")
	assert.Equal(t, caregiverDisableDeadlockAttempts, attempts)
}

func TestRunDisableTxWithDeadlockRetry_NonDeadlockErrorNotRetried(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := &caregiverCapabilityService{txHandler: modelBase.NewTxHandler(db)}
	boom := errors.New("validation failed")

	attempts := 0
	err := svc.runDisableTxWithDeadlockRetry(context.Background(), func(context.Context, bun.Tx) error {
		attempts++
		return boom
	})

	require.ErrorIs(t, err, boom)
	assert.Equal(t, 1, attempts, "only deadlock aborts may be retried")
}

func TestRunDisableTxWithDeadlockRetry_NoRetryInsideOuterTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	deadlockErr := capturedDeadlockError(t, db)
	svc := &caregiverCapabilityService{txHandler: modelBase.NewTxHandler(db)}

	outerTx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer func() { _ = outerTx.Rollback() }()
	ctx := modelBase.ContextWithTx(context.Background(), &outerTx)

	attempts := 0
	err = svc.runDisableTxWithDeadlockRetry(ctx, func(context.Context, bun.Tx) error {
		attempts++
		return deadlockErr
	})

	require.Error(t, err)
	assert.Equal(t, 1, attempts,
		"with an outer transaction the abort poisons the whole tx — retrying inside would re-run on a dead transaction")
}
