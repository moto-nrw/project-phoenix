package identityaccess

import (
	"context"
	"errors"
	"net"
	"time"
)

// Operator multi-factor authentication (#3331): the operator's e-mail
// challenge, its verification, the enrollment and its cascade, and the
// remember-device cookies. Operator MFA is mandatory and its cooldown, send
// cap and code lifetime are fixed — the per-school security.* settings apply
// to accounts, which operators are not.
//
// The errors are the account ones: the two surfaces answer with the same
// identities so a caller switches on one set.
var (
	// ErrOperatorMFAUnavailable reports a module composed without the
	// operator MFA dependencies.
	ErrOperatorMFAUnavailable = errors.New("operator mfa is not composed")
)

// The operator code policy. Operators have no per-school settings, so these
// values are the policy, not a fallback.
const (
	OperatorMFAChallengeTTL          = 10 * time.Minute
	OperatorMFARateLimitWindow       = 15 * time.Minute
	OperatorMFARateLimitMaxSent      = 3
	OperatorMFATrustedDeviceDuration = 90 * 24 * time.Hour
)

// OperatorMFAProof is what a redeemed operator code hands the exchange: the
// operator and the credential generation of the password check the code
// stems from (#6308). The session mint and the remembered device hold it to
// the operator's current generation under the operator lock a password write
// takes, so a proof from before a password change earns neither.
type OperatorMFAProof struct {
	OperatorID           int64
	CredentialGeneration int64
}

// OperatorMFAFlows is the capability the operator login and the operator MFA
// routes consume.
type OperatorMFAFlows interface {
	// HasOperatorMFAEnrollment is (false, nil) for a fresh operator and
	// refuses the login on any other failure.
	HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error)
	// OperatorTrustedDeviceDays is the fixed remember-device lifetime.
	OperatorTrustedDeviceDays() int

	// StartOperatorMFAChallenge mails a code and returns the challenge JWT.
	// credentialGeneration names the generation the password check behind
	// the code read (the enrollment token's); nothing is issued once the
	// operator holds another. An in-session start passes none.
	StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP, credentialGeneration *int64) (string, error)
	VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (OperatorMFAProof, error)
	// ResendOperatorMFAChallenge returns the renewed JWT so the frontend can
	// replace its in-flight token.
	ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	// VerifyOperatorMFACode is the JWT-less sibling used once the operator
	// is already authenticated out of band; credentialGeneration as for
	// StartOperatorMFAChallenge.
	VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string, credentialGeneration *int64) error

	EnrollOperatorMFA(ctx context.Context, operatorID int64) error
	// DisableOperatorMFA removes the enrollment, revokes every trusted
	// device and resets the lockout counter in one transaction.
	DisableOperatorMFA(ctx context.Context, operatorID int64) error

	// IssueOperatorTrustedDevice remembers the device only while the
	// operator still holds the proof's credential generation.
	IssueOperatorTrustedDevice(ctx context.Context, proof OperatorMFAProof, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)
	VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error)
	ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]OperatorTrustedDevice, error)
	// RevokeOperatorTrustedDeviceOwned proves the device belongs to the
	// calling operator before it revokes it.
	RevokeOperatorTrustedDeviceOwned(ctx context.Context, operatorID, deviceID int64) error
}
