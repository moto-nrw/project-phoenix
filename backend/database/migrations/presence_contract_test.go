package migrations

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Cutover contracts describe the rollback window, in which the mirror
// still exists. They restore it inside their own clone.
func setupPresenceStorageBeforeContract(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestorePresenceStorageBeforeContract(t, db)
	return db
}

// Isolated DDL tests use the same live checks as ordinary deployments.
// Registered execution enters through presenceContractUp.
func contractPresenceStorage(ctx context.Context, db *bun.DB) error {
	return contractPresenceStorageChecked(ctx, db, nil)
}

// presenceContractFixture rebuilds the frozen pre-Contract world: the
// historical authoritative columns, their backfill and the committed cutover
// with its rollback mirror, routing triggers and write counter. After the
// switch the current image writes the owners: the running block completes and
// a second participant is booked without an attendance row.
func presenceContractFixture(t *testing.T) (*testpkg.DB, presenceExpandFixture) {
	t.Helper()
	db := setupPresenceStorageBeforeCutover(t)
	f := presenceCutoverFixture(t, db)
	require.NoError(t, presenceCutoverUp(t.Context(), db))
	_, err := db.ExecContext(t.Context(), `UPDATE active.activity_sessions
		SET status = 'completed', completed_at = '2026-09-09 15:00:00+00', reopen_until = '2026-09-09 17:00:00+00',
			completion_snapshot = '{"present": 1}'
		WHERE tenant_id = ? AND schedule_instance_id = ?`, f.tenant, f.instance)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `UPDATE active.activity_session_attendance
		SET substatus = 'late', note = 'nach dem Cutover', student_status_day_id = ?, pickup_exception_id = ?
		WHERE tenant_id = ? AND instance_student_id = ?`, f.statusDay, f.pickup, f.tenant, f.participant)
	require.NoError(t, err)
	student := testpkg.CreateTestStudentForTenant(t, db, f.tenant, "Presence", "Contract", "2a")
	_, err = db.ExecContext(t.Context(), `INSERT INTO schedule.instance_students (tenant_id, instance_id, student_id)
		VALUES (?, ?, ?)`, f.tenant, f.instance, student.ID)
	require.NoError(t, err)
	return db, f
}

