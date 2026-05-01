package enrollment

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

// SubmissionRateLimitRepository implements per-IP / per-email rate
// limiting for the public enrollment submit endpoint. The single
// upsert query is atomic, so a burst of concurrent requests for the
// same key cannot oversubscribe the bucket.
type SubmissionRateLimitRepository struct {
	db *bun.DB
}

func NewSubmissionRateLimitRepository(db *bun.DB) enrollment.SubmissionRateLimitRepository {
	return &SubmissionRateLimitRepository{db: db}
}

// IncrementAttempts mirrors auth.PasswordResetRateLimitRepository.IncrementAttempts
// but adds tenant scope and a caller-provided window length. The query
// is a single SQL statement so the read-modify-write is atomic at the
// row level.
func (r *SubmissionRateLimitRepository) IncrementAttempts(ctx context.Context, tenantID int64, keyType, keyValue string, window time.Duration) (*enrollment.SubmissionRateLimitState, error) {
	type result struct {
		Attempts int       `bun:"attempts"`
		RetryAt  time.Time `bun:"retry_at"`
	}

	// Postgres `INTERVAL '%d seconds'` is interpolated directly because
	// time.Duration is fully tenant-internal, not user input.
	windowSeconds := int64(window.Seconds())
	if windowSeconds <= 0 {
		return nil, fmt.Errorf("invalid rate-limit window: %s", window)
	}

	query := fmt.Sprintf(`
		WITH upsert AS (
			INSERT INTO enrollment.submission_rate_limits
				(tenant_id, key_type, key_value, attempts, window_start)
			VALUES (?, ?, ?, 1, NOW())
			ON CONFLICT (tenant_id, key_type, key_value) DO UPDATE
			SET attempts = CASE
					WHEN enrollment.submission_rate_limits.window_start > NOW() - INTERVAL '%d seconds'
						THEN enrollment.submission_rate_limits.attempts + 1
					ELSE 1
				END,
				window_start = CASE
					WHEN enrollment.submission_rate_limits.window_start > NOW() - INTERVAL '%d seconds'
						THEN enrollment.submission_rate_limits.window_start
					ELSE NOW()
				END
			RETURNING attempts, window_start + INTERVAL '%d seconds' AS retry_at
		)
		SELECT attempts, retry_at FROM upsert
	`, windowSeconds, windowSeconds, windowSeconds)

	var state result
	if err := base.GetDB(ctx, r.db).NewRaw(query, tenantID, keyType, keyValue).Scan(ctx, &state); err != nil {
		return nil, fmt.Errorf("increment enrollment submission rate limit: %w", err)
	}

	return &enrollment.SubmissionRateLimitState{
		Attempts: state.Attempts,
		RetryAt:  state.RetryAt,
	}, nil
}

// CleanupExpired removes rows whose window_start is older than 24h.
// Scheduler-callable; nothing depends on the historical rows.
func (r *SubmissionRateLimitRepository) CleanupExpired(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
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
