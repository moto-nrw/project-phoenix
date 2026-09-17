package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// operatorProvisioningSources are the retained services and repositories the
// Organisation & Tenancy provisioning seams are bound to (#3253).
type operatorProvisioningSources struct {
	repos          *repositories.Factory
	organizations  organizationtenancy.Capability
	adapters       repositories.OperatorProvisioningAdapters
	authService    *auth.Service
	invitations    auth.InvitationService
	schoolIdentity auth.SchoolIdentityProvisioning
	roles          identityaccess.RoleCommand
	settings       config.SettingsService
	logger         *slog.Logger
}

func newOperatorProvisioning(sources operatorProvisioningSources) (organizationtenancy.Provisioning, error) {
	return organizationCompose.NewProvisioning(organizationCompose.ProvisioningDependencies{
		Organizations: sources.organizations,
		Identity: provisioningIdentity{
			repos: sources.repos, authService: sources.authService,
			invitations: sources.invitations, schoolIdentity: sources.schoolIdentity,
			roles: sources.roles,
		},
		Devices:           sources.adapters.Devices,
		People:            sources.adapters.People,
		Presence:          sources.adapters.Presence,
		Categories:        sources.adapters.Categories,
		Settings:          provisioningSettings{settings: sources.settings},
		Audit:             sources.adapters.Audit,
		Logger:            sources.logger,
		ActiveMemberships: sources.adapters.ActiveMemberships,
	})
}

// provisioningIdentity binds the Identity & Access part of provisioning to
// the public role administration and the retained auth services and
// repositories.
type provisioningIdentity struct {
	repos          *repositories.Factory
	authService    *auth.Service
	invitations    auth.InvitationService
	schoolIdentity auth.SchoolIdentityProvisioning
	roles          identityaccess.RoleCommand
}

var _ organizationCompose.ProvisioningIdentity = provisioningIdentity{}

func (p provisioningIdentity) ListSystemRoles(ctx context.Context) ([]organizationCompose.ProvisioningRole, error) {
	roles, err := p.repos.Role.List(ctx, map[string]any{"is_system": true})
	if err != nil {
		return nil, err
	}
	result := make([]organizationCompose.ProvisioningRole, 0, len(roles))
	for _, role := range roles {
		if role != nil {
			result = append(result, p.role(role.ID, role.Name, role.IsSystem, role.TenantID, role.BaseRole))
		}
	}
	return result, nil
}

func (p provisioningIdentity) FindSystemRole(ctx context.Context, name string) (organizationCompose.ProvisioningRole, bool, error) {
	role, err := auth.ResolveSystemRoleByName(ctx, p.repos.Role, name)
	if err != nil || role == nil {
		return organizationCompose.ProvisioningRole{}, false, err
	}
	return p.role(role.ID, role.Name, role.IsSystem, role.TenantID, role.BaseRole), true, nil
}

func (p provisioningIdentity) FindRole(ctx context.Context, id int64) (organizationCompose.ProvisioningRole, bool, error) {
	role, err := p.repos.Role.FindByID(ctx, id)
	if err != nil || role == nil {
		return organizationCompose.ProvisioningRole{}, false, err
	}
	return p.role(role.ID, role.Name, role.IsSystem, role.TenantID, role.BaseRole), true, nil
}

func (p provisioningIdentity) role(id int64, name string, system bool, tenantID *int64, baseRole *string) organizationCompose.ProvisioningRole {
	facts := &auth.RoleFacts{ID: id, TenantID: tenantID, Name: name, IsSystem: system, BaseRole: baseRole}
	return organizationCompose.ProvisioningRole{
		ID: id, Name: name, IsSystem: system, TenantID: tenantID, BaseRole: baseRole,
		Lehrkraft:            p.schoolIdentity.IsLehrkraftSystemRole(facts),
		CaregiverPermissions: p.schoolIdentity.IsPlatformCaregiverRole(facts),
	}
}

// InviteSchoolAdmin hands out the school admin role. The caller is
// operator-authenticated (platform scope, no tenant permission set), so the
// tenant-side role-grant check does not apply.
func (p provisioningIdentity) InviteSchoolAdmin(ctx context.Context, request organizationCompose.SchoolAdminInvitationRequest) (organizationCompose.SchoolAdminInvitation, error) {
	invitation, err := p.invitations.CreateInvitation(tenant.WithTenantID(ctx, request.TenantID), auth.InvitationRequest{
		Email: request.Email, RoleID: request.RoleID, TenantID: request.TenantID,
		FirstName: request.FirstName, LastName: request.LastName, Position: request.Position,
		CaregiverEnabled: request.CaregiverEnabled,
		OperatorGrant:    true,
	})
	if err != nil {
		return organizationCompose.SchoolAdminInvitation{}, err
	}
	result := organizationCompose.SchoolAdminInvitation{
		ID: invitation.ID, Email: invitation.Email, RoleID: invitation.RoleID, Token: invitation.Token,
		ExpiresAt: invitation.ExpiresAt, FirstName: invitation.FirstName, LastName: invitation.LastName,
		Position: invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled, CreatedBy: invitation.CreatedBy,
		EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError, EmailRetryCount: invitation.EmailRetryCount,
	}
	if invitation.Role != nil {
		result.RoleName = invitation.Role.Name
	}
	if invitation.Creator != nil {
		result.CreatorEmail = invitation.Creator.Email
	}
	return result, nil
}

