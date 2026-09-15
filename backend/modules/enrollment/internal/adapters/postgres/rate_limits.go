package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

// Store implements per-IP / per-email rate
// limiting for the public enrollment submit endpoint. The single
// upsert query is atomic, so a burst of concurrent requests for the
// same key cannot oversubscribe the bucket.
type Store struct {
	resolve  func(context.Context) (bun.IDB, error)
	tenantID func(context.Context) (int64, error)
}

func NewStore(resolve func(context.Context) (bun.IDB, error), tenantID func(context.Context) (int64, error)) *Store {
	return &Store{resolve: resolve, tenantID: tenantID}
}

// IncrementAttempts mirrors auth.PasswordResetRateLimitRepository.IncrementAttempts
// but adds tenant scope and a caller-provided window length. The query
// is a single SQL statement so the read-modify-write is atomic at the
// row level.
func (r *Store) IncrementAttempts(ctx context.Context, tenantID int64, keyType, keyValue string, window time.Duration) (*enrollment.SubmissionRateLimitState, error) {
	type result struct {
		Attempts int       `bun:"attempts"`
		RetryAt  time.Time `bun:"retry_at"`
	}

	windowSeconds := int64(window.Seconds())
	if windowSeconds <= 0 {
		return nil, fmt.Errorf("invalid rate-limit window: %s", window)
	}

	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	type bucket struct {
		bun.BaseModel `bun:"table:enrollment.submission_rate_limits,alias:submission_rate_limits"`
		TenantID      int64     `bun:"tenant_id"`
		KeyType       string    `bun:"key_type"`
		KeyValue      string    `bun:"key_value"`
		Attempts      int       `bun:"attempts"`
		WindowStart   time.Time `bun:"window_start"`
		UpdatedAt     time.Time `bun:"updated_at"`
	}
	row := bucket{TenantID: tenantID, KeyType: keyType, KeyValue: keyValue, Attempts: 1}
	var state result
	err = db.NewInsert().Model(&row).
		Value("window_start", "NOW()").
		Value("updated_at", "NOW()").
		On("CONFLICT (tenant_id, key_type, key_value) DO UPDATE").
		Set("attempts = CASE WHEN submission_rate_limits.window_start > NOW() - (? * INTERVAL '1 second') THEN submission_rate_limits.attempts + 1 ELSE 1 END", windowSeconds).
		Set("window_start = CASE WHEN submission_rate_limits.window_start > NOW() - (? * INTERVAL '1 second') THEN submission_rate_limits.window_start ELSE NOW() END", windowSeconds).
		Set("updated_at = NOW()").
		Returning("attempts, window_start + (? * INTERVAL '1 second') AS retry_at", windowSeconds).
		Scan(ctx, &state)
	if err != nil {
		return nil, fmt.Errorf("increment enrollment submission rate limit: %w", err)
	}
	return &enrollment.SubmissionRateLimitState{
		Attempts: state.Attempts,
		RetryAt:  state.RetryAt,
	}, nil
}

// CleanupExpired removes rows whose window_start is older than 24h.
// Scheduler-callable; nothing depends on the historical rows.
func (r *Store) CleanupExpired(ctx context.Context) (int, error) {
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	res, err := db.NewDelete().
		Table("enrollment.submission_rate_limits").
		Where("window_start < NOW() - INTERVAL '24 hours'").
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup enrollment submission rate limits: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cleanup enrollment submission rate limits: rows affected: %w", err)
	}
	return int(affected), nil
}
