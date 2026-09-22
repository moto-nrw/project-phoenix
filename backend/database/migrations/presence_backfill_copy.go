package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// presenceSourceScope narrows a copy statement to one keyset batch. A nil id
// list means the whole school, which the cutover's final delta uses under its
// write lock.
func presenceSourceScope(alias string, ids []int64) (string, []any) {
	if ids == nil {
		return "", nil
	}
	return " AND " + alias + ".id IN (?)", []any{bun.List(ids)}
}

func copyPresenceSessions(ctx context.Context, tx bun.Tx, tenantID int64, ids []int64) (int64, int64, error) {
	// A group may have moved to another instance since its previous batch.
	// Release only stale target bindings before the upsert, including swaps
	// within this batch. Never change the authoritative group assignment.
	scope, args := presenceSourceScope("incoming", ids)
	_, err := tx.ExecContext(ctx, `
 UPDATE active.activity_sessions target SET active_group_id = NULL
 FROM schedule.activity_instances incoming
 WHERE incoming.tenant_id = ?`+scope+`
 AND incoming.status IN ('active', 'completed')
 AND target.tenant_id = incoming.tenant_id
 AND target.active_group_id = incoming.active_group_id
 AND target.schedule_instance_id <> incoming.id`, append([]any{tenantID}, args...)...)
	if err != nil {
		return 0, 0, err
	}
	scope, args = presenceSourceScope("source", ids)
	deleted, err := tx.ExecContext(ctx, `
 DELETE FROM active.activity_sessions target USING schedule.activity_instances source
 WHERE source.tenant_id = ?`+scope+`
 AND source.status IN ('planned', 'cancelled')
 AND target.tenant_id = source.tenant_id AND target.schedule_instance_id = source.id`, append([]any{tenantID}, args...)...)
	if err != nil {
		return 0, 0, err
	}
	removed, err := deleted.RowsAffected()
	if err != nil {
		return 0, 0, err
	}
	scope, args = presenceSourceScope("source", ids)
	result, err := tx.ExecContext(ctx, `
 INSERT INTO active.activity_sessions AS target (tenant_id, schedule_instance_id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot, created_at, updated_at)
 SELECT source.tenant_id, source.id, source.status, source.active_group_id, source.started_by, source.started_at, source.completed_at, source.completed_by, source.reopen_until, source.completion_snapshot, source.created_at, source.updated_at FROM schedule.activity_instances source
 WHERE source.tenant_id = ?`+scope+` AND source.status IN ('active', 'completed')
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
 IS DISTINCT FROM ROW(EXCLUDED.status, EXCLUDED.active_group_id, EXCLUDED.started_by, EXCLUDED.started_at, EXCLUDED.completed_at, EXCLUDED.completed_by, EXCLUDED.reopen_until, EXCLUDED.completion_snapshot, EXCLUDED.created_at, EXCLUDED.updated_at)`, append([]any{tenantID}, args...)...)
	if err != nil {
		return 0, 0, err
	}
	copied, err := result.RowsAffected()
	return copied, removed, err
}

func copyPresenceAttendance(ctx context.Context, tx bun.Tx, tenantID int64, ids []int64) (int64, error) {
	scope, args := presenceSourceScope("source", ids)
	result, err := tx.ExecContext(ctx, `
 INSERT INTO active.activity_session_attendance AS target (tenant_id, instance_student_id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, created_at, updated_at)
 SELECT source.tenant_id, source.id, source.status, source.substatus, source.note, source.checked_in_at, source.checked_out_at, source.is_unplanned, source.not_scheduled, source.manual_status_at, source.student_status_day_id, source.pickup_exception_id, source.created_at, source.updated_at FROM schedule.instance_students source
 WHERE source.tenant_id = ?`+scope+`
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
 IS DISTINCT FROM ROW(EXCLUDED.status, EXCLUDED.substatus, EXCLUDED.note, EXCLUDED.checked_in_at, EXCLUDED.checked_out_at, EXCLUDED.is_unplanned, EXCLUDED.not_scheduled, EXCLUDED.manual_status_at, EXCLUDED.student_status_day_id, EXCLUDED.pickup_exception_id, EXCLUDED.created_at, EXCLUDED.updated_at)`, append([]any{tenantID}, args...)...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
