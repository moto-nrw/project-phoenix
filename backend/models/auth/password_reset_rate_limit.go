package auth

import (
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// PasswordResetRateLimit tracks password reset attempts for an email address.
type PasswordResetRateLimit struct {
	Email       string    `bun:"email,pk,notnull" json:"email"`
	Attempts    int       `bun:"attempts,notnull,default:1" json:"attempts"`
	WindowStart time.Time `bun:"window_start,notnull,default:current_timestamp" json:"window_start"`
}

// TableName returns the fully-qualified table name.
func (PasswordResetRateLimit) TableName() string {
	return `auth.password_reset_rate_limits`
}

// BeforeAppendModel ensures the schema-qualified table name is used with an alias.
func (m *PasswordResetRateLimit) BeforeAppendModel(query any) error {
	const tableExpr = `auth.password_reset_rate_limits AS "rate_limit"`

	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(tableExpr)
	case *bun.InsertQuery:
		q.ModelTableExpr(tableExpr)
	case *bun.UpdateQuery:
		q.ModelTableExpr(tableExpr)
	case *bun.DeleteQuery:
		q.ModelTableExpr(tableExpr)
	}
	return nil
}

// Validate ensures the rate limit record contains the required fields.
func (m *PasswordResetRateLimit) Validate() error {
	if m.Email == "" {
		return errors.New("email is required")
	}
	if m.Attempts < 0 {
		return errors.New("attempts cannot be negative")
	}
	return nil
}

// IsExpired reports whether the rate limit window has elapsed for the default one-hour window.
func (m *PasswordResetRateLimit) IsExpired(now time.Time) bool {
	return m.WindowStart.Add(time.Hour).Before(now)
}

// IncrementAttempts increments the attempts counter in memory.
func (m *PasswordResetRateLimit) IncrementAttempts() {
	m.Attempts++
}

// Reset clears the attempts counter and starts a new window.
func (m *PasswordResetRateLimit) Reset() {
	m.Attempts = 1
	m.WindowStart = time.Now().UTC()
}

// RateLimitState represents the rate limit metadata returned to services.
type RateLimitState struct {
	Attempts int
	RetryAt  time.Time
}

// RetryAfterSeconds returns the positive number of seconds until retry, or zero if already available.
func (s RateLimitState) RetryAfterSeconds(now time.Time) int {
	if s.RetryAt.IsZero() {
		return 0
	}
	if !s.RetryAt.After(now) {
		return 0
	}
	return int(s.RetryAt.Sub(now).Seconds())
}
