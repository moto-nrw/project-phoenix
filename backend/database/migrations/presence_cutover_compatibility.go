package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// installPresenceCompatibility runs inside the final-delta lock and
// transaction. It turns the old execution columns of schedule.activity_instances
// and the old attendance columns of schedule.instance_students into a
// rollback-only mirror of active.activity_sessions and
// active.activity_session_attendance, and routes the previous image's writes
// of those columns into the owner tables. The shape exists for previous-image
// rollback only: no current provider reads or writes the mirrored columns, and
// #2763 removes them after the rollback window.
//
// The old names stay base tables on purpose. The previous image materializes
// occurrences and books participants with INSERT ... ON CONFLICT, and
// PostgreSQL refuses ON CONFLICT through a view with INSTEAD OF triggers
// (there is no unique constraint on the view to match), so republishing the
// old names as views would break exactly the rollback the shape exists for.
func installPresenceCompatibility(ctx context.Context, tx bun.Tx) error {
	if _, err := tx.ExecContext(ctx, presenceCompatibilityCounters); err != nil {
		return fmt.Errorf("install presence compatibility counters: %w", err)
	}
	if _, err := tx.ExecContext(ctx, presenceSessionMirror); err != nil {
		return fmt.Errorf("install presence session mirror: %w", err)
	}
	if _, err := tx.ExecContext(ctx, presenceAttendanceMirror); err != nil {
		return fmt.Errorf("install presence attendance mirror: %w", err)
	}
	if _, err := tx.ExecContext(ctx, presenceSessionRouting); err != nil {
		return fmt.Errorf("install presence session routing: %w", err)
	}
	if _, err := tx.ExecContext(ctx, presenceAttendanceRouting); err != nil {
		return fmt.Errorf("install presence attendance routing: %w", err)
	}
	if _, err := tx.ExecContext(ctx, presenceCompatibilityComments); err != nil {
		return fmt.Errorf("install presence compatibility comments: %w", err)
	}
	return nil
}

// presenceCompatibilityCounters counts the rows the previous image routes into
// the owner tables. Reads of the mirrored columns cannot be counted on a base
// table; a previous image is observed through the writes it routes and
// through the deployment itself.
const presenceCompatibilityCounters = `
	CREATE SEQUENCE active.presence_compatibility_writes;
	COMMENT ON SEQUENCE active.presence_compatibility_writes IS
		'Rows the previous image wrote into the mirrored execution or attendance columns and the routing triggers copied into active.activity_sessions or active.activity_session_attendance (#2762). Must trend to zero before #2763.';
	GRANT USAGE ON SEQUENCE active.presence_compatibility_writes TO phoenix_tenant;
	GRANT USAGE, SELECT ON SEQUENCE active.presence_compatibility_writes TO phoenix_admin;`

// presenceSessionMirror copies every owner write of a session onto the old
// execution columns of its occurrence. An equality guard keeps the copy
// idempotent: the routing trigger below sees no difference afterwards and
// stops, which is what keeps the two triggers from calling each other
// forever. Deleting a session returns a running or completed occurrence to
// the planned state; a cancelled occurrence keeps the values the cancel left,
// as the previous image left them.
const presenceSessionMirror = `
	CREATE OR REPLACE FUNCTION active.mirror_activity_session()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF TG_OP = 'DELETE' THEN
			UPDATE schedule.activity_instances SET
				status = 'planned', active_group_id = NULL, started_by = NULL, started_at = NULL,
				completed_at = NULL, completed_by = NULL, reopen_until = NULL, completion_snapshot = NULL
			WHERE tenant_id = OLD.tenant_id AND id = OLD.schedule_instance_id
			  AND status IN ('active', 'completed');
			RETURN OLD;
		END IF;
		UPDATE schedule.activity_instances SET
			status = NEW.status, active_group_id = NEW.active_group_id, started_by = NEW.started_by,
			started_at = NEW.started_at, completed_at = NEW.completed_at, completed_by = NEW.completed_by,
			reopen_until = NEW.reopen_until, completion_snapshot = NEW.completion_snapshot
		WHERE tenant_id = NEW.tenant_id AND id = NEW.schedule_instance_id
		  AND ROW(status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot)
		      IS DISTINCT FROM ROW(NEW.status, NEW.active_group_id, NEW.started_by, NEW.started_at, NEW.completed_at, NEW.completed_by, NEW.reopen_until, NEW.completion_snapshot);
		RETURN NEW;
	END
	$function$;
	CREATE TRIGGER activity_sessions_mirror
		AFTER INSERT OR UPDATE OR DELETE ON active.activity_sessions
		FOR EACH ROW EXECUTE FUNCTION active.mirror_activity_session();`

