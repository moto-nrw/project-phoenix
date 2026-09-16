package domain

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// Account authentication errors. The messages are the wire and log contract
// the retained auth service established; the HTTP layers switch on the
// sentinels and the frontend maps ErrTenantAccessDenied's text.
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTenantAccessDenied = errors.New("account does not have access to this tenant")
	// ErrParentMustUseParentPortal refuses a guardian-only account at the
	// tenant login; ErrAccountNoGuardianRole refuses an account without a
	// guardian role at the parents login.
	ErrParentMustUseParentPortal = errors.New("guardian accounts must log in at the parents portal")
	ErrAccountNoGuardianRole     = errors.New("account is not a guardian at any school")
	// ErrAccountNoSchoolPortalRole refuses an account without a school-portal
	// role at the school login; ErrMustUseSchoolPortal refuses a
	// school-portal-only account at the tenant login (#2207).
	ErrAccountNoSchoolPortalRole = errors.New("account has no school portal role at any school")
	ErrMustUseSchoolPortal       = errors.New("school portal accounts must log in at the school portal")
	ErrInvalidToken              = errors.New("invalid token format")
	ErrTokenExpired              = errors.New("token has expired")
	ErrTokenNotFound             = errors.New("token not found")
	// ErrMFAStatusUnavailable refuses a login whose MFA gate could not be
	// decided: the second factor is never dropped on an infrastructure error.
	ErrMFAStatusUnavailable = errors.New("mfa status unavailable, please retry")
	// ErrAccountAuthenticationUnavailable reports a module composed without
	// the account-authentication dependencies.
	ErrAccountAuthenticationUnavailable = errors.New("account authentication is not composed")
)

// Portal scopes as they travel in JWT claims. The tenant scope is the empty
// string on the wire.
const (
	ScopeTenant   = ""
	ScopeOrg      = "org"
	ScopePlatform = "platform"
	ScopeParent   = "parent"
	ScopeSchool   = "school"
)

// LehrkraftRoleName is the school-portal system role (#2207).
const LehrkraftRoleName = "lehrkraft"

// AdminRoleName is the role whose holders carry the is_admin claim.
const AdminRoleName = "admin"

// MFAEnrollmentTokenTTL is the lifetime of an enrollment-scoped JWT: long
// enough to fetch the emailed code and confirm enrollment in one sitting,
// short enough that an abandoned enrollment session does not linger.
const MFAEnrollmentTokenTTL = 15 * time.Minute

// MaxActiveSessionsPerPortal bounds the live sessions one account keeps in
// one portal group; other portals keep their own sessions.
const MaxActiveSessionsPerPortal = 5

// RefreshTokenIdentifier labels every refresh session a service login mints.
const RefreshTokenIdentifier = "Service login"

// LoginAccount is the platform account a login authenticates. PasswordHash is
// empty for accounts without a password.
type LoginAccount struct {
	ID           int64
	Email        string
	Username     string
	PasswordHash string
	Active       bool
}

// RoleAssignment is one role an account holds at a school, with the facts the
// login decisions read.
type RoleAssignment struct {
	RoleID   int64
	Name     string
	IsSystem bool
	TenantID *int64
}

// School is the tenant fact login resolves: alive, active and its organization.
type School struct {
	ID             int64
	OrganizationID int64
	Active         bool
	Deleted        bool
}

// Live reports whether the school can be logged into on any portal.
func (s School) Live() bool { return !s.Deleted && s.Active }

// SessionClaims is the payload an access token is built from and the shape the
// session validation returns.
type SessionClaims struct {
	AccountID   int64
	Email       string
	Username    string
	FirstName   string
	LastName    string
	Roles       []string
	Permissions []string
	IsAdmin     bool
	Scope       string
	TenantID    int64
	OrgID       int64
	FamilyID    string
	// ReadOnly, ActingAdminID and PreviewID mark an admin staff-view preview
	// token (#2893); session validation refuses to pair one with a refresh.
	ReadOnly      bool
	ActingAdminID int64
	PreviewID     string
	ExpiresAt     int64
	IssuedAt      int64
}

// RefreshClaims is the parsed content of a refresh JWT.
type RefreshClaims struct {
	AccountID int64
	Token     string
	TenantID  int64
	Scope     string
	ExpiresAt int64
}

// LoginStatus discriminates the shapes a login response can take.
type LoginStatus string

const (
	LoginStatusAuthenticated         LoginStatus = "authenticated"
	LoginStatusMFARequired           LoginStatus = "mfa_required"
	LoginStatusMFAEnrollmentRequired LoginStatus = "mfa_enrollment_required"
)

// LoginResult is the discriminated login response. Exactly one of
// (AccessToken+RefreshToken) or ChallengeToken is populated.
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

