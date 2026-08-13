package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Operator-led management of which schools an existing account may access
// (issue #1021). The data model has always supported one account holding
// different roles at several schools; what was missing is a controlled way to
// create, change and revoke those mappings without hand-editing the database.
//
// This lives on the operator surface on purpose: granting an account access to
// a school it does not belong to yet is a cross-tenant act. A school admin can
// only ever pull an existing account into their OWN school, which stays with
// the invitation flow and POST /auth/link-to-tenant.

// accessAuditIP is used for auth events triggered by an operator, where the
// tenant-side request IP is not the meaningful actor address.
const accessAuditIP = "0.0.0.0"

// AccountTenantRole is one role an account holds at one school.
type AccountTenantRole struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	IsSystem bool    `json:"is_system"`
	BaseRole *string `json:"base_role,omitempty"`
}

// AccountTenantAccessEntry is one school an account has (or had) access to,
// including the roles it holds there.
type AccountTenantAccessEntry struct {
	authModels.AccountTenantAccessInfo
	Roles []AccountTenantRole `json:"roles"`
}

// GrantAccountTenantAccessRequest carries the optional person data used when
// the account has no person record anywhere yet.
type GrantAccountTenantAccessRequest struct {
	RoleID    int64
	FirstName string
	LastName  string
	Position  string
}

