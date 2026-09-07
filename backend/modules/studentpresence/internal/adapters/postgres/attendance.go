package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

func (s *Store) FindAttendance(ctx context.Context, id int64) (*ports.Attendance, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	row := new(attendanceRow)
	started := time.Now()
	err = db.NewSelect().Model(row).ModelTableExpr(`active.attendance AS "attendance"`).
		Where("attendance.tenant_id = ?", tenantID).Where("attendance.id = ?", id).Scan(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("find attendance: %w", err)
	}
	stats.Rows = 1
	return row.record(), stats, nil
}

func attendanceQuery(db bun.IDB, tenantID int64, rows any, filter ports.AttendanceFilter) *bun.SelectQuery {
	query := db.NewSelect().Model(rows).ModelTableExpr(`active.attendance AS "attendance"`).Where("attendance.tenant_id = ?", tenantID)
	if filter.IDs != nil {
		query = query.Where("attendance.id IN (?)", bun.List(filter.IDs))
	}
	if filter.StudentIDs != nil {
		query = query.Where("attendance.student_id IN (?)", bun.List(filter.StudentIDs))
	}
	if filter.FromDate != "" {
		query = query.Where("attendance.date >= ?", filter.FromDate)
	}
	if filter.UntilDate != "" {
		query = query.Where("attendance.date <= ?", filter.UntilDate)
	}
	if filter.BeforeDate != "" {
		query = query.Where("attendance.date < ?", filter.BeforeDate)
	}
	if filter.OpenOnly {
		query = query.Where("attendance.check_out_time IS NULL")
	}
	if filter.StudentOrder {
		query = query.OrderExpr("attendance.student_id ASC")
	}
	if filter.NewestFirst {
		query = query.OrderExpr("attendance.date DESC, attendance.check_in_time DESC")
	} else {
		query = query.OrderExpr("attendance.check_in_time ASC")
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.ForUpdate {
		query = query.For("UPDATE")
	}
	return query
}

func (s *Store) ListAttendance(ctx context.Context, filter ports.AttendanceFilter) ([]*ports.Attendance, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []*attendanceRow{}
	if (filter.IDs != nil && len(filter.IDs) == 0) || (filter.StudentIDs != nil && len(filter.StudentIDs) == 0) {
		return []*ports.Attendance{}, ports.Stats{}, nil
	}
	started := time.Now()
	err = attendanceQuery(db, tenantID, &rows, filter).Scan(ctx)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("list attendance: %w", err)
	}
	return attendanceRecordsFromRows(rows), stats, nil
}

func (s *Store) EnsureAttendance(ctx context.Context, row *ports.Attendance) (bool, ports.Stats, error) {
	return s.insertAttendance(ctx, row, true)
}

func (s *Store) insertAttendance(ctx context.Context, row *ports.Attendance, ifAbsent bool) (bool, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, ports.Stats{}, err
	}
	if row.TenantID != 0 && row.TenantID != tenantID {
		return false, ports.Stats{}, errors.New("student presence: attendance belongs to another tenant")
	}
	row.TenantID = tenantID
	stored := attendanceRowFromRecord(row)
	started := time.Now()
	query := db.NewInsert().Model(stored).ModelTableExpr("active.attendance")
	if ifAbsent {
		query = query.On("CONFLICT (student_id, date) WHERE check_out_time IS NULL DO NOTHING")
	}
	result, err := query.Exec(ctx)
	stats := ports.Stats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("ensure attendance: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("ensure attendance: %w", err)
	}
	*row = *stored.record()
	return stats.Rows > 0, stats, nil
}

func (s *Store) CloseAttendance(ctx context.Context, checkout ports.AttendanceCheckout) ([]*ports.Attendance, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []*attendanceRow{}
	if len(checkout.StudentIDs) == 0 {
		return []*ports.Attendance{}, ports.Stats{}, nil
	}
	query := db.NewUpdate().Model((*attendanceRow)(nil)).ModelTableExpr("active.attendance").
		Set("check_out_time = ?", checkout.At).Set("yard_since = NULL").
		Where("tenant_id = ?", tenantID).Where("student_id IN (?)", bun.List(checkout.StudentIDs)).
		Where("date = ?", checkout.Date).Where("check_out_time IS NULL").Returning("*")
	if checkout.StaffID > 0 {
		query = query.Set("checked_out_by = ?", checkout.StaffID)
	}
	if checkout.DeviceID > 0 {
		query = query.Set("checked_out_device_id = ?", checkout.DeviceID)
	}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, stats, fmt.Errorf("close attendance: %w", err)
	}
	return attendanceRecordsFromRows(rows), stats, nil
}
