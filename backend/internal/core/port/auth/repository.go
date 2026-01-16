package auth

import (
	"context"
	"time"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
)

// AccountRepository defines operations for managing accounts.
type AccountRepository interface {
	Create(ctx context.Context, account *domain.Account) error
	FindByID(ctx context.Context, id interface{}) (*domain.Account, error)
	FindByEmail(ctx context.Context, email string) (*domain.Account, error)
	FindByUsername(ctx context.Context, username string) (*domain.Account, error)
	Update(ctx context.Context, account *domain.Account) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.Account, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	FindByRole(ctx context.Context, role string) ([]*domain.Account, error)
	FindAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*domain.Account, error)
}

// RoleRepository defines operations for managing roles.
type RoleRepository interface {
	Create(ctx context.Context, role *domain.Role) error
	FindByID(ctx context.Context, id interface{}) (*domain.Role, error)
	Update(ctx context.Context, role *domain.Role) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.Role, error)
	FindByName(ctx context.Context, name string) (*domain.Role, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*domain.Role, error)
	GetRoleWithPermissions(ctx context.Context, roleID int64) (*domain.Role, error)
}

// PermissionRepository defines operations for managing permissions.
type PermissionRepository interface {
	Create(ctx context.Context, permission *domain.Permission) error
	FindByID(ctx context.Context, id interface{}) (*domain.Permission, error)
	Update(ctx context.Context, permission *domain.Permission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.Permission, error)
	FindByName(ctx context.Context, name string) (*domain.Permission, error)
	FindByResourceAction(ctx context.Context, resource, action string) (*domain.Permission, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*domain.Permission, error)
	FindDirectByAccountID(ctx context.Context, accountID int64) ([]*domain.Permission, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*domain.Permission, error)
	FindByRoleByName(ctx context.Context, roleName string) (*domain.Role, error)
	AssignPermissionToAccount(ctx context.Context, accountID int64, permissionID int64) error
	RemovePermissionFromAccount(ctx context.Context, accountID int64, permissionID int64) error
	AssignPermissionToRole(ctx context.Context, roleID int64, permissionID int64) error
	RemovePermissionFromRole(ctx context.Context, roleID int64, permissionID int64) error
}

// AccountParentRepository defines operations for managing parent accounts.
type AccountParentRepository interface {
	Create(ctx context.Context, account *domain.AccountParent) error
	FindByID(ctx context.Context, id interface{}) (*domain.AccountParent, error)
	FindByEmail(ctx context.Context, email string) (*domain.AccountParent, error)
	FindByUsername(ctx context.Context, username string) (*domain.AccountParent, error)
	Update(ctx context.Context, account *domain.AccountParent) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.AccountParent, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
}

// RolePermissionRepository defines operations for managing role-permission mappings.
type RolePermissionRepository interface {
	Create(ctx context.Context, rolePermission *domain.RolePermission) error
	FindByID(ctx context.Context, id interface{}) (*domain.RolePermission, error)
	Update(ctx context.Context, rolePermission *domain.RolePermission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.RolePermission, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*domain.RolePermission, error)
	FindByPermissionID(ctx context.Context, permissionID int64) ([]*domain.RolePermission, error)
	FindByRoleAndPermission(ctx context.Context, roleID, permissionID int64) (*domain.RolePermission, error)
	DeleteByRoleAndPermission(ctx context.Context, roleID, permissionID int64) error
	DeleteByRoleID(ctx context.Context, roleID int64) error
	DeleteByPermissionID(ctx context.Context, permissionID int64) error
	FindRolePermissionsWithDetails(ctx context.Context, filters map[string]interface{}) ([]*domain.RolePermission, error)
}

// AccountRoleRepository defines operations for managing account-role mappings.
type AccountRoleRepository interface {
	Create(ctx context.Context, accountRole *domain.AccountRole) error
	FindByID(ctx context.Context, id interface{}) (*domain.AccountRole, error)
	Update(ctx context.Context, accountRole *domain.AccountRole) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.AccountRole, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*domain.AccountRole, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*domain.AccountRole, error)
	FindByAccountAndRole(ctx context.Context, accountID, roleID int64) (*domain.AccountRole, error)
	DeleteByAccountAndRole(ctx context.Context, accountID, roleID int64) error
	DeleteByAccountID(ctx context.Context, accountID int64) error
	DeleteByRoleID(ctx context.Context, roleID int64) error
	FindAccountRolesWithDetails(ctx context.Context, filters map[string]interface{}) ([]*domain.AccountRole, error)
}

