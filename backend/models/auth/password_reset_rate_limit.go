package auth

import "time"

// PasswordResetRateLimit tracks password reset attempts for an email address.
type PasswordResetRateLimit struct {
	Email       string    `bun:"email,pk,notnull" json:"email"`
	Attempts    int       `bun:"attempts,notnull,default:1" json:"attempts"`
	WindowStart time.Time `bun:"window_start,notnull,default:current_timestamp" json:"window_start"`
}

// RateLimitState represents the rate limit metadata returned to services.
type RateLimitState struct {
	Attempts int
	RetryAt  time.Time
}
