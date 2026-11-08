package ports

import (
	"context"
	"net"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The rules the flows consult: credentials, tokens, the second factor and the rotation bounds.

// PasswordVerifier checks a password against the stored hash.
type PasswordVerifier interface {
	VerifyPassword(password, hash string) (bool, error)
}

// TokenCodec signs and parses the session JWTs.
type TokenCodec interface {
	IssueTokenPair(access domain.SessionClaims, refresh domain.RefreshClaims) (accessToken, refreshToken string, err error)
	IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error)
	// ParseAccessToken verifies the signature and expiry of an access JWT.
	ParseAccessToken(token string) (domain.SessionClaims, error)
	// ParseRefreshToken verifies the signature of a refresh JWT and refuses
	// challenge, enrollment and preview tokens.
	ParseRefreshToken(token string) (domain.RefreshClaims, error)
	RefreshExpiry() time.Duration
}

// MFAPolicy is a resolved MFA verdict waiting for the permission set it applies to.
type MFAPolicy = identityaccess.MFAPolicy

// MFAGate is the consumer-owned port over the retained MFA service. An
// unconfigured gate (Configured false) means "not required / not enrolled".
type MFAGate interface {
	Configured() bool
	IsRequired(ctx context.Context, accountID int64, email string, permissions []string, tenantID int64) (bool, error)
	ResolvePolicy(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error)
	// ResolvePolicyInTx re-reads the policy on the caller's transaction, past
	// every request-scoped cache.
	ResolvePolicyInTx(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error)
	HasEnrollment(ctx context.Context, accountID int64) (bool, error)
	VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, cookie string) (bool, error)
	StartChallenge(ctx context.Context, accountID, tenantID int64, scope, ipAddress string) (string, error)
	IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool
	TrustedDeviceDays(ctx context.Context, tenantID int64) int
}

// MFAPolicyLock pins a school's MFA mode for the rest of the caller's
// transaction, in shared mode.
type MFAPolicyLock interface {
	LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error
}

// SchoolRolePolicy is the consumer-owned port over the retained role
// assignment rules every school-access path shares (#1772, #2222): which
// roles may be handed out at a school, which role is the lehrkraft system
// role, which roles need a staff record, and the message a lehrkraft
// account's role change is refused with.
type SchoolRolePolicy interface {
	ValidateAssignableSchoolRole(role domain.RoleFact, tenantID int64) error
	IsLehrkraftRole(role domain.RoleFact) bool
	RoleNeedsStaffRecord(role domain.RoleFact) bool
	LehrkraftRoleImmutable() error
}

// OperatorMFAGate is the consumer-owned port over the retained operator MFA
// service. Operator MFA is mandatory: an unconfigured gate (Configured
// false) issues token pairs directly, as the early phases did.
type OperatorMFAGate interface {
	Configured() bool
	HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error)
	VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error)
	StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	OperatorTrustedDeviceDays() int
}

// PasswordHasher hashes a new password and applies the strength policy.
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	ValidatePasswordStrength(password string) error
}

// Rotation is the refresh-rotation recovery policy shared with the operator
// portal.
type Rotation interface {
	RecoveryGrace() time.Duration
	MaxRecoveryHops() int
	FamilyFingerprint(familyID string) string
}
