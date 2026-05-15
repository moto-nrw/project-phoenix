package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// SubmissionRateLimitKeyType discriminates between IP-based and
// email-based rate limit rows in the same table.
const (
	SubmissionRateLimitKeyTypeIP    = "ip"
	SubmissionRateLimitKeyTypeEmail = "email"
)

// SubmissionRateLimit is a row in enrollment.submission_rate_limits -
// one per (tenant, key_type, key_value) combination. The repository
// upserts rows atomically so concurrent submissions can't oversubscribe.
type SubmissionRateLimit struct {
	base.Model `bun:"schema:enrollment,table:submission_rate_limits"`
	base.TenantModel
	KeyType     string    `bun:"key_type,notnull" json:"key_type"`
	KeyValue    string    `bun:"key_value,notnull" json:"key_value"`
	Attempts    int       `bun:"attempts,notnull" json:"attempts"`
	WindowStart time.Time `bun:"window_start,notnull,default:current_timestamp" json:"window_start"`
}

// TableName returns the schema-qualified table name.
func (s *SubmissionRateLimit) TableName() string {
	return "enrollment.submission_rate_limits"
}

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
