package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// OperatorAccountAccess is the operator-led management of which schools an
// existing account may access (#3252, issue #1021). Granting an account
// access to a school it does not belong to yet is a cross-tenant act, so it
// lives on the operator surface; a school admin can only pull an existing
// account into their own school through the invitation flow.
//
// The mapping, role and permission writes are the module's own; the schools
// and organisations, the person and staff identity chain, the retained role
// rules and the ledgers arrive through consumer-owned ports. Every command
// runs in one administrative transaction and revokes the affected sessions
// through the account-session flows.
type OperatorAccountAccess struct {
	store         ports.Store
	login         ports.AccountLoginStore
	access        ports.AccountAccessStore
	sessions      *AccountAuthentication
	schools       ports.SchoolDirectory
	organizations ports.OrganizationDirectory
	identities    ports.SchoolIdentityProvisioner
	policy        ports.SchoolRolePolicy
	audit         ports.AuthAudit
	operatorAudit ports.OperatorAudit
	runtime       ports.Runtime
	logger        *slog.Logger
}

// OperatorAccountAccessDependencies are the ports the access flows consume.
type OperatorAccountAccessDependencies struct {
	Store         ports.Store
	Login         ports.AccountLoginStore
	Access        ports.AccountAccessStore
	Sessions      *AccountAuthentication
	Schools       ports.SchoolDirectory
	Organizations ports.OrganizationDirectory
	Identities    ports.SchoolIdentityProvisioner
	Policy        ports.SchoolRolePolicy
	Audit         ports.AuthAudit
	OperatorAudit ports.OperatorAudit
	Runtime       ports.Runtime
	Logger        *slog.Logger
}

