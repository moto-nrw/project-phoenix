package platform

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/randstr"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// CreateSchoolAccountRequest holds fields for operator-created school accounts.
type CreateSchoolAccountRequest struct {
	Email            string
	Password         string
	FirstName        string
	LastName         string
	RoleID           *int64
	Position         string // optional, maps to Teacher.Role
	CaregiverEnabled bool
}

// UpdateOrganizationRequest holds fields for updating an organization.
type UpdateOrganizationRequest struct {
	Name   string
	Slug   string
	Active bool
}

// UpdateSchoolRequest holds fields for updating a school.
type UpdateSchoolRequest struct {
	OrganizationID int64
	Name           string
	Slug           string
	Subdomain      string
	Address        string
	City           string
	Zip            string
	Phone          string
	Email          string
	Active         bool
	Hidden         bool
}

// defaultDeviceOnlineWindow is the fallback online/offline cutoff used when no
// settings resolver is wired or the per-tenant setting
// iot.device_online_window_minutes cannot be resolved (issue #586).
const defaultDeviceOnlineWindow = 5 * time.Minute

// DeviceTransferSession describes the open kiosk session blocking a transfer.
type DeviceTransferSession struct {
	ID           int64     `json:"id"`
	StartedAt    time.Time `json:"started_at"`
	ActivityName *string   `json:"activity_name,omitempty"`
	RoomName     *string   `json:"room_name,omitempty"`
}

// DeviceTransferStatus is the operator-facing preflight result.
type DeviceTransferStatus struct {
	CanTransfer   bool                   `json:"can_transfer"`
	IsOnline      bool                   `json:"is_online"`
	IsProtected   bool                   `json:"is_protected"`
	LastSeen      *time.Time             `json:"last_seen,omitempty"`
	ActiveSession *DeviceTransferSession `json:"active_session,omitempty"`
}

// ActiveDeviceSessionFinder is the narrow device-transfer session lookup dependency.
type ActiveDeviceSessionFinder interface {
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*activeModels.Group, error)
}

// TenantSettingsResolver is the narrow settings dependency for resolving
// per-tenant settings outside tenant middleware (operator requests carry no
// tenant context).
type TenantSettingsResolver interface {
	ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error)
}

// OperatorProvisioningService handles operator-led tenant provisioning.
type OperatorProvisioningService interface {
	CreateOrganization(ctx context.Context, organization *organizationModule.CreateOrganization, operatorID int64, clientIP net.IP) (*organizationModule.Organization, error)
	ListOrganizations(ctx context.Context) ([]organizationModule.Organization, error)
	UpdateOrganization(ctx context.Context, id int64, req UpdateOrganizationRequest, operatorID int64, clientIP net.IP) (*organizationModule.Organization, error)
	CreateSchool(ctx context.Context, school *platform.School, operatorID int64, clientIP net.IP) (*platform.School, error)
	ListSchools(ctx context.Context) ([]*platform.School, error)
	UpdateSchool(ctx context.Context, id int64, req UpdateSchoolRequest, operatorID int64, clientIP net.IP) (*platform.School, error)
	InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req authSvc.InvitationRequest) (*authModels.InvitationToken, error)
	CreateSchoolAccount(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req CreateSchoolAccountRequest) (*authModels.Account, error)
	ListSystemRoles(ctx context.Context) ([]*authModels.Role, error)
	ListSchoolAccounts(ctx context.Context, schoolID int64) ([]authModels.TenantAccountInfo, error)
	ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]authModels.OrgAccountInfo, error)
	ListAllAccounts(ctx context.Context) ([]authModels.OrgAccountInfo, error)
	ListAllDevices(ctx context.Context) ([]OperatorDeviceInfo, error)
	ListSchoolDevices(ctx context.Context, schoolID int64) ([]OperatorDeviceInfo, error)
	ListOrganizationDevices(ctx context.Context, organizationID int64) ([]OperatorDeviceInfo, error)
	CreateDevice(ctx context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*OperatorDeviceInfo, error)
	SetDeviceAPIKey(ctx context.Context, id int64, apiKey *string, operatorID int64, clientIP net.IP) (*OperatorDeviceInfo, error)
	GetDeviceTransferStatus(ctx context.Context, id int64) (*DeviceTransferStatus, error)
	TransferDevice(ctx context.Context, id, targetSchoolID, operatorID int64, clientIP net.IP) (*OperatorDeviceInfo, error)
	SoftDeleteSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error
	RestoreSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error
	SoftDeleteOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error
	RestoreOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error
	DeleteDevice(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error
	ListSchoolPersons(ctx context.Context, schoolID int64) ([]OperatorPersonInfo, error)
	SoftDeletePerson(ctx context.Context, personID int64, operatorID int64, clientIP net.IP) error
	GetProvisioningStats(ctx context.Context) (*ProvisioningStats, error)
	GetSchoolPWAUsage(ctx context.Context, schoolID int64) (*SchoolPWAUsage, error)
	ListOrganizationSummaries(ctx context.Context) ([]*OrganizationSummary, error)
	ListSchoolSummaries(ctx context.Context) ([]*SchoolSummary, error)
	ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*SchoolSummary, error)
	ListOrganizationPersons(ctx context.Context, organizationID int64) ([]OperatorPersonInfo, error)
	ListAccountTenantAccess(ctx context.Context, accountID int64) ([]AccountTenantAccessEntry, error)
	ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]*authModels.Role, error)
	GrantAccountTenantAccess(ctx context.Context, accountID, schoolID int64, req GrantAccountTenantAccessRequest, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error)
	UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error)
	RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP net.IP) ([]AccountTenantAccessEntry, error)
}

// OperatorPersonInfo aliases the model type so existing service callers keep
// referencing platformSvc.OperatorPersonInfo.
type OperatorPersonInfo = platform.OperatorPersonInfo

// OperatorDeviceInfo holds device information with school/org context for operator views.
// OperatorDeviceInfo is the device listing row; an alias of the repository
// read model so derived presentation fields stay attached.
type OperatorDeviceInfo = platform.OperatorDeviceRow

// maskAPIKey returns the first 10 characters followed by ellipsis.
func maskAPIKey(key *string) string {
	if key == nil || *key == "" {
		return ""
	}
	k := *key
	if len(k) <= 10 {
		return k
	}
	return k[:10] + "..."
}

// enrichDeviceInfo computes derived fields (IsOnline, MaskedAPIKey).
func enrichDeviceInfo(devices []OperatorDeviceInfo) []OperatorDeviceInfo {
	for i := range devices {
		devices[i].MaskedAPIKey = maskAPIKey(devices[i].APIKey)
		if devices[i].LastSeen != nil {
			devices[i].IsOnline = time.Since(*devices[i].LastSeen) <= 5*time.Minute
		}
	}
	return devices
}

type operatorProvisioningService struct {
	OperatorProvisioningServiceConfig
	txHandler     *tenant.TransactionRunner
	tenantRuntime *tenant.UnitOfWork
}

// SetTenantRuntime wires the transaction runtime used when an operator action
// crosses tenant boundaries.
func (s *operatorProvisioningService) SetTenantRuntime(runtime tenant.UnitOfWork) {
	s.tenantRuntime = &runtime
}

func (s *operatorProvisioningService) withTenantRuntime(ctx context.Context) context.Context {
	if s.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithUnitOfWork(ctx, *s.tenantRuntime)
}

func (s *operatorProvisioningService) withAdminTx(ctx context.Context, fn func(context.Context) error) error {
	ctx = s.withTenantRuntime(ctx)
	if s.txHandler == nil {
		return fn(tenant.ContextWithoutTenant(ctx))
	}
	return tenant.WithinAdmin(ctx, fn)
}

