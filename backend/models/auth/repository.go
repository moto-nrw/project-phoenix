package auth

import (
	"context"
	"time"
)

// AccountRepository defines operations for managing accounts
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	FindByID(ctx context.Context, id interface{}) (*Account, error)
	FindByEmail(ctx context.Context, email string) (*Account, error)
	FindByUsername(ctx context.Context, username string) (*Account, error)
	Update(ctx context.Context, account *Account) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Account, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	FindByRole(ctx context.Context, role string) ([]*Account, error)
	FindAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*Account, error)
}

// RoleRepository defines operations for managing roles
type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	FindByID(ctx context.Context, id interface{}) (*Role, error)
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Role, error)
	FindByName(ctx context.Context, name string) (*Role, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Role, error)
	AssignRoleToAccount(ctx context.Context, accountID int64, roleID int64) error
	RemoveRoleFromAccount(ctx context.Context, accountID int64, roleID int64) error
	GetRoleWithPermissions(ctx context.Context, roleID int64) (*Role, error)
}

// PermissionRepository defines operations for managing permissions
type PermissionRepository interface {
	Create(ctx context.Context, permission *Permission) error
	FindByID(ctx context.Context, id interface{}) (*Permission, error)
	Update(ctx context.Context, permission *Permission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Permission, error)
	FindByName(ctx context.Context, name string) (*Permission, error)
	FindByResourceAction(ctx context.Context, resource, action string) (*Permission, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Permission, error)
	FindDirectByAccountID(ctx context.Context, accountID int64) ([]*Permission, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*Permission, error)
	FindByRoleByName(ctx context.Context, roleName string) (*Role, error)
	AssignPermissionToAccount(ctx context.Context, accountID int64, permissionID int64) error
	RemovePermissionFromAccount(ctx context.Context, accountID int64, permissionID int64) error
	AssignPermissionToRole(ctx context.Context, roleID int64, permissionID int64) error
	RemovePermissionFromRole(ctx context.Context, roleID int64, permissionID int64) error
}

// AccountParentRepository defines operations for managing parent accounts
type AccountParentRepository interface {
	Create(ctx context.Context, account *AccountParent) error
	FindByID(ctx context.Context, id interface{}) (*AccountParent, error)
	FindByEmail(ctx context.Context, email string) (*AccountParent, error)
	FindByUsername(ctx context.Context, username string) (*AccountParent, error)
	Update(ctx context.Context, account *AccountParent) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*AccountParent, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
}

// RolePermissionRepository defines operations for managing role-permission mappings
type RolePermissionRepository interface {
	Create(ctx context.Context, rolePermission *RolePermission) error
	FindByID(ctx context.Context, id interface{}) (*RolePermission, error)
	Update(ctx context.Context, rolePermission *RolePermission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*RolePermission, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*RolePermission, error)
	FindByPermissionID(ctx context.Context, permissionID int64) ([]*RolePermission, error)
	FindByRoleAndPermission(ctx context.Context, roleID, permissionID int64) (*RolePermission, error)
	DeleteByRoleAndPermission(ctx context.Context, roleID, permissionID int64) error
	DeleteByRoleID(ctx context.Context, roleID int64) error
	FindRolePermissionsWithDetails(ctx context.Context, filters map[string]interface{}) ([]*RolePermission, error)
}

// AccountRoleRepository defines operations for managing account-role mappings
type AccountRoleRepository interface {
	Create(ctx context.Context, accountRole *AccountRole) error
	FindByID(ctx context.Context, id interface{}) (*AccountRole, error)
	Update(ctx context.Context, accountRole *AccountRole) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*AccountRole, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*AccountRole, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*AccountRole, error)
	FindByAccountAndRole(ctx context.Context, accountID, roleID int64) (*AccountRole, error)
	DeleteByAccountAndRole(ctx context.Context, accountID, roleID int64) error
	DeleteByAccountID(ctx context.Context, accountID int64) error
	FindAccountRolesWithDetails(ctx context.Context, filters map[string]interface{}) ([]*AccountRole, error)
}

