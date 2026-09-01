package enrollment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// uniqueKeyValue avoids tenant_id+key_type+key_value collisions between
// parallel runs.
func uniqueKeyValue(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func setupRateLimitTest(t *testing.T) (*bun.DB, enrollmentModels.SubmissionRateLimitRepository, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return db, enrollmentRepo.NewSubmissionRateLimitRepository(db), tenantID
}

// runUnscoped runs a closure with NO tenant in context. The rate limit
// repo's raw INSERT addresses tenant_id explicitly, so a tenant tx
// wrapper would just add a wasted RLS predicate. Match production:
// services/enrollment/request_service.go passes the tenant_id as an
// argument rather than via context.
func runUnscoped(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithAdminTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	})
}

// wipeRateLimitsForKey best-effort removes the rows we created so the
// next test starts clean.
func wipeRateLimitsForKey(db *bun.DB, tenantID int64, keyValuePrefix string) {
	_, _ = db.NewDelete().
		Table("enrollment.submission_rate_limits").
		Where("tenant_id = ? AND key_value LIKE ?", tenantID, keyValuePrefix+"%").
		Exec(context.Background())
}

// --- IncrementAttempts -----------------------------------------------

func TestSubmissionRateLimitRepository_IncrementAttempts_FirstCallReturnsOne(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupRateLimitTest(t)
	key := uniqueKeyValue("first")
	defer wipeRateLimitsForKey(db, tenantID, key)

	var state *enrollmentModels.SubmissionRateLimitState
	err := runUnscoped(t, db, func(ctx context.Context) error {
		var iErr error
		state, iErr = repo.IncrementAttempts(ctx, tenantID,
			enrollmentModels.SubmissionRateLimitKeyTypeIP, key, time.Hour)
		return iErr
	})
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 1, state.Attempts, "first hit on a fresh bucket returns 1")
	assert.False(t, state.RetryAt.IsZero(), "RetryAt populated")
}

func TestSubmissionRateLimitRepository_IncrementAttempts_RepeatHitsAccumulate(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupRateLimitTest(t)
	key := uniqueKeyValue("accumulate")
	defer wipeRateLimitsForKey(db, tenantID, key)

	for expected := 1; expected <= 3; expected++ {
		var state *enrollmentModels.SubmissionRateLimitState
		err := runUnscoped(t, db, func(ctx context.Context) error {
			var iErr error
			state, iErr = repo.IncrementAttempts(ctx, tenantID,
				enrollmentModels.SubmissionRateLimitKeyTypeEmail, key, time.Hour)
			return iErr
		})
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.Equal(t, expected, state.Attempts,
			"hit #%d within the window must increment the counter (got %d)", expected, state.Attempts)
	}
}

func TestSubmissionRateLimitRepository_IncrementAttempts_ExpiredWindowResetsToOne(t *testing.T) {
	t.Parallel()

	// Seed a row with an ancient window_start. The upsert's CASE branch
	// MUST detect the expired window and reset attempts back to 1.
	db, repo, tenantID := setupRateLimitTest(t)
	key := uniqueKeyValue("expired")
	defer wipeRateLimitsForKey(db, tenantID, key)

	// Pre-seed an existing row with attempts=5 + ancient window. Raw
	// SQL because the bun builder needs a Model().
	ancient := time.Now().Add(-2 * time.Hour) // older than the 1h window
	_, err := db.NewRaw(`
		INSERT INTO enrollment.submission_rate_limits
		  (tenant_id, key_type, key_value, attempts, window_start)
		VALUES (?, ?, ?, 5, ?)
	`, tenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, key, ancient).
		Exec(context.Background())
	require.NoError(t, err)

	var state *enrollmentModels.SubmissionRateLimitState
	err = runUnscoped(t, db, func(ctx context.Context) error {
		var iErr error
		state, iErr = repo.IncrementAttempts(ctx, tenantID,
			enrollmentModels.SubmissionRateLimitKeyTypeIP, key, time.Hour)
		return iErr
	})
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 1, state.Attempts,
		"expired window must reset attempts to 1 (parent gets a fresh budget)")
}

