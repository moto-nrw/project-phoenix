package migrations

// Like presence_expand_test.go, these registered migration/runner tests stay
// internal: architecture policy forbids external tests importing migrations.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func finishPresenceBackfill(t *testing.T, db *testpkg.DB, tenantID int64) PresenceBackfillReport {
	t.Helper()
	for batch := 0; batch < 40; batch++ {
		report, err := PresenceBackfillBatch(t.Context(), db, tenantID, 1)
		require.NoError(t, err)
		if report.Complete {
			return report
		}
	}
	t.Fatal("backfill did not converge")
	return PresenceBackfillReport{}
}

func TestPresenceBackfillResumesAtEveryBoundary(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	var before string
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(i)::text FROM schedule.activity_instances i WHERE id = ?`, f.instance).Scan(t.Context(), &before))

	var report PresenceBackfillReport
	for boundary := 0; boundary < 10; boundary++ {
		var err error
		report, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
		require.NoError(t, err)
		persisted, err := PresenceBackfillStatus(t.Context(), db, f.tenant)
		require.NoError(t, err)
		require.Equal(t, report, persisted, "restart must recover the committed checkpoint")
		if report.Complete {
			break
		}
	}
	require.True(t, report.Complete)
	require.Zero(t, report.Mismatches)
	require.EqualValues(t, 1, report.Sessions.SourceCount)
	require.EqualValues(t, 1, report.Sessions.TargetCount)
	require.Equal(t, report.Sessions.SourceChecksum, report.Sessions.TargetChecksum)
	require.EqualValues(t, 1, report.Attendance.SourceCount)
	require.Equal(t, report.Attendance.SourceChecksum, report.Attendance.TargetChecksum)
	var after, status string
	require.NoError(t, db.NewRaw(`SELECT to_jsonb(i)::text FROM schedule.activity_instances i WHERE id = ?`, f.instance).Scan(t.Context(), &after))
	require.Equal(t, before, after, "planned and legacy execution fields remain authoritative and untouched")
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, f.tenant, f.instance).Scan(t.Context(), &status))
	require.Equal(t, "active", status)

	copied := report.RowsCopied
	for boundary := 0; boundary < 10; boundary++ {
		var err error
		report, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
		require.NoError(t, err)
		if report.Complete {
			break
		}
	}
	require.True(t, report.Complete)
	require.Equal(t, copied, report.RowsCopied, "replaying completed batches must skip identical rows")
	require.Positive(t, report.RowsSkipped)
}

func TestPresenceBackfillRetriesTransactionFailures(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"40001", "40P01"} {
		t.Run(state, func(t *testing.T) {
			db := testpkg.SetupIsolatedTestDB(t)
			f := createPresenceExpandFixture(t, db)
			// Sequences survive rollback, making exactly the first attempt fail at
			// PostgreSQL's write boundary, not in a mock of the retry implementation.
			_, err := db.ExecContext(t.Context(), `CREATE SEQUENCE active.backfill_fault;
    CREATE FUNCTION active.backfill_fault() RETURNS trigger LANGUAGE plpgsql AS $$
    BEGIN
     IF nextval('active.backfill_fault') = 1 THEN
      RAISE EXCEPTION 'injected transaction failure' USING ERRCODE = TG_ARGV[0];
     END IF;
     RETURN NEW;
    END $$`)
			require.NoError(t, err)
			_, err = db.ExecContext(t.Context(), `CREATE TRIGGER backfill_fault BEFORE INSERT ON active.activity_sessions
    FOR EACH ROW EXECUTE FUNCTION active.backfill_fault(?)`, state)
			require.NoError(t, err)
			report, err := PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
			require.NoError(t, err)
			require.EqualValues(t, 1, report.RowsCopied)
			metrics, err := PresenceBackfillMetrics(t.Context(), db, f.tenant)
			require.NoError(t, err)
			require.EqualValues(t, 1, metrics.Retries)
			require.EqualValues(t, 1, metrics.Batches)
			if state == "40P01" {
				require.EqualValues(t, 1, metrics.Deadlocks)
			}
		})
	}
}

func TestPresenceBackfillReportsTerminalDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION active.always_deadlock() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN RAISE EXCEPTION 'injected deadlock' USING ERRCODE = '40P01'; END $$;
  CREATE TRIGGER always_deadlock BEFORE INSERT ON active.activity_sessions
  FOR EACH ROW EXECUTE FUNCTION active.always_deadlock()`)
	require.NoError(t, err)
	_, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	require.ErrorContains(t, err, "retries=5 deadlocks=6")
	state, err := PresenceBackfillStatus(t.Context(), db, f.tenant)
	require.NoError(t, err)
	require.Equal(t, "not_started", state.Phase)
}

