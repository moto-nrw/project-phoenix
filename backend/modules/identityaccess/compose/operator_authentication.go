package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The operator flows (#3252) need facts other owners hold and the retained
// rules the root still composes. The seams below are expressed in public
// values so the root can implement them without the module's internal
// vocabulary; this package adapts them to the consumer-owned ports.

// OperatorMFAGate is the retained operator MFA service as login consults
// it. Configured false means the gate is not wired and token pairs are
// issued directly.
type OperatorMFAGate interface {
	Configured() bool
	HasEnrollment(ctx context.Context, operatorID int64) (bool, error)
	VerifyTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error)
	StartChallenge(ctx context.Context, operatorID int64, ipAddress string) (string, error)
	TrustedDeviceDays() int
}

// OperatorAudit appends operator actions to the Audit platform's
// platform-scoped ledger on the caller's transaction.
type OperatorAudit interface {
	RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error
}

// OperatorCredentialCleanup invalidates the pending e-mail change links a
// password rotation must not leave alive.
type OperatorCredentialCleanup interface {
	InvalidateEmailChangeTokens(ctx context.Context, operatorID int64) error
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
	MFA           OperatorMFAGate
	Audit         OperatorAudit
	Credentials   OperatorCredentialCleanup
	Passwords     PasswordHasher
	Organizations OrganizationDirectory
	Identities    SchoolIdentityProvisioner
	Logger        *slog.Logger
}

func newOperatorFlows(service *application.Service, store *postgres.Store, auth *application.AccountAuthentication, sessions *SessionDependencies, deps *OperatorDependencies, lifecycle *application.AccountLifecycle) (*application.OperatorAuthentication, *application.OperatorAccountAccess, error) {
	if deps == nil {
		return nil, nil, nil
	}
	if sessions == nil || auth == nil {
		return nil, nil, errors.New("identity access compose: the operator flows require the session dependencies")
	}
	switch {
	case deps.MFA == nil, deps.Audit == nil, deps.Credentials == nil, deps.Passwords == nil,
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
		MFA:         deps.MFA,
		Audit:       operatorAudit{deps.Audit},
		Credentials: deps.Credentials,
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
	return a.source.RecordOperatorAction(ctx, public)
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

var errOperatorAuthenticationUnavailable = identityaccess.ErrOperatorAuthenticationUnavailable

func (e engine) LoginOperatorWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*identityaccess.OperatorLoginResult, error) {
	if e.operatorAuth == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	result, err := e.operatorAuth.LoginWithMFAGate(e.attach(ctx), email, password, ipAddress, userAgent, trustedDeviceCookie)
	return operatorLoginResult(result), operatorError(err)
}

func (e engine) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	if e.operatorAuth == nil {
		return "", "", errOperatorAuthenticationUnavailable
	}
	access, refresh, err := e.operatorAuth.IssueTokensForAuthenticatedOperator(e.attach(ctx), operatorID, ipAddress, userAgent)
	return access, refresh, operatorError(err)
}

func (e engine) RefreshOperatorToken(ctx context.Context, operatorID int64, refreshToken string) (string, string, error) {
	if e.operatorAuth == nil {
		return "", "", errOperatorAuthenticationUnavailable
	}
	access, refresh, err := e.operatorAuth.RefreshToken(e.attach(ctx), operatorID, refreshToken)
	return access, refresh, operatorError(err)
}

func (e engine) UpdateOperatorProfile(ctx context.Context, operatorID int64, displayName string) (identityaccess.Operator, error) {
	if e.operatorAuth == nil {
		return identityaccess.Operator{}, errOperatorAuthenticationUnavailable
	}
	operator, err := e.operatorAuth.UpdateProfile(e.attach(ctx), operatorID, displayName)
	return identityaccess.Operator(operator), operatorError(err)
}

func (e engine) ChangeOperatorPassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	if e.operatorAuth == nil {
		return errOperatorAuthenticationUnavailable
	}
	return operatorError(e.operatorAuth.ChangePassword(e.attach(ctx), operatorID, currentPassword, newPassword))
}

func (e engine) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.ListAccountTenantAccess(e.attach(ctx), accountID)
	return accountTenantAccess(entries), operatorError(err)
}

