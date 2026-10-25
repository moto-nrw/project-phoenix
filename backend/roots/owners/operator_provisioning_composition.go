package owners

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// newOperatorProvisioning composes Organisation & Tenancy's provisioning
// (#3253) over the organisations, the Identity & Access module and account
// directory, the adapters NewOperatorProvisioningAdapters bound and the
// settings.
func newOperatorProvisioning(
	organizations organizationtenancy.Capability, identity identityaccess.Owner, accounts OperatorAccountDirectory,
	adapters OperatorProvisioningAdapters, settings settingsCompose.SettingsService, logger *slog.Logger,
) (organizationtenancy.Provisioning, error) {
	return organizationCompose.NewProvisioning(organizationCompose.ProvisioningDependencies{
		PWAUsageWindowDays: deliveryCompose.PWAUsageWindowDays,
		Organizations:      organizations,
		Identity: ProvisioningIdentity{
			Sessions: identity, Invitations: identity, Provisioning: identity,
			Administration: identity, SchoolIdentity: identity, Roles: identity, Accounts: accounts,
			Traeger: identity,
		},
		Devices:    adapters.Devices,
		People:     adapters.People,
		Presence:   adapters.Presence,
		Categories: adapters.Categories,
		Settings:   provisioningSettings{settings: settings},
		Audit:      adapters.Audit,
		Secrets:    provisioningSecrets{},
		Logger:     logger,
	}, newOperatorDashboard(adapters.ActiveAccountCounts))
}

// provisioningSecrets draws the provisioning secrets from Security Runtime.
type provisioningSecrets struct{}

// APIKey returns a fresh device API key.
func (provisioningSecrets) APIKey() (string, error) { return securityruntime.DeviceAPIKey() }

// UsernameSuffix draws the owner's username suffix from Security Runtime's
// entropy.
func (provisioningSecrets) UsernameSuffix() string {
	return identityaccess.ProvisioningUsernameSuffix(securityruntime.RandomLowerAlphanumeric)
}

type ProvisioningRoles interface {
	identityaccess.RoleCommand
	ListRoles(context.Context, identityaccess.RoleFilter) ([]identityaccess.Role, error)
	GetRole(context.Context, int64) (identityaccess.Role, error)
}

// ProvisioningIdentity binds provisioning to public identity capabilities.
type ProvisioningIdentity struct {
	Accounts       OperatorAccountDirectory
	Sessions       identityaccess.AccountSessionMaintenance
	Invitations    identityaccess.SchoolInvitations
	Provisioning   identityaccess.AccountProvisioning
	Administration identityaccess.AccountAdministration
	SchoolIdentity identityaccess.SchoolIdentityProvisioning
	Roles          ProvisioningRoles
	Traeger        ProvisioningTraegerIdentity
}

// ProvisioningTraegerIdentity is the account lookup, the Träger roles and the
// participating reset of the second factor the operator's Träger-Büro
// management uses (ADR 0084, #4651).
type ProvisioningTraegerIdentity interface {
	identityaccess.GuardianAccessQuery
	identityaccess.TraegerRoles
	ResetMFAInTransaction(ctx context.Context, accountID int64) error
}

// provisioningTenantScope scopes an operator's identity write to the school
// it provisions; the operator's own context carries no tenant.
var provisioningTenantScope identityaccessCompose.TenantUnitOfWork

func (p ProvisioningIdentity) ListSystemRoles(ctx context.Context) ([]organizationCompose.ProvisioningRole, error) {
	var roles []identityaccess.Role
	err := provisioningRoleRead(ctx, func(txCtx context.Context) error {
		var err error
		roles, err = p.Roles.ListRoles(txCtx, identityaccess.RoleFilter{})
		return err
	})
	if err != nil {
		return nil, err
	}
	result := make([]organizationCompose.ProvisioningRole, 0, len(roles))
	for _, role := range identityaccess.SystemRoles(roles) {
		result = append(result, p.role(role.ID, role.Name, role.IsSystem, role.TenantID, role.BaseRole))
	}
	return result, nil
}

// FindSystemRole resolves the owner's platform system role with that name.
func (p ProvisioningIdentity) FindSystemRole(ctx context.Context, name string) (organizationCompose.ProvisioningRole, bool, error) {
	var roles []identityaccess.Role
	err := provisioningRoleRead(ctx, func(txCtx context.Context) error {
		var err error
		roles, err = p.Roles.ListRoles(txCtx, identityaccess.RoleFilter{})
		return err
	})
	if err != nil {
		return organizationCompose.ProvisioningRole{}, false, err
	}
	role, found := identityaccess.PlatformSystemRole(roles, name)
	if !found {
		return organizationCompose.ProvisioningRole{}, false, nil
	}
	return p.role(role.ID, role.Name, role.IsSystem, role.TenantID, role.BaseRole), true, nil
}