// AccountClaimsPayload is the tenant-scoped claims material: roles,
// permissions, person names, admin flag and organization.
type AccountClaimsPayload struct {
	RoleNames   []string
	Roles       []RoleAssignment
	Permissions []string
	Username    string
	FirstName   string
	LastName    string
	IsAdmin     bool
	TenantID    int64
	OrgID       int64
	Scope       string
}

// PersistedPortalScope maps a JWT scope to the portal scope a refresh
// session is stored under.
func PersistedPortalScope(jwtScope string) string {
	switch jwtScope {
	case ScopeTenant, PortalScopeTenant:
		return PortalScopeTenant
	case PortalScopeOrg, PortalScopeParent, PortalScopeSchool:
		return jwtScope
	default:
		return PortalScopeUnknown
	}
}

// SessionScopeMatches reports whether an access token's scope belongs to the
// portal a session validation names.
func SessionScopeMatches(scope, portal string) bool {
	switch portal {
	case "tenant":
		return scope == ScopeTenant || scope == "tenant" || scope == ScopeOrg
	case "platform", "parent", "school":
		return scope == portal
	default:
		return false
	}
}

// IsGuardianOnly reports whether every role name is the guardian role. An
// empty set is not guardian-only.
func IsGuardianOnly(roleNames []string) bool {
	if len(roleNames) == 0 {
		return false
	}
	for _, name := range roleNames {
		if !strings.EqualFold(name, GuardianRoleName) {
			return false
		}
	}
	return true
}

// IsSchoolPortalRole reports whether the role grants access to the school
// portal (#2207): today exactly the lehrkraft system role. A tenant-scoped
// custom role that merely shares the label carries arbitrary permissions and
// keeps its tenant-portal access.
func IsSchoolPortalRole(role RoleAssignment) bool {
	return role.IsSystem && strings.EqualFold(strings.TrimSpace(role.Name), LehrkraftRoleName)
}

// IsSchoolPortalOnly reports whether every role the account holds is a
// school-portal role. An empty role set is not school-portal-only.
func IsSchoolPortalOnly(roles []RoleAssignment) bool {
	if len(roles) == 0 {
		return false
	}
	for _, role := range roles {
		if !IsSchoolPortalRole(role) {
			return false
		}
	}
	return true
}

// HasSchoolPortalRole reports whether any role grants school-portal access.
func HasSchoolPortalRole(roles []RoleAssignment) bool {
	return slices.ContainsFunc(roles, IsSchoolPortalRole)
}

// HasGuardianRole reports whether any role is the guardian role.
func HasGuardianRole(roles []RoleAssignment) bool {
	return slices.ContainsFunc(roles, func(role RoleAssignment) bool {
		return strings.EqualFold(role.Name, GuardianRoleName)
	})
}

// IsAdmin reports whether the admin role is among the names.
func IsAdmin(roleNames []string) bool {
	return slices.Contains(roleNames, AdminRoleName)
}

// RoleNames projects assignments to their names.
func RoleNames(roles []RoleAssignment) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

// MaskEmail renders an address as `j***@example.com` so the frontend can
// show which mailbox received a code without leaking the full address.
func MaskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 1 {
		return local + "***" + domain
	}
	return string(local[0]) + "***" + domain
}

// IsAccountWideRevocation reports whether a revocation reason wipes every
// session of the account at every school.
func IsAccountWideRevocation(reason string) bool {
	switch reason {
	case "password_reset", "account_deactivated", "administrative_revoke":
		return true
	default:
		return false
	}
}

// SessionFamilyIDs returns the distinct non-empty family IDs of the sessions.
func SessionFamilyIDs(sessions []AccountSession) []string {
	seen := make(map[string]struct{}, len(sessions))
	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if session.FamilyID == "" {
			continue
		}
		if _, ok := seen[session.FamilyID]; ok {
			continue
		}
		seen[session.FamilyID] = struct{}{}
		ids = append(ids, session.FamilyID)
	}
	return ids
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

// InternalRevocationAuditIP marks revocation evidence no client request
// produced.
const InternalRevocationAuditIP = "0.0.0.0"

// AuthEvent is one authentication ledger entry with its typed evidence.
type AuthEvent struct {
	AccountID    int64
	TenantID     int64
	Type         string
	Success      bool
	IPAddress    string
	UserAgent    string
	ErrorMessage string
	// RevokedSessions is set on token_revoked events.
	RevokedSessions *RevokedSessionsEvidence
	// PendingWipe marks a token_revoked event that records an account-wide
	// wipe still to be completed after commit.
	PendingWipe *PendingWipeEvidence
	// CompletedWipe is set on account_wide_wipe_completed events.
	CompletedWipe *CompletedWipeEvidence
}

type RevokedSessionsEvidence struct {
	PortalScope       string
	FamilyFingerprint string
	Reason            string
	Count             int
}

type PendingWipeEvidence struct {
	Reason string
}

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