func (e engine) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]identityaccess.AccountTenantRole, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	roles, err := e.accountAccess.ListAssignableSchoolRoles(e.attach(ctx), schoolID)
	return accountTenantRoles(roles), operatorError(err)
}

func (e engine) GrantAccountTenantAccess(ctx context.Context, request identityaccess.GrantAccountTenantAccess) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.GrantAccountTenantAccess(e.attach(ctx), domain.GrantAccountTenantAccess(request))
	return accountTenantAccess(entries), operatorError(err)
}

func (e engine) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.UpdateAccountTenantRole(e.attach(ctx), accountID, schoolID, roleID, operatorID, clientIP)
	return accountTenantAccess(entries), operatorError(err)
}

func (e engine) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP string) ([]identityaccess.AccountTenantAccess, error) {
	if e.accountAccess == nil {
		return nil, errOperatorAuthenticationUnavailable
	}
	entries, err := e.accountAccess.RevokeAccountTenantAccess(e.attach(ctx), accountID, schoolID, operatorID, clientIP)
	return accountTenantAccess(entries), operatorError(err)
}

func operatorLoginResult(value *domain.OperatorLoginResult) *identityaccess.OperatorLoginResult {
	if value == nil {
		return nil
	}
	result := identityaccess.OperatorLoginResult{
		Status: identityaccess.LoginStatus(value.Status), AccessToken: value.AccessToken, RefreshToken: value.RefreshToken,
		ChallengeToken: value.ChallengeToken, MaskedEmail: value.MaskedEmail, MFAEnrollmentRequired: value.MFAEnrollmentRequired,
		TrustedDeviceEnabled: value.TrustedDeviceEnabled, TrustedDeviceDays: value.TrustedDeviceDays,
	}
	if value.Operator != nil {
		operator := identityaccess.Operator(*value.Operator)
		result.Operator = &operator
	}
	return &result
}

func accountTenantRoles(values []domain.AccountTenantRole) []identityaccess.AccountTenantRole {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.AccountTenantRole, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.AccountTenantRole(value))
	}
	return result
}

func accountTenantAccess(values []domain.AccountTenantAccess) []identityaccess.AccountTenantAccess {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.AccountTenantAccess, 0, len(values))
	for _, value := range values {
		roles := accountTenantRoles(value.Roles)
		if roles == nil {
			roles = []identityaccess.AccountTenantRole{}
		}
		result = append(result, identityaccess.AccountTenantAccess{
			TenantID: value.TenantID, SchoolName: value.SchoolName, SchoolSlug: value.SchoolSlug, SchoolActive: value.SchoolActive,
			OrganizationID: value.OrganizationID, OrganizationName: value.OrganizationName, Status: value.Status,
			ActivatedAt: value.ActivatedAt, DeactivatedAt: value.DeactivatedAt, HasPerson: value.HasPerson, HasStaff: value.HasStaff,
			Roles: roles,
		})
	}
	return result
}

var operatorSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrOperatorInvalidCredentials, identityaccess.ErrOperatorInvalidCredentials},
	{domain.ErrOperatorInactive, identityaccess.ErrOperatorInactive},
	{domain.ErrOperatorNotFound, identityaccess.ErrOperatorNotFound},
	{domain.ErrOperatorRefreshTokenInvalid, identityaccess.ErrOperatorRefreshTokenInvalid},
	{domain.ErrOperatorPasswordMismatch, identityaccess.ErrOperatorPasswordMismatch},
	{domain.ErrAccountTenantAccessNotFound, identityaccess.ErrAccountTenantAccessNotFound},
	{domain.ErrAccountTenantAccessExists, identityaccess.ErrAccountTenantAccessExists},
	{domain.ErrSchoolNotFound, identityaccess.ErrSchoolNotFound},
	{domain.ErrSchoolDeleted, identityaccess.ErrSchoolDeleted},
}

// operatorError translates an operator flow error to the public contract:
// an input rejection keeps its message under the public type, an operator
// sentinel gains its public counterpart, and everything else falls through
// to the account-authentication translation the access flows share.
func operatorError(err error) error {
	if err == nil {
		return nil
	}
	if invalid, ok := errors.AsType[*domain.InvalidInputError](err); ok {
		return &identityaccess.InvalidInputError{Err: invalid.Err}
	}
	for _, sentinel := range operatorSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return authenticationError(err)
}
