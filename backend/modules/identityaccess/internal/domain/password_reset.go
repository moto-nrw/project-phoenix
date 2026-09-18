package domain

import (
	"errors"
	"time"
)

// PasswordResetScope names the portal a reset link belongs to. The parent and
// school scopes only serve accounts that hold a role of that portal, so a
// link never lands on a host the account cannot sign in to.
type PasswordResetScope string

const (
	PasswordResetScopeStaff  PasswordResetScope = "staff"
	PasswordResetScopeParent PasswordResetScope = "parent"
	PasswordResetScopeSchool PasswordResetScope = "school"
)

// PasswordResetRateLimitThreshold is the number of reset requests per e-mail
// address the rolling one-hour window allows. The frontend countdown reads
// the Retry-After the rejection carries, so the value is a cross-layer
// contract.
const PasswordResetRateLimitThreshold = 3

// ErrPasswordResetRateLimited reports a reset request beyond the threshold.
var ErrPasswordResetRateLimited = errors.New("too many password reset requests")

// PasswordResetRateLimitError carries the window a rejected request reports.
type PasswordResetRateLimitError struct {
	Attempts int
	RetryAt  time.Time
}

func (e *PasswordResetRateLimitError) Error() string { return ErrPasswordResetRateLimited.Error() }

func (e *PasswordResetRateLimitError) Unwrap() error { return ErrPasswordResetRateLimited }

// PasswordResetWindow is the per-address state of the reset rate limit.
type PasswordResetWindow struct {
	Attempts int
	RetryAt  time.Time
}

// Exceeded reports whether the window rejects a request at now once it holds
// more than allowed attempts.
func (w PasswordResetWindow) Exceeded(allowed int, now time.Time) bool {
	return w.Attempts > allowed && w.RetryAt.After(now)
}

// PasswordResetToken is one auth.password_reset_tokens row: a one-time link
// that sets a new password while it is unused and unexpired.
type PasswordResetToken struct {
	ID        int64
	AccountID int64
	Token     string
	Expiry    time.Time
	Used      bool
	Delivery  TokenDelivery
	CreatedAt time.Time
}

// Validate rejects a reset link that cannot be stored; a link is never
// stored already expired or used.
func (t *PasswordResetToken) Validate(now time.Time) error {
	switch {
	case t.AccountID <= 0:
		return errors.New("account ID is required")
	case t.Token == "":
		return errors.New("token value is required")
	case !t.Expiry.After(now):
		return errors.New("token has already expired")
	case t.Used:
		return errors.New("token has already been used")
	}
	return nil
}
