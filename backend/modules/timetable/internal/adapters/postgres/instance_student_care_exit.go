package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// A care exit removes plans, not recorded events, and leaves completed or
// cancelled instances untouched. The cutoff is the inclusive last care day.
const carePlannedRosterPredicate = `
 AND s.checked_in_at IS NULL AND s.checked_out_at IS NULL
 AND ai.date > ?::date AND ai.status NOT IN ('completed', 'cancelled')
 AND s.tenant_id = ?`

type careExitRosterSnapshot struct {
	TenantID           int64      `json:"tenant_id"`
	StudentID          int64      `json:"student_id"`
	InstanceID         int64      `json:"instance_id"`
	RoomID             *int64     `json:"room_id"`
	Status             string     `json:"status"`
	Substatus          *string    `json:"substatus"`
	Note               *string    `json:"note"`
	IsUnplanned        bool       `json:"is_unplanned"`
	NotScheduled       bool       `json:"not_scheduled"`
	ManualStatusAt     *time.Time `json:"manual_status_at"`
	StudentStatusDayID *int64     `json:"student_status_day_id"`
	PickupExceptionID  *int64     `json:"pickup_exception_id"`
}

const careExitRosterRecordset = `jsonb_to_recordset(?::jsonb) AS rm(
 tenant_id bigint, student_id bigint, instance_id bigint,
 room_id bigint, status text, substatus text, note text,
 is_unplanned boolean, not_scheduled boolean, manual_status_at timestamptz,
 student_status_day_id bigint, pickup_exception_id bigint
)`

func encodeCareExitRoster(removals []domain.InstanceStudent) (string, error) {
	rows := make([]careExitRosterSnapshot, 0, len(removals))
	for _, row := range removals {
		rows = append(rows, careExitRosterSnapshot{
			TenantID: row.TenantID, StudentID: row.StudentID, InstanceID: row.InstanceID,
			RoomID: row.RoomID, Status: row.Status, Substatus: row.Substatus, Note: row.Note,
			IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	encoded, err := json.Marshal(rows)
	return string(encoded), err
}

func (s *Store) CountPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string, removals []domain.InstanceStudent) (map[int64]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	snapshot, err := encodeCareExitRoster(removals)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Total     int   `bun:"total"`
	}
	started := time.Now()
	err = db.NewRaw(`
 SELECT student_id, COUNT(*)::int AS total FROM (
  SELECT s.student_id FROM schedule.instance_students AS s
  JOIN schedule.activity_instances AS ai ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id
  WHERE s.student_id IN (?)`+carePlannedRosterPredicate+`
  UNION ALL
  SELECT rm.student_id FROM `+careExitRosterRecordset+`
  JOIN schedule.activity_instances AS ai ON ai.id = rm.instance_id AND ai.tenant_id = rm.tenant_id
  WHERE rm.tenant_id = ? AND rm.student_id IN (?) AND ai.date > ?::date
  AND ai.status NOT IN ('completed', 'cancelled')
  AND NOT EXISTS (
   SELECT 1 FROM schedule.instance_students AS live
   WHERE live.instance_id = rm.instance_id AND live.student_id = rm.student_id AND live.tenant_id = rm.tenant_id
  )
 ) AS baseline GROUP BY student_id`, bun.List(studentIDs), after, tenantID,
		snapshot, tenantID, bun.List(studentIDs), after).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: count planned roster rows after care end: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.StudentID] = row.Total
	}
	return counts, stats, nil
}

func (s *Store) RemovePlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) ([]domain.InstanceStudent, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []instanceStudentRow
	started := time.Now()
	err = db.NewRaw(`
 DELETE FROM schedule.instance_students AS s USING schedule.activity_instances AS ai
 WHERE s.instance_id = ai.id AND s.tenant_id = ai.tenant_id AND s.student_id IN (?)`+carePlannedRosterPredicate+`
 RETURNING s.*`, bun.List(studentIDs), after, tenantID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, classifyWriteError("remove planned roster rows after care end", err, &stats)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.InstanceStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, instanceStudentToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) RestoreCareExitStudentAssignments(ctx context.Context, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs []int64, removals []domain.InstanceStudent) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	snapshot, err := encodeCareExitRoster(removals)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats, err := execMeasuredWrite(ctx, db.NewRaw(`
 INSERT INTO schedule.instance_students (
 tenant_id, instance_id, student_id, room_id, status, substatus, note,
 is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id
 )
 SELECT rm.tenant_id, rm.instance_id, rm.student_id,
 CASE WHEN rm.room_id = ANY(?::BIGINT[]) THEN rm.room_id END,
 rm.status, rm.substatus, rm.note, rm.is_unplanned, rm.not_scheduled, rm.manual_status_at,
 CASE WHEN rm.student_status_day_id = ANY(?::BIGINT[]) THEN rm.student_status_day_id END,
 CASE WHEN rm.pickup_exception_id = ANY(?::BIGINT[]) THEN rm.pickup_exception_id END
 FROM `+careExitRosterRecordset+`
 JOIN schedule.activity_instances AS ai ON ai.tenant_id = rm.tenant_id AND ai.id = rm.instance_id
 WHERE rm.tenant_id = ? AND rm.student_id IN (?) AND ai.status NOT IN ('completed', 'cancelled')
 ON CONFLICT DO NOTHING`, pgdialect.Array(roomIDs), pgdialect.Array(statusDayIDs), pgdialect.Array(pickupExceptionIDs),
		snapshot, tenantID, bun.List(studentIDs)), "restore roster rows after care exit change")
	return stats.Rows, stats, err
}
