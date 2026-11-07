package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// RecordAttendances inserts finished attendance rows in one statement and
// fills their identifiers, assigned in input order, into rows.
func (s *Store) RecordAttendances(ctx context.Context, rows []*ports.Attendance) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil || len(rows) == 0 {
		return ports.Stats{}, err
	}
	stored := make([]*attendanceRow, 0, len(rows))
	for _, row := range rows {
		if row.TenantID != 0 && row.TenantID != tenantID {
			return ports.Stats{}, errors.New("student presence: attendance belongs to another tenant")
		}
		row.TenantID = tenantID
		stored = append(stored, attendanceRowFromRecord(row))
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&stored).ModelTableExpr("active.attendance").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("record attendance batch: %w", err)
	}
	if stats.Rows, err = result.RowsAffected(); err != nil {
		return stats, fmt.Errorf("record attendance batch: %w", err)
	}
	for i, row := range stored {
		*rows[i] = *row.record()
	}
	return stats, nil
}

// pastGroupRow inserts a session with the column handling of RecordGroup:
// a zero timeout is stored as NULL.
type pastGroupRow struct {
	bun.BaseModel  `bun:"table:active.groups,alias:active_group"`
	ID             int64      `bun:"id,pk,autoincrement"`
	TenantID       int64      `bun:"tenant_id"`
	CreatedAt      time.Time  `bun:"created_at"`
	UpdatedAt      time.Time  `bun:"updated_at"`
	StartTime      time.Time  `bun:"start_time"`
	EndTime        *time.Time `bun:"end_time"`
	LastActivity   time.Time  `bun:"last_activity"`
	TimeoutMinutes int        `bun:"timeout_minutes,nullzero"`
	GroupID        *int64     `bun:"group_id"`
	DeviceID       *int64     `bun:"device_id"`
	RoomID         int64      `bun:"room_id"`
}

// RecordGroups inserts sessions in one statement and returns them, with
// their identifiers assigned in input order, in input order.
func (s *Store) RecordGroups(ctx context.Context, groups []studentpresence.LiveGroup) ([]studentpresence.LiveGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil || len(groups) == 0 {
		return nil, ports.Stats{}, err
	}
	started := time.Now()
	stored := make([]*pastGroupRow, 0, len(groups))
	for _, group := range groups {
		if group.CreatedAt.IsZero() {
			group.CreatedAt = started
		}
		if group.UpdatedAt.IsZero() {
			group.UpdatedAt = started
		}
		stored = append(stored, &pastGroupRow{TenantID: tenantID, CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
			StartTime: group.StartTime, EndTime: group.EndTime, LastActivity: group.LastActivity, TimeoutMinutes: group.TimeoutMinutes,
			GroupID: group.ActivityGroupID, DeviceID: group.DeviceID, RoomID: group.RoomID})
	}
	_, err = db.NewInsert().Model(&stored).ModelTableExpr("active.groups").Returning("*").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("record group batch: %w", err)
	}
	recorded := make([]studentpresence.LiveGroup, 0, len(stored))
	for _, row := range stored {
		recorded = append(recorded, liveGroupRow{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime, TimeoutMinutes: row.TimeoutMinutes,
			GroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID}.record())
	}
	stats.Rows = int64(len(stored))
	return recorded, stats, nil
}

// RecordVisits inserts closed visits in one statement and fills their
// identifiers, assigned in input order, into rows.
func (s *Store) RecordVisits(ctx context.Context, rows []*studentpresence.Visit) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil || len(rows) == 0 {
		return ports.Stats{}, err
	}
	stored := make([]*visitRow, 0, len(rows))
	for _, row := range rows {
		if row.TenantID != 0 && row.TenantID != tenantID {
			return ports.Stats{}, errors.New("student presence: visit belongs to another tenant")
		}
		row.TenantID = tenantID
		stored = append(stored, visitRowFromRecord(row))
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&stored).ModelTableExpr("active.visits").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("record visit batch: %w", err)
	}
	if stats.Rows, err = result.RowsAffected(); err != nil {
		return stats, fmt.Errorf("record visit batch: %w", err)
	}
	for i, row := range stored {
		*rows[i] = *row.record()
	}
	return stats, nil
}
