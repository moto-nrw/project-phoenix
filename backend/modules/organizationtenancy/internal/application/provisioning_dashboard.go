package application

import (
	"context"
	"fmt"
	"time"

	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/pwa"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// GetProvisioningStats returns the platform-wide counts of the operator
// overview. Devices of soft-deleted schools are not counted.
func (p *Provisioning) GetProvisioningStats(ctx context.Context) (*organizationtenancy.ProvisioningStats, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.ProvisioningStats, error) {
		counts, err := p.dashboard.Counts(adminCtx)
		if err != nil {
			return nil, err
		}
		devices, err := p.devices.CountDevicesByTenant(adminCtx)
		if err != nil {
			return nil, fmt.Errorf("count devices for provisioning stats: %w", err)
		}
		schools, err := p.dashboard.SchoolSummaries(adminCtx, nil)
		if err != nil {
			return nil, fmt.Errorf("load schools for provisioning stats: %w", err)
		}
		total := 0
		for _, school := range schools {
			if school.DeletedAt == nil {
				total += devices[school.ID]
			}
		}
		return &organizationtenancy.ProvisioningStats{
			TraegerCount: counts.Organizations, SchulenCount: counts.Schools,
			KontenCount: counts.Accounts, GeraeteCount: total,
		}, nil
	})
}

// ListOrganizationSummaries lists every organisation, deleted ones included,
// with the device and person counts of its non-deleted schools.
func (p *Provisioning) ListOrganizationSummaries(ctx context.Context) ([]*organizationtenancy.OrganizationSummary, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]*organizationtenancy.OrganizationSummary, error) {
		rows, err := p.dashboard.OrganizationSummaries(adminCtx)
		if err != nil {
			return nil, err
		}
		result := make([]*organizationtenancy.OrganizationSummary, 0, len(rows))
		if len(rows) == 0 {
			return result, nil
		}
		devices, persons, err := p.tenantCounts(adminCtx, "organization")
		if err != nil {
			return nil, err
		}
		schools, err := p.dashboard.SchoolSummaries(adminCtx, nil)
		if err != nil {
			return nil, fmt.Errorf("load schools for organization summaries: %w", err)
		}
		deviceTotals := make(map[int64]int, len(rows))
		personTotals := make(map[int64]int, len(rows))
		for _, school := range schools {
			if school.DeletedAt != nil {
				continue
			}
			deviceTotals[school.OrganizationID] += devices[school.ID]
			personTotals[school.OrganizationID] += persons[school.ID]
		}
		for _, row := range rows {
			result = append(result, &organizationtenancy.OrganizationSummary{
				ID: row.ID, Name: row.Name, Slug: row.Slug, Active: row.Active,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt, Settings: row.Settings,
				SchulenCount: row.SchoolCount, KontenCount: row.AccountCount,
				GeraeteCount: deviceTotals[row.ID], PersonenCount: personTotals[row.ID],
			})
		}
		return result, nil
	})
}

// ListSchoolSummaries lists every school with its counts.
func (p *Provisioning) ListSchoolSummaries(ctx context.Context) ([]*organizationtenancy.SchoolSummary, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]*organizationtenancy.SchoolSummary, error) {
		return p.schoolSummaries(adminCtx, nil)
	})
}

// ListOrganizationSchoolSummaries lists the schools of one organisation,
// deleted ones included so the drill-in can show its Papierkorb.
func (p *Provisioning) ListOrganizationSchoolSummaries(ctx context.Context, organizationID int64) ([]*organizationtenancy.SchoolSummary, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]*organizationtenancy.SchoolSummary, error) {
		if _, err := p.organizations.FindOrganization(adminCtx, organizationID); err != nil {
			return nil, mapOrganizationError(err, organizationID)
		}
		return p.schoolSummaries(adminCtx, &organizationID)
	})
}

func (p *Provisioning) schoolSummaries(ctx context.Context, organizationID *int64) ([]*organizationtenancy.SchoolSummary, error) {
	rows, err := p.dashboard.SchoolSummaries(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]*organizationtenancy.SchoolSummary, 0, len(rows))
	if len(rows) == 0 {
		return result, nil
	}
	devices, persons, err := p.tenantCounts(ctx, "school")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result = append(result, &organizationtenancy.SchoolSummary{
			ID: row.ID, OrganizationID: row.OrganizationID, OrganizationName: row.OrganizationName,
			Name: row.Name, Slug: row.Slug, Subdomain: row.Subdomain, Active: row.Active, Hidden: row.Hidden,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
			Address: row.Address, City: row.City, Zip: row.Zip, Phone: row.Phone, Email: row.Email,
			Settings: row.Settings, KontenCount: row.AccountCount,
			GeraeteCount: devices[row.ID], PersonenCount: persons[row.ID],
		})
	}
	return result, nil
}

// tenantCounts reads the per-school device and person counts.
func (p *Provisioning) tenantCounts(ctx context.Context, scope string) (map[int64]int, map[int64]int, error) {
	devices, err := p.devices.CountDevicesByTenant(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("count devices for %s summaries: %w", scope, err)
	}
	persons, err := p.people.CountPersonsByTenant(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("count persons for %s summaries: %w", scope, err)
	}
	return devices, persons, nil
}

// GetSchoolPWAUsage returns one school's PWA standalone usage over the
// Delivery usage window. A portal without usage stays zero.
func (p *Provisioning) GetSchoolPWAUsage(ctx context.Context, schoolID int64) (*organizationtenancy.SchoolPWAUsage, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) (*organizationtenancy.SchoolPWAUsage, error) {
		if _, err := p.findSchool(adminCtx, schoolID); err != nil {
			return nil, err
		}
		rows, err := p.dashboard.PWAUsage(adminCtx, schoolID, pwa.UsageWindow)
		if err != nil {
			return nil, err
		}
		usage := &organizationtenancy.SchoolPWAUsage{WindowDays: pwa.UsageWindowDays}
		for _, row := range rows {
			portal := organizationtenancy.PWAPortalUsage{StandaloneUsers: row.StandaloneUsers, EligibleUsers: row.EligibleUsers}
			switch row.Portal {
			case deliveryModels.PushPortalStaff:
				usage.Staff = portal
			case deliveryModels.PushPortalParent:
				usage.Parent = portal
			}
		}
		return usage, nil
	})
}

// ListPWAUsage returns every school's PWA usage buckets within window.
func (p *Provisioning) ListPWAUsage(ctx context.Context, window time.Duration) ([]organizationtenancy.SchoolPWAUsageRow, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.SchoolPWAUsageRow, error) {
		rows, err := p.dashboard.PWAUsage(adminCtx, 0, window)
		if err != nil {
			return nil, err
		}
		result := make([]organizationtenancy.SchoolPWAUsageRow, 0, len(rows))
		for _, row := range rows {
			result = append(result, organizationtenancy.SchoolPWAUsageRow(row))
		}
		return result, nil
	})
}
