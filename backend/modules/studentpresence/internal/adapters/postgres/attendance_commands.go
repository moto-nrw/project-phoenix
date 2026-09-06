package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Store) RecordAttendance(ctx context.Context, row *ports.Attendance) (ports.Stats, error) {
	_, stats, err := s.insertAttendance(ctx, row, false)
	return stats, err
}

func (s *Store) ReviseAttendance(ctx context.Context, row *ports.Attendance) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	if row.TenantID != 0 && row.TenantID != tenantID {
		return ports.Stats{}, errors.New("student presence: attendance belongs to another tenant")
	}
	row.TenantID = tenantID
	started := time.Now()
	result, err := db.NewUpdate().Model(attendanceRowFromRecord(row)).ModelTableExpr(`active.attendance AS "attendance"`).
		WherePK().Where("attendance.tenant_id = ?", tenantID).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("update attendance: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("update attendance: %w", err)
	}
	if stats.Rows != 1 {
		return stats, fmt.Errorf("expected 1 rows affected, got %d", stats.Rows)
	}
	return stats, nil
}

func (s *Store) DeleteAttendance(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Table("active.attendance").Where("tenant_id = ?", tenantID).Where("id = ?", id).Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("delete attendance: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) EnsureAttendanceBatch(ctx context.Context, rows []*ports.Attendance) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	if len(rows) == 0 {
		return []int64{}, ports.Stats{}, nil
	}
	stored := make([]*attendanceRow, 0, len(rows))
	for _, row := range rows {
		if row.TenantID != 0 && row.TenantID != tenantID {
			return nil, ports.Stats{}, errors.New("student presence: attendance belongs to another tenant")
		}
		row.TenantID = tenantID
		stored = append(stored, attendanceRowFromRecord(row))
	}
	inserted := []int64{}
	started := time.Now()
	err = db.NewInsert().Model(&stored).ModelTableExpr("active.attendance").
		On("CONFLICT (student_id, date) WHERE check_out_time IS NULL DO NOTHING").
		Returning("student_id").Scan(ctx, &inserted)
	stats := ports.Stats{Queries: 1, Rows: int64(len(inserted)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("ensure attendance batch: %w", err)
	}
	return inserted, stats, nil
}

func (s *Store) CloseStaleAttendance(ctx context.Context, id int64, at, updatedAt time.Time) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	started := time.Now()
	result, err := db.NewUpdate().Table("active.attendance").Set("check_out_time = ?", at).Set("updated_at = ?", updatedAt).
		Where("tenant_id = ?", tenantID).Where("id = ?", id).Where("check_out_time IS NULL").Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("close stale attendance: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats, err
}

func (s *Store) LockStudentAttendance(ctx context.Context, id int64) (ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return ports.Stats{}, err
	}
	key := fmt.Sprintf("attendance:%d:%d", tenantID, id)
	started := time.Now()
	_, err = db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("lock student attendance: %w", err)
	}
	return stats, nil
}

func (s *Store) HasAttendance(ctx context.Context, filter ports.AttendanceFilter) (bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, ports.Stats{}, err
	}
	if (filter.IDs != nil && len(filter.IDs) == 0) || (filter.StudentIDs != nil && len(filter.StudentIDs) == 0) {
		return false, ports.Stats{}, nil
	}
	started := time.Now()
	exists, err := attendanceQuery(db, tenantID, (*attendanceRow)(nil), filter).Exists(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	return exists, stats, err
}

func (s *Store) CountAttendanceByStaff(ctx context.Context, id int64) (int, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().Table("active.attendance").Where("tenant_id = ?", tenantID).
		Where("(checked_in_by = ? OR checked_out_by = ?)", id, id).Count(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	return count, stats, err
}

func (s *Store) ListOpenAttendanceStudentIDs(ctx context.Context, date string) ([]int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []int64{}
	started := time.Now()
	err = db.NewSelect().Table("active.attendance").ColumnExpr("DISTINCT student_id").Where("tenant_id = ?", tenantID).
		Where("date = ?", date).Where("check_out_time IS NULL").Order("student_id").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	return rows, stats, err
}
