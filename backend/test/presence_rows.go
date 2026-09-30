package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The legacy activity instance and participant DTOs compose two owners since
// the presence cutover (#2762): Timetable plans the block and its participants
// (schedule.activity_instances, schedule.instance_students), Student Presence
// runs the block and records the attendance (active.activity_sessions,
// active.activity_session_attendance). The Contract (#2763) removed the old
// columns that mirrored the execution and the attendance onto the plan tables.
// These helpers let fixtures and tests arrange and read the composed shape
// directly in storage, the way the owners write it.

// activityExecutionColumns are the fields of the composed legacy instance DTO
// that active.activity_sessions stores.
var activityExecutionColumns = []string{
	"active_group_id", "started_by", "started_at", "completed_at", "completed_by", "reopen_until", "completion_snapshot",
}

// instanceStudentAttendanceColumns are the fields of the composed legacy
// participant DTO that active.activity_session_attendance stores.
var instanceStudentAttendanceColumns = []string{
	"status", "substatus", "note", "checked_in_at", "checked_out_at", "is_unplanned", "not_scheduled",
	"manual_status_at", "student_status_day_id", "pickup_exception_id",
}

func executionStatus(status string) bool {
	return status == schedule.InstanceStatusActive || status == schedule.InstanceStatusCompleted
}

// InsertActivityInstanceRow stores a composed instance DTO: the plan row, and
// for an active or completed block its session. row.ID, CreatedAt and
// UpdatedAt are filled in; the DTO keeps the status the caller asked for.
func InsertActivityInstanceRow(tb testing.TB, ctx context.Context, db bun.IDB, row *schedule.ActivityInstance) {
	tb.Helper()
	status := row.Status
	if status == "" {
		status = schedule.InstanceStatusPlanned
	}
	if executionStatus(status) {
		row.Status = schedule.InstanceStatusPlanned
	}
	_, err := db.NewInsert().Model(row).ModelTableExpr(`schedule.activity_instances`).
		ExcludeColumn(activityExecutionColumns...).Exec(ctx)
	row.Status = status
	require.NoError(tb, err, "insert activity instance plan")
	if executionStatus(status) {
		upsertActivitySession(tb, ctx, db, row)
	}
}

// SetActivityInstanceLifecycle moves one block into a lifecycle state:
// planned and cancelled are planning states of the block and end its session,
// active and completed are the state of its session on a planned block.
func SetActivityInstanceLifecycle(tb testing.TB, ctx context.Context, db bun.IDB, instanceID int64, status string) {
	tb.Helper()
	var tenantID int64
	require.NoError(tb, db.NewRaw(`SELECT tenant_id FROM schedule.activity_instances WHERE id = ?`, instanceID).
		Scan(ctx, &tenantID), "load activity instance %d", instanceID)
	if !executionStatus(status) {
		_, err := db.ExecContext(ctx, `DELETE FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`,
			tenantID, instanceID)
		require.NoError(tb, err, "end activity session")
		_, err = db.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = ? WHERE id = ?`, status, instanceID)
		require.NoError(tb, err, "set planning status")
		return
	}
	_, err := db.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = 'planned' WHERE id = ? AND status <> 'planned'`, instanceID)
	require.NoError(tb, err, "set planning status")
	_, err = db.ExecContext(ctx, `INSERT INTO active.activity_sessions (tenant_id, schedule_instance_id, status)
		VALUES (?, ?, ?)
		ON CONFLICT (tenant_id, schedule_instance_id) DO UPDATE SET status = EXCLUDED.status, updated_at = now()`,
		tenantID, instanceID, status)
	require.NoError(tb, err, "set activity session status")
}

// UpsertActivitySession writes every execution field of a composed instance
// DTO into its session. The block must exist and row.Status must be active
// or completed.
func UpsertActivitySession(tb testing.TB, ctx context.Context, db bun.IDB, row *schedule.ActivityInstance) {
	tb.Helper()
	require.True(tb, executionStatus(row.Status), "a session is active or completed, got %q", row.Status)
	upsertActivitySession(tb, ctx, db, row)
}