// AccountPermissionRepository defines operations for managing account-permission mappings
type AccountPermissionRepository interface {
	Create(ctx context.Context, accountPermission *AccountPermission) error
	FindByID(ctx context.Context, id interface{}) (*AccountPermission, error)
	Update(ctx context.Context, accountPermission *AccountPermission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*AccountPermission, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*AccountPermission, error)
	FindByPermissionID(ctx context.Context, permissionID int64) ([]*AccountPermission, error)
	FindByAccountAndPermission(ctx context.Context, accountID, permissionID int64) (*AccountPermission, error)
	GrantPermission(ctx context.Context, accountID, permissionID int64) error
	DenyPermission(ctx context.Context, accountID, permissionID int64) error
	RemovePermission(ctx context.Context, accountID, permissionID int64) error
	FindAccountPermissionsWithDetails(ctx context.Context, filters map[string]interface{}) ([]*AccountPermission, error)
}

// TokenRepository defines operations for managing authentication tokens
type TokenRepository interface {
	Create(ctx context.Context, token *Token) error
	FindByID(ctx context.Context, id interface{}) (*Token, error)
	Update(ctx context.Context, token *Token) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Token, error)
	FindByToken(ctx context.Context, token string) (*Token, error)
	FindByTokenForUpdate(ctx context.Context, token string) (*Token, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Token, error)
	FindByAccountIDAndIdentifier(ctx context.Context, accountID int64, identifier string) (*Token, error)
	DeleteExpiredTokens(ctx context.Context) (int, error)
	DeleteByAccountID(ctx context.Context, accountID int64) error
	DeleteByAccountIDAndIdentifier(ctx context.Context, accountID int64, identifier string) error
	FindValidTokens(ctx context.Context, filters map[string]interface{}) ([]*Token, error)
	FindTokensWithAccount(ctx context.Context, filters map[string]interface{}) ([]*Token, error)
	CleanupOldTokensForAccount(ctx context.Context, accountID int64, keepCount int) error

	// Token family tracking methods
	FindByFamilyID(ctx context.Context, familyID string) ([]*Token, error)
	DeleteByFamilyID(ctx context.Context, familyID string) error
	GetLatestTokenInFamily(ctx context.Context, familyID string) (*Token, error)
}

// PasswordResetTokenRepository defines operations for managing password reset tokens
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *PasswordResetToken) error
	FindByID(ctx context.Context, id interface{}) (*PasswordResetToken, error)
	Update(ctx context.Context, token *PasswordResetToken) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*PasswordResetToken, error)
	FindByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*PasswordResetToken, error)
	FindValidByToken(ctx context.Context, token string) (*PasswordResetToken, error)
	MarkAsUsed(ctx context.Context, tokenID int64) error
	DeleteExpiredTokens(ctx context.Context) (int, error)
	InvalidateTokensByAccountID(ctx context.Context, accountID int64) error
	FindTokensWithAccount(ctx context.Context, filters map[string]interface{}) ([]*PasswordResetToken, error)
}

// PasswordResetRateLimitRepository defines operations for managing password reset rate limiting.
type PasswordResetRateLimitRepository interface {
	CheckRateLimit(ctx context.Context, email string) (*RateLimitState, error)
	IncrementAttempts(ctx context.Context, email string) (*RateLimitState, error)
	CleanupExpired(ctx context.Context) (int, error)
}

// InvitationTokenRepository defines operations for managing invitation tokens.
type InvitationTokenRepository interface {
	Create(ctx context.Context, token *InvitationToken) error
	Update(ctx context.Context, token *InvitationToken) error
	FindByID(ctx context.Context, id interface{}) (*InvitationToken, error)
	FindByToken(ctx context.Context, token string) (*InvitationToken, error)
	FindValidByToken(ctx context.Context, token string, now time.Time) (*InvitationToken, error)
	FindByEmail(ctx context.Context, email string) ([]*InvitationToken, error)
	MarkAsUsed(ctx context.Context, id int64) error
	InvalidateByEmail(ctx context.Context, email string) (int, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*InvitationToken, error)
}
