package test

import (
	"context"
	_ "embed"
	"strings"
	"testing"

	"github.com/uptrace/bun"
)

//go:embed testdata/presence_storage_before_contract.sql
var presenceStorageBeforeContract string

// RestorePresenceStorageBeforeContract rebuilds the rollback-only mirror the
// Contract (#2763) removed: the old execution columns of
// schedule.activity_instances and the old attendance columns of
// schedule.instance_students with their keys, indexes and checks, the mirror
// and routing triggers and the write counter, filled from the owner rows.
// Contracts written for the Cutover (#2762) need that world; production never
// reverses the Contract.
//
// It requires a per-test database (SetupIsolatedTestDB, or a package that
// opted into PerTestDatabases).
func RestorePresenceStorageBeforeContract(tb testing.TB, db *bun.DB) {
	tb.Helper()
	requireIsolatedPresenceRestore(tb, db, "restore presence storage before contract")
	restorePresenceMirror(tb, db)
}

func requireIsolatedPresenceRestore(tb testing.TB, db *bun.DB, action string) {
	tb.Helper()
	if db == nil {
		tb.Fatalf("%s: database is required", action)
	}
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatalf("%s requires this test's isolated database", action)
	}
}

func presenceStorageContracted(tb testing.TB, db *bun.DB) bool {
	tb.Helper()
	var contracted bool
	if err := db.NewRaw(`SELECT NOT EXISTS (SELECT 1 FROM pg_attribute
		WHERE attrelid = 'schedule.instance_students'::regclass AND attname = 'checked_in_at' AND NOT attisdropped)`).
		Scan(context.Background(), &contracted); err != nil {
		tb.Fatalf("inspect presence storage schema: %v", err)
	}
	return contracted
}

// restorePresenceMirror copies the owner rows onto the restored columns before
// the triggers exist, so the routing sees nothing to route and the counter
// stays untouched.
func restorePresenceMirror(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if !presenceStorageContracted(tb, db) {
		tb.Fatal("restore presence mirror requires the contracted schema")
	}
	schema, triggers, found := strings.Cut(presenceStorageBeforeContract, "CREATE OR REPLACE FUNCTION")
	if !found {
		tb.Fatal("restore presence mirror: frozen schema has no functions")
	}
	if err := db.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, schema); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE schedule.activity_instances AS instance SET
				status = session.status, active_group_id = session.active_group_id, started_by = session.started_by,
				started_at = session.started_at, completed_at = session.completed_at, completed_by = session.completed_by,
				reopen_until = session.reopen_until, completion_snapshot = session.completion_snapshot
			FROM active.activity_sessions AS session
			WHERE session.tenant_id = instance.tenant_id AND session.schedule_instance_id = instance.id;
			UPDATE schedule.instance_students AS participant SET
				status = attendance.status, substatus = attendance.substatus, note = attendance.note,
				checked_in_at = attendance.checked_in_at, checked_out_at = attendance.checked_out_at,
				is_unplanned = attendance.is_unplanned, not_scheduled = attendance.not_scheduled,
				manual_status_at = attendance.manual_status_at, student_status_day_id = attendance.student_status_day_id,
				pickup_exception_id = attendance.pickup_exception_id
			FROM active.activity_session_attendance AS attendance
			WHERE attendance.tenant_id = participant.tenant_id AND attendance.instance_student_id = participant.id;`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "CREATE OR REPLACE FUNCTION"+triggers)
		return err
	}); err != nil {
		tb.Fatalf("restore presence mirror: %v", err)
	}
}

// RestorePresenceStorageBeforeCutover turns the execution columns of
// schedule.activity_instances and the attendance columns of
// schedule.instance_students back into the authoritative storage they were
// before migration 1.15.415 (#2762).
//
// Contracts written for the Expand (#2718) and Backfill (#2761) migrations
// describe a world in which those columns hold the rows and the two Presence
// targets are empty or trail them. The cutover made active.activity_sessions
// and active.activity_session_attendance authoritative and left the old
// columns as a rollback-only mirror kept by triggers, so those tests restore
// their world inside their own disposable clone first: the mirror already
// holds every value, so dropping the triggers, the counter and the targets is
// the whole restore. Production rollback deliberately keeps the compatibility
// shape instead of reversing it.
//
// RestorePresenceStorageBeforeContract rebuilds the mirror first when the
// Contract (#2763) has already removed it.
//
// It requires a per-test database (SetupIsolatedTestDB, or a package that
// opted into PerTestDatabases).
func RestorePresenceStorageBeforeCutover(tb testing.TB, db *bun.DB) {
	tb.Helper()
	requireIsolatedPresenceRestore(tb, db, "restore presence storage before cutover")
	if presenceStorageContracted(tb, db) {
		restorePresenceMirror(tb, db)
	}
	var installed bool
	if err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_trigger
		WHERE tgname = 'activity_sessions_mirror' AND tgrelid = 'active.activity_sessions'::regclass)`).
		Scan(context.Background(), &installed); err != nil {
		tb.Fatalf("restore presence storage before cutover: inspect triggers: %v", err)
	}
	if !installed {
		tb.Fatal("restore presence storage before cutover: the compatibility triggers are already gone")
	}
	if _, err := db.ExecContext(context.Background(), `
		DROP TRIGGER activity_sessions_mirror ON active.activity_sessions;
		DROP TRIGGER activity_session_attendance_mirror ON active.activity_session_attendance;
		DROP TRIGGER activity_instances_route_compatibility ON schedule.activity_instances;
		DROP TRIGGER instance_students_route_compatibility ON schedule.instance_students;
		DROP FUNCTION active.mirror_activity_session();
		DROP FUNCTION active.mirror_activity_session_attendance();
		DROP FUNCTION schedule.route_activity_instance_compatibility();
		DROP FUNCTION schedule.route_instance_student_compatibility();
		DROP SEQUENCE active.presence_compatibility_writes;
		TRUNCATE active.activity_session_attendance, active.activity_sessions;
		DELETE FROM active.presence_backfill_batches;
		DELETE FROM active.presence_backfill_checkpoints;
		COMMENT ON TABLE active.activity_sessions IS
			'Presence-owned target. Empty during Expand #2718; schedule.activity_instances remains authoritative until #2762.';
		COMMENT ON TABLE active.activity_session_attendance IS
			'Presence-owned target. Empty during Expand #2718; schedule.instance_students remains authoritative until #2762.';
	`); err != nil {
		tb.Fatalf("restore presence storage before cutover: %v", err)
	}
}