// OperatorProvisioningServiceConfig holds dependencies for operator provisioning.
type OperatorProvisioningServiceConfig struct {
	Organizations         organizationModule.Capability
	SchoolRepo            platform.SchoolRepository
	SummariesRepo         platform.OperatorSummariesRepository
	CategoryRepo          activityModels.CategoryRepository
	DeviceRepo            iotModels.DeviceRepository
	RoleRepo              authModels.RoleRepository
	AccountTenantRepo     authModels.AccountTenantRepository
	AccountRoleRepo       authModels.AccountRoleRepository
	AccountPermissionRepo authModels.AccountPermissionRepository
	AuthEventRepo         auditModels.AuthEventRepository
	PersonRepo            userModels.PersonRepository
	StaffRepo             userModels.StaffRepository
	AccountRepo           authModels.AccountRepository
	TeacherRepo           userModels.TeacherRepository
	StudentRepo           userModels.StudentRepository
	GroupSupervisorRepo   activeModels.GroupSupervisorRepository
	ActiveGroupRepo       ActiveDeviceSessionFinder
	Settings              TenantSettingsResolver
	InvitationService     authSvc.InvitationService
	AuthService           authSvc.AuthService
	AuditLogRepo          platform.OperatorAuditLogRepository
	DB                    *bun.DB
	Logger                *slog.Logger
}

// NewOperatorProvisioningService creates a provisioning service.
func NewOperatorProvisioningService(cfg OperatorProvisioningServiceConfig) OperatorProvisioningService {
	if cfg.SummariesRepo == nil {
		panic("operator provisioning service: SummariesRepo is required")
	}
	service := &operatorProvisioningService{
		OperatorProvisioningServiceConfig: cfg,
	}
	if cfg.DB != nil {
		service.txHandler = tenant.NewTransactionRunner()
	}
	return service
}

func (s *operatorProvisioningService) getLogger() *slog.Logger {
	return cmp.Or(s.Logger, slog.Default())
}

func (s *operatorProvisioningService) CreateOrganization(ctx context.Context, organization *organizationModule.CreateOrganization, operatorID int64, clientIP net.IP) (*organizationModule.Organization, error) {
	if organization == nil {
		return nil, &InvalidDataError{Err: fmt.Errorf("organization is required")}
	}

	var created *organizationModule.Organization
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		value, createErr := s.Organizations.CreateOrganization(adminCtx, *organization)
		if createErr != nil {
			return mapOrganizationCapabilityError(createErr, 0)
		}
		created = &value
		return s.recordAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceOrganization, &created.ID, clientIP, map[string]any{
			"name": created.Name,
			"slug": created.Slug,
		})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *operatorProvisioningService) ListOrganizations(ctx context.Context) ([]organizationModule.Organization, error) {
	return s.Organizations.ListOrganizations(ctx)
}

func (s *operatorProvisioningService) UpdateOrganization(ctx context.Context, id int64, req UpdateOrganizationRequest, operatorID int64, clientIP net.IP) (*organizationModule.Organization, error) {
	var updated *organizationModule.Organization
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		existing, err := s.Organizations.FindOrganizationForMutation(adminCtx, id)
		if err != nil {
			return mapOrganizationCapabilityError(err, id)
		}
		if existing.IsDeleted() {
			return &OrganizationAlreadyDeletedError{OrganizationID: id}
		}

		changes := map[string]any{}

		value, updateErr := s.Organizations.UpdateOrganization(adminCtx, organizationModule.UpdateOrganization{
			ID: id, Name: req.Name, Slug: req.Slug, Active: req.Active,
		})
		if updateErr != nil {
			return mapOrganizationCapabilityError(updateErr, id)
		}
		if value.Slug != existing.Slug {
			changes["slug"] = map[string]string{"old": existing.Slug, "new": value.Slug}
		}
		if value.Name != existing.Name {
			changes["name"] = map[string]string{"old": existing.Name, "new": value.Name}
		}
		if value.Active != existing.Active {
			changes["active"] = map[string]bool{"old": existing.Active, "new": value.Active}
		}

		updated = &value
		return s.recordAction(adminCtx, operatorID, platform.ActionUpdate, platform.ResourceOrganization, &id, clientIP, changes)
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *operatorProvisioningService) CreateSchool(ctx context.Context, school *platform.School, operatorID int64, clientIP net.IP) (*platform.School, error) {
	if school == nil {
		return nil, &InvalidDataError{Err: fmt.Errorf("school is required")}
	}
	if err := school.Validate(); err != nil {
		return nil, &InvalidDataError{Err: err}
	}

	var created *platform.School
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		if err := s.validateSchoolCreate(adminCtx, school); err != nil {
			return err
		}
		if createErr := s.SchoolRepo.Create(adminCtx, school); createErr != nil {
			if modelBase.IsUniqueViolation(createErr) {
				return mapSchoolCreateConflict(adminCtx, s.SchoolRepo, school)
			}
			return createErr
		}
		if seedErr := s.seedDefaultActivityCategories(adminCtx, school.ID); seedErr != nil {
			return seedErr
		}
		if deviceErr := s.createWebManualDevice(adminCtx, school.ID); deviceErr != nil {
			return deviceErr
		}
		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceSchool, &school.ID, clientIP, map[string]any{
			"name":           school.Name,
			"slug":           school.Slug,
			"subdomain":      school.Subdomain,
			"organizationID": school.OrganizationID,
		})
		created = school
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *operatorProvisioningService) ListSchools(ctx context.Context) ([]*platform.School, error) {
	return s.SchoolRepo.List(ctx)
}

