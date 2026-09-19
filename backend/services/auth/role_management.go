package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
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
	return s.runInTx(ctx, func(txCtx context.Context) error {
		// ReplaceRolePermissions locks the role before its mappings. Take the
		// same lock first so deleting a role cannot wait for a mapping while a
		// concurrent replacement waits for the role.
		role, err := s.repos.Role.FindByIDForUpdate(txCtx, int64(id))
		if err != nil {
			return &AuthError{Op: "delete role", Err: err}
		}
		if role.IsSystem {
			return &AuthError{Op: "delete role", Err: ErrSystemRoleImmutable}
		}

		if err := s.repos.AccountRole.DeleteByRoleID(txCtx, int64(id)); err != nil {
			return &AuthError{Op: "delete account role mappings", Err: err}
		}
		if err := s.repos.RolePermission.DeleteByRoleID(txCtx, int64(id)); err != nil {
			return &AuthError{Op: "delete role permissions", Err: err}
		}
		if err := s.repos.Role.Delete(txCtx, int64(id)); err != nil {
			return &AuthError{Op: "delete role", Err: err}
		}

		return nil
	})
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
	var revoked []RevokedSession
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		// Serialize assignments with tenant-access revocation, which holds the
		// same account lock while removing the tenant's roles and mapping.
		if err := s.lockManageableAccount(txCtx, accountID, "assign role"); err != nil {
			return err
		}

		// System roles have tenant_id NULL and must remain resolvable while the
		// assignment itself is written for the caller's tenant. Custom roles are
		// still restricted to that tenant.
		roleLookupCtx := tenant.ContextWithoutTenant(txCtx)
		role, err := s.repos.Role.FindByID(roleLookupCtx, int64(roleID))
		if err != nil {
			return &AuthError{Op: "assign role", Err: errors.New("role not found")}
		}
		if tenantID := tenant.FromContext(txCtx); tenantID > 0 && role.TenantID != nil && *role.TenantID != tenantID {
			return &AuthError{Op: "assign role", Err: errors.New("role not found")}
		}

		if !IsLehrkraftSystemRole(role) {
			isLehrkraft, roleErr := s.accountHoldsLehrkraftRole(txCtx, int64(accountID))
			if roleErr != nil {
				return &AuthError{Op: "assign role", Err: roleErr}
			}
			if isLehrkraft {
				return &AuthError{Op: "assign role", Err: ErrLehrkraftRoleImmutable}
			}
		}

		// Server-side mirror of the operator guards (#1772): the Lehrkraft
		// role must not land on an account whose identity at this school
		// carries a live caregiver profile — a direct call to the tenant
		// RBAC endpoint would otherwise strand users.teachers and its group
		// supervisions under a class_day-only JWT. Same rule as
		// GrantAccountTenantAccess / UpdateAccountTenantRole and the
		// invitation flow.
		if IsLehrkraftSystemRole(role) {
			hasProfile, profErr := HasLiveCaregiverProfile(txCtx, s.repos.Person, s.repos.Staff, s.repos.Teacher, int64(accountID))
			if profErr != nil {
				return &AuthError{Op: "assign role", Err: profErr}
			}
			if hasProfile {
				return &AuthError{Op: "assign role", Err: ErrRoleLehrkraftCaregiverProfile}
			}
		}

		// And the same swap in reverse (#1772): a Lehrkraft account has no
		// users.teachers row by construction, so handing it a caregiver role
		// here would grant the permissions without the profile they read
		// through — no groups, no supervision, an empty caregiver landing
		// page. The operator role change provisions the profile before it
		// assigns the role and therefore passes; this endpoint has no
		// identity fields, so the switch belongs to offboarding plus a fresh
		// account. Roles that legitimately run without a profile (admin) are
		// unaffected.
		provisioning, err := s.schoolIdentity("assign role")
		if err != nil {
			return err
		}
		if provisioning.RoleNeedsCaregiverProfile(RoleFactsOf(role)) {
			isLehrkraft, roleErr := s.accountHoldsLehrkraftRole(txCtx, int64(accountID))
			if roleErr != nil {
				return &AuthError{Op: "assign role", Err: roleErr}
			}
			if isLehrkraft {
				hasProfile, profErr := HasLiveCaregiverProfile(txCtx, s.repos.Person, s.repos.Staff, s.repos.Teacher, int64(accountID))
				if profErr != nil {
					return &AuthError{Op: "assign role", Err: profErr}
				}
				if !hasProfile {
					return &AuthError{Op: "assign role", Err: ErrRoleCaregiverNeedsProfile}
				}
			}
		}

		// Check if role is already assigned using the repository
		existingRole, err := s.repos.AccountRole.FindByAccountAndRole(txCtx, int64(accountID), int64(roleID))
		if err != nil && !strings.Contains(err.Error(), "no rows") {
			return &AuthError{Op: "check role assignment", Err: err}
		}

		if existingRole == nil {
			accountRole := &auth.AccountRole{
				AccountID: int64(accountID),
				RoleID:    int64(roleID),
			}
			accountRole.SetTenantID(tenant.FromContext(txCtx))

			if err := s.repos.AccountRole.Create(txCtx, accountRole); err != nil {
				return &AuthError{Op: "assign role to account", Err: err}
			}

			sessions, err := s.accountSessions("revoke tokens after role assignment")
			if err != nil {
				return err
			}
			tokens, err := sessions.DeleteAccountSessionsWithAudit(txCtx, int64(accountID), "role_changed", "", "")
			if err != nil {
				return &AuthError{Op: "revoke tokens after role assignment", Err: err}
			}
			revoked = tokens
		}

		// Handing out a staff-tier role is the same act as inviting one, so it
		// owes the same identity (#2222). Without this, this endpoint is a
		// fifth way to produce the broken state: an account that holds the
		// role, logs in, and is not staff as far as the database is concerned.
		//
		// Runs for an already-assigned role too, so a repeated call repairs an
		// account the old behaviour left half-written.
		return s.ensureIdentityForAssignedRole(txCtx, int64(accountID), role)
	})
	if err != nil {
		return err
	}
	if s.sessions != nil {
		s.sessions.QueuePushCleanup(ctx, int64(accountID), revoked, "role_changed")
	}
	return nil
}