// ListAccountTenantAccess returns every school mapping of one account.
func (s *operatorProvisioningService) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccessEntry, error) {
	var result []AccountTenantAccessEntry
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		if _, err := s.loadAccount(adminCtx, accountID); err != nil {
			return err
		}
		entries, listErr := s.loadAccountTenantAccess(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GrantAccountTenantAccess gives an existing account access to an additional
// school and assigns the requested role there. Reactivating a previously
// revoked mapping goes through the same path, which is why EnsureActive (not
// Create) is used.
func (s *operatorProvisioningService) GrantAccountTenantAccess(
	ctx context.Context,
	accountID, schoolID int64,
	req GrantAccountTenantAccessRequest,
	operatorID int64,
	clientIP net.IP,
) ([]AccountTenantAccessEntry, error) {
	var result []AccountTenantAccessEntry
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		account, err := s.loadAccountForUpdate(adminCtx, accountID)
		if err != nil {
			return err
		}
		school, err := s.loadActiveSchool(adminCtx, schoolID)
		if err != nil {
			return err
		}
		role, err := s.validateAssignableSchoolRole(adminCtx, req.RoleID, schoolID)
		if err != nil {
			return err
		}

		hasAccess, err := s.AccountTenantRepo.ExistsByAccountAndTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		if hasAccess {
			return &ConflictError{Err: fmt.Errorf("account already has access to this school")}
		}
		activeMappings, err := s.AccountTenantRepo.FindActiveByAccountID(adminCtx, accountID)
		if err != nil {
			return err
		}
		// Names for the person record that carries the account at this school.
		// Fall back to a person the account already has at another school so
		// the operator does not have to retype them.
		firstName, lastName := strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName)
		if firstName == "" || lastName == "" {
			existing, findErr := s.activePersonForAccount(adminCtx, accountID)
			if findErr != nil {
				return findErr
			}
			if existing != nil {
				if firstName == "" {
					firstName = existing.FirstName
				}
				if lastName == "" {
					lastName = existing.LastName
				}
			}
		}
		if firstName == "" || lastName == "" {
			return &InvalidDataError{Err: fmt.Errorf("first_name and last_name are required for accounts without a person record")}
		}

		tenantCtx := tenant.WithTenantID(adminCtx, schoolID)

		// Same guard as UpdateAccountTenantRole: a revoke deliberately leaves
		// person/staff/teacher records behind (see RevokeAccountTenantAccess),
		// so re-granting the same school with the Lehrkraft role would revive
		// a live caregiver profile — users.teachers plus its group
		// supervisions — on an account whose JWT only carries class_day
		// permissions. Offboarded (soft-deleted) profiles do not block.
		if authSvc.IsLehrkraftSystemRole(role) {
			hasCaregiverProfile, profErr := s.hasSchoolCaregiverProfile(tenantCtx, accountID)
			if profErr != nil {
				return profErr
			}
			if hasCaregiverProfile {
				return &InvalidDataError{Err: fmt.Errorf("cannot assign the lehrkraft role: the account has a caregiver profile at this school; remove it via staff offboarding first")}
			}
		}

		mapping := &authModels.AccountTenant{AccountID: accountID, TenantID: schoolID}
		if err := s.AccountTenantRepo.EnsureActive(tenantCtx, mapping); err != nil {
			return fmt.Errorf("activate account-tenant mapping: %w", err)
		}

		if err := s.AuthService.AssignRoleToAccount(tenantCtx, int(accountID), int(role.ID)); err != nil {
			return err
		}

		if err := s.ensureSchoolIdentity(tenantCtx, accountID, schoolID, role, firstName, lastName, req.Position, true); err != nil {
			return err
		}
		// A globally inactive account is restored only when the preceding revoke
		// removed its final active school mapping. Accounts that still have an
		// active mapping were disabled deliberately and must stay disabled.
		if len(activeMappings) == 0 && !account.Active {
			if err := s.AuthService.ActivateAccount(adminCtx, int(accountID)); err != nil {
				return fmt.Errorf("reactivate account after restoring school access: %w", err)
			}
		}
		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceAccountTenant, &accountID, clientIP, map[string]any{
			"schoolID": school.ID,
			"email":    account.Email,
			"roleID":   role.ID,
			"roleName": role.Name,
		})
		if err := s.recordAccessAuthEvent(tenantCtx, accountID, schoolID, auditModels.EventTypeTenantAccessGranted, map[string]any{
			"school_id":   school.ID,
			"school_name": school.Name,
			"role":        role.Name,
			"operator_id": operatorID,
		}); err != nil {
			return err
		}

		entries, listErr := s.loadAccountTenantAccess(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateAccountTenantRole changes the role an account holds at one school.
//
// The requested role is assigned and the account's OTHER roles at that school
// are removed, except the ones owned by other features (see
// rolesOwnedByOtherFeatures). Practically this means: switching an account
// between Verwaltung and Betreuung works, while an admin who also does care
// work keeps the caregiver role until it is removed through "Betreuung
// verwalten".
func (s *operatorProvisioningService) UpdateAccountTenantRole(
	ctx context.Context,
	accountID, schoolID, roleID, operatorID int64,
	clientIP net.IP,
) ([]AccountTenantAccessEntry, error) {
	var result []AccountTenantAccessEntry
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		account, err := s.loadAccountForUpdate(adminCtx, accountID)
		if err != nil {
			return err
		}
		school, err := s.loadActiveSchool(adminCtx, schoolID)
		if err != nil {
			return err
		}
		role, err := s.validateAssignableSchoolRole(adminCtx, roleID, schoolID)
		if err != nil {
			return err
		}

		hasAccess, err := s.AccountTenantRepo.ExistsByAccountAndTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		if !hasAccess {
			return &AccountTenantAccessNotFoundError{AccountID: accountID, SchoolID: schoolID}
		}

		current, err := s.rolesForTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}

		tenantCtx := tenant.WithTenantID(adminCtx, schoolID)
		// Server-side mirror of the UI guards (role-management-modal,
		// account-tenant-access-modal): switching an account whose school
		// identity includes a caregiver profile to Lehrkraft would strand
		// users.teachers and its active group supervisions on an account
		// whose JWT only carries class_day permissions. The profile is
		// removed through the school's own staff offboarding, never through
		// a role swap — so a direct endpoint call must be rejected too.
		if authSvc.IsLehrkraftSystemRole(role) {
			hasCaregiverProfile, profErr := s.hasSchoolCaregiverProfile(tenantCtx, accountID)
			if profErr != nil {
				return profErr
			}
			if hasCaregiverProfile {
				return &InvalidDataError{Err: fmt.Errorf("cannot assign the lehrkraft role: the account has a caregiver profile at this school; remove it via staff offboarding first")}
			}
		}
		// A caregiver requires a local person, staff and teacher record; a
		// Lehrkraft (#1772) requires person and staff (no teacher profile) so
		// the school can assign classes under Mitarbeiter. The role-change
		// endpoint has no identity fields, so copy the deterministically
		// selected active identity from another school (or the local one).
		// Without one, reject the change before assigning the role.
		firstName, lastName := "", ""
		if isSystemCaregiverRole(role) || authSvc.IsLehrkraftSystemRole(role) {
			existing, findErr := s.activePersonForAccount(adminCtx, accountID)
			if findErr != nil {
				return findErr
			}
			if existing == nil {
				return &InvalidDataError{Err: fmt.Errorf("a person record is required before assigning this role")}
			}
			firstName, lastName = existing.FirstName, existing.LastName
			if err := s.ensureSchoolIdentity(tenantCtx, accountID, schoolID, role, firstName, lastName, "", true); err != nil {
				return err
			}
		}
		if err := s.AuthService.AssignRoleToAccount(tenantCtx, int(accountID), int(role.ID)); err != nil {
			return err
		}

		removed := make([]string, 0, len(current))
		for _, existing := range current {
			if existing.ID == role.ID || roleOwnedByOtherFeature(existing) {
				continue
			}
			if err := s.AccountRoleRepo.DeleteByAccountRoleAndTenant(adminCtx, accountID, existing.ID, schoolID); err != nil {
				return err
			}
			removed = append(removed, existing.Name)
		}
		if err := s.AuthService.RevokeAllTokensWithReason(adminCtx, int(accountID), "role_changed"); err != nil {
			return fmt.Errorf("revoke tokens after role change: %w", err)
		}

		s.logAction(adminCtx, operatorID, platform.ActionUpdate, platform.ResourceAccountTenant, &accountID, clientIP, map[string]any{
			"schoolID":     school.ID,
			"email":        account.Email,
			"roleID":       role.ID,
			"roleName":     role.Name,
			"removedRoles": removed,
		})
		if err := s.recordAccessAuthEvent(tenantCtx, accountID, schoolID, auditModels.EventTypeTenantRoleChanged, map[string]any{
			"school_id":     school.ID,
			"school_name":   school.Name,
			"role":          role.Name,
			"removed_roles": removed,
			"operator_id":   operatorID,
		}); err != nil {
			return err
		}

		entries, listErr := s.loadAccountTenantAccess(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RevokeAccountTenantAccess withdraws an account's access to one school: the
// mapping goes inactive, the tenant-scoped role assignments are removed and the
// account itself is deactivated once no active mapping remains.
//
// Person and staff records at that school are deliberately left untouched —
// they carry the name for historical attendance and work-session rows. Removing
// them is the school's own "Personal löschen" (staff offboarding), which also
// checks active supervisions.
func (s *operatorProvisioningService) RevokeAccountTenantAccess(
	ctx context.Context,
	accountID, schoolID, operatorID int64,
	clientIP net.IP,
) ([]AccountTenantAccessEntry, error) {
	var result []AccountTenantAccessEntry
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		account, err := s.loadAccountForUpdate(adminCtx, accountID)
		if err != nil {
			return err
		}
		school, err := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if err != nil || school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}

		hasAccess, err := s.AccountTenantRepo.ExistsByAccountAndTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		if !hasAccess {
			return &AccountTenantAccessNotFoundError{AccountID: accountID, SchoolID: schoolID}
		}

		current, err := s.rolesForTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		for _, existing := range current {
			if roleBlocksAccessRevocation(existing) {
				return &InvalidDataError{Err: fmt.Errorf("school access with role %q must be removed through its dedicated flow", existing.Name)}
			}
			if err := s.AccountRoleRepo.DeleteByAccountRoleAndTenant(adminCtx, accountID, existing.ID, schoolID); err != nil {
				return err
			}
		}
		if s.AccountPermissionRepo == nil {
			return fmt.Errorf("account permission repository is not configured")
		}
		if _, err := s.AccountPermissionRepo.DeleteByAccountID(tenant.WithTenantID(adminCtx, schoolID), accountID); err != nil {
			return fmt.Errorf("delete direct account permissions: %w", err)
		}
		if err := s.AuthService.RevokeAllTokensWithReason(adminCtx, int(accountID), "tenant_access_revoked"); err != nil {
			return fmt.Errorf("revoke tokens after access revocation: %w", err)
		}

		if err := s.AccountTenantRepo.Deactivate(adminCtx, accountID, schoolID); err != nil {
			return err
		}

		remaining, err := s.AccountTenantRepo.FindActiveByAccountID(adminCtx, accountID)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			if err := s.AuthService.DeactivateAccount(adminCtx, int(accountID)); err != nil {
				return err
			}
		}

		s.logAction(adminCtx, operatorID, platform.ActionDelete, platform.ResourceAccountTenant, &accountID, clientIP, map[string]any{
			"schoolID":           school.ID,
			"email":              account.Email,
			"accountDeactivated": len(remaining) == 0,
		})
		if err := s.recordAccessAuthEvent(tenant.WithTenantID(adminCtx, schoolID), accountID, schoolID, auditModels.EventTypeTenantAccessRevoked, map[string]any{
			"school_id":           school.ID,
			"school_name":         school.Name,
			"account_deactivated": len(remaining) == 0,
			"operator_id":         operatorID,
		}); err != nil {
			return err
		}

		entries, listErr := s.loadAccountTenantAccess(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// loadAccountTenantAccess merges the mapping rows with the roles the account
// holds at each school.
func (s *operatorProvisioningService) loadAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccessEntry, error) {
	rows, err := s.AccountTenantRepo.ListTenantAccessByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	rolesByTenant, err := s.rolesByTenant(ctx, accountID)
	if err != nil {
		return nil, err
	}

	entries := make([]AccountTenantAccessEntry, 0, len(rows))
	for _, row := range rows {
		roles := rolesByTenant[row.TenantID]
		if roles == nil {
			roles = []AccountTenantRole{}
		}
		entries = append(entries, AccountTenantAccessEntry{AccountTenantAccessInfo: row, Roles: roles})
	}
	return entries, nil
}

// rolesByTenant groups an account's role assignments by school. The context
// must NOT be tenant-scoped, otherwise the repository filters to one school.
func (s *operatorProvisioningService) rolesByTenant(ctx context.Context, accountID int64) (map[int64][]AccountTenantRole, error) {
	assignments, err := s.AccountRoleRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	grouped := make(map[int64][]AccountTenantRole, len(assignments))
	for _, assignment := range assignments {
		if assignment == nil {
			continue
		}
		name := ""
		isSystem := false
		var baseRole *string
		if assignment.Role != nil {
			name = assignment.Role.Name
			isSystem = assignment.Role.IsSystem
			baseRole = assignment.Role.BaseRole
		}
		tenantID := assignment.GetTenantID()
		grouped[tenantID] = append(grouped[tenantID], AccountTenantRole{ID: assignment.RoleID, Name: name, IsSystem: isSystem, BaseRole: baseRole})
	}
	return grouped, nil
}

func (s *operatorProvisioningService) rolesForTenant(ctx context.Context, accountID, schoolID int64) ([]AccountTenantRole, error) {
	grouped, err := s.rolesByTenant(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return grouped[schoolID], nil
}

// loadAccount resolves the account or returns AccountNotFoundError.
func (s *operatorProvisioningService) loadAccount(ctx context.Context, accountID int64) (*authModels.Account, error) {
	if accountID <= 0 {
		return nil, &AccountNotFoundError{AccountID: accountID}
	}
	account, err := s.AccountRepo.FindByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, &AccountNotFoundError{AccountID: accountID}
	}
	return account, nil
}

// loadAccountForUpdate serializes account-scoped access changes. Grants,
// revocations and role changes all derive their mapping and active-account
// state from this locked row, avoiding competing decisions in parallel admin
// requests.
func (s *operatorProvisioningService) loadAccountForUpdate(ctx context.Context, accountID int64) (*authModels.Account, error) {
	if accountID <= 0 {
		return nil, &AccountNotFoundError{AccountID: accountID}
	}
	account, err := s.AccountRepo.FindByIDForUpdate(ctx, accountID)
	if err != nil || account == nil {
		return nil, &AccountNotFoundError{AccountID: accountID}
	}
	return account, nil
}

// validateAssignableSchoolRole applies the shared role-assignment policy and
// maps its sentinels onto the operator error shape.
func (s *operatorProvisioningService) validateAssignableSchoolRole(ctx context.Context, roleID, schoolID int64) (*authModels.Role, error) {
	role, err := authSvc.ValidateAssignableSchoolRole(ctx, s.RoleRepo, roleID, schoolID)
	if err != nil {
		return nil, &InvalidDataError{Err: err}
	}
	return role, nil
}

// ListAssignableSchoolRoles returns the system and target-school roles that
// may be selected by the operator. The same policy as mutation paths is
// applied so the UI cannot offer roles the backend would reject.
func (s *operatorProvisioningService) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]*authModels.Role, error) {
	var result []*authModels.Role
	err := tenant.WithAdminTxOrDirect(ctx, s.adminDB(), func(adminCtx context.Context) error {
		if _, err := s.loadActiveSchool(adminCtx, schoolID); err != nil {
			return err
		}
		// Query via the admin context: the tenant-role RLS policy can hide
		// platform roles (tenant_id NULL), but the explicit policy below still
		// admits only system roles and custom roles of this school.
		roles, err := s.RoleRepo.List(adminCtx, nil)
		if err != nil {
			return err
		}
		result = make([]*authModels.Role, 0, len(roles))
		for _, role := range roles {
			if role == nil {
				continue
			}
			if _, err := authSvc.ValidateAssignableSchoolRole(adminCtx, s.RoleRepo, role.ID, schoolID); err == nil {
				result = append(result, role)
			}
		}
		return nil
	})
	return result, err
}