func (s *operatorProvisioningService) UpdateSchool(ctx context.Context, id int64, req UpdateSchoolRequest, operatorID int64, clientIP net.IP) (*platform.School, error) {
	var updated *platform.School
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		existing, err := s.SchoolRepo.FindByID(adminCtx, id)
		if err != nil {
			if isLookupNotFound(err) {
				return &SchoolNotFoundError{SchoolID: id}
			}
			return err
		}
		if existing == nil {
			return &SchoolNotFoundError{SchoolID: id}
		}
		if existing.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: id}
		}

		changes := map[string]any{}

		if req.OrganizationID != existing.OrganizationID {
			org, orgErr := s.Organizations.FindOrganizationForSchoolMutation(adminCtx, req.OrganizationID)
			if orgErr != nil {
				return mapOrganizationCapabilityError(orgErr, req.OrganizationID)
			}
			if org.IsDeleted() {
				return &OrganizationDeletedError{OrganizationID: req.OrganizationID}
			}
			changes["organization_id"] = map[string]int64{"old": existing.OrganizationID, "new": req.OrganizationID}
		}

		targetOrgID := req.OrganizationID
		if req.Slug != existing.Slug || req.OrganizationID != existing.OrganizationID {
			taken, findErr := s.SchoolRepo.FindByOrganizationAndSlug(adminCtx, targetOrgID, req.Slug)
			if findErr != nil {
				return findErr
			}
			if taken != nil && taken.ID != id {
				return &ConflictError{Err: fmt.Errorf("school slug already exists in this organization")}
			}
			if req.Slug != existing.Slug {
				changes["slug"] = map[string]string{"old": existing.Slug, "new": req.Slug}
			}
		}

		if req.Subdomain != existing.Subdomain {
			taken, findErr := s.SchoolRepo.FindBySubdomain(adminCtx, req.Subdomain)
			if findErr != nil {
				return findErr
			}
			if taken != nil && taken.ID != id {
				return &ConflictError{Err: fmt.Errorf("school subdomain already exists")}
			}
			changes["subdomain"] = map[string]string{"old": existing.Subdomain, "new": req.Subdomain}
		}

		if req.Name != existing.Name {
			changes["name"] = map[string]string{"old": existing.Name, "new": req.Name}
		}
		if req.Active != existing.Active {
			changes["active"] = map[string]bool{"old": existing.Active, "new": req.Active}
		}
		if req.Hidden != existing.Hidden {
			changes["hidden"] = map[string]bool{"old": existing.Hidden, "new": req.Hidden}
		}

		existing.OrganizationID = req.OrganizationID
		existing.Name = req.Name
		existing.Slug = req.Slug
		existing.Subdomain = req.Subdomain
		existing.Address = req.Address
		existing.City = req.City
		existing.Zip = req.Zip
		existing.Phone = req.Phone
		existing.Email = req.Email
		existing.Active = req.Active
		existing.Hidden = req.Hidden

		if updateErr := s.SchoolRepo.Update(adminCtx, existing); updateErr != nil {
			if modelBase.IsUniqueViolation(updateErr) {
				return mapSchoolCreateConflict(adminCtx, s.SchoolRepo, existing)
			}
			return &InvalidDataError{Err: updateErr}
		}

		s.logAction(adminCtx, operatorID, platform.ActionUpdate, platform.ResourceSchool, &id, clientIP, changes)
		updated = existing
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *operatorProvisioningService) InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req authSvc.InvitationRequest) (*authModels.InvitationToken, error) {
	var invitation *authModels.InvitationToken
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, adminRole, err := s.resolveAdminInviteContext(adminCtx, schoolID)
		if err != nil {
			return err
		}

		req = normalizeAdminInviteRequest(req, adminRole.ID, school.ID)

		invitationCtx := tenant.WithTenantID(adminCtx, school.ID)
		created, createErr := s.InvitationService.CreateInvitation(invitationCtx, req)
		if createErr != nil {
			return createErr
		}
		invitation = created
		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceInvitation, &invitation.ID, clientIP, map[string]any{
			"schoolID": school.ID,
			"email":    invitation.Email,
			"roleID":   invitation.RoleID,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return invitation, nil
}

func (s *operatorProvisioningService) CreateSchoolAccount(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req CreateSchoolAccountRequest) (*authModels.Account, error) {
	var account *authModels.Account
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, err := s.loadActiveSchool(adminCtx, schoolID)
		if err != nil {
			return err
		}

		roleID := req.RoleID
		var selectedRole *authModels.Role
		if roleID == nil {
			adminRole, roleErr := authSvc.ResolveSystemRoleByName(adminCtx, s.RoleRepo, "admin")
			if roleErr != nil {
				return roleErr
			}
			if adminRole == nil {
				return &InvalidDataError{Err: fmt.Errorf("admin role not found")}
			}
			roleID = &adminRole.ID
			selectedRole = adminRole
		} else {
			// Validate that the provided role is a supported system role.
			role, roleErr := s.RoleRepo.FindByID(adminCtx, *roleID)
			if roleErr != nil {
				return fmt.Errorf("lookup role: %w", roleErr)
			}
			if role == nil {
				return &InvalidDataError{Err: fmt.Errorf("role with ID %d not found", *roleID)}
			}
			if !role.IsSystem {
				return &InvalidDataError{Err: fmt.Errorf("only system roles are allowed for school account creation")}
			}
			if strings.EqualFold(role.Name, "guardian") {
				return &InvalidDataError{Err: fmt.Errorf("guardian accounts must be created through the guardian invitation flow")}
			}
			if strings.EqualFold(role.Name, "teacher") {
				return &InvalidDataError{Err: fmt.Errorf("legacy teacher role is no longer assignable; use the user role for caregiver accounts")}
			}
			// Same invariant the invitation flow enforces (#1772): the
			// caregiver upgrade would hand a Lehrkraft the full user role
			// plus a caregiver profile, defeating its class-scoped
			// read-only design.
			if req.CaregiverEnabled && authSvc.IsLehrkraftSystemRole(role) {
				return &InvalidDataError{Err: fmt.Errorf("the lehrkraft role cannot be combined with caregiver capability")}
			}
			selectedRole = role
		}

		// Auto-generate username from name
		suffix := generateRandomSuffix(6)
		username := fmt.Sprintf("%s_%s_%s",
			strings.ToLower(strings.TrimSpace(req.FirstName)),
			strings.ToLower(strings.TrimSpace(req.LastName)),
			suffix,
		)

		// Step 1: Create Account
		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)
		created, createErr := s.AuthService.Register(tenantCtx, req.Email, username, req.Password, roleID, school.ID)
		if createErr != nil {
			return createErr
		}
		account = created

		// Step 2: person, staff and (for caregiver roles) the teacher profile.
		// Same walk every other school-access path uses (#2222).
		if err := s.ensureSchoolIdentityWithCaregiver(tenantCtx, account.ID, school.ID, selectedRole, req); err != nil {
			return err
		}

		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceAccount, &account.ID, clientIP, map[string]any{
			"schoolID": school.ID,
			"email":    account.Email,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

// ensureSchoolIdentityWithCaregiver provisions the account's identity at the
// school and, when the caregiver capability was requested for a role that does
// not already carry it, hands out the platform user role for its permissions.
// base_role classifies a role, it does not grant anything — a school's own role
// of caregiver tier gets the profile, not the platform role's permissions.
func (s *operatorProvisioningService) ensureSchoolIdentityWithCaregiver(
	ctx context.Context,
	accountID, schoolID int64,
	role *authModels.Role,
	req CreateSchoolAccountRequest,
) error {
	if err := s.ensureSchoolIdentityForCaregiverRequest(ctx, accountID, schoolID, role, req); err != nil {
		return err
	}
	if req.CaregiverEnabled && !authSvc.IsPlatformCaregiverRole(role) {
		if err := s.ensureUserRole(ctx, accountID); err != nil {
			return fmt.Errorf("assign caregiver role: %w", err)
		}
	}
	return nil
}

func (s *operatorProvisioningService) ensureSchoolIdentityForCaregiverRequest(
	ctx context.Context,
	accountID, schoolID int64,
	role *authModels.Role,
	req CreateSchoolAccountRequest,
) error {
	_, err := authSvc.EnsureSchoolIdentity(ctx, authSvc.SchoolIdentityRepos{
		Persons:  s.PersonRepo,
		Staff:    s.StaffRepo,
		Teachers: s.TeacherRepo,
		Students: s.StudentRepo,
	}, authSvc.SchoolIdentityInput{
		AccountID:        accountID,
		TenantID:         schoolID,
		Role:             role,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		Position:         req.Position,
		CaregiverUpgrade: req.CaregiverEnabled,
		CreatePerson:     true,
	})
	if errors.Is(err, authSvc.ErrSchoolIdentityNamesRequired) ||
		errors.Is(err, authSvc.ErrSchoolIdentityPersonIsStudent) {
		return &InvalidDataError{Err: err}
	}
	return err
}

func (s *operatorProvisioningService) ensureUserRole(ctx context.Context, accountID int64) error {
	userRole, err := authSvc.ResolveSystemRoleByName(ctx, s.RoleRepo, "user")
	if err != nil {
		return err
	}
	if userRole == nil {
		return fmt.Errorf("user role not found")
	}

	return s.AuthService.AssignRoleToAccount(ctx, int(accountID), int(userRole.ID))
}

func (s *operatorProvisioningService) ListSystemRoles(ctx context.Context) ([]*authModels.Role, error) {
	var result []*authModels.Role
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		roles, listErr := s.RoleRepo.List(adminCtx, map[string]interface{}{"is_system": true})
		if listErr != nil {
			return listErr
		}
		result = roles
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *operatorProvisioningService) ListSchoolAccounts(ctx context.Context, schoolID int64) ([]authModels.TenantAccountInfo, error) {
	var result []authModels.TenantAccountInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, findErr := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if school.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: schoolID}
		}
		accounts, listErr := s.AccountTenantRepo.ListAccountsByTenantID(adminCtx, schoolID)
		if listErr != nil {
			return listErr
		}
		result = accounts
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *operatorProvisioningService) ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]authModels.OrgAccountInfo, error) {
	var result []authModels.OrgAccountInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		_, findErr := s.Organizations.FindOrganization(adminCtx, organizationID)
		if findErr != nil {
			return mapOrganizationCapabilityError(findErr, organizationID)
		}
		accounts, listErr := s.AccountTenantRepo.ListAccountsByOrganizationID(adminCtx, organizationID)
		if listErr != nil {
			return listErr
		}
		result = accounts
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *operatorProvisioningService) ListAllAccounts(ctx context.Context) ([]authModels.OrgAccountInfo, error) {
	var result []authModels.OrgAccountInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		accounts, listErr := s.AccountTenantRepo.ListAllAccounts(adminCtx)
		if listErr != nil {
			return listErr
		}
		result = accounts
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// queryDevices runs the shared device listing through the summaries
// repository and computes the derived presentation fields.
func (s *operatorProvisioningService) queryDevices(adminCtx context.Context, filter platform.OperatorDeviceFilter) ([]OperatorDeviceInfo, error) {
	result, err := s.SummariesRepo.ListDeviceRows(adminCtx, filter)
	if err != nil {
		return nil, err
	}
	return enrichDeviceInfo(result), nil
}

func (s *operatorProvisioningService) ListAllDevices(ctx context.Context) ([]OperatorDeviceInfo, error) {
	var result []OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		var queryErr error
		result, queryErr = s.queryDevices(adminCtx, platform.OperatorDeviceFilter{})
		return queryErr
	})
	return result, err
}

