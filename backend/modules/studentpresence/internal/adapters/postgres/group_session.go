package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type liveGroupRow struct {
	ID             int64      `bun:"id"`
	TenantID       int64      `bun:"tenant_id"`
	CreatedAt      time.Time  `bun:"created_at"`
	UpdatedAt      time.Time  `bun:"updated_at"`
	StartTime      time.Time  `bun:"start_time"`
	LastActivity   time.Time  `bun:"last_activity"`
	EndTime        *time.Time `bun:"end_time"`
	TimeoutMinutes int        `bun:"timeout_minutes"`
	GroupID        *int64     `bun:"group_id"`
	DeviceID       *int64     `bun:"device_id"`
	RoomID         int64      `bun:"room_id"`
}

func (row liveGroupRow) record() ports.LiveGroup {
	return ports.LiveGroup{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
		TimeoutMinutes: row.TimeoutMinutes, ActivityGroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID,
	}
}

// LockGroup reads one group FOR UPDATE. The tenant filter keeps a foreign ID
// from matching, so the caller sees ErrGroupNotFound instead of another
// school's session.
func (s *Store) LockGroup(ctx context.Context, groupID int64) (ports.LiveGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.LiveGroup{}, ports.Stats{}, err
	}
	var row liveGroupRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, tenant_id, created_at, updated_at, start_time, last_activity, end_time,
		timeout_minutes, group_id, device_id, room_id
		FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, groupID).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.LiveGroup{}, stats, ports.ErrGroupNotFound
	}
	if err != nil {
		return ports.LiveGroup{}, stats, fmt.Errorf("lock group: %w", err)
	}
	stats.Rows = 1
	return row.record(), stats, nil
}

// EndGroupSession closes the group's open visits and supervisions, then the
// group itself. Every statement carries the tenant filter, and the final
// group update must change exactly one row: the lock taken first guarantees
// that nobody else ended the session in between. endDate is the calendar day
// of at in the school's calendar; supervisions end on a date, not an instant.
func (s *Store) EndGroupSession(ctx context.Context, groupID int64, at time.Time, endDate ports.Date) (result ports.EndedGroupSession, stats ports.Stats, err error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return result, stats, err
	}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()

	var endTime *time.Time
	stats.Queries++
	err = db.NewRaw(`SELECT end_time FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, groupID).Scan(ctx, &endTime)
	if errors.Is(err, sql.ErrNoRows) {
		return result, stats, ports.ErrGroupNotFound
	}
	if err != nil {
		return result, stats, fmt.Errorf("end group session: lock lifecycle: %w", err)
	}
	if endTime != nil {
		return result, stats, ports.ErrGroupEnded
	}

	visits := []*visitRow{}
	stats.Queries++
	err = db.NewUpdate().Model((*visitRow)(nil)).ModelTableExpr(`active.visits AS "visit"`).
		Set("exit_time = ?", at).Set("updated_at = ?", at).
		Where(`"visit".tenant_id = ?`, tenantID).Where(`"visit".active_group_id = ?`, groupID).
		Where(`"visit".exit_time IS NULL`).Returning("*").Scan(ctx, &visits)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, stats, fmt.Errorf("end group session: close visits: %w", err)
	}
	stats.Rows += int64(len(visits))

	supervisorIDs := []int64{}
	stats.Queries++
	err = db.NewRaw(`UPDATE active.group_supervisors SET end_date = ?, updated_at = ?
		WHERE tenant_id = ? AND group_id = ? AND end_date IS NULL RETURNING id`,
		endDate, at, tenantID, groupID).Scan(ctx, &supervisorIDs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, stats, fmt.Errorf("end group session: end supervisors: %w", err)
	}
	stats.Rows += int64(len(supervisorIDs))

	stats.Queries++
	ended, err := db.NewUpdate().Table("active.groups").Set("end_time = ?", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("id = ? AND end_time IS NULL", groupID).Exec(ctx)
	if err != nil {
		return result, stats, fmt.Errorf("end group session: end group: %w", err)
	}
	changed, err := ended.RowsAffected()
	if err != nil {
		return result, stats, fmt.Errorf("end group session: end group: %w", err)
	}
	if changed != 1 {
		return result, stats, fmt.Errorf("end group session: expected 1 group row, updated %d", changed)
	}
	stats.Rows += changed

	return ports.EndedGroupSession{
		GroupID: groupID, EndedAt: at,
		ClosedVisits: visitRecordsFromRows(visits), EndedSupervisorIDs: supervisorIDs,
	}, stats, nil
}