// FindRole reports a missing role as not found rather than as a failure, so
// provisioning answers 400 for an unknown role_id instead of 500.
func (p ProvisioningIdentity) FindRole(ctx context.Context, id int64) (organizationCompose.ProvisioningRole, bool, error) {
	var role identityaccess.Role
	err := provisioningRoleRead(ctx, func(txCtx context.Context) error {
		var err error
		role, err = p.Roles.GetRole(txCtx, id)
		return err
	})
	if errors.Is(err, identityaccess.ErrRoleNotFound) {
		return organizationCompose.ProvisioningRole{}, false, nil
	}
	if err != nil {
		return organizationCompose.ProvisioningRole{}, false, err
	}
	return p.role(role.ID, role.Name, role.IsSystem, role.TenantID, role.BaseRole), true, nil
}

func (p ProvisioningIdentity) role(id int64, name string, system bool, tenantID *int64, baseRole *string) organizationCompose.ProvisioningRole {
	facts := &identityaccess.RoleFacts{ID: id, TenantID: tenantID, Name: name, IsSystem: system, BaseRole: baseRole}
	return organizationCompose.ProvisioningRole{
		ID: id, Name: name, IsSystem: system, TenantID: tenantID, BaseRole: baseRole,
		Lehrkraft:            identityaccess.IsLehrkraftRole(facts),
		CaregiverPermissions: identityaccess.IsPlatformCaregiverRole(facts),
	}
}

// InviteSchoolAdmin hands out the school admin role as Identity & Access's
// operator invitation; the caller is operator-authenticated (platform scope,
// no tenant permission set).
func (p ProvisioningIdentity) InviteSchoolAdmin(ctx context.Context, request organizationCompose.SchoolAdminInvitationRequest) (organizationCompose.SchoolAdminInvitation, error) {
	invitation, err := p.Invitations.CreateSchoolInvitation(provisioningTenantScope.WithTenantID(ctx, request.TenantID), identityaccess.OperatorSchoolInvitation(
		request.Email, request.RoleID, request.TenantID,
		request.FirstName, request.LastName, request.Position, request.CaregiverEnabled,
	))
	if err != nil {
		// The operator routes classify on the provisioning capability's own
		// contract; they may not name the owner's (#3364).
		return organizationCompose.SchoolAdminInvitation{}, provisioningIdentityError(err)
	}
	return organizationCompose.SchoolAdminInvitation{
		ID: invitation.ID, Email: invitation.Email, RoleID: invitation.RoleID, RoleName: invitation.RoleName,
		Token: invitation.Token, ExpiresAt: invitation.ExpiresAt, FirstName: invitation.FirstName,
		LastName: invitation.LastName, Position: invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled,
		CreatedBy: invitation.CreatedBy, CreatorEmail: invitation.CreatorEmail,
		EmailSentAt: invitation.Delivery.SentAt, EmailError: invitation.Delivery.Error,
		EmailRetryCount: invitation.Delivery.RetryCount,
	}, nil
}

func (p ProvisioningIdentity) RegisterSchoolAccount(ctx context.Context, registration organizationCompose.SchoolAccountRegistration) (organizationCompose.CreatedAccount, error) {
	roleID := registration.RoleID
	// The operator routes classify on the provisioning capability's contract
	// and may not name the owner's, so the root translates for them (#3364).
	provisioned, err := p.Provisioning.RegisterSchoolAccount(provisioningTenantScope.WithTenantID(ctx, registration.TenantID),
		identityaccess.SchoolAccountRegistration{
			TenantID: registration.TenantID, Email: registration.Email, Username: registration.Username,
			Password: registration.Password, RoleID: &roleID,
		})
	if err != nil {
		return organizationCompose.CreatedAccount{}, provisioningIdentityError(err)
	}
	account := provisioned.Account
	username := account.Username
	// The identity chain is provisioned by the caller's next step, and a
	// fresh account carries neither an avatar nor a one-time password.
	return organizationCompose.CreatedAccount{
		ID: account.ID, CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt, Email: account.Email,
		Username: &username, Active: account.Active, LastLogin: account.LastLogin,
	}, nil
}

