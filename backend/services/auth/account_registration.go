package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Register creates a new user account without provisioning a school identity.
// Callers that provision the person/staff chain themselves (operator account
// creation) use this; everything that creates staff should use
// RegisterSchoolAccount so account and identity land in one transaction.
func (s *Service) Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*auth.Account, error) {
	account, _, err := s.RegisterSchoolAccount(ctx, email, username, password, roleID, tenantID, nil)
	return account, err
}

// RegisterSchoolAccount creates an account, maps it to the school, assigns the
// role and provisions the school identity (person → staff → caregiver profile)
// in ONE transaction.
//
// identity is optional. With it, the account is staff at the school the moment
// the transaction commits; without it, only the account is created and the
// caller owns the identity. Splitting those two across separate requests is
// what leaves accounts that hold a role and are not staff (#2222).
func (s *Service) RegisterSchoolAccount(
	ctx context.Context,
	email, username, password string,
	roleID *int64,
	tenantID int64,
	identity *SchoolAccountIdentity,
) (*auth.Account, *SchoolIdentity, error) {
	// Validate and normalize registration inputs
	if err := s.validateRegistrationInputs(ctx, email, username, password); err != nil {
		return nil, nil, err
	}

	if roleID != nil && *roleID > 0 && tenantID <= 0 {
		return nil, nil, &AuthError{Op: "register", Err: ErrTenantRequiredForRoleAssignment}
	}
	var role *auth.Role
	if roleID != nil && *roleID > 0 {
		var err error
		role, err = s.resolveAssignableSchoolRole(ctx, *roleID, tenantID)
		if err != nil {
			return nil, nil, &AuthError{Op: "register", Err: err}
		}
	}

	// Create account object with hashed password
	account, err := s.createAccountObject(email, username, password)
	if err != nil {
		return nil, nil, err
	}

	// Persist account, assign role and provision the identity in one transaction
	schoolIdentity, err := s.persistAccountWithRole(ctx, account, role, roleID, tenantID, identity)
	if err != nil {
		return nil, nil, err
	}

	return account, schoolIdentity, nil
}

// validateRegistrationInputs validates registration data and checks for conflicts
func (s *Service) validateRegistrationInputs(ctx context.Context, email, username, password string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if err := ValidatePasswordStrength(password); err != nil {
		return &AuthError{Op: "register", Err: err}
	}

	// Check if email already exists
	if _, err := s.repos.Account.FindByEmail(ctx, email); err == nil {
		return &AuthError{Op: "register", Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	if _, err := s.repos.Account.FindByUsername(ctx, username); err == nil {
		return &AuthError{Op: "register", Err: ErrUsernameAlreadyExists}
	}

	return nil
}

// createAccountObject creates a new account with hashed password
func (s *Service) createAccountObject(email, username, password string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: opHashPassword, Err: err}
	}

	usernamePtr := &username
	now := time.Now()

	return &auth.Account{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
		LastLogin:    &now,
	}, nil
}

// persistAccountWithRole saves account, maps it to a tenant, and assigns a role.
// Uses WithTenantTx so that RLS on auth.account_roles enforces tenant isolation
// at the database level. phoenix_tenant has CRUD on all tables in the auth schema
// (including auth.accounts which has no RLS), so no admin escalation is needed.
// The WITH CHECK policy on auth.account_roles guarantees the inserted tenant_id
// matches the transaction's app.current_tenant_id — a code bug cannot silently
// create cross-tenant role assignments.
func (s *Service) persistAccountWithRole(
	ctx context.Context,
	account *auth.Account,
	role *auth.Role,
	roleID *int64,
	tenantID int64,
	identity *SchoolAccountIdentity,
) (*SchoolIdentity, error) {
	if tenantID <= 0 {
		// No tenant context (e.g. tests) — fall back to admin tx for the account insert only.
		return nil, tenant.WithAdminTx(s.withTenantRuntime(ctx), s.db, func(ctx context.Context, tx bun.Tx) error {
			return s.repos.Account.Create(ctx, account)
		})
	}

	var schoolIdentity *SchoolIdentity
	err := tenant.WithTenantTx(s.withTenantRuntime(ctx), s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {

		// Create account (auth.accounts has no tenant_id, no RLS — plain INSERT)
		if err := s.repos.Account.Create(ctx, account); err != nil {
			return err
		}

		// Map account to tenant so the user can log into this school
		now := time.Now()
		mapping := &auth.AccountTenant{
			AccountID:   account.ID,
			TenantID:    tenantID,
			Status:      auth.AccountTenantStatusActive,
			ActivatedAt: &now,
		}
		if err := s.repos.AccountTenant.Create(ctx, mapping); err != nil {
			return fmt.Errorf("failed to create account-tenant mapping: %w", err)
		}

		// Assign role scoped to this tenant (RLS WITH CHECK enforces tenant_id match)
		if roleID != nil && *roleID > 0 {
			accountRole := &auth.AccountRole{
				AccountID: account.ID,
				RoleID:    *roleID,
			}
			accountRole.SetTenantID(tenantID)
			if err := s.repos.AccountRole.Create(ctx, accountRole); err != nil {
				return fmt.Errorf("failed to assign role to account: %w", err)
			}
		}

		provisioned, err := s.provisionSchoolIdentity(ctx, account.ID, tenantID, role, identity)
		if err != nil {
			return err
		}
		schoolIdentity = provisioned

		return nil
	})
	return schoolIdentity, err
}

