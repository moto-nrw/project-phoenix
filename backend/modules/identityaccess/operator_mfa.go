package identityaccess

import (
	"context"
	"errors"
	"net"
	"time"
)

// Operator MFA records (#2723): the operator's MFA enrollment, the hashed
// e-mail codes of its login challenges and the server-side records of its
// remember-device cookies. The rows are platform-wide like the operator
// itself. Every operation joins the caller's transaction when one is active,
// so a multi-table write such as disabling MFA commits as one unit of work.
var (
	// ErrOperatorMFACredentialNotFound reports an operator without MFA
	// enrollment.
	ErrOperatorMFACredentialNotFound = errors.New("operator mfa credential not found")
	// ErrOperatorMFAChallengeNotFound reports an operator without an
	// unconsumed, unexpired e-mail code.
	ErrOperatorMFAChallengeNotFound = errors.New("operator mfa challenge not found")
	// ErrOperatorMFAChallengeStateChanged reports an activation or
	// consumption that found the code no longer in the expected state. The
	// loser of two concurrent verifications receives it.
	ErrOperatorMFAChallengeStateChanged = errors.New("operator mfa challenge was already consumed or activated")
	// ErrOperatorTrustedDeviceNotFound reports a lookup without an active
	// trusted device, or a revocation of a device already revoked or gone.
	ErrOperatorTrustedDeviceNotFound = errors.New("operator trusted device not found")
)

// OperatorMFACredential records that an operator enrolled in e-mail MFA.
type OperatorMFACredential struct {
	ID         int64
	OperatorID int64
	Method     string
	EnrolledAt time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorMFAChallenge is one hashed e-mail code. ConsumedAt is set while
// the code is not redeemable: before its delivery was confirmed and after
// it was used.
type OperatorMFAChallenge struct {
	ID         int64
	OperatorID int64
	CodeHash   string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	IPAddress  net.IP
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorTrustedDevice is the server-side record of a remember-device
// cookie; TokenHash is the hash of the cookie's raw token.
type OperatorTrustedDevice struct {
	ID         int64
	OperatorID int64
	TokenHash  string
	UserAgent  *string
	IPAddress  net.IP
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OperatorMFAQuery resolves operator MFA records. "Active" means unconsumed
// (codes) or unrevoked (devices) and not yet expired.
type OperatorMFAQuery interface {
	FindOperatorMFACredential(ctx context.Context, operatorID int64) (OperatorMFACredential, error)
	// FindActiveOperatorMFAChallenge returns the active code with the latest
	// expiry.
	FindActiveOperatorMFAChallenge(ctx context.Context, operatorID int64) (OperatorMFAChallenge, error)
	// CountOperatorMFAChallengesSince counts every code created at or after
	// since, redeemed or not; the send rate limit reads it.
	CountOperatorMFAChallengesSince(ctx context.Context, operatorID int64, since time.Time) (int, error)
	FindActiveOperatorTrustedDevice(ctx context.Context, operatorID int64, tokenHash string) (OperatorTrustedDevice, error)
	// ListActiveOperatorTrustedDevices orders by last use, most recent first,
	// then by creation.
	ListActiveOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]OperatorTrustedDevice, error)
}

// OperatorMFACommand changes operator MFA records. The creates validate the
// record and return it with its identity and timestamps.
type OperatorMFACommand interface {
	CreateOperatorMFACredential(ctx context.Context, credential OperatorMFACredential) (OperatorMFACredential, error)
	TouchOperatorMFACredential(ctx context.Context, id int64, usedAt time.Time) error
	DeleteOperatorMFACredentials(ctx context.Context, operatorID int64) error
	CreateOperatorMFAChallenge(ctx context.Context, challenge OperatorMFAChallenge) (OperatorMFAChallenge, error)
	// ActivateOperatorMFAChallenge makes a delivered code redeemable.
	ActivateOperatorMFAChallenge(ctx context.Context, id int64) error
	// ConsumeOperatorMFAChallenge redeems a code exactly once.
	ConsumeOperatorMFAChallenge(ctx context.Context, id int64, consumedAt time.Time) error
	CreateOperatorTrustedDevice(ctx context.Context, device OperatorTrustedDevice) (OperatorTrustedDevice, error)
	TouchOperatorTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error
	RevokeOperatorTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error
	RevokeOperatorTrustedDevices(ctx context.Context, operatorID int64, revokedAt time.Time) error
}

// OperatorMFARecords is the capability the operator MFA flows consume.
type OperatorMFARecords interface {
	OperatorMFAQuery
	OperatorMFACommand
}