func TestSubmissionRateLimitRepository_IncrementAttempts_SeparateKeysSeparateBuckets(t *testing.T) {
	t.Parallel()

	// Two different key_values must NOT share a counter. Catches the
	// "we accidentally dropped key_value from the ON CONFLICT" class of
	// bug.
	db, repo, tenantID := setupRateLimitTest(t)
	keyA := uniqueKeyValue("bucketA")
	keyB := uniqueKeyValue("bucketB")
	defer wipeRateLimitsForKey(db, tenantID, keyA)
	defer wipeRateLimitsForKey(db, tenantID, keyB)

	for i := 0; i < 3; i++ {
		_, err := repoIncrement(t, db, repo, tenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, keyA, time.Hour)
		require.NoError(t, err)
	}
	stateB, err := repoIncrement(t, db, repo, tenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, keyB, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, stateB.Attempts, "second key starts at 1 even when first key has 3 hits")
}

func TestSubmissionRateLimitRepository_IncrementAttempts_SeparateKeyTypesSeparateBuckets(t *testing.T) {
	t.Parallel()

	// Same key_value but different key_type (ip vs email) must NOT
	// share a counter — UNIQUE is on (tenant, type, value).
	db, repo, tenantID := setupRateLimitTest(t)
	key := uniqueKeyValue("keytypeshare")
	defer wipeRateLimitsForKey(db, tenantID, key)

	for i := 0; i < 3; i++ {
		_, err := repoIncrement(t, db, repo, tenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, key, time.Hour)
		require.NoError(t, err)
	}
	stateEmail, err := repoIncrement(t, db, repo, tenantID, enrollmentModels.SubmissionRateLimitKeyTypeEmail, key, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, stateEmail.Attempts,
		"different key_type must have its own bucket — same key_value MUST NOT leak across types")
}

