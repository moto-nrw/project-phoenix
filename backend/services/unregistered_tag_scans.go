package services

import (
	"context"
	"errors"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

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
		panic("services: device fleet is required for unregistered tag scans")
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
