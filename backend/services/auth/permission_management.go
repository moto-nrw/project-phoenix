package auth

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	// First remove all account-permission mappings (batch delete)
	if err := s.repos.AccountPermission.DeleteByPermissionID(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete account permissions", Err: err}
	}

	// Then remove all role-permission mappings for this permission (batch delete)
	if err := s.repos.RolePermission.DeleteByPermissionID(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete role permissions", Err: err}
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
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.lockManageableAccount(txCtx, accountID, "grant permission"); err != nil {
			return err
		}
		if _, err := s.repos.Permission.FindByID(txCtx, int64(permissionID)); err != nil {
			return &AuthError{Op: "grant permission", Err: ErrPermissionNotFound}
		}
		if err := s.repos.AccountPermission.GrantPermission(txCtx, int64(accountID), int64(permissionID)); err != nil {
			return &AuthError{Op: "grant permission to account", Err: err}
		}
		return nil
	})
}

// DenyPermissionToAccount explicitly denies a permission to an account
func (s *Service) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.lockManageableAccount(txCtx, accountID, "deny permission"); err != nil {
			return err
		}
		if _, err := s.repos.Permission.FindByID(txCtx, int64(permissionID)); err != nil {
			return &AuthError{Op: "deny permission", Err: ErrPermissionNotFound}
		}
		if err := s.repos.AccountPermission.DenyPermission(txCtx, int64(accountID), int64(permissionID)); err != nil {
			return &AuthError{Op: "deny permission to account", Err: err}
		}
		return nil
	})
}

// RemovePermissionFromAccount removes a permission from an account
func (s *Service) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.lockManageableAccount(txCtx, accountID, "remove permission"); err != nil {
			return err
		}
		if err := s.repos.AccountPermission.RemovePermission(txCtx, int64(accountID), int64(permissionID)); err != nil {
			return &AuthError{Op: "remove permission from account", Err: err}
		}
		return nil
	})
}

// GetAccountPermissions retrieves all permissions for an account (direct and role-based)
func (s *Service) GetAccountPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	if _, err := s.repos.Account.FindManageableByID(ctx, int64(accountID)); err != nil {
		return nil, &AuthError{Op: "get account permissions", Err: ErrAccountNotFound}
	}
	if err := s.ensureOrganizationRBACMembership(ctx, accountID, "get account permissions", false); err != nil {
		return nil, err
	}
	permissions, err := s.getAccountPermissions(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account permissions", Err: err}
	}
	return permissions, nil
}

// GetAccountDirectPermissions retrieves only direct permissions for an account (not role-based)
func (s *Service) GetAccountDirectPermissions(ctx context.Context, accountID int) ([]*auth.Permission, error) {
	if _, err := s.repos.Account.FindManageableByID(ctx, int64(accountID)); err != nil {
		return nil, &AuthError{Op: "get account direct permissions", Err: ErrAccountNotFound}
	}
	if err := s.ensureOrganizationRBACMembership(ctx, accountID, "get account direct permissions", false); err != nil {
		return nil, err
	}
	permissions, err := s.repos.Permission.FindDirectByAccountID(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account direct permissions", Err: err}
	}
	return permissions, nil
}

func (s *Service) lockManageableAccount(ctx context.Context, accountID int, op string) error {
	if _, err := s.repos.Account.FindManageableByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	if err := s.ensureOrganizationRBACMembership(ctx, accountID, op, false); err != nil {
		return err
	}
	if _, err := s.repos.Account.FindByIDForUpdate(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	if _, err := s.repos.Account.FindManageableByID(ctx, int64(accountID)); err != nil {
		return &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	if err := s.ensureOrganizationRBACMembership(ctx, accountID, op, true); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensureOrganizationRBACMembership(ctx context.Context, accountID int, op string, lock bool) error {
	if tenant.ScopeFromContext(ctx) != tenant.ScopeOrg {
		return nil
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	var (
		exists bool
		err    error
	)
	if lock {
		exists, err = s.repos.AccountTenant.ExistsActiveByAccountAndTenantForShare(ctx, int64(accountID), tenantID)
	} else {
		exists, err = s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, int64(accountID), tenantID)
	}
	if err != nil {
		return &AuthError{Op: op, Err: err}
	}
	if !exists {
		return &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	return nil
}

// AssignPermissionToRole assigns a permission to a role. System roles cannot be modified.
func (s *Service) AssignPermissionToRole(ctx context.Context, roleID, permissionID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := s.lockMutableRole(txCtx, roleID, opAssignPermissionToRole); err != nil {
			return err
		}
		if _, err := s.repos.Permission.FindByID(txCtx, int64(permissionID)); err != nil {
			return &AuthError{Op: opAssignPermissionToRole, Err: ErrPermissionNotFound}
		}
		if err := s.repos.Permission.AssignPermissionToRole(txCtx, int64(roleID), int64(permissionID)); err != nil {
			return &AuthError{Op: opAssignPermissionToRole, Err: err}
		}
		return nil
	})
}

// lockMutableRole serializes every role-permission mutation on the role row.
// It preserves infrastructure failures so callers do not turn them into a
// misleading not-found response.
func (s *Service) lockMutableRole(ctx context.Context, roleID int, op string) (*auth.Role, error) {
	role, err := s.repos.Role.FindByIDForUpdate(ctx, int64(roleID))
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, &AuthError{Op: op, Err: ErrRoleNotFound}
		}
		return nil, &AuthError{Op: op, Err: err}
	}
	if role.IsSystem {
		return nil, &AuthError{Op: op, Err: ErrSystemRoleImmutable}
	}
	return role, nil
}

