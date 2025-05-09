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
}

// PermissionRepository defines operations for managing permissions
type PermissionRepository interface {
	Create(ctx context.Context, permission *Permission) error
	FindByID(ctx context.Context, id interface{}) (*Permission, error)
	Update(ctx context.Context, permission *Permission) error
	Delete(ctx context.Context, id interface{}) error
	List(ctx context.Context, filters map[string]interface{}) ([]*Permission, error)
	FindByName(ctx context.Context, name string) (*Permission, error)
	FindByAccountID(ctx context.Context, accountID int64) ([]*Permission, error)
	FindByRoleID(ctx context.Context, roleID int64) ([]*Permission, error)
	AssignPermissionToAccount(ctx context.Context, accountID int64, permissionID int64) error
	RemovePermissionFromAccount(ctx context.Context, accountID int64, permissionID int64) error
	AssignPermissionToRole(ctx context.Context, roleID int64, permissionID int64) error
	RemovePermissionFromRole(ctx context.Context, roleID int64, permissionID int64) error
}
