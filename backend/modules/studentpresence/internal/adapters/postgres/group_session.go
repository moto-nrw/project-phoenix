package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
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

func (row liveGroupRow) record() studentpresence.LiveGroup {
	return studentpresence.LiveGroup{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
		TimeoutMinutes: row.TimeoutMinutes, ActivityGroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID,
	}
}

// LockGroup reads one group FOR UPDATE. The tenant filter keeps a foreign ID
// from matching, so the caller sees ErrGroupNotFound instead of another
// school's session.
func (s *Store) LockGroup(ctx context.Context, groupID int64) (studentpresence.LiveGroup, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return studentpresence.LiveGroup{}, ports.Stats{}, err
	}
	var row liveGroupRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, tenant_id, created_at, updated_at, start_time, last_activity, end_time,
		timeout_minutes, group_id, device_id, room_id
		FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, groupID).Scan(ctx, &row)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return studentpresence.LiveGroup{}, stats, studentpresence.ErrGroupNotFound
	}
	if err != nil {
		return studentpresence.LiveGroup{}, stats, fmt.Errorf("lock group: %w", err)
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

	stats.Queries++
	if err = lockOpenGroupLifecycle(ctx, db, tenantID, groupID); err != nil {
		return result, stats, err
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
		WHERE tenant_id = ? AND group_id = ? AND start_date <= ?
		AND (end_date IS NULL OR end_date > ?) RETURNING id`,
		endDate, at, tenantID, groupID, endDate, endDate).Scan(ctx, &supervisorIDs)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, stats, fmt.Errorf("end group session: end supervisors: %w", err)
	}
	stats.Rows += int64(len(supervisorIDs))

	stats.Queries++
	changed, err := endOpenGroupRow(ctx, db, tenantID, groupID, at)
	if err != nil {
		return result, stats, err
	}
	stats.Rows += changed

	return ports.EndedGroupSession{
		GroupID: groupID, EndedAt: at,
		ClosedVisits: visitRecordsFromRows(visits), EndedSupervisorIDs: supervisorIDs,
	}, stats, nil
}

// lockOpenGroupLifecycle locks the group row for the session end and requires
// it to exist and still be open.
func lockOpenGroupLifecycle(ctx context.Context, db bun.IDB, tenantID, groupID int64) error {
	var endTime *time.Time
	err := db.NewRaw(`SELECT end_time FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, groupID).Scan(ctx, &endTime)
	if errors.Is(err, sql.ErrNoRows) {
		return studentpresence.ErrGroupNotFound
	}
	if err != nil {
		return fmt.Errorf("end group session: lock lifecycle: %w", err)
	}
	if endTime != nil {
		return studentpresence.ErrGroupEnded
	}
	return nil
}

// endOpenGroupRow stamps the end time on the still open group row and
// requires exactly that one row to change.
func endOpenGroupRow(ctx context.Context, db bun.IDB, tenantID, groupID int64, at time.Time) (int64, error) {
	ended, err := db.NewUpdate().Table("active.groups").Set("end_time = ?", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("id = ? AND end_time IS NULL", groupID).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("end group session: end group: %w", err)
	}
	changed, err := ended.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("end group session: end group: %w", err)
	}
	if changed != 1 {
		return 0, fmt.Errorf("end group session: expected 1 group row, updated %d", changed)
	}
	return changed, nil
}

// EndGroupSessions is the nightly bulk close. It ends only groups that are
// still open, so a retry after a rolled-back run finds the same set again and
// a group closed in between is left alone. Visit exits keep the clamp to the
// entry time the nightly job always applied: one skewed interval must not
// abort the whole batch. It then compacts the visit revision events of the
// tenant's ended sessions (compactVisitRevisionEvents).
func (s *Store) EndGroupSessions(ctx context.Context, groupIDs []int64, at time.Time, endDate ports.Date) (result studentpresence.EndedGroupSessions, stats ports.Stats, err error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return result, stats, err
	}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()
	result, stats, err = endOpenGroupSessions(ctx, db, tenantID, groupIDs, at, endDate)
	// Successful empty batches still maintain revisions for sessions ended
	// manually or deleted since the last nightly run.
	if err == nil {
		err = compactVisitRevisions(ctx, db, tenantID, &stats)
	}
	return result, stats, err
}

