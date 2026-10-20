package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Operation names the provisioning flows report. They are the envelope the
// retained consumers read, so their text is part of the contract (#3332).
const (
	opRegisterAccount   = "register"
	opLinkAccount       = "link-to-tenant"
	opProvisionIdentity = "provision school identity"
)

// AccountProvisioning creates accounts at a school and gives existing ones
// access to one (#3332): POST /auth/register, POST /auth/link-to-tenant,
// the operator-led school account creation and the seeder.
//
// Both flows write the account, its school mapping, its role and its
// identity chain in ONE transaction. Splitting them across requests is what
// leaves accounts that hold a role and are not staff (#2222).
type AccountProvisioning struct {
	accounts  ports.SchoolAccountStore
	logins    ports.AccountLoginStore
	roleStore ports.Store
	roles     ports.RoleStore
	policy    ports.RoleAssignmentPolicy
	identity  ports.SchoolIdentity
	profiles  ports.CaregiverProfiles
	passwords ports.PasswordPolicy
	runtime   ports.Runtime
	attach    func(context.Context) context.Context
	logger    *slog.Logger
}

// AccountProvisioningDependencies are the ports the flows consume.
type AccountProvisioningDependencies struct {
	Accounts  ports.SchoolAccountStore
	Logins    ports.AccountLoginStore
	RoleStore ports.Store
	Roles     ports.RoleStore
	Policy    ports.RoleAssignmentPolicy
	Identity  ports.SchoolIdentity
	Profiles  ports.CaregiverProfiles
	Passwords ports.PasswordPolicy
	Runtime   ports.Runtime
	// Attach binds the composed unit of work ahead of every public flow.
	Attach func(context.Context) context.Context
	Logger *slog.Logger
}

func NewAccountProvisioning(deps AccountProvisioningDependencies) (*AccountProvisioning, error) {
	switch {
	case deps.Accounts == nil, deps.Logins == nil, deps.RoleStore == nil, deps.Roles == nil:
		return nil, fmt.Errorf("identity access account provisioning: stores are required")
	case deps.Policy == nil, deps.Identity == nil, deps.Profiles == nil:
		return nil, fmt.Errorf("identity access account provisioning: role policy, school identity and caregiver profiles are required")
	case deps.Passwords == nil, deps.Runtime == nil:
		return nil, fmt.Errorf("identity access account provisioning: password policy and tenant runtime are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountProvisioning{
		accounts: deps.Accounts, logins: deps.Logins, roleStore: deps.RoleStore, roles: deps.Roles,
		policy: deps.Policy, identity: deps.Identity, profiles: deps.Profiles, passwords: deps.Passwords,
		runtime: deps.Runtime, attach: deps.Attach, logger: logger,
	}, nil
}

// RegisterSchoolAccount creates the account, maps it to the school, assigns
// the role and provisions the school identity in one transaction.
//
// Without a tenant only the account is created, on the administrative
// transaction: that is the tenantless path the tests and the CLI take, and
// there is no mapping, role or identity to write.
func (s *AccountProvisioning) RegisterSchoolAccount(ctx context.Context, registration identityaccess.SchoolAccountRegistration) (_ identityaccess.ProvisionedAccount, err error) {
	if s == nil {
		return identityaccess.ProvisionedAccount{}, identityaccess.ErrAccountProvisioningUnavailable
	}
	defer func() { err = provisioningError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	request := domainSchoolAccountRegistration(registration)
	email, err := domain.NormalizeAccountEmail(request.Email)
	if err != nil {
		return identityaccess.ProvisionedAccount{}, failed(opRegisterAccount, err)
	}
	username := strings.TrimSpace(request.Username)

	if err := s.passwords.ValidatePasswordStrength(request.Password); err != nil {
		return identityaccess.ProvisionedAccount{}, failed(opRegisterAccount, err)
	}
	if err := s.ensureCredentialsFree(ctx, email, username); err != nil {
		return identityaccess.ProvisionedAccount{}, err
	}
	role, err := s.registrationRole(ctx, request)
	if err != nil {
		return identityaccess.ProvisionedAccount{}, err
	}
	hash, hashErr := s.passwords.HashPassword(request.Password)
	if hashErr != nil {
		return identityaccess.ProvisionedAccount{}, failed(opHashPassword, hashErr)
	}
	newAccount := domain.NewSchoolAccount{Email: email, Username: username, PasswordHash: hash}

	if request.TenantID <= 0 {
		var created domain.RegisteredAccount
		insertErr := s.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
			account, _, err := s.accounts.InsertSchoolAccount(txCtx, newAccount)
			created = account
			return err
		})
		if insertErr != nil {
			return identityaccess.ProvisionedAccount{}, insertErr
		}
		return publicProvisionedAccount(domain.ProvisionedAccount{Account: created}), nil
	}

	var result domain.ProvisionedAccount
	// WithTenantTx so RLS on auth.account_roles enforces tenant isolation in
	// the database: its WITH CHECK policy guarantees the inserted tenant_id
	// matches the transaction's tenant, so a code bug cannot silently create
	// a cross-tenant role assignment.
	err = s.runtime.WithTenantTx(ctx, request.TenantID, func(txCtx context.Context) error {
		var err error
		result, err = s.insertTenantAccount(txCtx, request, newAccount, role)
		return err
	})
	if err != nil {
		return identityaccess.ProvisionedAccount{}, err
	}
	return publicProvisionedAccount(result), nil
}

