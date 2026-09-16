package platform

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// The operator identity rows (platform.operators) and the operator login
// flows are owned by Identity & Access (#2720, #3252). The retained e-mail
// change, invitation, MFA and passkey flows in this package reach them
// through the consumer-owned ports below; the serving root binds the ports
// to the public module.

// OperatorDirectory reads and writes operator rows for the retained flows.
// A missing row is (nil, nil), validation runs on the retained model before
// the owner sees the value, and the identity and timestamps are written
// back into the caller's value.
type OperatorDirectory interface {
	Create(ctx context.Context, operator *platform.Operator) error
	FindByID(ctx context.Context, id int64) (*platform.Operator, error)
	// FindByIDForUpdate locks the operator row for the caller's transaction
	// so concurrent credential changes serialize.
	FindByIDForUpdate(ctx context.Context, id int64) (*platform.Operator, error)
	FindByEmail(ctx context.Context, email string) (*platform.Operator, error)
	Update(ctx context.Context, operator *platform.Operator) error
	List(ctx context.Context) ([]*platform.Operator, error)
	// IncrementMFAAttempts atomically bumps mfa_attempts and applies the
	// lockout window once threshold is reached, so concurrent failed
	// verifications cannot collapse into a single counted attempt.
	IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (OperatorMFAAttempts, error)
	// ResetMFAAttempts atomically clears mfa_attempts and mfa_locked_until
	// after a successful verification.
	ResetMFAAttempts(ctx context.Context, id int64) error
}

// OperatorMFAAttempts is the counter snapshot after one failed MFA
// verification: the new attempt count and the lockout, if it was applied.
type OperatorMFAAttempts struct {
	Attempts    int
	LockedUntil *time.Time
}

// OperatorSessions is the Identity & Access operator authentication the
// retained operator contract delegates to: login behind the MFA gate, the
// token pair for an operator proven via a non-password channel, refresh,
// and the profile and password changes with their session revocation.
// Errors arrive in the retained shapes (InvalidCredentialsError,
// OperatorNotFoundError, OperatorInactiveError,
// OperatorRefreshTokenInvalidError, PasswordMismatchError,
// InvalidDataError).
type OperatorSessions interface {
	LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*OperatorLoginResult, error)
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (accessToken, refreshToken string, err error)
	UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platform.Operator, error)
	ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error
}
