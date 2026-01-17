package auth

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
)

// GetAccountByID retrieves an account by ID
func (s *Service) GetAccountByID(ctx context.Context, id int) (*auth.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}
	return account, nil
}

// GetAccountByEmail retrieves an account by email
func (s *Service) GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: "get account by email", Err: ErrAccountNotFound}
	}
	return account, nil
}

// getAccountPermissions retrieves all permissions for an account (both direct and role-based)
func (s *Service) getAccountPermissions(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	// Get permissions directly assigned to the account
	directPermissions, err := s.repos.Permission.FindByAccountID(ctx, accountID)
	if err != nil {
		return []*auth.Permission{}, err
	}

	// Create a map to prevent duplicate permissions
	permMap := make(map[int64]*auth.Permission)

	// Add direct permissions to the map
	s.addPermissionsToMap(permMap, directPermissions)

	// Add role-based permissions to the map
	s.addRolePermissionsToMap(ctx, accountID, permMap)

	// Convert map to slice
	return s.convertPermissionMapToSlice(permMap), nil
}

// addPermissionsToMap adds permissions to the map to prevent duplicates
func (s *Service) addPermissionsToMap(permMap map[int64]*auth.Permission, permissions []*auth.Permission) {
	for _, p := range permissions {
		permMap[p.ID] = p
	}
}

// addRolePermissionsToMap adds permissions from account roles to the map
func (s *Service) addRolePermissionsToMap(ctx context.Context, accountID int64, permMap map[int64]*auth.Permission) {
	accountRoles, err := s.repos.AccountRole.FindByAccountID(ctx, accountID)
	if err != nil {
		return // Continue even if error occurs
	}

	for _, ar := range accountRoles {
		if ar.RoleID <= 0 {
			continue
		}

		rolePermissions, err := s.repos.Permission.FindByRoleID(ctx, ar.RoleID)
		if err != nil {
			continue // Continue even if error occurs
		}

		s.addPermissionsToMap(permMap, rolePermissions)
	}
}

// convertPermissionMapToSlice converts permission map to slice
func (s *Service) convertPermissionMapToSlice(permMap map[int64]*auth.Permission) []*auth.Permission {
	permissions := make([]*auth.Permission, 0, len(permMap))
	for _, p := range permMap {
		permissions = append(permissions, p)
	}
	return permissions
}

// getRoleByName retrieves a role by its name
func (s *Service) getRoleByName(ctx context.Context, name string) (*auth.Role, error) {
	return s.repos.Permission.FindByRoleByName(ctx, name)
}