func (p ProvisioningIdentity) EnsureSchoolIdentity(ctx context.Context, request organizationCompose.SchoolIdentityRequest) error {
	role := request.Role
	_, err := p.SchoolIdentity.EnsureSchoolIdentity(provisioningTenantScope.WithTenantID(ctx, request.TenantID), identityaccess.SchoolIdentityInput{
		AccountID: request.AccountID,
		TenantID:  request.TenantID,
		Role: &identityaccess.RoleFacts{
			ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
		},
		FirstName:        request.FirstName,
		LastName:         request.LastName,
		Position:         request.Position,
		CaregiverUpgrade: request.CaregiverUpgrade,
		CreatePerson:     identityaccess.OperatorProvisioningCreatesPerson,
	})
	if errors.Is(err, identityaccess.ErrSchoolIdentityNamesRequired) || errors.Is(err, identityaccess.ErrSchoolIdentityPersonIsStudent) {
		return invalidSchoolIdentityError{err: err}
	}
	return err
}

func (p ProvisioningIdentity) AssignRole(ctx context.Context, tenantID, accountID, roleID int64) error {
	return provisioningIdentityError(p.Roles.AssignRoleToAccount(provisioningTenantScope.WithTenantID(ctx, tenantID), accountID, roleID))
}

func (p ProvisioningIdentity) ListSchoolAccounts(ctx context.Context, tenantID int64) ([]organizationCompose.SchoolAccount, error) {
	accounts, err := p.Accounts.ListAccountsByTenantID(ctx, tenantID)
	if err != nil || accounts == nil {
		return nil, err
	}
	result := make([]organizationCompose.SchoolAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, organizationCompose.SchoolAccount(account))
	}
	return result, nil
}

func (p ProvisioningIdentity) ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]organizationCompose.OrganizationAccount, error) {
	accounts, err := p.Accounts.ListAccountsByOrganizationID(ctx, organizationID)
	if err != nil || accounts == nil {
		return nil, err
	}
	result := make([]organizationCompose.OrganizationAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, organizationCompose.OrganizationAccount{
			SchoolAccount: organizationCompose.SchoolAccount(account.TenantAccountInfo),
			SchoolID:      account.SchoolID, SchoolName: account.SchoolName,
		})
	}
	return result, nil
}

func (p ProvisioningIdentity) ListAllAccounts(ctx context.Context) ([]organizationCompose.OrganizationAccount, error) {
	accounts, err := p.Accounts.ListAllAccounts(ctx)
	if err != nil || accounts == nil {
		return nil, err
	}
	result := make([]organizationCompose.OrganizationAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, organizationCompose.OrganizationAccount{
			SchoolAccount: organizationCompose.SchoolAccount(account.TenantAccountInfo),
			SchoolID:      account.SchoolID, SchoolName: account.SchoolName,
		})
	}
	return result, nil
}

func (p ProvisioningIdentity) RevokeSchoolSessions(ctx context.Context, tenantID int64) (int, error) {
	return p.Sessions.RevokeTokensByTenantID(ctx, tenantID)
}

func (p ProvisioningIdentity) InvalidatePendingInvitations(ctx context.Context, tenantID int64) (int, error) {
	return p.Invitations.RevokeTenantSchoolInvitations(ctx, tenantID)
}

func (p ProvisioningIdentity) DeactivateAccount(ctx context.Context, accountID int64) error {
	return provisioningIdentityError(p.Administration.DeactivateAccount(ctx, accountID))
}

func (p ProvisioningIdentity) AnonymizeAccount(ctx context.Context, accountID int64, email string) error {
	return p.Administration.AnonymizeAccountForDeletion(ctx, accountID, email)
}

// FindAccountIDByEmail resolves the account behind an address; an unknown
// address is found=false.
func (p ProvisioningIdentity) FindAccountIDByEmail(ctx context.Context, email string) (int64, bool, error) {
	account, err := p.Traeger.FindAccountByEmail(ctx, email)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return account.ID, true, nil
}

// AssignTraegerRole grants the Träger role on the operator's transaction.
func (p ProvisioningIdentity) AssignTraegerRole(ctx context.Context, organizationID, accountID int64, role string) error {
	err := p.Traeger.AssignTraegerRole(ctx, organizationID, accountID, role)
	switch {
	case errors.Is(err, identityaccess.ErrInvalidTraegerRole):
		return organizationtenancy.ErrInvalidTraegerRole
	case errors.Is(err, identityaccess.ErrAccountNotFound):
		return organizationtenancy.ErrTraegerAccountNotFound
	}
	return err
}

// ResetTraegerMFA resets the member's second factor on the operator's
// transaction.
func (p ProvisioningIdentity) ResetTraegerMFA(ctx context.Context, accountID int64) error {
	err := p.Traeger.ResetMFAInTransaction(ctx, accountID)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return organizationtenancy.ErrTraegerMemberNotFound
	}
	return err
}

