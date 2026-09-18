package identityaccess

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Account authentication (#3251): tenant, parent and school login, refresh,
// tenant and school switching, logout, session validation, session cleanup
// and session revocation. The error messages are the wire contract the
// retained auth service established; HTTP layers switch on the sentinels.
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrTenantNotFound     = errors.New("tenant not found")
	// ErrTenantAccessDenied's text is mapped by the tenant frontend; keep it.
	ErrTenantAccessDenied        = errors.New("account does not have access to this tenant")
	ErrParentMustUseParentPortal = errors.New("guardian accounts must log in at the parents portal")
	ErrAccountNoGuardianRole     = errors.New("account is not a guardian at any school")
	ErrAccountNoSchoolPortalRole = errors.New("account has no school portal role at any school")
	ErrMustUseSchoolPortal       = errors.New("school portal accounts must log in at the school portal")
	ErrInvalidToken              = errors.New("invalid token format")
	ErrTokenExpired              = errors.New("token has expired")
	ErrTokenNotFound             = errors.New("token not found")
	ErrMFAStatusUnavailable      = errors.New("mfa status unavailable, please retry")
	// ErrAccountAuthenticationUnavailable reports a module composed without
	// the account-authentication dependencies (repository fixtures, CLI roots
	// that only read).
	ErrAccountAuthenticationUnavailable = errors.New("account authentication is not composed")
)

// AuthenticationError reports which operation of a flow failed. Its text is
// the retained auth service's ("auth error during <op>: <cause>") so the
// responses that render it stay unchanged; Err carries the public sentinel
// for errors.Is.
type AuthenticationError struct {
	Op  string
	Err error
}

func (e *AuthenticationError) Error() string {
	if e.Err == nil {
		return "auth error during " + e.Op
	}
	return "auth error during " + e.Op + ": " + e.Err.Error()
}

func (e *AuthenticationError) Unwrap() error { return e.Err }

// LoginStatus discriminates the shapes a login response can take.
type LoginStatus string

const (
	// LoginStatusAuthenticated: credentials (and MFA, if applicable) passed
	// and the response carries a usable token pair.
	LoginStatusAuthenticated LoginStatus = "authenticated"
	// LoginStatusMFARequired: credentials were valid but the account must
	// present a second factor; the response carries a challenge token.
	LoginStatusMFARequired LoginStatus = "mfa_required"
	// LoginStatusMFAEnrollmentRequired: the tenant requires MFA and the
	// account has no credential yet; the response carries an
	// enrollment-scoped access token.
	LoginStatusMFAEnrollmentRequired LoginStatus = "mfa_enrollment_required"
)

// LoginResult is the discriminated login response. Exactly one of
// (AccessToken+RefreshToken) or ChallengeToken is populated; the
// trusted-device fields are populated on the MFA-required branch only.
type LoginResult struct {
	Status                LoginStatus
	AccessToken           string
	RefreshToken          string
	ChallengeToken        string
	MaskedEmail           string
	MFAEnrollmentRequired bool
	TrustedDeviceEnabled  bool
	TrustedDeviceDays     int
}

// SessionClaims is the content of an access token: what a login mints and
// what session validation returns.
type SessionClaims struct {
	AccountID     int64
	Email         string
	Username      string
	FirstName     string
	LastName      string
	Roles         []string
	Permissions   []string
	IsAdmin       bool
	Scope         string
	TenantID      int64
	OrgID         int64
	FamilyID      string
	ReadOnly      bool
	ActingAdminID int64
	PreviewID     string
	ExpiresAt     int64
	IssuedAt      int64
}

// RefreshClaims is the content of a refresh token.
type RefreshClaims struct {
	AccountID int64
	Token     string
	TenantID  int64
	Scope     string
	ExpiresAt int64
}

// AccountRole is one role an account holds at a school.
type AccountRole struct {
	ID       int64
	Name     string
	IsSystem bool
	TenantID *int64
}

// AccountClaims is the tenant-scoped claims material of an account: the
// roles and permissions at the school, the person names, the admin flag and
// the organization.
type AccountClaims struct {
	RoleNames   []string
	Roles       []AccountRole
	Permissions []string
	Username    string
	FirstName   string
	LastName    string
	IsAdmin     bool
	TenantID    int64
	OrgID       int64
	Scope       string
}

// IsGuardianOnly reports whether every role is the guardian role.
func (c AccountClaims) IsGuardianOnly() bool {
	if len(c.RoleNames) == 0 {
		return false
	}
	for _, name := range c.RoleNames {
		if !strings.EqualFold(name, "guardian") {
			return false
		}
	}
	return true
}

