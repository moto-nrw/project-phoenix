package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// RoleAdministration runs role and permission management (#3314): role and
// permission CRUD, account role assignment with the school identity it owes,
// direct account grants and role-permission selections. The identity-owned
// rows are read and written through the role store the composition binds;
// the school identity chain and the session revocation are the module's own
// flows, reached through ports so the rules can be exercised in isolation.
type RoleAdministration struct {
	*RoleCatalog
	accounts      ports.ManageableAccounts
	store         ports.RoleStore
	profiles      ports.CaregiverProfiles
	policy        ports.RoleAssignmentPolicy
	identityRoles ports.RolePolicy
	identity      ports.SchoolIdentity
	sessions      ports.SessionRevocation
	runtime       ports.Runtime
	attach        func(context.Context) context.Context
	logger        *slog.Logger
}

// RoleAdministrationDependencies are the ports the administration consumes.
type RoleAdministrationDependencies struct {
	Accounts ports.ManageableAccounts
	Store    ports.RoleStore
	Profiles ports.CaregiverProfiles
	Policy   ports.RoleAssignmentPolicy
	// IdentityRoles decides which roles owe a caregiver profile.
	IdentityRoles ports.RolePolicy
	Identity      ports.SchoolIdentity
	Sessions      ports.SessionRevocation
	Runtime       ports.Runtime
	// Attach binds the composed unit of work ahead of every public flow.
	Attach func(context.Context) context.Context
	Logger *slog.Logger
}

// NewRoleAdministration composes the administration over its ports.
func NewRoleAdministration(deps RoleAdministrationDependencies) (*RoleAdministration, error) {
	switch {
	case deps.Store == nil, deps.Profiles == nil, deps.Accounts == nil:
		return nil, fmt.Errorf("identity access role administration: role store, manageable accounts and caregiver profiles are required")
	case deps.Policy == nil, deps.IdentityRoles == nil:
		return nil, fmt.Errorf("identity access role administration: role policies are required")
	case deps.Identity == nil, deps.Sessions == nil:
		return nil, fmt.Errorf("identity access role administration: school identity and session revocation are required")
	case deps.Runtime == nil:
		return nil, fmt.Errorf("identity access role administration: tenant runtime is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &RoleAdministration{
		RoleCatalog: NewRoleCatalog(deps.Store, deps.Runtime.RunInTx),
		accounts:    deps.Accounts, store: deps.Store, profiles: deps.Profiles, policy: deps.Policy, identityRoles: deps.IdentityRoles,
		identity: deps.Identity, sessions: deps.Sessions, runtime: deps.Runtime, attach: deps.Attach, logger: logger,
	}, nil
}

// --- roles -----------------------------------------------------------------

func (r *RoleAdministration) CreateRole(ctx context.Context, name, description string, baseRole *string) (value0 identityaccess.Role, err error) {
	if r == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (domain.ManagedRole, error) {
		if baseRole == nil || *baseRole == "" {
			return domain.ManagedRole{}, failed("create role", domain.ErrBaseRoleRequired)
		}
		role, err := r.store.CreateRole(ctx, domain.ManagedRole{Name: name, Description: description, BaseRole: baseRole})
		if err != nil {
			return domain.ManagedRole{}, failed("create role", err)
		}
		return role, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = identityaccess.Role(result)
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) GetRole(ctx context.Context, id int64) (value0 identityaccess.Role, err error) {
	if r == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (domain.ManagedRole, error) {
		role, found, err := r.store.FindRole(ctx, id)
		if err != nil {
			return domain.ManagedRole{}, failed("get role", err)
		}
		if !found {
			return domain.ManagedRole{}, failed("get role", domain.ErrRoleNotFound)
		}
		return role, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = identityaccess.Role(result)
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) resolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (domain.ManagedRole, []string, error) {
	if roleID <= 0 {
		return domain.ManagedRole{}, nil, r.policy.ValidateAssignableSchoolRole(nil, tenantID)
	}
	role, found, err := r.store.FindRole(ctx, roleID)
	if err != nil {
		return domain.ManagedRole{}, nil, err
	}
	if !found {
		return domain.ManagedRole{}, nil, r.policy.ValidateAssignableSchoolRole(nil, tenantID)
	}
	if err := r.policy.ValidateAssignableSchoolRole(role.Facts(), tenantID); err != nil {
		return domain.ManagedRole{}, nil, err
	}
	permissions, err := r.store.ListRolePermissions(ctx, role.ID)
	if err != nil {
		return domain.ManagedRole{}, nil, err
	}
	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.Name)
	}
	return role, names, nil
}

func (r *RoleAdministration) UpdateRole(ctx context.Context, role identityaccess.Role) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context, role domain.ManagedRole) error {
		existing, found, err := r.store.FindRole(ctx, role.ID)
		if err != nil || !found {
			return failed("update role", domain.ErrRoleNotFound)
		}
		if existing.IsSystem {
			return failed("update role", identityaccess.ErrSystemRoleImmutable)
		}
		if err := r.store.UpdateRole(ctx, role); err != nil {
			return failed("update role", err)
		}
		return nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		return roleError(operation(bindRuntime(r.attach, txCtx), domain.ManagedRole(role)))
	})
	return
}