func TestPresenceBackfillReconcilesChangesBehindHighWater(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	account := testpkg.CreateTestAccount(t, db, "presence-backfill-completion@example.test")
	first, err := PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	require.NoError(t, err)
	require.Equal(t, f.instance, first.SessionHighWater)
	// Old writers keep operating, including ones that do not bump updated_at.
	_, err = db.ExecContext(t.Context(), `UPDATE schedule.activity_instances SET status = 'completed',
  started_by = ?, started_at = '2026-09-09 10:00:00+02', completed_at = '2026-09-09 11:00:00+02',
  reopen_until = '2026-09-09 11:05:00+02', completion_snapshot = '{"visit_ids":[],"attendance":[]}', completed_by = ?
  WHERE tenant_id = ? AND id = ?`, f.staff, account.ID, f.tenant, f.instance)
	require.NoError(t, err)
	for boundary := 0; boundary < 5; boundary++ {
		report, err := PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
		require.NoError(t, err)
		if report.AttendanceHighWater == f.participant {
			break
		}
	}
	_, err = db.ExecContext(t.Context(), `UPDATE schedule.instance_students SET status = 'absent', substatus = 'excused',
  note = 'Früher abgeholt', checked_in_at = '2026-09-09 10:15:00+02', checked_out_at = '2026-09-09 10:45:00+02',
  is_unplanned = true, not_scheduled = true, manual_status_at = '2026-09-09 10:46:00+02',
  student_status_day_id = ?, pickup_exception_id = ? WHERE tenant_id = ? AND id = ?`, f.statusDay, f.pickup, f.tenant, f.participant)
	require.NoError(t, err)
	var mismatch PresenceBackfillReport
	for boundary := 0; boundary < 4; boundary++ {
		mismatch, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
		require.NoError(t, err)
		if mismatch.Mismatches != 0 {
			break
		}
	}
	require.EqualValues(t, 2, mismatch.Mismatches)
	require.False(t, mismatch.Complete)
	require.EqualValues(t, 0, mismatch.SessionHighWater)
	final := finishPresenceBackfill(t, db, f.tenant)
	require.Zero(t, final.Mismatches)
	require.NotEmpty(t, final.FinalDeltaSnapshot)
	require.NotNil(t, final.VerifiedAt)
	require.EqualValues(t, 1, final.PlannedOccurrences)
	require.EqualValues(t, 1, final.PlannedAssignments)

	for _, tc := range []struct {
		source, target string
		sourceID       int64
		columns        []string
	}{
		{"schedule.activity_instances", "active.activity_sessions", f.instance,
			[]string{"status", "active_group_id", "started_by", "started_at", "completed_at", "completed_by", "reopen_until", "completion_snapshot", "created_at", "updated_at"}},
		{"schedule.instance_students", "active.activity_session_attendance", f.participant,
			[]string{"status", "substatus", "note", "checked_in_at", "checked_out_at", "is_unplanned", "not_scheduled", "manual_status_at", "student_status_day_id", "pickup_exception_id", "created_at", "updated_at"}},
	} {
		var oldJSON, newJSON string
		require.NoError(t, db.NewRaw("SELECT to_jsonb(source)::text FROM "+tc.source+" source WHERE id = ?", tc.sourceID).Scan(t.Context(), &oldJSON))
		require.NoError(t, db.NewRaw("SELECT to_jsonb(target)::text FROM "+tc.target+" target WHERE tenant_id = ?", f.tenant).Scan(t.Context(), &newJSON))
		var oldFields, newFields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(oldJSON), &oldFields))
		require.NoError(t, json.Unmarshal([]byte(newJSON), &newFields))
		for _, column := range tc.columns {
			require.JSONEq(t, string(oldFields[column]), string(newFields[column]), column)
		}
	}
	_, err = db.ExecContext(t.Context(), `UPDATE schedule.instance_students SET status = 'expected', substatus = NULL,
  note = NULL, checked_in_at = NULL, checked_out_at = NULL, is_unplanned = false, not_scheduled = false,
  manual_status_at = NULL, student_status_day_id = NULL, pickup_exception_id = NULL WHERE id = ?`, f.participant)
	require.NoError(t, err)
	final = finishPresenceBackfill(t, db, f.tenant)
	require.Zero(t, final.Mismatches, "reruns must also clear formerly non-NULL target fields")
	var noteIsNull bool
	require.NoError(t, db.NewRaw(`SELECT note IS NULL FROM active.activity_session_attendance WHERE tenant_id = ?`, f.tenant).Scan(t.Context(), &noteIsNull))
	require.True(t, noteIsNull)
	// A source returning to planning-only state must not leave an actual session.
	_, err = db.ExecContext(t.Context(), `UPDATE schedule.activity_instances SET status = 'cancelled' WHERE id = ?`, f.instance)
	require.NoError(t, err)
	final = finishPresenceBackfill(t, db, f.tenant)
	require.Zero(t, final.Sessions.TargetCount)
	require.EqualValues(t, 1, final.Attendance.TargetCount, "planned assignments retain their actual status even without a session")
	require.EqualValues(t, 1, final.RowsRemoved)
}

