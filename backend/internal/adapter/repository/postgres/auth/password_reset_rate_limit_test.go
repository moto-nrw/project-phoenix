package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Test Helpers
// ============================================================================

// cleanupRateLimitRecords removes rate limit records by email.
func cleanupRateLimitRecords(t *testing.T, db *bun.DB, emails ...string) {
	t.Helper()
	if len(emails) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("auth.password_reset_rate_limits").
		Where("email IN (?)", bun.In(emails)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup rate limits: %v", err)
	}
}

// ============================================================================
// CheckRateLimit Tests
// ============================================================================

func TestPasswordResetRateLimitRepository_CheckRateLimit_NoRecord(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	// Use unique email to avoid conflicts
	email := fmt.Sprintf("no-record-%d@example.com", time.Now().UnixNano())

	// ACT
	state, err := repo.CheckRateLimit(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, 0, state.Attempts)
	assert.True(t, state.RetryAt.Before(time.Now().Add(time.Second)))
}

func TestPasswordResetRateLimitRepository_CheckRateLimit_ExistingRecord(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	// Create a rate limit record
	email := fmt.Sprintf("existing-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, email)

	// First, create a record via IncrementAttempts
	_, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)

	// ACT
	state, err := repo.CheckRateLimit(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, 1, state.Attempts)
	assert.True(t, state.RetryAt.After(time.Now()))
}

// ============================================================================
// IncrementAttempts Tests
// ============================================================================

func TestPasswordResetRateLimitRepository_IncrementAttempts_FirstAttempt(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	email := fmt.Sprintf("first-attempt-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, email)

	// ACT
	state, err := repo.IncrementAttempts(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, 1, state.Attempts)
	assert.True(t, state.RetryAt.After(time.Now()))
}

func TestPasswordResetRateLimitRepository_IncrementAttempts_MultipleAttempts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	email := fmt.Sprintf("multi-attempt-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, email)

	// ACT - Make multiple attempts
	state1, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 1, state1.Attempts)

	state2, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 2, state2.Attempts)

	state3, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 3, state3.Attempts)

	// Verify via CheckRateLimit
	finalState, err := repo.CheckRateLimit(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 3, finalState.Attempts)
}

func TestPasswordResetRateLimitRepository_IncrementAttempts_WindowReset(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	email := fmt.Sprintf("window-reset-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, email)

	// Create old record with expired window using raw SQL
	oldWindowStart := time.Now().Add(-2 * time.Hour) // 2 hours ago (outside 1 hour window)
	_, err := db.NewRaw(`
		INSERT INTO auth.password_reset_rate_limits (email, attempts, window_start)
		VALUES (?, ?, ?)
		ON CONFLICT (email) DO UPDATE
		SET attempts = 5, window_start = ?
	`, email, 5, oldWindowStart, oldWindowStart).Exec(ctx)
	require.NoError(t, err)

	// ACT - This should reset the counter since window expired
	state, err := repo.IncrementAttempts(ctx, email)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, state.Attempts) // Should be reset to 1, not 6
}

// ============================================================================
// CleanupExpired Tests
// ============================================================================

func TestPasswordResetRateLimitRepository_CleanupExpired_Success(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	// Create an old record using raw SQL
	expiredEmail := fmt.Sprintf("expired-%d@example.com", time.Now().UnixNano())
	recentEmail := fmt.Sprintf("recent-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, expiredEmail, recentEmail)

	// Insert expired record (window started 25 hours ago)
	oldWindowStart := time.Now().Add(-25 * time.Hour)
	_, err := db.NewRaw(`
		INSERT INTO auth.password_reset_rate_limits (email, attempts, window_start)
		VALUES (?, ?, ?)
	`, expiredEmail, 3, oldWindowStart).Exec(ctx)
	require.NoError(t, err)

	// Insert recent record
	_, err = repo.IncrementAttempts(ctx, recentEmail)
	require.NoError(t, err)

	// ACT
	deleted, err := repo.CleanupExpired(ctx)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 1)

	// Verify expired record is gone
	expiredState, err := repo.CheckRateLimit(ctx, expiredEmail)
	require.NoError(t, err)
	assert.Equal(t, 0, expiredState.Attempts) // No record = 0 attempts

	// Verify recent record still exists
	recentState, err := repo.CheckRateLimit(ctx, recentEmail)
	require.NoError(t, err)
	assert.Equal(t, 1, recentState.Attempts)
}

func TestPasswordResetRateLimitRepository_CleanupExpired_NoExpiredRecords(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	// Create only a recent record
	recentEmail := fmt.Sprintf("only-recent-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, recentEmail)

	_, err := repo.IncrementAttempts(ctx, recentEmail)
	require.NoError(t, err)

	// ACT - should work even with no expired records
	deleted, err := repo.CleanupExpired(ctx)

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, deleted, 0) // May or may not delete anything

	// Verify recent record still exists
	state, err := repo.CheckRateLimit(ctx, recentEmail)
	require.NoError(t, err)
	assert.Equal(t, 1, state.Attempts)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestPasswordResetRateLimitRepository_RateLimitFlow(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).PasswordResetRateLimit
	ctx := context.Background()

	email := fmt.Sprintf("full-flow-%d@example.com", time.Now().UnixNano())
	defer cleanupRateLimitRecords(t, db, email)

	// 1. Initially no record
	initialState, err := repo.CheckRateLimit(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 0, initialState.Attempts)

	// 2. First request
	state1, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 1, state1.Attempts)

	// 3. Second request
	state2, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 2, state2.Attempts)

	// 4. Third request (should be at limit - 3 per hour)
	state3, err := repo.IncrementAttempts(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 3, state3.Attempts)

	// 5. Check state via CheckRateLimit
	finalState, err := repo.CheckRateLimit(ctx, email)
	require.NoError(t, err)
	assert.Equal(t, 3, finalState.Attempts)
	assert.True(t, finalState.RetryAt.After(time.Now()))
}
