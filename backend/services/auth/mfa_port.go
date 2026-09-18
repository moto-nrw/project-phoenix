package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
)

// The account second factor and the school-portal passkey ceremonies moved
// into modules/identityaccess with #3331. This file is what the HTTP surfaces
// that may not name the module yet consume: the capability they call, the
// values they render and the error identities they classify on. The
// composition root binds the module behind these ports and translates its
// errors into the sentinels below, so every status code, error string and
// wire shape is the one those handlers already produced. The ports disappear
// with the handler relocation (#3230, #3231).

// The code policy the request bodies and the challenge screens depend on.
const (
	MFAEmailCodeLength = 6
	// MFAChallengeTTL, MFALockoutThreshold and MFALockoutDuration are the
	// defaults the gate applies where a school set no security.* value.
	MFAChallengeTTL     = 10 * time.Minute
	MFALockoutThreshold = 5
	MFALockoutDuration  = 15 * time.Minute
	// MFAChallengeScopeTenant and MFAChallengeScopeSchool name the portal a
	// challenge belongs to. A code of one portal is never redeemable on the
	// other.
	MFAChallengeScopeTenant = "tenant"
	MFAChallengeScopeSchool = "school"
)

// The admin override values. "none" means the school's security.mfa_mode
// decides; the two force values overrule it for one account.
const (
	MFAAdminOverrideNone     = "none"
	MFAAdminOverrideForceOff = "force_off"
	MFAAdminOverrideForceOn  = "force_on"
)

// IsValidMFAAdminOverride is the allow list the admin surfaces check before
// they hand a value to the capability, so the database CHECK constraint is
// never the first line of defence.
func IsValidMFAAdminOverride(value string) bool {
	switch value {
	case MFAAdminOverrideNone, MFAAdminOverrideForceOff, MFAAdminOverrideForceOn:
		return true
	}
	return false
}

// The error identities the MFA and passkey routes classify on. The
// composition maps the module's errors onto exactly these values, so
// errors.Is keeps deciding the same status codes it did before #3331.
var (
	ErrMFAChallengeTokenInvalid = errors.New("invalid or expired challenge token")
	ErrMFACodeInvalid           = errors.New("invalid or expired code")
	ErrMFALocked                = errors.New("account locked due to too many failed attempts")
	ErrMFARateLimited           = errors.New("too many code requests, please wait")
	ErrMFANotEnrolled           = errors.New("mfa not enrolled for this account")
	ErrMFAAlreadyEnrolled       = errors.New("mfa already enrolled for this account")
	ErrMFAPermissionDenied      = errors.New("permission denied")
	ErrMFAInvalidOverride       = errors.New("invalid mfa override value")
	ErrMFAUnsupportedScope      = errors.New("operator-scope MFA is wired up in a separate phase")
	// ErrMFAStatusUnavailable is the fail-closed answer: the gate could not
	// determine whether this login needs a second factor, so this login is
	// refused with a 503 instead of silently degrading to "not required".
	ErrMFAStatusUnavailable = errors.New("mfa status unavailable, please retry")

	ErrPasskeyOriginInvalid  = errors.New("passkey origin is invalid")
	ErrPasskeySessionInvalid = errors.New("passkey session is invalid")
	ErrPasskeyNotFound       = errors.New("passkey not found")
)

// MFAPolicy is a resolved second-factor verdict with the role predicate left
// unapplied.
type MFAPolicy interface {
	RequiredFor(roleNames []string) bool
}

// VerifiedMFAChallenge is what a redeemed challenge hands the caller, so the
// session mint needs no second look at the challenge JWT.
type VerifiedMFAChallenge struct {
	AccountID int64
	Scope     string
	TenantID  int64
}

// MFAAdminState is the read the admin "Manage MFA" modal shows. Override is
// "none" when the school set none.
type MFAAdminState struct {
	Enrolled bool
	Override string
}