func TestPresenceBackfillRollbackOnlyClearsSelectedTenantTargets(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	a, b := createPresenceExpandFixture(t, db), createPresenceExpandFixture(t, db)
	finishPresenceBackfill(t, db, a.tenant)
	bBefore := finishPresenceBackfill(t, db, b.tenant)
	require.NoError(t, RestartPresenceBackfill(t.Context(), db, a.tenant))
	state, err := PresenceBackfillStatus(t.Context(), db, a.tenant)
	require.NoError(t, err)
	require.False(t, state.Complete)
	require.Zero(t, state.SessionHighWater)
	require.Zero(t, state.AttendanceHighWater)
	var oldRows, targetRows int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM schedule.activity_instances WHERE tenant_id = ?)
  + (SELECT count(*) FROM schedule.instance_students WHERE tenant_id = ?)`, a.tenant, a.tenant).Scan(t.Context(), &oldRows))
	require.Equal(t, 2, oldRows)
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM active.activity_sessions WHERE tenant_id = ?)
  + (SELECT count(*) FROM active.activity_session_attendance WHERE tenant_id = ?)`, a.tenant, a.tenant).Scan(t.Context(), &targetRows))
	require.Zero(t, targetRows)
	bAfter, err := PresenceBackfillStatus(t.Context(), db, b.tenant)
	require.NoError(t, err)
	require.Equal(t, bBefore, bAfter)
	finishPresenceBackfill(t, db, a.tenant)
}

func TestPresenceBackfillFailureRollsBackOnlyCurrentBatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	_, err := PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	require.NoError(t, err)
	before, err := PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	require.NoError(t, err)
	require.Equal(t, "attendance", before.Phase)
	_, err = db.ExecContext(t.Context(), `CREATE FUNCTION active.reject_backfill() RETURNS trigger LANGUAGE plpgsql AS $$
  BEGIN RAISE EXCEPTION 'reject completed write' USING ERRCODE = '23514'; END $$;
  CREATE TRIGGER reject_backfill AFTER INSERT ON active.activity_session_attendance
  FOR EACH ROW EXECUTE FUNCTION active.reject_backfill()`)
	require.NoError(t, err)
	_, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	requirePresenceSQLState(t, err, "23514")
	after, err := PresenceBackfillStatus(t.Context(), db, f.tenant)
	require.NoError(t, err)
	require.Equal(t, before, after)
	var sessions, attendance int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM active.activity_sessions WHERE tenant_id = ?),
  (SELECT count(*) FROM active.activity_session_attendance WHERE tenant_id = ?)`, f.tenant, f.tenant).Scan(t.Context(), &sessions, &attendance))
	require.Equal(t, 1, sessions, "previous batch remains committed")
	require.Zero(t, attendance, "failed batch leaves no target writes")
	_, err = db.ExecContext(t.Context(), `DROP TRIGGER reject_backfill ON active.activity_session_attendance`)
	require.NoError(t, err)
	finishPresenceBackfill(t, db, f.tenant)
}

func TestPresenceBackfillMeasuresActualLockWaits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	finishPresenceBackfill(t, db, f.tenant)
	_, err := db.ExecContext(t.Context(), `UPDATE schedule.activity_instances SET started_at = clock_timestamp() WHERE id = ?`, f.instance)
	require.NoError(t, err)
	locker, err := db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = locker.Rollback() }()
	_, err = locker.ExecContext(t.Context(), `SELECT id FROM active.activity_sessions WHERE tenant_id = ? FOR UPDATE`, f.tenant)
	require.NoError(t, err)
	result := make(chan error, 1)
	go func() { _, err := PresenceBackfillBatch(t.Context(), db, f.tenant, 1); result <- err }()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()
   AND wait_event_type = 'Lock'`).Scan(t.Context(), &waiting)
		return err == nil && waiting > 0
	}, 3*time.Second, 10*time.Millisecond)
	// Hold long enough for several 10 ms samples; no query-duration proxy.
	time.Sleep(80 * time.Millisecond)
	require.NoError(t, locker.Rollback())
	require.NoError(t, <-result)
	metrics, err := PresenceBackfillMetrics(t.Context(), db, f.tenant)
	require.NoError(t, err)
	require.Positive(t, metrics.LockWaitMS)
	require.Positive(t, metrics.PoolWaitMS)
	require.GreaterOrEqual(t, metrics.BatchMaxMS, metrics.BatchP95MS)
}