// presenceContractRows snapshots the owner rows and every retained plan
// column of every school, with the planning status as the Contract narrows
// it, so the Contract must leave the snapshot equal.
func presenceContractRows(t *testing.T, db bun.IDB) []string {
	t.Helper()
	var snapshots []string
	for _, query := range []string{
		`SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY r.id)::text, '[]') FROM active.activity_sessions AS r`,
		`SELECT coalesce(jsonb_agg(to_jsonb(r) ORDER BY r.id)::text, '[]') FROM active.activity_session_attendance AS r`,
		`SELECT coalesce(jsonb_agg(` + presenceContractPlanRow + ` ORDER BY r.id)::text, '[]') FROM schedule.activity_instances AS r`,
		`SELECT coalesce(jsonb_agg(` + presenceContractParticipantRow + ` ORDER BY r.id)::text, '[]') FROM schedule.instance_students AS r`,
	} {
		var snapshot string
		require.NoError(t, db.NewRaw(query).Scan(t.Context(), &snapshot))
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func presenceMirrorRetained(t *testing.T, db bun.IDB) bool {
	t.Helper()
	var retained bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('active.presence_compatibility_writes') IS NOT NULL
		AND to_regprocedure('schedule.route_instance_student_compatibility()') IS NOT NULL
		AND EXISTS (SELECT FROM pg_trigger WHERE tgname = 'activity_sessions_mirror')
		AND EXISTS (SELECT FROM pg_attribute WHERE attrelid = 'schedule.instance_students'::regclass
			AND attname = 'checked_in_at' AND NOT attisdropped)`).Scan(t.Context(), &retained))
	return retained
}

func TestPresenceContractRunsLockedCheckBeforeDestructiveDDL(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	refused := errors.New("locked replay check refused cleanup")
	checked := false
	err := contractPresenceStorageChecked(t.Context(), db, func(ctx context.Context, connection bun.IDB) error {
		_, inTransaction := connection.(bun.Tx)
		require.True(t, inTransaction)
		checked = true
		return refused
	})
	require.ErrorIs(t, err, refused)
	require.True(t, checked)
	require.True(t, presenceMirrorRetained(t, db))
}

func TestPresenceContractRemovesMirrorAndPreservesOwners(t *testing.T) {
	t.Parallel()
	db, f := presenceContractFixture(t)
	before := presenceContractRows(t, db)
	require.NoError(t, contractPresenceStorage(t.Context(), db))
	require.Equal(t, before, presenceContractRows(t, db))

	var absent bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('active.presence_compatibility_writes') IS NULL
		AND to_regprocedure('active.mirror_activity_session()') IS NULL
		AND to_regprocedure('active.mirror_activity_session_attendance()') IS NULL
		AND to_regprocedure('schedule.route_activity_instance_compatibility()') IS NULL
		AND to_regprocedure('schedule.route_instance_student_compatibility()') IS NULL
		AND NOT EXISTS (SELECT FROM pg_trigger WHERE tgname IN ('activity_sessions_mirror', 'activity_session_attendance_mirror',
			'activity_instances_route_compatibility', 'instance_students_route_compatibility'))
		AND NOT EXISTS (SELECT FROM pg_attribute WHERE NOT attisdropped AND (
			(attrelid = 'schedule.activity_instances'::regclass AND attname = ANY(`+presenceMirroredExecutionColumns+`))
			OR (attrelid = 'schedule.instance_students'::regclass AND attname = ANY(`+presenceMirroredAttendanceColumns+`))))
		AND to_regclass('schedule.idx_activity_instances_active_group_unique') IS NULL
		AND to_regclass('schedule.idx_activity_instances_reopenable') IS NULL
		AND to_regclass('schedule.idx_instance_students_status') IS NULL`).Scan(t.Context(), &absent))
	require.True(t, absent, "every mirror column, trigger, function, index and the counter must be gone")

	// The running block the mirror showed as completed is a planned
	// occurrence whose session holds the execution; the planning status
	// accepts nothing else any more.
	var planning, session string
	require.NoError(t, db.NewRaw(`SELECT instance.status, session.status FROM schedule.activity_instances AS instance
		JOIN active.activity_sessions AS session ON session.tenant_id = instance.tenant_id AND session.schedule_instance_id = instance.id
		WHERE instance.id = ?`, f.instance).Scan(t.Context(), &planning, &session))
	require.Equal(t, "planned", planning)
	require.Equal(t, "completed", session)
	_, err := db.ExecContext(t.Context(), `UPDATE schedule.activity_instances SET status = 'active' WHERE id = ?`, f.instance)
	requirePresenceSQLState(t, err, "23514")

	require.ErrorContains(t, presenceContractDown(t.Context(), db), "restore the verified pre-Contract backup")
	require.Equal(t, before, presenceContractRows(t, db), "refused Down must not rewrite current owner data")

	_, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	require.ErrorContains(t, err, "removed by the Contract")
	require.ErrorContains(t, RestartPresenceBackfill(t.Context(), db, f.tenant), "removed by the Contract")
}

func TestPresenceContractKeepsOwnerStorageIsolatedAndCascading(t *testing.T) {
	t.Parallel()
	db, a := presenceContractFixture(t)
	b := createPresenceExpandFixture(t, db)
	requireRoutedOwnerRows(t, db, b)
	require.NoError(t, contractPresenceStorage(t.Context(), db))

	var unforced int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_class WHERE oid IN ('schedule.activity_instances'::regclass,
		'schedule.instance_students'::regclass, 'active.activity_sessions'::regclass, 'active.activity_session_attendance'::regclass)
		AND NOT (relrowsecurity AND relforcerowsecurity)`).Scan(t.Context(), &unforced))
	require.Zero(t, unforced)
	for _, own := range []presenceExpandFixture{a, b} {
		var tenants []int64
		require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own.tenant, func(ctx context.Context, tx testpkg.Tx) error {
			return tx.NewRaw(`SELECT tenant_id FROM active.activity_sessions
				UNION ALL SELECT tenant_id FROM active.activity_session_attendance`).Scan(ctx, &tenants)
		}))
		require.NotEmpty(t, tenants)
		for _, tenant := range tenants {
			require.Equal(t, own.tenant, tenant, "owner reads stay tenant scoped after the Contract")
		}
	}

	_, err := db.ExecContext(t.Context(), `DELETE FROM schedule.activity_instances WHERE tenant_id = ? AND id = ?`, a.tenant, a.instance)
	require.NoError(t, err)
	var orphans int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM active.activity_sessions WHERE tenant_id = ?0 AND schedule_instance_id = ?1)
		+ (SELECT count(*) FROM active.activity_session_attendance WHERE tenant_id = ?0 AND instance_student_id = ?2)`,
		a.tenant, a.instance, a.participant).Scan(t.Context(), &orphans))
	require.Zero(t, orphans, "deleting the plan must still cascade into both owners")
}

