package postgres

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func (s *Store) CloseGroupVisits(ctx context.Context, ids []int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if len(ids) == 0 {
		return ports.Stats{}, nil
	}
	started := time.Now()
	// The app's check-in clock may be ahead of the DB clock. Preserve the
	// existing zero-duration clamp so one skewed interval cannot abort a batch.
	result, err := db.NewUpdate().Table("active.visits").
		Set("exit_time = GREATEST(now(), entry_time)").Where("tenant_id = ?", tenantID).
		Where("active_group_id IN (?)", bun.List(ids)).Where("exit_time IS NULL").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) TransferOpenVisits(ctx context.Context, from, to int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	visits, err := writeVisits(ctx, db, tenantID, studentpresence.VisitWriteScope{}, []int64{from}, true)
	if err != nil {
		return ports.Stats{Queries: 1}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.visits").
		Set("active_group_id = ?", to).Set("updated_at = ?", time.Now()).
		Where("tenant_id = ?", tenantID).Where("active_group_id = ?", from).
		Where("exit_time IS NULL").Where("id IN (?)", bun.List(visitWriteIDs(visits))).Exec(ctx)
	stats := ports.Stats{Queries: 2, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) TransferRecentDeviceVisits(ctx context.Context, to, deviceID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var groups []int64
	err = db.NewRaw("SELECT id FROM active.groups WHERE tenant_id = ? AND device_id = ? AND end_time IS NOT NULL AND end_time > NOW() - INTERVAL '1 hour' ORDER BY id", tenantID, deviceID).Scan(ctx, &groups)
	if err != nil {
		return ports.Stats{Queries: 1}, err
	}
	visits, err := writeVisits(ctx, db, tenantID, studentpresence.VisitWriteScope{}, groups, true)
	if err != nil {
		return ports.Stats{Queries: 2}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.visits").Set("active_group_id = ?", to).
		Where("tenant_id = ?", tenantID).
		Where(`active_group_id IN (
			SELECT id FROM active.groups WHERE tenant_id = ? AND device_id = ?
			AND end_time IS NOT NULL AND end_time > NOW() - INTERVAL '1 hour'
		) AND exit_time IS NULL`, tenantID, deviceID).Where("id IN (?)", bun.List(visitWriteIDs(visits))).Exec(ctx)
	stats := ports.Stats{Queries: 3, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, err
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

// deleteExpiredVisitsSQL deletes the completed visits of every listed
// student past its own cutoff in one statement and counts the rows it
// actually deleted per student.
const deleteExpiredVisitsSQL = `WITH expiry AS (
	SELECT * FROM unnest(?::bigint[], ?::timestamptz[], ?::boolean[]) AS e(student_id, cutoff, by_exit)
), deleted AS (
	DELETE FROM active.visits AS v USING expiry AS e
	WHERE v.tenant_id = ? AND v.student_id = e.student_id AND v.exit_time IS NOT NULL
	AND CASE WHEN e.by_exit THEN v.exit_time < e.cutoff ELSE v.created_at < e.cutoff END
	RETURNING v.student_id
)
SELECT student_id, COUNT(*) AS deleted FROM deleted GROUP BY student_id`

func (s *Store) DeleteExpiredVisits(ctx context.Context, expiries []ports.VisitExpiry) (map[int64]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	studentIDs := make([]int64, len(expiries))
	cutoffs := make([]time.Time, len(expiries))
	byExit := make([]bool, len(expiries))
	for i, expiry := range expiries {
		studentIDs[i], cutoffs[i], byExit[i] = expiry.StudentID, expiry.Cutoff, expiry.ByExit
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Deleted   int64 `bun:"deleted"`
	}
	started := time.Now()
	err = db.NewRaw(deleteExpiredVisitsSQL, pgdialect.Array(studentIDs), pgdialect.Array(cutoffs), pgdialect.Array(byExit), tenantID).Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	deleted := make(map[int64]int64, len(rows))
	for _, row := range rows {
		deleted[row.StudentID] = row.Deleted
		stats.Rows += row.Deleted
	}
	return deleted, stats, nil
}