// SchoolAccountIdentity carries the person fields needed to make an account
// staff at a school. There is no other source for them: an account holds an
// email and a username, never a person's name.
type SchoolAccountIdentity struct {
	FirstName string
	LastName  string
	TagID     *string
}

// provisionSchoolIdentity gives the account the person/staff chain its role
// requires. Runs inside the caller's tenant transaction, so the account, its
// role and its identity commit together or not at all.
func (s *Service) provisionSchoolIdentity(
	ctx context.Context,
	accountID, tenantID int64,
	role *auth.Role,
	identity *SchoolAccountIdentity,
) (*SchoolIdentity, error) {
	if identity == nil || role == nil {
		return nil, nil
	}
	provisioning, err := s.schoolIdentity("provision school identity")
	if err != nil {
		return nil, err
	}
	provisioned, err := provisioning.EnsureSchoolIdentity(ctx, SchoolIdentityInput{
		AccountID:    accountID,
		TenantID:     tenantID,
		Role:         RoleFactsOf(role),
		FirstName:    identity.FirstName,
		LastName:     identity.LastName,
		TagID:        identity.TagID,
		CreatePerson: true,
	})
	if err != nil {
		return nil, &AuthError{Op: "provision school identity", Err: err}
	}
	return provisioned, nil
}

// LinkAccountToTenant links an existing account to a tenant with an optional role assignment.
// The password is NOT changed — the user keeps their current credentials.
// Returns ErrAccountNotFound if no account exists with the given email.
// Returns ErrAccountInactive if the account is deactivated.
func (s *Service) LinkAccountToTenant(ctx context.Context, email string, roleID *int64, tenantID int64) (*auth.Account, error) {
	account, _, err := s.LinkSchoolAccount(ctx, email, roleID, tenantID, nil)
	return account, err
}

// LinkSchoolAccount links an existing account to a school and provisions the
// school identity its role requires, in one transaction. identity is optional
// with the same meaning as in RegisterSchoolAccount: without it the account is
// linked but not made staff, which only makes sense when the caller provisions
// the chain itself.
func (s *Service) LinkSchoolAccount(
	ctx context.Context,
	email string,
	roleID *int64,
	tenantID int64,
	identity *SchoolAccountIdentity,
) (*auth.Account, *SchoolIdentity, error) {
	const op = "link-to-tenant"
	email = strings.TrimSpace(strings.ToLower(email))

	if tenantID <= 0 {
		return nil, nil, &AuthError{Op: op, Err: ErrTenantRequiredForRoleAssignment}
	}

	// Same role policy as operator-led school access: no guardian (that is the
	// guardian invitation flow), no retired teacher role, and no role belonging
	// to a different school (issue #1021).
	var role *auth.Role
	if roleID != nil && *roleID > 0 {
		var err error
		role, err = s.resolveAssignableSchoolRole(ctx, *roleID, tenantID)
		if err != nil {
			return nil, nil, &AuthError{Op: op, Err: err}
		}
	}

	// Find existing account
	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		return nil, nil, &AuthError{Op: op, Err: ErrAccountNotFound}
	}

	if !account.Active {
		return nil, nil, &AuthError{Op: op, Err: ErrAccountInactive}
	}

	// Link to tenant (idempotent — handles already-linked case)
	schoolIdentity, err := s.performAccountTenantLink(ctx, account, role, roleID, tenantID, identity)
	if err != nil {
		// Reported as the caller's own error, unwrapped: prefixing "link failed"
		// onto a German sentence the operator is meant to read makes it noise.
		if IsSchoolIdentityRequestError(err) || errors.Is(err, ErrRoleLehrkraftCaregiverProfile) {
			return nil, nil, &AuthError{Op: op, Err: err}
		}
		return nil, nil, &AuthError{Op: op, Err: fmt.Errorf("link failed: %w", err)}
	}

	s.getLogger().Info("account linked to tenant",
		slog.Int64("account_id", account.ID),
		slog.Int64("tenant_id", tenantID))

	return account, schoolIdentity, nil
}

// resolveAssignableSchoolRole reuses an ambient request or operator
// transaction. Opening an admin transaction inside a tenant transaction is a
// privilege escalation and the runtime correctly rejects it; opening another
// transaction is unnecessary because auth.roles exposes system roles to every
// tenant and the policy validator rejects roles from another school.
func (s *Service) resolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*auth.Role, error) {
	lookup := func(lookupCtx context.Context) (*auth.Role, error) {
		return validateAssignableSchoolRole(lookupCtx, s.repos.Role, s.lifecycle, roleID, tenantID)
	}

	if tx, ok := tenant.TransactionFromContext(ctx); ok && tx != nil {
		return lookup(ctx)
	}

	var role *auth.Role
	err := tenant.WithAdminTxOrDirect(s.withTenantRuntime(ctx), s.db, func(adminCtx context.Context) error {
		var lookupErr error
		role, lookupErr = lookup(adminCtx)
		return lookupErr
	})
	return role, err
}

