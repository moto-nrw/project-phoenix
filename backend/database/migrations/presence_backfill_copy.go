package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func copyPresenceSessions(ctx context.Context, tx bun.Tx, tenantID int64, ids []int64) (int64, int64, error) {
	// A group may have moved to another instance since its previous batch.
	// Release only stale target bindings before the upsert, including swaps
	// within this batch. Never change the authoritative group assignment.
	_, err := tx.ExecContext(ctx, `
 UPDATE active.activity_sessions target SET active_group_id = NULL
 FROM schedule.activity_instances incoming
 WHERE incoming.tenant_id = ? AND incoming.id IN (?)
 AND incoming.status IN ('active', 'completed')
 AND target.tenant_id = incoming.tenant_id
 AND target.active_group_id = incoming.active_group_id
 AND target.schedule_instance_id <> incoming.id`, tenantID, bun.List(ids))
	if err != nil {
		return 0, 0, err
	}
	deleted, err := tx.ExecContext(ctx, `
 DELETE FROM active.activity_sessions target USING schedule.activity_instances source
 WHERE source.tenant_id = ? AND source.id IN (?)
 AND source.status IN ('planned', 'cancelled')
 AND target.tenant_id = source.tenant_id AND target.schedule_instance_id = source.id`, tenantID, bun.List(ids))
	if err != nil {
		return 0, 0, err
	}
	removed, err := deleted.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	result, err := tx.ExecContext(ctx, `
 INSERT INTO active.activity_sessions AS target (tenant_id, schedule_instance_id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot, created_at, updated_at)
 SELECT tenant_id, id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot, created_at, updated_at FROM schedule.activity_instances
 WHERE tenant_id = ? AND id IN (?) AND status IN ('active', 'completed')
 ON CONFLICT (tenant_id, schedule_instance_id) DO UPDATE SET
 status = EXCLUDED.status,
 active_group_id = EXCLUDED.active_group_id,
 started_by = EXCLUDED.started_by,
 started_at = EXCLUDED.started_at,
 completed_at = EXCLUDED.completed_at,
 completed_by = EXCLUDED.completed_by,
 reopen_until = EXCLUDED.reopen_until,
 completion_snapshot = EXCLUDED.completion_snapshot,
 created_at = EXCLUDED.created_at,
 updated_at = EXCLUDED.updated_at
 WHERE ROW(target.status, target.active_group_id, target.started_by, target.started_at, target.completed_at, target.completed_by, target.reopen_until, target.completion_snapshot, target.created_at, target.updated_at)
 IS DISTINCT FROM ROW(EXCLUDED.status, EXCLUDED.active_group_id, EXCLUDED.started_by, EXCLUDED.started_at, EXCLUDED.completed_at, EXCLUDED.completed_by, EXCLUDED.reopen_until, EXCLUDED.completion_snapshot, EXCLUDED.created_at, EXCLUDED.updated_at)`, tenantID, bun.List(ids))
	if err != nil {
		return 0, 0, err
	}
	copied, err := result.RowsAffected()
	return copied, removed, err
}

func copyPresenceAttendance(ctx context.Context, tx bun.Tx, tenantID int64, ids []int64) (int64, error) {
	result, err := tx.ExecContext(ctx, `
 INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, created_at, updated_at)
 SELECT tenant_id, id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, created_at, updated_at FROM schedule.instance_students
 WHERE tenant_id = ? AND id IN (?)
 ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
 status = EXCLUDED.status,
 substatus = EXCLUDED.substatus,
 note = EXCLUDED.note,
 checked_in_at = EXCLUDED.checked_in_at,
 checked_out_at = EXCLUDED.checked_out_at,
 is_unplanned = EXCLUDED.is_unplanned,
 not_scheduled = EXCLUDED.not_scheduled,
 manual_status_at = EXCLUDED.manual_status_at,
 student_status_day_id = EXCLUDED.student_status_day_id,
 pickup_exception_id = EXCLUDED.pickup_exception_id,
 created_at = EXCLUDED.created_at,
 updated_at = EXCLUDED.updated_at
 WHERE ROW(target.status, target.substatus, target.note, target.checked_in_at, target.checked_out_at, target.is_unplanned, target.not_scheduled, target.manual_status_at, target.student_status_day_id, target.pickup_exception_id, target.created_at, target.updated_at)
 IS DISTINCT FROM ROW(EXCLUDED.status, EXCLUDED.substatus, EXCLUDED.note, EXCLUDED.checked_in_at, EXCLUDED.checked_out_at, EXCLUDED.is_unplanned, EXCLUDED.not_scheduled, EXCLUDED.manual_status_at, EXCLUDED.student_status_day_id, EXCLUDED.pickup_exception_id, EXCLUDED.created_at, EXCLUDED.updated_at)`, tenantID, bun.List(ids))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