// requireRoutedOwnerRows proves that a school a previous image wrote after the
// switch reached the owners through the routing.
func requireRoutedOwnerRows(t *testing.T, db *testpkg.DB, f presenceExpandFixture) {
	t.Helper()
	var sessions, attendance int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM active.activity_sessions WHERE tenant_id = ?0),
		(SELECT count(*) FROM active.activity_session_attendance WHERE tenant_id = ?0)`, f.tenant).Scan(t.Context(), &sessions, &attendance))
	require.Equal(t, 1, sessions, "the routing opened the running block's session")
	require.Equal(t, 1, attendance, "the routing recorded the present participant")
}

func TestPresenceContractIgnoresExecutionLeftoversOutsideTheMapping(t *testing.T) {
	t.Parallel()
	db, f := presenceContractFixture(t)
	// A previous image cancelled a started block: the routing removed its
	// session, and the mirror kept the old start on the cancelled row. The
	// backfill never mapped a cancelled block to a session either.
	room := testpkg.CreateTestRoomForTenant(t, db, f.tenant, "Presence Contract")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, f.tenant).ID
	cancelled := testpkg.CreateTestActivityInstanceForTenant(t, db, f.tenant, testpkg.Date(2026, 9, 10), room.ID,
		testpkg.ActivityInstanceOpts{Status: "active", ActiveGroupID: &group}).ID
	_, err := db.ExecContext(t.Context(), `UPDATE schedule.activity_instances SET status = 'cancelled', started_at = '2026-09-10 12:00:00+00'
		WHERE id = ?`, cancelled)
	require.NoError(t, err)
	var sessions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.activity_sessions WHERE schedule_instance_id = ?`, cancelled).Scan(t.Context(), &sessions))
	require.Zero(t, sessions)
	before := presenceContractRows(t, db)
	require.NoError(t, contractPresenceStorage(t.Context(), db))
	require.Equal(t, before, presenceContractRows(t, db))
}

