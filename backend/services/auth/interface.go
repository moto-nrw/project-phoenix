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
	Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*auth.Account, error)
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
	ResetPassword(ctx context.Context, token, newPassword string) error
	CleanupExpiredRateLimits(ctx context.Context) (int, error)

	// Token Management
	CleanupExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
	RevokeAllTokens(ctx context.Context, accountID int) error
	RevokeTokensByTenantID(ctx context.Context, tenantID int64) (int, error)
	GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error)

	// Tenant Switching
	SwitchTenant(ctx context.Context, accountID int64, tenantSlug string) (accessToken, refreshToken string, err error)

	// Multi-Tenant Account Linking
	LinkAccountToTenant(ctx context.Context, email string, roleID *int64, tenantID int64) (*auth.Account, error)

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
