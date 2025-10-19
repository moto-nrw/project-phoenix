package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	modelAuth "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const passwordResetWindow = time.Hour

// PasswordResetRateLimitRepository provides per-email rate limiting for password resets.
type PasswordResetRateLimitRepository struct {
	db *bun.DB
}

// NewPasswordResetRateLimitRepository creates a new rate limit repository.
func NewPasswordResetRateLimitRepository(db *bun.DB) modelAuth.PasswordResetRateLimitRepository {
	return &PasswordResetRateLimitRepository{db: db}
}

// CheckRateLimit returns the current rate limit state for the provided email.
func (r *PasswordResetRateLimitRepository) CheckRateLimit(ctx context.Context, email string) (*modelAuth.RateLimitState, error) {
	record := new(modelAuth.PasswordResetRateLimit)
	err := r.db.NewSelect().
		Model(record).
		ModelTableExpr(`auth.password_reset_rate_limits AS "password_reset_rate_limit"`).
		Where(`"password_reset_rate_limit".email = ?`, email).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &modelAuth.RateLimitState{
				Attempts: 0,
				RetryAt:  time.Now(),
			}, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "check password reset rate limit",
			Err: err,
		}
	}

	return &modelAuth.RateLimitState{
		Attempts: record.Attempts,
		RetryAt:  record.WindowStart.Add(passwordResetWindow),
	}, nil
}

// IncrementAttempts increments the attempt counter and returns the new rate limit state.
func (r *PasswordResetRateLimitRepository) IncrementAttempts(ctx context.Context, email string) (*modelAuth.RateLimitState, error) {
	type result struct {
		Attempts int       `bun:"attempts"`
		RetryAt  time.Time `bun:"retry_at"`
	}

	var state result
	query := `
		WITH upsert AS (
			INSERT INTO auth.password_reset_rate_limits (email, attempts, window_start)
			VALUES (?, 1, NOW())
			ON CONFLICT (email) DO UPDATE
			SET attempts = CASE
					WHEN auth.password_reset_rate_limits.window_start > NOW() - INTERVAL '1 hour'
						THEN auth.password_reset_rate_limits.attempts + 1
					ELSE 1
				END,
				window_start = CASE
					WHEN auth.password_reset_rate_limits.window_start > NOW() - INTERVAL '1 hour'
						THEN auth.password_reset_rate_limits.window_start
					ELSE NOW()
				END
			RETURNING attempts, window_start + INTERVAL '1 hour' AS retry_at
		)
		SELECT attempts, retry_at FROM upsert
	`

	if err := r.db.NewRaw(query, email).Scan(ctx, &state); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "increment password reset rate limit",
			Err: err,
		}
	}

	return &modelAuth.RateLimitState{
		Attempts: state.Attempts,
		RetryAt:  state.RetryAt,
	}, nil
}

// CleanupExpired removes rate limit records older than 24 hours to keep the table compact.
func (r *PasswordResetRateLimitRepository) CleanupExpired(ctx context.Context) (int, error) {
	res, err := r.db.NewDelete().
		Table("auth.password_reset_rate_limits").
		Where("window_start < NOW() - INTERVAL '24 hours'").
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "cleanup password reset rate limits",
			Err: err,
		}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to read rows affected for rate limit cleanup: %w", err)
	}

	return int(affected), nil
}