// IsSchoolPortalOnly reports whether every role is a school-portal role
// (the lehrkraft system role, #2207).
func (c AccountClaims) IsSchoolPortalOnly() bool {
	if len(c.Roles) == 0 {
		return false
	}
	for _, role := range c.Roles {
		if !role.IsSystem || !strings.EqualFold(strings.TrimSpace(role.Name), "lehrkraft") {
			return false
		}
	}
	return true
}

// School is the tenant fact login resolves.
type School struct {
	ID             int64
	OrganizationID int64
	Name           string
	Slug           string
	// Subdomain is the host label tenant routing resolves by (#1977); the
	// invitation answers carry it so the client lands on the right host.
	Subdomain string
	Active    bool
	Deleted   bool
	// LogoURL is the school's branding image, as the public invitation
	// pages show it. Empty when the school configured none.
	LogoURL string
}

// MFAPolicy is a resolved MFA verdict waiting for the role set it applies to.
type MFAPolicy interface {
	RequiredFor(roleNames []string) bool
}

// Authentication audit event types, as the Audit platform stores them.
const (
	AuthEventLogin                    = "login"
	AuthEventLogout                   = "logout"
	AuthEventTokenRefresh             = "token_refresh"
	AuthEventTokenRevoked             = "token_revoked"
	AuthEventAccountWideWipeCompleted = "account_wide_wipe_completed"
	AuthEventTenantSwitch             = "tenant_switch"
)

// AuthEvent is one authentication ledger entry with its typed evidence.
type AuthEvent struct {
	AccountID    int64
	TenantID     int64
	Type         string
	Success      bool
	IPAddress    string
	UserAgent    string
	ErrorMessage string
	// TenantAccess is set on the tenant-visible school access events an
	// operator causes (granted, role changed, revoked).
	TenantAccess    *TenantAccessEvidence
	RevokedSessions *RevokedSessionsEvidence
	PendingWipe     *PendingWipeEvidence
	CompletedWipe   *CompletedWipeEvidence
	// MFA is set on the mfa_* events (#3331).
	MFA *MFAEvidence
}

// TenantAccessEvidence describes an operator-led change of the school
// access of an account. RemovedRoles is meaningful for a role change,
// AccountDeactivated for a revocation, Role for a grant or role change.
type TenantAccessEvidence struct {
	SchoolID           int64
	SchoolName         string
	Role               string
	RemovedRoles       []string
	AccountDeactivated bool
	OperatorID         int64
}

// RevokedSessionsEvidence describes one group of revoked sessions.
type RevokedSessionsEvidence struct {
	PortalScope       string
	FamilyFingerprint string
	Reason            string
	Count             int
}

// PendingWipeEvidence marks an account-wide wipe still to be completed
// after commit.
type PendingWipeEvidence struct {
	Reason string
}

// CompletedWipeEvidence closes a pending wipe.
type CompletedWipeEvidence struct {
	PendingEventID int64
}

// PendingAccountWideWipe is a recorded account-wide revoke that may still
// need recovery after a failed after-commit wipe.
type PendingAccountWideWipe struct {
	EventID   int64
	TenantID  int64
	AccountID int64
	Reason    string
	CreatedAt time.Time
}

// AccountAuthentication is the capability tenant, parent and school login,
// MFA exchange, refresh, switching, logout and session validation consume.
type AccountAuthentication interface {
	LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (accessToken, refreshToken string, err error)
	LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*LoginResult, error)
	LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*LoginResult, error)
	LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*LoginResult, error)
	IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (accessToken, newRefreshToken string, err error)
	LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error
	// SwitchTenant retires the presented family with a short grace because
	// the browser replaces that session with the returned pair; an empty
	// presentedFamilyID skips the retirement.
	SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (accessToken, refreshToken string, err error)
	SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error)
	ValidateSessionTokens(ctx context.Context, accessToken, refreshToken, portal string) (SessionClaims, error)
	VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error)
}

// AccountSessionMaintenance is the capability session cleanup and session
// revocation consume: the cleanup CLI, the admin token routes and the
// retained account, role and password flows.
type AccountSessionMaintenance interface {
	CountExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredTokens(ctx context.Context) (int, error)
	ListActiveSessions(ctx context.Context, accountID int64) ([]AccountSession, error)
	ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error)
	RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error
	RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error)
	// DeleteAccountSessionsWithAudit revokes the account's sessions at the
	// caller's school on the caller's transaction, or schedules the
	// account-wide wipe an account-wide reason asks for.
	DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]AccountSession, error)
	// QueuePushCleanup removes the push subscriptions the revoked sessions
	// orphaned, after the caller's transaction commits.
	QueuePushCleanup(ctx context.Context, accountID int64, sessions []AccountSession, reason string)
	ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error
	MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error
}

