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

// rolesOwnedByOtherFeatures are never removed implicitly when the role at a
// school is changed:
//
//   - guardian is granted and revoked by the guardian invitation flow; dropping
//     it here would silently cut a parent off from the parents portal.
//   - user carries the caregiver capability, whose removal has to run through
//     CaregiverCapabilityService so active supervisions and group assignments
//     are checked first.
var rolesOwnedByOtherFeatures = map[string]bool{
	authModels.BaseRoleGuardian: true,
	authModels.BaseRoleUser:     true,
}

// AccountTenantRole is one role an account holds at one school.
type AccountTenantRole struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
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
		account, err := s.loadAccount(adminCtx, accountID)
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

		// Names for the person record that carries the account at this school.
		// Fall back to a person the account already has at another school so
		// the operator does not have to retype them.
		firstName, lastName := strings.TrimSpace(req.FirstName), strings.TrimSpace(req.LastName)
		if firstName == "" || lastName == "" {
			existing, findErr := s.PersonRepo.FindByAccountID(adminCtx, accountID)
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

		// An account without any school mapping is deactivated on revoke; the
		// grant has to undo that or the person still cannot log in.
		if !account.Active {
			if err := s.AuthService.ActivateAccount(adminCtx, int(accountID)); err != nil {
				return err
			}
		}

		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceAccountTenant, &accountID, clientIP, map[string]any{
			"schoolID": school.ID,
			"email":    account.Email,
			"roleID":   role.ID,
			"roleName": role.Name,
		})
		s.recordAccessAuthEvent(tenantCtx, accountID, schoolID, auditModels.EventTypeTenantAccessGranted, map[string]any{
			"school_id":   school.ID,
			"school_name": school.Name,
			"role":        role.Name,
			"operator_id": operatorID,
		})

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
		account, err := s.loadAccount(adminCtx, accountID)
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
		if err := s.AuthService.AssignRoleToAccount(tenantCtx, int(accountID), int(role.ID)); err != nil {
			return err
		}

		removed := make([]string, 0, len(current))
		for _, existing := range current {
			if existing.ID == role.ID || rolesOwnedByOtherFeatures[strings.ToLower(existing.Name)] {
				continue
			}
			if err := s.AccountRoleRepo.DeleteByAccountRoleAndTenant(adminCtx, accountID, existing.ID, schoolID); err != nil {
				return err
			}
			removed = append(removed, existing.Name)
		}

		// A role change can turn a non-caregiver into one; make sure the
		// school-side records exist either way.
		if err := s.ensureSchoolIdentity(tenantCtx, accountID, schoolID, role, "", "", "", false); err != nil {
			return err
		}

		s.logAction(adminCtx, operatorID, platform.ActionUpdate, platform.ResourceAccountTenant, &accountID, clientIP, map[string]any{
			"schoolID":     school.ID,
			"email":        account.Email,
			"roleID":       role.ID,
			"roleName":     role.Name,
			"removedRoles": removed,
		})
		s.recordAccessAuthEvent(tenantCtx, accountID, schoolID, auditModels.EventTypeTenantRoleChanged, map[string]any{
			"school_id":     school.ID,
			"school_name":   school.Name,
			"role":          role.Name,
			"removed_roles": removed,
			"operator_id":   operatorID,
		})

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
		account, err := s.loadAccount(adminCtx, accountID)
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
			if err := s.AccountRoleRepo.DeleteByAccountRoleAndTenant(adminCtx, accountID, existing.ID, schoolID); err != nil {
				return err
			}
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
		s.recordAccessAuthEvent(tenant.WithTenantID(adminCtx, schoolID), accountID, schoolID, auditModels.EventTypeTenantAccessRevoked, map[string]any{
			"school_id":           school.ID,
			"school_name":         school.Name,
			"account_deactivated": len(remaining) == 0,
			"operator_id":         operatorID,
		})

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
		if assignment.Role != nil {
			name = assignment.Role.Name
		}
		tenantID := assignment.GetTenantID()
		grouped[tenantID] = append(grouped[tenantID], AccountTenantRole{ID: assignment.RoleID, Name: name})
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

// validateAssignableSchoolRole applies the shared role-assignment policy and
// maps its sentinels onto the operator error shape.
func (s *operatorProvisioningService) validateAssignableSchoolRole(ctx context.Context, roleID, schoolID int64) (*authModels.Role, error) {
	role, err := authSvc.ValidateAssignableSchoolRole(ctx, s.RoleRepo, roleID, schoolID)
	if err != nil {
		return nil, &InvalidDataError{Err: err}
	}
	return role, nil
}

// ensureSchoolIdentity creates the person, staff and (for caregiver roles)
// teacher rows the account needs to be usable at a school. Every step is
// idempotent so a re-grant after a revoke reuses the existing records.
//
// firstName/lastName are only consulted when no person exists yet; the caller
// resolves them beforehand. createPerson is false on the role-change path: an
// account without a person at that school (a guardian-only mapping, or one
// created by the older link-to-tenant flow) must not be turned into staff as a
// side effect of switching its role.
func (s *operatorProvisioningService) ensureSchoolIdentity(
	ctx context.Context,
	accountID, schoolID int64,
	role *authModels.Role,
	firstName, lastName, position string,
	createPerson bool,
) error {
	if !role.IsSystem {
		return nil
	}

	person, err := s.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return err
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
	if staff == nil {
		staff = &userModels.Staff{PersonID: person.ID}
		staff.SetTenantID(schoolID)
		if err := s.StaffRepo.Create(ctx, staff); err != nil {
			return fmt.Errorf("create staff: %w", err)
		}
	}

	if !shouldCreateTeacher(role.Name) {
		return nil
	}

	teacher, err := s.TeacherRepo.FindByStaffID(ctx, staff.ID)
	if err != nil {
		return err
	}
	if teacher != nil {
		return nil
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

// recordAccessAuthEvent writes the tenant-visible audit trail. The operator
// audit log records the same change on the platform side; this one exists so
// the affected school can see access changes to its own data. Failures are
// logged, never fatal — the access change itself already happened.
func (s *operatorProvisioningService) recordAccessAuthEvent(
	ctx context.Context,
	accountID, schoolID int64,
	eventType string,
	metadata map[string]any,
) {
	if s.AuthEventRepo == nil {
		return
	}
	event := auditModels.NewAuthEvent(accountID, eventType, true, accessAuditIP)
	event.SetTenantID(schoolID)
	for key, value := range metadata {
		event.SetMetadata(key, value)
	}
	if err := s.AuthEventRepo.Create(ctx, event); err != nil {
		s.getLogger().Error("failed to record tenant access auth event",
			"error", err,
			"event_type", eventType,
			"tenant_id", schoolID,
		)
	}
}
