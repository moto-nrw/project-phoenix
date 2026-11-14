package identityaccess

import (
	"context"
	"errors"
	"net"
	"time"
)

// Account multi-factor authentication (#3331): the gate that decides whether
// a login needs a second factor, the e-mail challenge and its verification,
// the enrollment, the remember-device cookies and the admin overrides that
// force MFA on or off for one account.
//
// Nothing here fails open. An override, mode or enrollment the module cannot
// read refuses that one login with ErrMFAStatusUnavailable rather than
// dropping the second factor for everyone.
var (
	// ErrAccountMFAUnavailable reports a module composed without the MFA
	// dependencies.
	ErrAccountMFAUnavailable = errors.New("account mfa is not composed")
	// ErrMFAChallengeTokenInvalid refuses a challenge token that is
	// unreadable, expired or not this surface's.
	ErrMFAChallengeTokenInvalid = errors.New("invalid or expired challenge token")
	// ErrMFACodeInvalid refuses a code that does not match, has expired or
	// was already used.
	ErrMFACodeInvalid = errors.New("invalid or expired code")
	// ErrMFALocked reports an account inside its failure cooldown.
	ErrMFALocked = errors.New("account locked due to too many failed attempts")
	// ErrMFARateLimited reports a send beyond the per-account window.
	ErrMFARateLimited = errors.New("too many code requests, please wait")
	// ErrMFANotEnrolled reports an account without an enrolled second
	// factor.
	ErrMFANotEnrolled = errors.New("mfa not enrolled for this account")
	// ErrMFAAlreadyEnrolled refuses a second enrollment.
	ErrMFAAlreadyEnrolled = errors.New("mfa already enrolled for this account")
	// ErrMFAPermissionDenied refuses an admin action the actor may not take,
	// and a device or account it may not reach.
	ErrMFAPermissionDenied = errors.New("permission denied")
	// ErrOperatorAuditUnavailable refuses an unaudited privileged operation.
	ErrOperatorAuditUnavailable = errors.New("operator audit unavailable")
	// ErrMFAInvalidOverride refuses an override value outside the allow
	// list.
	ErrMFAInvalidOverride = errors.New("invalid mfa override value")
	// ErrMFAUnsupportedScope refuses a challenge of another portal.
	ErrMFAUnsupportedScope = errors.New("operator-scope MFA is wired up in a separate phase")
)

// The portal a challenge belongs to. A code is only redeemable at the
// surface it was started for.
const (
	MFAChallengeScopeTenant   = "tenant"
	MFAChallengeScopePlatform = "platform"
	MFAChallengeScopeSchool   = "school"
	// MFAChallengeScopeOrg is the Träger-Portal (#4651).
	MFAChallengeScopeOrg = "org"
)

// The fixed code policy, as the flows apply it (internal/domain/mfa.go):
// a code lives MFAChallengeTTL, and MFALockoutThreshold failed codes lock an
// operator's second factor. A school's lockout follows its
// security.account_lockout_* settings instead.
const (
	MFAChallengeTTL     = 10 * time.Minute
	MFALockoutThreshold = 5
)

// The admin-override values. There is no "none" row: its absence is the
// none outcome.
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

// MFAAdminState is the read the admin "Manage MFA" modal shows. Override is
// "none" when the school set none.
type MFAAdminState struct {
	Enrolled bool
	Override string
}

// VerifiedMFAChallenge is what a redeemed challenge hands the caller, so the
// session mint needs no second look at the challenge JWT.
type VerifiedMFAChallenge struct {
	MFAResetGeneration int64
	AccountID          int64
	Scope              string
	TenantID           int64
}

// AccountMFAGate is the read side every login path consults.
type AccountMFAGate interface {
	// IsRequired applies the resolved policy to the account's effective permissions.
	IsRequired(ctx context.Context, accountID int64, permissions []string, tenantID int64) (bool, error)
	// ResolveMFAPolicy performs the reads and leaves the permission predicate
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
}