// ReplaceRolePermissions applies a complete permission selection to a custom
// role in one transaction. All requested permissions are validated before the
// current selection is changed, so an invalid request leaves it untouched.
func (s *Service) ReplaceRolePermissions(ctx context.Context, roleID int, permissionIDs []int64) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		// The role row serializes full replacements and the legacy one-at-a-time
		// mutations. Lock it before loading mappings so each operation observes
		// the preceding committed selection.
		if _, err := s.lockMutableRole(txCtx, roleID, "replace role permissions"); err != nil {
			return err
		}

		desired := make(map[int64]struct{}, len(permissionIDs))
		for _, permissionID := range permissionIDs {
			if permissionID <= 0 {
				return &AuthError{Op: "replace role permissions", Err: ErrPermissionNotFound}
			}
			if _, err := s.repos.Permission.FindByID(txCtx, permissionID); err != nil {
				if modelBase.IsNoRows(err) {
					return &AuthError{Op: "replace role permissions", Err: ErrPermissionNotFound}
				}
				return &AuthError{Op: "replace role permissions", Err: err}
			}
			desired[permissionID] = struct{}{}
		}

		currentPermissions, err := s.repos.Permission.FindByRoleID(txCtx, int64(roleID))
		if err != nil {
			return &AuthError{Op: "replace role permissions", Err: err}
		}
		current := make(map[int64]struct{}, len(currentPermissions))
		for _, permission := range currentPermissions {
			current[permission.ID] = struct{}{}
			if _, keep := desired[permission.ID]; keep {
				continue
			}
			if err := s.repos.Permission.RemovePermissionFromRole(txCtx, int64(roleID), permission.ID); err != nil {
				return &AuthError{Op: "replace role permissions", Err: err}
			}
		}

		for permissionID := range desired {
			if _, assigned := current[permissionID]; assigned {
				continue
			}
			if err := s.repos.Permission.AssignPermissionToRole(txCtx, int64(roleID), permissionID); err != nil {
				return &AuthError{Op: "replace role permissions", Err: err}
			}
		}

		return nil
	})
}

// RemovePermissionFromRole removes a permission from a role. System roles cannot be modified.
func (s *Service) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if _, err := s.lockMutableRole(txCtx, roleID, "remove permission from role"); err != nil {
			return err
		}
		if err := s.repos.Permission.RemovePermissionFromRole(txCtx, int64(roleID), int64(permissionID)); err != nil {
			return &AuthError{Op: "remove permission from role", Err: err}
		}
		return nil
	})
}

// GetRolePermissions retrieves all permissions for a role
func (s *Service) GetRolePermissions(ctx context.Context, roleID int) ([]*auth.Permission, error) {
	permissions, err := s.repos.Permission.FindByRoleID(ctx, int64(roleID))
	if err != nil {
		return nil, &AuthError{Op: "get role permissions", Err: err}
	}
	return permissions, nil
}

// getAccountPermissions retrieves all permissions for an account (both direct and role-based)
// Uses the repository's FindByAccountID which combines direct and role-based permissions in a single query
func (s *Service) getAccountPermissions(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	// FindByAccountID already uses a CTE to combine direct and role-based permissions
	// in a single query, avoiding N+1 queries
	return s.repos.Permission.FindByAccountID(ctx, accountID)
}

var (
	// ErrPermissionNotFound returned when permission doesn't exist
	ErrPermissionNotFound = errors.New("permission not found")
)

// PermissionOperations manage permissions and their grants.
type PermissionOperations interface {
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
	ReplaceRolePermissions(ctx context.Context, roleID int, permissionIDs []int64) error
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID int) error
	GetRolePermissions(ctx context.Context, roleID int) ([]*auth.Permission, error)
}