func (s *operatorProvisioningService) ListSchoolDevices(ctx context.Context, schoolID int64) ([]OperatorDeviceInfo, error) {
	var result []OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, findErr := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if school.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: schoolID}
		}
		var queryErr error
		result, queryErr = s.queryDevices(adminCtx, platform.OperatorDeviceFilter{SchoolID: &schoolID})
		return queryErr
	})
	return result, err
}

func (s *operatorProvisioningService) ListOrganizationDevices(ctx context.Context, organizationID int64) ([]OperatorDeviceInfo, error) {
	var result []OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, findErr := s.Organizations.FindOrganization(adminCtx, organizationID)
		if findErr != nil {
			return mapOrganizationCapabilityError(findErr, organizationID)
		}
		if org.IsDeleted() {
			return &OrganizationDeletedError{OrganizationID: organizationID}
		}
		var queryErr error
		result, queryErr = s.queryDevices(adminCtx, platform.OperatorDeviceFilter{OrganizationID: &organizationID})
		return queryErr
	})
	return result, err
}

// resolveAPIKey returns the provided key if non-empty, otherwise auto-generates one.
func (s *operatorProvisioningService) resolveAPIKey(apiKey *string) (string, error) {
	if apiKey != nil && *apiKey != "" {
		return *apiKey, nil
	}
	return randstr.APIKey()
}

// generateRandomSuffix creates a cryptographically random lowercase alphanumeric
// string of the given length.
func generateRandomSuffix(length int) string {
	s, _ := randstr.String(length, randstr.LowerAlphanumeric)
	return s
}

// isAPIKeyConstraintViolation checks whether a unique violation is on the api_key column.
// Uses pgErr.Field('n') for the PostgreSQL constraint name (not error message matching).
// The constraint name "devices_api_key_key" is auto-generated by PG from the migration
// at database/migrations/001003009_iot_devices.go:76. Update this if the constraint is renamed.
func isAPIKeyConstraintViolation(err error) bool {
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('n') == "devices_api_key_key"
	}
	return false
}

// queryDeviceSingle wraps queryDevices and returns a single result with zero-row guard.
func (s *operatorProvisioningService) queryDeviceSingle(adminCtx context.Context, op string, deviceRowID int64) (*OperatorDeviceInfo, error) {
	devices, err := s.queryDevices(adminCtx, platform.OperatorDeviceFilter{DeviceRowID: &deviceRowID})
	if err != nil {
		return nil, fmt.Errorf("%s: re-query failed: %w", op, err)
	}
	if len(devices) == 0 {
		s.getLogger().Error("device not found after successful write",
			slog.String("op", op),
			slog.Int64("device_row_id", deviceRowID),
		)
		return nil, fmt.Errorf("%s: device not found after write (inconsistent state)", op)
	}
	return &devices[0], nil
}

const maxAPIKeyRetries = 3

func (s *operatorProvisioningService) CreateDevice(ctx context.Context, schoolID int64, deviceID, deviceType string, name, apiKey *string, operatorID int64, clientIP net.IP) (*OperatorDeviceInfo, error) {
	if schoolID <= 0 {
		return nil, &InvalidDataError{Err: fmt.Errorf("school_id is required")}
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, &InvalidDataError{Err: fmt.Errorf("device_id is required")}
	}
	if strings.TrimSpace(deviceType) == "" {
		return nil, &InvalidDataError{Err: fmt.Errorf("device_type is required")}
	}

	isManual := apiKey != nil && *apiKey != ""

	var result *OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, findErr := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if school.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: schoolID}
		}
		if !school.Active {
			return &SchoolInactiveError{SchoolID: schoolID}
		}

		device := &iotModels.Device{
			DeviceID:   strings.TrimSpace(deviceID),
			DeviceType: strings.TrimSpace(deviceType),
			Name:       name,
			Status:     iotModels.DeviceStatusActive,
		}
		device.SetTenantID(schoolID)

		deviceCtx := tenant.WithTenantID(adminCtx, schoolID)

		var created bool
		for attempt := 0; attempt < maxAPIKeyRetries; attempt++ {
			key, genErr := s.resolveAPIKey(apiKey)
			if genErr != nil {
				return fmt.Errorf("CreateDevice: generate API key: %w", genErr)
			}
			device.APIKey = &key

			createErr := s.DeviceRepo.Create(deviceCtx, device)
			if createErr == nil {
				created = true
				break
			}
			if modelBase.IsUniqueViolation(createErr) {
				if isAPIKeyConstraintViolation(createErr) {
					if isManual {
						return &ConflictError{Err: fmt.Errorf("api_key already in use")}
					}
					continue
				}
				return &ConflictError{Err: fmt.Errorf("device_id already exists for this school")}
			}
			return createErr
		}
		if !created {
			return fmt.Errorf("CreateDevice: failed to generate unique API key after %d attempts", maxAPIKeyRetries)
		}

		apiKeyMode := "auto"
		if isManual {
			apiKeyMode = "manual"
		}
		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceDevice, &device.ID, clientIP, map[string]any{
			"device_id":    device.DeviceID,
			"device_type":  device.DeviceType,
			"school_id":    schoolID,
			"api_key_mode": apiKeyMode,
		})

		var queryErr error
		result, queryErr = s.queryDeviceSingle(adminCtx, "CreateDevice", device.ID)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *operatorProvisioningService) SetDeviceAPIKey(ctx context.Context, id int64, apiKey *string, operatorID int64, clientIP net.IP) (*OperatorDeviceInfo, error) {
	if id <= 0 {
		return nil, &InvalidDataError{Err: fmt.Errorf("device id is required")}
	}

	isManual := apiKey != nil && *apiKey != ""

	var result *OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		// FindByID works cross-tenant in admin tx: applyTenantFilter (base.go:36-43)
		// only adds WHERE when tenant.FromContext(ctx) > 0. WithAdminTx does not set
		// tenant context (tenant/tx.go:63-75). Admin role bypasses RLS.
		device, findErr := s.DeviceRepo.FindByID(adminCtx, id)
		if findErr != nil {
			if errors.Is(findErr, sql.ErrNoRows) {
				return &OperatorDeviceNotFoundError{DeviceID: id}
			}
			var dbErr *modelBase.DatabaseError
			if errors.As(findErr, &dbErr) && errors.Is(dbErr.Err, sql.ErrNoRows) {
				return &OperatorDeviceNotFoundError{DeviceID: id}
			}
			return findErr
		}
		if device == nil {
			return &OperatorDeviceNotFoundError{DeviceID: id}
		}

		// Reject key rotation for devices in inactive schools.
		school, schoolErr := s.SchoolRepo.FindByID(adminCtx, device.TenantID)
		if schoolErr != nil {
			return fmt.Errorf("SetDeviceAPIKey: lookup school: %w", schoolErr)
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: device.TenantID}
		}
		if school.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: device.TenantID}
		}
		if !school.Active {
			return &SchoolInactiveError{SchoolID: device.TenantID}
		}

		var updated bool
		for attempt := 0; attempt < maxAPIKeyRetries; attempt++ {
			key, genErr := s.resolveAPIKey(apiKey)
			if genErr != nil {
				return fmt.Errorf("SetDeviceAPIKey: generate API key: %w", genErr)
			}
			device.APIKey = &key

			// Update also works cross-tenant in admin context (base.go:142-146, WherePK).
			updateErr := s.DeviceRepo.Update(adminCtx, device)
			if updateErr == nil {
				updated = true
				break
			}
			if modelBase.IsUniqueViolation(updateErr) && isAPIKeyConstraintViolation(updateErr) {
				if isManual {
					return &ConflictError{Err: fmt.Errorf("api_key already in use")}
				}
				continue
			}
			return updateErr
		}
		if !updated {
			return fmt.Errorf("SetDeviceAPIKey: failed to generate unique API key after %d attempts", maxAPIKeyRetries)
		}

		apiKeyMode := "auto"
		if isManual {
			apiKeyMode = "manual"
		}
		s.logAction(adminCtx, operatorID, platform.ActionRotateAPIKey, platform.ResourceDevice, &id, clientIP, map[string]any{
			"device_id":    device.DeviceID,
			"school_id":    device.TenantID,
			"api_key_mode": apiKeyMode,
		})

		var queryErr error
		result, queryErr = s.queryDeviceSingle(adminCtx, "SetDeviceAPIKey", id)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *operatorProvisioningService) DeleteDevice(ctx context.Context, id int64, operatorID int64, clientIP net.IP) error {
	if id <= 0 {
		return &InvalidDataError{Err: fmt.Errorf("device id is required")}
	}

	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		device, findErr := s.DeviceRepo.FindByID(adminCtx, id)
		if findErr != nil {
			if errors.Is(findErr, sql.ErrNoRows) {
				return &OperatorDeviceNotFoundError{DeviceID: id}
			}
			var dbErr *modelBase.DatabaseError
			if errors.As(findErr, &dbErr) && errors.Is(dbErr.Err, sql.ErrNoRows) {
				return &OperatorDeviceNotFoundError{DeviceID: id}
			}
			return findErr
		}
		if device == nil {
			return &OperatorDeviceNotFoundError{DeviceID: id}
		}

		// Prevent deletion of system-managed virtual devices (e.g. WEB-MANUAL-001).
		if device.DeviceID == iotModels.WebManualDeviceID {
			return &DeviceProtectedError{DeviceID: id, Reason: "system device required for manual web check-ins"}
		}

		deleteErr := s.DeviceRepo.Delete(adminCtx, id)
		if deleteErr != nil {
			// ON DELETE RESTRICT on active.attendance / active.groups → FK violation.
			if isForeignKeyViolation(deleteErr) {
				return &DeviceInUseError{DeviceID: id}
			}
			return fmt.Errorf("DeleteDevice: %w", deleteErr)
		}

		s.logAction(adminCtx, operatorID, platform.ActionDelete, platform.ResourceDevice, &id, clientIP, map[string]any{
			"device_id":   device.DeviceID,
			"device_type": device.DeviceType,
			"school_id":   device.TenantID,
		})

		return nil
	})
}