// AccountMFAChallenges is the e-mail challenge flow.
type AccountMFAChallenges interface {
	// StartMFAChallenge mails a code for one portal and returns the
	// challenge JWT naming its row. Every caller follows a password check:
	// credentialGeneration is the credential generation that check read (a
	// sign-in, or the enrollment token it handed out), and no code goes out
	// once a password write or MFA reset moved it on (ErrInvalidCredentials,
	// #6289).
	StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP, credentialGeneration int64) (string, error)
	// VerifyMFAChallenge verifies a tenant-portal challenge.
	VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (VerifiedMFAChallenge, error)
	// VerifyMFAChallengeForScope refuses a challenge of another portal
	// before any code is compared.
	VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (VerifiedMFAChallenge, error)
	// VerifyMFAChallengeForOwner additionally pins the challenge to an
	// account and school the caller authenticated by other means, and
	// refuses a foreign challenge before it is consumed.
	VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (VerifiedMFAChallenge, error)
	// VerifyMFACodeForAccount is the JWT-less sibling for callers that
	// authenticated the user out of band. The scope and school are required:
	// without a challenge id they are what keeps the lookup inside its own
	// portal.
	VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) (VerifiedMFAChallenge, error)
	// ResendMFAChallengeForScope re-issues a code for a challenge of this
	// portal and returns the renewed JWT.
	ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error)
}

// AccountMFAEnrollment is the enrollment and its cascade.
type AccountMFAEnrollment interface {
	// EnrollMFA records the e-mail factor from the code the enrollment confirm
	// redeemed. It writes nothing and answers ErrInvalidCredentials when the
	// account's credential generation moved past the proof's since (#6518).
	EnrollMFA(ctx context.Context, proof VerifiedMFAChallenge) error
	// DisableMFA removes the enrollment, revokes every trusted device and
	// resets the lockout counter in one transaction.
	DisableMFA(ctx context.Context, accountID int64) error
	// ResetMFAInTransaction runs the same cascade on the caller's
	// administrative transaction and never opens one itself (ADR 0067); the
	// operator's reset of a Träger-Büro member participates this way.
	ResetMFAInTransaction(ctx context.Context, accountID int64) error
}

// TrustedDeviceIssue persists a device using the policy resolved at preparation.
// Call it only after session issuance succeeds.
type TrustedDeviceIssue = func(context.Context) (cookieValue string, expiresAt time.Time, err error)

// AccountTrustedDevices is the remember-device cookie.
type AccountTrustedDevices interface {
	// PrepareTrustedDevice resolves policy without writes. A nil operation
	// means the feature is disabled. Either policy read can return an error;
	// login callers then skip remembering the device, not the verified login.
	// The operation does not re-read settings.
	PrepareTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (TrustedDeviceIssue, error)
	// IssueTrustedDevice answers an empty cookie value without persisting a
	// row when the school turned the feature off; the caller then writes no
	// Set-Cookie header.
	IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (cookieValue string, expiresAt time.Time, err error)
	VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error)
	ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]AccountTrustedDevice, error)
	RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error
}

// AccountMFAAdministration is the admin side: a school admin managing a
// user's second factor, and the operator doing the same across schools.
type AccountMFAAdministration interface {
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

// AccountMFA is the capability the login paths, the MFA routes and the admin
// surfaces consume.
type AccountMFA interface {
	AccountMFAGate
	AccountMFAChallenges
	AccountMFAEnrollment
	AccountTrustedDevices
	AccountMFAAdministration
}

// traegerDeviceKey carries the Träger whose remembered devices a
// trusted-device call without a school means.
type traegerDeviceKey struct{}

// WithTraegerDevice binds the Träger of the Träger-Portal to the
// trusted-device calls of ctx (#6955). A Träger session names its Träger and
// no school, so its device does too: AccountTrustedDevices called with
// tenantID zero then prepares, issues, verifies, lists and revokes only the
// devices of that Träger. Without the binding a call without a school means
// the caterer portal's devices (#4497); a call that names a school ignores
// it. A non-positive organizationID binds nothing.
func WithTraegerDevice(ctx context.Context, organizationID int64) context.Context {
	if organizationID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, traegerDeviceKey{}, organizationID)
}

// TraegerDeviceOf is the Träger WithTraegerDevice bound, or zero.
func TraegerDeviceOf(ctx context.Context) int64 {
	organizationID, _ := ctx.Value(traegerDeviceKey{}).(int64)
	return organizationID
}
