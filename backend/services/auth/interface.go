package auth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// AuthenticationOperations handles core authentication flows
type AuthenticationOperations interface {
	Login(ctx context.Context, email, password string) (accessToken, refreshToken string, err error)
	LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
	Register(ctx context.Context, email, username, password string, roleID *int64) (*auth.Account, error)
	ValidateToken(ctx context.Context, token string) (*auth.Account, error)
	RefreshToken(ctx context.Context, refreshToken string) (accessToken, newRefreshToken string, err error)
	RefreshTokenWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) (accessToken, newRefreshToken string, err error)
	Logout(ctx context.Context, refreshToken string) error
	LogoutWithAudit(ctx context.Context, refreshToken, ipAddress, userAgent string) error
	ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error
}

// AccountLookup handles account retrieval operations
type AccountLookup interface {
	GetAccountByID(ctx context.Context, id int) (*auth.Account, error)
	GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error)
}

// AccountManagement handles account lifecycle operations
type AccountManagement interface {
	ActivateAccount(ctx context.Context, accountID int) error
	DeactivateAccount(ctx context.Context, accountID int) error
	UpdateAccount(ctx context.Context, account *auth.Account) error
	ListAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error)
	GetAccountsByRole(ctx context.Context, roleName string) ([]*auth.Account, error)
	GetAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error)
}

// RoleManagement handles role CRUD and assignment operations
type RoleManagement interface {
	CreateRole(ctx context.Context, name, description string) (*auth.Role, error)
	GetRoleByID(ctx context.Context, id int) (*auth.Role, error)
	GetRoleByName(ctx context.Context, name string) (*auth.Role, error)
	UpdateRole(ctx context.Context, role *auth.Role) error
	DeleteRole(ctx context.Context, id int) error
	ListRoles(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error)
	AssignRoleToAccount(ctx context.Context, accountID, roleID int) error
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error
	GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error)
}

// PermissionManagement handles permission CRUD and assignment operations
type PermissionManagement interface {
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
}

// PasswordResetOperations handles password reset flows
type PasswordResetOperations interface {
	InitiatePasswordReset(ctx context.Context, email string) (*auth.PasswordResetToken, error)
	ResetPassword(ctx context.Context, token, newPassword string) error
	CleanupExpiredRateLimits(ctx context.Context) (int, error)
}

// TokenManagement handles token lifecycle operations
type TokenManagement interface {
	CleanupExpiredTokens(ctx context.Context) (int, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) (int, error)
	RevokeAllTokens(ctx context.Context, accountID int) error
	GetActiveTokens(ctx context.Context, accountID int) ([]*auth.Token, error)
}

// ParentAccountManagement handles parent-specific account operations
type ParentAccountManagement interface {
	CreateParentAccount(ctx context.Context, email, username, password string) (*auth.AccountParent, error)
	GetParentAccountByID(ctx context.Context, id int) (*auth.AccountParent, error)
	GetParentAccountByEmail(ctx context.Context, email string) (*auth.AccountParent, error)
	UpdateParentAccount(ctx context.Context, account *auth.AccountParent) error
	ActivateParentAccount(ctx context.Context, accountID int) error
	DeactivateParentAccount(ctx context.Context, accountID int) error
	ListParentAccounts(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error)
}

// AuthService composes all authentication-related operations.
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type AuthService interface {
	base.TransactionalService
	AuthenticationOperations
	AccountLookup
	AccountManagement
	RoleManagement
	PermissionManagement
	PasswordResetOperations
	TokenManagement
	ParentAccountManagement
}

// Note: The NewService function is implemented in auth_service.go