func TestSubmissionRateLimitRepository_IncrementAttempts_SeparateTenantsSeparateBuckets(t *testing.T) {
	t.Parallel()

	// Tenant isolation — same key in tenant A and tenant B must NOT
	// share a counter.
	db, repo, _ := setupRateLimitTest(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	key := uniqueKeyValue("tenantshare")
	defer wipeRateLimitsForKey(db, tenantA, key)
	defer wipeRateLimitsForKey(db, tenantB, key)

	for i := 0; i < 3; i++ {
		_, err := repoIncrement(t, db, repo, tenantA, enrollmentModels.SubmissionRateLimitKeyTypeIP, key, time.Hour)
		require.NoError(t, err)
	}
	stateB, err := repoIncrement(t, db, repo, tenantB, enrollmentModels.SubmissionRateLimitKeyTypeIP, key, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, stateB.Attempts,
		"different tenant must have its own bucket — rate limiting is per-school")
}

func TestSubmissionRateLimitRepository_IncrementAttempts_ZeroWindowRejected(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupRateLimitTest(t)
	err := runUnscoped(t, db, func(ctx context.Context) error {
		_, iErr := repo.IncrementAttempts(ctx, tenantID,
			enrollmentModels.SubmissionRateLimitKeyTypeIP, "x", 0)
		return iErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "window")
}

func TestSubmissionRateLimitRepository_IncrementAttempts_NegativeWindowRejected(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupRateLimitTest(t)
	err := runUnscoped(t, db, func(ctx context.Context) error {
		_, iErr := repo.IncrementAttempts(ctx, tenantID,
			enrollmentModels.SubmissionRateLimitKeyTypeIP, "x", -1*time.Hour)
		return iErr
	})
	require.Error(t, err)
}

func TestSubmissionRateLimitRepository_IncrementAttempts_RetryAtIsWindowStartPlusWindow(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	// The retry_at the repo returns must equal window_start + window —
	// rate-limited callers use it as a Retry-After hint and we don't
	// want to leak more (or less) than the actual reset time.
	db, repo, tenantID := setupRateLimitTest(t)
	key := uniqueKeyValue("retryat")
	defer wipeRateLimitsForKey(db, tenantID, key)

	state, err := repoIncrement(t, db, repo, tenantID,
		enrollmentModels.SubmissionRateLimitKeyTypeIP, key, time.Hour)
	require.NoError(t, err)

	// Pull window_start back out of the DB and compare.
	var windowStart time.Time
	require.NoError(t, db.NewSelect().
		ColumnExpr("window_start").
		TableExpr("enrollment.submission_rate_limits").
		Where("tenant_id = ? AND key_type = ? AND key_value = ?",
			tenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, key).
		Scan(context.Background(), &windowStart))

	// RetryAt - WindowStart should be ~1h.
	diff := state.RetryAt.Sub(windowStart)
	assert.InDelta(t, time.Hour.Seconds(), diff.Seconds(), 1,
		"RetryAt MUST equal window_start + window (got diff %s)", diff)
}

// --- CleanupExpired ---------------------------------------------------

func TestSubmissionRateLimitRepository_CleanupExpired_RemovesAncientRows(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, repo, tenantID := setupRateLimitTest(t)
	ancientKey := uniqueKeyValue("ancient")
	freshKey := uniqueKeyValue("fresh")
	defer wipeRateLimitsForKey(db, tenantID, ancientKey)
	defer wipeRateLimitsForKey(db, tenantID, freshKey)

	// Insert an ancient row (>24h) and a fresh row. Raw SQL because
	// the bun builder needs a Model() and we want to backdate
	// window_start directly without a struct hook fighting us.
	ancient := time.Now().Add(-25 * time.Hour)
	fresh := time.Now().Add(-1 * time.Hour)
	for _, row := range []struct {
		key   string
		start time.Time
	}{
		{ancientKey, ancient},
		{freshKey, fresh},
	} {
		_, err := db.NewRaw(`
			INSERT INTO enrollment.submission_rate_limits
			  (tenant_id, key_type, key_value, attempts, window_start)
			VALUES (?, ?, ?, 1, ?)
		`, tenantID, enrollmentModels.SubmissionRateLimitKeyTypeIP, row.key, row.start).
			Exec(context.Background())
		require.NoError(t, err)
	}

	var n int
	err := runUnscoped(t, db, func(ctx context.Context) error {
		var cErr error
		n, cErr = repo.CleanupExpired(ctx)
		return cErr
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1, "at least the ancient row must be deleted")

	// Confirm the ancient row is gone, the fresh row remains.
	c, err := db.NewSelect().
		Table("enrollment.submission_rate_limits").
		Where("tenant_id = ? AND key_value = ?", tenantID, ancientKey).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, c, "ancient row MUST be deleted by CleanupExpired")

	c, err = db.NewSelect().
		Table("enrollment.submission_rate_limits").
		Where("tenant_id = ? AND key_value = ?", tenantID, freshKey).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, c, "fresh row MUST survive CleanupExpired")
}

func TestSubmissionRateLimitRepository_CleanupExpired_ZeroAffectedIsNoError(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	// Running CleanupExpired on an empty (or all-fresh) table must
	// return (0, nil) — scheduler-callable, must not error out when
	// there's nothing to clean.
	db, repo, _ := setupRateLimitTest(t)
	var n int
	err := runUnscoped(t, db, func(ctx context.Context) error {
		var cErr error
		n, cErr = repo.CleanupExpired(ctx)
		return cErr
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 0)
}

// --- helpers ----------------------------------------------------------

// repoIncrement reduces the per-call boilerplate in the bucket-isolation
// tests by wrapping the runUnscoped + IncrementAttempts dance.
func repoIncrement(
	t *testing.T, db *bun.DB,
	repo enrollmentModels.SubmissionRateLimitRepository,
	tenantID int64, keyType, keyValue string, window time.Duration,
) (*enrollmentModels.SubmissionRateLimitState, error) {
	t.Helper()
	var state *enrollmentModels.SubmissionRateLimitState
	err := runUnscoped(t, db, func(ctx context.Context) error {
		var iErr error
		state, iErr = repo.IncrementAttempts(ctx, tenantID, keyType, keyValue, window)
		return iErr
	})
	return state, err
}
