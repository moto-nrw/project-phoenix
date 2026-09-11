package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// RecordUnregisteredTagScan appends one scan for the caller's tenant. The
// scan is bookkeeping for the operator review; it never blocks the kiosk.
func (s *Service) RecordUnregisteredTagScan(ctx context.Context, input domain.RecordUnregisteredTagScan) (domain.UnregisteredTagScan, error) {
	var recorded domain.UnregisteredTagScan
	err := s.runWrite(ctx, "record_unregistered_tag_scan", func(ctx context.Context, stats *domain.OperationStats) error {
		input.Normalize(s.deps.Now())
		if err := input.Validate(); err != nil {
			return err
		}
		stored, queryStats, err := s.deps.Scans.Insert(ctx, input)
		stats.Add(queryStats)
		recorded = stored
		return err
	})
	return recorded, err
}

// FindUnregisteredTagScan reads one scan with its device identity.
func (s *Service) FindUnregisteredTagScan(ctx context.Context, id int64) (domain.UnregisteredTagScan, error) {
	var found domain.UnregisteredTagScan
	err := s.run(ctx, "find_unregistered_tag_scan", func(ctx context.Context, stats *domain.OperationStats) error {
		scan, err := s.findUnregisteredTagScan(ctx, id, stats)
		found = scan
		return err
	})
	return found, err
}

// ListUnregisteredTagScans reads the scans matching filter, newest first,
// with their device identities.
func (s *Service) ListUnregisteredTagScans(ctx context.Context, filter domain.UnregisteredTagScanFilter) ([]domain.UnregisteredTagScan, error) {
	var scans []domain.UnregisteredTagScan
	err := s.run(ctx, "list_unregistered_tag_scans", func(ctx context.Context, stats *domain.OperationStats) error {
		stored, queryStats, err := s.deps.Scans.List(ctx, filter)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if err := s.attachScanDevices(ctx, stored, stats); err != nil {
			return err
		}
		scans = stored
		return nil
	})
	return scans, err
}

// ResolveUnregisteredTagScan marks one open scan as handled. A scan that is
// gone reports ErrUnregisteredTagScanNotFound; one already handled reports
// ErrUnregisteredTagScanResolved, so a retry after a lost response is loud
// but never double-stamps.
func (s *Service) ResolveUnregisteredTagScan(ctx context.Context, input domain.ResolveUnregisteredTagScan) (domain.UnregisteredTagScan, error) {
	var resolved domain.UnregisteredTagScan
	err := s.runWrite(ctx, "resolve_unregistered_tag_scan", func(ctx context.Context, stats *domain.OperationStats) error {
		input.Normalize(s.deps.Now())
		if err := input.Validate(); err != nil {
			return err
		}
		affected, queryStats, err := s.deps.Scans.Resolve(ctx, input)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if affected != 1 {
			_, found, findStats, findErr := s.deps.Scans.FindByID(ctx, input.ID)
			stats.Add(findStats)
			if findErr != nil {
				return findErr
			}
			if !found {
				return domain.ErrUnregisteredTagScanNotFound
			}
			return domain.ErrUnregisteredTagScanResolved
		}
		scan, err := s.findUnregisteredTagScan(ctx, input.ID, stats)
		resolved = scan
		return err
	})
	return resolved, err
}

// DeleteExpiredUnregisteredTagScans removes the caller tenant's scans older
// than cutoff.
func (s *Service) DeleteExpiredUnregisteredTagScans(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64
	err := s.runWrite(ctx, "delete_expired_unregistered_tag_scans", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var err error
		deleted, queryStats, err = s.deps.Scans.DeleteExpired(ctx, cutoff)
		stats.Add(queryStats)
		return err
	})
	return deleted, err
}

func (s *Service) findUnregisteredTagScan(ctx context.Context, id int64, stats *domain.OperationStats) (domain.UnregisteredTagScan, error) {
	scan, found, queryStats, err := s.deps.Scans.FindByID(ctx, id)
	stats.Add(queryStats)
	if err != nil {
		return domain.UnregisteredTagScan{}, err
	}
	if !found {
		return domain.UnregisteredTagScan{}, domain.ErrUnregisteredTagScanNotFound
	}
	scans := []domain.UnregisteredTagScan{scan}
	if err := s.attachScanDevices(ctx, scans, stats); err != nil {
		return domain.UnregisteredTagScan{}, err
	}
	return scans[0], nil
}

// attachScanDevices fills each scan's device identity from the owner's own
// device store. A device of another tenant leaves both fields unset, as the
// retired tenant-matched join did.
func (s *Service) attachScanDevices(ctx context.Context, scans []domain.UnregisteredTagScan, stats *domain.OperationStats) error {
	ids := make([]int64, 0, len(scans))
	seen := make(map[int64]struct{}, len(scans))
	for _, scan := range scans {
		if scan.DeviceID == nil || *scan.DeviceID <= 0 {
			continue
		}
		if _, ok := seen[*scan.DeviceID]; ok {
			continue
		}
		seen[*scan.DeviceID] = struct{}{}
		ids = append(ids, *scan.DeviceID)
	}
	if len(ids) == 0 {
		return nil
	}
	devices, queryStats, err := s.deps.Devices.ListByIDs(ctx, ids)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	byID := make(map[int64]domain.Device, len(devices))
	for _, device := range devices {
		byID[device.ID] = device
	}
	for i := range scans {
		if scans[i].DeviceID == nil {
			continue
		}
		device, ok := byID[*scans[i].DeviceID]
		if !ok || device.TenantID != scans[i].TenantID {
			continue
		}
		identifier := device.DeviceID
		scans[i].DeviceIdentifier = &identifier
		scans[i].DeviceName = device.Name
	}
	return nil
}
