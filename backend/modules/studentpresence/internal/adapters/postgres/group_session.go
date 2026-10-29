package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
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
	// Take the lock in a separate statement so a waiting compaction gets a
	// fresh READ COMMITTED snapshot after its predecessor commits. Otherwise
	// it can consume a deleted session's events while missing the predecessor's
	// replacement entry rows, losing the session's contribution to the room.
	stats.Queries++
	key := fmt.Sprintf("active-visit-revision-compaction:%d", tenantID)
	if _, err := db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key); err != nil {
		return fmt.Errorf("end group sessions: lock visit revision compaction: %w", err)
	}
	settled, err := lockSettledSessions(ctx, db, tenantID, stats)
	if err != nil {
		return err
	}
	stats.Queries++
	if _, err := db.NewRaw(compactVisitRevisionEvents, compactVisitRevisionArgs(tenantID, settled)...).Exec(ctx); err != nil {
		return fmt.Errorf("end group sessions: compact visit revision events: %w", err)
	}
	return nil
}

// lockSettledSessions locks the ended sessions the compaction folds into
// their rooms (#5273, ADR 0133): those still in a room's set whose session
// row and visits no other transaction holds. A writer that holds one of them
// keeps its session in the set until the next compaction. A writer that
// comes later waits for this transaction: a visit inserted into or moved
// into the session needs a key-share lock on its row, every other visit
// change a lock on the visit. Every event of a settled session therefore
// commits before the compaction's snapshot or after its commit, and a late
// one changes no room count while the session stays ended. SKIP LOCKED never
// waits, so the compaction adds no lock edge to a writer.
func lockSettledSessions(ctx context.Context, db bun.IDB, tenantID int64, stats *ports.Stats) ([]int64, error) {
	ended := []int64{}
	stats.Queries++
	err := db.NewRaw(`SELECT g.id FROM active.groups AS g
		WHERE g.tenant_id = ? AND g.end_time IS NOT NULL AND g.id IN (
			SELECT e.entered_group_id FROM active.visit_revision_events AS e
			WHERE e.tenant_id = ? AND e.entered_group_id IS NOT NULL)
		ORDER BY g.id FOR UPDATE OF g SKIP LOCKED`, tenantID, tenantID).Scan(ctx, &ended)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("end group sessions: lock settled sessions: %w", err)
	}
	if len(ended) == 0 {
		return ended, nil
	}
	locked := []int64{}
	stats.Queries++
	err = db.NewRaw(`SELECT v.active_group_id FROM active.visits AS v
		WHERE v.tenant_id = ? AND v.active_group_id = ANY(?) FOR SHARE SKIP LOCKED`,
		tenantID, pgdialect.Array(ended)).Scan(ctx, &locked)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("end group sessions: lock visits of settled sessions: %w", err)
	}
	var visits []struct {
		GroupID int64 `bun:"group_id"`
		Visits  int   `bun:"visits"`
	}
	stats.Queries++
	err = db.NewRaw(`SELECT v.active_group_id AS group_id, count(*) AS visits FROM active.visits AS v
		WHERE v.tenant_id = ? AND v.active_group_id = ANY(?) GROUP BY v.active_group_id`,
		tenantID, pgdialect.Array(ended)).Scan(ctx, &visits)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("end group sessions: count visits of settled sessions: %w", err)
	}
	// A session settles when this transaction locked every visit it has.
	unlocked := make(map[int64]int, len(visits))
	for _, row := range visits {
		unlocked[row.GroupID] = row.Visits
	}
	for _, groupID := range locked {
		unlocked[groupID]--
	}
	settled := ended[:0]
	for _, groupID := range ended {
		if unlocked[groupID] == 0 {
			settled = append(settled, groupID)
		}
	}
	return settled, nil
}

// compactVisitRevisionArgs binds the settled sessions to the first
// placeholder of compactVisitRevisionEvents and the school to every other.
func compactVisitRevisionArgs(tenantID int64, settled []int64) []any {
	args := make([]any, strings.Count(compactVisitRevisionEvents, "?"))
	args[0] = pgdialect.Array(settled)
	for i := 1; i < len(args); i++ {
		args[i] = tenantID
	}
	return args
}

