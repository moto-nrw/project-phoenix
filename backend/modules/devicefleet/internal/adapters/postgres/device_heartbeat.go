package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/devicefleet/internal/domain"
)

// RecordHeartbeat folds the old live-device lookup into the update. Select
// the primary key in a subquery so a concurrent rename or archival after the
// statement snapshot does not change which row the subsequent update targets.
// Only last_seen is assigned, retaining the existing updated_at trigger, and
// like TouchLastSeen it only moves forward.
func (s *DeviceStore) RecordHeartbeat(ctx context.Context, deviceID string, seenAt time.Time) (int64, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	candidate := db.NewSelect().TableExpr(`iot.devices AS candidate`).ColumnExpr("candidate.id").
		Where("candidate.device_id = ?", deviceID).Where("candidate.archived_at IS NULL")
	if !scope.AcrossSchools {
		candidate = candidate.Where("candidate.tenant_id = ?", scope.TenantID)
	}
	query := db.NewUpdate().Model((*deviceRow)(nil)).ModelTableExpr(deviceTableExpr).
		Set(`last_seen = GREATEST("device".last_seen, ?)`, seenAt).Where(`"device".id = (?)`, candidate)
	if !scope.AcrossSchools {
		query = query.Where(`"device".tenant_id = ?`, scope.TenantID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("devicefleet postgres: record heartbeat: %w", err)
	}
	affected, err := result.RowsAffected()
	stats.Rows = affected
	return affected, stats, err
}
