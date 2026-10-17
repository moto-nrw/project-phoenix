package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The operator flows (#3252) need facts other owners hold and the retained
// rules the root still composes. The seams below are expressed in public
// values so the root can implement them without the module's internal
// vocabulary; this package adapts them to the consumer-owned ports.

// OperatorAudit appends operator actions to the Audit platform's
// platform-scoped ledger on the caller's transaction.
type OperatorAudit interface {
	RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error
}

// PasswordHasher hashes a new password and applies the strength policy.
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	ValidatePasswordStrength(password string) error
}

// OrganizationName is the organisation fact the access listing shows.
type OrganizationName struct {
	ID   int64
	Name string
}

// OrganizationDirectory reads the Organisation & Tenancy facts the access
// listing shows: the schools behind the mappings and the organisation
// names in the owner's name order.
type OrganizationDirectory interface {
	ListSchools(ctx context.Context, ids []int64) ([]identityaccess.School, error)
	ListOrganizationNames(ctx context.Context, ids []int64) ([]OrganizationName, error)
}

// AccountPersonIdentity is one person row that carries the account at any
// school, with the facts the name resolution decides on.
type AccountPersonIdentity struct {
	ID        int64
	TenantID  int64
	FirstName string
	LastName  string
	Deleted   bool
	IsStudent bool
}

// AccountIdentityFact reports whether a person and a staff record back the
// account at one school.
type AccountIdentityFact struct {
	TenantID  int64
	HasPerson bool
	HasStaff  bool
}

// SchoolIdentityRequest describes the identity chain a school access
// requires: person, staff and (for caregiver roles) teacher rows.
type SchoolIdentityRequest struct {
	AccountID    int64
	TenantID     int64
	Role         identityaccess.SchoolRole
	FirstName    string
	LastName     string
	Position     string
	CreatePerson bool
}

// SchoolIdentityProvisioner is the People Directory and School Membership
// identity chain a school access requires and the facts the listing shows
// about it. The tenant-scoped reads take the tenant from the context; the
// platform-wide reads run administratively. A request the caller can
// correct is reported as *identityaccess.InvalidInputError.
type SchoolIdentityProvisioner interface {
	HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error)
	ListAccountPersons(ctx context.Context, accountID int64) ([]AccountPersonIdentity, error)
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
	EnsureSchoolIdentity(ctx context.Context, request SchoolIdentityRequest) error
	ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]AccountIdentityFact, error)
}

// OperatorDependencies are the seams the operator flows need beyond the
// account-session dependencies they share (the password verifier, the JWT
// codec, the auth ledger, the schools and the tenant runtime).
type OperatorDependencies struct {
	Audit         OperatorAudit
	Passwords     PasswordHasher
	Organizations OrganizationDirectory
	Identities    SchoolIdentityProvisioner
	Logger        *slog.Logger
}

func newOperatorFlows(service *application.Service, store *postgres.Store, tokens *application.OperatorTokens, auth *application.AccountAuthentication, sessions *SessionDependencies, deps *OperatorDependencies, lifecycle *application.AccountLifecycle, mfa ports.OperatorMFAGate) (*application.OperatorAuthentication, *application.OperatorAccountAccess, error) {
	if deps == nil {
		return nil, nil, nil
	}
	if sessions == nil || auth == nil {
		return nil, nil, errors.New("identity access compose: the operator flows require the session dependencies")
	}
	switch {
	case deps.Audit == nil, deps.Passwords == nil,
		deps.Organizations == nil, deps.Identities == nil:
		return nil, nil, errors.New("identity access compose: every operator dependency is required")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	runtime := tenantRuntime{attach: attach, runner: tenant.NewTransactionRunner()}
	operatorAuth, err := application.NewOperatorAuthentication(service, application.OperatorAuthenticationDependencies{
		Passwords:   sessions.Passwords,
		Hasher:      deps.Passwords,
		Codec:       tokenCodec{sessions.Codec},
		MFA:         mfa,
		Audit:       operatorAudit{deps.Audit},
		Credentials: emailChangeRevocation{tokens},
		Runtime:     runtime,
		Rotation:    rotationPolicy{},
		Logger:      deps.Logger,
	})
	if err != nil {
		return nil, nil, err
	}
	accountAccess, err := application.NewOperatorAccountAccess(application.OperatorAccountAccessDependencies{
		Store:         store,
		Login:         store,
		Access:        store,
		Sessions:      auth,
		Schools:       schoolDirectory{sessions.Schools},
		Organizations: organizationDirectory{deps.Organizations},
		Identities:    schoolIdentityProvisioner{source: deps.Identities, lifecycle: lifecycle},
		Policy:        schoolRolePolicy{},
		Audit:         authAudit{sessions.Audit},
		OperatorAudit: operatorAudit{deps.Audit},
		Runtime:       runtime,
		Logger:        deps.Logger,
	})
	if err != nil {
		return nil, nil, err
	}
	return operatorAuth, accountAccess, nil
}

type operatorAudit struct{ source OperatorAudit }

func (a operatorAudit) RecordOperatorAction(ctx context.Context, entry domain.OperatorAuditEntry) error {
	return a.source.RecordOperatorAction(ctx, publicOperatorAuditEntry(entry))
}

// publicOperatorAuditEntry renders one ledger entry across the module
// boundary, with the typed evidence each action carries.
func publicOperatorAuditEntry(entry domain.OperatorAuditEntry) identityaccess.OperatorAuditEntry {
	public := identityaccess.OperatorAuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, IPAddress: entry.IPAddress,
	}
	if entry.RevokedSessions != nil {
		evidence := identityaccess.RevokedSessionsEvidence(*entry.RevokedSessions)
		public.RevokedSessions = &evidence
	}
	if entry.AccessChange != nil {
		change := identityaccess.OperatorAccessChange(*entry.AccessChange)
		public.AccessChange = &change
	}
	if entry.MFA != nil {
		evidence := identityaccess.OperatorMFAEvidence{Reason: entry.MFA.Reason, LockedUntil: entry.MFA.LockedUntil}
		if entry.MFA.Override != nil {
			override := identityaccess.OperatorMFAOverrideEvidence(*entry.MFA.Override)
			evidence.Override = &override
		}
		public.MFA = &evidence
	}
	return public
}

