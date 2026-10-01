package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
)

// The presence Contract (#2763) follows the student (#2760), request-child
// (#2719), staff (#2754) and guardian (#2757) Contracts: deployment stops the
// application and verifies a complete release backup, preflight asks the
// integrity questions before downtime, and the same checks run again under
// locks right before the destructive DDL.
//
// It removes only the rollback shape the cutover (#2762) left behind: the old
// execution columns of schedule.activity_instances and the old attendance
// columns of schedule.instance_students with their keys and indexes, the two
// mirror triggers on the owner tables, the two routing triggers on the plan
// tables, their four functions and the write counter. The planning status
// keeps its name and is narrowed to the planning states: a block the mirror
// showed as active or completed is a planned occurrence whose session lives in
// active.activity_sessions, exactly as the backfill mapped it.
//
// The mirrored values are never copied over owner state. The Contract refuses
// while the mirror still holds an execution or an attendance the owner tables
// lack or contradict (mirror drift), because dropping the columns would lose
// it. Execution values a previous image left on a planned or cancelled block
// are outside that mapping: the backfill never carried them into a session,
// so they are not drift.

// presenceMirroredExecutionColumns are the old execution columns of
// schedule.activity_instances that active.activity_sessions owns.
const presenceMirroredExecutionColumns = `ARRAY['active_group_id', 'started_by', 'started_at', 'completed_at',
	'completed_by', 'reopen_until', 'completion_snapshot']::text[]`

// presenceMirroredAttendanceColumns are the old attendance columns of
// schedule.instance_students that active.activity_session_attendance owns.
const presenceMirroredAttendanceColumns = `ARRAY['status', 'substatus', 'note', 'checked_in_at', 'checked_out_at',
	'is_unplanned', 'not_scheduled', 'manual_status_at', 'student_status_day_id', 'pickup_exception_id']::text[]`

// presenceDistinctMirrorWords matches the mirrored column names no retained
// plan column shares. Status and note are left out: the retained planning
// status and unrelated note columns share them.
const presenceDistinctMirrorWords = `(^|[^[:alnum:]_])(active_group_id|started_by|started_at|completed_at|completed_by|reopen_until|completion_snapshot|substatus|checked_in_at|checked_out_at|is_unplanned|not_scheduled|manual_status_at|student_status_day_id|pickup_exception_id)([^[:alnum:]_]|$)`

// presenceContractChecksum hashes one row expression per school.
const (
	presenceContractChecksumOpen  = `encode(sha256(convert_to(string_agg(encode(sha256(convert_to((`
	presenceContractChecksumOrder = `)::text, 'UTF8')), 'hex'), '' ORDER BY (`
	presenceContractChecksumClose = `)::text), 'UTF8')), 'hex')`
	// Every retained plan column, with the planning status as the Contract
	// narrows it.
	presenceContractPlanRow = `(to_jsonb(r) - ` + presenceMirroredExecutionColumns +
		`) || jsonb_build_object('status', CASE WHEN r.status = 'cancelled' THEN 'cancelled' ELSE 'planned' END)`
	presenceContractParticipantRow = `to_jsonb(r) - ` + presenceMirroredAttendanceColumns
)

func presenceContractPrecondition(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return nil // The replay has not created the timetable schema yet.
	}
	if migrationPending(ctx, presenceCutoverVersion) {
		return fmt.Errorf("presence contract: the cutover %s is pending in the same release; it must be live in an earlier production release first", presenceCutoverVersion)
	}
	return presenceContractDataPreflight(ctx, db)
}

func presenceContractUp(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return contractPresenceStorageChecked(ctx, db, func(ctx context.Context, connection bun.IDB) error {
			var occupied bool
			err := connection.NewRaw(`SELECT EXISTS (SELECT 1 FROM schedule.activity_instances)
				OR EXISTS (SELECT 1 FROM schedule.instance_students)
				OR EXISTS (SELECT 1 FROM active.activity_sessions)
				OR EXISTS (SELECT 1 FROM active.activity_session_attendance)`).Scan(ctx, &occupied)
			if err != nil {
				return fmt.Errorf("presence contract: verify empty initial replay: %w", err)
			}
			if occupied {
				return errors.New("presence contract: initial replay contains timetable or presence rows")
			}
			return nil
		})
	}
	return contractPresenceStorageChecked(ctx, db, nil)
}