func TestPresenceBackfillTenantIsolationAndCheckpointLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015381", false))
	require.NoError(t, runPresenceMigration(t.Context(), db, "001015381", true))
	a, b := createPresenceExpandFixture(t, db), createPresenceExpandFixture(t, db)
	finishPresenceBackfill(t, db, a.tenant)
	finishPresenceBackfill(t, db, b.tenant)
	for _, table := range []string{"active.activity_sessions", "active.activity_session_attendance", "active.presence_backfill_checkpoints", "active.presence_backfill_batches"} {
		var enabled, forced bool
		require.NoError(t, db.NewRaw(`SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE oid = ?::regclass`, table).Scan(t.Context(), &enabled, &forced))
		require.True(t, enabled)
		require.True(t, forced)
		for _, own := range []presenceExpandFixture{a, b} {
			require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
				var tenants []int64
				err := tx.NewRaw("SELECT DISTINCT tenant_id FROM "+table).Scan(ctx, &tenants)
				require.Equal(t, []int64{own.tenant}, tenants)
				return err
			}))
		}
	}
	err := testpkg.WithTenantTx(t, t.Context(), db, a.tenant, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE active.presence_backfill_checkpoints SET state = '{}' WHERE tenant_id = ?`, a.tenant)
		return err
	})
	requirePresenceSQLState(t, err, "42501")
	require.ErrorContains(t, runPresenceMigration(t.Context(), db, "001015381", false), "checkpoints must be retained")
	require.ErrorContains(t, runPresenceMigration(t.Context(), db, "001015378", false), "requires empty target tables")
	// Source deletion remains authoritative; existing target FKs remove orphans.
	_, err = db.ExecContext(t.Context(), `DELETE FROM schedule.instance_students WHERE id = ?`, a.participant)
	require.NoError(t, err)
	report := finishPresenceBackfill(t, db, a.tenant)
	require.Zero(t, report.Attendance.TargetCount)
	require.Zero(t, report.Orphans)
}

func TestPresenceBackfillPlansUseTenantKeysetIndexes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := createPresenceExpandFixture(t, db)
	_, err := db.ExecContext(t.Context(), `INSERT INTO schedule.activity_instances
  (tenant_id, date, title, start_time, end_time, room_id, status)
  SELECT tenant_id, date, 'Backfill plan ' || n, start_time, end_time, room_id,
   CASE WHEN n % 2 = 0 THEN 'planned' ELSE 'cancelled' END
  FROM schedule.activity_instances CROSS JOIN generate_series(1, 2048) n WHERE id = ?`, f.instance)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO schedule.instance_students (tenant_id, instance_id, student_id)
  SELECT i.tenant_id, i.id, p.student_id FROM schedule.activity_instances i
  CROSS JOIN schedule.instance_students p WHERE i.tenant_id = ? AND i.id <> ? AND p.id = ?`, f.tenant, f.instance, f.participant)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `ANALYZE schedule.activity_instances; ANALYZE schedule.instance_students`)
	require.NoError(t, err)
	for _, table := range []string{"schedule.activity_instances", "schedule.instance_students"} {
		var plan []string
		require.NoError(t, db.NewRaw("EXPLAIN (ANALYZE, BUFFERS) SELECT id FROM "+table+
			" WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT 100", f.tenant, f.instance).Scan(t.Context(), &plan))
		evidence := strings.Join(plan, "\n")
		require.Contains(t, evidence, "Index")
		require.Contains(t, evidence, "Index Cond:")
		require.NotContains(t, evidence, "Sort")
		require.NotContains(t, evidence, "Seq Scan")
		t.Log(table, evidence)
	}
	var report PresenceBackfillReport
	for batch := 0; batch < 20; batch++ {
		report, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 500)
		require.NoError(t, err)
		if report.Complete {
			break
		}
	}
	require.True(t, report.Complete)
	require.EqualValues(t, 2049, report.PlannedOccurrences)
	require.EqualValues(t, 2049, report.PlannedAssignments)
	require.EqualValues(t, 1, report.Sessions.TargetCount, "planned/cancelled occurrences are not actual sessions")
	require.EqualValues(t, 2049, report.Attendance.TargetCount)
	require.Zero(t, report.Mismatches)
	metrics, err := PresenceBackfillMetrics(t.Context(), db, f.tenant)
	require.NoError(t, err)
	require.Zero(t, metrics.OldestUnmigratedSeconds)
}
