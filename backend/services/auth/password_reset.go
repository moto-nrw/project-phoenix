package auth

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Password reset is served by Identity & Access (#2722): issuing the link,
// the per-address rate limit, setting the new password with the session
// revocation, and the cleanup. The methods below delegate to the consumer-
// owned port the composition root binds, so the retained reset routes keep
// their contract.

// PasswordResetScope names the portal a reset link opens.
type PasswordResetScope string

const (
	PasswordResetScopeStaff  PasswordResetScope = "staff"
	PasswordResetScopeParent PasswordResetScope = "parent"
	PasswordResetScopeSchool PasswordResetScope = "school"
)

// ErrPasswordResetUnavailable reports a service composed without the
// Identity & Access password reset port.
var ErrPasswordResetUnavailable = errors.New("password reset is not composed")

// PasswordResetLink is an issued reset link as the owner reports it.
type PasswordResetLink struct {
	ID              int64
	AccountID       int64
	Token           string
	Expiry          time.Time
	CreatedAt       time.Time
	EmailSentAt     *time.Time
	EmailError      *string
	EmailRetryCount int
}

// PasswordResets is the consumer-owned port over the Identity & Access
// password reset capability. Errors arrive in the AuthError envelope with
// this package's sentinels; a rate-limit rejection is a *RateLimitError.
type PasswordResets interface {
	// InitiatePasswordReset returns (nil, nil) when no link was issued.
	InitiatePasswordReset(ctx context.Context, email string, scope PasswordResetScope) (*PasswordResetLink, error)
	ResetPassword(ctx context.Context, token, newPassword string) error
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
	CleanupExpiredRateLimits(ctx context.Context) (int, error)
}

func (s *Service) passwordResets(op string) (PasswordResets, error) {
	if s.resets == nil {
		return nil, &AuthError{Op: op, Err: ErrPasswordResetUnavailable}
	}
	return s.resets, nil
}

// InitiatePasswordReset creates a password reset token for an account
func (s *Service) InitiatePasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	return s.initiatePasswordReset(ctx, emailAddress, PasswordResetScopeStaff)
}

// InitiateParentPasswordReset creates a password reset token for guardian accounts only.
func (s *Service) InitiateParentPasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	return s.initiatePasswordReset(ctx, emailAddress, PasswordResetScopeParent)
}

// InitiateSchoolPasswordReset creates a reset link for school-portal accounts
// only, so the link remains on the isolated school host.
func (s *Service) InitiateSchoolPasswordReset(ctx context.Context, emailAddress string) (*auth.PasswordResetToken, error) {
	return s.initiatePasswordReset(ctx, emailAddress, PasswordResetScopeSchool)
}

func (s *Service) initiatePasswordReset(ctx context.Context, emailAddress string, scope PasswordResetScope) (*auth.PasswordResetToken, error) {
	resets, err := s.passwordResets("initiate password reset")
	if err != nil {
		return nil, err
	}
	link, err := resets.InitiatePasswordReset(ctx, emailAddress, scope)
	if err != nil || link == nil {
		return nil, err
	}
	token := &auth.PasswordResetToken{
		AccountID: link.AccountID, Token: link.Token, Expiry: link.Expiry,
		EmailSentAt: link.EmailSentAt, EmailError: link.EmailError, EmailRetryCount: link.EmailRetryCount,
	}
	token.ID, token.CreatedAt = link.ID, link.CreatedAt
	return token, nil
}

// ResetPassword resets a password using a reset token
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	resets, err := s.passwordResets("reset password")
	if err != nil {
		return err
	}
	return resets.ResetPassword(ctx, token, newPassword)
}

// CleanupExpiredPasswordResetTokens removes expired and used password reset tokens.
func (s *Service) CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error) {
	resets, err := s.passwordResets("cleanup expired password reset tokens")
	if err != nil {
		return 0, err
	}
	return resets.CleanupExpiredPasswordResetTokens(ctx)
}

// CleanupExpiredRateLimits purges stale password reset rate limit windows.
func (s *Service) CleanupExpiredRateLimits(ctx context.Context) (int, error) {
	resets, err := s.passwordResets("cleanup password reset rate limits")
	if err != nil {
		return 0, err
	}
	return resets.CleanupExpiredRateLimits(ctx)
}

// Password reset rate limit outcomes.
var (
	// ErrRateLimitExceeded returned when password reset attempts exceed rate limit
	ErrRateLimitExceeded = errors.New("too many password reset requests")
)

// RateLimitError provides additional context for rate-limit responses.
type RateLimitError struct {
	Err      error
	Attempts int
	RetryAt  time.Time
}

// Error returns the error message for the rate limit error.
func (e *RateLimitError) Error() string {
	if e.Err == nil {
		return "rate limit exceeded"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *RateLimitError) Unwrap() error {
	return e.Err
}

// RetryAfterSeconds returns the positive number of seconds until retry, or zero if already allowed.
func (e *RateLimitError) RetryAfterSeconds(now time.Time) int {
	if e == nil || e.RetryAt.IsZero() {
		return 0
	}
	if !e.RetryAt.After(now) {
		return 0
	}
	return int(e.RetryAt.Sub(now).Seconds())
}

// PasswordResetOperations run the password reset flows and their maintenance.
type PasswordResetOperations interface {
	// Password Reset
	InitiatePasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	InitiateParentPasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	InitiateSchoolPasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	ResetPassword(ctx context.Context, token, newPassword string) error
	CleanupExpiredRateLimits(ctx context.Context) (int, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
}
