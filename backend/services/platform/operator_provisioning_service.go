package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

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
	ListSchoolAccounts(ctx context.Context, schoolID int64) ([]authModels.TenantAccountInfo, error)
	ListOrganizationAccounts(ctx context.Context, organizationID int64) ([]authModels.OrgAccountInfo, error)
}

type operatorProvisioningService struct {
	organizationRepo  platform.OrganizationRepository
	schoolRepo        platform.SchoolRepository
	categoryRepo      activityModels.CategoryRepository
	deviceRepo        iotModels.DeviceRepository
	roleRepo          authModels.RoleRepository
	accountTenantRepo authModels.AccountTenantRepository
	invitationService authSvc.InvitationService
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
	InvitationService authSvc.InvitationService
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
		invitationService: cfg.InvitationService,
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
