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
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// OperatorProvisioningService handles operator-led tenant provisioning.
type OperatorProvisioningService interface {
	CreateOrganization(ctx context.Context, organization *platform.Organization, operatorID int64, clientIP net.IP) (*platform.Organization, error)
	ListOrganizations(ctx context.Context) ([]*platform.Organization, error)
	CreateSchool(ctx context.Context, school *platform.School, operatorID int64, clientIP net.IP) (*platform.School, error)
	ListSchools(ctx context.Context) ([]*platform.School, error)
	InviteSchoolAdmin(ctx context.Context, schoolID, operatorID int64, clientIP net.IP, req authSvc.InvitationRequest) (*authModels.InvitationToken, error)
}

type operatorProvisioningService struct {
	organizationRepo  platform.OrganizationRepository
	schoolRepo        platform.SchoolRepository
	categoryRepo      activityModels.CategoryRepository
	roleRepo          authModels.RoleRepository
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
	RoleRepo          authModels.RoleRepository
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
		roleRepo:          cfg.RoleRepo,
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
