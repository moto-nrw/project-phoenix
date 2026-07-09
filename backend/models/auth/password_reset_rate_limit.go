package auth

import (
	"errors"
	"time"
)

// PasswordResetRateLimit tracks password reset attempts for an email address.
type PasswordResetRateLimit struct {
	Email       string    `bun:"email,pk,notnull" json:"email"`
	Attempts    int       `bun:"attempts,notnull,default:1" json:"attempts"`
	WindowStart time.Time `bun:"window_start,notnull,default:current_timestamp" json:"window_start"`
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

// IncrementAttempts increments the attempts counter in memory.
func (m *PasswordResetRateLimit) IncrementAttempts() {
	m.Attempts++
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