// NewOperatorAccountAccess composes the flows.
func NewOperatorAccountAccess(deps OperatorAccountAccessDependencies) (*OperatorAccountAccess, error) {
	switch {
	case deps.Store == nil, deps.Login == nil, deps.Access == nil, deps.Sessions == nil, deps.Schools == nil,
		deps.Organizations == nil, deps.Identities == nil, deps.Policy == nil, deps.Audit == nil,
		deps.OperatorAudit == nil, deps.Runtime == nil:
		return nil, errors.New("identity access operator account access: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &OperatorAccountAccess{
		store: deps.Store, login: deps.Login, access: deps.Access, sessions: deps.Sessions, schools: deps.Schools,
		organizations: deps.Organizations, identities: deps.Identities, policy: deps.Policy, audit: deps.Audit,
		operatorAudit: deps.OperatorAudit, runtime: deps.Runtime, logger: logger.With("component", "operator-account-access"),
	}, nil
}

const lehrkraftCaregiverProfileMessage = "cannot assign the lehrkraft role: the account has a caregiver profile at this school; remove it via staff offboarding first"

// ListAccountTenantAccess returns every school mapping of one account.
func (s *OperatorAccountAccess) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]domain.AccountTenantAccess, error) {
	var result []domain.AccountTenantAccess
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		if _, err := s.loadAccount(adminCtx, accountID, false); err != nil {
			return err
		}
		entries, err := s.loadTenantAccess(adminCtx, accountID)
		if err != nil {
			return err
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListAssignableSchoolRoles returns the system and target-school roles an
// operator may select. The same policy as the mutation paths applies, so the
// UI cannot offer roles the backend would reject.
func (s *OperatorAccountAccess) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]domain.AccountTenantRole, error) {
	var result []domain.AccountTenantRole
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		if _, err := s.loadActiveSchool(adminCtx, schoolID); err != nil {
			return err
		}
		roles, _, err := s.access.ListRoles(adminCtx)
		if err != nil {
			return err
		}
		result = make([]domain.AccountTenantRole, 0, len(roles))
		for _, role := range roles {
			if err := s.policy.ValidateAssignableSchoolRole(role, schoolID); err == nil {
				result = append(result, roleProjection(role))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GrantAccountTenantAccess gives an existing account access to an
// additional school and assigns the requested role there. Reactivating a
// previously revoked mapping goes through the same path.
func (s *OperatorAccountAccess) GrantAccountTenantAccess(ctx context.Context, request domain.GrantAccountTenantAccess) ([]domain.AccountTenantAccess, error) {
	var result []domain.AccountTenantAccess
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		account, err := s.loadAccount(adminCtx, request.AccountID, true)
		if err != nil {
			return err
		}
		school, err := s.loadActiveSchool(adminCtx, request.SchoolID)
		if err != nil {
			return err
		}
		role, err := s.validateAssignableSchoolRole(adminCtx, request.RoleID, request.SchoolID)
		if err != nil {
			return err
		}
		hasAccess, _, err := s.store.HasActiveAccountTenant(adminCtx, request.AccountID, request.SchoolID)
		if err != nil {
			return err
		}
		if hasAccess {
			return domain.ErrAccountTenantAccessExists
		}
		// A revoke removes the school's roles, but a role assignment left
		// without its mapping must not let the grant swap a Lehrkraft role
		// away, as the retained role assignment refused.
		current, err := s.rolesAtTenant(adminCtx, request.AccountID, request.SchoolID)
		if err != nil {
			return err
		}
		if err := s.refuseLeavingLehrkraft(current, role); err != nil {
			return err
		}
		activeTenantIDs, _, err := s.login.ListActiveTenantIDs(adminCtx, request.AccountID)
		if err != nil {
			return err
		}
		tenantCtx := s.runtime.WithTenantID(adminCtx, request.SchoolID)

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
		s.logOperatorAction(adminCtx, request.OperatorID, domain.OperatorAuditActionCreate, request.AccountID, request.ClientIP, domain.OperatorAccessChange{
			SchoolID: school.ID, Email: account.Email, RoleID: role.ID, RoleName: role.Name,
		})
		if err := s.recordAccessAuthEvent(tenantCtx, request.AccountID, request.SchoolID, domain.AuthEventTenantAccessGranted, domain.TenantAccessEvidence{
			SchoolID: school.ID, SchoolName: school.Name, Role: role.Name, OperatorID: request.OperatorID,
		}); err != nil {
			return err
		}
		entries, err := s.loadTenantAccess(adminCtx, request.AccountID)
		if err != nil {
			return err
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UpdateAccountTenantRole changes the role an account holds at one school:
// the requested role is assigned and the account's other roles at that
// school are removed, except the ones owned by other features. Switching an
// account between Verwaltung and Betreuung works, while an admin who also
// does care work keeps the caregiver role until it is removed through the
// caregiver flow.
func (s *OperatorAccountAccess) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]domain.AccountTenantAccess, error) {
	var result []domain.AccountTenantAccess
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		account, err := s.loadAccount(adminCtx, accountID, true)
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
		hasAccess, _, err := s.store.HasActiveAccountTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		if !hasAccess {
			return domain.ErrAccountTenantAccessNotFound
		}
		current, err := s.rolesAtTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		tenantCtx := s.runtime.WithTenantID(adminCtx, schoolID)
		if err := s.refuseLeavingLehrkraft(current, role); err != nil {
			return err
		}
		// Server-side mirror of the UI guards: switching an account whose
		// school identity includes a caregiver profile to Lehrkraft would
		// strand users.teachers and its active group supervisions on an
		// account whose JWT only carries class_day permissions.
		if err := s.refuseLehrkraftOverCaregiverProfile(tenantCtx, accountID, role); err != nil {
			return err
		}
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
		if err := s.assignRole(tenantCtx, accountID, schoolID, role); err != nil {
			return err
		}
		removed := make([]string, 0, len(current))
		for _, existing := range current {
			if existing.ID == role.ID || domain.RoleOwnedByOtherFeature(existing) {
				continue
			}
			if _, err := s.access.RemoveAccountRole(adminCtx, accountID, existing.ID, schoolID); err != nil {
				return err
			}
			removed = append(removed, existing.Name)
		}
		if err := s.sessions.RevokeAllTokensWithReason(tenantCtx, accountID, domain.RevocationReasonRoleChanged); err != nil {
			return fmt.Errorf("revoke tokens after role change: %w", err)
		}
		s.logOperatorAction(adminCtx, operatorID, domain.OperatorAuditActionUpdate, accountID, clientIP, domain.OperatorAccessChange{
			SchoolID: school.ID, Email: account.Email, RoleID: role.ID, RoleName: role.Name, RemovedRoles: removed,
		})
		if err := s.recordAccessAuthEvent(tenantCtx, accountID, schoolID, domain.AuthEventTenantRoleChanged, domain.TenantAccessEvidence{
			SchoolID: school.ID, SchoolName: school.Name, Role: role.Name, RemovedRoles: removed, OperatorID: operatorID,
		}); err != nil {
			return err
		}
		entries, err := s.loadTenantAccess(adminCtx, accountID)
		if err != nil {
			return err
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RevokeAccountTenantAccess withdraws an account's access to one school:
// the mapping goes inactive, the tenant-scoped role assignments and direct
// permissions are removed, the school's sessions are revoked and the
// account itself is deactivated once no active mapping remains.
//
// Person and staff records at that school are deliberately left untouched;
// they carry the name for historical attendance and work-session rows.
// Removing them is the school's own staff offboarding.
func (s *OperatorAccountAccess) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]domain.AccountTenantAccess, error) {
	var result []domain.AccountTenantAccess
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		account, err := s.loadAccount(adminCtx, accountID, true)
		if err != nil {
			return err
		}
		school, found, err := s.schools.FindSchool(adminCtx, schoolID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrSchoolNotFound
		}
		hasAccess, _, err := s.store.HasActiveAccountTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		if !hasAccess {
			return domain.ErrAccountTenantAccessNotFound
		}
		current, err := s.rolesAtTenant(adminCtx, accountID, schoolID)
		if err != nil {
			return err
		}
		for _, existing := range current {
			if domain.RoleBlocksAccessRevocation(existing) {
				return domain.InvalidInput(fmt.Sprintf("school access with role %q must be removed through its dedicated flow", existing.Name))
			}
			if _, err := s.access.RemoveAccountRole(adminCtx, accountID, existing.ID, schoolID); err != nil {
				return err
			}
		}
		if _, err := s.access.DeleteAccountPermissionsAtTenant(adminCtx, accountID, schoolID); err != nil {
			return fmt.Errorf("delete direct account permissions: %w", err)
		}
		tenantCtx := s.runtime.WithTenantID(adminCtx, schoolID)
		if err := s.sessions.RevokeAllTokensWithReason(tenantCtx, accountID, domain.RevocationReasonTenantAccessRevoked); err != nil {
			return fmt.Errorf("revoke tokens after access revocation: %w", err)
		}
		if _, err := s.access.DeactivateTenantMapping(adminCtx, accountID, schoolID); err != nil {
			return err
		}
		remaining, _, err := s.login.ListActiveTenantIDs(adminCtx, accountID)
		if err != nil {
			return err
		}
		if len(remaining) == 0 {
			if err := s.deactivateAccount(adminCtx, accountID); err != nil {
				return err
			}
		}
		s.logOperatorAction(adminCtx, operatorID, domain.OperatorAuditActionDelete, accountID, clientIP, domain.OperatorAccessChange{
			SchoolID: school.ID, Email: account.Email, AccountDeactivated: len(remaining) == 0,
		})
		if err := s.recordAccessAuthEvent(tenantCtx, accountID, schoolID, domain.AuthEventTenantAccessRevoked, domain.TenantAccessEvidence{
			SchoolID: school.ID, SchoolName: school.Name, AccountDeactivated: len(remaining) == 0, OperatorID: operatorID,
		}); err != nil {
			return err
		}
		entries, err := s.loadTenantAccess(adminCtx, accountID)
		if err != nil {
			return err
		}
		result = entries
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// loadAccount resolves the account or reports ErrAccountNotFound. With
// forUpdate the row is locked: grants, revocations and role changes derive
// their mapping and active-account state from the locked row, avoiding
// competing decisions in parallel operator requests.
func (s *OperatorAccountAccess) loadAccount(ctx context.Context, accountID int64, forUpdate bool) (domain.ManagedAccount, error) {
	if accountID <= 0 {
		return domain.ManagedAccount{}, domain.ErrAccountNotFound
	}
	account, found, _, err := s.access.FindManagedAccount(ctx, accountID, forUpdate)
	if err != nil {
		return domain.ManagedAccount{}, err
	}
	if !found {
		return domain.ManagedAccount{}, domain.ErrAccountNotFound
	}
	return account, nil
}

// loadActiveSchool resolves a live, active school.
func (s *OperatorAccountAccess) loadActiveSchool(ctx context.Context, schoolID int64) (domain.School, error) {
	school, found, err := s.schools.FindSchool(ctx, schoolID)
	if err != nil {
		return domain.School{}, err
	}
	if !found {
		return domain.School{}, domain.ErrSchoolNotFound
	}
	if school.Deleted {
		return domain.School{}, domain.ErrSchoolDeleted
	}
	if !school.Active {
		return domain.School{}, domain.InvalidInput("school is inactive")
	}
	return school, nil
}

// validateAssignableSchoolRole resolves the role and applies the shared
// role-assignment policy; a missing role fails the same policy check.
func (s *OperatorAccountAccess) validateAssignableSchoolRole(ctx context.Context, roleID, schoolID int64) (domain.RoleFact, error) {
	var role domain.RoleFact
	if roleID > 0 {
		resolved, found, _, err := s.access.FindRole(ctx, roleID)
		if err != nil {
			return domain.RoleFact{}, err
		}
		if found {
			role = resolved
		}
	}
	if err := s.policy.ValidateAssignableSchoolRole(role, schoolID); err != nil {
		return domain.RoleFact{}, &domain.InvalidInputError{Err: err}
	}
	return role, nil
}

// refuseLeavingLehrkraft keeps a Lehrkraft account on its role at the
// school: any other role would invalidate the account's school identity
// and must go through offboarding plus a new account or invitation. This
// also covers a caregiver-tier role, whose profile a Lehrkraft account
// never has.
func (s *OperatorAccountAccess) refuseLeavingLehrkraft(current []domain.AccountTenantRole, role domain.RoleFact) error {
	if s.policy.IsLehrkraftSystemRole(role) {
		return nil
	}
	for _, existing := range current {
		if existing.IsSystem && strings.EqualFold(existing.Name, domain.LehrkraftRoleName) {
			return &domain.InvalidInputError{Err: s.policy.LehrkraftRoleImmutable()}
		}
	}
	return nil
}

func (s *OperatorAccountAccess) refuseLehrkraftOverCaregiverProfile(tenantCtx context.Context, accountID int64, role domain.RoleFact) error {
	if !s.policy.IsLehrkraftSystemRole(role) {
		return nil
	}
	hasProfile, err := s.identities.HasLiveCaregiverProfile(tenantCtx, accountID)
	if err != nil {
		return err
	}
	if hasProfile {
		return domain.InvalidInput(lehrkraftCaregiverProfileMessage)
	}
	return nil
}

// assignRole assigns the role at the school once. A new assignment revokes
// the account's sessions at that school, because its claims changed, and
// the identity chain a staff-tier role owes is completed idempotently so a
// repeated call repairs an account an earlier half-written grant left
// behind (#2222).
func (s *OperatorAccountAccess) assignRole(tenantCtx context.Context, accountID, schoolID int64, role domain.RoleFact) error {
	created, _, err := s.store.AssignAccountRole(tenantCtx, accountID, role.ID, schoolID)
	if err != nil {
		return fmt.Errorf("assign role to account: %w", err)
	}
	if created {
		revoked, err := s.sessions.DeleteAccountSessionsWithAudit(tenantCtx, accountID, domain.RevocationReasonRoleChanged, "", "")
		if err != nil {
			return fmt.Errorf("revoke tokens after role assignment: %w", err)
		}
		s.sessions.QueuePushCleanup(tenantCtx, accountID, revoked, domain.RevocationReasonRoleChanged)
	}
	if err := s.identities.EnsureSchoolIdentity(tenantCtx, domain.SchoolIdentityRequest{
		AccountID: accountID, TenantID: schoolID, Role: role, CreatePerson: false,
	}); err != nil {
		return fmt.Errorf("provision school identity: %w", err)
	}
	return nil
}

// identityNamesForSchool resolves the name a new person at this school
// would carry. Caller-supplied names win; an account that already has a
// live person at this school needs no name; otherwise the name is borrowed
// from an unambiguous non-student identity at another of the account's
// schools, and the request is refused without one.
func (s *OperatorAccountAccess) identityNamesForSchool(tenantCtx, adminCtx context.Context, accountID int64, firstName, lastName string) (string, string, error) {
	if firstName != "" && lastName != "" {
		return firstName, lastName, nil
	}
	live, err := s.identities.HasLivePersonAtSchool(tenantCtx, accountID)
	if err != nil {
		return "", "", err
	}
	if live {
		return "", "", nil
	}
	persons, err := s.identities.ListAccountPersons(adminCtx, accountID)
	if err != nil {
		return "", "", err
	}
	if existing, found := domain.UnambiguousPersonIdentity(persons); found {
		if firstName == "" {
			firstName = existing.FirstName
		}
		if lastName == "" {
			lastName = existing.LastName
		}
	}
	if firstName == "" || lastName == "" {
		return "", "", domain.InvalidInput("first_name and last_name are required: the account has no person record at this school, and none of its other schools provides an unambiguous name to use")
	}
	return firstName, lastName, nil
}

func (s *OperatorAccountAccess) rolesAtTenant(ctx context.Context, accountID, schoolID int64) ([]domain.AccountTenantRole, error) {
	assignments, _, err := s.access.ListAccountRoleAssignments(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return domain.RolesAtTenant(assignments, schoolID), nil
}

// activateAccount restores a deactivated account and closes the pending
// account-wide wipe its deactivation recorded, on the same transaction.
func (s *OperatorAccountAccess) activateAccount(adminCtx context.Context, accountID int64) error {
	if _, err := s.access.SetAccountActive(adminCtx, accountID, true); err != nil {
		return err
	}
	warn := func(err error) {
		if err != nil {
			s.logger.Warn("failed to clear pending account-wide wipe after reactivation",
				slog.Int64("account_id", accountID),
				slog.Any("error", err))
		}
	}
	// As the retained reactivation did: once the grant commits, the pending
	// wipe is cleared in its own administrative transaction, so a failed
	// cleanup cannot abort the grant. Without after-commit hooks it runs
	// inside the caller's administrative transaction.
	if s.runtime.HasAfterCommitHooks(adminCtx) {
		detached := s.runtime.Detach(adminCtx)
		s.runtime.RegisterAfterCommit(adminCtx, func() {
			warn(s.runtime.WithAdminTx(detached, func(txCtx context.Context) error {
				return s.sessions.MarkAccountWideWipeCompleted(txCtx, accountID)
			}))
		})
		return nil
	}
	warn(s.sessions.MarkAccountWideWipeCompleted(adminCtx, accountID))
	return nil
}

// deactivateAccount disables the account and wipes its sessions at every
// school; inside the administrative transaction the wipe runs immediately.
func (s *OperatorAccountAccess) deactivateAccount(adminCtx context.Context, accountID int64) error {
	if _, err := s.access.SetAccountActive(adminCtx, accountID, false); err != nil {
		return err
	}
	if err := s.sessions.ScheduleAccountWideRevoke(adminCtx, accountID, domain.RevocationReasonAccountDeactivated, "", ""); err != nil {
		return failed("revoke tokens during account deactivation", err)
	}
	return nil
}

// loadTenantAccess merges the mapping rows with the school and organisation
// facts, the identity facts and the roles the account holds at each school:
// deleted schools are hidden, and the entries are ordered by organisation
// name, then school name.
func (s *OperatorAccountAccess) loadTenantAccess(ctx context.Context, accountID int64) ([]domain.AccountTenantAccess, error) {
	mappings, _, err := s.access.ListTenantMappings(ctx, accountID)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.AccountTenantAccess, 0, len(mappings))
	if len(mappings) == 0 {
		return entries, nil
	}
	tenantIDs := make([]int64, 0, len(mappings))
	for _, mapping := range mappings {
		tenantIDs = append(tenantIDs, mapping.TenantID)
	}
	schools, err := s.organizations.ListSchools(ctx, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("load schools for account access: %w", err)
	}
	schoolByID := make(map[int64]domain.School, len(schools))
	for _, school := range schools {
		schoolByID[school.ID] = school
	}
	organizationIDs := make([]int64, 0, len(mappings))
	seenOrganizations := make(map[int64]struct{}, len(mappings))
	for _, mapping := range mappings {
		school, found := schoolByID[mapping.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing for account access", mapping.TenantID)
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
	facts, err := s.identities.ListAccountIdentityFacts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	factByTenant := make(map[int64]domain.AccountIdentityFact, len(facts))
	for _, fact := range facts {
		factByTenant[fact.TenantID] = fact
	}
	organizations, err := s.organizations.ListOrganizationNames(ctx, organizationIDs)
	if err != nil {
		return nil, fmt.Errorf("load organizations for account tenant access: %w", err)
	}
	if !domain.SortTenantAccessByOrganization(entries, organizations) {
		return nil, errors.New("organization missing from account tenant access query")
	}
	assignments, _, err := s.access.ListAccountRoleAssignments(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for index := range entries {
		fact := factByTenant[entries[index].TenantID]
		entries[index].HasPerson = fact.HasPerson
		entries[index].HasStaff = fact.HasStaff
		if roles := domain.RolesAtTenant(assignments, entries[index].TenantID); roles != nil {
			entries[index].Roles = roles
		}
	}
	return entries, nil
}

func roleProjection(role domain.RoleFact) domain.AccountTenantRole {
	return domain.AccountTenantRole{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole}
}

// logOperatorAction records the platform-side audit entry; a failed write
// is logged, as the retained provisioning service did.
func (s *OperatorAccountAccess) logOperatorAction(ctx context.Context, operatorID int64, action string, accountID int64, clientIP string, change domain.OperatorAccessChange) {
	entry := domain.OperatorAuditEntry{
		OperatorID: operatorID, Action: action, ResourceType: domain.OperatorAuditResourceMapping,
		ResourceID: &accountID, AccessChange: &change, IPAddress: clientIP,
	}
	if err := s.operatorAudit.RecordOperatorAction(ctx, entry); err != nil {
		s.logger.Error("failed to create operator audit log",
			slog.Any("error", err),
			slog.String("resource_type", domain.OperatorAuditResourceMapping))
	}
}

// recordAccessAuthEvent writes the tenant-visible audit trail so the
// affected school can see access changes to its own data. It is part of the
// surrounding transaction, so failures are returned rather than swallowed.
func (s *OperatorAccountAccess) recordAccessAuthEvent(tenantCtx context.Context, accountID, schoolID int64, eventType string, evidence domain.TenantAccessEvidence) error {
	event := domain.AuthEvent{
		AccountID: accountID, TenantID: schoolID, Type: eventType, Success: true,
		IPAddress: domain.AccessAuditIP, TenantAccess: &evidence,
	}
	if err := s.audit.RecordAuthEvent(tenantCtx, event); err != nil {
		return fmt.Errorf("record tenant access auth event: %w", err)
	}
	return nil
}
