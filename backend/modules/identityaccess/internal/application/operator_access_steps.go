package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type operatorAccessFacts struct {
	account         domain.ManagedAccount
	school          domain.School
	role            domain.RoleFact
	current         []domain.AccountTenantRole
	activeTenantIDs []int64
}

func (s *OperatorAccountAccess) grantAccessFacts(adminCtx context.Context, request domain.GrantAccountTenantAccess) (facts operatorAccessFacts, err error) {
	account, err := s.loadAccount(adminCtx, request.AccountID, true)
	if err != nil {
		return facts, err
	}
	school, err := s.loadActiveSchool(adminCtx, request.SchoolID)
	if err != nil {
		return facts, err
	}
	role, err := s.validateAssignableSchoolRole(adminCtx, request.RoleID, request.SchoolID)
	if err != nil {
		return facts, err
	}
	hasAccess, _, err := s.store.HasActiveAccountTenant(adminCtx, request.AccountID, request.SchoolID)
	if err != nil {
		return facts, err
	}
	if hasAccess {
		return facts, domain.ErrAccountTenantAccessExists
	}
	// A revoke removes the school's roles, but a role assignment left
	// without its mapping must not let the grant swap a Lehrkraft role
	// away, as the retained role assignment refused.
	current, err := s.rolesAtTenant(adminCtx, request.AccountID, request.SchoolID)
	if err != nil {
		return facts, err
	}
	if err := s.refuseLeavingLehrkraft(current, role); err != nil {
		return facts, err
	}
	activeTenantIDs, _, err := s.login.ListActiveTenantIDs(adminCtx, request.AccountID)
	if err != nil {
		return facts, err
	}
	facts = operatorAccessFacts{account: account, school: school, role: role, activeTenantIDs: activeTenantIDs}
	return facts, nil
}

func (s *OperatorAccountAccess) grantSchoolIdentity(tenantCtx, adminCtx context.Context, request domain.GrantAccountTenantAccess, account domain.ManagedAccount, role domain.RoleFact, activeTenantIDs []int64) error {
	// Names for the person record that carries the account at this
	// school, resolved before anything is written and only when one is
	// actually needed: a re-grant after a revoke finds the person still
	// there and needs no name at all.
	firstName, lastName, err := s.identityNamesForSchool(tenantCtx, adminCtx, request.AccountID,
		strings.TrimSpace(request.FirstName), strings.TrimSpace(request.LastName))
	if err != nil {
		return err
	}
	// A revoke deliberately leaves person, staff and teacher records
	// behind, so re-granting the same school with the Lehrkraft role
	// would revive a live caregiver profile on an account whose JWT
	// only carries class_day permissions.
	if err := s.refuseLehrkraftOverCaregiverProfile(tenantCtx, request.AccountID, role); err != nil {
		return err
	}
	if _, err := s.store.EnsureActiveTenantMapping(tenantCtx, request.AccountID, request.SchoolID); err != nil {
		return fmt.Errorf("activate account-tenant mapping: %w", err)
	}
	if err := s.assignRole(tenantCtx, request.AccountID, request.SchoolID, role); err != nil {
		return err
	}
	if err := s.identities.EnsureSchoolIdentity(tenantCtx, domain.SchoolIdentityRequest{
		AccountID: request.AccountID, TenantID: request.SchoolID, Role: role,
		FirstName: firstName, LastName: lastName, Position: request.Position, CreatePerson: true,
	}); err != nil {
		return err
	}
	// A globally inactive account is restored only when the preceding
	// revoke removed its final active school mapping. Accounts that
	// still have an active mapping were disabled deliberately and must
	// stay disabled.
	if len(activeTenantIDs) == 0 && !account.Active {
		if err := s.activateAccount(adminCtx, request.AccountID); err != nil {
			return fmt.Errorf("reactivate account after restoring school access: %w", err)
		}
	}
	return nil
}

