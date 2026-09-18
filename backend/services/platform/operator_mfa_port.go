package platform

import (
	"context"
	"encoding/json"
	"net"
	"time"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// The operator second factor and the operator passkey ceremonies moved into
// modules/identityaccess with #3331. The operator routes reach them through
// the ports below; the composition root binds the module behind them and
// translates its errors into the account sentinels the operator handlers
// already classified on (the two surfaces answer with one error set). The
// ports disappear with the handler relocation (#3230, #3231).

// The operator code policy. Operators have no per-school settings, so these
// values are the policy, not a fallback.
const (
	OperatorMFAChallengeTTL          = 10 * time.Minute
	OperatorMFARateLimitWindow       = 15 * time.Minute
	OperatorMFARateLimitMaxSent      = 3
	OperatorMFATrustedDeviceDuration = 90 * 24 * time.Hour
)

// OperatorTrustedDevice is one operator remember-device row as the
// trusted-device list renders it.
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

// OperatorPasskeyRegistrationStart starts an operator registration.
type OperatorPasskeyRegistrationStart struct {
	OperatorID     int64
	ExpectedOrigin string
	Code           string
	Name           string
}

// OperatorPasskeyRegistrationFinish completes an operator registration.
type OperatorPasskeyRegistrationFinish struct {
	OperatorID         int64
	SessionID          string
	CredentialResponse json.RawMessage
	Name               string
}

// OperatorPasskeyLoginFinish completes an operator login.
type OperatorPasskeyLoginFinish struct {
	SessionID          string
	CredentialResponse json.RawMessage
	IPAddress          string
	UserAgent          string
}

// OperatorMFAService is the operator second factor the operator login and
// the operator MFA routes consume.
type OperatorMFAService interface {
	// HasOperatorMFAEnrollment is (false, nil) for a fresh operator and
	// refuses the login on any other failure.
	HasOperatorMFAEnrollment(ctx context.Context, operatorID int64) (bool, error)
	// OperatorTrustedDeviceDays is the fixed remember-device lifetime.
	OperatorTrustedDeviceDays() int

	// StartOperatorMFAChallenge mails a code and returns the challenge JWT.
	StartOperatorMFAChallenge(ctx context.Context, operatorID int64, ip net.IP) (string, error)
	// VerifyOperatorMFAChallenge returns the operator the redeemed code
	// belongs to, so the login mints its token pair without re-reading the
	// JWT.
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

// OperatorPasskeyService is the operator-portal ceremony capability.
type OperatorPasskeyService interface {
	StartOperatorPasskeyEnrollment(ctx context.Context, operatorID int64, ip net.IP) (authService.PasskeyEnrollmentChallenge, error)
	BeginOperatorPasskeyRegistration(ctx context.Context, request OperatorPasskeyRegistrationStart) (authService.PasskeyCeremonyOptions, error)
	FinishOperatorPasskeyRegistration(ctx context.Context, request OperatorPasskeyRegistrationFinish) (authService.PasskeyCredentialSummary, error)
	BeginOperatorPasskeyLogin(ctx context.Context, expectedOrigin string) (authService.PasskeyCeremonyOptions, error)
	FinishOperatorPasskeyLogin(ctx context.Context, request OperatorPasskeyLoginFinish) (authService.PasskeyLoginResult, error)
	ListOperatorPasskeyCredentials(ctx context.Context, operatorID int64) ([]authService.PasskeyCredentialSummary, error)
	RevokeOperatorPasskeyCredential(ctx context.Context, operatorID, credentialID int64) error
}