// LinkSchoolAccount gives an existing account access to the school and
// provisions the school identity its role requires, in one transaction. The
// credential is never touched.
func (s *AccountProvisioning) LinkSchoolAccount(ctx context.Context, link identityaccess.SchoolAccountLink) (_ identityaccess.ProvisionedAccount, err error) {
	if s == nil {
		return identityaccess.ProvisionedAccount{}, identityaccess.ErrAccountProvisioningUnavailable
	}
	defer func() { err = provisioningError(err) }()
	ctx = bindRuntime(s.attach, ctx)
	request := domainSchoolAccountLink(link)
	email := normalizeEmail(request.Email)
	if request.TenantID <= 0 {
		return identityaccess.ProvisionedAccount{}, failed(opLinkAccount, domain.ErrTenantRequiredForRoleAssignment)
	}

	// Same role policy as operator-led school access: no guardian (that is
	// the guardian invitation flow), no retired teacher role, and no role
	// belonging to a different school (#1021).
	var role *domain.ManagedRole
	if assignsRole(request.RoleID) {
		resolved, err := s.resolveAssignableRole(ctx, *request.RoleID, request.TenantID)
		if err != nil {
			return identityaccess.ProvisionedAccount{}, failed(opLinkAccount, err)
		}
		role = &resolved
	}

	account, found, _, err := findLoginAccountByEmail(ctx, s.runtime, s.logins, email)
	if err != nil || !found {
		return identityaccess.ProvisionedAccount{}, failed(opLinkAccount, domain.ErrAccountNotFound)
	}
	if !account.Active {
		return identityaccess.ProvisionedAccount{}, failed(opLinkAccount, identityaccess.ErrAccountInactive)
	}

	var identity *domain.SchoolIdentity
	err = s.runtime.WithTenantTx(ctx, request.TenantID, func(txCtx context.Context) error {
		var err error
		identity, err = s.linkTenantAccount(txCtx, request, account.ID, role)
		return err
	})
	if err != nil {
		// Reported as the caller's own error, unwrapped: prefixing "link
		// failed" onto a German sentence the operator is meant to read makes
		// it noise.
		if domain.IsSchoolIdentityRequestError(err) || errors.Is(err, identityaccess.ErrRoleLehrkraftCaregiverProfile) {
			return identityaccess.ProvisionedAccount{}, failed(opLinkAccount, err)
		}
		return identityaccess.ProvisionedAccount{}, failed(opLinkAccount, fmt.Errorf("link failed: %w", err))
	}

	s.logger.Info("account linked to tenant",
		slog.Int64("account_id", account.ID),
		slog.Int64("tenant_id", request.TenantID))

	return publicProvisionedAccount(domain.ProvisionedAccount{
		Account: domain.RegisteredAccount{
			ID: account.ID, Email: account.Email, Username: account.Username, Active: account.Active,
		},
		Identity: identity,
	}), nil
}

// ensureCredentialsFree refuses a registration for an address or a name that
// already belongs to an account, at any school: both columns are unique
// platform-wide.
func (s *AccountProvisioning) ensureCredentialsFree(ctx context.Context, email, username string) error {
	return globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		return s.ensureCredentialsFreeInTransaction(txCtx, email, username)
	})
}

func (s *AccountProvisioning) ensureCredentialsFreeInTransaction(ctx context.Context, email, username string) error {
	if _, found, _, err := findLoginAccountByEmail(ctx, s.runtime, s.logins, email); err != nil {
		return failed(opRegisterAccount, err)
	} else if found {
		return failed(opRegisterAccount, identityaccess.ErrEmailAlreadyExists)
	}
	if _, found, _, err := s.accounts.FindLoginAccountByUsername(ctx, username); err != nil {
		return failed(opRegisterAccount, err)
	} else if found {
		return failed(opRegisterAccount, identityaccess.ErrUsernameAlreadyExists)
	}
	return nil
}