// compactVisitRevisionEvents keeps the change events small (#5128, #5205)
// and a room's read independent of its history (#5273) without changing a
// session or room revision:
//
//   - The events of a deleted session (its session rows and its entry rows)
//     become one room row per room it entered, carrying the session's sum
//     plus that room's entry sum. A deleted session receives no event after
//     its deletion commits, so the fold is exact; its id never returns.
//   - The entry rows of a settled session (lockSettledSessions, the first
//     placeholder) fold the same way, but its session rows stay: the session
//     revision keeps them, and a reopening enters the session into its room
//     again. No event of a settled session commits between this statement's
//     snapshot and its commit, so the fold is exact as well.
//   - The session rows of each ended session, the entry rows of each
//     session and room, and the room rows of each room collapse into one row
//     with their sum.
//
// A concurrent writer's events are not in this statement's snapshot and stay
// as they are. Compactions serialize per tenant before taking this statement's
// snapshot, so replaced entry rows remain visible to the next compaction when
// it folds a session deleted in between.
const compactVisitRevisionEvents = `WITH settled AS (
		SELECT unnest(?::bigint[]) AS group_id
	), gone AS (
		SELECT k.group_id FROM (
			SELECT e.active_group_id AS group_id FROM active.visit_revision_events AS e
			WHERE e.tenant_id = ? AND e.active_group_id IS NOT NULL
			UNION
			SELECT e.entered_group_id FROM active.visit_revision_events AS e
			WHERE e.tenant_id = ? AND e.entered_group_id IS NOT NULL
		) AS k
		LEFT JOIN active.groups AS g ON g.tenant_id = ? AND g.id = k.group_id
		WHERE g.id IS NULL
	), folded AS (
		DELETE FROM active.visit_revision_events AS e
		WHERE e.tenant_id = ? AND (e.active_group_id IN (SELECT group_id FROM gone)
			OR e.entered_group_id IN (SELECT group_id FROM gone)
			OR e.entered_group_id IN (SELECT group_id FROM settled))
		RETURNING e.active_group_id, e.entered_group_id, e.room_id, e.changes
	), folded_sessions AS (
		SELECT s.group_id, sum(s.changes) AS changes FROM (
			SELECT f.active_group_id AS group_id, f.changes FROM folded AS f
			WHERE f.active_group_id IS NOT NULL
			UNION ALL
			SELECT e.active_group_id, e.changes FROM active.visit_revision_events AS e
			WHERE e.tenant_id = ? AND e.active_group_id IN (SELECT group_id FROM settled)
		) AS s GROUP BY s.group_id
	), folded_rooms AS (
		SELECT f.room_id, sum(f.changes + COALESCE(fs.changes, 0)) AS changes
		FROM (SELECT entered_group_id, room_id, sum(changes) AS changes FROM folded
			WHERE entered_group_id IS NOT NULL GROUP BY entered_group_id, room_id) AS f
		LEFT JOIN folded_sessions AS fs ON fs.group_id = f.entered_group_id
		GROUP BY f.room_id
	), candidates AS (
		SELECT e.id, e.active_group_id, e.entered_group_id, e.room_id,
			COALESCE(e.active_group_id, 0) AS group_key, COALESCE(e.entered_group_id, 0) AS entry_key,
			COALESCE(e.room_id, 0) AS room_key
		FROM active.visit_revision_events AS e
		LEFT JOIN active.groups AS g ON g.tenant_id = e.tenant_id AND g.id = e.active_group_id
		LEFT JOIN active.groups AS h ON h.tenant_id = e.tenant_id AND h.id = e.entered_group_id
		WHERE e.tenant_id = ? AND (
			(e.active_group_id IS NOT NULL AND g.end_time IS NOT NULL)
			OR (e.room_id IS NOT NULL AND e.entered_group_id IS NULL)
			OR h.id IS NOT NULL)
			AND (e.entered_group_id IS NULL OR e.entered_group_id NOT IN (SELECT group_id FROM settled))
	), targets AS (
		SELECT c.* FROM candidates AS c
		JOIN (SELECT group_key, entry_key, room_key FROM candidates GROUP BY group_key, entry_key, room_key
			HAVING count(*) > 1) AS k
			ON k.group_key = c.group_key AND k.entry_key = c.entry_key AND k.room_key = c.room_key
	), collapsed AS (
		DELETE FROM active.visit_revision_events AS e
		WHERE e.tenant_id = ? AND e.id IN (SELECT id FROM targets)
		RETURNING e.active_group_id, e.entered_group_id, e.room_id, e.changes
	)
	INSERT INTO active.visit_revision_events (tenant_id, active_group_id, entered_group_id, room_id, changes)
	SELECT ?, NULL, NULL, r.room_id, r.changes FROM folded_rooms AS r
	UNION ALL
	SELECT ?, c.active_group_id, c.entered_group_id, c.room_id, sum(c.changes) FROM collapsed AS c
	GROUP BY c.active_group_id, c.entered_group_id, c.room_id`

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
