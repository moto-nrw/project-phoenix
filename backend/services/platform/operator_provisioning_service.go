package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"strings"
	"time"

	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// CreateSchoolAccountRequest holds fields for operator-created school accounts.
type CreateSchoolAccountRequest struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	RoleID    *int64
	Position  string // optional, maps to Teacher.Role
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

// OperatorProvisioningService handles operator-led tenant provisioning.
type OperatorProvisioningService interface {
	CreateOrganization(ctx context.Context, organization *platform.Organization, operatorID int64, clientIP net.IP) (*platform.Organization, error)
	ListOrganizations(ctx context.Context) ([]*platform.Organization, error)
	UpdateOrganization(ctx context.Context, id int64, req UpdateOrganizationRequest, operatorID int64, clientIP net.IP) (*platform.Organization, error)
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
	SoftDeleteSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error
	RestoreSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error
	PurgeSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) ([]byte, error)
	PurgeDeletedSchools(ctx context.Context, olderThan time.Duration) (int, error)
}

// OperatorDeviceInfo holds device information with school/org context for operator views.
type OperatorDeviceInfo struct {
	ID               int64      `bun:"id" json:"id"`
	DeviceID         string     `bun:"device_id" json:"device_id"`
	DeviceType       string     `bun:"device_type" json:"device_type"`
	Name             *string    `bun:"name" json:"name,omitempty"`
	Status           string     `bun:"status" json:"status"`
	APIKey           *string    `bun:"api_key" json:"api_key,omitempty"`
	MaskedAPIKey     string     `bun:"-" json:"masked_api_key"`
	LastSeen         *time.Time `bun:"last_seen" json:"last_seen,omitempty"`
	IsOnline         bool       `bun:"-" json:"is_online"`
	SchoolID         int64      `bun:"school_id" json:"school_id"`
	SchoolName       string     `bun:"school_name" json:"school_name"`
	OrganizationID   int64      `bun:"organization_id" json:"organization_id"`
	OrganizationName string     `bun:"organization_name" json:"organization_name"`
	CreatedAt        time.Time  `bun:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `bun:"updated_at" json:"updated_at"`
}

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
	organizationRepo  platform.OrganizationRepository
	schoolRepo        platform.SchoolRepository
	categoryRepo      activityModels.CategoryRepository
	deviceRepo        iotModels.DeviceRepository
	roleRepo          authModels.RoleRepository
	accountTenantRepo authModels.AccountTenantRepository
	personRepo        userModels.PersonRepository
	staffRepo         userModels.StaffRepository
	teacherRepo       userModels.TeacherRepository
	invitationService authSvc.InvitationService
	authService       authSvc.AuthService
	auditLogRepo      platform.OperatorAuditLogRepository
	txHandler         *modelBase.TxHandler
	logger            *slog.Logger
}

// OperatorProvisioningServiceConfig holds dependencies for operator provisioning.
type OperatorProvisioningServiceConfig struct {
	OrganizationRepo  platform.OrganizationRepository
	SchoolRepo        platform.SchoolRepository
	CategoryRepo      activityModels.CategoryRepository
	DeviceRepo        iotModels.DeviceRepository
	RoleRepo          authModels.RoleRepository
	AccountTenantRepo authModels.AccountTenantRepository
	PersonRepo        userModels.PersonRepository
	StaffRepo         userModels.StaffRepository
	TeacherRepo       userModels.TeacherRepository
	InvitationService authSvc.InvitationService
	AuthService       authSvc.AuthService
	AuditLogRepo      platform.OperatorAuditLogRepository
	DB                *bun.DB
	Logger            *slog.Logger
}

// NewOperatorProvisioningService creates a provisioning service.
func NewOperatorProvisioningService(cfg OperatorProvisioningServiceConfig) OperatorProvisioningService {
	return &operatorProvisioningService{
		organizationRepo:  cfg.OrganizationRepo,
		schoolRepo:        cfg.SchoolRepo,
		categoryRepo:      cfg.CategoryRepo,
		deviceRepo:        cfg.DeviceRepo,
		roleRepo:          cfg.RoleRepo,
		accountTenantRepo: cfg.AccountTenantRepo,
		personRepo:        cfg.PersonRepo,
		staffRepo:         cfg.StaffRepo,
		teacherRepo:       cfg.TeacherRepo,
		invitationService: cfg.InvitationService,
		authService:       cfg.AuthService,
		auditLogRepo:      cfg.AuditLogRepo,
		txHandler:         modelBase.NewTxHandler(cfg.DB),
		logger:            cfg.Logger,
	}
}

func (s *operatorProvisioningService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

func (s *operatorProvisioningService) CreateOrganization(ctx context.Context, organization *platform.Organization, operatorID int64, clientIP net.IP) (*platform.Organization, error) {
	if organization == nil {
		return nil, &InvalidDataError{Err: fmt.Errorf("organization is required")}
	}
	if err := organization.Validate(); err != nil {
		return nil, &InvalidDataError{Err: err}
	}

	var created *platform.Organization
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		existing, findErr := s.organizationRepo.FindBySlug(adminCtx, organization.Slug)
		if findErr != nil {
			return findErr
		}
		if existing != nil {
			return &ConflictError{Err: fmt.Errorf("organization slug already exists")}
		}
		if createErr := s.organizationRepo.Create(adminCtx, organization); createErr != nil {
			if isUniqueViolation(createErr) {
				return &ConflictError{Err: fmt.Errorf("organization slug already exists")}
			}
			return createErr
		}
		s.logAction(adminCtx, operatorID, platform.ActionCreate, platform.ResourceOrganization, &organization.ID, clientIP, map[string]any{
			"name": organization.Name,
			"slug": organization.Slug,
		})
		created = organization
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *operatorProvisioningService) ListOrganizations(ctx context.Context) ([]*platform.Organization, error) {
	return s.organizationRepo.List(ctx)
}

func (s *operatorProvisioningService) UpdateOrganization(ctx context.Context, id int64, req UpdateOrganizationRequest, operatorID int64, clientIP net.IP) (*platform.Organization, error) {
	var updated *platform.Organization
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		existing, err := s.organizationRepo.FindByID(adminCtx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return &OrganizationNotFoundError{OrganizationID: id}
		}

		changes := map[string]any{}

		if req.Slug != existing.Slug {
			taken, findErr := s.organizationRepo.FindBySlug(adminCtx, req.Slug)
			if findErr != nil {
				return findErr
			}
			if taken != nil && taken.ID != id {
				return &ConflictError{Err: fmt.Errorf("organization slug already exists")}
			}
			changes["slug"] = map[string]string{"old": existing.Slug, "new": req.Slug}
		}
		if req.Name != existing.Name {
			changes["name"] = map[string]string{"old": existing.Name, "new": req.Name}
		}
		if req.Active != existing.Active {
			changes["active"] = map[string]bool{"old": existing.Active, "new": req.Active}
		}

		existing.Name = req.Name
		existing.Slug = req.Slug
		existing.Active = req.Active

		if updateErr := s.organizationRepo.Update(adminCtx, existing); updateErr != nil {
			if isUniqueViolation(updateErr) {
				return &ConflictError{Err: fmt.Errorf("organization slug already exists")}
			}
			return &InvalidDataError{Err: updateErr}
		}

		s.logAction(adminCtx, operatorID, platform.ActionUpdate, platform.ResourceOrganization, &id, clientIP, changes)
		updated = existing
		return nil
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
		if createErr := s.schoolRepo.Create(adminCtx, school); createErr != nil {
			if isUniqueViolation(createErr) {
				return mapSchoolCreateConflict(adminCtx, s.schoolRepo, school)
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
	return s.schoolRepo.List(ctx)
}

func (s *operatorProvisioningService) UpdateSchool(ctx context.Context, id int64, req UpdateSchoolRequest, operatorID int64, clientIP net.IP) (*platform.School, error) {
	var updated *platform.School
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		existing, err := s.schoolRepo.FindByID(adminCtx, id)
		if err != nil {
			if isSchoolLookupNotFound(err) {
				return &SchoolNotFoundError{SchoolID: id}
			}
			return err
		}
		if existing == nil {
			return &SchoolNotFoundError{SchoolID: id}
		}

		changes := map[string]any{}

		if req.OrganizationID != existing.OrganizationID {
			org, orgErr := s.organizationRepo.FindByID(adminCtx, req.OrganizationID)
			if orgErr != nil {
				return orgErr
			}
			if org == nil {
				return &OrganizationNotFoundError{OrganizationID: req.OrganizationID}
			}
			changes["organization_id"] = map[string]int64{"old": existing.OrganizationID, "new": req.OrganizationID}
		}

		targetOrgID := req.OrganizationID
		if req.Slug != existing.Slug || req.OrganizationID != existing.OrganizationID {
			taken, findErr := s.schoolRepo.FindByOrganizationAndSlug(adminCtx, targetOrgID, req.Slug)
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
			taken, findErr := s.schoolRepo.FindBySubdomain(adminCtx, req.Subdomain)
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

		if updateErr := s.schoolRepo.Update(adminCtx, existing); updateErr != nil {
			if isUniqueViolation(updateErr) {
				return mapSchoolCreateConflict(adminCtx, s.schoolRepo, existing)
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
		created, createErr := s.invitationService.CreateInvitation(invitationCtx, req)
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
		if roleID == nil {
			adminRole, roleErr := s.resolveSystemRoleByName(adminCtx, "admin")
			if roleErr != nil {
				return roleErr
			}
			if adminRole == nil {
				return &InvalidDataError{Err: fmt.Errorf("admin role not found")}
			}
			roleID = &adminRole.ID
		} else {
			// Validate that the provided role is a system role and not guardian
			role, roleErr := s.roleRepo.FindByID(adminCtx, *roleID)
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
		created, createErr := s.authService.Register(tenantCtx, req.Email, username, req.Password, roleID, school.ID)
		if createErr != nil {
			return createErr
		}
		account = created

		// Step 2: Create Person
		person := &userModels.Person{
			FirstName: req.FirstName,
			LastName:  req.LastName,
		}
		person.SetTenantID(school.ID)
		if err := s.personRepo.Create(tenantCtx, person); err != nil {
			return fmt.Errorf("create person: %w", err)
		}

		// Link person to account
		if err := s.personRepo.LinkToAccount(tenantCtx, person.ID, account.ID); err != nil {
			return fmt.Errorf("link person to account: %w", err)
		}

		// Step 3: Create Staff (only for system roles)
		if roleID != nil {
			role, roleErr := s.roleRepo.FindByID(tenantCtx, *roleID)
			if roleErr != nil {
				return fmt.Errorf("lookup role for staff creation: %w", roleErr)
			}
			if role != nil && role.IsSystem {
				staff := &userModels.Staff{PersonID: person.ID}
				staff.SetTenantID(school.ID)
				if err := s.staffRepo.Create(tenantCtx, staff); err != nil {
					return fmt.Errorf("create staff: %w", err)
				}

				// Create Teacher for "user" and "teacher" roles
				if shouldCreateTeacher(role.Name) {
					teacher := &userModels.Teacher{StaffID: staff.ID}
					teacher.SetTenantID(school.ID)
					if req.Position != "" {
						teacher.Role = req.Position
					}
					if err := s.teacherRepo.Create(tenantCtx, teacher); err != nil {
						return fmt.Errorf("create teacher: %w", err)
					}
				}
			}
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

func (s *operatorProvisioningService) ListSystemRoles(ctx context.Context) ([]*authModels.Role, error) {
	var result []*authModels.Role
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		roles, listErr := s.roleRepo.List(adminCtx, map[string]interface{}{"is_system": true})
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
		school, findErr := s.schoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isSchoolLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		accounts, listErr := s.accountTenantRepo.ListAccountsByTenantID(adminCtx, schoolID)
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
		org, findErr := s.organizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		accounts, listErr := s.accountTenantRepo.ListAccountsByOrganizationID(adminCtx, organizationID)
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
		accounts, listErr := s.accountTenantRepo.ListAllAccounts(adminCtx)
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

const operatorDeviceQuery = `
SELECT
	"d".id,
	"d".device_id,
	"d".device_type,
	"d".name,
	"d".status,
	"d".api_key,
	"d".last_seen,
	"d".created_at,
	"d".updated_at,
	"s".id AS school_id,
	"s".name AS school_name,
	"o".id AS organization_id,
	"o".name AS organization_name
FROM iot.devices AS "d"
INNER JOIN platform.schools AS "s" ON "s".id = "d".tenant_id
INNER JOIN platform.organizations AS "o" ON "o".id = "s".organization_id
`

// queryDevices runs the shared device query with an optional WHERE clause.
// All callers pass static WHERE strings with parameterized args — no user input is concatenated.
func (s *operatorProvisioningService) queryDevices(adminCtx context.Context, whereClause string, args ...interface{}) ([]OperatorDeviceInfo, error) {
	var db bun.IDB = s.txHandler.DB
	if tx, ok := modelBase.TxFromContext(adminCtx); ok && tx != nil {
		db = tx
	}
	q := operatorDeviceQuery
	if whereClause != "" {
		q += " WHERE " + whereClause
	}
	q += ` ORDER BY "o".name, "s".name, "d".device_id`

	var result []OperatorDeviceInfo
	if err := db.NewRaw(q, args...).Scan(adminCtx, &result); err != nil {
		return nil, err
	}
	if result == nil {
		result = []OperatorDeviceInfo{}
	}
	return enrichDeviceInfo(result), nil
}

func (s *operatorProvisioningService) ListAllDevices(ctx context.Context) ([]OperatorDeviceInfo, error) {
	var result []OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		var queryErr error
		result, queryErr = s.queryDevices(adminCtx, "")
		return queryErr
	})
	return result, err
}

func (s *operatorProvisioningService) ListSchoolDevices(ctx context.Context, schoolID int64) ([]OperatorDeviceInfo, error) {
	var result []OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, findErr := s.schoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isSchoolLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		var queryErr error
		result, queryErr = s.queryDevices(adminCtx, `"d".tenant_id = ?`, schoolID)
		return queryErr
	})
	return result, err
}

func (s *operatorProvisioningService) ListOrganizationDevices(ctx context.Context, organizationID int64) ([]OperatorDeviceInfo, error) {
	var result []OperatorDeviceInfo
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		org, findErr := s.organizationRepo.FindByID(adminCtx, organizationID)
		if findErr != nil {
			return findErr
		}
		if org == nil {
			return &OrganizationNotFoundError{OrganizationID: organizationID}
		}
		var queryErr error
		result, queryErr = s.queryDevices(adminCtx, `"o".id = ?`, organizationID)
		return queryErr
	})
	return result, err
}

// generateAPIKey creates a cryptographically random API key with "dev_" prefix.
// Duplicated from services/iot/iot_service.go to avoid coupling operator service to IoT service.
func (s *operatorProvisioningService) generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("dev_%s", hex.EncodeToString(bytes)), nil
}

// resolveAPIKey returns the provided key if non-empty, otherwise auto-generates one.
func (s *operatorProvisioningService) resolveAPIKey(apiKey *string) (string, error) {
	if apiKey != nil && *apiKey != "" {
		return *apiKey, nil
	}
	return s.generateAPIKey()
}

// generateRandomSuffix creates a cryptographically random alphanumeric string of the given length.
func generateRandomSuffix(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[n.Int64()]
	}
	return string(b)
}

// shouldCreateTeacher returns true for roles that should have a Teacher record.
func shouldCreateTeacher(roleName string) bool {
	switch strings.ToLower(strings.TrimSpace(roleName)) {
	case "user", "teacher":
		return true
	default:
		return false
	}
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
func (s *operatorProvisioningService) queryDeviceSingle(adminCtx context.Context, op, whereClause string, args ...interface{}) (*OperatorDeviceInfo, error) {
	devices, err := s.queryDevices(adminCtx, whereClause, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: re-query failed: %w", op, err)
	}
	if len(devices) == 0 {
		s.getLogger().Error("device not found after successful write",
			slog.String("op", op),
			slog.String("where", whereClause),
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
		school, findErr := s.schoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil {
			if isSchoolLookupNotFound(findErr) {
				return &SchoolNotFoundError{SchoolID: schoolID}
			}
			return findErr
		}
		if school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
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

			createErr := s.deviceRepo.Create(deviceCtx, device)
			if createErr == nil {
				created = true
				break
			}
			if isUniqueViolation(createErr) {
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
		result, queryErr = s.queryDeviceSingle(adminCtx, "CreateDevice", `"d".id = ?`, device.ID)
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
		device, findErr := s.deviceRepo.FindByID(adminCtx, id)
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
		school, schoolErr := s.schoolRepo.FindByID(adminCtx, device.TenantID)
		if schoolErr != nil {
			return fmt.Errorf("SetDeviceAPIKey: lookup school: %w", schoolErr)
		}
		if school == nil || !school.Active {
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
			updateErr := s.deviceRepo.Update(adminCtx, device)
			if updateErr == nil {
				updated = true
				break
			}
			if isUniqueViolation(updateErr) && isAPIKeyConstraintViolation(updateErr) {
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
		result, queryErr = s.queryDeviceSingle(adminCtx, "SetDeviceAPIKey", `"d".id = ?`, id)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *operatorProvisioningService) validateSchoolCreate(ctx context.Context, school *platform.School) error {
	org, err := s.organizationRepo.FindByID(ctx, school.OrganizationID)
	if err != nil {
		return err
	}
	if org == nil {
		return &OrganizationNotFoundError{OrganizationID: school.OrganizationID}
	}
	if err := s.ensureSchoolSlugAvailable(ctx, school.OrganizationID, school.Slug); err != nil {
		return err
	}
	return s.ensureSchoolSubdomainAvailable(ctx, school.Subdomain)
}

func (s *operatorProvisioningService) ensureSchoolSlugAvailable(ctx context.Context, organizationID int64, slug string) error {
	existing, err := s.schoolRepo.FindByOrganizationAndSlug(ctx, organizationID, slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return &ConflictError{Err: fmt.Errorf("school slug already exists in this organization")}
	}
	return nil
}

func (s *operatorProvisioningService) ensureSchoolSubdomainAvailable(ctx context.Context, subdomain string) error {
	existing, err := s.schoolRepo.FindBySubdomain(ctx, subdomain)
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
	adminRole, err := s.resolveSystemRoleByName(ctx, "admin")
	if err != nil {
		return nil, nil, err
	}
	if adminRole == nil {
		return nil, nil, &InvalidDataError{Err: fmt.Errorf("admin role not found")}
	}
	return school, adminRole, nil
}

func (s *operatorProvisioningService) loadActiveSchool(ctx context.Context, schoolID int64) (*platform.School, error) {
	school, err := s.schoolRepo.FindByID(ctx, schoolID)
	if err != nil {
		if isSchoolLookupNotFound(err) {
			return nil, &SchoolNotFoundError{SchoolID: schoolID}
		}
		return nil, err
	}
	if school == nil {
		return nil, &SchoolNotFoundError{SchoolID: schoolID}
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
	return req
}

func (s *operatorProvisioningService) seedDefaultActivityCategories(ctx context.Context, tenantID int64) error {
	if s.categoryRepo == nil || tenantID <= 0 {
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
	}

	categoryCtx := tenant.WithTenantID(ctx, tenantID)
	for i := range defaults {
		category := defaults[i]
		if err := s.categoryRepo.Create(categoryCtx, &category); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return err
		}
	}

	return nil
}

// webManualDeviceID is the device_id for the virtual web check-in device.
// Each tenant gets its own instance so RLS keeps it visible only to that school.
const webManualDeviceID = "WEB-MANUAL-001"

func (s *operatorProvisioningService) createWebManualDevice(ctx context.Context, tenantID int64) error {
	if s.deviceRepo == nil || tenantID <= 0 {
		return nil
	}

	deviceName := "Web-Portal (Manuell)"
	device := &iotModels.Device{
		DeviceID:   webManualDeviceID,
		DeviceType: "virtual",
		Name:       &deviceName,
		Status:     iotModels.DeviceStatusActive,
	}
	device.SetTenantID(tenantID)

	deviceCtx := tenant.WithTenantID(ctx, tenantID)
	if err := s.deviceRepo.Create(deviceCtx, device); err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("create web manual device for tenant %d: %w", tenantID, err)
	}

	s.getLogger().Info("created web manual device for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.String("device_id", webManualDeviceID),
	)
	return nil
}

func (s *operatorProvisioningService) withAdminTx(ctx context.Context, fn func(context.Context) error) error {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return fn(ctx)
	}
	if s.txHandler == nil || s.txHandler.DB == nil {
		return fn(ctx)
	}
	return tenant.WithAdminTx(ctx, s.txHandler.DB, func(adminCtx context.Context, _ bun.Tx) error {
		return fn(adminCtx)
	})
}

func (s *operatorProvisioningService) resolveSystemRoleByName(ctx context.Context, name string) (*authModels.Role, error) {
	roles, err := s.roleRepo.List(ctx, map[string]interface{}{
		"name":      strings.TrimSpace(strings.ToLower(name)),
		"is_system": true,
	})
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		if !strings.EqualFold(role.Name, name) {
			continue
		}
		if role.TenantID == nil && role.IsSystem {
			return role, nil
		}
	}
	return nil, nil
}

func (s *operatorProvisioningService) logAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) {
	entry := &platform.OperatorAuditLog{
		OperatorID:   operatorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RequestIP:    clientIP,
	}
	if len(changes) > 0 {
		if payload, err := json.Marshal(changes); err == nil {
			entry.Changes = payload
		}
	}
	if err := s.auditLogRepo.Create(ctx, entry); err != nil {
		s.getLogger().Error(
			"failed to create operator audit log",
			slog.Any("error", err),
			slog.String("resource_type", resourceType),
		)
	}
}

func isSchoolLookupNotFound(err error) bool {
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

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation() && pgErr.Field('C') == "23505"
	}
	return false
}

func (s *operatorProvisioningService) SoftDeleteSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, err := s.schoolRepo.FindByID(adminCtx, schoolID)
		if err != nil || school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if school.IsDeleted() {
			return &SchoolAlreadyDeletedError{SchoolID: schoolID}
		}

		if err := s.schoolRepo.SoftDelete(adminCtx, schoolID); err != nil {
			return err
		}

		s.logAction(adminCtx, operatorID, platform.ActionSoftDelete, platform.ResourceSchool, &schoolID, clientIP, map[string]any{
			"name":      school.Name,
			"slug":      school.Slug,
			"subdomain": school.Subdomain,
		})
		return nil
	})
}

func (s *operatorProvisioningService) RestoreSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error {
	return s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, err := s.schoolRepo.FindByID(adminCtx, schoolID)
		if err != nil || school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if !school.IsDeleted() {
			return &SchoolNotDeletedError{SchoolID: schoolID}
		}

		if err := s.schoolRepo.Restore(adminCtx, schoolID); err != nil {
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

func (s *operatorProvisioningService) PurgeSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) ([]byte, error) {
	// 1. Validate and write audit log in its own transaction.
	//    This commits independently so the audit trail survives even if the purge fails.
	err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
		school, findErr := s.schoolRepo.FindByID(adminCtx, schoolID)
		if findErr != nil || school == nil {
			return &SchoolNotFoundError{SchoolID: schoolID}
		}
		if !school.IsDeleted() {
			return &SchoolNotDeletedError{SchoolID: schoolID}
		}

		s.logAction(adminCtx, operatorID, platform.ActionPurge, platform.ResourceSchool, &schoolID, clientIP, map[string]any{
			"name":       school.Name,
			"slug":       school.Slug,
			"subdomain":  school.Subdomain,
			"deleted_at": school.DeletedAt,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 2. Purge in a separate transaction — the stored procedure deletes the school row.
	var purgeResult []byte
	err = s.withAdminTx(ctx, func(adminCtx context.Context) error {
		result, purgeErr := s.schoolRepo.PurgeTenant(adminCtx, schoolID)
		if purgeErr != nil {
			return &TenantPurgeError{SchoolID: schoolID, Err: purgeErr}
		}
		purgeResult = result
		return nil
	})
	return purgeResult, err
}

func (s *operatorProvisioningService) PurgeDeletedSchools(ctx context.Context, olderThan time.Duration) (int, error) {
	schools, err := s.schoolRepo.FindDeletedOlderThan(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("find deleted schools: %w", err)
	}

	purged := 0
	failed := 0
	for _, school := range schools {
		schoolID := school.ID
		schoolName := school.Name
		err := s.withAdminTx(ctx, func(adminCtx context.Context) error {
			_, purgeErr := s.schoolRepo.PurgeTenant(adminCtx, schoolID)
			return purgeErr
		})
		if err != nil {
			s.getLogger().Error("failed to purge tenant",
				slog.Int64("school_id", schoolID),
				slog.Any("error", err),
			)
			s.getLogger().Debug("failed purge tenant details",
				slog.Int64("school_id", schoolID),
				slog.String("school_name", schoolName),
			)
			failed++
			continue
		}
		s.getLogger().Info("purged tenant",
			slog.Int64("school_id", schoolID),
		)
		s.getLogger().Debug("purged tenant details",
			slog.Int64("school_id", schoolID),
			slog.String("school_name", schoolName),
		)
		purged++
	}
	if failed > 0 {
		return purged, fmt.Errorf("purged %d schools, %d failed (see logs for details)", purged, failed)
	}
	return purged, nil
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