// ReplaceAccountRole makes roleID the account's only staff role at the caller's
// tenant. The Konto field "Systemrolle" is single-valued (#3116): after a
// successful call the account holds the target role and no other staff role at
// this school, however many an earlier half-finished swap or a direct POST left
// behind. Guardian-tier roles are the exception and stay untouched: they carry
// parent-portal access, which is a separate relationship and never something a
// staff role change may revoke.
//
// The target is assigned first so the account never temporarily loses its
// staff role; a later failure rolls back the complete exchange.
func (s *Service) ReplaceAccountRole(ctx context.Context, accountID, roleID int) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		// Take the same account lock as single-role mutations before changing an
		// assignment. This serializes replacements with individual assignments
		// and removals for the same account.
		if err := s.lockManageableAccount(txCtx, accountID, "replace account role"); err != nil {
			return err
		}

		if err := s.AssignRoleToAccount(txCtx, accountID, roleID); err != nil {
			return err
		}

		// FindByAccountID applies the tenant filter, so roles held at other
		// schools are never in this list and never removed here.
		current, err := s.repos.Role.FindByAccountID(txCtx, int64(accountID))
		if err != nil {
			return &AuthError{Op: "replace account role", Err: err}
		}
		for _, role := range current {
			if role.ID == int64(roleID) || isGuardianTierRole(role) {
				continue
			}
			if err := s.RemoveRoleFromAccount(txCtx, accountID, int(role.ID)); err != nil {
				return err
			}
		}

		return nil
	})
}

// isGuardianTierRole reports whether a role hands out guardian privileges. The
// decision is by tier, not by name, for the same reason as in
// ValidateAssignableSchoolRole: a school role labelled "Guardian" is not the
// parent-portal role, and the parent-portal role may carry a custom label.
func isGuardianTierRole(role *auth.Role) bool {
	return authorize.EffectiveBaseRole(role) == auth.BaseRoleGuardian
}

