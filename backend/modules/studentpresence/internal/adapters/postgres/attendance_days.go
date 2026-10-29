package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
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