// deviceOnlineWindow resolves the tenant's iot.device_online_window_minutes
// setting, falling back to defaultDeviceOnlineWindow when no resolver is wired
// or the lookup fails. ctx must be the outer request context, not the admin-tx
// context: ResolveIntForTenant opens its own tenant transaction, which the
// nested-transaction guard rejects inside an admin transaction.
func (s *operatorProvisioningService) deviceOnlineWindow(ctx context.Context, tenantID int64) time.Duration {
	if s.Settings == nil {
		return defaultDeviceOnlineWindow
	}
	minutes, err := s.Settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyDeviceOnlineWindowMinutes)
	if err != nil {
		s.getLogger().Warn("device transfer: resolve online window failed, using fallback",
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return defaultDeviceOnlineWindow
	}
	if minutes <= 0 {
		return defaultDeviceOnlineWindow
	}
	return time.Duration(minutes) * time.Minute
}

func (s *operatorProvisioningService) transferStatus(ctx, adminCtx context.Context, device *iotModels.Device) (*DeviceTransferStatus, error) {
	status := &DeviceTransferStatus{LastSeen: device.LastSeen}
	if device.DeviceID == iotModels.WebManualDeviceID {
		status.IsProtected = true
		return status, nil
	}
	if device.LastSeen != nil {
		status.IsOnline = time.Since(*device.LastSeen) <= s.deviceOnlineWindow(ctx, device.TenantID)
	}
	if s.ActiveGroupRepo == nil {
		return nil, fmt.Errorf("device transfer: active group repository is not configured")
	}
	group, err := s.ActiveGroupRepo.FindActiveByDeviceIDWithNames(adminCtx, device.ID)
	if err != nil {
		return nil, fmt.Errorf("device transfer: find active session: %w", err)
	}
	if group != nil {
		session := &DeviceTransferSession{ID: group.ID, StartedAt: group.StartTime}
		if group.ActualGroup != nil {
			session.ActivityName = &group.ActualGroup.Name
		}
		if group.Room != nil {
			session.RoomName = &group.Room.Name
		}
		status.ActiveSession = session
	}
	status.CanTransfer = !status.IsOnline && status.ActiveSession == nil
	return status, nil
}

// GetDeviceTransferStatus returns current blockers without mutating the device.
func (s *operatorProvisioningService) GetDeviceTransferStatus(ctx context.Context, id int64) (*DeviceTransferStatus, error) {
	if id <= 0 {
		return nil, &InvalidDataError{Err: fmt.Errorf("device id is required")}
	}
	var result *DeviceTransferStatus
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		device, findErr := s.DeviceRepo.FindByID(adminCtx, id)
		if findErr != nil {
			if isLookupNotFound(findErr) {
				return &OperatorDeviceNotFoundError{DeviceID: id}
			}
			return findErr
		}
		if device == nil {
			return &OperatorDeviceNotFoundError{DeviceID: id}
		}
		var statusErr error
		result, statusErr = s.transferStatus(ctx, adminCtx, device)
		return statusErr
	})
	return result, err
}

func (s *operatorProvisioningService) loadTransferSource(adminCtx context.Context, id, targetSchoolID int64) (*iotModels.Device, error) {
	source, err := s.DeviceRepo.FindByIDForUpdate(adminCtx, id)
	if err != nil {
		if isLookupNotFound(err) {
			return nil, &OperatorDeviceNotFoundError{DeviceID: id}
		}
		return nil, err
	}
	if source == nil {
		return nil, &OperatorDeviceNotFoundError{DeviceID: id}
	}
	if source.DeviceID == iotModels.WebManualDeviceID {
		return nil, &DeviceTransferProtectedError{DeviceID: id, Reason: "system device required for manual web check-ins"}
	}
	if source.TenantID == targetSchoolID {
		return nil, &DeviceTransferSameSchoolError{SchoolID: targetSchoolID}
	}
	return source, nil
}