// DeleteRole deletes a custom role with its account and permission mappings.
func (r *RoleAdministration) DeleteRole(ctx context.Context, id int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		// ReplaceRolePermissions locks the role before its mappings. Take the
		// same lock first so deleting a role cannot wait for a mapping while a
		// concurrent replacement waits for the role.
		role, found, err := r.store.FindRoleForUpdate(txCtx, id)
		if err != nil {
			return failed("delete role", err)
		}
		if !found {
			return failed("delete role", domain.ErrRoleNotFound)
		}
		if role.IsSystem {
			return failed("delete role", identityaccess.ErrSystemRoleImmutable)
		}

		if err := r.store.DeleteRoleAssignments(txCtx, id); err != nil {
			return failed("delete account role mappings", err)
		}
		if err := r.store.DeleteRolePermissions(txCtx, id); err != nil {
			return failed("delete role permissions", err)
		}
		if err := r.store.DeleteRole(txCtx, id); err != nil {
			return failed("delete role", err)
		}
		return nil
	})
}

// AssignRoleToAccount assigns a role to an account at the tenant in context
// and completes the school identity the role owes.
func (r *RoleAdministration) AssignRoleToAccount(ctx context.Context, accountID, roleID int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	var revoked []identityaccess.AccountSession
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		// Serialize assignments with tenant-access revocation, which holds the
		// same account lock while removing the tenant's roles and mapping.
		if err := r.lockManageableAccount(txCtx, accountID, "assign role"); err != nil {
			return err
		}

		// System roles have tenant_id NULL and must remain resolvable while the
		// assignment itself is written for the caller's tenant. Custom roles are
		// still restricted to that tenant.
		role, err := r.roleForAssignment(txCtx, roleID)
		if err != nil {
			return err
		}
		facts := role.Facts()

		if err := r.requireLehrkraftPreserved(txCtx, accountID, facts); err != nil {
			return err
		}
		if err := r.requireNoCaregiverForLehrkraft(txCtx, accountID, facts); err != nil {
			return err
		}
		if err := r.requireCaregiverAssignmentProfile(txCtx, accountID, facts); err != nil {
			return err
		}
		revoked, err = r.assignMissingAccountRole(txCtx, accountID, roleID)
		if err != nil {
			return err
		}
		// Handing out a staff-tier role is the same act as inviting one, so it
		// owes the same identity (#2222). Without this, the assignment is a
		// way to produce an account that holds the role, logs in, and is not
		// staff as far as the database is concerned.
		//
		// Runs for an already-assigned role too, so a repeated call repairs an
		// account an earlier behaviour left half-written.
		return r.ensureIdentityForAssignedRole(txCtx, accountID, facts)
	})
	if err != nil {
		return err
	}
	r.sessions.QueuePushCleanup(ctx, accountID, revoked, "role_changed")
	return nil
}

