package auth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
)

// accountMetadata holds account-related metadata for JWT claims
type accountMetadata struct {
	roleNames      []string
	permissionStrs []string
	username       string
	firstName      string
	lastName       string
	isAdmin        bool
	isTeacher      bool
}

// loadAccountMetadata loads roles, permissions, and person information
// Returns partial data with logged warnings if any lookups fail
func (s *Service) loadAccountMetadata(ctx context.Context, account *auth.Account) *accountMetadata {
	s.ensureAccountRolesLoaded(ctx, account)

	permissions := s.loadAccountPermissions(ctx, account.ID)
	roleNames := s.extractRoleNames(account.Roles)
	permissionStrs := s.extractPermissionNames(permissions)

	username := s.extractUsername(account)
	firstName, lastName := s.loadPersonNames(ctx, account.ID)
	isAdmin, isTeacher := s.checkRoleFlags(roleNames)

	return &accountMetadata{
		roleNames:      roleNames,
		permissionStrs: permissionStrs,
		username:       username,
		firstName:      firstName,
		lastName:       lastName,
		isAdmin:        isAdmin,
		isTeacher:      isTeacher,
	}
}

// ensureAccountRolesLoaded loads account roles if not already loaded
func (s *Service) ensureAccountRolesLoaded(ctx context.Context, account *auth.Account) {
	if len(account.Roles) > 0 {
		return
	}

	accountRoles, err := s.repos.AccountRole.FindByAccountID(ctx, account.ID)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithField("account_id", account.ID).WithError(err).Warn("failed to load roles for account")
		}
		return
	}

	for _, ar := range accountRoles {
		if ar.Role != nil {
			account.Roles = append(account.Roles, ar.Role)
		}
	}
}

// loadAccountPermissions retrieves permissions for the account
func (s *Service) loadAccountPermissions(ctx context.Context, accountID int64) []*auth.Permission {
	permissions, err := s.getAccountPermissions(ctx, accountID)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithField("account_id", accountID).WithError(err).Warn("failed to load permissions for account")
		}
		return []*auth.Permission{}
	}
	return permissions
}

// ensureAccountPermissionsLoaded loads account permissions if not already loaded
func (s *Service) ensureAccountPermissionsLoaded(ctx context.Context, account *auth.Account) {
	if len(account.Permissions) > 0 {
		return
	}

	permissions, err := s.getAccountPermissions(ctx, account.ID)
	if err != nil {
		if logger.Logger != nil {
			logger.Logger.WithField("account_id", account.ID).WithError(err).Warn("failed to load permissions for account")
		}
		return
	}

	account.Permissions = permissions
}

// extractRoleNames converts roles to string slice
func (s *Service) extractRoleNames(roles []*auth.Role) []string {
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}
	return roleNames
}

// extractPermissionNames converts permissions to string slice
func (s *Service) extractPermissionNames(permissions []*auth.Permission) []string {
	permissionStrs := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionStrs = append(permissionStrs, perm.GetFullName())
	}
	return permissionStrs
}

// extractUsername safely extracts username from account
func (s *Service) extractUsername(account *auth.Account) string {
	if account.Username != nil {
		return *account.Username
	}
	return ""
}

// loadPersonNames retrieves first and last name from person record
func (s *Service) loadPersonNames(ctx context.Context, accountID int64) (string, string) {
	person, err := s.repos.Person.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return "", ""
	}
	return person.FirstName, person.LastName
}

// checkRoleFlags determines if account has admin or teacher roles
func (s *Service) checkRoleFlags(roleNames []string) (bool, bool) {
	isAdmin := false
	isTeacher := false

	for _, roleName := range roleNames {
		if roleName == "admin" {
			isAdmin = true
		}
		if roleName == "teacher" {
			isTeacher = true
		}
	}

	return isAdmin, isTeacher
}