// AccountClaimsQuery resolves the claims material and portal facts the
// retained staff preview and password reset flows read.
type AccountClaimsQuery interface {
	// LoadAccountClaims runs on the caller's transaction.
	LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (AccountClaims, error)
	FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error)
	FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error)
}

func (m *Module) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	return m.engine.LoginWithAudit(ctx, email, password, ipAddress, userAgent, tenantSlug)
}

func (m *Module) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*LoginResult, error) {
	return m.engine.LoginWithMFAGate(ctx, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie)
}

func (m *Module) LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error) {
	return m.engine.LoginParentWithAudit(ctx, email, password, ipAddress, userAgent)
}

func (m *Module) LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*LoginResult, error) {
	return m.engine.LoginSchoolWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
}

func (m *Module) LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*LoginResult, error) {
	return m.engine.LoginSchoolAtTenantWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug)
}

func (m *Module) IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	return m.engine.IssueTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
}

func (m *Module) IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error) {
	return m.engine.IssueSchoolTokensForAuthenticatedAccount(ctx, accountID, tenantID, ipAddress, userAgent)
}

func (m *Module) RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (string, string, error) {
	return m.engine.RefreshTokenWithAudit(ctx, refreshToken, ipAddress, userAgent)
}

func (m *Module) LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error {
	return m.engine.LogoutWithAudit(ctx, refreshToken, ipAddress, userAgent)
}

func (m *Module) SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (string, string, error) {
	return m.engine.SwitchTenant(ctx, accountID, tenantSlug, presentedFamilyID)
}

func (m *Module) SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (string, string, error) {
	return m.engine.SwitchSchool(ctx, accountID, tenantSlug, ipAddress, userAgent)
}

func (m *Module) HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return m.engine.HasSchoolPortalAccess(ctx, accountID, tenantID)
}

func (m *Module) ValidateSessionTokens(ctx context.Context, accessToken, refreshToken, portal string) (SessionClaims, error) {
	return m.engine.ValidateSessionTokens(ctx, accessToken, refreshToken, portal)
}

func (m *Module) VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return m.engine.VerifyAccountTenantMembership(ctx, accountID, tenantID)
}

func (m *Module) CountExpiredTokens(ctx context.Context) (int, error) {
	return m.engine.CountExpiredTokens(ctx)
}

func (m *Module) CleanupExpiredTokens(ctx context.Context) (int, error) {
	return m.engine.CleanupExpiredTokens(ctx)
}

func (m *Module) ListActiveSessions(ctx context.Context, accountID int64) ([]AccountSession, error) {
	return m.engine.ListActiveSessions(ctx, accountID)
}

func (m *Module) ListSessionIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return m.engine.ListSessionIDs(ctx, accountID)
}

func (m *Module) RevokeAllTokensWithReason(ctx context.Context, accountID int64, reason string) error {
	return m.engine.RevokeAllTokensWithReason(ctx, accountID, reason)
}

func (m *Module) RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error) {
	return m.engine.RevokeTokensByTenantID(ctx, tenantID)
}

func (m *Module) DeleteAccountSessionsWithAudit(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) ([]AccountSession, error) {
	return m.engine.DeleteAccountSessionsWithAudit(ctx, accountID, reason, ipAddress, userAgent)
}

func (m *Module) QueuePushCleanup(ctx context.Context, accountID int64, sessions []AccountSession, reason string) {
	m.engine.QueuePushCleanup(ctx, accountID, sessions, reason)
}

func (m *Module) ScheduleAccountWideRevoke(ctx context.Context, accountID int64, reason, ipAddress, userAgent string) error {
	return m.engine.ScheduleAccountWideRevoke(ctx, accountID, reason, ipAddress, userAgent)
}

func (m *Module) MarkAccountWideWipeCompleted(ctx context.Context, accountID int64) error {
	return m.engine.MarkAccountWideWipeCompleted(ctx, accountID)
}

func (m *Module) LoadAccountClaims(ctx context.Context, accountID, tenantID int64) (AccountClaims, error) {
	return m.engine.LoadAccountClaims(ctx, accountID, tenantID)
}

func (m *Module) FindGuardianTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	return m.engine.FindGuardianTenant(ctx, accountID)
}

func (m *Module) FindSchoolPortalTenant(ctx context.Context, accountID int64) (bool, int64, error) {
	return m.engine.FindSchoolPortalTenant(ctx, accountID)
}