// resolveAssignableRole reuses the caller's transaction. Opening an
// administrative transaction inside a tenant transaction is a privilege
// escalation and the runtime correctly rejects it; opening another one is
// unnecessary because auth.roles exposes system roles to every tenant and
// the policy rejects roles from another school.
func (s *AccountProvisioning) resolveAssignableRole(ctx context.Context, roleID, tenantID int64) (domain.ManagedRole, error) {
	lookup := func(lookupCtx context.Context) (domain.ManagedRole, error) {
		role, found, err := s.roles.FindRoleIgnoringTenant(lookupCtx, roleID)
		if err != nil {
			return domain.ManagedRole{}, err
		}
		if !found {
			// The policy owns the answer for a role that does not exist.
			return domain.ManagedRole{}, s.policy.ValidateAssignableSchoolRole(nil, tenantID)
		}
		if policyErr := s.policy.ValidateAssignableSchoolRole(role.Facts(), tenantID); policyErr != nil {
			return domain.ManagedRole{}, policyErr
		}
		return role, nil
	}
	if s.runtime.HasTransaction(ctx) {
		return lookup(ctx)
	}
	var role domain.ManagedRole
	err := s.runtime.WithAdminTx(ctx, func(adminCtx context.Context) error {
		resolved, lookupErr := lookup(adminCtx)
		role = resolved
		return lookupErr
	})
	return role, err
}

// provisionIdentity gives the account the person/staff chain its role
// requires. It runs inside the caller's tenant transaction, so the account,
// its role and its identity commit together or not at all. Without a role or
// without identity fields there is nothing to provision.
func (s *AccountProvisioning) provisionIdentity(
	ctx context.Context,
	accountID, tenantID int64,
	role *domain.ManagedRole,
	identity *domain.SchoolAccountIdentity,
) (*domain.SchoolIdentity, error) {
	if role == nil || identity == nil {
		return nil, nil
	}
	provisioned, err := s.identity.EnsureSchoolIdentity(ctx, domain.SchoolIdentityInput{
		AccountID:    accountID,
		TenantID:     tenantID,
		Role:         role.Facts(),
		FirstName:    identity.FirstName,
		LastName:     identity.LastName,
		TagID:        identity.TagID,
		CreatePerson: true,
	})
	if err != nil {
		return nil, failed(opProvisionIdentity, err)
	}
	return provisioned, nil
}

func assignsRole(roleID *int64) bool { return roleID != nil && *roleID > 0 }

func normalizeEmail(email string) string { return strings.TrimSpace(strings.ToLower(email)) }

func (s *AccountProvisioning) insertTenantAccount(txCtx context.Context, request domain.SchoolAccountRegistration, newAccount domain.NewSchoolAccount, role *domain.ManagedRole) (result domain.ProvisionedAccount, err error) {
	account, _, insertErr := s.accounts.InsertSchoolAccount(txCtx, newAccount)
	if insertErr != nil {
		return result, insertErr
	}
	if _, mappingErr := s.accounts.InsertTenantMappingIfAbsent(txCtx, account.ID, request.TenantID); mappingErr != nil {
		return result, fmt.Errorf("failed to create account-tenant mapping: %w", mappingErr)
	}
	if assignsRole(request.RoleID) {
		if _, _, roleErr := s.roleStore.AssignAccountRole(txCtx, account.ID, *request.RoleID, request.TenantID); roleErr != nil {
			return result, fmt.Errorf("failed to assign role to account: %w", roleErr)
		}
	}
	provisioned, identityErr := s.provisionIdentity(txCtx, account.ID, request.TenantID, role, request.Identity)
	if identityErr != nil {
		return result, identityErr
	}
	result = domain.ProvisionedAccount{Account: account, Identity: provisioned}
	return result, nil
}

