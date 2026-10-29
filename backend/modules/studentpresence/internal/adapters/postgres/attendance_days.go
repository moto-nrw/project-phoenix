package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// ListAttendanceDays counts school days, not the number of check-ins.
func (s *Store) ListAttendanceDays(ctx context.Context, from, to ports.Date) ([]ports.AttendanceDay, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []ports.AttendanceDay
	started := time.Now()
	err = db.NewSelect().Table("active.attendance").Column("student_id", "date").
		Where("tenant_id = ?", tenantID).Where("date >= ? AND date <= ?", from, to).
		Group("student_id", "date").Order("student_id", "date").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("statistics attendance days: %w", err)
	}
	return rows, stats, nil
}

// ListAttendanceSpans reads the first check-in and the last check-out of each
// child-day (#4774). A day with an open row has no last check-out yet.
func (s *Store) ListAttendanceSpans(ctx context.Context, from, to ports.Date, studentIDs []int64) ([]ports.AttendanceSpan, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var rows []struct {
		StudentID    int64      `bun:"student_id"`
		Date         ports.Date `bun:"date"`
		FirstCheckIn time.Time  `bun:"first_check_in"`
		LastCheckOut *time.Time `bun:"last_check_out"`
		Open         bool       `bun:"open"`
	}
	started := time.Now()
	query := db.NewSelect().Table("active.attendance").
		Column("student_id", "date").
		ColumnExpr("MIN(check_in_time) AS first_check_in").
		ColumnExpr("MAX(check_out_time) AS last_check_out").
		ColumnExpr("BOOL_OR(check_out_time IS NULL) AS open").
		Where("tenant_id = ?", tenantID).Where("date >= ? AND date <= ?", from, to)
	if len(studentIDs) > 0 {
		query = query.Where("student_id IN (?)", bun.In(studentIDs))
	}
	err = query.Group("student_id", "date").Order("student_id", "date").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("attendance spans: %w", err)
	}
	spans := make([]ports.AttendanceSpan, 0, len(rows))
	for _, row := range rows {
		span := ports.AttendanceSpan{StudentID: row.StudentID, Date: row.Date, FirstCheckIn: row.FirstCheckIn}
		if !row.Open {
			span.LastCheckOut = row.LastCheckOut
		}
		spans = append(spans, span)
	}
	return spans, stats, nil
}
