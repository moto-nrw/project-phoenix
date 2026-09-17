package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// ProvisioningDependencies are the owners operator provisioning (#3253)
// works through. Every field is required.
type ProvisioningDependencies struct {
	// Organizations is this module's public owner facade; provisioning
	// creates and changes organisations and schools only through it, so the
	// owner's normalisation and validation apply.
	Organizations organizationtenancy.Capability
	Transaction   ports.Transaction
	Dashboard     ports.OperatorDashboard
	Identity      ports.ProvisioningIdentity
	Devices       ports.ProvisioningDevices
	People        ports.ProvisioningPeople
	Presence      ports.ProvisioningPresence
	Categories    ports.ProvisioningCategories
	Settings      ports.ProvisioningSettings
	Audit         ports.OperatorAudit
	Secrets       ports.ProvisioningSecrets
	Logger        *slog.Logger
}

// Provisioning implements the operator provisioning capability. Every
// operation runs in one administrative transaction; operations on one school
// scope their owner calls to that school through the ports.
type Provisioning struct {
	organizations organizationtenancy.Capability
	tx            ports.Transaction
	dashboard     ports.OperatorDashboard
	identity      ports.ProvisioningIdentity
	devices       ports.ProvisioningDevices
	people        ports.ProvisioningPeople
	presence      ports.ProvisioningPresence
	categories    ports.ProvisioningCategories
	settings      ports.ProvisioningSettings
	audit         ports.OperatorAudit
	secrets       ports.ProvisioningSecrets
	logger        *slog.Logger
}

var _ organizationtenancy.Provisioning = (*Provisioning)(nil)

// NewProvisioning builds the provisioning application.
func NewProvisioning(deps ProvisioningDependencies) (*Provisioning, error) {
	if deps.Organizations == nil || deps.Transaction == nil || deps.Dashboard == nil || deps.Identity == nil ||
		deps.Devices == nil || deps.People == nil || deps.Presence == nil || deps.Categories == nil ||
		deps.Settings == nil || deps.Audit == nil || deps.Secrets == nil {
		return nil, errors.New("organization tenancy provisioning: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Provisioning{
		organizations: deps.Organizations, tx: deps.Transaction, dashboard: deps.Dashboard,
		identity: deps.Identity, devices: deps.Devices, people: deps.People, presence: deps.Presence,
		categories: deps.Categories, settings: deps.Settings, audit: deps.Audit, secrets: deps.Secrets,
		logger: logger,
	}, nil
}

func (p *Provisioning) inAdmin(ctx context.Context, fn func(context.Context) error) error {
	return p.tx.RunAdmin(ctx, fn)
}

// adminValue runs fn in the administrative transaction and returns its value.
func adminValue[T any](ctx context.Context, p *Provisioning, fn func(context.Context) (T, error)) (T, error) {
	var result T
	err := p.inAdmin(ctx, func(adminCtx context.Context) error {
		value, err := fn(adminCtx)
		if err != nil {
			return err
		}
		result = value
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// CreateOrganization creates an organisation and records the action.
func (p *Provisioning) CreateOrganization(ctx context.Context, input *organizationtenancy.CreateOrganization, operatorID int64, clientIP net.IP) (*organizationtenancy.Organization, error) {
	if input == nil {
		return nil, &organizationtenancy.InvalidProvisioningDataError{Err: errors.New("organization is required")}
	}
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.Organization, error) {
		created, err := p.organizations.CreateOrganization(adminCtx, *input)
		if err != nil {
			return nil, mapOrganizationError(err, 0)
		}
		return &created, p.recordAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceOrganization, &created.ID, clientIP, map[string]any{
			"name": created.Name,
			"slug": created.Slug,
		})
	})
}

// ListOrganizations lists every organisation.
func (p *Provisioning) ListOrganizations(ctx context.Context) ([]organizationtenancy.Organization, error) {
	return p.organizations.ListOrganizations(ctx)
}

// UpdateOrganization changes an organisation and records the changed fields.
func (p *Provisioning) UpdateOrganization(ctx context.Context, id int64, changes organizationtenancy.OrganizationChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.Organization, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.Organization, error) {
		existing, err := p.organizations.FindOrganizationForMutation(adminCtx, id)
		if err != nil {
			return nil, mapOrganizationError(err, id)
		}
		if existing.IsDeleted() {
			return nil, &organizationtenancy.OrganizationAlreadyDeletedError{OrganizationID: id}
		}
		updated, err := p.organizations.UpdateOrganization(adminCtx, organizationtenancy.UpdateOrganization{
			ID: id, Name: changes.Name, Slug: changes.Slug, Active: changes.Active,
		})
		if err != nil {
			return nil, mapOrganizationError(err, id)
		}
		diff := map[string]any{}
		if updated.Slug != existing.Slug {
			diff["slug"] = map[string]string{"old": existing.Slug, "new": updated.Slug}
		}
		if updated.Name != existing.Name {
			diff["name"] = map[string]string{"old": existing.Name, "new": updated.Name}
		}
		if updated.Active != existing.Active {
			diff["active"] = map[string]bool{"old": existing.Active, "new": updated.Active}
		}
		return &updated, p.recordAction(adminCtx, operatorID, domain.AuditActionUpdate, domain.AuditResourceOrganization, &id, clientIP, diff)
	})
}