// presenceAttendanceMirror copies every owner write of an attendance row onto
// the old attendance columns of its participant. A missing attendance row
// means expected attendance, so deleting one resets the columns to their
// defaults. updated_at follows the attendance row, as the previous image
// bumped it on every attendance change.
const presenceAttendanceMirror = `
	CREATE OR REPLACE FUNCTION active.mirror_activity_session_attendance()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	BEGIN
		IF TG_OP = 'DELETE' THEN
			UPDATE schedule.instance_students SET
				status = 'expected', substatus = NULL, note = NULL, checked_in_at = NULL, checked_out_at = NULL,
				is_unplanned = false, not_scheduled = false, manual_status_at = NULL,
				student_status_day_id = NULL, pickup_exception_id = NULL, updated_at = now()
			WHERE tenant_id = OLD.tenant_id AND id = OLD.instance_student_id
			  AND ROW(status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id)
			      IS DISTINCT FROM ROW('expected', NULL::text, NULL::text, NULL::timestamptz, NULL::timestamptz, false, false, NULL::timestamptz, NULL::bigint, NULL::bigint);
			RETURN OLD;
		END IF;
		UPDATE schedule.instance_students SET
			status = NEW.status, substatus = NEW.substatus, note = NEW.note, checked_in_at = NEW.checked_in_at,
			checked_out_at = NEW.checked_out_at, is_unplanned = NEW.is_unplanned, not_scheduled = NEW.not_scheduled,
			manual_status_at = NEW.manual_status_at, student_status_day_id = NEW.student_status_day_id,
			pickup_exception_id = NEW.pickup_exception_id, updated_at = NEW.updated_at
		WHERE tenant_id = NEW.tenant_id AND id = NEW.instance_student_id
		  AND ROW(status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id)
		      IS DISTINCT FROM ROW(NEW.status, NEW.substatus, NEW.note, NEW.checked_in_at, NEW.checked_out_at, NEW.is_unplanned, NEW.not_scheduled, NEW.manual_status_at, NEW.student_status_day_id, NEW.pickup_exception_id);
		RETURN NEW;
	END
	$function$;
	CREATE TRIGGER activity_session_attendance_mirror
		AFTER INSERT OR UPDATE OR DELETE ON active.activity_session_attendance
		FOR EACH ROW EXECUTE FUNCTION active.mirror_activity_session_attendance();`

// presenceSessionRouting sends a previous-image write of the old execution
// columns to the session that owns them. A write that already matches the
// session is the mirror's own copy and routes nothing. An occurrence written
// as active or completed gets its session upserted; one written back to
// planned or cancelled loses it, as the backfill mapping defined. Current
// providers end a session before they change the planning status, so their
// writes never reach the delete branch.
const presenceSessionRouting = `
	CREATE OR REPLACE FUNCTION schedule.route_activity_instance_compatibility()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	DECLARE
		session active.activity_sessions%ROWTYPE;
	BEGIN
		SELECT * INTO session FROM active.activity_sessions AS s
		WHERE s.tenant_id = NEW.tenant_id AND s.schedule_instance_id = NEW.id;
		IF NEW.status IN ('active', 'completed') THEN
			IF FOUND AND ROW(session.status, session.active_group_id, session.started_by, session.started_at, session.completed_at, session.completed_by, session.reopen_until, session.completion_snapshot)
			   IS NOT DISTINCT FROM ROW(NEW.status, NEW.active_group_id, NEW.started_by, NEW.started_at, NEW.completed_at, NEW.completed_by, NEW.reopen_until, NEW.completion_snapshot) THEN
				RETURN NULL;
			END IF;
			PERFORM nextval('active.presence_compatibility_writes');
			INSERT INTO active.activity_sessions (tenant_id, schedule_instance_id, status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot)
			VALUES (NEW.tenant_id, NEW.id, NEW.status, NEW.active_group_id, NEW.started_by, NEW.started_at, NEW.completed_at, NEW.completed_by, NEW.reopen_until, NEW.completion_snapshot)
			ON CONFLICT (tenant_id, schedule_instance_id) DO UPDATE SET
				status = EXCLUDED.status, active_group_id = EXCLUDED.active_group_id, started_by = EXCLUDED.started_by,
				started_at = EXCLUDED.started_at, completed_at = EXCLUDED.completed_at, completed_by = EXCLUDED.completed_by,
				reopen_until = EXCLUDED.reopen_until, completion_snapshot = EXCLUDED.completion_snapshot, updated_at = now();
		ELSIF FOUND THEN
			PERFORM nextval('active.presence_compatibility_writes');
			DELETE FROM active.activity_sessions AS s WHERE s.tenant_id = NEW.tenant_id AND s.schedule_instance_id = NEW.id;
		END IF;
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER activity_instances_route_compatibility
		AFTER INSERT OR UPDATE OF status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot
		ON schedule.activity_instances
		FOR EACH ROW EXECUTE FUNCTION schedule.route_activity_instance_compatibility();`