func upsertActivitySession(tb testing.TB, ctx context.Context, db bun.IDB, row *schedule.ActivityInstance) {
	tb.Helper()
	var snapshot any
	if len(row.CompletionSnapshot) > 0 {
		snapshot = string(row.CompletionSnapshot)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO active.activity_sessions
		(tenant_id, schedule_instance_id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb)
		ON CONFLICT (tenant_id, schedule_instance_id) DO UPDATE SET
			status = EXCLUDED.status, active_group_id = EXCLUDED.active_group_id, started_by = EXCLUDED.started_by,
			started_at = EXCLUDED.started_at, completed_at = EXCLUDED.completed_at, completed_by = EXCLUDED.completed_by,
			reopen_until = EXCLUDED.reopen_until, completion_snapshot = EXCLUDED.completion_snapshot, updated_at = now()`,
		row.TenantID, row.ID, row.Status, row.ActiveGroupID, row.StartedBy, row.StartedAt, row.CompletedAt,
		row.CompletedBy, row.ReopenUntil, snapshot)
	require.NoError(tb, err, "upsert activity session")
}

// ActivityInstanceByID reads one composed instance DTO: the plan and, when
// the block runs or ran, its session.
func ActivityInstanceByID(tb testing.TB, ctx context.Context, db bun.IDB, id int64) *schedule.ActivityInstance {
	tb.Helper()
	rows := ActivityInstancesWhere(tb, ctx, db, `"activity_instance".id = ?`, id)
	require.Len(tb, rows, 1, "activity instance %d", id)
	return rows[0]
}

// ActivityInstancesWhere reads the composed instance DTOs matching a
// condition on the plan row (alias "activity_instance") or its session
// (alias "session"), ordered by ID.
func ActivityInstancesWhere(tb testing.TB, ctx context.Context, db bun.IDB, condition string, args ...any) []*schedule.ActivityInstance {
	tb.Helper()
	var rows []*schedule.ActivityInstance
	require.NoError(tb, db.NewSelect().Model(&rows).ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Join(`LEFT JOIN active.activity_sessions AS session
			ON session.tenant_id = "activity_instance".tenant_id AND session.schedule_instance_id = "activity_instance".id`).
		ColumnExpr(`"activity_instance".id, "activity_instance".tenant_id, "activity_instance".created_at,
			"activity_instance".updated_at, "activity_instance".date, "activity_instance".activity_group_id,
			"activity_instance".calendar_period_id, "activity_instance".title, "activity_instance".description,
			"activity_instance".start_time, "activity_instance".end_time, "activity_instance".room_id,
			"activity_instance".required_staff, coalesce(session.status, "activity_instance".status) AS status,
			"activity_instance".list_kind, "activity_instance".is_spontaneous, "activity_instance".understaffed_ack,
			"activity_instance".understaffed_note, "activity_instance".cancel_reason, "activity_instance".notes,
			"activity_instance".idempotency_key, "activity_instance".idempotency_fingerprint,
			"activity_instance".created_by, session.active_group_id, session.started_by, session.started_at,
			session.completed_at, session.completed_by, session.reopen_until, session.completion_snapshot`).
		Where(condition, args...).OrderExpr(`"activity_instance".id`).Scan(ctx), "load activity instances")
	return rows
}

// InsertInstanceStudentRow stores a composed participant DTO: the plan row,
// and an attendance row unless the attendance is still the expected default.
// row.ID, CreatedAt and UpdatedAt are filled in.
func InsertInstanceStudentRow(tb testing.TB, ctx context.Context, db bun.IDB, row *schedule.InstanceStudent) {
	tb.Helper()
	if row.Status == "" {
		row.Status = schedule.AttendanceStatusExpected
	}
	_, err := db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_students`).
		ExcludeColumn(instanceStudentAttendanceColumns...).Exec(ctx)
	require.NoError(tb, err, "insert instance student plan")
	if !expectedAttendance(row) {
		UpsertSessionAttendance(tb, ctx, db, row)
	}
}

func expectedAttendance(row *schedule.InstanceStudent) bool {
	return row.Status == schedule.AttendanceStatusExpected && row.Substatus == nil && row.Note == nil &&
		row.CheckedInAt == nil && row.CheckedOutAt == nil && !row.IsUnplanned && !row.NotScheduled &&
		row.ManualStatusAt == nil && row.StudentStatusDayID == nil && row.PickupExceptionID == nil
}