func (s *operatorProvisioningService) validateTransferDestination(adminCtx context.Context, source *iotModels.Device, targetSchoolID int64) error {
	sourceSchool, err := s.SchoolRepo.FindByID(adminCtx, source.TenantID)
	if err != nil {
		if isLookupNotFound(err) {
			return &SchoolNotFoundError{SchoolID: source.TenantID}
		}
		return fmt.Errorf("TransferDevice: lookup source school: %w", err)
	}
	if sourceSchool == nil {
		return &SchoolNotFoundError{SchoolID: source.TenantID}
	}

	targetSchool, err := s.SchoolRepo.FindByID(adminCtx, targetSchoolID)
	if err != nil {
		if isLookupNotFound(err) {
			return &SchoolNotFoundError{SchoolID: targetSchoolID}
		}
		return fmt.Errorf("TransferDevice: lookup target school: %w", err)
	}
	if targetSchool == nil {
		return &SchoolNotFoundError{SchoolID: targetSchoolID}
	}
	if targetSchool.IsDeleted() {
		return &SchoolAlreadyDeletedError{SchoolID: targetSchoolID}
	}
	if !targetSchool.Active {
		return &SchoolInactiveError{SchoolID: targetSchoolID}
	}
	if sourceSchool.OrganizationID != targetSchool.OrganizationID {
		return &DeviceTransferOrganizationMismatchError{SourceSchoolID: source.TenantID, TargetSchoolID: targetSchoolID}
	}
	return nil
}

func (s *operatorProvisioningService) ensureDeviceTransferable(ctx, adminCtx context.Context, source *iotModels.Device) error {
	status, err := s.transferStatus(ctx, adminCtx, source)
	if err != nil {
		return err
	}
	if status.IsOnline {
		return &DeviceTransferBlockedError{DeviceID: source.ID, Reason: DeviceTransferBlockedOnline}
	}
	if status.ActiveSession != nil {
		return &DeviceTransferBlockedError{DeviceID: source.ID, Reason: DeviceTransferBlockedActiveSession}
	}
	return nil
}

func (s *operatorProvisioningService) archiveAndCreateTransferredDevice(adminCtx context.Context, source *iotModels.Device, targetSchoolID int64) (*iotModels.Device, error) {
	originalStatus := source.Status
	var transferredAPIKey *string
	if source.APIKey != nil {
		key := *source.APIKey
		transferredAPIKey = &key
	}

	now := time.Now()
	source.ArchivedAt = &now
	source.APIKey = nil
	source.Status = iotModels.DeviceStatusInactive
	source.RoomID = nil
	source.RegisteredByID = nil
	if err := s.DeviceRepo.Update(adminCtx, source); err != nil {
		return nil, fmt.Errorf("TransferDevice: archive source: %w", err)
	}

	target := &iotModels.Device{
		DeviceID:   source.DeviceID,
		DeviceType: source.DeviceType,
		Name:       source.Name,
		Status:     originalStatus,
		APIKey:     transferredAPIKey,
	}
	target.SetTenantID(targetSchoolID)
	if err := s.DeviceRepo.Create(tenant.WithTenantID(adminCtx, targetSchoolID), target); err != nil {
		if modelBase.IsUniqueViolation(err) {
			if isAPIKeyConstraintViolation(err) {
				return nil, &ConflictError{Err: fmt.Errorf("api_key already in use")}
			}
			return nil, &ConflictError{Err: fmt.Errorf("device_id already exists for target school")}
		}
		return nil, fmt.Errorf("TransferDevice: create target: %w", err)
	}

	source.TransferredToDeviceID = &target.ID
	if err := s.DeviceRepo.Update(adminCtx, source); err != nil {
		return nil, fmt.Errorf("TransferDevice: link source history: %w", err)
	}
	return target, nil
}

// TransferDevice moves the current device identity and API key to another school
// while retaining the archived source row for historical foreign keys.
func (s *operatorProvisioningService) TransferDevice(ctx context.Context, id, targetSchoolID, operatorID int64, clientIP net.IP) (*OperatorDeviceInfo, error) {
	if id <= 0 || targetSchoolID <= 0 {
		return nil, &InvalidDataError{Err: fmt.Errorf("device id and target_school_id are required")}
	}

	var result *OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		source, err := s.loadTransferSource(adminCtx, id, targetSchoolID)
		if err != nil {
			return err
		}
		if err := s.validateTransferDestination(adminCtx, source, targetSchoolID); err != nil {
			return err
		}
		if err := s.ensureDeviceTransferable(ctx, adminCtx, source); err != nil {
			return err
		}
		target, err := s.archiveAndCreateTransferredDevice(adminCtx, source, targetSchoolID)
		if err != nil {
			return err
		}

		s.logAction(adminCtx, operatorID, platform.ActionTransfer, platform.ResourceDevice, &id, clientIP, map[string]any{
			"device_id":        source.DeviceID,
			"source_device_id": id,
			"target_device_id": target.ID,
			"source_school_id": source.TenantID,
			"target_school_id": targetSchoolID,
		})

		var queryErr error
		result, queryErr = s.queryDeviceSingle(adminCtx, "TransferDevice", target.ID)
		return queryErr
	})
	return result, err
}

func (s *operatorProvisioningService) ListSchoolPersons(ctx context.Context, schoolID int64) ([]OperatorPersonInfo, error) {
	var result []OperatorPersonInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, findErr := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}

		persons, scanErr := s.SummariesRepo.PersonsBySchool(adminCtx, schoolID)
		if scanErr != nil {
			return scanErr
		}
		result = persons
		return nil
	})
	return result, err
}

func (s *operatorProvisioningService) SoftDeletePerson(ctx context.Context, personID int64, operatorID int64, clientIP net.IP) error {
	if personID <= 0 {
		return &InvalidDataError{Err: fmt.Errorf("person id is required")}
	}

	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		// Find person. Cross-tenant by design: the admin context carries no
		// tenant ID, so the repository's tenant filter is a no-op, and BUN's
		// soft-delete handling auto-excludes deleted rows.
		person, err := s.PersonRepo.FindByID(adminCtx, personID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &PersonNotFoundError{PersonID: personID}
			}
			return fmt.Errorf("SoftDeletePerson: find person: %w", err)
		}

		// If person is staff, check for active supervisions
		staff, staffErr := s.StaffRepo.FindByPersonID(adminCtx, personID)
		if staffErr == nil && staff != nil {
			// Staff exists — check active supervisions
			if s.GroupSupervisorRepo != nil {
				supervisors, supErr := s.GroupSupervisorRepo.FindActiveByStaffID(adminCtx, staff.ID)
				if supErr != nil {
					s.getLogger().Warn("soft_delete_supervision_check_failed",
						slog.Int64("person_id", personID),
						slog.Int64("staff_id", staff.ID),
						slog.Any("error", supErr),
					)
				} else if len(supervisors) > 0 {
					return &PersonHasActiveSupervisionsError{PersonID: personID, Count: len(supervisors)}
				}
			}
		}

		// Unlink RFID card
		if person.TagID != nil {
			if err := s.PersonRepo.UnlinkFromRFIDCard(adminCtx, personID); err != nil {
				return fmt.Errorf("SoftDeletePerson: unlink rfid: %w", err)
			}
		}

		// Deactivate account + anonymize email
		if person.AccountID != nil {
			accountID := *person.AccountID

			// Deactivate account and delete tokens
			if deactivateErr := s.AuthService.DeactivateAccount(adminCtx, int(accountID)); deactivateErr != nil {
				s.getLogger().Warn("soft_delete_account_deactivation_failed",
					slog.Int64("person_id", personID),
					slog.Int64("account_id", accountID),
					slog.Any("error", deactivateErr),
				)
			}

			// Anonymize email
			anonymizedEmail := fmt.Sprintf("deleted-%d@anonymized.local", personID)
			if err := s.AccountRepo.AnonymizeForDeletion(adminCtx, accountID, anonymizedEmail); err != nil {
				return fmt.Errorf("SoftDeletePerson: anonymize account: %w", err)
			}

			// Unlink account from person
			if err := s.PersonRepo.UnlinkFromAccount(adminCtx, personID); err != nil {
				return fmt.Errorf("SoftDeletePerson: unlink account: %w", err)
			}
		}

		// Anonymize PII and soft delete
		if err := s.PersonRepo.AnonymizeAndSoftDelete(adminCtx, personID); err != nil {
			return fmt.Errorf("SoftDeletePerson: anonymize and soft delete: %w", err)
		}

		s.logAction(adminCtx, operatorID, platform.ActionSoftDelete, platform.ResourcePerson, &personID, clientIP, map[string]any{
			"person_id": personID,
			"school_id": person.TenantID,
		})

		s.getLogger().Info("person_soft_deleted",
			slog.Int64("person_id", personID),
			slog.Int64("school_id", person.TenantID),
			slog.Int64("operator_id", operatorID),
		)

		return nil
	})
}

