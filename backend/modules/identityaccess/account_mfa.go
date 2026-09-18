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
)

// MFAEmailCodeLength is the number of decimal digits in an e-mail code. The
// HTTP surfaces validate the body against it before a code reaches the flow.
const MFAEmailCodeLength = 6

// MFALockoutThreshold and MFALockoutDuration are the failure policy the MFA
// gate applies when a school set no security.account_lockout_* override. The
// PIN lockout shares them, so the 5-attempt / 15-minute rule has one source
// of truth across the platform (#586).
const (
	MFALockoutThreshold = 5
	MFALockoutDuration  = 15 * time.Minute
)

// The code policy the flows apply. The window and the cap are an abuse
// defense, not a UX knob, and stay fixed; the remember-device lifetime is
// the fallback the school's security.mfa_trusted_device_days overrides.
const (
	MFAChallengeTTL                   = 10 * time.Minute
	MFAEmailRateLimitWindow           = 15 * time.Minute
	MFAEmailRateLimitMaxSent          = 3
	MFATrustedDeviceCookieDefaultDays = 90
)

// The modes a school's security.mfa_mode setting takes, and the role whose
// holders required_admins gates on.
const (
	MFAModeOff            = "off"
	MFAModeRequiredAll    = "required_all"
	MFAModeRequiredAdmins = "required_admins"
	// AdminRoleName is the role required_admins looks for.
	AdminRoleName = "admin"
)

// The admin-override values. There is no "none" row: its absence is the
// none outcome.
const (
	MFAAdminOverrideNone     = "none"
	MFAAdminOverrideForceOff = "force_off"
	MFAAdminOverrideForceOn  = "force_on"
)

// MFAAdminState is the read the admin "Manage MFA" modal shows. Override is
// "none" when the school set none.
type MFAAdminState struct {
	Enrolled bool
	Override string
}

// VerifiedMFAChallenge is what a redeemed challenge hands the caller, so the
// session mint needs no second look at the challenge JWT.
type VerifiedMFAChallenge struct {
	AccountID int64
	Scope     string
	TenantID  int64
}

// AccountMFAGate is the read side every login path consults.
type AccountMFAGate interface {
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
}

// AccountMFAChallenges is the e-mail challenge flow.
type AccountMFAChallenges interface {
	// StartMFAChallenge mails a code for one portal and returns the
	// challenge JWT naming its row.
	StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error)
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
	VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error
	// ResendMFAChallengeForScope re-issues a code for a challenge of this
	// portal and returns the renewed JWT.
	ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error)
	// ResendMFAChallenge accepts a challenge of any scope and is therefore
	// not for an HTTP surface.
	ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error)
}

// AccountMFAEnrollment is the enrollment and its cascade.
type AccountMFAEnrollment interface {
	EnrollMFA(ctx context.Context, accountID int64) error
	// DisableMFA removes the enrollment, revokes every trusted device and
	// resets the lockout counter in one transaction.
	DisableMFA(ctx context.Context, accountID int64) error
}