func TestPresenceContractRefusesUnsafeStorage(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name    string
		arrange string
		want    string
	}{
		{"execution only in the mirror", `ALTER TABLE active.activity_sessions DISABLE TRIGGER activity_sessions_mirror;
			DELETE FROM active.activity_sessions WHERE schedule_instance_id = {instance};
			ALTER TABLE active.activity_sessions ENABLE TRIGGER activity_sessions_mirror`, "execution mirror drift"},
		{"execution differs", `ALTER TABLE schedule.activity_instances DISABLE TRIGGER activity_instances_route_compatibility;
			UPDATE schedule.activity_instances SET started_at = '2026-01-01 08:00:00+00' WHERE id = {instance};
			ALTER TABLE schedule.activity_instances ENABLE TRIGGER activity_instances_route_compatibility`, "execution mirror drift"},
		{"attendance only in the mirror", `ALTER TABLE schedule.instance_students DISABLE TRIGGER instance_students_route_compatibility;
			UPDATE schedule.instance_students SET status = 'absent' WHERE id = (SELECT max(id) FROM schedule.instance_students WHERE tenant_id = {tenant});
			ALTER TABLE schedule.instance_students ENABLE TRIGGER instance_students_route_compatibility`, "attendance mirror drift"},
		{"attendance differs", `ALTER TABLE active.activity_session_attendance DISABLE TRIGGER activity_session_attendance_mirror;
			UPDATE active.activity_session_attendance SET note = 'nur beim Owner' WHERE instance_student_id = {participant};
			ALTER TABLE active.activity_session_attendance ENABLE TRIGGER activity_session_attendance_mirror`, "attendance mirror drift"},
		{"dependent view", `CREATE VIEW schedule.contract_unexpected_dependency AS SELECT id, checked_in_at FROM schedule.instance_students`,
			"views depend on the mirrored columns"},
		{"stored function", `CREATE FUNCTION schedule.contract_hidden_reader() RETURNS bigint LANGUAGE sql
			AS 'SELECT count(*) FROM schedule.instance_students WHERE checked_in_at IS NOT NULL'`,
			"stored function still references the rollback mirror"},
		{"unvalidated FK", `ALTER TABLE active.activity_sessions ADD CONSTRAINT contract_unvalidated
			FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) NOT VALID`, "unvalidated foreign keys"},
		{"RLS not forced", `ALTER TABLE active.activity_session_attendance NO FORCE ROW LEVEL SECURITY`, "RLS disabled"},
		{"cutover missing", `DROP TRIGGER activity_sessions_mirror ON active.activity_sessions`, "requires the completed cutover"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, f := presenceContractFixture(t)
			arrange := strings.NewReplacer("{tenant}", strconv.FormatInt(f.tenant, 10),
				"{instance}", strconv.FormatInt(f.instance, 10), "{participant}", strconv.FormatInt(f.participant, 10)).
				Replace(scenario.arrange)
			_, err := db.ExecContext(t.Context(), arrange)
			require.NoError(t, err)
			before := presenceContractRows(t, db)
			require.ErrorContains(t, presenceContractDataPreflight(t.Context(), db), scenario.want)
			require.ErrorContains(t, contractPresenceStorage(t.Context(), db), scenario.want)
			require.Equal(t, before, presenceContractRows(t, db))
			var columns bool
			require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM pg_attribute WHERE attrelid = 'schedule.instance_students'::regclass
				AND attname = 'checked_in_at' AND NOT attisdropped)`).Scan(t.Context(), &columns))
			require.True(t, columns, "a refused Contract must retain the rollback columns")
		})
	}
}

func TestPresenceContractAllowsHistoricalCompatibilityWrites(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	_, err := db.ExecContext(t.Context(), `SELECT setval('active.presence_compatibility_writes', 12)`)
	require.NoError(t, err)
	require.NoError(t, presenceContractDataPreflight(t.Context(), db))
	require.NoError(t, contractPresenceStorage(t.Context(), db))
}

func TestPresenceContractRollsBackDDLOnFailedFingerprint(t *testing.T) {
	t.Parallel()
	db, f := presenceContractFixture(t)
	// The mirror is already dropped inside the transaction when the
	// fingerprint comparison runs; a change of owner rows must roll every
	// DROP back rather than commit a partially cleaned schema.
	_, err := db.ExecContext(t.Context(), `CREATE FUNCTION active.contract_mutate_owner() RETURNS event_trigger LANGUAGE plpgsql AS $$
		BEGIN UPDATE active.activity_session_attendance SET note = coalesce(note, '') || '!' WHERE instance_student_id = `+strconv.FormatInt(f.participant, 10)+`; END $$;
		CREATE EVENT TRIGGER contract_mutate_presence_owner ON sql_drop EXECUTE FUNCTION active.contract_mutate_owner()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP EVENT TRIGGER IF EXISTS contract_mutate_presence_owner; DROP FUNCTION IF EXISTS active.contract_mutate_owner()`)
	})
	before := presenceContractRows(t, db)
	require.ErrorContains(t, contractPresenceStorage(t.Context(), db), "rows or checksums changed during Contract")
	require.True(t, presenceMirrorRetained(t, db), "a failed fingerprint must roll back the DDL")
	require.Equal(t, before, presenceContractRows(t, db))
}

func TestPresenceContractHistoricalFixtureRestoresAfterRemoval(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	before := presenceContractRows(t, db)
	require.NoError(t, contractPresenceStorage(t.Context(), db))
	testpkg.RestorePresenceStorageBeforeContract(t, db)
	require.NoError(t, presenceContractDataPreflight(t.Context(), db))
	require.Zero(t, presenceCompatibilityWrites(t, db), "restoring the mirror routes nothing")
	require.NoError(t, contractPresenceStorage(t.Context(), db))
	require.Equal(t, before, presenceContractRows(t, db))
}

func TestPresenceContractHonorsCancellationBeforeDDL(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
	cancel()
	require.Error(t, contractPresenceStorage(ctx, db))
	require.NoError(t, presenceContractDataPreflight(t.Context(), db))
	require.True(t, presenceMirrorRetained(t, db))
}

func TestPresenceContractObservedLockWaitAndTimeout(t *testing.T) {
	t.Parallel()
	for _, holdUntilTimeout := range []bool{false, true} {
		name := "released"
		if holdUntilTimeout {
			name = "timeout"
		}
		t.Run(name, func(t *testing.T) {
			db, _ := presenceContractFixture(t)
			before := presenceContractRows(t, db)
			holder, err := db.BeginTx(t.Context(), nil)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback() }()
			_, err = holder.ExecContext(t.Context(), `LOCK TABLE active.activity_session_attendance IN SHARE MODE`)
			require.NoError(t, err)
			var holderPID int
			require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &holderPID))
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- contractPresenceStorage(ctx, db) }()
			waiting := func() bool {
				var blocked bool
				err := db.NewRaw(`SELECT EXISTS (SELECT FROM pg_stat_activity
					WHERE datname = current_database() AND wait_event_type = 'Lock'
					AND ? = ANY(pg_blocking_pids(pid))
					AND query LIKE '%LOCK TABLE schedule.activity_instances%')`, holderPID).Scan(t.Context(), &blocked)
				return err == nil && blocked
			}
			require.Eventually(t, waiting, 3*time.Second, 5*time.Millisecond)
			if !holdUntilTimeout {
				require.NoError(t, holder.Commit())
			}
			select {
			case result := <-done:
				if holdUntilTimeout {
					require.ErrorContains(t, result, "lock timeout")
				} else {
					require.NoError(t, result)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Contract did not finish after lock release or timeout")
			}
			if holdUntilTimeout {
				require.NoError(t, holder.Rollback())
			}
			require.Equal(t, before, presenceContractRows(t, db))
			require.Equal(t, holdUntilTimeout, presenceMirrorRetained(t, db))
		})
	}
}

func TestPresenceContractOrdinaryUpgradeContractsExistingStorage(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015432';
		SELECT setval('active.presence_compatibility_writes', 7)`)
	require.NoError(t, err)
	before := presenceContractRows(t, db)
	var output bytes.Buffer
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
	require.Contains(t, output.String(), "migration preflight OK 1.15.432")
	require.NoError(t, Migrate(t.Context(), db))
	require.Equal(t, before, presenceContractRows(t, db))
	require.False(t, presenceMirrorRetained(t, db), "ordinary upgrades must perform cleanup, not silently defer it")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015432')`).Scan(t.Context(), &applied))
	require.True(t, applied)
}

