package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// defaultActivityCategories are the categories every operator-provisioned
// school starts with. Essenszeiten need a fitting Pflichtkategorie when a
// Termin is created; Mensa was missing here until #2131, and migration
// 1.15.260 backfills the schools created before it. Keep the three values in
// sync with that migration.
var defaultActivityCategories = []domain.ActivityCategory{
	{Name: "Sport", Description: "Sportliche Aktivitäten für Kinder", Color: "#7ED321"},
	{Name: "Kunst & Basteln", Description: "Kreative Aktivitäten und Handwerken", Color: "#F5A623"},
	{Name: "Musik", Description: "Musikalische Aktivitäten und Gesang", Color: "#BD10E0"},
	{Name: "Spiele", Description: "Brett-, Karten- und Gruppenspiele", Color: "#50E3C2"},
	{Name: "Lesen", Description: "Leseförderung und Literatur", Color: "#B8E986"},
	{Name: "Hausaufgabenhilfe", Description: "Unterstützung bei den Hausaufgaben", Color: "#4A90E2"},
	{Name: "Natur & Forschen", Description: "Naturerkundung und einfache Experimente", Color: "#7ED321"},
	{Name: "Computer", Description: "Grundlagen im Umgang mit dem Computer", Color: "#9013FE"},
	{Name: "Gruppenraum", Description: "Aktivitäten im Gruppenraum", Color: "#FF6900"},
	{Name: "Mensa", Description: "Aktivitäten rund um das Mittagessen", Color: "#FF9500"},
}

const webManualDeviceName = "Web-Portal (Manuell)"

// CreateSchool creates a school with its default activity categories and the
// virtual device that attributes manual web check-ins.
func (p *Provisioning) CreateSchool(ctx context.Context, input *organizationtenancy.CreateSchool, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
	if input == nil {
		return nil, &organizationtenancy.InvalidProvisioningDataError{Err: errors.New("school is required")}
	}
	school := *input
	if err := organizationtenancy.NormalizeCreateSchool(&school); err != nil {
		return nil, &organizationtenancy.InvalidProvisioningDataError{Err: err}
	}
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.School, error) {
		if err := p.validateSchoolCreate(adminCtx, school); err != nil {
			return nil, err
		}
		created, err := p.organizations.CreateSchool(adminCtx, school)
		if err != nil {
			return nil, mapSchoolError(err, 0, school.OrganizationID)
		}
		if err := p.categories.SeedCategories(adminCtx, created.ID, defaultActivityCategories); err != nil {
			return nil, err
		}
		if err := p.createWebManualDevice(adminCtx, created.ID); err != nil {
			return nil, err
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionCreate, domain.AuditResourceSchool, &created.ID, clientIP, map[string]any{
			"name":           created.Name,
			"slug":           created.Slug,
			"subdomain":      created.Subdomain,
			"organizationID": created.OrganizationID,
		})
		return &created, nil
	})
}

func (p *Provisioning) validateSchoolCreate(ctx context.Context, school organizationtenancy.CreateSchool) error {
	organization, err := p.organizations.FindOrganizationForSchoolMutation(ctx, school.OrganizationID)
	if err != nil {
		return mapOrganizationError(err, school.OrganizationID)
	}
	if organization.IsDeleted() {
		return &organizationtenancy.OrganizationDeletedError{OrganizationID: school.OrganizationID}
	}
	if taken, err := p.schoolBySlug(ctx, school.OrganizationID, school.Slug); err != nil {
		return err
	} else if taken != nil {
		return &organizationtenancy.ProvisioningConflictError{Err: errors.New("school slug already exists in this organization")}
	}
	if taken, err := p.schoolBySubdomain(ctx, school.Subdomain); err != nil {
		return err
	} else if taken != nil {
		return &organizationtenancy.ProvisioningConflictError{Err: errors.New("school subdomain already exists")}
	}
	return nil
}

func (p *Provisioning) createWebManualDevice(ctx context.Context, tenantID int64) error {
	name := webManualDeviceName
	_, err := p.devices.CreateDevice(ctx, domain.NewDevice{
		TenantID: tenantID, DeviceID: domain.WebManualDeviceID, DeviceType: domain.DeviceTypeVirtual,
		Name: &name, Status: domain.DeviceStatusActive,
	})
	if errors.Is(err, domain.ErrDeviceIDTaken) || errors.Is(err, domain.ErrDeviceAPIKeyTaken) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create web manual device for tenant %d: %w", tenantID, err)
	}
	p.logger.Info("created web manual device for tenant",
		slog.Int64("tenant_id", tenantID),
		slog.String("device_id", domain.WebManualDeviceID),
	)
	return nil
}

