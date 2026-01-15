package auth

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Permission Management

// CreatePermission creates a new permission
func (s *Service) CreatePermission(ctx context.Context, name, description, resource, action string) (*auth.Permission, error) {
	permission := &auth.Permission{
		Name:        name,
		Description: description,
		Resource:    resource,
		Action:      action,
	}

	if err := s.repos.Permission.Create(ctx, permission); err != nil {
		return nil, &AuthError{Op: "create permission", Err: err}
	}

	return permission, nil
}

// GetPermissionByID retrieves a permission by its ID
func (s *Service) GetPermissionByID(ctx context.Context, id int) (*auth.Permission, error) {
	permission, err := s.repos.Permission.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get permission", Err: err}
	}
	return permission, nil
}

// GetPermissionByName retrieves a permission by its name
func (s *Service) GetPermissionByName(ctx context.Context, name string) (*auth.Permission, error) {
	permission, err := s.repos.Permission.FindByName(ctx, name)
	if err != nil {
		return nil, &AuthError{Op: "get permission by name", Err: err}
	}
	return permission, nil
}

// UpdatePermission updates an existing permission
func (s *Service) UpdatePermission(ctx context.Context, permission *auth.Permission) error {
	if err := s.repos.Permission.Update(ctx, permission); err != nil {
		return &AuthError{Op: "update permission", Err: err}
	}
	return nil
}

// DeletePermission deletes a permission
func (s *Service) DeletePermission(ctx context.Context, id int) error {
	// First remove all account-permission mappings
	accountPermissions, err := s.repos.AccountPermission.FindByPermissionID(ctx, int64(id))
	if err == nil {
		for _, ap := range accountPermissions {
			if err := s.repos.AccountPermission.Delete(ctx, ap.ID); err != nil {
				return &AuthError{Op: "delete account permissions", Err: err}
			}
		}
	}

	// Then remove all role-permission mappings for this permission
	rolePermissions, err := s.repos.RolePermission.FindByPermissionID(ctx, int64(id))
	if err == nil {
		for _, rp := range rolePermissions {
			if err := s.repos.RolePermission.Delete(ctx, rp.ID); err != nil {
				return &AuthError{Op: "delete role permissions", Err: err}
			}
		}
	}

	// Finally delete the permission
	if err := s.repos.Permission.Delete(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete permission", Err: err}
	}

	return nil
}

// ListPermissions retrieves permissions matching the provided filters
func (s *Service) ListPermissions(ctx context.Context, filters map[string]interface{}) ([]*auth.Permission, error) {
	permissions, err := s.repos.Permission.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list permissions", Err: err}
	}
	return permissions, nil
}

// GrantPermissionToAccount grants a permission directly to an account
func (s *Service) GrantPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	// Verify account exists
	if _, err := s.repos.Account.FindByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "grant permission", Err: ErrAccountNotFound}
	}

	// Verify permission exists
	if _, err := s.repos.Permission.FindByID(ctx, int64(permissionID)); err != nil {
		return &AuthError{Op: "grant permission", Err: ErrPermissionNotFound}
	}

	if err := s.repos.AccountPermission.GrantPermission(ctx, int64(accountID), int64(permissionID)); err != nil {
		return &AuthError{Op: "grant permission to account", Err: err}
	}

	return nil
}

// DenyPermissionToAccount explicitly denies a permission to an account
func (s *Service) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	// Verify account exists
	if _, err := s.repos.Account.FindByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: "deny permission", Err: ErrAccountNotFound}
	}

	// Verify permission exists
	if _, err := s.repos.Permission.FindByID(ctx, int64(permissionID)); err != nil {
		return &AuthError{Op: "deny permission", Err: ErrPermissionNotFound}
	}

	if err := s.repos.AccountPermission.DenyPermission(ctx, int64(accountID), int64(permissionID)); err != nil {
		return &AuthError{Op: "deny permission to account", Err: err}
	}

	return nil
}

// RemovePermissionFromAccount removes a permission from an account
func (s *Service) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int) error {
	if err := s.repos.AccountPermission.RemovePermission(ctx, int64(accountID), int64(permissionID)); err != nil {
		return &AuthError{Op: "remove permission from account", Err: err}
	}
	return nil
}

// GetAccountPermissions retrieves all permissions for an account (direct and role-based)
func (s *Service) GetAccountPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	permissions, err := s.getAccountPermissions(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account permissions", Err: err}
	}
	return permissions, nil
}

// GetAccountDirectPermissions retrieves only direct permissions for an account (not role-based)
func (s *Service) GetAccountDirectPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	permissions, err := s.repos.Permission.FindDirectByAccountID(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account direct permissions", Err: err}
	}
	return permissions, nil
}

// AssignPermissionToRole assigns a permission to a role
func (s *Service) AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error {
	// Verify role exists
	if _, err := s.repos.Role.FindByID(ctx, int64(roleID)); err != nil {
		return &AuthError{Op: opAssignPermissionToRole, Err: errors.New("role not found")}
	}

	// Verify permission exists
	if _, err := s.repos.Permission.FindByID(ctx, int64(permissionID)); err != nil {
		return &AuthError{Op: opAssignPermissionToRole, Err: ErrPermissionNotFound}
	}

	if err := s.repos.Permission.AssignPermissionToRole(ctx, int64(roleID), int64(permissionID)); err != nil {
		return &AuthError{Op: opAssignPermissionToRole, Err: err}
	}

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (s *Service) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error {
	if err := s.repos.Permission.RemovePermissionFromRole(ctx, int64(roleID), int64(permissionID)); err != nil {
		return &AuthError{Op: "remove permission from role", Err: err}
	}
	return nil
}

// GetRolePermissions retrieves all permissions for a role
func (s *Service) GetRolePermissions(ctx context.Context, roleID int) ([]*auth.Permission, error) {
	permissions, err := s.repos.Permission.FindByRoleID(ctx, int64(roleID))
	if err != nil {
		return nil, &AuthError{Op: "get role permissions", Err: err}
	}
	return permissions, nil
}
