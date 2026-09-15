package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/uptrace/bun"
)

// NewDeviceFleet composes the device owner behind the legacy composition seam
// for graphs that do not record observations and never serve the info-point
// dashboard. Production roots compose the module themselves (api/base.go) so
// runtime evidence and the dashboard collaborators are kept.
func NewDeviceFleet(db *bun.DB, onlineWindows ...func(context.Context) time.Duration) (devicefleet.Capability, error) {
	rooms, err := NewFacilities(db)
	if err != nil {
		return nil, err
	}
	var onlineWindow func(context.Context) time.Duration
	if len(onlineWindows) > 0 {
		onlineWindow = onlineWindows[0]
	}
	return devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB:           db,
		Rooms:        rooms,
		OnlineWindow: onlineWindow,
		Observe:      func(devicefleetCompose.Observation) {},
	})
}

// activeDeviceDirectory hands the device owner to the session repository that
// used to join iot.devices itself (#2676).
type activeDeviceDirectory struct{ devices devicefleet.Query }

func (d activeDeviceDirectory) ListDevicesByID(ctx context.Context, ids []int64) ([]activeRepo.DirectoryDevice, error) {
	devices, err := d.devices.ListDevicesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.DirectoryDevice, 0, len(devices))
	for _, device := range devices {
		result = append(result, activeRepo.DirectoryDevice{
			ID: device.ID, TenantID: device.TenantID, CreatedAt: device.CreatedAt,
			UpdatedAt: device.UpdatedAt, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
			Name: device.Name, Status: string(device.Status), LastSeen: device.LastSeen,
		})
	}
	return result, nil
}

// unregisteredTagScanRepository serves the retained audit repository shape
// from the Device Fleet owner, which owns audit.unregistered_tag_scans
// (#2678). The retained service keeps its port; every read and write below
// goes through the public capability, and the owner's stable errors are
// returned unchanged so callers can match them.
type unregisteredTagScanRepository struct{ scans devicefleet.Capability }

// NewUnregisteredTagScanRepository binds the retained repository contract to
// the owner capability.
func NewUnregisteredTagScanRepository(scans devicefleet.Capability) auditModels.UnregisteredTagScanRepository {
	if scans == nil {
		panic("repository factory: device fleet is required for unregistered tag scans")
	}
	return unregisteredTagScanRepository{scans: scans}
}

func (r unregisteredTagScanRepository) Create(ctx context.Context, scan *auditModels.UnregisteredTagScan) error {
	if scan == nil {
		return errors.New("unregistered tag scan is required")
	}
	recorded, err := r.scans.RecordUnregisteredTagScan(ctx, devicefleet.RecordUnregisteredTagScan{
		TagUID: scan.TagUID, DeviceID: scan.DeviceID, ScannedAt: scan.ScannedAt,
	})
	if err != nil {
		return err
	}
	*scan = toAuditUnregisteredTagScan(recorded)
	return nil
}

func (r unregisteredTagScanRepository) FindByID(ctx context.Context, id int64) (*auditModels.UnregisteredTagScan, error) {
	scan, err := r.scans.FindUnregisteredTagScan(ctx, id)
	if errors.Is(err, devicefleet.ErrUnregisteredTagScanNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := toAuditUnregisteredTagScan(scan)
	return &result, nil
}

func (r unregisteredTagScanRepository) ListForOperator(ctx context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	tenantIDs := filter.SchoolIDs
	if filter.SchoolID != nil {
		if tenantIDs == nil {
			tenantIDs = []int64{*filter.SchoolID}
		} else {
			// The retired query applied both predicates, so a school outside
			// the organization's schools matches nothing.
			narrowed := make([]int64, 0, 1)
			for _, id := range tenantIDs {
				if id == *filter.SchoolID {
					narrowed = append(narrowed, id)
				}
			}
			tenantIDs = narrowed
		}
	}
	scans, err := r.scans.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{
		TenantIDs: tenantIDs, UnresolvedOnly: filter.UnresolvedOnly,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*auditModels.UnregisteredTagScan, 0, len(scans))
	for _, scan := range scans {
		mapped := toAuditUnregisteredTagScan(scan)
		result = append(result, &mapped)
	}
	return result, nil
}

func (r unregisteredTagScanRepository) Resolve(ctx context.Context, id, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
	scan, err := r.scans.ResolveUnregisteredTagScan(ctx, devicefleet.ResolveUnregisteredTagScan{
		ID: id, OperatorID: operatorID, Note: note,
	})
	if err != nil {
		return nil, err
	}
	result := toAuditUnregisteredTagScan(scan)
	return &result, nil
}

func (r unregisteredTagScanRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error) {
	deleted, err := r.scans.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

func toAuditUnregisteredTagScan(scan devicefleet.UnregisteredTagScan) auditModels.UnregisteredTagScan {
	result := auditModels.UnregisteredTagScan{
		TagUID: scan.TagUID, DeviceID: scan.DeviceID, ScannedAt: scan.ScannedAt, ResolvedAt: scan.ResolvedAt,
		ResolvedByOperatorID: scan.ResolvedByOperatorID, ResolutionNote: scan.ResolutionNote,
		SchoolID: scan.TenantID, DeviceIdentifier: scan.DeviceIdentifier, DeviceName: scan.DeviceName,
	}
	result.ID = scan.ID
	result.CreatedAt = scan.CreatedAt
	result.UpdatedAt = scan.UpdatedAt
	result.SetTenantID(scan.TenantID)
	return result
}

func mustNewDeviceFleet(db *bun.DB) devicefleet.Capability {
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose device fleet: %v", err))
	}
	return fleet
}
