package enrollment

import (
	"context"
	"time"
)

// SubmissionRateLimitKeyType discriminates between IP-based and
// email-based rate limit rows in the same table.
const (
	SubmissionRateLimitKeyTypeIP    = "ip"
	SubmissionRateLimitKeyTypeEmail = "email"
)

// SubmissionRateLimitState is the post-increment view the service uses
// to decide whether to admit a submission.
type SubmissionRateLimitState struct {
	Attempts int
	RetryAt  time.Time
}

// SubmissionRateLimitRepository is the DB contract the request service
// uses to enforce per-IP / per-email throttling. The window length is
// caller-provided so one repo serves both IP (1h window) and email
// (24h window) buckets without separate methods.
type SubmissionRateLimitRepository interface {
	// IncrementAttempts atomically increments the counter for the given
	// (tenant, keyType, keyValue) bucket and returns the new state.
	// When the existing window is older than `window`, the counter
	// resets to 1 and window_start is bumped to NOW().
	IncrementAttempts(ctx context.Context, tenantID int64, keyType, keyValue string, window time.Duration) (*SubmissionRateLimitState, error)

	// CleanupExpired drops rows whose window started more than 24h ago
	// (well past the longest live window). Scheduler-callable.
	CleanupExpired(ctx context.Context) (int, error)
}
