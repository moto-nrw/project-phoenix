package auth

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StaffPINAuthenticator verifies a staff-specific PIN inside the staff
// member's tenant boundary. Device middleware uses this narrow interface to
// bind kiosk attribution to a person without depending on the full auth API.
type StaffPINAuthenticator interface {
	AuthenticateStaffPIN(ctx context.Context, tenantID, staffID int64, pin string) (*userModels.Staff, error)
}

// AuthService defines the operations for authentication and user management
type AuthService interface {
	// Existing methods
	Login(ctx context.Context, email, password string) (accessToken, refreshToken string, err error)
	LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (accessToken, refreshToken string, err error)
	// IssueTokensForAuthenticatedAccount mints an access/refresh token pair for
	// an account that has already proven its identity via a non-password channel
	// (currently: MFA email-code or recovery-code verification). Skips credential
	// checks but otherwise mirrors LoginWithAudit's loadMetadata → persistRefreshToken
	// → buildClaims → genTokens flow so downstream consumers (refresh, audit,
	// permissions) are indistinguishable from a regular login.
	IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// LoginWithMFAGate is the MFA-aware sibling of LoginWithAudit. After a
	// successful credential check it consults the optional MFAService and
	// returns either a regular token pair or a short-lived challenge token
	// the caller must redeem at /auth/mfa/verify. trustedDeviceCookie may
	// be empty; when set and verifiable, MFA is skipped even if the account
	// would normally require it.
	LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug, trustedDeviceCookie string) (*LoginResult, error)
	// SetMFAService wires the optional MFA gate. Pass nil to disable the
	// gate (login then behaves exactly as LoginWithAudit).
	SetMFAService(svc MFAService)
	LoginParent(ctx context.Context, email, password string) (accessToken, refreshToken string, err error)
	LoginParentWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	// LoginSchoolWithMFAGate authenticates a school-portal user (#2207) and
	// issues a school-scope token pair bound to the first school where the
	// account holds a school-portal role (today: lehrkraft). MFA-aware like
	// LoginWithMFAGate; school challenges carry the school challenge scope
	// and are redeemable only at the school verify endpoint.
	LoginSchoolWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*LoginResult, error)
	// LoginSchoolAtTenantWithMFAGate is the selected-school variant of
	// LoginSchoolWithMFAGate. It verifies that the account has a school-portal
	// role at tenantSlug before issuing a school-scope result.
	LoginSchoolAtTenantWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie, tenantSlug string) (*LoginResult, error)
	// IssueSchoolTokensForAuthenticatedAccount is the school-scope sibling of
	// IssueTokensForAuthenticatedAccount, used by the school MFA verify
	// endpoint. Re-validates the school-portal role at the tenant.
	IssueSchoolTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*auth.Account, error)
	// RegisterSchoolAccount is Register plus the school identity (person →
	// staff → caregiver profile) the role requires, in one transaction (#2222).
	RegisterSchoolAccount(ctx context.Context, email, username, password string, roleID *int64, tenantID int64, identity *SchoolAccountIdentity) (*auth.Account, *SchoolIdentity, error)
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (accessToken, newRefreshToken string, err error)
	LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error
	ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error
	GetAccountByID(ctx context.Context, id int) (*auth.Account, error)
	// VerifyAccountTenantMembership reports whether the account has a tenant
	// mapping for the given school (issue #584 lookup; repository result
	// returned verbatim).
	VerifyAccountTenantMembership(ctx context.Context, accountID, tenantID int64) (bool, error)

	// Role Management
	CreateRole(ctx context.Context, name, description string, baseRole *string) (*auth.Role, error)
	GetRoleByID(ctx context.Context, id int) (*auth.Role, error)
	ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*auth.Role, error)
	UpdateRole(ctx context.Context, role *auth.Role) error
	DeleteRole(ctx context.Context, id int) error
	ListRoles(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error)
	AssignRoleToAccount(ctx context.Context, accountID, roleID int) error
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error
	GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error)
	GetAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	GetAccountEmailsByIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	GetAccountAvatarsByIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)

	// Permission Management
	CreatePermission(ctx context.Context, name, description, resource, action string) (*auth.Permission, error)
	GetPermissionByID(ctx context.Context, id int) (*auth.Permission, error)
	GetPermissionByName(ctx context.Context, name string) (*auth.Permission, error)
	UpdatePermission(ctx context.Context, permission *auth.Permission) error
	DeletePermission(ctx context.Context, id int) error
	ListPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Permission, error)
	GrantPermissionToAccount(ctx context.Context, accountID, permissionID int) error
	DenyPermissionToAccount(ctx context.Context, accountID, permissionID int) error
	RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int) error
	GetAccountPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error)
	GetAccountDirectPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error)
	AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error
	GetRolePermissions(ctx context.Context, roleID int) ([]*auth.Permission, error)

	// Account Management Extensions
	ActivateAccount(ctx context.Context, accountID int) error
	DeactivateAccount(ctx context.Context, accountID int) error
	UpdateAccount(ctx context.Context, account *auth.Account) error

	// PIN lockout (issue #586): the brute-force lockout decision and the
	// atomic counter mutations live in the service, not on the model.
	IsPINLocked(account *auth.Account, now time.Time) bool
	RecordFailedPINAttempt(ctx context.Context, accountID int64) error
	ResetPINLockout(ctx context.Context, accountID int64) error
	ListAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error)
	GetAccountsByRole(ctx context.Context, roleName string) ([]*auth.Account, error)
	GetAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error)

	// Password Reset
	InitiatePasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	InitiateParentPasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	InitiateSchoolPasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	ResetPassword(ctx context.Context, token, newPassword string) error
	CleanupExpiredRateLimits(ctx context.Context) (int, error)

	// Token Management
	CleanupExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
	RevokeAllTokens(ctx context.Context, accountID int) error
	RevokeAllTokensWithReason(ctx context.Context, accountID int, reason string) error
	RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error)
	GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error)

	// Tenant Switching
	// presentedFamilyID is the refresh-token family behind the caller's access
	// token; it is retired with a short grace period because the browser
	// replaces that session with the returned one. Empty skips retirement.
	SwitchTenant(ctx context.Context, accountID int64, tenantSlug, presentedFamilyID string) (accessToken, refreshToken string, err error)
	// HasSchoolPortalAccess reports whether the account still holds a
	// school-portal role at this school. For surfaces that authenticate once
	// and then stay open for the token's whole lifetime — the school SSE
	// stream re-checks with it while streaming (#2208).
	HasSchoolPortalAccess(ctx context.Context, accountID, tenantID int64) (bool, error)
	// SwitchSchool is the school-portal sibling of SwitchTenant (#2207):
	// re-authenticates a school-scope session to another school where the
	// account holds a school-portal role. ipAddress/userAgent are required for
	// the tenant_switch audit event — the audit write is skipped when the IP
	// is empty.
	SwitchSchool(ctx context.Context, accountID int64, tenantSlug, ipAddress, userAgent string) (accessToken, refreshToken string, err error)

	// Admin staff-view preview (#2893): a read-only, access-only token that
	// sees the tenant portal exactly as the target staff member. Start mints
	// (and re-mints) the token, End records the audit trail, the candidate
	// list feeds the picker. The route layer restricts all three to
	// effective admins. End is given the preview token it closes and reads
	// the previewed account from it, so the audit trail cannot be stamped
	// with a preview that never happened. Start takes the token the client
	// currently holds so a re-mint continues the running preview instead of
	// opening a second one in the audit trail.
	StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*StaffPreviewSession, error)
	EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error)
	ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]StaffPreviewCandidate, error)

	// Multi-Tenant Account Linking
	LinkAccountToTenant(ctx context.Context, email string, roleID *int64, tenantID int64) (*auth.Account, error)
	// LinkSchoolAccount is LinkAccountToTenant plus the school identity the
	// role requires, in one transaction (#2222).
	LinkSchoolAccount(ctx context.Context, email string, roleID *int64, tenantID int64, identity *SchoolAccountIdentity) (*auth.Account, *SchoolIdentity, error)

	// Parent Account Management
	CreateParentAccount(ctx context.Context, email, username, password string) (*auth.AccountParent, error)
	GetParentAccountByID(ctx context.Context, id int) (*auth.AccountParent, error)
	GetParentAccountByEmail(ctx context.Context, email string) (*auth.AccountParent, error)
	UpdateParentAccount(ctx context.Context, account *auth.AccountParent) error
	ActivateParentAccount(ctx context.Context, accountID int) error
	DeactivateParentAccount(ctx context.Context, accountID int) error
	ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error)
}

// Note: The NewService function is implemented in auth_service.go