// activePersonForAccount chooses a non-deleted source identity deterministically
// when an account is linked to people at more than one school.
func (s *operatorProvisioningService) activePersonForAccount(ctx context.Context, accountID int64) (*userModels.Person, error) {
	persons, err := s.PersonRepo.List(ctx, map[string]interface{}{"account_id": accountID})
	if err != nil {
		return nil, err
	}
	var selected *userModels.Person
	for _, person := range persons {
		if person == nil || person.DeletedAt != nil {
			continue
		}
		if selected == nil || person.TenantID < selected.TenantID || (person.TenantID == selected.TenantID && person.ID < selected.ID) {
			selected = person
		}
	}
	return selected, nil
}

// hasSchoolCaregiverProfile reports whether the account's identity at the
// school in ctx includes a live caregiver profile (users.teachers). Delegates
// to the shared walk so this guard cannot drift from the tenant RBAC path.
func (s *operatorProvisioningService) hasSchoolCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return authSvc.HasLiveCaregiverProfile(ctx, s.PersonRepo, s.StaffRepo, s.TeacherRepo, accountID)
}

// ensureSchoolIdentity creates the person, staff and (for caregiver roles)
// teacher rows the account needs to be usable at a school. Every step is
// idempotent so a re-grant after a revoke reuses the existing records.
//
// firstName/lastName are only consulted when no person exists yet; the caller
// resolves them beforehand.
func (s *operatorProvisioningService) ensureSchoolIdentity(
	ctx context.Context,
	accountID, schoolID int64,
	role *authModels.Role,
	firstName, lastName, position string,
	createPerson bool,
) error {
	baseRole := roleBaseName(role)
	if baseRole == "" {
		return nil
	}
	if strings.EqualFold(baseRole, authModels.BaseRoleGuardian) {
		return nil
	}

	person, err := s.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	if person != nil && person.DeletedAt != nil {
		person = nil
	}
	if person == nil {
		if !createPerson {
			return nil
		}
		if firstName == "" || lastName == "" {
			return &InvalidDataError{Err: fmt.Errorf("first_name and last_name are required for accounts without a person record")}
		}
		person = &userModels.Person{FirstName: firstName, LastName: lastName}
		person.SetTenantID(schoolID)
		if err := s.PersonRepo.Create(ctx, person); err != nil {
			return fmt.Errorf("create person: %w", err)
		}
		if err := s.PersonRepo.LinkToAccount(ctx, person.ID, accountID); err != nil {
			return fmt.Errorf("link person to account: %w", err)
		}
	}

	// StaffRepo reports "no staff" as sql.ErrNoRows, not as a nil result.
	staff, err := s.StaffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		staff = nil
	}
	if staff != nil && staff.DeletedAt != nil {
		staff = nil
	}
	if staff == nil {
		staff = &userModels.Staff{PersonID: person.ID}
		staff.SetTenantID(schoolID)
		if err := s.StaffRepo.Create(ctx, staff); err != nil {
			return fmt.Errorf("create staff: %w", err)
		}
	}

	if !isSystemCaregiverRole(role) {
		return nil
	}

	teacher, err := s.TeacherRepo.FindByStaffID(ctx, staff.ID)
	if err != nil {
		return err
	}
	if teacher != nil {
		if teacher.DeletedAt != nil {
			teacher = nil
		} else {
			return nil
		}
	}
	teacher = &userModels.Teacher{StaffID: staff.ID}
	teacher.SetTenantID(schoolID)
	if position != "" {
		teacher.Role = position
	}
	if err := s.TeacherRepo.Create(ctx, teacher); err != nil {
		return fmt.Errorf("create teacher: %w", err)
	}
	return nil
}