// performAccountTenantLink creates a tenant mapping, role assignment and school
// identity for an existing account.
func (s *Service) performAccountTenantLink(
	ctx context.Context,
	account *auth.Account,
	role *auth.Role,
	roleID *int64,
	tenantID int64,
	identity *SchoolAccountIdentity,
) (*SchoolIdentity, error) {
	var schoolIdentity *SchoolIdentity
	err := tenant.WithTenantTx(s.withTenantRuntime(ctx), s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {

		if err := s.ensureTenantMapping(ctx, account.ID, tenantID); err != nil {
			return err
		}
		// Unlike /auth/register this account already exists, and it may already
		// carry an identity at this school — from an earlier link, or from a
		// revoke that deliberately left person/staff/teacher behind. Handing it
		// the Lehrkraft role while that identity holds a live caregiver profile
		// strands users.teachers and its group supervisions under a JWT that
		// only carries class_day permissions (#1772). The same rule the tenant
		// RBAC endpoint and both operator paths apply, so this endpoint cannot
		// be the way around them.
		if isLehrkraftRole(s.lifecycle, role) {
			hasProfile, profErr := s.lifecycle.HasLiveCaregiverProfile(ctx, account.ID)
			if profErr != nil {
				return profErr
			}
			if hasProfile {
				return ErrRoleLehrkraftCaregiverProfile
			}
		}
		if err := s.ensureRoleAssignment(ctx, account.ID, roleID, tenantID); err != nil {
			return err
		}
		provisioned, err := s.provisionSchoolIdentity(ctx, account.ID, tenantID, role, identity)
		if err != nil {
			return err
		}
		schoolIdentity = provisioned
		return nil
	})
	return schoolIdentity, err
}

// ensureTenantMapping creates an account-tenant mapping if one does not already exist.
func (s *Service) ensureTenantMapping(ctx context.Context, accountID, tenantID int64) error {
	now := time.Now()
	mapping := &auth.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      auth.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.repos.AccountTenant.Create(ctx, mapping); err != nil {
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("failed to create account-tenant mapping: %w", err)
		}
	}
	return nil
}

// ensureRoleAssignment assigns a role to an account for a tenant, ignoring duplicates.
func (s *Service) ensureRoleAssignment(ctx context.Context, accountID int64, roleID *int64, tenantID int64) error {
	if roleID == nil || *roleID <= 0 {
		return nil
	}
	accountRole := &auth.AccountRole{
		AccountID: accountID,
		RoleID:    *roleID,
	}
	accountRole.SetTenantID(tenantID)
	if err := s.repos.AccountRole.Create(ctx, accountRole); err != nil {
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("failed to assign role to account: %w", err)
		}
	}
	return nil
}

// isDuplicateKeyError checks if a database error is a unique constraint violation (PG code 23505).
func isDuplicateKeyError(err error) bool {
	return modelBase.IsUniqueViolation(err)
}

// Registration outcomes.
var (
	// ErrEmailAlreadyExists returned when email is already registered
	ErrEmailAlreadyExists = errors.New("Diese E-Mail-Adresse ist bereits registriert") //nolint:staticcheck // ST1005: user-facing German message

	// ErrUsernameAlreadyExists returned when username is already taken
	ErrUsernameAlreadyExists = errors.New("Dieser Benutzername ist bereits vergeben") //nolint:staticcheck // ST1005: user-facing German message

	// ErrTenantRequiredForRoleAssignment returned when tenant-scoped role setup is requested without a tenant context
	ErrTenantRequiredForRoleAssignment = errors.New("tenant context is required when assigning a role during registration")
)

// RegistrationOperations create accounts and link them to schools.
type RegistrationOperations interface {
	Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*auth.Account, error)
	// RegisterSchoolAccount is Register plus the school identity (person →
	// staff → caregiver profile) the role requires, in one transaction (#2222).
	RegisterSchoolAccount(ctx context.Context, email, username, password string, roleID *int64, tenantID int64, identity *SchoolAccountIdentity) (*auth.Account, *SchoolIdentity, error)
	// Multi-Tenant Account Linking
	LinkAccountToTenant(ctx context.Context, email string, roleID *int64, tenantID int64) (*auth.Account, error)
	// LinkSchoolAccount is LinkAccountToTenant plus the school identity the
	// role requires, in one transaction (#2222).
	LinkSchoolAccount(ctx context.Context, email string, roleID *int64, tenantID int64, identity *SchoolAccountIdentity) (*auth.Account, *SchoolIdentity, error)
}