// presenceAttendanceRouting sends a previous-image write of the old attendance
// columns to the attendance row that owns them. A participant whose columns
// still read as expected attendance needs no row; one that differs gets its
// row inserted or updated. A write that already matches the row is the
// mirror's own copy and routes nothing.
const presenceAttendanceRouting = `
	CREATE OR REPLACE FUNCTION schedule.route_instance_student_compatibility()
	RETURNS trigger LANGUAGE plpgsql SECURITY INVOKER SET search_path = pg_catalog AS $function$
	DECLARE
		attendance active.activity_session_attendance%ROWTYPE;
	BEGIN
		SELECT * INTO attendance FROM active.activity_session_attendance AS a
		WHERE a.tenant_id = NEW.tenant_id AND a.instance_student_id = NEW.id;
		IF FOUND THEN
			IF ROW(attendance.status, attendance.substatus, attendance.note, attendance.checked_in_at, attendance.checked_out_at, attendance.is_unplanned, attendance.not_scheduled, attendance.manual_status_at, attendance.student_status_day_id, attendance.pickup_exception_id)
			   IS NOT DISTINCT FROM ROW(NEW.status, NEW.substatus, NEW.note, NEW.checked_in_at, NEW.checked_out_at, NEW.is_unplanned, NEW.not_scheduled, NEW.manual_status_at, NEW.student_status_day_id, NEW.pickup_exception_id) THEN
				RETURN NULL;
			END IF;
			PERFORM nextval('active.presence_compatibility_writes');
			UPDATE active.activity_session_attendance AS a SET
				status = NEW.status, substatus = NEW.substatus, note = NEW.note, checked_in_at = NEW.checked_in_at,
				checked_out_at = NEW.checked_out_at, is_unplanned = NEW.is_unplanned, not_scheduled = NEW.not_scheduled,
				manual_status_at = NEW.manual_status_at, student_status_day_id = NEW.student_status_day_id,
				pickup_exception_id = NEW.pickup_exception_id, updated_at = NEW.updated_at
			WHERE a.tenant_id = NEW.tenant_id AND a.instance_student_id = NEW.id;
		ELSIF ROW(NEW.status, NEW.substatus, NEW.note, NEW.checked_in_at, NEW.checked_out_at, NEW.is_unplanned, NEW.not_scheduled, NEW.manual_status_at, NEW.student_status_day_id, NEW.pickup_exception_id)
		      IS DISTINCT FROM ROW('expected', NULL::text, NULL::text, NULL::timestamptz, NULL::timestamptz, false, false, NULL::timestamptz, NULL::bigint, NULL::bigint) THEN
			PERFORM nextval('active.presence_compatibility_writes');
			INSERT INTO active.activity_session_attendance (tenant_id, instance_student_id, status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id, created_at, updated_at)
			VALUES (NEW.tenant_id, NEW.id, NEW.status, NEW.substatus, NEW.note, NEW.checked_in_at, NEW.checked_out_at, NEW.is_unplanned, NEW.not_scheduled, NEW.manual_status_at, NEW.student_status_day_id, NEW.pickup_exception_id, NEW.created_at, NEW.updated_at);
		END IF;
		RETURN NULL;
	END
	$function$;
	CREATE TRIGGER instance_students_route_compatibility
		AFTER INSERT OR UPDATE OF status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id
		ON schedule.instance_students
		FOR EACH ROW EXECUTE FUNCTION schedule.route_instance_student_compatibility();`

// presenceCompatibilityComments name the mirrored columns for the operator
// who inspects the tables during the rollback window.
const presenceCompatibilityComments = `
	COMMENT ON TABLE active.activity_sessions IS
		'Presence-owned execution of an activity instance (#2762). schedule.activity_instances.status/active_group_id/started_by/started_at/completed_at/completed_by/reopen_until/completion_snapshot are a rollback-only mirror of this table until #2763.';
	COMMENT ON TABLE active.activity_session_attendance IS
		'Presence-owned attendance of a planned participant (#2762). A missing row means expected attendance. The attendance columns of schedule.instance_students are a rollback-only mirror of this table until #2763.';
	COMMENT ON COLUMN schedule.activity_instances.status IS
		'Planning state written by Timetable (planned, cancelled). active and completed are the rollback-only mirror of active.activity_sessions.status (#2762).';
	COMMENT ON COLUMN schedule.activity_instances.active_group_id IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.activity_instances.started_by IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.activity_instances.started_at IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.activity_instances.completed_at IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.activity_instances.completed_by IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.activity_instances.reopen_until IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.activity_instances.completion_snapshot IS 'Rollback-only mirror of active.activity_sessions (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.status IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.substatus IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.note IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.checked_in_at IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.checked_out_at IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.is_unplanned IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.not_scheduled IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.manual_status_at IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.student_status_day_id IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
	COMMENT ON COLUMN schedule.instance_students.pickup_exception_id IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';`