// UpsertSessionAttendance writes every attendance field of a composed
// participant DTO into its attendance row. The participant must exist.
func UpsertSessionAttendance(tb testing.TB, ctx context.Context, db bun.IDB, row *schedule.InstanceStudent) {
	tb.Helper()
	status := row.Status
	if status == "" {
		status = schedule.AttendanceStatusExpected
	}
	_, err := db.ExecContext(ctx, `INSERT INTO active.activity_session_attendance
		(tenant_id, instance_student_id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned,
		 not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (tenant_id, instance_student_id) DO UPDATE SET
			status = EXCLUDED.status, substatus = EXCLUDED.substatus, note = EXCLUDED.note,
			checked_in_at = EXCLUDED.checked_in_at, checked_out_at = EXCLUDED.checked_out_at,
			is_unplanned = EXCLUDED.is_unplanned, not_scheduled = EXCLUDED.not_scheduled,
			manual_status_at = EXCLUDED.manual_status_at, student_status_day_id = EXCLUDED.student_status_day_id,
			pickup_exception_id = EXCLUDED.pickup_exception_id, updated_at = now()`,
		row.TenantID, row.ID, status, row.Substatus, row.Note, row.CheckedInAt, row.CheckedOutAt, row.IsUnplanned,
		row.NotScheduled, row.ManualStatusAt, row.StudentStatusDayID, row.PickupExceptionID)
	require.NoError(tb, err, "upsert session attendance")
}

// UpdateSessionAttendance changes attendance fields of one participant, the
// way an attendance command does; a participant without a row gets one.
// Keys are attendance column names.
func UpdateSessionAttendance(tb testing.TB, ctx context.Context, db bun.IDB, participantID int64, values map[string]any) {
	tb.Helper()
	row := InstanceStudentByIDContext(tb, ctx, db, participantID)
	for column, value := range values {
		applyAttendanceValue(tb, row, column, value)
	}
	UpsertSessionAttendance(tb, ctx, db, row)
}

func applyAttendanceValue(tb testing.TB, row *schedule.InstanceStudent, column string, value any) {
	tb.Helper()
	switch column {
	case "status":
		row.Status = value.(string)
	case "substatus":
		row.Substatus = optional[string](value)
	case "note":
		row.Note = optional[string](value)
	case "checked_in_at":
		row.CheckedInAt = optional[time.Time](value)
	case "checked_out_at":
		row.CheckedOutAt = optional[time.Time](value)
	case "is_unplanned":
		row.IsUnplanned = value.(bool)
	case "not_scheduled":
		row.NotScheduled = value.(bool)
	case "manual_status_at":
		row.ManualStatusAt = optional[time.Time](value)
	case "student_status_day_id":
		row.StudentStatusDayID = optional[int64](value)
	case "pickup_exception_id":
		row.PickupExceptionID = optional[int64](value)
	default:
		tb.Fatalf("update session attendance: unknown attendance column %q", column)
	}
}

// optional accepts a value, a pointer to it or nil for a nullable field.
func optional[T any](value any) *T {
	switch typed := value.(type) {
	case nil:
		return nil
	case T:
		return &typed
	default:
		return typed.(*T)
	}
}

// InstanceStudentByIDContext reloads one participant with its attendance. A
// participant without an attendance row reads as expected attendance.
func InstanceStudentByIDContext(tb testing.TB, ctx context.Context, db bun.IDB, id int64) *schedule.InstanceStudent {
	tb.Helper()
	rows := InstanceStudentsWhere(tb, ctx, db, `"instance_student".id = ?`, id)
	require.Len(tb, rows, 1, "instance student %d", id)
	return rows[0]
}

// InstanceStudentsWhere reads the composed participant DTOs matching a
// condition on the plan row (alias "instance_student") or its attendance
// (alias "attendance"), ordered by ID.
func InstanceStudentsWhere(tb testing.TB, ctx context.Context, db bun.IDB, condition string, args ...any) []*schedule.InstanceStudent {
	tb.Helper()
	var rows []*schedule.InstanceStudent
	require.NoError(tb, db.NewSelect().Model(&rows).ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Join(`LEFT JOIN active.activity_session_attendance AS attendance
			ON attendance.tenant_id = "instance_student".tenant_id AND attendance.instance_student_id = "instance_student".id`).
		ColumnExpr(`"instance_student".id, "instance_student".tenant_id, "instance_student".created_at,
			"instance_student".updated_at, "instance_student".instance_id, "instance_student".student_id,
			"instance_student".room_id, coalesce(attendance.status, 'expected') AS status, attendance.substatus,
			attendance.note, attendance.checked_in_at, attendance.checked_out_at,
			coalesce(attendance.is_unplanned, false) AS is_unplanned,
			coalesce(attendance.not_scheduled, false) AS not_scheduled, attendance.manual_status_at,
			attendance.student_status_day_id, attendance.pickup_exception_id`).
		Where(condition, args...).OrderExpr(`"instance_student".id`).Scan(ctx), "load instance students")
	return rows
}
