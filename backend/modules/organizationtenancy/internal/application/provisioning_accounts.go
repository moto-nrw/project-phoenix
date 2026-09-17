package application

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

const (
	adminRoleName     = "admin"
	userRoleName      = "user"
	guardianRoleName  = "guardian"
	legacyTeacherRole = "teacher"
)

// InviteSchoolAdmin invites the administrator of an active school. The
// caller is operator-authenticated, so the tenant role-grant check does not
// apply to handing out the admin role.
func (p *Provisioning) InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, input organizationtenancy.SchoolAdminInvitationInput) (*organizationtenancy.SchoolAdminInvitation, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.SchoolAdminInvitation, error) {
		school, err := p.findInvitableSchool(adminCtx, schoolID)
		if err != nil {
			return nil, err
		}
		adminRole, err := p.systemRole(adminCtx, adminRoleName)
		if err != nil {
			return nil, err
		}
		invitation, err := p.identity.InviteSchoolAdmin(adminCtx, domain.SchoolAdminInvitationRequest{
			TenantID: school.ID, RoleID: adminRole.ID,
			Email:     strings.TrimSpace(strings.ToLower(input.Email)),
			FirstName: input.FirstName, LastName: input.LastName, Position: input.Position,
			CaregiverEnabled: input.CaregiverEnabled,
		})
		if err != nil {
			return nil, err
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceInvitation, &invitation.ID, clientIP, map[string]any{
			"schoolID": school.ID,
			"email":    invitation.Email,
			"roleID":   invitation.RoleID,
		})
		view := organizationtenancy.SchoolAdminInvitation(invitation)
		return &view, nil
	})
}

// CreateSchoolAccount creates an account at an active school together with
// its person, staff and, where requested, caregiver identity (#2222).
func (p *Provisioning) CreateSchoolAccount(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, input organizationtenancy.SchoolAccountInput) (*organizationtenancy.CreatedAccount, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.CreatedAccount, error) {
		school, err := p.findInvitableSchool(adminCtx, schoolID)
		if err != nil {
			return nil, err
		}
		role, err := p.accountRole(adminCtx, input)
		if err != nil {
			return nil, err
		}
		username := fmt.Sprintf("%s_%s_%s",
			strings.ToLower(strings.TrimSpace(input.FirstName)),
			strings.ToLower(strings.TrimSpace(input.LastName)),
			p.secrets.UsernameSuffix(),
		)
		account, err := p.identity.RegisterSchoolAccount(adminCtx, domain.SchoolAccountRegistration{
			TenantID: school.ID, Email: input.Email, Username: username, Password: input.Password, RoleID: role.ID,
		})
		if err != nil {
			return nil, err
		}
		if err := p.ensureSchoolIdentity(adminCtx, account.ID, school.ID, role, input); err != nil {
			return nil, err
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceAccount, &account.ID, clientIP, map[string]any{
			"schoolID": school.ID,
			"email":    account.Email,
		})
		view := organizationtenancy.CreatedAccount(account)
		return &view, nil
	})
}

// findInvitableSchool reads a school an account may be added to: it exists,
// is not deleted and is active.
func (p *Provisioning) findInvitableSchool(ctx context.Context, schoolID int64) (organizationtenancy.School, error) {
	school, err := p.findLiveSchool(ctx, schoolID)
	if err != nil {
		return organizationtenancy.School{}, err
	}
	if !school.Active {
		return organizationtenancy.School{}, &organizationtenancy.InvalidProvisioningDataError{Err: errors.New("school is inactive")}
	}
	return school, nil
}

// accountRole resolves the role a new school account receives: the admin
// system role by default, otherwise a supported system role.
func (p *Provisioning) accountRole(ctx context.Context, input organizationtenancy.SchoolAccountInput) (domain.Role, error) {
	if input.RoleID == nil {
		return p.systemRole(ctx, adminRoleName)
	}
	role, found, err := p.identity.FindRole(ctx, *input.RoleID)
	if err != nil {
		return domain.Role{}, fmt.Errorf("lookup role: %w", err)
	}
	if !found {
		return domain.Role{}, invalidData(fmt.Sprintf("role with ID %d not found", *input.RoleID))
	}
	if !role.IsSystem {
		return domain.Role{}, invalidData("only system roles are allowed for school account creation")
	}
	if strings.EqualFold(role.Name, guardianRoleName) {
		return domain.Role{}, invalidData("guardian accounts must be created through the guardian invitation flow")
	}
	if strings.EqualFold(role.Name, legacyTeacherRole) {
		return domain.Role{}, invalidData("legacy teacher role is no longer assignable; use the user role for caregiver accounts")
	}
	// Same invariant the invitation flow enforces (#1772): the caregiver
	// upgrade would hand a Lehrkraft the full user role plus a caregiver
	// profile, defeating its class-scoped read-only design.
	if input.CaregiverEnabled && role.Lehrkraft {
		return domain.Role{}, invalidData("the lehrkraft role cannot be combined with caregiver capability")
	}
	return role, nil
}

