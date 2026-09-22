package test

import (
	"context"
	"testing"

	"github.com/uptrace/bun"
)

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
// It requires a per-test database (SetupIsolatedTestDB, or a package that
// opted into PerTestDatabases).
func RestorePresenceStorageBeforeCutover(tb testing.TB, db *bun.DB) {
	tb.Helper()
	if db == nil {
		tb.Fatal("restore presence storage before cutover: database is required")
	}
	entry, isolated := isolatedTestDatabases.Load(topLevelTestName(tb))
	if !isolated || entry.(*isolatedTestDatabase).db != db {
		tb.Fatal("restore presence storage before cutover requires this test's isolated database")
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