func (s *OperatorAccountAccess) roleChangeFacts(adminCtx context.Context, accountID, schoolID, roleID int64) (facts operatorAccessFacts, err error) {
	account, err := s.loadAccount(adminCtx, accountID, true)
	if err != nil {
		return facts, err
	}
	school, err := s.loadActiveSchool(adminCtx, schoolID)
	if err != nil {
		return facts, err
	}
	role, err := s.validateAssignableSchoolRole(adminCtx, roleID, schoolID)
	if err != nil {
		return facts, err
	}
	hasAccess, _, err := s.store.HasActiveAccountTenant(adminCtx, accountID, schoolID)
	if err != nil {
		return facts, err
	}
	if !hasAccess {
		return facts, domain.ErrAccountTenantAccessNotFound
	}
	current, err := s.rolesAtTenant(adminCtx, accountID, schoolID)
	if err != nil {
		return facts, err
	}
	facts = operatorAccessFacts{account: account, school: school, role: role, current: current}
	return facts, nil
}

func (s *OperatorAccountAccess) ensureRoleChangeIdentity(tenantCtx, adminCtx context.Context, accountID, schoolID int64, role domain.RoleFact) error {
	// Every staff-tier role requires a local person and staff record, and
	// caregiver-tier roles additionally a teacher profile (#2222). The
	// role-change endpoint carries no identity fields, so a school that
	// has no person for this account yet has to borrow the name.
	if s.policy.RoleNeedsStaffRecord(role) {
		firstName, lastName, err := s.identityNamesForSchool(tenantCtx, adminCtx, accountID, "", "")
		if err != nil {
			return err
		}
		if err := s.identities.EnsureSchoolIdentity(tenantCtx, domain.SchoolIdentityRequest{
			AccountID: accountID, TenantID: schoolID, Role: role, FirstName: firstName, LastName: lastName, CreatePerson: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *OperatorAccountAccess) removeReplacedRoles(adminCtx context.Context, accountID, schoolID int64, role domain.RoleFact, current []domain.AccountTenantRole) ([]string, error) {
	removed := make([]string, 0, len(current))
	for _, existing := range current {
		if existing.ID == role.ID || domain.RoleOwnedByOtherFeature(existing) {
			continue
		}
		if _, err := s.access.RemoveAccountRole(adminCtx, accountID, existing.ID, schoolID); err != nil {
			return nil, err
		}
		removed = append(removed, existing.Name)
	}
	return removed, nil
}

func (s *OperatorAccountAccess) removeRevokedRoles(adminCtx context.Context, accountID, schoolID int64, current []domain.AccountTenantRole) error {
	for _, existing := range current {
		if domain.RoleBlocksAccessRevocation(existing) {
			return domain.InvalidInput(fmt.Sprintf("school access with role %q must be removed through its dedicated flow", existing.Name))
		}
		if _, err := s.access.RemoveAccountRole(adminCtx, accountID, existing.ID, schoolID); err != nil {
			return err
		}
	}
	return nil
}

func tenantAccessEntries(mappings []domain.TenantMapping, schoolByID map[int64]domain.School) ([]domain.AccountTenantAccess, []int64, error) {
	entries := make([]domain.AccountTenantAccess, 0, len(mappings))
	organizationIDs := make([]int64, 0, len(mappings))
	seenOrganizations := make(map[int64]struct{}, len(mappings))
	for _, mapping := range mappings {
		school, found := schoolByID[mapping.TenantID]
		if !found {
			return nil, nil, fmt.Errorf("school %d missing for account access", mapping.TenantID)
		}
		if school.Deleted {
			continue
		}
		entries = append(entries, domain.AccountTenantAccess{
			TenantID: mapping.TenantID, SchoolName: school.Name, SchoolSlug: school.Slug, SchoolActive: school.Active,
			OrganizationID: school.OrganizationID, Status: mapping.Status, ActivatedAt: mapping.ActivatedAt, DeactivatedAt: mapping.DeactivatedAt,
			Roles: []domain.AccountTenantRole{},
		})
		if _, seen := seenOrganizations[school.OrganizationID]; !seen {
			seenOrganizations[school.OrganizationID] = struct{}{}
			organizationIDs = append(organizationIDs, school.OrganizationID)
		}
	}
	domain.SortTenantAccessBySchoolName(entries)
	return entries, organizationIDs, nil
}