func presenceContractDown(context.Context, *bun.DB) error {
	return errors.New("presence Contract is irreversible: restore the verified pre-Contract backup and its prior application image together; automatic Down cannot recreate the removed rollback mirror")
}

func contractPresenceStorageChecked(ctx context.Context, db *bun.DB, check func(context.Context, bun.IDB) error) error {
	if db == nil {
		return errors.New("presence contract: database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// The cutover's lock order.
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			SET LOCAL TIME ZONE 'UTC';
			LOCK TABLE schedule.activity_instances, schedule.instance_students,
				active.activity_sessions, active.activity_session_attendance
				IN ACCESS EXCLUSIVE MODE;
		`); err != nil {
			return fmt.Errorf("presence contract: lock timetable and presence storage: %w", err)
		}
		if err := presenceContractDataPreflight(ctx, tx); err != nil {
			return err
		}
		if check != nil {
			if err := check(ctx, tx); err != nil {
				return err
			}
		}
		before, err := presenceContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		// Triggers first, so narrowing the planning status routes nothing.
		// RESTRICT is intentional. An unknown dependency must abort and roll
		// back even an earlier DROP, never be silently removed by CASCADE.
		// Dropping a column drops the indexes, checks and foreign keys on it.
		if _, err := tx.ExecContext(ctx, `
			DROP TRIGGER activity_sessions_mirror ON active.activity_sessions RESTRICT;
			DROP TRIGGER activity_session_attendance_mirror ON active.activity_session_attendance RESTRICT;
			DROP TRIGGER activity_instances_route_compatibility ON schedule.activity_instances RESTRICT;
			DROP TRIGGER instance_students_route_compatibility ON schedule.instance_students RESTRICT;
			DROP FUNCTION active.mirror_activity_session() RESTRICT;
			DROP FUNCTION active.mirror_activity_session_attendance() RESTRICT;
			DROP FUNCTION schedule.route_activity_instance_compatibility() RESTRICT;
			DROP FUNCTION schedule.route_instance_student_compatibility() RESTRICT;
			DROP SEQUENCE active.presence_compatibility_writes RESTRICT;
			UPDATE schedule.activity_instances SET status = 'planned' WHERE status IN ('active', 'completed');
			ALTER TABLE schedule.activity_instances
				DROP CONSTRAINT check_activity_instance_status,
				ADD CONSTRAINT check_activity_instance_status CHECK (status IN ('planned', 'cancelled')),
				DROP COLUMN active_group_id RESTRICT,
				DROP COLUMN started_by RESTRICT,
				DROP COLUMN started_at RESTRICT,
				DROP COLUMN completed_at RESTRICT,
				DROP COLUMN completed_by RESTRICT,
				DROP COLUMN reopen_until RESTRICT,
				DROP COLUMN completion_snapshot RESTRICT;
			ALTER TABLE schedule.instance_students
				DROP COLUMN status RESTRICT,
				DROP COLUMN substatus RESTRICT,
				DROP COLUMN note RESTRICT,
				DROP COLUMN checked_in_at RESTRICT,
				DROP COLUMN checked_out_at RESTRICT,
				DROP COLUMN is_unplanned RESTRICT,
				DROP COLUMN not_scheduled RESTRICT,
				DROP COLUMN manual_status_at RESTRICT,
				DROP COLUMN student_status_day_id RESTRICT,
				DROP COLUMN pickup_exception_id RESTRICT;
			COMMENT ON COLUMN schedule.activity_instances.status IS
				'Planning state written by Timetable (planned, cancelled). The execution of a block lives in active.activity_sessions (#2762, #2763).';
			COMMENT ON TABLE active.activity_sessions IS
				'Presence-owned execution of an activity instance (#2762). The only storage of its status, live group, actor, timestamps and completion snapshot since #2763.';
			COMMENT ON TABLE active.activity_session_attendance IS
				'Presence-owned attendance of a planned participant (#2762). A missing row means expected attendance. The only storage of the attendance since #2763.';
		`); err != nil {
			return fmt.Errorf("presence contract: remove the rollback mirror: %w", err)
		}
		after, err := presenceContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		if !slices.Equal(before, after) {
			return errors.New("presence contract: owner or plan rows or checksums changed during Contract")
		}
		return nil
	})
}

type presenceContractFingerprint struct {
	Object   string
	TenantID int64
	Rows     int64
	Checksum string
}

// Every owner column participates, and every retained plan column with the
// planning status as the Contract narrows it, so the comparison proves that
// only the mirror went away. Grouping by school prevents totals from masking
// tenant drift.
func presenceContractFingerprints(ctx context.Context, db bun.IDB) ([]presenceContractFingerprint, error) {
	var result []presenceContractFingerprint
	if err := db.NewRaw(`
		SELECT 'active.activity_sessions' AS object, tenant_id, count(*) AS rows,
			`+presenceContractChecksumOpen+`to_jsonb(r)`+presenceContractChecksumOrder+`to_jsonb(r)`+presenceContractChecksumClose+` AS checksum
		FROM active.activity_sessions r GROUP BY tenant_id
		UNION ALL
		SELECT 'active.activity_session_attendance', tenant_id, count(*),
			`+presenceContractChecksumOpen+`to_jsonb(r)`+presenceContractChecksumOrder+`to_jsonb(r)`+presenceContractChecksumClose+`
		FROM active.activity_session_attendance r GROUP BY tenant_id
		UNION ALL
		SELECT 'schedule.activity_instances', tenant_id, count(*),
			`+presenceContractChecksumOpen+presenceContractPlanRow+presenceContractChecksumOrder+presenceContractPlanRow+presenceContractChecksumClose+`
		FROM schedule.activity_instances r GROUP BY tenant_id
		UNION ALL
		SELECT 'schedule.instance_students', tenant_id, count(*),
			`+presenceContractChecksumOpen+presenceContractParticipantRow+presenceContractChecksumOrder+presenceContractParticipantRow+presenceContractChecksumClose+`
		FROM schedule.instance_students r GROUP BY tenant_id
		ORDER BY object, tenant_id
	`).Scan(ctx, &result); err != nil {
		return nil, fmt.Errorf("presence contract: fingerprint timetable and presence storage: %w", err)
	}
	return result, nil
}

// presenceContractDataPreflight reads the mirror but never changes rows or the
// counter. It runs again under the Contract lock: a pre-deployment observation
// alone cannot exclude writes between preflight and DDL.
func presenceContractDataPreflight(ctx context.Context, db bun.IDB) error {
	var ready bool
	if err := db.NewRaw(`SELECT
		to_regclass('active.presence_compatibility_writes') IS NOT NULL
		AND to_regprocedure('active.mirror_activity_session()') IS NOT NULL
		AND to_regprocedure('active.mirror_activity_session_attendance()') IS NOT NULL
		AND to_regprocedure('schedule.route_activity_instance_compatibility()') IS NOT NULL
		AND to_regprocedure('schedule.route_instance_student_compatibility()') IS NOT NULL
		AND (SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND (tgrelid, tgname) IN (
			('active.activity_sessions'::regclass, 'activity_sessions_mirror'),
			('active.activity_session_attendance'::regclass, 'activity_session_attendance_mirror'),
			('schedule.activity_instances'::regclass, 'activity_instances_route_compatibility'),
			('schedule.instance_students'::regclass, 'instance_students_route_compatibility'))) = 4
		AND (SELECT count(*) FROM pg_attribute WHERE attrelid = 'schedule.activity_instances'::regclass
			AND NOT attisdropped AND attname = ANY(`+presenceMirroredExecutionColumns+`)) = 7
		AND (SELECT count(*) FROM pg_attribute WHERE attrelid = 'schedule.instance_students'::regclass
			AND NOT attisdropped AND attname = ANY(`+presenceMirroredAttendanceColumns+`)) = 10`).Scan(ctx, &ready); err != nil {
		return fmt.Errorf("presence contract: inspect cutover schema: %w", err)
	}
	if !ready {
		return errors.New("presence contract: requires the completed cutover with its rollback mirror")
	}
	// The historical write counter is diagnostic, not a cleanup gate.
	// Deployment stops the old application before migration and restores the
	// complete release backup on failure; only current integrity matters here.
	//
	// Stored functions are not tracked as column dependencies. One that names
	// a plan table and a mirrored column would break on its next call.
	var functionName string
	if err := db.NewRaw(`SELECT coalesce((SELECT p.oid::regprocedure::text
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		AND replace(p.prosrc, '"', '') ~* '(^|[^[:alnum:]_])(activity_instances|instance_students)([^[:alnum:]_]|$)'
		AND p.prosrc ~* ?0
		AND p.oid NOT IN ('active.mirror_activity_session()'::regprocedure,
			'active.mirror_activity_session_attendance()'::regprocedure,
			'schedule.route_activity_instance_compatibility()'::regprocedure,
			'schedule.route_instance_student_compatibility()'::regprocedure)
		ORDER BY p.oid LIMIT 1), '')`, presenceDistinctMirrorWords).Scan(ctx, &functionName); err != nil {
		return fmt.Errorf("presence contract: inspect stored function callers: %w", err)
	}
	if functionName != "" {
		return fmt.Errorf("presence contract: stored function still references the rollback mirror: %s", functionName)
	}
	var counts [5]int64
	if err := db.NewRaw(`WITH tables(oid) AS (VALUES
			('schedule.activity_instances'::regclass), ('schedule.instance_students'::regclass),
			('active.activity_sessions'::regclass), ('active.activity_session_attendance'::regclass)),
		mirrored(relid, attnum) AS (
			SELECT attrelid, attnum FROM pg_attribute
			WHERE attrelid = 'schedule.activity_instances'::regclass AND NOT attisdropped
				AND attname = ANY(`+presenceMirroredExecutionColumns+`)
			UNION ALL
			SELECT attrelid, attnum FROM pg_attribute
			WHERE attrelid = 'schedule.instance_students'::regclass AND NOT attisdropped
				AND attname = ANY(`+presenceMirroredAttendanceColumns+`))
		SELECT
		(SELECT count(*) FROM pg_depend d JOIN mirrored m ON m.relid = d.refobjid AND m.attnum = d.refobjsubid
			WHERE d.classid = 'pg_rewrite'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND NOT convalidated
			AND (confrelid IN (SELECT oid FROM tables) OR conrelid IN (SELECT oid FROM tables))),
		(SELECT count(*) FROM pg_class
			WHERE oid IN (SELECT oid FROM tables) AND (NOT relrowsecurity OR NOT relforcerowsecurity)),
		(SELECT count(*)
			FROM schedule.activity_instances AS instance
			LEFT JOIN active.activity_sessions AS session
				ON session.tenant_id = instance.tenant_id AND session.schedule_instance_id = instance.id
			WHERE (session.id IS NULL AND instance.status IN ('active', 'completed'))
			OR (session.id IS NOT NULL AND ROW(instance.status, instance.active_group_id, instance.started_by,
					instance.started_at, instance.completed_at, instance.completed_by, instance.reopen_until,
					instance.completion_snapshot)
				IS DISTINCT FROM ROW(session.status, session.active_group_id, session.started_by,
					session.started_at, session.completed_at, session.completed_by, session.reopen_until,
					session.completion_snapshot))),
		(SELECT count(*)
			FROM schedule.instance_students AS participant
			LEFT JOIN active.activity_session_attendance AS attendance
				ON attendance.tenant_id = participant.tenant_id AND attendance.instance_student_id = participant.id
			WHERE ROW(participant.status, participant.substatus, participant.note, participant.checked_in_at,
					participant.checked_out_at, participant.is_unplanned, participant.not_scheduled,
					participant.manual_status_at, participant.student_status_day_id, participant.pickup_exception_id)
				IS DISTINCT FROM ROW(coalesce(attendance.status, 'expected'), attendance.substatus, attendance.note,
					attendance.checked_in_at, attendance.checked_out_at, coalesce(attendance.is_unplanned, false),
					coalesce(attendance.not_scheduled, false), attendance.manual_status_at,
					attendance.student_status_day_id, attendance.pickup_exception_id))
	`).Scan(ctx, &counts[0], &counts[1], &counts[2], &counts[3], &counts[4]); err != nil {
		return fmt.Errorf("presence contract: inspect storage integrity: %w", err)
	}
	for index, name := range []string{
		"views depend on the mirrored columns", "unvalidated foreign keys",
		"RLS disabled", "execution mirror drift", "attendance mirror drift",
	} {
		if counts[index] != 0 {
			return fmt.Errorf("presence contract: %s: %d", name, counts[index])
		}
	}
	return nil
}
