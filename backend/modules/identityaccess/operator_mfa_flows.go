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

// OperatorMFAFlows is the capability the operator login and the operator MFA
// routes consume.
type OperatorMFAFlows interface {
	// HasOperatorMFAEnrollment is (false, nil) for a fresh operator and
	// refuses the login on any other failure.
	HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error)
	// OperatorTrustedDeviceDays is the fixed remember-device lifetime.
	OperatorTrustedDeviceDays() int

	// StartOperatorMFAChallenge mails a code and returns the challenge JWT.
	StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (int64, error)
	// ResendOperatorMFAChallenge returns the renewed JWT so the frontend can
	// replace its in-flight token.
	ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error)
	// VerifyOperatorMFACode is the JWT-less sibling used once the operator
	// is already authenticated out of band.
	VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string) error

	EnrollOperatorMFA(ctx context.Context, operatorID int64) error
	// DisableOperatorMFA removes the enrollment, revokes every trusted
	// device and resets the lockout counter in one transaction.
	DisableOperatorMFA(ctx context.Context, operatorID int64) error

	IssueOperatorTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)
	VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error)
	ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]OperatorTrustedDevice, error)
	// RevokeOperatorTrustedDeviceOwned proves the device belongs to the
	// calling operator before it revokes it.
	RevokeOperatorTrustedDeviceOwned(ctx context.Context, operatorID, deviceID int64) error
}

func (m *Module) HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	return m.engine.HasOperatorMFAEnrollment(ctx, operatorID)
}

func (m *Module) OperatorTrustedDeviceDays() int {
	return m.engine.OperatorTrustedDeviceDays()
}

func (m *Module) StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error) {
	return m.engine.StartOperatorMFAChallenge(ctx, operatorID, ip)
}

// VerifyOperatorMFAChallenge returns the operator the redeemed code belongs
// to, so the login mints its token pair without re-reading the JWT.
func (m *Module) VerifyOperatorMFAChallenge(ctx context.Context, challengeToken, code string) (int64, error) {
	return m.engine.VerifyOperatorMFAChallenge(ctx, challengeToken, code)
}

func (m *Module) ResendOperatorMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	return m.engine.ResendOperatorMFAChallenge(ctx, challengeToken, ip)
}

func (m *Module) VerifyOperatorMFACode(ctx context.Context, operatorID int64, code string) error {
	return m.engine.VerifyOperatorMFACode(ctx, operatorID, code)
}

func (m *Module) EnrollOperatorMFA(ctx context.Context, operatorID int64) error {
	return m.engine.EnrollOperatorMFA(ctx, operatorID)
}

func (m *Module) DisableOperatorMFA(ctx context.Context, operatorID int64) error {
	return m.engine.DisableOperatorMFA(ctx, operatorID)
}

func (m *Module) IssueOperatorTrustedDevice(ctx context.Context, operatorID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	return m.engine.IssueOperatorTrustedDevice(ctx, operatorID, userAgent, ip)
}

func (m *Module) VerifyOperatorTrustedDevice(ctx context.Context, operatorID int64, signedCookie string) (bool, error) {
	return m.engine.VerifyOperatorTrustedDevice(ctx, operatorID, signedCookie)
}

func (m *Module) ListOperatorTrustedDevices(ctx context.Context, operatorID int64) ([]OperatorTrustedDevice, error) {
	return m.engine.ListOperatorTrustedDevices(ctx, operatorID)
}

func (m *Module) RevokeOperatorTrustedDeviceOwned(ctx context.Context, operatorID, deviceID int64) error {
	return m.engine.RevokeOperatorTrustedDeviceOwned(ctx, operatorID, deviceID)
}