func (p provisioningIdentity) RegisterSchoolAccount(ctx context.Context, registration organizationCompose.SchoolAccountRegistration) (organizationCompose.CreatedAccount, error) {
	roleID := registration.RoleID
	account, err := p.authService.Register(tenant.WithTenantID(ctx, registration.TenantID),
		registration.Email, registration.Username, registration.Password, &roleID, registration.TenantID)
	if err != nil {
		return organizationCompose.CreatedAccount{}, err
	}
	return organizationCompose.CreatedAccount{
		ID: account.ID, CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt, Email: account.Email,
		Username: account.Username, Avatar: account.Avatar, Active: account.Active,
		IsPasswordOTP: account.IsPasswordOTP, LastLogin: account.LastLogin,
	}, nil
}

func (p provisioningIdentity) EnsureSchoolIdentity(ctx context.Context, request organizationCompose.SchoolIdentityRequest) error {
	role := request.Role
	_, err := p.schoolIdentity.EnsureSchoolIdentity(tenant.WithTenantID(ctx, request.TenantID), auth.SchoolIdentityInput{
		AccountID: request.AccountID,
		TenantID:  request.TenantID,
		Role: &auth.RoleFacts{
			ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
		},
		FirstName:        request.FirstName,
		LastName:         request.LastName,
		Position:         request.Position,
		CaregiverUpgrade: request.CaregiverUpgrade,
		CreatePerson:     true,
	})
	if errors.Is(err, auth.ErrSchoolIdentityNamesRequired) || errors.Is(err, auth.ErrSchoolIdentityPersonIsStudent) {
		return invalidSchoolIdentityError{err: err}
	}
	return err
}

func (p provisioningIdentity) AssignRole(ctx context.Context, tenantID, accountID, roleID int64) error {
	return authServiceError(p.roles.AssignRoleToAccount(tenant.WithTenantID(ctx, tenantID), accountID, roleID))
}

func (p provisioningIdentity) ListSchoolAccounts(ctx context.Context, tenantID int64) ([]organizationCompose.SchoolAccount, error) {
	accounts, err := p.repos.AccountTenant.ListAccountsByTenantID(ctx, tenantID)
	if err != nil || accounts == nil {
		return nil, err
	}
	result := make([]organizationCompose.SchoolAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, organizationCompose.SchoolAccount(account))
	}
	return result, nil
}

func (p provisioningIdentity) ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]organizationCompose.OrganizationAccount, error) {
	accounts, err := p.repos.AccountTenant.ListAccountsByOrganizationID(ctx, organizationID)
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

func (p provisioningIdentity) ListAllAccounts(ctx context.Context) ([]organizationCompose.OrganizationAccount, error) {
	accounts, err := p.repos.AccountTenant.ListAllAccounts(ctx)
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

func (p provisioningIdentity) RevokeSchoolSessions(ctx context.Context, tenantID int64) (int, error) {
	return p.authService.RevokeTokensByTenantID(ctx, tenantID)
}

func (p provisioningIdentity) InvalidatePendingInvitations(ctx context.Context, tenantID int64) (int, error) {
	return p.invitations.InvalidatePendingInvitationsByTenantID(ctx, tenantID)
}

func (p provisioningIdentity) DeactivateAccount(ctx context.Context, accountID int64) error {
	return p.authService.DeactivateAccount(ctx, int(accountID))
}

func (p provisioningIdentity) AnonymizeAccount(ctx context.Context, accountID int64, email string) error {
	return p.repos.Account.AnonymizeForDeletion(ctx, accountID, email)
}

// invalidSchoolIdentityError marks an identity chain input error for the
// provisioning flow while keeping the owner's user-facing message.
type invalidSchoolIdentityError struct{ err error }

func (e invalidSchoolIdentityError) Error() string { return e.err.Error() }
func (e invalidSchoolIdentityError) Unwrap() []error {
	return []error{organizationCompose.ErrInvalidSchoolIdentity, e.err}
}

// provisioningSettings resolves the device online window of one school.
type provisioningSettings struct{ settings config.SettingsService }

func (s provisioningSettings) DeviceOnlineWindowMinutes(ctx context.Context, tenantID int64) (int, error) {
	return s.settings.ResolveIntForTenant(ctx, tenantID, configModels.KeyDeviceOnlineWindowMinutes)
}

// pwaUsageCounts serves the Delivery usage snapshot from the operator
// dashboard projection.
type pwaUsageCounts struct {
	provisioning organizationtenancy.Provisioning
}

func (c pwaUsageCounts) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]pwa.UsageRow, error) {
	rows, err := c.provisioning.ListPWAUsage(ctx, window)
	if err != nil {
		return nil, err
	}
	result := make([]pwa.UsageRow, 0, len(rows))
	for _, row := range rows {
		if tenantID <= 0 || row.TenantID == tenantID {
			result = append(result, pwa.UsageRow(row))
		}
	}
	return result, nil
}