// AccountPermissionRepository defines operations for managing account-permission mappings.
type AccountPermissionRepository interface {
	Create(ctx context.Context, accountPermission *domain.AccountPermission) error
	FindByID(ctx context.Context, id interface{}) (*domain.AccountPermission, error)
	Update(ctx context.Context, accountPermission *domain.AccountPermission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.AccountPermission, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*domain.AccountPermission, error)
	FindByPermissionID(ctx context.Context, permissionID int64) ([]*domain.AccountPermission, error)
	FindByAccountAndPermission(ctx context.Context, accountID, permissionID int64) (*domain.AccountPermission, error)
	GrantPermission(ctx context.Context, accountID, permissionID int64) error
	DenyPermission(ctx context.Context, accountID, permissionID int64) error
	RemovePermission(ctx context.Context, accountID, permissionID int64) error
	DeleteByPermissionID(ctx context.Context, permissionID int64) error
	FindAccountPermissionsWithDetails(ctx context.Context, filters map[string]interface{}) ([]*domain.AccountPermission, error)
}

// TokenRepository defines operations for managing authentication tokens.
type TokenRepository interface {
	Create(ctx context.Context, token *domain.Token) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.Token, error)
	FindByToken(ctx context.Context, token string) (*domain.Token, error)
	FindByTokenForUpdate(ctx context.Context, token string) (*domain.Token, error)
	DeleteExpiredTokens(ctx context.Context) (int, error)
	DeleteByAccountID(ctx context.Context, accountID int64) error
	CleanupOldTokensForAccount(ctx context.Context, accountID int64, keepCount int) error

	// Token family tracking methods
	DeleteByFamilyID(ctx context.Context, familyID string) error
	GetLatestTokenInFamily(ctx context.Context, familyID string) (*domain.Token, error)
}

// PasswordResetTokenRepository defines operations for managing password reset tokens.
type PasswordResetTokenRepository interface {
	Create(ctx context.Context, token *domain.PasswordResetToken) error
	FindByID(ctx context.Context, id interface{}) (*domain.PasswordResetToken, error)
	Update(ctx context.Context, token *domain.PasswordResetToken) error
	Delete(ctx context.Context, id interface{}) error
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.PasswordResetToken, error)
	FindByToken(ctx context.Context, token string) (*domain.PasswordResetToken, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*domain.PasswordResetToken, error)
	FindValidByToken(ctx context.Context, token string) (*domain.PasswordResetToken, error)
	MarkAsUsed(ctx context.Context, tokenID int64) error
	DeleteExpiredTokens(ctx context.Context) (int, error)
	InvalidateTokensByAccountID(ctx context.Context, accountID int64) error
	FindTokensWithAccount(ctx context.Context, filters map[string]interface{}) ([]*domain.PasswordResetToken, error)
}

// PasswordResetRateLimitRepository defines operations for managing password reset rate limiting.
type PasswordResetRateLimitRepository interface {
	CheckRateLimit(ctx context.Context, email string) (*domain.RateLimitState, error)
	IncrementAttempts(ctx context.Context, email string) (*domain.RateLimitState, error)
	CleanupExpired(ctx context.Context) (int, error)
}

// InvitationTokenRepository defines operations for managing invitation tokens.
type InvitationTokenRepository interface {
	Create(ctx context.Context, token *domain.InvitationToken) error
	Update(ctx context.Context, token *domain.InvitationToken) error
	FindByID(ctx context.Context, id interface{}) (*domain.InvitationToken, error)
	FindByToken(ctx context.Context, token string) (*domain.InvitationToken, error)
	UpdateDeliveryResult(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error
	FindValidByToken(ctx context.Context, token string, now time.Time) (*domain.InvitationToken, error)
	FindByEmail(ctx context.Context, email string) ([]*domain.InvitationToken, error)
	MarkAsUsed(ctx context.Context, id int64) error
	InvalidateByEmail(ctx context.Context, email string) (int, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*domain.InvitationToken, error)
}

// GuardianInvitationRepository defines operations for managing guardian invitations.
type GuardianInvitationRepository interface {
	// Create inserts a new guardian invitation
	Create(ctx context.Context, invitation *domain.GuardianInvitation) error

	// Update updates an existing guardian invitation
	Update(ctx context.Context, invitation *domain.GuardianInvitation) error

	// FindByID retrieves a guardian invitation by ID
	FindByID(ctx context.Context, id int64) (*domain.GuardianInvitation, error)

	// FindByToken retrieves a guardian invitation by token
	FindByToken(ctx context.Context, token string) (*domain.GuardianInvitation, error)

	// FindByGuardianProfileID retrieves invitations for a guardian profile
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*domain.GuardianInvitation, error)

	// FindPending retrieves all pending (not accepted, not expired) invitations
	FindPending(ctx context.Context) ([]*domain.GuardianInvitation, error)

	// FindExpired retrieves all expired invitations
	FindExpired(ctx context.Context) ([]*domain.GuardianInvitation, error)

	// MarkAsAccepted marks an invitation as accepted
	MarkAsAccepted(ctx context.Context, id int64) error

	// UpdateEmailStatus updates the email delivery status
	UpdateEmailStatus(ctx context.Context, id int64, sentAt *time.Time, emailError *string, retryCount int) error

	// DeleteExpired deletes expired invitations
	DeleteExpired(ctx context.Context) (int, error)

	// Count returns the total number of guardian invitations
	Count(ctx context.Context) (int, error)
}
