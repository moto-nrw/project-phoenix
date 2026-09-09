package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

func New(database Database) *Store {
	if database == nil {
		panic("student presence postgres: database is required")
	}
	return &Store{database: database}
}

func (s *Store) ListOpenPresence(ctx context.Context, ids []int64) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	result := []int64{}
	if len(ids) == 0 {
		return result, ports.Stats{}, nil
	}
	started := time.Now()
	err = db.NewRaw(`
		SELECT student_id FROM active.attendance
		WHERE tenant_id = ? AND student_id IN (?) AND check_out_time IS NULL
		UNION
		SELECT student_id FROM active.visits
		WHERE tenant_id = ? AND student_id IN (?) AND exit_time IS NULL
		ORDER BY student_id
	`, tenantID, bun.List(ids), tenantID, bun.List(ids)).Scan(ctx, &result)
	stats := ports.Stats{Queries: 1, Rows: int64(len(result)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find open presence: %w", err)
	}
	return result, stats, nil
}

func (s *Store) LockOpenVisits(ctx context.Context, groupID int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().TableExpr(`active.visits AS "visit"`).ColumnExpr(`"visit".id`).
		Where(`"visit".tenant_id = ?`, tenantID).Where(`"visit".active_group_id = ?`, groupID).
		Where(`"visit".exit_time IS NULL`).OrderExpr(`"visit".id ASC`).For("UPDATE").Scan(ctx, &ids)
	stats := ports.Stats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock open visits: %w", err)
	}
	return stats, nil
}

func (s *Store) RestoreVisits(ctx context.Context, ids []int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if len(ids) == 0 {
		return ports.Stats{}, nil
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.visits").Set("exit_time = NULL").
		Where("tenant_id = ?", tenantID).Where("id IN (?) AND exit_time IS NOT NULL", bun.List(ids)).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("restore visits: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("restore visits: %w", err)
	}
	if stats.Rows != int64(len(ids)) {
		return stats, fmt.Errorf("restore visits: snapshot mismatch for visits: expected %d rows, updated %d", len(ids), stats.Rows)
	}
	return stats, nil
}

func (s *Store) CloseOpenPresence(ctx context.Context, ids []int64, at time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var stats ports.Stats
	if len(ids) == 0 {
		return stats, nil
	}
	for _, query := range []*bun.UpdateQuery{
		db.NewUpdate().Table("active.attendance").Set("check_out_time = ?", at).Set("updated_at = ?", at).
			Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(ids)).Where("check_out_time IS NULL"),
		db.NewUpdate().Table("active.visits").Set("exit_time = ?", at).Set("updated_at = ?", at).
			Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(ids)).Where("exit_time IS NULL"),
	} {
		started := time.Now()
		result, err := query.Exec(ctx)
		stats.Queries++
		stats.StatementDuration += time.Since(started)
		if err != nil {
			return stats, fmt.Errorf("close open presence: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return stats, fmt.Errorf("close open presence: %w", err)
		}
		stats.Rows += rows
	}
	return stats, nil
}

func (s *Store) LatestPresenceDate(ctx context.Context, studentID int64) (*string, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var day *string
	started := time.Now()
	err = db.NewRaw(`
		SELECT MAX(recorded.day)::text FROM (
			SELECT attendance.date AS day
			FROM active.attendance AS attendance
			WHERE attendance.tenant_id = ? AND attendance.student_id = ?
			UNION ALL
			SELECT (visit.entry_time AT TIME ZONE 'Europe/Berlin')::date AS day
			FROM active.visits AS visit
			WHERE visit.tenant_id = ? AND visit.student_id = ?
		) AS recorded
	`, tenantID, studentID, tenantID, studentID).Scan(ctx, &day)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find latest attendance before care exit: %w", err)
	}
	return day, stats, nil
}

func (s *Store) CountAttendanceRecords(ctx context.Context, studentID int64) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	var count int
	err = db.NewRaw(`
		SELECT (
			(SELECT COUNT(*) FROM active.attendance WHERE tenant_id = ? AND student_id = ?) +
			(SELECT COUNT(*) FROM active.scheduled_checkouts WHERE tenant_id = ? AND student_id = ?)
		)::int
	`, tenantID, studentID, tenantID, studentID).Scan(ctx, &count)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("count attendance records: %w", err)
	}
	return count, stats, nil
}

func (s *Store) LockOpenPresence(ctx context.Context, ids []int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	var stats ports.Stats
	if len(ids) == 0 {
		return stats, nil
	}
	for _, statement := range []struct {
		op    string
		query *bun.SelectQuery
	}{
		{"lock attendance for care exit", db.NewSelect().Table("active.attendance").Column("id").
			Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(ids)).Where("check_out_time IS NULL").Order("id").For("UPDATE")},
		{"lock visits for care exit", db.NewSelect().Table("active.visits").Column("id").
			Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(ids)).Where("exit_time IS NULL").Order("id").For("UPDATE")},
	} {
		started := time.Now()
		var locked []int64
		err := statement.query.Scan(ctx, &locked)
		stats.Queries++
		stats.StatementDuration += time.Since(started)
		if err != nil {
			return stats, fmt.Errorf("%s: %w", statement.op, err)
		}
	}
	return stats, nil
}