func endOpenGroupSessions(ctx context.Context, db bun.IDB, tenantID int64, groupIDs []int64, at time.Time, endDate ports.Date) (result studentpresence.EndedGroupSessions, stats ports.Stats, err error) {
	if len(groupIDs) == 0 {
		return result, stats, nil
	}

	open := []int64{}
	stats.Queries++
	err = db.NewRaw(`SELECT id FROM active.groups WHERE tenant_id = ? AND id IN (?) AND end_time IS NULL ORDER BY id`,
		tenantID, bun.List(groupIDs)).Scan(ctx, &open)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return result, stats, fmt.Errorf("end group sessions: list open groups: %w", err)
	}
	if len(open) == 0 {
		return result, stats, nil
	}

	stats.Queries++
	visits, err := db.NewUpdate().Table("active.visits").
		Set("exit_time = GREATEST(?, entry_time)", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("active_group_id IN (?)", bun.List(open)).Where("exit_time IS NULL").Exec(ctx)
	if err != nil {
		return result, stats, fmt.Errorf("end group sessions: close visits: %w", err)
	}
	if result.VisitsClosed, err = visits.RowsAffected(); err != nil {
		return result, stats, fmt.Errorf("end group sessions: close visits: %w", err)
	}

	stats.Queries++
	groups, err := db.NewUpdate().Table("active.groups").Set("end_time = ?", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("id IN (?)", bun.List(open)).Where("end_time IS NULL").Exec(ctx)
	if err != nil {
		return result, stats, fmt.Errorf("end group sessions: end groups: %w", err)
	}
	if result.SessionsEnded, err = groups.RowsAffected(); err != nil {
		return result, stats, fmt.Errorf("end group sessions: end groups: %w", err)
	}

	stats.Queries++
	supervisors, err := db.NewUpdate().Table("active.group_supervisors").Set("end_date = ?", endDate).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("group_id IN (?)", bun.List(open)).
		Where("start_date <= ?", endDate).Where("(end_date IS NULL OR end_date > ?)", endDate).Exec(ctx)
	if err != nil {
		return result, stats, fmt.Errorf("end group sessions: end supervisors: %w", err)
	}
	if result.SupervisorsEnded, err = supervisors.RowsAffected(); err != nil {
		return result, stats, fmt.Errorf("end group sessions: end supervisors: %w", err)
	}
	result.EndedActiveGroupIDs = open
	stats.Rows = result.VisitsClosed + result.SessionsEnded + result.SupervisorsEnded
	return result, stats, nil
}

func compactVisitRevisions(ctx context.Context, db bun.IDB, tenantID int64, stats *ports.Stats) error {
	stats.Queries++
	if _, err := db.NewRaw(compactVisitRevisionEvents, tenantID, tenantID, tenantID).Exec(ctx); err != nil {
		return fmt.Errorf("end group sessions: compact visit revision events: %w", err)
	}
	return nil
}

// compactVisitRevisionEvents keeps the change events small (#5128, #5205)
// without changing a session or room revision. It replaces the events of
// each ended session and room by one row carrying their sum, and the room
// events (no session) of each room by one row. The events of a deleted
// session, whose id never returns, keep only their room: they become room
// events, so the room revision stays, and those without a room go. A
// concurrent writer's events are not in this statement's snapshot and stay
// as they are. The DELETE returns only rows this statement deleted, so a
// second compaction running at the same time cannot count a row twice.
const compactVisitRevisionEvents = `WITH candidates AS (
		SELECT e.id, e.room_id, g.id AS kept_group,
			COALESCE(g.id, 0) AS group_key, COALESCE(e.room_id, 0) AS room_key,
			e.active_group_id IS NOT NULL AND g.id IS NULL AS deleted_session
		FROM active.visit_revision_events AS e
		LEFT JOIN active.groups AS g ON g.tenant_id = e.tenant_id AND g.id = e.active_group_id
		WHERE e.tenant_id = ? AND (e.active_group_id IS NULL OR g.id IS NULL OR g.end_time IS NOT NULL)
	), targets AS (
		SELECT c.id, c.kept_group, c.room_id FROM candidates AS c
		JOIN (SELECT group_key, room_key FROM candidates GROUP BY group_key, room_key
			HAVING count(*) > 1 OR bool_or(deleted_session)) AS k
			ON k.group_key = c.group_key AND k.room_key = c.room_key
	), compacted AS (
		DELETE FROM active.visit_revision_events AS e USING targets AS t
		WHERE e.tenant_id = ? AND e.id = t.id
		RETURNING t.kept_group, t.room_id, e.changes
	)
	INSERT INTO active.visit_revision_events (tenant_id, active_group_id, room_id, changes)
	SELECT ?, c.kept_group, c.room_id, sum(c.changes) FROM compacted AS c
	WHERE c.kept_group IS NOT NULL OR c.room_id IS NOT NULL
	GROUP BY c.kept_group, c.room_id`

// EndGroup stamps the end time of one still-open group and touches nothing
// else. The lifecycle lock distinguishes a missing group from an ended one so
// the caller sees the same errors as for a full session end.
func (s *Store) EndGroup(ctx context.Context, groupID int64, at time.Time) (stats ports.Stats, err error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return stats, err
	}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()

	var endTime *time.Time
	stats.Queries++
	err = db.NewRaw(`SELECT end_time FROM active.groups WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, groupID).Scan(ctx, &endTime)
	if errors.Is(err, sql.ErrNoRows) {
		return stats, studentpresence.ErrGroupNotFound
	}
	if err != nil {
		return stats, fmt.Errorf("end group: lock lifecycle: %w", err)
	}
	if endTime != nil {
		return stats, studentpresence.ErrGroupEnded
	}

	stats.Queries++
	ended, err := db.NewUpdate().Table("active.groups").Set("end_time = ?", at).Set("updated_at = ?", at).
		Where("tenant_id = ?", tenantID).Where("id = ? AND end_time IS NULL", groupID).Exec(ctx)
	if err != nil {
		return stats, fmt.Errorf("end group: %w", err)
	}
	if stats.Rows, err = ended.RowsAffected(); err != nil {
		return stats, fmt.Errorf("end group: %w", err)
	}
	if stats.Rows != 1 {
		return stats, fmt.Errorf("end group: expected 1 group row, updated %d", stats.Rows)
	}
	return stats, nil
}