// ReplaceAccountRole makes roleID the account's only staff role at the
// caller's tenant. The Konto field "Systemrolle" is single-valued (#3116):
// after a successful call the account holds the target role and no other
// staff role at this school, however many an earlier half-finished swap or a
// direct POST left behind. Guardian-tier roles are the exception and stay
// untouched: they carry parent-portal access, which is a separate
// relationship and never something a staff role change may revoke.
//
// The target is assigned first so the account never temporarily loses its
// staff role; a later failure rolls back the complete exchange.
func (r *RoleAdministration) ReplaceAccountRole(ctx context.Context, accountID, roleID int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		return r.replaceAccountRoleInTx(txCtx, accountID, roleID)
	})
}

// replaceAccountRoleInTx assigns the role and removes the account's other
// school roles inside the caller's transaction.
func (r *RoleAdministration) replaceAccountRoleInTx(txCtx context.Context, accountID, roleID int64) error {
	// Take the same account lock as single-role mutations before changing
	// an assignment. This serializes replacements with individual
	// assignments and removals for the same account.
	if err := r.lockManageableAccount(txCtx, accountID, "replace account role"); err != nil {
		return err
	}

	if err := r.AssignRoleToAccount(txCtx, accountID, roleID); err != nil {
		return err
	}

	// The listing applies the tenant filter, so roles held at other
	// schools are never in it and never removed here.
	current, err := r.store.ListAccountRoles(txCtx, accountID)
	if err != nil {
		return failed("replace account role", err)
	}
	for _, role := range current {
		// Guardian is decided by tier, not by name: a school role labelled
		// "Guardian" is not the parent-portal role, and the parent-portal
		// role may carry a custom label.
		if role.ID == roleID || r.policy.IsGuardianTierRole(role.Facts()) {
			continue
		}
		if err := r.RemoveRoleFromAccount(txCtx, accountID, role.ID); err != nil {
			return err
		}
	}
	return nil
}

// ensureIdentityForAssignedRole completes the school identity chain for a
// role that was just assigned, inside the caller's transaction.
//
// It never creates the person: the assignment carries no identity fields, and
// inventing a name is not something a role assignment may do — the same line
// the operator role change draws. An account without a person at this school
// is left to the flows that do have a name (staff creation, invitation),
// which is also why nothing here fails when there is none.
func (r *RoleAdministration) ensureIdentityForAssignedRole(ctx context.Context, accountID int64, role *domain.RoleFacts) error {
	if _, err := r.identity.EnsureSchoolIdentityChain(ctx, domain.SchoolIdentityInput{
		AccountID:    accountID,
		TenantID:     r.runtime.TenantID(ctx),
		Role:         role,
		CreatePerson: false,
	}); err != nil {
		return failed("provision school identity", err)
	}
	return nil
}

func (r *RoleAdministration) AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (value0 bool, err error) {
	if r == nil {
		return false, errRoleAdministrationUnavailable
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		holds, err := r.accountHoldsLehrkraftRole(bindRuntime(r.attach, txCtx), accountID)
		value0 = holds
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) accountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error) {
	roles, err := r.store.ListAccountRoles(ctx, accountID)
	if err != nil {
		return false, err
	}
	for _, role := range roles {
		if r.policy.IsLehrkraftSystemRole(role.Facts()) {
			return true, nil
		}
	}
	return false, nil
}

// RemoveRoleFromAccount removes a role from an account; removing a role the
// account does not hold is a no-op.
func (r *RoleAdministration) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	var revoked []identityaccess.AccountSession
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockManageableAccount(txCtx, accountID, "remove role"); err != nil {
			return err
		}
		assigned, err := r.store.AccountHoldsRole(txCtx, accountID, roleID)
		if err != nil {
			return failed("check role assignment", err)
		}
		if !assigned {
			return nil
		}

		if err := r.store.DeleteAccountRole(txCtx, accountID, roleID); err != nil {
			return failed("remove role from account", err)
		}

		tokens, err := r.sessions.DeleteAccountSessionsWithAudit(txCtx, accountID, "role_changed", "", "")
		if err != nil {
			return failed("revoke tokens after role removal", err)
		}
		revoked = tokens
		return nil
	})
	if err != nil {
		return err
	}
	r.sessions.QueuePushCleanup(ctx, accountID, revoked, "role_changed")
	return nil
}

func (r *RoleAdministration) GetAccountRoles(ctx context.Context, accountID int64) (value0 []identityaccess.Role, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		roles, err := r.accountRoles(bindRuntime(r.attach, txCtx), accountID)
		value0 = publicRoles(roles)
		return roleError(err)
	})
	return
}