// AccountTrustedDevices is the remember-device cookie.
type AccountTrustedDevices interface {
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

func (m *Module) IsRequired(ctx context.Context, accountID int64, roleNames []string, tenantID int64) (bool, error) {
	return m.engine.IsRequired(ctx, accountID, roleNames, tenantID)
}

func (m *Module) ResolveMFAPolicy(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error) {
	return m.engine.ResolveMFAPolicy(ctx, accountID, tenantID)
}

func (m *Module) ResolveMFAPolicyInTx(ctx context.Context, accountID, tenantID int64) (MFAPolicy, error) {
	return m.engine.ResolveMFAPolicyInTx(ctx, accountID, tenantID)
}

func (m *Module) HasMFAEnrollment(ctx context.Context, accountID int64) (bool, error) {
	return m.engine.HasMFAEnrollment(ctx, accountID)
}

func (m *Module) AccountBelongsToSchool(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return m.engine.AccountBelongsToSchool(ctx, accountID, tenantID)
}

func (m *Module) IsTrustedDeviceEnabled(ctx context.Context, tenantID int64) bool {
	return m.engine.IsTrustedDeviceEnabled(ctx, tenantID)
}

func (m *Module) TrustedDeviceDays(ctx context.Context, tenantID int64) int {
	return m.engine.TrustedDeviceDays(ctx, tenantID)
}

func (m *Module) StartMFAChallenge(ctx context.Context, accountID, tenantID int64, scope string, ip net.IP) (string, error) {
	return m.engine.StartMFAChallenge(ctx, accountID, tenantID, scope, ip)
}

func (m *Module) VerifyMFAChallenge(ctx context.Context, challengeToken, code string) (VerifiedMFAChallenge, error) {
	return m.engine.VerifyMFAChallenge(ctx, challengeToken, code)
}

func (m *Module) VerifyMFAChallengeForScope(ctx context.Context, challengeToken, code, expectedScope string) (VerifiedMFAChallenge, error) {
	return m.engine.VerifyMFAChallengeForScope(ctx, challengeToken, code, expectedScope)
}

func (m *Module) VerifyMFAChallengeForOwner(ctx context.Context, challengeToken, code, expectedScope string, accountID, tenantID int64) (VerifiedMFAChallenge, error) {
	return m.engine.VerifyMFAChallengeForOwner(ctx, challengeToken, code, expectedScope, accountID, tenantID)
}

func (m *Module) VerifyMFACodeForAccount(ctx context.Context, accountID, tenantID int64, code, expectedScope string) error {
	return m.engine.VerifyMFACodeForAccount(ctx, accountID, tenantID, code, expectedScope)
}

func (m *Module) ResendMFAChallengeForScope(ctx context.Context, challengeToken string, ip net.IP, expectedScope string) (string, error) {
	return m.engine.ResendMFAChallengeForScope(ctx, challengeToken, ip, expectedScope)
}

func (m *Module) ResendMFAChallenge(ctx context.Context, challengeToken string, ip net.IP) (string, error) {
	return m.engine.ResendMFAChallenge(ctx, challengeToken, ip)
}

func (m *Module) EnrollMFA(ctx context.Context, accountID int64) error {
	return m.engine.EnrollMFA(ctx, accountID)
}

func (m *Module) DisableMFA(ctx context.Context, accountID int64) error {
	return m.engine.DisableMFA(ctx, accountID)
}

func (m *Module) IssueTrustedDevice(ctx context.Context, accountID, tenantID int64, userAgent string, ip net.IP) (string, time.Time, error) {
	return m.engine.IssueTrustedDevice(ctx, accountID, tenantID, userAgent, ip)
}

func (m *Module) VerifyTrustedDevice(ctx context.Context, accountID, tenantID int64, signedCookie string) (bool, error) {
	return m.engine.VerifyTrustedDevice(ctx, accountID, tenantID, signedCookie)
}

func (m *Module) ListTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]AccountTrustedDevice, error) {
	return m.engine.ListTrustedDevices(ctx, accountID, tenantID)
}

func (m *Module) RevokeTrustedDevice(ctx context.Context, accountID, tenantID, deviceID int64) error {
	return m.engine.RevokeTrustedDevice(ctx, accountID, tenantID, deviceID)
}

func (m *Module) AdminDisableMFA(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	return m.engine.AdminDisableMFA(ctx, actorID, actorTenantID, targetAccountID, reason, actorPermissions)
}

func (m *Module) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	return m.engine.SetMFAOverride(ctx, actorID, actorTenantID, targetAccountID, override, reason, actorPermissions)
}

func (m *Module) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	return m.engine.GetTenantMFAOverride(ctx, accountID, tenantID)
}

func (m *Module) GetMFAAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (MFAAdminState, error) {
	return m.engine.GetMFAAdminState(ctx, actorID, actorTenantID, targetAccountID, actorPermissions)
}

func (m *Module) OperatorDisableMFA(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	return m.engine.OperatorDisableMFA(ctx, operatorID, schoolID, targetAccountID, reason)
}

func (m *Module) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	return m.engine.OperatorSetMFAOverride(ctx, operatorID, schoolID, targetAccountID, override, reason)
}

func (m *Module) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	return m.engine.OperatorSetGlobalMFAOverride(ctx, operatorID, targetAccountID, override, reason)
}

func (m *Module) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	return m.engine.GetGlobalMFAOverride(ctx, accountID)
}
