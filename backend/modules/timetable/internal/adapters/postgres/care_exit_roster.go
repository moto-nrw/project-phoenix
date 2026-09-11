package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

// A manually set future status is still a plan; only observed presence and
// completed/cancelled instances are excluded from a care exit.
const plannedCareExitRosterPredicate = `
 AND s.checked_in_at IS NULL AND s.checked_out_at IS NULL
 AND ai.date > ? AND ai.status NOT IN ('completed', 'cancelled')
 AND s.tenant_id = ?`

func (s *Store) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.ExecContext(ctx, `SELECT s.instance_id FROM schedule.instance_students AS s
  JOIN schedule.activity_instances AS ai ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id
  WHERE s.student_id IN (?)`+plannedCareExitRosterPredicate+` FOR UPDATE OF s`, bun.List(studentIDs), after, tenantID)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("timetable postgres: lock planned roster for care exit: %w", err)
	}
	return stats, nil
}
func (s *Store) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]domain.CareExitRosterRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.CareExitRosterRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`DELETE FROM schedule.instance_students AS s
  USING schedule.activity_instances AS ai
  WHERE s.instance_id = ai.id AND s.tenant_id = ai.tenant_id AND s.student_id IN (?)`+plannedCareExitRosterPredicate+`
  RETURNING s.tenant_id, s.student_id, s.instance_id, s.room_id, s.status, s.substatus, s.note,
   s.is_unplanned, s.not_scheduled, s.manual_status_at, s.student_status_day_id, s.pickup_exception_id`,
		bun.List(studentIDs), after, tenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: remove planned roster for care exit: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}
func (s *Store) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []domain.CareExitRosterRow) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return 0, domain.OperationStats{}, fmt.Errorf("timetable postgres: encode care exit roster: %w", err)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.ExecContext(ctx, `INSERT INTO schedule.instance_students (
  tenant_id, instance_id, student_id, room_id, status, substatus, note,
  is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id)
 SELECT rm.tenant_id, rm.instance_id, rm.student_id, rm.room_id, rm.status, rm.substatus, rm.note,
  rm.is_unplanned, rm.not_scheduled, rm.manual_status_at, rm.student_status_day_id, rm.pickup_exception_id
 FROM jsonb_to_recordset(?::jsonb) AS rm(tenant_id bigint, student_id bigint, instance_id bigint,
  room_id bigint, status text, substatus text, note text, is_unplanned boolean, not_scheduled boolean,
  manual_status_at timestamptz, student_status_day_id bigint, pickup_exception_id bigint)
 JOIN schedule.activity_instances AS ai ON ai.tenant_id = rm.tenant_id AND ai.id = rm.instance_id
 WHERE rm.tenant_id = ? AND rm.student_id IN (?) AND ai.status NOT IN ('completed', 'cancelled')
 ON CONFLICT DO NOTHING`, string(payload), tenantID, bun.List(studentIDs))
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: restore care exit roster: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("timetable postgres: count restored care exit roster: %w", err)
	}
	stats.Rows = count
	return count, stats, nil
}