func (s *operatorProvisioningService) validateSchoolCreate(ctx context.Context, school *platform.School) error {
	org, err := s.Organizations.FindOrganizationForSchoolMutation(ctx, school.OrganizationID)
	if err != nil {
		return mapOrganizationCapabilityError(err, school.OrganizationID)
	}
	if org.IsDeleted() {
		return &OrganizationDeletedError{OrganizationID: school.OrganizationID}
	}
	if err := s.ensureSchoolSlugAvailable(ctx, school.OrganizationID, school.Slug); err != nil {
		return err
	}
	return s.ensureSchoolSubdomainAvailable(ctx, school.Subdomain)
}

func (s *operatorProvisioningService) ensureSchoolSlugAvailable(ctx context.Context, organizationID int64, slug string) error {
	existing, err := s.SchoolRepo.FindByOrganizationAndSlug(ctx, organizationID, slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return &ConflictError{Err: fmt.Errorf("school slug already exists in this organization")}
	}
	return nil
}

func (s *operatorProvisioningService) ensureSchoolSubdomainAvailable(ctx context.Context, subdomain string) error {
	existing, err := s.SchoolRepo.FindBySubdomain(ctx, subdomain)
	if err != nil {
		return err
	}
	if existing != nil {
		return &ConflictError{Err: fmt.Errorf("school subdomain already exists")}
	}
	return nil
}

func (s *operatorProvisioningService) resolveAdminInviteContext(ctx context.Context, schoolID int64) (*platform.School, *authModels.Role, error) {
	school, err := s.loadActiveSchool(ctx, schoolID)
	if err != nil {
		return nil, nil, err
	}
	adminRole, err := authSvc.ResolveSystemRoleByName(ctx, s.RoleRepo, "admin")
	if err != nil {
		return nil, nil, err
	}
	if adminRole == nil {
		return nil, nil, &InvalidDataError{Err: fmt.Errorf("admin role not found")}
	}
	return school, adminRole, nil
}

func (s *operatorProvisioningService) loadActiveSchool(ctx context.Context, schoolID int64) (*platform.School, error) {
	school, err := s.SchoolRepo.FindByID(ctx, schoolID)
	if err != nil {
		if isLookupNotFound(err) {
			return nil, &SchoolNotFoundError{SchoolID: schoolID}
		}
		return nil, err
	}
	if school == nil {
		return nil, &SchoolNotFoundError{SchoolID: schoolID}
	}
	if school.IsDeleted() {
		return nil, &SchoolAlreadyDeletedError{SchoolID: schoolID}
	}
	if !school.Active {
		return nil, &InvalidDataError{Err: fmt.Errorf("school is inactive")}
	}
	return school, nil
}

func normalizeAdminInviteRequest(req authSvc.InvitationRequest, roleID, tenantID int64) authSvc.InvitationRequest {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.RoleID = roleID
	req.TenantID = tenantID
	// Handing out the school admin role is the whole point of this flow, and the
	// caller is operator-authenticated (platform scope, no tenant permission
	// set), so the tenant-side role-grant check does not apply here.
	req.OperatorGrant = true
	return req
}

func (s *operatorProvisioningService) seedDefaultActivityCategories(ctx context.Context, tenantID int64) error {
	if s.CategoryRepo == nil || tenantID <= 0 {
		return nil
	}

	defaults := []activityModels.Category{
		{Name: "Sport", Description: "Sportliche Aktivitäten für Kinder", Color: "#7ED321"},
		{Name: "Kunst & Basteln", Description: "Kreative Aktivitäten und Handwerken", Color: "#F5A623"},
		{Name: "Musik", Description: "Musikalische Aktivitäten und Gesang", Color: "#BD10E0"},
		{Name: "Spiele", Description: "Brett-, Karten- und Gruppenspiele", Color: "#50E3C2"},
		{Name: "Lesen", Description: "Leseförderung und Literatur", Color: "#B8E986"},
		{Name: "Hausaufgabenhilfe", Description: "Unterstützung bei den Hausaufgaben", Color: "#4A90E2"},
		{Name: "Natur & Forschen", Description: "Naturerkundung und einfache Experimente", Color: "#7ED321"},
		{Name: "Computer", Description: "Grundlagen im Umgang mit dem Computer", Color: "#9013FE"},
		{Name: "Gruppenraum", Description: "Aktivitäten im Gruppenraum", Color: "#FF6900"},
		// Essenszeiten need a fitting Pflichtkategorie when a Termin is
		// created. Mensa existed in the pre-multi-tenant seed but was missing
		// from this list, so every operator-provisioned school lacked it
		// (#2131). Migration 1.15.260 backfills the schools created before
		// this line; keep the three values in sync with it.
		{Name: "Mensa", Description: "Aktivitäten rund um das Mittagessen", Color: "#FF9500"},
	}

	categoryCtx := tenant.WithTenantID(ctx, tenantID)
	for i := range defaults {
		category := defaults[i]
		if err := s.CategoryRepo.Create(categoryCtx, &category); err != nil {
			if modelBase.IsUniqueViolation(err) {
				continue
			}
			return err
		}
	}

	return nil
}

func (s *operatorProvisioningService) createWebManualDevice(ctx context.Context, tenantID int64) error {
	if s.DeviceRepo == nil || tenantID <= 0 {
		return nil
	}

	deviceName := "Web-Portal (Manuell)"
	device := &iotModels.Device{
		DeviceID:   iotModels.WebManualDeviceID,
		DeviceType: iotModels.DeviceTypeVirtual,
		Name:       &deviceName,
		Status:     iotModels.DeviceStatusActive,
	}
	device.SetTenantID(tenantID)

	deviceCtx := tenant.WithTenantID(ctx, tenantID)
	if err := s.DeviceRepo.Create(deviceCtx, device); err != nil {
		if modelBase.IsUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("create web manual device for tenant %d: %w", tenantID, err)
	}

	s.getLogger().Info("created web manual device for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.String("device_id", iotModels.WebManualDeviceID),
	)
	return nil
}

func (s *operatorProvisioningService) logAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) {
	if err := s.recordAction(ctx, operatorID, action, resourceType, resourceID, clientIP, changes); err != nil {
		s.getLogger().Error(
			"failed to create operator audit log",
			slog.Any("error", err),
			slog.String("resource_type", resourceType),
		)
	}
}

func (s *operatorProvisioningService) recordAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) error {
	entry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestIP:    clientIP,
	}
	if len(changes) > 0 {
		payload, err := json.Marshal(changes)
		if err != nil {
			return fmt.Errorf("encode operator audit log changes: %w", err)
		}
		entry.Changes = payload
	}
	if s.AuditLogRepo == nil {
		return errors.New("operator audit log repository is required")
	}
	if err := s.AuditLogRepo.Create(ctx, entry); err != nil {
		return fmt.Errorf("create operator audit log: %w", err)
	}
	return nil
}

// isLookupNotFound reports whether a repository FindByID-style error means
// "no such row" (directly or wrapped in a DatabaseError), regardless of entity.
func isLookupNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}