// ListTraegerAccounts reads address and Träger roles of the members.
func (p ProvisioningIdentity) ListTraegerAccounts(ctx context.Context, organizationID int64, accountIDs []int64) ([]organizationtenancy.TraegerAccount, error) {
	accounts, err := p.Traeger.ListTraegerAccounts(ctx, organizationID, accountIDs)
	if err != nil {
		return nil, err
	}
	result := make([]organizationtenancy.TraegerAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, organizationtenancy.TraegerAccount(account))
	}
	return result, nil
}

// invalidSchoolIdentityError marks an identity chain input error for the
// provisioning flow while keeping the owner's user-facing message.
type invalidSchoolIdentityError struct{ err error }

func (e invalidSchoolIdentityError) Error() string { return e.err.Error() }

func (e invalidSchoolIdentityError) Unwrap() []error {
	return []error{organizationCompose.ErrInvalidSchoolIdentity, e.err}
}

// provisioningSettings resolves the device online window of one school.
type provisioningSettings struct {
	settings settingsCompose.SettingsService
}

func (s provisioningSettings) DeviceOnlineWindowMinutes(ctx context.Context, tenantID int64) (int, error) {
	return s.settings.ResolveIntForTenant(ctx, tenantID, configModels.KeyDeviceOnlineWindowMinutes)
}

// pwaUsageCounts serves the Delivery usage snapshot from the operator
// dashboard projection.
type pwaUsageCounts struct {
	provisioning organizationtenancy.ProvisioningDashboard
}

func (c pwaUsageCounts) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]deliveryCompose.PWAUsageRow, error) {
	rows, err := c.provisioning.ListPWAUsage(ctx, window)
	if err != nil {
		return nil, err
	}
	result := make([]deliveryCompose.PWAUsageRow, 0, len(rows))
	for _, row := range rows {
		if tenantID <= 0 || row.TenantID == tenantID {
			result = append(result, deliveryCompose.PWAUsageRow(row))
		}
	}
	return result, nil
}

// provisioningIdentityTranslation maps the identity outcomes of operator
// provisioning onto the contract Organisation & Tenancy reports them in.
// The operator routes classify on that contract and may not name Identity &
// Access, so the translation lives here, once (#3364).
var provisioningIdentityTranslation = []retainedSentinel{
	{identityaccess.ErrEmailAlreadyExists, organizationtenancy.ErrAccountEmailExists},
	{identityaccess.ErrUsernameAlreadyExists, organizationtenancy.ErrAccountUsernameExists},
	{identityaccess.ErrAccountNotFound, organizationtenancy.ErrAccountNotFound},
	{identityaccess.ErrInvitationNameRequired, organizationtenancy.ErrInvitationNameRequired},
	{identityaccess.ErrInvitationPasswordMismatch, organizationtenancy.ErrPasswordMismatch},
	{identityaccess.ErrPasswordTooWeak, organizationtenancy.ErrPasswordTooWeak},
}

// provisioningIdentityError puts an identity refusal into the provisioning
// envelope, keeping the owner's operation name and its message.
func provisioningIdentityError(err error) error {
	if err == nil {
		return nil
	}
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation == err {
		return &organizationtenancy.ProvisioningIdentityError{Op: operation.Op, Err: provisioningIdentityError(operation.Err)}
	}
	for _, sentinel := range provisioningIdentityTranslation {
		if !errors.Is(err, sentinel.public) {
			continue
		}
		if err == sentinel.public {
			return sentinel.retained
		}
		// A store failure stays marked even when it also carries a
		// translated refusal, as the routes classified it before (#2736).
		return organizationtenancy.MarkIdentityStoreFailure(&retainedError{text: err.Error(), sentinel: sentinel.retained, cause: err})
	}
	return organizationtenancy.MarkIdentityStoreFailure(err)
}

// operatorPersonStudentMembership reads the operator listing's student facts
// through the retained People Directory runtime over the student directory
// projection.
func operatorPersonStudentMembership(db transactionruntime.Database) operatorPersonStudents {
	runtime := peopleCompose.NewLegacyRepositoryRuntime(db.Pool())
	students := studentDirectoryView.NewReader()
	return func(ctx context.Context, personIDs []int64) (map[int64]bool, error) {
		return peopleCompose.OperatorPersonStudentMembership(ctx, runtime, students, personIDs)
	}
}

// provisioningRoleRead joins the provisioning command, or explicitly reads
// the platform role catalog before a target-school transaction exists.
func provisioningRoleRead(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	return provisioningTenantScope.WithinAdmin(ctx, fn)
}