// AccountTrustedDevice is one remember-device row as the trusted-device list
// renders it.
type AccountTrustedDevice struct {
	ID         int64
	AccountID  int64
	TenantID   int64
	TokenHash  string
	UserAgent  *string
	IPAddress  net.IP
	ExpiresAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PasskeyEnrollmentChallenge is what a registration start hands the portal.
// The tags are the wire contract both portals render it under.
type PasskeyEnrollmentChallenge struct {
	ChallengeToken string `json:"challenge_token"`
	MaskedEmail    string `json:"masked_email"`
}

// PasskeyCeremonyOptions names one started ceremony and carries the options
// the browser passes to the authenticator.
type PasskeyCeremonyOptions struct {
	SessionID string `json:"session_id"`
	Options   any    `json:"options"`
}

// PasskeyCredentialSummary is one registered credential as the portals list
// it.
type PasskeyCredentialSummary struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// PasskeyLoginResult is the token pair a completed login ceremony minted.
type PasskeyLoginResult struct {
	AccessToken  string
	RefreshToken string
}

// AccountPasskeyRegistrationStart starts a school-portal registration.
type AccountPasskeyRegistrationStart struct {
	AccountID       int64
	TenantID        int64
	TenantSubdomain string
	ExpectedOrigin  string
	Code            string
	Name            string
}

// AccountPasskeyRegistrationFinish completes a school-portal registration.
type AccountPasskeyRegistrationFinish struct {
	AccountID          int64
	SessionID          string
	CredentialResponse json.RawMessage
	Name               string
}

// AccountPasskeyLoginStart starts a school-portal discoverable login.
type AccountPasskeyLoginStart struct {
	TenantID        int64
	TenantSubdomain string
	ExpectedOrigin  string
}

// AccountPasskeyLoginFinish completes a school-portal login.
type AccountPasskeyLoginFinish struct {
	SessionID          string
	CredentialResponse json.RawMessage
	IPAddress          string
	UserAgent          string
}

// MFAService is the account second factor the login paths, the MFA routes
// and the admin surfaces consume.
type MFAService interface {
	// IsRequired applies the resolved policy to the account's roles.
	IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error)
	// ResolveMFAPolicy performs the reads and leaves the role predicate
	// unapplied.
	ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error)
	// ResolveMFAPolicyInTx re-reads the policy on the caller's transaction,
	// past every request-scoped cache, so a mode an admin changed while the
	// login was in flight still counts.
	ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error)
	HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error)
	// AccountBelongsToSchool reports the membership the MFA admin surfaces
	// gate on.
	AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error)
	IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool
	TrustedDeviceDays(ctx context.Context, tenantID int64) int

	// StartMFAChallenge mails a code for one portal and returns the
	// challenge JWT naming its row.
	StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
	VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (VerifiedMFAChallenge, error)
	// VerifyMFAChallengeForScope refuses a challenge of another portal
	// before any code is compared.
	VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (VerifiedMFAChallenge, error)
	// VerifyMFAChallengeForOwner additionally pins the challenge to an
	// account and school the caller authenticated by other means.
	VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (VerifiedMFAChallenge, error)
	// VerifyMFACodeForAccount is the JWT-less sibling for callers that
	// authenticated the user out of band.
	VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error
	// ResendMFAChallengeForScope re-issues a code for a challenge of this
	// portal and returns the renewed JWT.
	ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error)
	// ResendMFAChallenge accepts a challenge of any scope and is therefore
	// not for an HTTP surface.
	ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error)

	EnrollMFA(ctx context.Context, accountID int64) error
	// DisableMFA removes the enrollment, revokes every trusted device and
	// resets the lockout counter in one transaction.
	DisableMFA(ctx context.Context, accountID int64) error

	// IssueTrustedDevice answers an empty cookie value without persisting a
	// row when the school turned the feature off; the caller then writes no
	// Set-Cookie header.
	IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)
	VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error)
	ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]AccountTrustedDevice, error)
	RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error

	// AdminDisableMFA requires users:manage and a target in the actor's own
	// school.
	AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error
	// SetMFAOverride writes a school-scoped override; "none" deletes that
	// school's row and never the platform-wide one.
	SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error
	GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error)
	GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (MFAAdminState, error)
	// OperatorDisableMFA is the operator variant of AdminDisableMFA. The
	// route layer carries the platform gate; the school membership is still
	// proven here.
	OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error
	OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error
	// OperatorSetGlobalMFAOverride writes or clears the account-wide
	// emergency switch. A force_off here revokes trust at every school.
	OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error
	GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error)
}

// PasskeyService is the school-portal ceremony capability. A credential
// belongs to the account and carries no school; the login verifies the
// account's membership in the school whose portal started the ceremony
// before it mints a session.
type PasskeyService interface {
	StartAccountPasskeyEnrollment(ctx context.Context, accountID, tenantID int64, ip net.IP) (PasskeyEnrollmentChallenge, error)
	BeginAccountPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationStart) (PasskeyCeremonyOptions, error)
	FinishAccountPasskeyRegistration(ctx context.Context, request AccountPasskeyRegistrationFinish) (PasskeyCredentialSummary, error)
	BeginAccountPasskeyLogin(ctx context.Context, request AccountPasskeyLoginStart) (PasskeyCeremonyOptions, error)
	FinishAccountPasskeyLogin(ctx context.Context, request AccountPasskeyLoginFinish) (PasskeyLoginResult, error)
	ListAccountPasskeyCredentials(ctx context.Context, accountID int64) ([]PasskeyCredentialSummary, error)
	RevokeAccountPasskeyCredential(ctx context.Context, accountID, credentialID int64) error
}