// isRowsAffectedMismatch returns true when a DatabaseError wraps a "expected N rows affected, got M"
// failure from AssertRowsAffected. This happens when a concurrent operation changed the row between
// our read and our conditional UPDATE (e.g. soft-delete WHERE deleted_at IS NULL).
func isRowsAffectedMismatch(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Err != nil && strings.Contains(dbErr.Err.Error(), "rows affected")
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation() && pgErr.Field('C') == "23503"
	}
	return false
}

// SoftDeleteSchool marks a school as deleted. The school remains in the database but is excluded
// from login, tenant resolution, and all tenant-scoped operations.
//
// Session handling after soft-delete:
//   - New logins: blocked immediately (resolveAccountTenantBySlug + resolveAccountTenantDefault
//     both reject deleted schools)
//   - IoT devices: blocked immediately (rejectDeletedSchool checks deleted_at on every request,
//     since devices use long-lived API keys that don't expire)
//   - Refresh tokens: revoked immediately via bulk DELETE from auth.tokens. As a second
//     layer, validateTenantAccess checks school.deleted_at on every refresh attempt,
//     catching tokens that a concurrent refresh may have inserted after the bulk DELETE.
//   - Existing JWT sessions: drain naturally within the 15-min access token TTL. The JWT
//     middleware trusts token claims without a DB lookup per request.
//   - Pending invitations: invalidated immediately (marked as used, so invite links can no
//     longer be redeemed for the deleted school)
func (s *operatorProvisioningService) SoftDeleteSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, err := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if err != nil {
			if isLookupNotFound(err) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return err
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if school.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: schoolID}
		}

		if err := s.SchoolRepo.SoftDelete(adminCtx, schoolID); err != nil {
			// If another operator concurrently deleted this school between our read and
			// update, the WHERE deleted_at IS NULL clause matches zero rows. Map that
			// race to the same conflict error the pre-check would have returned.
			if isRowsAffectedMismatch(err) {
				return &SchoolAlreadyDeletedError{SchoolID: schoolID}
			}
			return err
		}

		// Revoke all refresh tokens for the deleted school so users cannot
		// obtain new access tokens via /auth/refresh after the 15-min drain.
		// These steps are fatal: if they fail the transaction rolls back so we
		// never commit a soft-delete without actually revoking access.
		var revokedTokens int
		if s.AuthService != nil {
			revokedTokens, err = s.AuthService.RevokeTokensByTenantID(adminCtx, schoolID)
			if err != nil {
				return fmt.Errorf("revoke tokens for school %d: %w", schoolID, err)
			}
		}

		// Invalidate all pending invitations so they cannot be redeemed after deletion.
		var invalidatedInvitations int
		if s.InvitationService != nil {
			invalidatedInvitations, err = s.InvitationService.InvalidatePendingInvitationsByTenantID(adminCtx, schoolID)
			if err != nil {
				return fmt.Errorf("invalidate invitations for school %d: %w", schoolID, err)
			}
		}

		s.logAction(adminCtx, operatorID, platform.ActionSoftDelete, platform.ResourceSchool, &schoolID, clientIP, map[string]any{
			"name":                school.Name,
			"slug":                school.Slug,
			"subdomain":           school.Subdomain,
			"revoked_tokens":      revokedTokens,
			"invalidated_invites": invalidatedInvitations,
		})
		return nil
	})
}

// RestoreSchool returns a soft-deleted school to its pre-deletion state.
// The active field is preserved — a school that was inactive before deletion remains inactive after restore.
func (s *operatorProvisioningService) RestoreSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, err := s.SchoolRepo.FindByID(adminCtx, schoolID)
		if err != nil {
			if isLookupNotFound(err) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return err
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if !school.IsDeleted() {
			return &SchoolNotDeletedError{SchoolID: schoolID}
		}

		parentOrg, orgErr := s.Organizations.FindOrganizationForSchoolMutation(adminCtx, school.OrganizationID)
		if orgErr != nil {
			return mapOrganizationCapabilityError(orgErr, school.OrganizationID)
		}
		if parentOrg.IsDeleted() {
			return &OrganizationDeletedError{OrganizationID: school.OrganizationID}
		}

		if err := s.SchoolRepo.Restore(adminCtx, schoolID); err != nil {
			// If another operator concurrently restored this school between our read and
			// update, the WHERE deleted_at IS NOT NULL clause matches zero rows. Map that
			// race to the same conflict error the pre-check would have returned.
			if isRowsAffectedMismatch(err) {
				return &SchoolNotDeletedError{SchoolID: schoolID}
			}
			return err
		}

		s.logAction(adminCtx, operatorID, platform.ActionRestore, platform.ResourceSchool, &schoolID, clientIP, map[string]any{
			"name":      school.Name,
			"slug":      school.Slug,
			"subdomain": school.Subdomain,
		})
		return nil
	})
}

// SoftDeleteOrganization marks an organization as deleted. Blocked if the organization still
// has non-deleted schools — the operator must delete each school individually first.
func (s *operatorProvisioningService) SoftDeleteOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, err := s.Organizations.SoftDeleteOrganization(adminCtx, organizationID)
		if err != nil {
			return mapOrganizationCapabilityError(err, organizationID)
		}

		return s.recordAction(adminCtx, operatorID, platform.ActionSoftDelete, platform.ResourceOrganization, &organizationID, clientIP, map[string]any{
			"name": org.Name,
			"slug": org.Slug,
		})
	})
}

// RestoreOrganization returns a soft-deleted organization to its pre-deletion state.
func (s *operatorProvisioningService) RestoreOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, err := s.Organizations.RestoreOrganization(adminCtx, organizationID)
		if err != nil {
			return mapOrganizationCapabilityError(err, organizationID)
		}

		return s.recordAction(adminCtx, operatorID, platform.ActionRestore, platform.ResourceOrganization, &organizationID, clientIP, map[string]any{
			"name": org.Name,
			"slug": org.Slug,
		})
	})
}

func mapOrganizationCapabilityError(err error, organizationID int64) error {
	switch {
	case errors.Is(err, organizationModule.ErrOrganizationNotFound):
		return &OrganizationNotFoundError{OrganizationID: organizationID}
	case errors.Is(err, organizationModule.ErrOrganizationSlugConflict):
		return &ConflictError{Err: fmt.Errorf("organization slug already exists")}
	case errors.Is(err, organizationModule.ErrOrganizationAlreadyDeleted):
		return &OrganizationAlreadyDeletedError{OrganizationID: organizationID}
	case errors.Is(err, organizationModule.ErrOrganizationNotDeleted):
		return &OrganizationNotDeletedError{OrganizationID: organizationID}
	case errors.Is(err, organizationModule.ErrOrganizationHasSchools):
		var hasSchools *organizationModule.OrganizationHasSchoolsError
		if errors.As(err, &hasSchools) {
			return &OrganizationHasSchoolsError{OrganizationID: organizationID, SchoolCount: hasSchools.SchoolCount}
		}
		return &OrganizationHasSchoolsError{OrganizationID: organizationID}
	case errors.Is(err, organizationModule.ErrInvalidOrganization):
		return &InvalidDataError{Err: err}
	default:
		return err
	}
}

func mapSchoolCreateConflict(ctx context.Context, schoolRepo platform.SchoolRepository, school *platform.School) error {
	if school == nil {
		return &ConflictError{Err: fmt.Errorf("school already exists")}
	}
	if existing, err := schoolRepo.FindBySubdomain(ctx, school.Subdomain); err == nil && existing != nil {
		return &ConflictError{Err: fmt.Errorf("school subdomain already exists")}
	}
	if existing, err := schoolRepo.FindByOrganizationAndSlug(ctx, school.OrganizationID, school.Slug); err == nil && existing != nil {
		return &ConflictError{Err: fmt.Errorf("school slug already exists in this organization")}
	}
	return &ConflictError{Err: fmt.Errorf("school already exists")}
}