// ensureIdentityForAssignedRole completes the school identity chain for a role
// that was just assigned, inside the caller's transaction.
//
// It never creates the person: this endpoint carries no identity fields, and
// inventing a name is not something a role assignment may do — the same line
// the operator role change draws. An account without a person at this school is
// left to the flows that do have a name (staff creation, invitation), which is
// also why nothing here fails when there is none.
func (s *Service) ensureIdentityForAssignedRole(ctx context.Context, accountID int64, role *auth.Role) error {
	provisioning, err := s.schoolIdentity("provision school identity")
	if err != nil {
		return err
	}
	if _, err := provisioning.EnsureSchoolIdentity(ctx, SchoolIdentityInput{
		AccountID:    accountID,
		TenantID:     tenant.FromContext(ctx),
		Role:         RoleFactsOf(role),
		CreatePerson: false,
	}); err != nil {
		return &AuthError{Op: "provision school identity", Err: err}
	}
	return nil
}

// accountHoldsLehrkraftRole reports whether the account already holds the
// Lehrkraft system role at the tenant in ctx. FindByAccountID applies the
// tenant filter to auth.account_roles, so a Lehrkraft assignment at another
// school does not answer for this one.
func (s *Service) accountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error) {
	roles, err := s.repos.Role.FindByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if IsLehrkraftSystemRole(role) {
			return true, nil
		}
	}
	return false, nil
}

// RemoveRoleFromAccount removes a role from an account
func (s *Service) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error {
	var revoked []RevokedSession
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.lockManageableAccount(txCtx, accountID, "remove role"); err != nil {
			return err
		}
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

		sessions, err := s.accountSessions("revoke tokens after role removal")
		if err != nil {
			return err
		}
		tokens, err := sessions.DeleteAccountSessionsWithAudit(txCtx, int64(accountID), "role_changed", "", "")
		if err != nil {
			return &AuthError{Op: "revoke tokens after role removal", Err: err}
		}
		revoked = tokens
		return nil
	})
	if err != nil {
		return err
	}
	if s.sessions != nil {
		s.sessions.QueuePushCleanup(ctx, int64(accountID), revoked, "role_changed")
	}
	return nil
}

// GetAccountRoles retrieves all roles for an account
func (s *Service) GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error) {
	if _, err := s.repos.Account.FindManageableByID(ctx, int64(accountID)); err != nil {
		return nil, &AuthError{Op: "get account roles", Err: ErrAccountNotFound}
	}
	if err := s.ensureOrganizationRBACMembership(ctx, accountID, "get account roles", false); err != nil {
		return nil, err
	}
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

var (
	// ErrRoleNotFound returned when role doesn't exist
	ErrRoleNotFound = errors.New("role not found")

	// ErrSystemRoleImmutable returned when attempting to modify a system role
	ErrSystemRoleImmutable = errors.New("system roles cannot be modified")
)

// RoleOperations manage roles and their assignments.
type RoleOperations interface {
	// Role Management
	CreateRole(ctx context.Context, name, description string, baseRole *string) (*auth.Role, error)
	GetRoleByID(ctx context.Context, id int) (*auth.Role, error)
	ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*auth.Role, error)
	UpdateRole(ctx context.Context, role *auth.Role) error
	DeleteRole(ctx context.Context, id int) error
	ListRoles(ctx context.Context, filters map[string]interface{}) ([]*auth.Role, error)
	AssignRoleToAccount(ctx context.Context, accountID, roleID int) error
	ReplaceAccountRole(ctx context.Context, accountID, roleID int) error
	RemoveRoleFromAccount(ctx context.Context, accountID, roleID int) error
	GetAccountRoles(ctx context.Context, accountID int) ([]*auth.Role, error)
	GetAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	GetAccountEmailsByIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)
	GetAccountAvatarsByIDs(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}