func roleBaseName(role *authModels.Role) string {
	if role == nil {
		return ""
	}
	if !role.IsSystem {
		if role.BaseRole == nil || strings.EqualFold(*role.BaseRole, authModels.BaseRoleGuardian) {
			return ""
		}
		return *role.BaseRole
	}
	return role.Name
}

func isSystemCaregiverRole(role *authModels.Role) bool {
	return role != nil && role.IsSystem && shouldCreateTeacher(role.Name)
}

func roleOwnedByOtherFeature(role AccountTenantRole) bool {
	if role.BaseRole != nil && strings.EqualFold(strings.TrimSpace(*role.BaseRole), authModels.BaseRoleGuardian) {
		return true
	}
	if !role.IsSystem {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(role.Name)) {
	case authModels.BaseRoleGuardian, authModels.BaseRoleUser, "teacher":
		return true
	default:
		return false
	}
}

func roleBlocksAccessRevocation(role AccountTenantRole) bool {
	if role.BaseRole != nil && strings.EqualFold(strings.TrimSpace(*role.BaseRole), authModels.BaseRoleGuardian) {
		return true
	}
	return role.IsSystem && (strings.EqualFold(role.Name, authModels.BaseRoleGuardian) || strings.EqualFold(role.Name, authModels.BaseRoleUser) || strings.EqualFold(role.Name, "teacher"))
}

// recordAccessAuthEvent writes the tenant-visible audit trail. The operator
// audit log records the same change on the platform side; this one exists so
// the affected school can see access changes to its own data. It is part of
// the surrounding transaction, so failures are returned rather than swallowed.
func (s *operatorProvisioningService) recordAccessAuthEvent(
	ctx context.Context,
	accountID, schoolID int64,
	eventType string,
	metadata map[string]any,
) error {
	if s.AuthEventRepo == nil {
		return nil
	}
	event := auditModels.NewAuthEvent(accountID, eventType, true, accessAuditIP)
	event.SetTenantID(schoolID)
	for key, value := range metadata {
		event.SetMetadata(key, value)
	}
	if err := s.AuthEventRepo.Create(ctx, event); err != nil {
		return fmt.Errorf("record tenant access auth event: %w", err)
	}
	return nil
}
