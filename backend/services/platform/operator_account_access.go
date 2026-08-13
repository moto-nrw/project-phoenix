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
		tenantCtx := tenant.WithTenantID(adminCtx, schoolID)

		// Names for the person record that carries the account at this school.
		// Resolved before anything is written, and only when one is actually
		// needed — a re-grant after a revoke finds the person still there and
		// needs no name at all (see identityNamesForSchool).
		firstName, lastName, err := s.identityNamesForSchool(
			tenantCtx, adminCtx, accountID,
			strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName))
		if err != nil {
			return err
		}

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
		// Every staff-tier role requires a local person and staff record, and
		// caregiver-tier roles additionally a teacher profile (#2222) — a role
		// that puts someone on the payroll screens but not in users.staff is
		// the half-state this whole path exists to avoid. The role-change
		// endpoint carries no identity fields, so a school that has no person
		// for this account yet has to borrow the name.
		if authSvc.RoleNeedsStaffRecord(role) {
			firstName, lastName, nameErr := s.identityNamesForSchool(tenantCtx, adminCtx, accountID, "", "")
			if nameErr != nil {
				return nameErr
			}
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
		if err := s.AuthService.RevokeAllTokens(adminCtx, int(accountID)); err != nil {
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
		if err := s.AuthService.RevokeAllTokens(adminCtx, int(accountID)); err != nil {
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

// activePersonForAccount returns the identity the account already carries
// elsewhere, but only when there is one answer to return.
//
// What comes back becomes a person — and therefore a staff member — at the
// target school, so this is not a convenience lookup. Two restrictions follow,
// the same two migration 1.15.284 applies in SQL when it repairs the identities
// this path used to leave broken:
//
//   - A child's record is never the account holder's identity. Copying its name
//     into a new staff person files a child as personnel at another school, and
//     afterwards nothing tells that row apart from a legitimate one.
//   - Only a name every candidate agrees on. Two different names is an ambiguity
//     this code is no more entitled to resolve than the login is — taking the
//     lowest-numbered school's version is a coin toss with someone's name.
//
// Unlike the migration this does NOT require the school to still be actively
// mapped. The migration runs unattended and reproduces what the login shows, so
// it only trusts the schools an account currently belongs to. Here the person
// carrying this account's id is the same human whether or not the mapping was
// since revoked — a revoked mapping deliberately keeps the person row — and
// there is an operator present to correct a name that has gone stale.
//
// Returns (nil, nil) when there is no such identity, whether because none exists
// or because the candidates disagree. Callers ask the operator for names or
// refuse; neither may guess.
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
		firstName, lastName := strings.TrimSpace(person.FirstName), strings.TrimSpace(person.LastName)
		if firstName == "" || lastName == "" {
			continue
		}
		isStudent, studentErr := s.personIsStudent(ctx, person.ID)
		if studentErr != nil {
			return nil, studentErr
		}
		if isStudent {
			continue
		}

		if selected == nil {
			selected = person
			continue
		}
		// selected's name never changes below, so any second spelling anywhere in
		// the set is caught here regardless of iteration order.
		if firstName != strings.TrimSpace(selected.FirstName) || lastName != strings.TrimSpace(selected.LastName) {
			return nil, nil
		}
		// Same name — pick a stable representative so the outcome does not depend
		// on row order.
		if person.TenantID < selected.TenantID || (person.TenantID == selected.TenantID && person.ID < selected.ID) {
			selected = person
		}
	}
	return selected, nil
}

// identityNamesForSchool resolves the name a new person at this school would
// carry. Shared by the two operator paths that provision an identity: granting
// access, which may bring names of its own, and a role change, which never
// does.
//
// Three questions, and the order is the point:
//
//  1. Did the caller supply both names? Then they win and nothing is looked up.
//  2. Does the account already have a live person at THIS school? Then no name
//     is needed and none may be resolved. EnsureSchoolIdentity completes the
//     chain on that person rather than inventing a second one, which the
//     partial unique index on (tenant_id, account_id) refuses anyway — so the
//     empty strings returned here are never read. Asking the other schools
//     first is what made a re-grant after a revoke fail: the revoke
//     deliberately keeps person, staff and teacher, so the identity is already
//     sitting there complete, and a differently spelled name at some other
//     school would refuse a request that needed no name in the first place.
//  3. Only then borrow from another of the account's schools, and only an
//     unambiguous non-student identity will do (see activePersonForAccount).
//     Without one the request is refused rather than completed halfway or with
//     a guessed name.
func (s *operatorProvisioningService) identityNamesForSchool(
	tenantCtx, adminCtx context.Context,
	accountID int64,
	firstName, lastName string,
) (string, string, error) {
	if firstName != "" && lastName != "" {
		return firstName, lastName, nil
	}

	local, err := s.PersonRepo.FindByAccountID(tenantCtx, accountID)
	if err != nil {
		return "", "", err
	}
	if local != nil && local.DeletedAt == nil {
		return "", "", nil
	}

	existing, err := s.activePersonForAccount(adminCtx, accountID)
	if err != nil {
		return "", "", err
	}
	if existing != nil {
		if firstName == "" {
			firstName = existing.FirstName
		}
		if lastName == "" {
			lastName = existing.LastName
		}
	}
	if firstName == "" || lastName == "" {
		return "", "", &InvalidDataError{Err: fmt.Errorf(
			"first_name and last_name are required: the account has no person record at this school, and none of its other schools provides an unambiguous name to use")}
	}
	return firstName, lastName, nil
}

// personIsStudent reports whether the person record belongs to a child.
//
// The repository reports "no student" as sql.ErrNoRows wrapped in a
// DatabaseError, the same shape authSvc.EnsureSchoolIdentity handles.
func (s *operatorProvisioningService) personIsStudent(ctx context.Context, personID int64) (bool, error) {
	student, err := s.StudentRepo.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
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
	_, err := authSvc.EnsureSchoolIdentity(ctx, authSvc.SchoolIdentityRepos{
		Persons:  s.PersonRepo,
		Staff:    s.StaffRepo,
		Teachers: s.TeacherRepo,
		Students: s.StudentRepo,
	}, authSvc.SchoolIdentityInput{
		AccountID:    accountID,
		TenantID:     schoolID,
		Role:         role,
		FirstName:    firstName,
		LastName:     lastName,
		Position:     position,
		CreatePerson: createPerson,
	})
	if errors.Is(err, authSvc.ErrSchoolIdentityNamesRequired) ||
		errors.Is(err, authSvc.ErrSchoolIdentityPersonIsStudent) {
		return &InvalidDataError{Err: err}
	}
	return err
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