type organizationDirectory struct{ source OrganizationDirectory }

func (d organizationDirectory) ListSchools(ctx context.Context, ids []int64) ([]domain.School, error) {
	schools, err := d.source.ListSchools(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]domain.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, domain.School(school))
	}
	return result, nil
}

func (d organizationDirectory) ListOrganizationNames(ctx context.Context, ids []int64) ([]domain.OrganizationName, error) {
	organizations, err := d.source.ListOrganizationNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]domain.OrganizationName, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, domain.OrganizationName(organization))
	}
	return result, nil
}

type schoolIdentityProvisioner struct {
	source    SchoolIdentityProvisioner
	lifecycle *application.AccountLifecycle
}

func (p schoolIdentityProvisioner) HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error) {
	return p.source.HasLivePersonAtSchool(ctx, accountID)
}

func (p schoolIdentityProvisioner) ListAccountPersons(ctx context.Context, accountID int64) ([]domain.AccountPersonIdentity, error) {
	persons, err := p.source.ListAccountPersons(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AccountPersonIdentity, 0, len(persons))
	for _, person := range persons {
		result = append(result, domain.AccountPersonIdentity(person))
	}
	return result, nil
}

func (p schoolIdentityProvisioner) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return p.source.HasLiveCaregiverProfile(ctx, accountID)
}

func (p schoolIdentityProvisioner) EnsureSchoolIdentity(ctx context.Context, request domain.SchoolIdentityRequest) error {
	if p.lifecycle != nil {
		role := request.Role
		_, err := p.lifecycle.EnsureSchoolIdentity(ctx, domain.SchoolIdentityInput{
			AccountID: request.AccountID, TenantID: request.TenantID,
			Role: &domain.RoleFacts{
				ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
			},
			FirstName: request.FirstName, LastName: request.LastName, Position: request.Position, CreatePerson: request.CreatePerson,
		})
		if errors.Is(err, domain.ErrSchoolIdentityNamesRequired) || errors.Is(err, domain.ErrSchoolIdentityPersonIsStudent) {
			return &domain.InvalidInputError{Err: err}
		}
		return err
	}
	err := p.source.EnsureSchoolIdentity(ctx, SchoolIdentityRequest{
		AccountID: request.AccountID, TenantID: request.TenantID, Role: schoolRoleFact(request.Role),
		FirstName: request.FirstName, LastName: request.LastName, Position: request.Position, CreatePerson: request.CreatePerson,
	})
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return &domain.InvalidInputError{Err: invalid.Err}
	}
	return err
}

func (p schoolIdentityProvisioner) ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]domain.AccountIdentityFact, error) {
	facts, err := p.source.ListAccountIdentityFacts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AccountIdentityFact, 0, len(facts))
	for _, fact := range facts {
		result = append(result, domain.AccountIdentityFact(fact))
	}
	return result, nil
}

// schoolRolePolicy binds the public school-role policy every school-access
// path shares (#3314). A role with ID zero is one that does not exist; the
// validation reports why a role may not be handed out at the school with the
// message the operator sees.
type schoolRolePolicy struct{}

func (schoolRolePolicy) ValidateAssignableSchoolRole(role domain.RoleFact, tenantID int64) error {
	return identityaccess.ValidateAssignableSchoolRole(operatorRoleFacts(role), tenantID)
}

func (schoolRolePolicy) IsLehrkraftSystemRole(role domain.RoleFact) bool {
	return identityaccess.IsLehrkraftSystemRole(operatorRoleFacts(role))
}

// RoleNeedsStaffRecord classifies the fact as given, ID zero included, as the
// operator flows always did; only the lookup-shaped decisions treat ID zero
// as no role.
func (schoolRolePolicy) RoleNeedsStaffRecord(role domain.RoleFact) bool {
	return identityaccess.RoleNeedsStaffRecord(&identityaccess.RoleFacts{
		ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
	})
}

func (schoolRolePolicy) LehrkraftRoleImmutable() error {
	return identityaccess.ErrLehrkraftRoleImmutable
}

// operatorRoleFacts projects a resolved role; ID zero is no role at all.
func operatorRoleFacts(role domain.RoleFact) *identityaccess.RoleFacts {
	if role.ID <= 0 {
		return nil
	}
	return &identityaccess.RoleFacts{ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole}
}

func schoolRoleFact(role domain.RoleFact) identityaccess.SchoolRole {
	return identityaccess.SchoolRole{ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole}
}

// Engine methods: every operator flow answers
// ErrOperatorAuthenticationUnavailable when the module was composed without
// operator dependencies.

// emailChangeRevocation spends the operator's pending e-mail change links
// when the password changes. The module owns both, so the revocation joins
// the password change's transaction (#2722).
type emailChangeRevocation struct{ tokens *application.OperatorTokens }

func (r emailChangeRevocation) InvalidateEmailChangeTokens(ctx context.Context, operatorID int64) error {
	return r.tokens.RevokeEmailChanges(ctx, operatorID)
}
