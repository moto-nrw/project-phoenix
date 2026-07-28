package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Role Management

// CreateRole creates a new tenant-scoped role.
// baseRole is required — it maps this custom role to a system role for announcement targeting.
func (s *Service) CreateRole(ctx context.Context, name, description string, baseRole *string) (*auth.Role, error) {
	if baseRole == nil || *baseRole == "" {
		return nil, &AuthError{Op: "create role", Err: errors.New("base_role is required for custom roles")}
	}

	role := &auth.Role{
		Name:        name,
		Description: description,
		BaseRole:    baseRole,
	}

	// tenant_id is auto-set by base.Repository.Create via TenantScoped interface

	if err := s.repos.Role.Create(ctx, role); err != nil {
		return nil, &AuthError{Op: "create role", Err: err}
	}

	return role, nil
}

// GetRoleByID retrieves a role by its ID
func (s *Service) GetRoleByID(ctx context.Context, id int) (*auth.Role, error) {
	role, err := s.repos.Role.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: "get role", Err: err}
	}
	return role, nil
}

// ResolveAssignableSchoolRole returns the role only if it may be handed out for
// the given school. It is the layer boundary for the policy in
// role_assignment_policy.go: handlers must not reach a repository themselves
// (backend-conventions Rule 1), so the account-creating flows reach the same
// rules the invitation flow uses through here.
func (s *Service) ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*auth.Role, error) {
	role, err := ValidateAssignableSchoolRole(ctx, s.repos.Role, roleID, tenantID)
	if err != nil {
		return nil, err
	}
	role.Permissions, err = s.repos.Permission.FindByRoleID(ctx, role.ID)
	if err != nil {
		return nil, err
	}
	return role, nil
}

// UpdateRole updates an existing role. System roles cannot be modified.
func (s *Service) UpdateRole(ctx context.Context, role *auth.Role) error {
	// Always verify against the DB record — never trust the caller's IsSystem value
	existing, err := s.repos.Role.FindByID(ctx, role.ID)
	if err != nil {
		return &AuthError{Op: "update role", Err: ErrRoleNotFound}
	}
	if existing.IsSystem {
		return &AuthError{Op: "update role", Err: ErrSystemRoleImmutable}
	}
	if err := s.repos.Role.Update(ctx, role); err != nil {
		return &AuthError{Op: "update role", Err: err}
	}
	return nil
}

// DeleteRole deletes a role. System roles cannot be deleted.
func (s *Service) DeleteRole(ctx context.Context, id int) error {
	// Verify the role exists and check if it's a system role
	role, err := s.repos.Role.FindByID(ctx, int64(id))
	if err != nil {
		return &AuthError{Op: "delete role", Err: err}
	}
	if role.IsSystem {
		return &AuthError{Op: "delete role", Err: ErrSystemRoleImmutable}
	}

	// First remove all account-role mappings for this role (batch delete)
	if err := s.repos.AccountRole.DeleteByRoleID(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete account role mappings", Err: err}
	}

	// Then remove all role-permission mappings (batch delete)
	if err := s.repos.RolePermission.DeleteByRoleID(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete role permissions", Err: err}
	}

	// Finally delete the role
	if err := s.repos.Role.Delete(ctx, int64(id)); err != nil {
		return &AuthError{Op: "delete role", Err: err}
	}

	return nil
}

// ListRoles retrieves roles matching the provided filters
func (s *Service) ListRoles(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error) {
	roles, err := s.repos.Role.List(ctx, filters)
	if err != nil {
		return nil, &AuthError{Op: "list roles", Err: err}
	}
	return roles, nil
}

// AssignRoleToAccount assigns a role to an account
func (s *Service) AssignRoleToAccount(ctx context.Context, accountID, roleID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		// Verify account exists
		if _, err := s.repos.Account.FindByID(txCtx, int64(accountID)); err != nil {
			return &AuthError{Op: "assign role", Err: ErrAccountNotFound}
		}

		// Verify role exists
		if _, err := s.repos.Role.FindByID(txCtx, int64(roleID)); err != nil {
			return &AuthError{Op: "assign role", Err: errors.New("role not found")}
		}

		// Check if role is already assigned using the repository
		existingRole, err := s.repos.AccountRole.FindByAccountAndRole(txCtx, int64(accountID), int64(roleID))
		if err != nil && !strings.Contains(err.Error(), "no rows") {
			return &AuthError{Op: "check role assignment", Err: err}
		}

		if existingRole != nil {
			// Role already assigned, no action needed
			return nil
		}

		accountRole := &auth.AccountRole{
			AccountID: int64(accountID),
			RoleID:    int64(roleID),
		}
		accountRole.SetTenantID(tenant.FromContext(txCtx))

		if err := s.repos.AccountRole.Create(txCtx, accountRole); err != nil {
			return &AuthError{Op: "assign role to account", Err: err}
		}

		if err := s.repos.Token.DeleteByAccountID(txCtx, int64(accountID)); err != nil {
			return &AuthError{Op: "revoke tokens after role assignment", Err: err}
		}

		return nil
	})
}

// RemoveRoleFromAccount removes a role from an account
func (s *Service) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		existingRole, err := s.repos.AccountRole.FindByAccountAndRole(txCtx, int64(accountID), int64(roleID))
		if err != nil && !strings.Contains(err.Error(), "no rows") {
			return &AuthError{Op: "check role assignment", Err: err}
		}

		if existingRole == nil {
			return nil
		}

		if err := s.repos.AccountRole.DeleteByAccountAndRole(txCtx, int64(accountID), int64(roleID)); err != nil {
			return &AuthError{Op: "remove role from account", Err: err}
		}

		if err := s.repos.Token.DeleteByAccountID(txCtx, int64(accountID)); err != nil {
			return &AuthError{Op: "revoke tokens after role removal", Err: err}
		}

		return nil
	})
}

// GetAccountRoles retrieves all roles for an account
func (s *Service) GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error) {
	roles, err := s.repos.Role.FindByAccountID(ctx, int64(accountID))
	if err != nil {
		return nil, &AuthError{Op: "get account roles", Err: err}
	}
	return roles, nil
}

// GetAccountRoleNames batch-loads the primary role name for multiple accounts.
// Returns a map of accountID → role name.
func (s *Service) GetAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	roleNames, err := s.repos.Role.FindRoleNamesByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, &AuthError{Op: "get account role names", Err: err}
	}
	return roleNames, nil
}

// GetAccountEmailsByIDs batch-loads email addresses for multiple accounts.
// Returns a map of accountID → email.
func (s *Service) GetAccountEmailsByIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	emails, err := s.repos.Account.FindEmailsByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, &AuthError{Op: "get account emails by IDs", Err: err}
	}
	return emails, nil
}

// GetAccountAvatarsByIDs batch-loads avatar paths for multiple accounts.
// Returns a map of accountID → avatar path.
func (s *Service) GetAccountAvatarsByIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	avatars, err := s.repos.Account.FindAvatarsByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, &AuthError{Op: "get account avatars by IDs", Err: err}
	}
	return avatars, nil
}
