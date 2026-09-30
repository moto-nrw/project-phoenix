-- Historical test-only rollback mirror schema at migration 1.15.431.
-- Captured from a disposable local migration database with PostgreSQL pg_dump
-- (schema-only, schedule.activity_instances, schedule.instance_students and
-- the compatibility counter) and pg_get_functiondef/pg_get_triggerdef for the
-- mirror and routing functions and triggers. Only the objects the presence
-- Contract (#2763) removed are listed; the retained planning columns, keys,
-- RLS and grants are untouched. No data included. Session settings, owners and
-- psql directives removed.
ALTER TABLE schedule.activity_instances
    DROP CONSTRAINT check_activity_instance_status,
    ADD CONSTRAINT check_activity_instance_status CHECK ((status = ANY (ARRAY['planned'::text, 'active'::text, 'completed'::text, 'cancelled'::text]))),
    ADD COLUMN active_group_id bigint,
    ADD COLUMN started_by bigint,
    ADD COLUMN started_at timestamp with time zone,
    ADD COLUMN completed_at timestamp with time zone,
    ADD COLUMN completed_by bigint,
    ADD COLUMN reopen_until timestamp with time zone,
    ADD COLUMN completion_snapshot jsonb;
ALTER TABLE schedule.instance_students
    ADD COLUMN status text DEFAULT 'expected'::text NOT NULL,
    ADD COLUMN substatus text,
    ADD COLUMN note text,
    ADD COLUMN checked_in_at timestamp with time zone,
    ADD COLUMN checked_out_at timestamp with time zone,
    ADD COLUMN is_unplanned boolean DEFAULT false NOT NULL,
    ADD COLUMN student_status_day_id bigint,
    ADD COLUMN not_scheduled boolean DEFAULT false NOT NULL,
    ADD COLUMN manual_status_at timestamp with time zone,
    ADD COLUMN pickup_exception_id bigint,
    ADD CONSTRAINT check_instance_student_note_length CHECK (((note IS NULL) OR (char_length(note) <= 500))),
    ADD CONSTRAINT check_instance_student_status CHECK ((status = ANY (ARRAY['expected'::text, 'present'::text, 'absent'::text]))),
    ADD CONSTRAINT check_instance_student_substatus CHECK (((substatus IS NULL) OR (substatus = ANY (ARRAY['late'::text, 'excused'::text, 'sick'::text, 'field_trip'::text, 'other'::text]))));
COMMENT ON COLUMN schedule.activity_instances.status IS 'Planning state written by Timetable (planned, cancelled). active and completed are the rollback-only mirror of active.activity_sessions.status (#2762).';
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
COMMENT ON COLUMN schedule.instance_students.student_status_day_id IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
COMMENT ON COLUMN schedule.instance_students.not_scheduled IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
COMMENT ON COLUMN schedule.instance_students.manual_status_at IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
COMMENT ON COLUMN schedule.instance_students.pickup_exception_id IS 'Rollback-only mirror of active.activity_session_attendance (#2762); nothing current reads or writes it.';
COMMENT ON TABLE active.activity_sessions IS 'Presence-owned execution of an activity instance (#2762). schedule.activity_instances.status/active_group_id/started_by/started_at/completed_at/completed_by/reopen_until/completion_snapshot are a rollback-only mirror of this table until #2763.';
COMMENT ON TABLE active.activity_session_attendance IS 'Presence-owned attendance of a planned participant (#2762). A missing row means expected attendance. The attendance columns of schedule.instance_students are a rollback-only mirror of this table until #2763.';
CREATE UNIQUE INDEX idx_activity_instances_active_group_unique ON schedule.activity_instances USING btree (active_group_id) WHERE (active_group_id IS NOT NULL);
CREATE INDEX idx_activity_instances_reopenable ON schedule.activity_instances USING btree (tenant_id, reopen_until) WHERE ((status = 'completed'::text) AND (reopen_until IS NOT NULL));
CREATE INDEX idx_instance_students_pickup_exception ON schedule.instance_students USING btree (pickup_exception_id) WHERE (pickup_exception_id IS NOT NULL);
CREATE INDEX idx_instance_students_status ON schedule.instance_students USING btree (instance_id, status);
CREATE INDEX idx_instance_students_status_day ON schedule.instance_students USING btree (student_status_day_id) WHERE (student_status_day_id IS NOT NULL);
ALTER TABLE ONLY schedule.activity_instances
    ADD CONSTRAINT activity_instances_completed_by_fkey FOREIGN KEY (completed_by) REFERENCES auth.accounts(id) ON DELETE SET NULL;
ALTER TABLE ONLY schedule.activity_instances
    ADD CONSTRAINT fk_activity_instances_active_group_tenant FOREIGN KEY (tenant_id, active_group_id) REFERENCES active.groups(tenant_id, id) ON DELETE SET NULL (active_group_id);
ALTER TABLE ONLY schedule.activity_instances
    ADD CONSTRAINT fk_activity_instances_started_by_tenant FOREIGN KEY (tenant_id, started_by) REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (started_by);
ALTER TABLE ONLY schedule.instance_students
    ADD CONSTRAINT fk_instance_students_pickup_exception FOREIGN KEY (tenant_id, pickup_exception_id) REFERENCES schedule.student_pickup_exceptions(tenant_id, id) ON DELETE SET NULL (pickup_exception_id);
ALTER TABLE ONLY schedule.instance_students
    ADD CONSTRAINT fk_instance_students_status_day FOREIGN KEY (tenant_id, student_status_day_id) REFERENCES active.student_status_days(tenant_id, id) ON DELETE SET NULL (student_status_day_id);
CREATE SEQUENCE active.presence_compatibility_writes
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
COMMENT ON SEQUENCE active.presence_compatibility_writes IS 'Rows the previous image wrote into the mirrored execution or attendance columns and the routing triggers copied into active.activity_sessions or active.activity_session_attendance (#2762). Must trend to zero before #2763.';
GRANT USAGE ON SEQUENCE active.presence_compatibility_writes TO phoenix_tenant;
GRANT ALL ON SEQUENCE active.presence_compatibility_writes TO phoenix_admin;
CREATE OR REPLACE FUNCTION active.mirror_activity_session()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
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
	$function$
;
CREATE OR REPLACE FUNCTION active.mirror_activity_session_attendance()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
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
	$function$
;
CREATE OR REPLACE FUNCTION schedule.route_activity_instance_compatibility()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
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
	$function$
;
CREATE OR REPLACE FUNCTION schedule.route_instance_student_compatibility()
 RETURNS trigger
 LANGUAGE plpgsql
 SET search_path TO 'pg_catalog'
AS $function$
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
	$function$
;
CREATE TRIGGER activity_sessions_mirror AFTER INSERT OR DELETE OR UPDATE ON active.activity_sessions FOR EACH ROW EXECUTE FUNCTION active.mirror_activity_session();
CREATE TRIGGER activity_session_attendance_mirror AFTER INSERT OR DELETE OR UPDATE ON active.activity_session_attendance FOR EACH ROW EXECUTE FUNCTION active.mirror_activity_session_attendance();
CREATE TRIGGER activity_instances_route_compatibility AFTER INSERT OR UPDATE OF status, active_group_id, started_by, started_at, completed_at, completed_by, reopen_until, completion_snapshot ON schedule.activity_instances FOR EACH ROW EXECUTE FUNCTION schedule.route_activity_instance_compatibility();
CREATE TRIGGER instance_students_route_compatibility AFTER INSERT OR UPDATE OF status, substatus, note, checked_in_at, checked_out_at, is_unplanned, not_scheduled, manual_status_at, student_status_day_id, pickup_exception_id ON schedule.instance_students FOR EACH ROW EXECUTE FUNCTION schedule.route_instance_student_compatibility();