func (s *AccountProvisioning) linkTenantAccount(txCtx context.Context, request domain.SchoolAccountLink, accountID int64, role *domain.ManagedRole) (identity *domain.SchoolIdentity, err error) {
	if _, mappingErr := s.accounts.InsertTenantMappingIfAbsent(txCtx, accountID, request.TenantID); mappingErr != nil {
		return identity, fmt.Errorf("failed to create account-tenant mapping: %w", mappingErr)
	}
	// Unlike a registration this account already exists, and it may
	// already carry an identity at this school — from an earlier link, or
	// from a revoke that deliberately left person/staff/teacher behind.
	// Handing it the Lehrkraft role while that identity holds a live
	// caregiver profile strands users.teachers and its group supervisions
	// under a JWT that only carries class_day permissions (#1772). The
	// same rule the tenant RBAC endpoint and both operator paths apply,
	// so this endpoint cannot be the way around them.
	if role != nil && s.policy.IsLehrkraftSystemRole(role.Facts()) {
		hasProfile, profileErr := s.profiles.HasLiveCaregiverProfile(txCtx, accountID)
		if profileErr != nil {
			return identity, profileErr
		}
		if hasProfile {
			return identity, identityaccess.ErrRoleLehrkraftCaregiverProfile
		}
	}
	if assignsRole(request.RoleID) {
		if _, _, roleErr := s.roleStore.AssignAccountRole(txCtx, accountID, *request.RoleID, request.TenantID); roleErr != nil {
			return identity, fmt.Errorf("failed to assign role to account: %w", roleErr)
		}
	}
	provisioned, identityErr := s.provisionIdentity(txCtx, accountID, request.TenantID, role, request.Identity)
	if identityErr != nil {
		return identity, identityErr
	}
	identity = provisioned
	return identity, nil
}

func provisioningIdentity(identity *identityaccess.SchoolAccountIdentity) *domain.SchoolAccountIdentity {
	if identity == nil {
		return nil
	}
	return &domain.SchoolAccountIdentity{FirstName: identity.FirstName, LastName: identity.LastName, TagID: identity.TagID}
}

func publicProvisionedAccount(provisioned domain.ProvisionedAccount) identityaccess.ProvisionedAccount {
	result := identityaccess.ProvisionedAccount{
		Account: identityaccess.RegisteredAccount{
			ID: provisioned.Account.ID, Email: provisioned.Account.Email,
			Username: provisioned.Account.Username, Active: provisioned.Account.Active,
			CreatedAt: provisioned.Account.CreatedAt, UpdatedAt: provisioned.Account.UpdatedAt,
			LastLogin: provisioned.Account.LastLogin,
		},
	}
	if provisioned.Identity != nil {
		result.Identity = &identityaccess.SchoolIdentity{
			PersonID: provisioned.Identity.PersonID, StaffID: provisioned.Identity.StaffID,
			TeacherID: provisioned.Identity.TeacherID,
		}
	}
	return result
}

var provisioningSentinels = []struct {
	internal error
	public   error
}{
	{identityaccess.ErrEmailAlreadyExists, identityaccess.ErrEmailAlreadyExists},
	{identityaccess.ErrUsernameAlreadyExists, identityaccess.ErrUsernameAlreadyExists},
	{domain.ErrTenantRequiredForRoleAssignment, identityaccess.ErrTenantRequiredForRoleAssignment},
}

// provisioningError translates a flow error to the public contract: the
// operation envelope keeps its text and the registration sentinels gain
// their public twin. Everything else falls through to the role mapping,
// which already carries the role, account and school identity sentinels
// these flows share.
func provisioningError(err error) error {
	if err == nil {
		return nil
	}
	var operation *OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: provisioningError(operation.Err)}
	}
	for _, sentinel := range provisioningSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return roleError(err)
}

// registrationRole resolves the role a registration assigns; a role needs a
// school.
func (s *AccountProvisioning) registrationRole(ctx context.Context, request domain.SchoolAccountRegistration) (*domain.ManagedRole, error) {
	if !assignsRole(request.RoleID) {
		return nil, nil
	}
	if request.TenantID <= 0 {
		return nil, failed(opRegisterAccount, domain.ErrTenantRequiredForRoleAssignment)
	}
	resolved, err := s.resolveAssignableRole(ctx, *request.RoleID, request.TenantID)
	if err != nil {
		return nil, failed(opRegisterAccount, err)
	}
	return &resolved, nil
}

func domainSchoolAccountRegistration(registration identityaccess.SchoolAccountRegistration) domain.SchoolAccountRegistration {
	return domain.SchoolAccountRegistration{
		TenantID: registration.TenantID, Email: registration.Email, Username: registration.Username,
		Password: registration.Password, RoleID: registration.RoleID, Identity: provisioningIdentity(registration.Identity),
	}
}

func domainSchoolAccountLink(link identityaccess.SchoolAccountLink) domain.SchoolAccountLink {
	return domain.SchoolAccountLink{
		TenantID: link.TenantID, Email: link.Email, RoleID: link.RoleID, Identity: provisioningIdentity(link.Identity),
	}
}