// ListSchools lists every school with its organisation.
func (p *Provisioning) ListSchools(ctx context.Context) ([]*organizationtenancy.School, error) {
	schools, err := p.organizations.ListSchools(ctx)
	if err != nil {
		return nil, err
	}
	names, err := p.organizationsByID(ctx, schools)
	if err != nil {
		return nil, err
	}
	result := make([]*organizationtenancy.School, 0, len(schools))
	for i := range schools {
		school := schools[i]
		if organization, ok := names[school.OrganizationID]; ok {
			school.Organization = &organization
		}
		result = append(result, &school)
	}
	return result, nil
}

func (p *Provisioning) organizationsByID(ctx context.Context, schools []organizationtenancy.School) (map[int64]organizationtenancy.Organization, error) {
	ids := make([]int64, 0, len(schools))
	seen := make(map[int64]bool, len(schools))
	for _, school := range schools {
		if !seen[school.OrganizationID] {
			seen[school.OrganizationID] = true
			ids = append(ids, school.OrganizationID)
		}
	}
	result := make(map[int64]organizationtenancy.Organization, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	organizations, err := p.organizations.ListOrganizationsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, organization := range organizations {
		result[organization.ID] = organization
	}
	return result, nil
}

// UpdateSchool changes a school and records the changed fields.
func (p *Provisioning) UpdateSchool(ctx context.Context, id int64, changes organizationtenancy.SchoolChanges, operatorID int64, clientIP net.IP) (*organizationtenancy.School, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.School, error) {
		existing, err := p.findSchool(adminCtx, id)
		if err != nil {
			return nil, err
		}
		if existing.IsDeleted() {
			return nil, &organizationtenancy.SchoolAlreadyDeletedError{SchoolID: id}
		}
		diff, err := p.schoolChangeSet(adminCtx, existing, changes)
		if err != nil {
			return nil, err
		}
		updated, err := p.organizations.UpdateSchool(adminCtx, organizationtenancy.UpdateSchool{
			ID: id, OrganizationID: changes.OrganizationID, Name: changes.Name, Slug: changes.Slug,
			Subdomain: changes.Subdomain, Active: changes.Active, Hidden: changes.Hidden,
			Settings: existing.Settings, Address: changes.Address, City: changes.City, Zip: changes.Zip,
			Phone: changes.Phone, Email: changes.Email, DevicePinHash: existing.DevicePinHash,
		})
		if err != nil {
			if mapped, ok := translateSchoolError(err, id, changes.OrganizationID); ok {
				return nil, mapped
			}
			return nil, &organizationtenancy.InvalidProvisioningDataError{Err: err}
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionUpdate, domain.AuditResourceSchool, &id, clientIP, diff)
		return &updated, nil
	})
}

// schoolChangeSet checks the new organisation, slug and subdomain and
// describes every changed audited field.
func (p *Provisioning) schoolChangeSet(ctx context.Context, existing organizationtenancy.School, changes organizationtenancy.SchoolChanges) (map[string]any, error) {
	diff := map[string]any{}
	if changes.OrganizationID != existing.OrganizationID {
		organization, err := p.organizations.FindOrganizationForSchoolMutation(ctx, changes.OrganizationID)
		if err != nil {
			return nil, mapOrganizationError(err, changes.OrganizationID)
		}
		if organization.IsDeleted() {
			return nil, &organizationtenancy.OrganizationDeletedError{OrganizationID: changes.OrganizationID}
		}
		diff["organization_id"] = map[string]int64{"old": existing.OrganizationID, "new": changes.OrganizationID}
	}
	if changes.Slug != existing.Slug || changes.OrganizationID != existing.OrganizationID {
		taken, err := p.schoolBySlug(ctx, changes.OrganizationID, changes.Slug)
		if err != nil {
			return nil, err
		}
		if taken != nil && taken.ID != existing.ID {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("school slug already exists in this organization")}
		}
		if changes.Slug != existing.Slug {
			diff["slug"] = map[string]string{"old": existing.Slug, "new": changes.Slug}
		}
	}
	if changes.Subdomain != existing.Subdomain {
		taken, err := p.schoolBySubdomain(ctx, changes.Subdomain)
		if err != nil {
			return nil, err
		}
		if taken != nil && taken.ID != existing.ID {
			return nil, &organizationtenancy.ProvisioningConflictError{Err: errors.New("school subdomain already exists")}
		}
		diff["subdomain"] = map[string]string{"old": existing.Subdomain, "new": changes.Subdomain}
	}
	if changes.Name != existing.Name {
		diff["name"] = map[string]string{"old": existing.Name, "new": changes.Name}
	}
	if changes.Active != existing.Active {
		diff["active"] = map[string]bool{"old": existing.Active, "new": changes.Active}
	}
	if changes.Hidden != existing.Hidden {
		diff["hidden"] = map[string]bool{"old": existing.Hidden, "new": changes.Hidden}
	}
	return diff, nil
}

