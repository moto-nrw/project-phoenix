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
// to the public module. The same holds for the operator MFA records (#2723).

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

// OperatorMFARecords reads and writes the operator MFA enrollment, e-mail
// challenge and trusted-device rows, which Identity & Access owns (#2723).
// A missing row is (nil, nil). Activation, consumption and single-device
// revocation return an error when the row was not in the expected state, so
// the loser of a concurrent verification is refused. Every call joins the
// caller's transaction when one is active.
type OperatorMFARecords interface {
	FindCredential(ctx context.Context, operatorID int64) (*platform.OperatorMFACredential, error)
	CreateCredential(ctx context.Context, credential *platform.OperatorMFACredential) error
	TouchCredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteCredentials(ctx context.Context, operatorID int64) error

	CreateChallenge(ctx context.Context, challenge *platform.OperatorMFAEmailChallenge) error
	FindActiveChallenge(ctx context.Context, operatorID int64) (*platform.OperatorMFAEmailChallenge, error)
	CountChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error)
	ActivateChallenge(ctx context.Context, id int64) error
	ConsumeChallenge(ctx context.Context, id int64, consumedAt time.Time) error

	CreateTrustedDevice(ctx context.Context, device *platform.OperatorMFATrustedDevice) error
	FindActiveTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (*platform.OperatorMFATrustedDevice, error)
	ListActiveTrustedDevices(ctx context.Context, operatorID int64) ([]*platform.OperatorMFATrustedDevice, error)
	TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeAllTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error
}

// OperatorSessions is the operator login capability the retained MFA and
// passkey exchanges consume: a token pair for an operator whose identity
// was proven via a non-password channel. Errors arrive in the retained
// shapes (OperatorNotFoundError, OperatorInactiveError).
type OperatorSessions interface {
	IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
}
