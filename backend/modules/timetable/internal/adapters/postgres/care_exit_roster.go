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

const careExitRosterColumns = `s.id AS participant_id, s.tenant_id, s.student_id, s.instance_id, s.room_id`

// ListPlannedRosterForCareExit lists the students' participants on not
// cancelled instances after the day. Which of them are observed or on a
// block that ended is Student Presence's answer; the application asks it.
func (s *Store) ListPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string) ([]domain.CareExitRosterRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.CareExitRosterRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT `+careExitRosterColumns+` FROM schedule.instance_students AS s
  JOIN schedule.activity_instances AS ai ON ai.id = s.instance_id AND ai.tenant_id = s.tenant_id
  WHERE s.tenant_id = ? AND s.student_id IN (?) AND ai.date > ? AND ai.status <> 'cancelled'
  ORDER BY s.id`, tenantID, bun.List(studentIDs), after).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: list planned roster for care exit: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func (s *Store) RemovePlannedRosterForCareExit(ctx context.Context, participantIDs []int64) ([]domain.CareExitRosterRow, domain.OperationStats, error) {
	if len(participantIDs) == 0 {
		return []domain.CareExitRosterRow{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.CareExitRosterRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`DELETE FROM schedule.instance_students AS s
  WHERE s.tenant_id = ? AND s.id IN (?)
  RETURNING `+careExitRosterColumns, tenantID, bun.List(participantIDs)).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: remove planned roster for care exit: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

// RestoreRosterForCareExit re-inserts removed participants whose instance is
// not cancelled and not among the excluded (ended) ones, and returns the
// rows it inserted with their new ids.
func (s *Store) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, rows []domain.CareExitRosterRow, excludedInstanceIDs []int64) ([]domain.CareExitRosterRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return nil, domain.OperationStats{}, fmt.Errorf("timetable postgres: encode care exit roster: %w", err)
	}
	excluded := excludedInstanceIDs
	if excluded == nil {
		excluded = []int64{}
	}
	restored := []domain.CareExitRosterRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`WITH restored AS (
   INSERT INTO schedule.instance_students (tenant_id, instance_id, student_id, room_id)
   SELECT rm.tenant_id, rm.instance_id, rm.student_id, rm.room_id
   FROM jsonb_to_recordset(?::jsonb) AS rm(tenant_id bigint, student_id bigint, instance_id bigint, room_id bigint)
   JOIN schedule.activity_instances AS ai ON ai.tenant_id = rm.tenant_id AND ai.id = rm.instance_id
   WHERE rm.tenant_id = ? AND rm.student_id IN (?) AND ai.status <> 'cancelled'
     AND NOT (ai.id = ANY(?::bigint[]))
   ON CONFLICT DO NOTHING
   RETURNING id, tenant_id, student_id, instance_id, room_id)
  SELECT s.id AS participant_id, s.tenant_id, s.student_id, s.instance_id, s.room_id FROM restored AS s ORDER BY s.id`,
		string(payload), tenantID, bun.List(studentIDs), pgdialect.Array(excluded)).Scan(ctx, &restored)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: restore care exit roster: %w", err)
	}
	stats.Rows = int64(len(restored))
	return restored, stats, nil
}

// ListRestorableRosterForCareExit returns the removed rows whose instance is
// still after `after`, not cancelled, and does not list the child again.
func (s *Store) ListRestorableRosterForCareExit(ctx context.Context, studentIDs []int64, after string, restorable []domain.CareExitRosterRow) ([]domain.CareExitRosterRow, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	payload, err := json.Marshal(restorable)
	if err != nil {
		return nil, domain.OperationStats{}, fmt.Errorf("timetable postgres: encode restorable care exit roster: %w", err)
	}
	rows := []domain.CareExitRosterRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT rm.participant_id, rm.tenant_id, rm.student_id, rm.instance_id, rm.room_id
  FROM jsonb_to_recordset(?::jsonb) AS rm(participant_id bigint, tenant_id bigint, student_id bigint, instance_id bigint, room_id bigint)
  JOIN schedule.activity_instances AS ai ON ai.id = rm.instance_id AND ai.tenant_id = rm.tenant_id
  WHERE rm.tenant_id = ? AND rm.student_id IN (?)
    AND ai.date > ? AND ai.status <> 'cancelled'
    AND NOT EXISTS (
      SELECT 1 FROM schedule.instance_students AS live
       WHERE live.instance_id = rm.instance_id AND live.student_id = rm.student_id
         AND live.tenant_id = rm.tenant_id)
  ORDER BY rm.student_id, rm.instance_id`, string(payload), tenantID, bun.List(studentIDs), after).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("timetable postgres: list restorable roster for care exit: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}