// SoftDeleteSchool marks a school as deleted. The school stays in the
// database but is excluded from login, tenant resolution and every
// tenant-scoped operation:
//   - new logins and device requests are refused immediately;
//   - refresh sessions are revoked in the same transaction, and a refresh
//     re-checks deleted_at, catching a session a concurrent refresh created;
//   - existing access tokens drain within their 15-minute lifetime;
//   - pending invitations are consumed so their links cannot be redeemed.
//
// Revocation failures roll the deletion back: a school is never deleted
// without its access being revoked.
func (p *Provisioning) SoftDeleteSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error {
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		school, err := p.findSchool(adminCtx, schoolID)
		if err != nil {
			return err
		}
		if school.IsDeleted() {
			return &organizationtenancy.SchoolAlreadyDeletedError{SchoolID: schoolID}
		}
		if _, err := p.organizations.SoftDeleteSchool(adminCtx, schoolID); err != nil {
			return mapSchoolError(err, schoolID, school.OrganizationID)
		}
		revokedTokens, err := p.identity.RevokeSchoolSessions(adminCtx, schoolID)
		if err != nil {
			return fmt.Errorf("revoke tokens for school %d: %w", schoolID, err)
		}
		invalidatedInvitations, err := p.identity.InvalidatePendingInvitations(adminCtx, schoolID)
		if err != nil {
			return fmt.Errorf("invalidate invitations for school %d: %w", schoolID, err)
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionSoftDelete, domain.AuditResourceSchool, &schoolID, clientIP, map[string]any{
			"name":                school.Name,
			"slug":                school.Slug,
			"subdomain":           school.Subdomain,
			"revoked_tokens":      revokedTokens,
			"invalidated_invites": invalidatedInvitations,
		})
		return nil
	})
}

// RestoreSchool returns a soft-deleted school to its pre-deletion state; an
// inactive school stays inactive.
func (p *Provisioning) RestoreSchool(ctx context.Context, schoolID, operatorID int64, clientIP net.IP) error {
	return p.inAdmin(ctx, func(adminCtx context.Context) error {
		school, err := p.findSchool(adminCtx, schoolID)
		if err != nil {
			return err
		}
		if !school.IsDeleted() {
			return &organizationtenancy.SchoolNotDeletedError{SchoolID: schoolID}
		}
		organization, err := p.organizations.FindOrganizationForSchoolMutation(adminCtx, school.OrganizationID)
		if err != nil {
			return mapOrganizationError(err, school.OrganizationID)
		}
		if organization.IsDeleted() {
			return &organizationtenancy.OrganizationDeletedError{OrganizationID: school.OrganizationID}
		}
		if _, err := p.organizations.RestoreSchool(adminCtx, schoolID); err != nil {
			return mapSchoolError(err, schoolID, school.OrganizationID)
		}
		p.logAction(adminCtx, operatorID, domain.AuditActionRestore, domain.AuditResourceSchool, &schoolID, clientIP, map[string]any{
			"name":      school.Name,
			"slug":      school.Slug,
			"subdomain": school.Subdomain,
		})
		return nil
	})
}

// findSchool reads one school of any state; a missing school is a
// SchoolNotFoundError.
func (p *Provisioning) findSchool(ctx context.Context, schoolID int64) (organizationtenancy.School, error) {
	school, err := p.organizations.FindSchool(ctx, schoolID)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) || errors.Is(err, organizationtenancy.ErrInvalidSchool) {
		return organizationtenancy.School{}, &organizationtenancy.SchoolNotFoundError{SchoolID: schoolID}
	}
	return school, err
}

// findLiveSchool reads a school that exists and is not deleted.
func (p *Provisioning) findLiveSchool(ctx context.Context, schoolID int64) (organizationtenancy.School, error) {
	school, err := p.findSchool(ctx, schoolID)
	if err != nil {
		return organizationtenancy.School{}, err
	}
	if school.IsDeleted() {
		return organizationtenancy.School{}, &organizationtenancy.SchoolAlreadyDeletedError{SchoolID: schoolID}
	}
	return school, nil
}

func (p *Provisioning) schoolBySlug(ctx context.Context, organizationID int64, slug string) (*organizationtenancy.School, error) {
	return optionalSchool(p.organizations.FindSchoolByOrganizationAndSlug(ctx, organizationID, slug))
}

func (p *Provisioning) schoolBySubdomain(ctx context.Context, subdomain string) (*organizationtenancy.School, error) {
	return optionalSchool(p.organizations.FindSchoolBySubdomain(ctx, subdomain))
}

// optionalSchool reports a missing school as nil. Input the owner rejects
// is invalid provisioning data.
func optionalSchool(school organizationtenancy.School, err error) (*organizationtenancy.School, error) {
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return nil, nil
	}
	if errors.Is(err, organizationtenancy.ErrInvalidSchool) {
		return nil, &organizationtenancy.InvalidProvisioningDataError{Err: err}
	}
	if err != nil {
		return nil, err
	}
	return &school, nil
}
