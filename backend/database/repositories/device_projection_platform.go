package repositories

import (
	"context"
	"fmt"
	"sort"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// deviceOperatorSummariesRepository attaches the operator dashboard's device
// counts and device listings from the Device Fleet owner. iot.devices belongs
// to that owner (#2676), so the platform queries no longer join it.
type deviceOperatorSummariesRepository struct {
	platformModels.OperatorSummariesRepository
	devices devicefleet.Query
}

// Stats adds the platform-wide device count. Devices of soft-deleted schools
// are excluded, exactly as the retired join's INNER JOIN did.
func (r deviceOperatorSummariesRepository) Stats(ctx context.Context) (*platformModels.ProvisioningStats, error) {
	stats, err := r.OperatorSummariesRepository.Stats(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := r.devices.CountDevicesByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("count devices for provisioning stats: %w", err)
	}
	schools, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for provisioning stats: %w", err)
	}
	total := 0
	for _, school := range schools {
		if school.DeletedAt != nil {
			continue
		}
		total += counts[school.ID]
	}
	stats.GeraeteCount = total
	return stats, nil
}

// OrganizationSummaries adds each organization's device count.
func (r deviceOperatorSummariesRepository) OrganizationSummaries(ctx context.Context) ([]*platformModels.OrganizationSummary, error) {
	rows, err := r.OperatorSummariesRepository.OrganizationSummaries(ctx)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	counts, err := r.devices.CountDevicesByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("count devices for organization summaries: %w", err)
	}
	schools, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for organization summaries: %w", err)
	}
	byOrganization := make(map[int64]int, len(rows))
	for _, school := range schools {
		if school.DeletedAt != nil {
			continue
		}
		byOrganization[school.OrganizationID] += counts[school.ID]
	}
	for _, row := range rows {
		row.GeraeteCount = byOrganization[row.ID]
	}
	return rows, nil
}

// SchoolSummaries adds each school's device count.
func (r deviceOperatorSummariesRepository) SchoolSummaries(ctx context.Context) ([]*platformModels.SchoolSummary, error) {
	rows, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, err
	}
	return rows, r.attachSchoolDeviceCounts(ctx, rows)
}

// SchoolSummariesByOrganization adds each school's device count.
func (r deviceOperatorSummariesRepository) SchoolSummariesByOrganization(ctx context.Context, organizationID int64) ([]*platformModels.SchoolSummary, error) {
	rows, err := r.OperatorSummariesRepository.SchoolSummariesByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return rows, r.attachSchoolDeviceCounts(ctx, rows)
}

func (r deviceOperatorSummariesRepository) attachSchoolDeviceCounts(ctx context.Context, rows []*platformModels.SchoolSummary) error {
	if len(rows) == 0 {
		return nil
	}
	counts, err := r.devices.CountDevicesByTenant(ctx)
	if err != nil {
		return fmt.Errorf("count devices for school summaries: %w", err)
	}
	for _, row := range rows {
		row.GeraeteCount = counts[row.ID]
	}
	return nil
}

// ListDeviceRows assembles the operator device listing from the Device Fleet
// owner and the school/organization summaries.
//
// Devices belonging to a soft-deleted school or organization are filtered out
// unconditionally so global listings never surface entries for tenants that
// are in the Papierkorb.
func (r deviceOperatorSummariesRepository) ListDeviceRows(ctx context.Context, filter platformModels.OperatorDeviceFilter) ([]platformModels.OperatorDeviceRow, error) {
	schools, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for operator device listing: %w", err)
	}
	organizations, err := r.OperatorSummariesRepository.OrganizationSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("load organizations for operator device listing: %w", err)
	}
	deletedOrganizations := make(map[int64]struct{}, len(organizations))
	for _, organization := range organizations {
		if organization.DeletedAt != nil {
			deletedOrganizations[organization.ID] = struct{}{}
		}
	}

	visible := make(map[int64]*platformModels.SchoolSummary, len(schools))
	tenantIDs := make([]int64, 0, len(schools))
	for _, school := range schools {
		if school.DeletedAt != nil {
			continue
		}
		if _, deleted := deletedOrganizations[school.OrganizationID]; deleted {
			continue
		}
		if filter.SchoolID != nil && school.ID != *filter.SchoolID {
			continue
		}
		if filter.OrganizationID != nil && school.OrganizationID != *filter.OrganizationID {
			continue
		}
		visible[school.ID] = school
		tenantIDs = append(tenantIDs, school.ID)
	}
	if len(tenantIDs) == 0 {
		return []platformModels.OperatorDeviceRow{}, nil
	}

	devices, err := r.devices.ListDevicesByTenant(ctx, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("list devices for operator device listing: %w", err)
	}

	result := make([]platformModels.OperatorDeviceRow, 0, len(devices))
	for _, device := range devices {
		if filter.DeviceRowID != nil && device.ID != *filter.DeviceRowID {
			continue
		}
		school, found := visible[device.TenantID]
		if !found {
			continue
		}
		result = append(result, platformModels.OperatorDeviceRow{
			ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
			Name: device.Name, Status: string(device.Status), APIKey: device.APIKey,
			LastSeen: device.LastSeen, SchoolID: school.ID, SchoolName: school.Name,
			OrganizationID: school.OrganizationID, OrganizationName: school.OrganizationName,
			CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].OrganizationName != result[j].OrganizationName {
			return result[i].OrganizationName < result[j].OrganizationName
		}
		if result[i].SchoolName != result[j].SchoolName {
			return result[i].SchoolName < result[j].SchoolName
		}
		return result[i].DeviceID < result[j].DeviceID
	})
	return result, nil
}
