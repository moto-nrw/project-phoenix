package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type careExitCount struct {
	StudentID int64 `bun:"student_id"`
	Total     int   `bun:"total"`
}

// CountRunningEnrollmentsForCareExit counts the live bookings still running on
// validUntil. A booking the earlier exit capped is judged by its previous end.
func (s *Store) CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string, capped []domain.CareExitEnrollmentRemoval) (map[int64]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	payload, err := json.Marshal(careExitEnrollmentPayload(capped))
	if err != nil {
		return nil, domain.OperationStats{}, fmt.Errorf("timetable postgres: encode capped care exit bookings: %w", err)
	}
	return scanCareExitCounts(ctx, db.NewRaw(`SELECT enrollment.student_id AS student_id, COUNT(*)::int AS total
  FROM activities.student_enrollments AS enrollment
  LEFT JOIN jsonb_to_recordset(?::jsonb) AS removal(tenant_id bigint, enrollment_id bigint, previous_valid_until date)
    ON removal.tenant_id = enrollment.tenant_id AND removal.enrollment_id = enrollment.id
  WHERE enrollment.tenant_id = ? AND enrollment.student_id IN (?)
    AND ((removal.enrollment_id IS NULL AND (enrollment.valid_until IS NULL OR enrollment.valid_until > ?::date))
      OR (removal.enrollment_id IS NOT NULL AND (removal.previous_valid_until IS NULL OR removal.previous_valid_until > ?::date)))
  GROUP BY enrollment.student_id`, string(payload), tenantID, bun.List(studentIDs), validUntil, validUntil),
		"count running enrollments for care exit")
}

// CountRestorableEnrollmentsForCareExit counts the bookings the earlier exit
// deleted that would still run on validUntil and have not been re-created.
func (s *Store) CountRestorableEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string, deleted []domain.CareExitEnrollmentRemoval) (map[int64]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	payload, err := json.Marshal(careExitEnrollmentPayload(deleted))
	if err != nil {
		return nil, domain.OperationStats{}, fmt.Errorf("timetable postgres: encode deleted care exit bookings: %w", err)
	}
	return scanCareExitCounts(ctx, db.NewRaw(`SELECT removal.student_id AS student_id, COUNT(*)::int AS total
  FROM jsonb_to_recordset(?::jsonb) AS removal(tenant_id bigint, student_id bigint, enrollment_id bigint, previous_valid_until date)
  WHERE removal.tenant_id = ? AND removal.student_id IN (?)
    AND (removal.previous_valid_until IS NULL OR removal.previous_valid_until > ?::date)
    AND NOT EXISTS (
      SELECT 1 FROM activities.student_enrollments AS live
       WHERE live.id = removal.enrollment_id AND live.tenant_id = removal.tenant_id)
  GROUP BY removal.student_id`, string(payload), tenantID, bun.List(studentIDs), validUntil),
		"count restorable enrollments for care exit")
}

func scanCareExitCounts(ctx context.Context, query *bun.RawQuery, operation string) (map[int64]int, domain.OperationStats, error) {
	var rows []careExitCount
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: %s: %w", operation, err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	stats.Rows = int64(len(rows))
	return counts, stats, nil
}