// SoftDeleteOrganization deletes an organisation without schools.
func (p *Provisioning) SoftDeleteOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error {
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		organization, err := p.organizations.SoftDeleteOrganization(adminCtx, organizationID)
		if err != nil {
			return mapOrganizationError(err, organizationID)
		}
		return p.recordAction(adminCtx, operatorID, domain.AuditActionSoftDelete, domain.AuditResourceOrganization, &organizationID, clientIP, map[string]any{
			"name": organization.Name,
			"slug": organization.Slug,
		})
	})
}

// RestoreOrganization returns a soft-deleted organisation.
func (p *Provisioning) RestoreOrganization(ctx context.Context, organizationID, operatorID int64, clientIP net.IP) error {
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		organization, err := p.organizations.RestoreOrganization(adminCtx, organizationID)
		if err != nil {
			return mapOrganizationError(err, organizationID)
		}
		return p.recordAction(adminCtx, operatorID, domain.AuditActionRestore, domain.AuditResourceOrganization, &organizationID, clientIP, map[string]any{
			"name": organization.Name,
			"slug": organization.Slug,
		})
	})
}

// logAction records the action and only logs a failure: the provisioning
// write stands even when its audit entry cannot be written.
func (p *Provisioning) logAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) {
	if err := p.recordAction(ctx, operatorID, action, resourceType, resourceID, clientIP, changes); err != nil {
		p.logger.Error("failed to create operator audit log",
			slog.Any("error", err),
			slog.String("resource_type", resourceType),
		)
	}
}

func (p *Provisioning) recordAction(ctx context.Context, operatorID int64, action, resourceType string, resourceID *int64, clientIP net.IP, changes map[string]any) error {
	entry := domain.OperatorAuditEntry{
		OperatorID: operatorID, Action: action, ResourceType: resourceType,
		ResourceID: resourceID, ClientIP: clientIP,
	}
	if len(changes) > 0 {
		payload, err := json.Marshal(changes)
		if err != nil {
			return fmt.Errorf("encode operator audit log changes: %w", err)
		}
		entry.Changes = payload
	}
	if err := p.audit.RecordOperatorAction(ctx, entry); err != nil {
		return fmt.Errorf("create operator audit log: %w", err)
	}
	return nil
}

func mapOrganizationError(err error, organizationID int64) error {
	switch {
	case errors.Is(err, organizationtenancy.ErrOrganizationNotFound):
		return &organizationtenancy.OrganizationNotFoundError{OrganizationID: organizationID}
	case errors.Is(err, organizationtenancy.ErrOrganizationSlugConflict):
		return &organizationtenancy.ProvisioningConflictError{Err: errors.New("organization slug already exists")}
	case errors.Is(err, organizationtenancy.ErrOrganizationAlreadyDeleted):
		return &organizationtenancy.OrganizationAlreadyDeletedError{OrganizationID: organizationID}
	case errors.Is(err, organizationtenancy.ErrOrganizationNotDeleted):
		return &organizationtenancy.OrganizationNotDeletedError{OrganizationID: organizationID}
	case errors.Is(err, organizationtenancy.ErrOrganizationHasSchools):
		return err
	case errors.Is(err, organizationtenancy.ErrInvalidOrganization):
		return &organizationtenancy.InvalidProvisioningDataError{Err: err}
	default:
		return err
	}
}

// mapSchoolError translates the owner's school errors; an error it does not
// know is returned unchanged.
func mapSchoolError(err error, schoolID, organizationID int64) error {
	if mapped, ok := translateSchoolError(err, schoolID, organizationID); ok {
		return mapped
	}
	return err
}

func translateSchoolError(err error, schoolID, organizationID int64) (error, bool) {
	switch {
	case errors.Is(err, organizationtenancy.ErrSchoolDomainConflict):
		return &organizationtenancy.ProvisioningConflictError{Err: errors.New("school subdomain already exists")}, true
	case errors.Is(err, organizationtenancy.ErrSchoolSlugConflict):
		return &organizationtenancy.ProvisioningConflictError{Err: errors.New("school slug already exists in this organization")}, true
	case errors.Is(err, organizationtenancy.ErrSchoolNotFound):
		return &organizationtenancy.SchoolNotFoundError{SchoolID: schoolID}, true
	case errors.Is(err, organizationtenancy.ErrSchoolAlreadyDeleted):
		return &organizationtenancy.SchoolAlreadyDeletedError{SchoolID: schoolID}, true
	case errors.Is(err, organizationtenancy.ErrSchoolNotDeleted):
		return &organizationtenancy.SchoolNotDeletedError{SchoolID: schoolID}, true
	case errors.Is(err, organizationtenancy.ErrOrganizationDeleted):
		return &organizationtenancy.OrganizationDeletedError{OrganizationID: organizationID}, true
	case errors.Is(err, organizationtenancy.ErrOrganizationNotFound):
		return &organizationtenancy.OrganizationNotFoundError{OrganizationID: organizationID}, true
	case errors.Is(err, organizationtenancy.ErrInvalidSchool):
		return &organizationtenancy.InvalidProvisioningDataError{Err: err}, true
	default:
		return nil, false
	}
}
