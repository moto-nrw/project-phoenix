package identityaccess

import (
	"context"
	"errors"
	"time"
)

// Password reset (#2722): a one-time link that sets a new password for a
// staff, parent or school account. The link is never stored already expired
// or spent, is redeemable once, and a new password revokes the account's
// sessions in the same transaction. The request answers identically whether
// or not the address belongs to an eligible account.
var (
	// ErrPasswordResetUnavailable reports a module composed without the
	// password reset dependencies.
	ErrPasswordResetUnavailable = errors.New("password reset is not composed")
	// ErrPasswordResetRateLimited reports a request beyond the per-address
	// limit; the error is a *PasswordResetRateLimitError.
	ErrPasswordResetRateLimited = errors.New("too many password reset requests")
)

// PasswordResetScope names the portal a reset link opens.
type PasswordResetScope string

const (
	PasswordResetScopeStaff  PasswordResetScope = "staff"
	PasswordResetScopeParent PasswordResetScope = "parent"
	PasswordResetScopeSchool PasswordResetScope = "school"
)

// PasswordResetRateLimitError carries the window of a rejected request; the
// Retry-After header the frontend countdown reads derives from RetryAt.
type PasswordResetRateLimitError struct {
	Attempts int
	RetryAt  time.Time
}

func (e *PasswordResetRateLimitError) Error() string { return ErrPasswordResetRateLimited.Error() }

func (e *PasswordResetRateLimitError) Unwrap() error { return ErrPasswordResetRateLimited }

// PasswordResetLink is an issued reset link.
type PasswordResetLink struct {
	ID        int64
	AccountID int64
	Token     string
	Expiry    time.Time
	Delivery  TokenDelivery
	CreatedAt time.Time
}

// PasswordResets is the password reset capability the public reset routes
// and the maintenance jobs consume. Flow errors arrive in the
// AuthenticationError envelope.
type PasswordResets interface {
	// InitiatePasswordReset returns nil without an error when no link was
	// issued (unknown address or no role of the scope's portal).
	InitiatePasswordReset(ctx context.Context, email string, scope PasswordResetScope) (*PasswordResetLink, error)
	ResetPassword(ctx context.Context, token, newPassword string) error
	RecordPasswordResetDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
	DeleteSpentPasswordResetTokens(ctx context.Context) (int, error)
	DeleteStalePasswordResetWindows(ctx context.Context) (int, error)
}

func (m *Module) InitiatePasswordReset(ctx context.Context, email string, scope PasswordResetScope) (*PasswordResetLink, error) {
	return m.engine.InitiatePasswordReset(ctx, email, scope)
}

func (m *Module) ResetPassword(ctx context.Context, token, newPassword string) error {
	return m.engine.ResetPassword(ctx, token, newPassword)
}

func (m *Module) RecordPasswordResetDelivery(ctx context.Context, id int64, delivery TokenDelivery) error {
	return m.engine.RecordPasswordResetDelivery(ctx, id, delivery)
}

func (m *Module) DeleteSpentPasswordResetTokens(ctx context.Context) (int, error) {
	return m.engine.DeleteSpentPasswordResetTokens(ctx)
}

func (m *Module) DeleteStalePasswordResetWindows(ctx context.Context) (int, error) {
	return m.engine.DeleteStalePasswordResetWindows(ctx)
}
