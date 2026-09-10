package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

func verifyPresenceBackfill(ctx context.Context, tx bun.Tx, report *PresenceBackfillReport) error {
	if err := tx.NewRaw(`
 WITH source AS (
 SELECT id, created_at, jsonb_build_array(id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot, created_at, updated_at) AS payload
 FROM schedule.activity_instances WHERE tenant_id = ? AND status IN ('active', 'completed')
 ), target AS (
 SELECT schedule_instance_id AS id, jsonb_build_array(schedule_instance_id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot, created_at, updated_at) AS payload
 FROM active.activity_sessions WHERE tenant_id = ?
 )
 SELECT count(source.id) AS source_count, count(target.id) AS target_count,
 encode(sha256(coalesce(string_agg(sha256(convert_to(source.payload::text, 'UTF8')), ''::bytea ORDER BY source.id), ''::bytea)), 'hex') AS source_checksum,
 encode(sha256(coalesce(string_agg(sha256(convert_to(target.payload::text, 'UTF8')), ''::bytea ORDER BY target.id), ''::bytea)), 'hex') AS target_checksum,
 count(*) FILTER (WHERE source.payload IS DISTINCT FROM target.payload) AS mismatches,
 coalesce(max(greatest(0, extract(epoch FROM (clock_timestamp() - source.created_at))))
 FILTER (WHERE source.payload IS DISTINCT FROM target.payload), 0)::double precision AS oldest_unmigrated_seconds
 FROM source FULL JOIN target USING (id)`,
		report.TenantID, report.TenantID).Scan(ctx, &report.Sessions); err != nil {
		return err
	}
	if err := tx.NewRaw(`
 WITH source AS (
 SELECT id, created_at, jsonb_build_array(id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, created_at, updated_at) AS payload
 FROM schedule.instance_students WHERE tenant_id = ?
 ), target AS (
 SELECT instance_student_id AS id, jsonb_build_array(instance_student_id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, created_at, updated_at) AS payload
 FROM active.activity_session_attendance WHERE tenant_id = ?
 )
 SELECT count(source.id) AS source_count, count(target.id) AS target_count,
 encode(sha256(coalesce(string_agg(sha256(convert_to(source.payload::text, 'UTF8')), ''::bytea ORDER BY source.id), ''::bytea)), 'hex') AS source_checksum,
 encode(sha256(coalesce(string_agg(sha256(convert_to(target.payload::text, 'UTF8')), ''::bytea ORDER BY target.id), ''::bytea)), 'hex') AS target_checksum,
 count(*) FILTER (WHERE source.payload IS DISTINCT FROM target.payload) AS mismatches,
 coalesce(max(greatest(0, extract(epoch FROM (clock_timestamp() - source.created_at))))
 FILTER (WHERE source.payload IS DISTINCT FROM target.payload), 0)::double precision AS oldest_unmigrated_seconds
 FROM source FULL JOIN target USING (id)`,
		report.TenantID, report.TenantID).Scan(ctx, &report.Attendance); err != nil {
		return err
	}
	// Planned rows already have their target owner and remain in place. Record
	// the eventual planning-state projection without narrowing the old column.
	if err := tx.NewRaw(`
 SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(jsonb_build_array(
 id, tenant_id, date, activity_group_id, calendar_period_id, title, description, start_time, end_time, room_id, required_staff, list_kind, is_spontaneous, understaffed_ack, understaffed_note, cancel_reason, notes, created_by,
 CASE WHEN status IN ('active', 'completed') THEN 'planned' ELSE status END
 )::text, 'UTF8')), ''::bytea ORDER BY id), ''::bytea)), 'hex')
 FROM schedule.activity_instances WHERE tenant_id = ?`, report.TenantID).Scan(ctx, &report.PlannedOccurrences, &report.PlannedOccurrenceChecksum); err != nil {
		return err
	}
	if err := tx.NewRaw(`
 SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(jsonb_build_array(id, tenant_id, instance_id, student_id, room_id)::text, 'UTF8')), ''::bytea ORDER BY id), ''::bytea)), 'hex')
 FROM schedule.instance_students WHERE tenant_id = ?`, report.TenantID).Scan(ctx, &report.PlannedAssignments, &report.PlannedAssignmentChecksum); err != nil {
		return err
	}
	report.Mismatches = report.Sessions.Mismatches + report.Attendance.Mismatches
	if err := tx.NewRaw(`SELECT
  (SELECT count(*) FROM active.activity_sessions target LEFT JOIN schedule.activity_instances source
   ON source.tenant_id = target.tenant_id AND source.id = target.schedule_instance_id
   WHERE target.tenant_id = ? AND source.id IS NULL)
  + (SELECT count(*) FROM active.activity_session_attendance target LEFT JOIN schedule.instance_students source
   ON source.tenant_id = target.tenant_id AND source.id = target.instance_student_id
   WHERE target.tenant_id = ? AND source.id IS NULL)`, report.TenantID, report.TenantID).Scan(ctx, &report.Orphans); err != nil {
		return err
	}
	if report.Mismatches != 0 || report.Orphans != 0 {
		report.Phase = "sessions"
		report.SessionHighWater, report.AttendanceHighWater = 0, 0
		report.Pass++
		return nil
	}
	var verifiedAt time.Time
	if err := tx.NewRaw(`SELECT clock_timestamp(), txid_current_snapshot()::text`).Scan(ctx, &verifiedAt, &report.FinalDeltaSnapshot); err != nil {
		return err
	}
	report.VerifiedAt = &verifiedAt
	report.Complete = true
	report.Phase = "complete"
	return nil
}