func TestPresenceContractOrdinaryUpgradeRejectsMirrorDrift(t *testing.T) {
	t.Parallel()
	db, f := presenceContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015432';
		ALTER TABLE schedule.instance_students DISABLE TRIGGER instance_students_route_compatibility;
		UPDATE schedule.instance_students SET note = 'nur im Spiegel' WHERE id = `+strconv.FormatInt(f.participant, 10)+`;
		ALTER TABLE schedule.instance_students ENABLE TRIGGER instance_students_route_compatibility`)
	require.NoError(t, err)
	var output bytes.Buffer
	require.ErrorContains(t, migratePreflightTo(t.Context(), db, &output), "attendance mirror drift")
	require.ErrorContains(t, Migrate(t.Context(), db), "attendance mirror drift")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015432')`).Scan(t.Context(), &applied))
	require.False(t, applied)
	require.True(t, presenceMirrorRetained(t, db))
}

func TestPresenceContractPreflightRequiresTheCutoverInAnEarlierRelease(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	ctx := context.WithValue(t.Context(), pendingMigrationsKey{}, map[string]bool{presenceCutoverVersion: true, presenceContractVersion: true})
	require.ErrorContains(t, presenceContractPrecondition(ctx, db), "must be live in an earlier production release")
	require.NoError(t, presenceContractPrecondition(t.Context(), db))
}

func TestPresenceContractInitialMigrationReachesContractedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var applied, removed bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM public.bun_migrations WHERE name = '001015432'),
		to_regclass('active.presence_compatibility_writes') IS NULL
		AND NOT EXISTS (SELECT FROM pg_attribute WHERE attrelid = 'schedule.instance_students'::regclass
			AND attname = 'checked_in_at' AND NOT attisdropped)`).Scan(t.Context(), &applied, &removed))
	require.True(t, applied)
	require.True(t, removed)
}

func TestPresenceContractFreshReplayRefusesUnexpectedData(t *testing.T) {
	t.Parallel()
	db, _ := presenceContractFixture(t)
	ctx := context.WithValue(t.Context(), freshStudentStorageKey{}, true)
	require.NoError(t, presenceContractPrecondition(ctx, db), "a fresh replay has no schema to inspect before the cutover ran")
	require.ErrorContains(t, presenceContractUp(ctx, db), "initial replay contains timetable or presence rows")
	require.True(t, presenceMirrorRetained(t, db))
}