func (p *Provisioning) systemRole(ctx context.Context, name string) (domain.Role, error) {
	role, found, err := p.identity.FindSystemRole(ctx, name)
	if err != nil {
		return domain.Role{}, err
	}
	if !found {
		return domain.Role{}, invalidData(name + " role not found")
	}
	return role, nil
}

// ensureSchoolIdentity provisions the account's identity at the school and,
// when the caregiver capability was requested for a role that does not
// already carry it, hands out the platform user role for its permissions.
// base_role classifies a role, it does not grant anything — a school's own
// role of caregiver tier gets the profile, not the platform role's
// permissions.
func (p *Provisioning) ensureSchoolIdentity(ctx context.Context, accountID, schoolID int64, role domain.Role, input organizationtenancy.SchoolAccountInput) error {
	err := p.identity.EnsureSchoolIdentity(ctx, domain.SchoolIdentityRequest{
		AccountID: accountID, TenantID: schoolID, Role: role,
		FirstName: input.FirstName, LastName: input.LastName, Position: input.Position,
		CaregiverUpgrade: input.CaregiverEnabled,
	})
	if errors.Is(err, domain.ErrInvalidSchoolIdentity) {
		return &organizationtenancy.InvalidProvisioningDataError{Err: err}
	}
	if err != nil {
		return err
	}
	if !input.CaregiverEnabled || role.CaregiverPermissions {
		return nil
	}
	userRole, found, err := p.identity.FindSystemRole(ctx, userRoleName)
	if err == nil && !found {
		err = errors.New("user role not found")
	}
	if err == nil {
		err = p.identity.AssignRole(ctx, schoolID, accountID, userRole.ID)
	}
	if err != nil {
		return fmt.Errorf("assign caregiver role: %w", err)
	}
	return nil
}

// ListSystemRoles lists the platform roles an operator can hand out.
func (p *Provisioning) ListSystemRoles(ctx context.Context) ([]organizationtenancy.SystemRole, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.SystemRole, error) {
		roles, err := p.identity.ListSystemRoles(adminCtx)
		if err != nil {
			return nil, err
		}
		result := make([]organizationtenancy.SystemRole, 0, len(roles))
		for _, role := range roles {
			result = append(result, organizationtenancy.SystemRole{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem})
		}
		return result, nil
	})
}

// ListSchoolAccounts lists the accounts of a non-deleted school.
func (p *Provisioning) ListSchoolAccounts(ctx context.Context, schoolID int64) ([]organizationtenancy.SchoolAccount, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.SchoolAccount, error) {
		if _, err := p.findLiveSchool(adminCtx, schoolID); err != nil {
			return nil, err
		}
		accounts, err := p.identity.ListSchoolAccounts(adminCtx, schoolID)
		if err != nil || accounts == nil {
			return nil, err
		}
		result := make([]organizationtenancy.SchoolAccount, 0, len(accounts))
		for _, account := range accounts {
			result = append(result, organizationtenancy.SchoolAccount(account))
		}
		return result, nil
	})
}

// ListOrganizationAccounts lists the accounts of an organisation's schools.
func (p *Provisioning) ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]organizationtenancy.OrganizationAccount, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OrganizationAccount, error) {
		if _, err := p.organizations.FindOrganization(adminCtx, organizationID); err != nil {
			return nil, mapOrganizationError(err, organizationID)
		}
		accounts, err := p.identity.ListOrganizationAccounts(adminCtx, organizationID)
		return organizationAccounts(accounts), err
	})
}

// ListAllAccounts lists the accounts of every school.
func (p *Provisioning) ListAllAccounts(ctx context.Context) ([]organizationtenancy.OrganizationAccount, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.OrganizationAccount, error) {
		accounts, err := p.identity.ListAllAccounts(adminCtx)
		return organizationAccounts(accounts), err
	})
}

func organizationAccounts(accounts []domain.OrganizationAccount) []organizationtenancy.OrganizationAccount {
	if accounts == nil {
		return nil
	}
	result := make([]organizationtenancy.OrganizationAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, organizationtenancy.OrganizationAccount{
			SchoolAccount: organizationtenancy.SchoolAccount(account.SchoolAccount),
			SchoolID:      account.SchoolID, SchoolName: account.SchoolName,
		})
	}
	return result
}

func invalidData(message string) error {
	return &organizationtenancy.InvalidProvisioningDataError{Err: errors.New(message)}
}
