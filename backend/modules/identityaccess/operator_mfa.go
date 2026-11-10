package identityaccess

import (
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
	ErrOperatorMFACredentialNotFound = notFoundError("operator mfa credential not found")
	// ErrOperatorMFAChallengeNotFound reports an operator without an
	// unconsumed, unexpired e-mail code.
	ErrOperatorMFAChallengeNotFound = notFoundError("operator mfa challenge not found")
	// ErrOperatorMFAChallengeStateChanged reports an activation or
	// consumption that found the code no longer in the expected state. The
	// loser of two concurrent verifications receives it.
	ErrOperatorMFAChallengeStateChanged = conflictError("operator mfa challenge was already consumed or activated")
	// ErrOperatorTrustedDeviceNotFound reports a lookup without an active
	// trusted device, or a revocation of a device already revoked or gone.
	ErrOperatorTrustedDeviceNotFound = notFoundError("operator trusted device not found")
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
	// CredentialGeneration is the operator's credential generation the code
	// was issued under; it redeems only while that is current (#6308).
	CredentialGeneration int64
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
	// CredentialGeneration is the operator's credential generation the
	// device was remembered under; it verifies only while that is current
	// (#6308).
	CredentialGeneration int64
}