// AccountHoldsRole reports whether the account holds the role of that exact
// name. A failed lookup counts as not holding it: the answer only ever
// withholds.
func (r *RoleAdministration) AccountHoldsRole(ctx context.Context, accountID int64, roleName string) bool {
	held, err := r.GetAccountRoles(ctx, accountID)
	if err != nil {
		return false
	}
	for _, role := range held {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

func (r *RoleAdministration) accountRoles(ctx context.Context, accountID int64) ([]domain.ManagedRole, error) {
	if _, err := r.accounts.FindManageableAccount(ctx, accountID); err != nil {
		return nil, failed("get account roles", domain.ErrAccountNotFound)
	}
	if err := r.ensureOrganizationRBACMembership(ctx, accountID, "get account roles", false); err != nil {
		return nil, err
	}
	roles, err := r.store.ListAccountRoles(ctx, accountID)
	if err != nil {
		return nil, failed("get account roles", err)
	}
	return roles, nil
}

func (r *RoleAdministration) accountsHoldingRole(ctx context.Context, accountIDs []int64, roleName string) (map[int64]bool, error) {
	if len(accountIDs) == 0 || roleName == "" {
		return map[int64]bool{}, nil
	}
	uniqueAccountIDs := make([]int64, 0, len(accountIDs))
	seen := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		if _, exists := seen[accountID]; exists {
			continue
		}
		seen[accountID] = struct{}{}
		uniqueAccountIDs = append(uniqueAccountIDs, accountID)
	}
	matches, err := r.store.ListAccountsHoldingRole(ctx, uniqueAccountIDs, roleName)
	if err != nil {
		return nil, failed("get accounts holding role", err)
	}
	return matches, nil
}

func (r *RoleAdministration) GetAccountRoleNames(ctx context.Context, accountIDs []int64) (value0 map[int64]string, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (map[int64]string, error) {
		names, err := r.store.ListAccountRoleNames(ctx, accountIDs)
		if err != nil {
			return nil, failed("get account role names", err)
		}
		return names, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = result
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) GetAccountEmails(ctx context.Context, accountIDs []int64) (value0 map[int64]string, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (map[int64]string, error) {
		emails, err := r.store.ListAccountEmails(ctx, accountIDs)
		if err != nil {
			return nil, failed("get account emails by IDs", err)
		}
		return emails, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = result
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) GetAccountAvatars(ctx context.Context, accountIDs []int64) (value0 map[int64]string, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (map[int64]string, error) {
		avatars, err := r.store.ListAccountAvatars(ctx, accountIDs)
		if err != nil {
			return nil, failed("get account avatars by IDs", err)
		}
		return avatars, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = result
		return roleError(err)
	})
	return
}

// --- account locks -----------------------------------------------------------

// lockManageableAccount checks that the caller may manage the account, locks
// the account row and re-checks both under the lock. In organisation scope
// the account's active school membership is taken FOR SHARE, so a concurrent
// tenant-access revocation waits for the mutation.
func (r *RoleAdministration) lockManageableAccount(ctx context.Context, accountID int64, op string) error {
	if _, err := r.accounts.FindManageableAccount(ctx, accountID); err != nil {
		return failed(op, domain.ErrAccountNotFound)
	}
	if err := r.ensureOrganizationRBACMembership(ctx, accountID, op, false); err != nil {
		return err
	}
	if found, err := r.store.LockAccount(ctx, accountID); err != nil || !found {
		return failed(op, domain.ErrAccountNotFound)
	}
	if _, err := r.accounts.FindManageableAccount(ctx, accountID); err != nil {
		return failed(op, domain.ErrAccountNotFound)
	}
	if err := r.ensureOrganizationRBACMembership(ctx, accountID, op, true); err != nil {
		return err
	}
	return nil
}

// ensureOrganizationRBACMembership restricts organisation-scoped callers to
// accounts mapped to the school in context. Tenant and platform scopes pass.
func (r *RoleAdministration) ensureOrganizationRBACMembership(ctx context.Context, accountID int64, op string, lock bool) error {
	if r.runtime.Scope(ctx) != domain.ScopeOrg {
		return nil
	}
	tenantID := r.runtime.TenantID(ctx)
	if tenantID == 0 {
		return failed(op, domain.ErrAccountNotFound)
	}
	exists, err := r.store.HasTenantMembership(ctx, accountID, tenantID, lock)
	if err != nil {
		return failed(op, err)
	}
	if !exists {
		return failed(op, domain.ErrAccountNotFound)
	}
	return nil
}

type LateSchoolIdentity struct {
	Current func() *AccountLifecycle
}

func (i LateSchoolIdentity) EnsureSchoolIdentityChain(ctx context.Context, input domain.SchoolIdentityInput) (*domain.SchoolIdentity, error) {
	lifecycle := i.Current()
	if lifecycle == nil {
		return nil, errors.New("identity access compose: the school identity chain is not composed")
	}
	return lifecycle.EnsureSchoolIdentityChain(ctx, input)
}

// accountAdministration serves the offboarding port: role removals run
// through the role administration so their session revocation holds, the
// deactivation through the account administration, which schedules the
// account-wide session wipe with it (#3332).
type AccountAdministrationSeam struct {
	Roles    *RoleAdministration
	Accounts *AccountAdministration
}

func (a AccountAdministrationSeam) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error {
	return a.Roles.RemoveRoleFromAccount(ctx, accountID, roleID)
}

func (a AccountAdministrationSeam) DeactivateAccount(ctx context.Context, accountID int64) error {
	return a.Accounts.DeactivateAccount(ctx, accountID)
}

func (d StaffDirectory) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return d.Source.HasLiveCaregiverProfile(ctx, accountID)
}

// roleAssignmentPolicy binds the public assignment rules to the application
// port.
type RoleAssignmentPolicy struct{}

func (RoleAssignmentPolicy) IsLehrkraftSystemRole(role *domain.RoleFacts) bool {
	return identityaccess.IsLehrkraftSystemRole(publicRoleFacts(role))
}

func (RoleAssignmentPolicy) IsGuardianTierRole(role *domain.RoleFacts) bool {
	return identityaccess.IsGuardianTierRole(publicRoleFacts(role))
}

func (RoleAssignmentPolicy) IsPlatformCaregiverRole(role *domain.RoleFacts) bool {
	return identityaccess.IsPlatformCaregiverRole(publicRoleFacts(role))
}

func (RoleAssignmentPolicy) ValidateAssignableSchoolRole(role *domain.RoleFacts, tenantID int64) error {
	return identityaccess.ValidateAssignableSchoolRole(publicRoleFacts(role), tenantID)
}

func (r *RoleAdministration) requireLehrkraftPreserved(txCtx context.Context, accountID int64, facts *domain.RoleFacts) error {
	if !r.policy.IsLehrkraftSystemRole(facts) {
		isLehrkraft, roleErr := r.accountHoldsLehrkraftRole(txCtx, accountID)
		if roleErr != nil {
			return failed("assign role", roleErr)
		}
		if isLehrkraft {
			return failed("assign role", identityaccess.ErrLehrkraftRoleImmutable)
		}
	}

	return nil
}

func (r *RoleAdministration) requireNoCaregiverForLehrkraft(txCtx context.Context, accountID int64, facts *domain.RoleFacts) error {
	// Server-side mirror of the operator guards (#1772): the Lehrkraft
	// role must not land on an account whose identity at this school
	// carries a live caregiver profile — a direct call to the tenant RBAC
	// endpoint would otherwise strand users.teachers and its group
	// supervisions under a class_day-only JWT. Same rule as the operator
	// school access and the invitation flow.
	if r.policy.IsLehrkraftSystemRole(facts) {
		hasProfile, profErr := r.profiles.HasLiveCaregiverProfile(txCtx, accountID)
		if profErr != nil {
			return failed("assign role", profErr)
		}
		if hasProfile {
			return failed("assign role", identityaccess.ErrRoleLehrkraftCaregiverProfile)
		}
	}

	return nil
}

func (r *RoleAdministration) requireCaregiverAssignmentProfile(txCtx context.Context, accountID int64, facts *domain.RoleFacts) error {
	// And the same swap in reverse (#1772): a Lehrkraft account has no
	// users.teachers row by construction, so handing it a caregiver role
	// here would grant the permissions without the profile they read
	// through — no groups, no supervision, an empty caregiver landing
	// page. The operator role change provisions the profile before it
	// assigns the role and therefore passes; this command has no identity
	// fields, so the switch belongs to offboarding plus a fresh account.
	// Roles that legitimately run without a profile (admin) are
	// unaffected.
	if r.identityRoles.RoleNeedsCaregiverProfile(facts) {
		isLehrkraft, roleErr := r.accountHoldsLehrkraftRole(txCtx, accountID)
		if roleErr != nil {
			return failed("assign role", roleErr)
		}
		if isLehrkraft {
			hasProfile, profErr := r.profiles.HasLiveCaregiverProfile(txCtx, accountID)
			if profErr != nil {
				return failed("assign role", profErr)
			}
			if !hasProfile {
				return failed("assign role", identityaccess.ErrRoleCaregiverNeedsProfile)
			}
		}
	}

	return nil
}

func (r *RoleAdministration) assignMissingAccountRole(txCtx context.Context, accountID, roleID int64) (revoked []identityaccess.AccountSession, err error) {
	assigned, err := r.store.AccountHoldsRole(txCtx, accountID, roleID)
	if err != nil {
		return revoked, failed("check role assignment", err)
	}
	if !assigned {
		if err := r.store.CreateAccountRole(txCtx, accountID, roleID, r.runtime.TenantID(txCtx)); err != nil {
			return revoked, failed("assign role to account", err)
		}
		tokens, err := r.sessions.DeleteAccountSessionsWithAudit(txCtx, accountID, "role_changed", "", "")
		if err != nil {
			return revoked, failed("revoke tokens after role assignment", err)
		}
		revoked = tokens
	}

	return revoked, nil
}

func (r *RoleAdministration) ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (value0 *identityaccess.AssignableSchoolRole, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (*identityaccess.AssignableSchoolRole, error) {
		role, permissions, err := r.resolveAssignableSchoolRole(bindRuntime(r.attach, ctx), roleID, tenantID)
		if err != nil {
			return nil, roleError(err)
		}
		return &identityaccess.AssignableSchoolRole{Role: identityaccess.Role(role), Permissions: permissions}, nil
	}
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (r *RoleAdministration) AccountsHoldingRole(ctx context.Context, accountIDs []int64, roleName string) (value0 map[int64]bool, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	if len(accountIDs) == 0 {
		return map[int64]bool{}, nil
	}
	if roleName == "" {
		return map[int64]bool{}, nil
	}
	operation := func(ctx context.Context) (map[int64]bool, error) {
		matches, err := r.accountsHoldingRole(bindRuntime(r.attach, ctx), accountIDs, roleName)
		return matches, roleError(err)
	}
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (r *RoleAdministration) CreatePermission(ctx context.Context, name, description, resource, action string) (value0 identityaccess.Permission, err error) {
	if r == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.Permission, error) {
		permission, err := r.store.CreatePermission(bindRuntime(r.attach, ctx), domain.ManagedPermission{
			Name: name, Description: description, Resource: resource, Action: action,
		})
		if err != nil {
			return identityaccess.Permission{}, roleError(failed("create permission", err))
		}
		return identityaccess.Permission(permission), nil
	}
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (r *RoleAdministration) UpdatePermission(ctx context.Context, permission identityaccess.Permission) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) error {
		if err := r.store.UpdatePermission(bindRuntime(r.attach, ctx), domain.ManagedPermission(permission)); err != nil {
			return roleError(failed("update permission", err))
		}
		return nil
	}
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (r *RoleAdministration) DeletePermission(ctx context.Context, id int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) error {
		return roleError(r.deletePermission(bindRuntime(r.attach, ctx), id))
	}
	err = globalIdentityRead(ctx, r.runtime, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (r *RoleAdministration) GrantStaffDefaultPermission(ctx context.Context, accountID int64, isTeacher bool, permissionName string) {
	if r == nil {
		return
	}
	r.grantStaffDefaultPermission(bindRuntime(r.attach, ctx), accountID, isTeacher, permissionName)
}
