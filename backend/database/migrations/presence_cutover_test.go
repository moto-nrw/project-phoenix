package migrations

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Expand and Backfill contracts describe a world in which the old
// execution and attendance columns are still authoritative. Restore that world
// inside the disposable clone so those tests keep testing what they were
// written for; production rollback deliberately retains the compatibility
// shape instead.
func setupPresenceStorageBeforeCutover(t *testing.T) *testpkg.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.RestorePresenceStorageBeforeCutover(t, db)
	return db
}

// presenceCutoverFixture returns a backfilled school that is ready to be
// switched: one running instance with a present participant.
func presenceCutoverFixture(t *testing.T, db *testpkg.DB) presenceExpandFixture {
	t.Helper()
	f := createPresenceExpandFixture(t, db)
	finishPresenceBackfill(t, db, f.tenant)
	return f
}

func presenceOldShapeJSON(t *testing.T, db bun.IDB, tenantID int64) string {
	t.Helper()
	var rows string
	require.NoError(t, db.NewRaw(`SELECT jsonb_build_object(
		'instances', coalesce((SELECT jsonb_agg(jsonb_build_array(i.id, i.status, i.active_group_id, i.started_by, i.started_at, i.completed_at, i.completed_by, i.reopen_until, i.completion_snapshot) ORDER BY i.id)
			FROM schedule.activity_instances i WHERE i.tenant_id = ?0), '[]'::jsonb),
		'participants', coalesce((SELECT jsonb_agg(jsonb_build_array(p.id, p.status, p.substatus, p.note, p.checked_in_at, p.checked_out_at, p.is_unplanned, p.not_scheduled, p.manual_status_at, p.student_status_day_id, p.pickup_exception_id) ORDER BY p.id)
			FROM schedule.instance_students p WHERE p.tenant_id = ?0), '[]'::jsonb))::text`, tenantID).Scan(t.Context(), &rows))
	return rows
}

func presenceCompatibilityWrites(t *testing.T, db bun.IDB) int64 {
	t.Helper()
	var writes int64
	// last_value reports 1 for a sequence nextval has never touched, so the
	// counter is read through pg_sequence_last_value, which reports NULL.
	require.NoError(t, db.NewRaw(`SELECT coalesce(pg_sequence_last_value('active.presence_compatibility_writes'), 0)`).Scan(t.Context(), &writes))
	return writes
}

// presenceDrift counts old-column rows that no longer match their owner row.
// A participant without an attendance row must read as expected attendance.
func presenceDrift(t *testing.T, db bun.IDB, tenantID int64) (sessions, attendance int64) {
	t.Helper()
	require.NoError(t, db.NewRaw(presenceDriftQuery, tenantID).Scan(t.Context(), &sessions, &attendance))
	return sessions, attendance
}

const presenceDriftQuery = `SELECT
	(SELECT count(*) FROM schedule.activity_instances i
	 LEFT JOIN active.activity_sessions s ON s.tenant_id = i.tenant_id AND s.schedule_instance_id = i.id
	 WHERE i.tenant_id = ?0 AND (
	   (s.id IS NULL AND i.status IN ('active', 'completed'))
	   OR (s.id IS NOT NULL AND ROW(i.status, i.active_group_id, i.started_by, i.started_at, i.completed_at, i.completed_by, i.reopen_until, i.completion_snapshot)
	       IS DISTINCT FROM ROW(s.status, s.active_group_id, s.started_by, s.started_at, s.completed_at, s.completed_by, s.reopen_until, s.completion_snapshot)))),
	(SELECT count(*) FROM schedule.instance_students p
	 LEFT JOIN active.activity_session_attendance a ON a.tenant_id = p.tenant_id AND a.instance_student_id = p.id
	 WHERE p.tenant_id = ?0 AND ROW(p.status, p.substatus, p.note, p.checked_in_at, p.checked_out_at, p.is_unplanned, p.not_scheduled, p.manual_status_at, p.student_status_day_id, p.pickup_exception_id)
	   IS DISTINCT FROM ROW(coalesce(a.status, 'expected'), a.substatus, a.note, a.checked_in_at, a.checked_out_at, coalesce(a.is_unplanned, false), coalesce(a.not_scheduled, false), a.manual_status_at, a.student_status_day_id, a.pickup_exception_id))`

func TestPresenceCutoverPreservesContractGolden(t *testing.T) {
	t.Parallel()
	db := setupPresenceStorageBeforeCutover(t)
	f := presenceCutoverFixture(t, db)
	before := presenceOldShapeJSON(t, db, f.tenant)
	require.NoError(t, finalizePresenceStorage(t.Context(), db, installPresenceCompatibility))
	require.JSONEq(t, before, presenceOldShapeJSON(t, db, f.tenant),
		"the previous image must read the same execution and attendance columns after the switch")

	installed, err := presenceCompatibilityInstalled(t.Context(), db)
	require.NoError(t, err)
	require.True(t, installed)
	sessions, attendance := presenceDrift(t, db, f.tenant)
	require.Zero(t, sessions)
	require.Zero(t, attendance)
	require.Zero(t, presenceCompatibilityWrites(t, db), "the switch itself routes nothing")

	report, err := PresenceBackfillStatus(t.Context(), db, f.tenant)
	require.NoError(t, err)
	require.Equal(t, presenceCutoverPhase, report.Phase)
	require.True(t, report.Complete, "the switch leaves its own verdict in the checkpoint")
	require.NotNil(t, report.VerifiedAt)
	require.Equal(t, report.Sessions.SourceChecksum, report.Sessions.TargetChecksum)
	require.Equal(t, report.Attendance.SourceChecksum, report.Attendance.TargetChecksum)

	_, err = PresenceBackfillBatch(t.Context(), db, f.tenant, 1)
	require.ErrorContains(t, err, "already cut over")
	require.ErrorContains(t, RestartPresenceBackfill(t.Context(), db, f.tenant), "already cut over")
}

func TestPresenceCutoverRequiresCompletedBackfill(t *testing.T) {
	t.Parallel()
	db := setupPresenceStorageBeforeCutover(t)
	createPresenceExpandFixture(t, db)
	switched := false
	err := finalizePresenceStorage(t.Context(), db, func(context.Context, bun.Tx) error {
		switched = true
		return nil
	})
	require.ErrorContains(t, err, "requires a completed backfill pass")
	require.False(t, switched)
}

func TestPresenceCutoverPreflightNamesSchoolsWithoutAPass(t *testing.T) {
	t.Parallel()
	db := setupPresenceStorageBeforeCutover(t)
	f := createPresenceExpandFixture(t, db)
	err := presenceCutoverPrecondition(t.Context(), db)
	require.ErrorContains(t, err, "no completed presence backfill pass")
	finishPresenceBackfill(t, db, f.tenant)
	require.NoError(t, presenceCutoverPrecondition(t.Context(), db))
}

func TestPresenceCutoverAppliesTheFinalDelta(t *testing.T) {
	t.Parallel()
	db := setupPresenceStorageBeforeCutover(t)
	f := presenceCutoverFixture(t, db)
	ctx := t.Context()
	account := testpkg.CreateTestAccount(t, db, "presence-cutover-delta@example.test")
	// Changes the backfill has not seen: a completion, an attendance edit, a
	// newcomer that started, and a target-only edit that the source overwrites.
	_, err := db.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = 'completed',
		completed_at = '2026-09-09 11:00:00+02', completed_by = ?, reopen_until = '2026-09-09 11:05:00+02',
		completion_snapshot = '{"visit_ids":[],"attendance":[]}' WHERE tenant_id = ? AND id = ?`, account.ID, f.tenant, f.instance)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE schedule.instance_students SET status = 'absent', substatus = 'sick', note = 'Fieber',
		student_status_day_id = ? WHERE tenant_id = ? AND id = ?`, f.statusDay, f.tenant, f.participant)
	require.NoError(t, err)
	room := testpkg.CreateTestRoomForTenant(t, db, f.tenant, "Presence Cutover Newcomer")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, f.tenant).ID
	newcomer := testpkg.CreateTestActivityInstanceForTenant(t, db, f.tenant, testpkg.Date(2026, 9, 10), room.ID,
		testpkg.ActivityInstanceOpts{Status: "active", ActiveGroupID: &group}).ID
	_, err = db.ExecContext(ctx, `UPDATE active.activity_sessions SET started_at = '2020-01-01 00:00:00+00' WHERE tenant_id = ? AND schedule_instance_id = ?`, f.tenant, f.instance)
	require.NoError(t, err)
	before := presenceOldShapeJSON(t, db, f.tenant)

	require.NoError(t, finalizePresenceStorage(ctx, db, installPresenceCompatibility))
	require.JSONEq(t, before, presenceOldShapeJSON(t, db, f.tenant))
	sessions, attendance := presenceDrift(t, db, f.tenant)
	require.Zero(t, sessions)
	require.Zero(t, attendance)
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, f.tenant, newcomer).Scan(ctx, &status))
	require.Equal(t, "active", status)
	var startedAt time.Time
	require.NoError(t, db.NewRaw(`SELECT coalesce(started_at, '2026-01-01') FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, f.tenant, f.instance).Scan(ctx, &startedAt))
	require.NotEqual(t, 2020, startedAt.Year(), "a target-only edit is overwritten from the source")
	report, err := PresenceBackfillStatus(ctx, db, f.tenant)
	require.NoError(t, err)
	require.EqualValues(t, 2, report.Sessions.TargetCount)
}

func TestPresenceCutoverFinalDeltaRollsBackWithTheSwitch(t *testing.T) {
	t.Parallel()
	db := setupPresenceStorageBeforeCutover(t)
	f := presenceCutoverFixture(t, db)
	ctx := t.Context()
	_, err := db.ExecContext(ctx, `UPDATE schedule.instance_students SET status = 'absent', substatus = 'excused' WHERE tenant_id = ? AND id = ?`, f.tenant, f.participant)
	require.NoError(t, err)
	checkpointBefore, err := PresenceBackfillStatus(ctx, db, f.tenant)
	require.NoError(t, err)

	injected := errors.New("injected switch failure")
	err = finalizePresenceStorage(ctx, db, func(ctx context.Context, tx bun.Tx) error {
		// A half-applied switch: the counter exists, then the install fails.
		if _, err := tx.ExecContext(ctx, presenceCompatibilityCounters); err != nil {
			return err
		}
		return injected
	})
	require.ErrorIs(t, err, injected)
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_session_attendance WHERE tenant_id = ? AND instance_student_id = ?`, f.tenant, f.participant).Scan(ctx, &status))
	require.Equal(t, "present", status, "the final delta rolled back with the failed switch")
	checkpointAfter, err := PresenceBackfillStatus(ctx, db, f.tenant)
	require.NoError(t, err)
	require.Equal(t, checkpointBefore, checkpointAfter, "the checkpoint evidence rolled back too")
	installed, err := presenceCompatibilityInstalled(ctx, db)
	require.NoError(t, err)
	require.False(t, installed)
	var counter bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('active.presence_compatibility_writes') IS NOT NULL`).Scan(ctx, &counter))
	require.False(t, counter, "nothing of the half-applied switch survives")

	// A clean retry switches.
	require.NoError(t, presenceCutoverUp(ctx, db))
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_session_attendance WHERE tenant_id = ? AND instance_student_id = ?`, f.tenant, f.participant).Scan(ctx, &status))
	require.Equal(t, "absent", status)
	require.NoError(t, presenceCutoverUp(ctx, db), "running the migration again resumes as a no-op")
	require.ErrorContains(t, finalizePresenceStorage(ctx, db, installPresenceCompatibility), "already installed")
}

func TestPresenceCutoverRefusesUnequalTargets(t *testing.T) {
	t.Parallel()
	db := setupPresenceStorageBeforeCutover(t)
	f := presenceCutoverFixture(t, db)
	ctx := t.Context()
	// A trigger that rejects one delta row leaves the targets unequal to the
	// source; the switch must refuse rather than freeze a wrong mirror.
	_, err := db.ExecContext(ctx, `CREATE FUNCTION active.refuse_delta() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.status = 'absent' THEN RETURN NULL; END IF; RETURN NEW; END $$;
		CREATE TRIGGER refuse_delta BEFORE UPDATE ON active.activity_session_attendance FOR EACH ROW EXECUTE FUNCTION active.refuse_delta()`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE schedule.instance_students SET status = 'absent' WHERE tenant_id = ? AND id = ?`, f.tenant, f.participant)
	require.NoError(t, err)
	err = finalizePresenceStorage(ctx, db, installPresenceCompatibility)
	require.ErrorContains(t, err, "is not reproducible")
	require.ErrorContains(t, err, "1 mismatched")
	installed, err := presenceCompatibilityInstalled(ctx, db)
	require.NoError(t, err)
	require.False(t, installed)
}

// The new image writes the owner tables only; the mirror keeps the old
// columns equal without routing anything.
func TestPresenceCompatibilityMirrorsOwnerWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	writesBefore := presenceCompatibilityWrites(t, db)
	room := testpkg.CreateTestRoom(t, db, "Presence Mirror")
	student := testpkg.CreateTestStudent(t, db, "Presence", "Mirror", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Presence", "Mirror")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID).ID
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2026, 9, 14), room.ID, testpkg.ActivityInstanceOpts{}).ID
	participant := testpkg.CreateTestInstanceStudent(t, db, instance, student.ID, "").ID

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO active.activity_sessions (tenant_id, schedule_instance_id, status, active_group_id, started_by, started_at)
			VALUES (?, ?, 'active', ?, ?, '2026-09-14 14:00:00+02')`, tenantID, instance, group, staff.ID)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO active.activity_session_attendance (tenant_id, instance_student_id, status, checked_in_at)
			VALUES (?, ?, 'present', '2026-09-14 14:01:00+02')`, tenantID, participant)
		return err
	}))
	var status string
	var activeGroup *int64
	require.NoError(t, db.NewRaw(`SELECT status, active_group_id FROM schedule.activity_instances WHERE id = ?`, instance).Scan(ctx, &status, &activeGroup))
	require.Equal(t, "active", status)
	require.NotNil(t, activeGroup)
	require.Equal(t, group, *activeGroup)
	var attendance string
	var checkedIn *time.Time
	require.NoError(t, db.NewRaw(`SELECT status, checked_in_at FROM schedule.instance_students WHERE id = ?`, participant).Scan(ctx, &attendance, &checkedIn))
	require.Equal(t, "present", attendance)
	require.NotNil(t, checkedIn)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE active.activity_sessions SET status = 'completed', completed_at = '2026-09-14 15:00:00+02',
			reopen_until = '2026-09-14 15:05:00+02', completion_snapshot = '{"visit_ids":[1]}' WHERE tenant_id = ? AND schedule_instance_id = ?`, tenantID, instance)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE active.activity_session_attendance SET checked_out_at = '2026-09-14 15:00:00+02' WHERE tenant_id = ? AND instance_student_id = ?`, tenantID, participant)
		return err
	}))
	var snapshot string
	require.NoError(t, db.NewRaw(`SELECT status, completion_snapshot::text FROM schedule.activity_instances WHERE id = ?`, instance).Scan(ctx, &status, &snapshot))
	require.Equal(t, "completed", status)
	require.JSONEq(t, `{"visit_ids":[1]}`, snapshot)
	sessions, drifted := presenceDrift(t, db, tenantID)
	require.Zero(t, sessions)
	require.Zero(t, drifted)

	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM active.activity_session_attendance WHERE tenant_id = ? AND instance_student_id = ?`, tenantID, participant); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, tenantID, instance)
		return err
	}))
	var resetGroup, resetStarted *int64
	require.NoError(t, db.NewRaw(`SELECT status, active_group_id, started_by FROM schedule.activity_instances WHERE id = ?`, instance).Scan(ctx, &status, &resetGroup, &resetStarted))
	require.Equal(t, "planned", status)
	require.Nil(t, resetGroup)
	require.Nil(t, resetStarted)
	var resetCheckedIn *time.Time
	require.NoError(t, db.NewRaw(`SELECT status, checked_in_at FROM schedule.instance_students WHERE id = ?`, participant).Scan(ctx, &attendance, &resetCheckedIn))
	require.Equal(t, "expected", attendance)
	require.Nil(t, resetCheckedIn)
	require.Equal(t, writesBefore, presenceCompatibilityWrites(t, db), "owner writes never count as compatibility writes")
}

// The previous image writes the old columns; the routing triggers carry the
// change into the owner tables and count it.
func TestPresenceCompatibilityRoutesPreviousImageWrites(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	room := testpkg.CreateTestRoom(t, db, "Presence Routing")
	student := testpkg.CreateTestStudent(t, db, "Presence", "Routing", "3b")
	walkIn := testpkg.CreateTestStudent(t, db, "Presence", "WalkIn", "3b")
	staff := testpkg.CreateTestStaff(t, db, "Presence", "Routing")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID).ID
	instance := testpkg.CreateTestActivityInstance(t, db, testpkg.Date(2026, 9, 15), room.ID, testpkg.ActivityInstanceOpts{}).ID
	participant := testpkg.CreateTestInstanceStudent(t, db, instance, student.ID, "").ID
	writesBefore := presenceCompatibilityWrites(t, db)

	// Start, as the previous image writes it.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = 'active', active_group_id = ?, started_by = ?, started_at = '2026-09-15 14:00:00+02'
			WHERE tenant_id = ? AND id = ?`, group, staff.ID, tenantID, instance)
		return err
	}))
	var sessionStatus string
	var sessionGroup *int64
	require.NoError(t, db.NewRaw(`SELECT status, active_group_id FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, tenantID, instance).Scan(ctx, &sessionStatus, &sessionGroup))
	require.Equal(t, "active", sessionStatus)
	require.Equal(t, group, *sessionGroup)
	require.Equal(t, writesBefore+1, presenceCompatibilityWrites(t, db))

	// Check-in and a walk-in insert, as the previous image writes them.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE schedule.instance_students SET status = 'present', checked_in_at = '2026-09-15 14:02:00+02', updated_at = now()
			WHERE tenant_id = ? AND id = ?`, tenantID, participant); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO schedule.instance_students (tenant_id, instance_id, student_id, status, checked_in_at, is_unplanned)
			VALUES (?, ?, ?, 'present', '2026-09-15 14:03:00+02', true)`, tenantID, instance, walkIn.ID)
		return err
	}))
	var attendanceRows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.activity_session_attendance a
		JOIN schedule.instance_students p ON p.tenant_id = a.tenant_id AND p.id = a.instance_student_id
		WHERE a.tenant_id = ? AND p.instance_id = ? AND a.status = 'present'`, tenantID, instance).Scan(ctx, &attendanceRows))
	require.Equal(t, 2, attendanceRows)
	require.Equal(t, writesBefore+3, presenceCompatibilityWrites(t, db))
	var unplanned bool
	require.NoError(t, db.NewRaw(`SELECT a.is_unplanned FROM active.activity_session_attendance a
		JOIN schedule.instance_students p ON p.tenant_id = a.tenant_id AND p.id = a.instance_student_id
		WHERE a.tenant_id = ? AND p.student_id = ?`, tenantID, walkIn.ID).Scan(ctx, &unplanned))
	require.True(t, unplanned)

	// A planned participant inserted with default columns needs no row.
	extra := testpkg.CreateTestStudent(t, db, "Presence", "Planned", "3b")
	planned := testpkg.CreateTestInstanceStudent(t, db, instance, extra.ID, "").ID
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.activity_session_attendance WHERE tenant_id = ? AND instance_student_id = ?`, tenantID, planned).Scan(ctx, &rows))
	require.Zero(t, rows)
	require.Equal(t, writesBefore+3, presenceCompatibilityWrites(t, db))

	// A full-row update with unchanged values routes nothing.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = status, active_group_id = active_group_id, started_by = started_by,
			started_at = started_at, title = title WHERE tenant_id = ? AND id = ?`, tenantID, instance)
		return err
	}))
	require.Equal(t, writesBefore+3, presenceCompatibilityWrites(t, db))

	// Completion, then a reset to planned removes the session.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = 'completed', completed_at = '2026-09-15 15:00:00+02' WHERE tenant_id = ? AND id = ?`, tenantID, instance)
		return err
	}))
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, tenantID, instance).Scan(ctx, &sessionStatus))
	require.Equal(t, "completed", sessionStatus)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = 'planned', active_group_id = NULL, started_by = NULL, started_at = NULL, completed_at = NULL
			WHERE tenant_id = ? AND id = ?`, tenantID, instance)
		return err
	}))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, tenantID, instance).Scan(ctx, &rows))
	require.Zero(t, rows)
	require.Equal(t, writesBefore+5, presenceCompatibilityWrites(t, db))
	sessions, drifted := presenceDrift(t, db, tenantID)
	require.Zero(t, sessions)
	require.Zero(t, drifted)

	// Deleting the occurrence cascades through both owners.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM schedule.activity_instances WHERE tenant_id = ? AND id = ?`, tenantID, instance)
		return err
	}))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.activity_session_attendance a WHERE a.tenant_id = ? AND NOT EXISTS (
		SELECT 1 FROM schedule.instance_students p WHERE p.tenant_id = a.tenant_id AND p.id = a.instance_student_id)`, tenantID).Scan(ctx, &rows))
	require.Zero(t, rows)
}

func TestPresenceCompatibilityIsTenantIsolated(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	own := testpkg.Tenant(t)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	room := testpkg.CreateTestRoomForTenant(t, db, other, "Presence Foreign")
	student := testpkg.CreateTestStudentForTenant(t, db, other, "Presence", "Foreign", "4a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, other).ID
	instance := testpkg.CreateTestActivityInstanceForTenant(t, db, other, testpkg.Date(2026, 9, 16), room.ID,
		testpkg.ActivityInstanceOpts{Status: "active", ActiveGroupID: &group}).ID
	var participant int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.instance_students (tenant_id, instance_id, student_id, status)
		VALUES (?, ?, ?, 'present') RETURNING id`, other, instance, student.ID).Scan(ctx, &participant))

	// The own tenant's session can neither read, route into, nor mirror onto
	// the other school's rows, and RLS refuses a mislabeled owner row.
	require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, own, func(ctx context.Context, tx testpkg.Tx) error {
		var visible int
		if err := tx.NewRaw(`SELECT count(*) FROM active.activity_sessions WHERE schedule_instance_id = ?`, instance).Scan(ctx, &visible); err != nil {
			return err
		}
		require.Zero(t, visible)
		result, err := tx.ExecContext(ctx, `UPDATE schedule.activity_instances SET status = 'completed' WHERE id = ?`, instance)
		if err != nil {
			return err
		}
		updated, _ := result.RowsAffected()
		require.Zero(t, updated)
		_, err = tx.ExecContext(ctx, `INSERT INTO active.activity_session_attendance (tenant_id, instance_student_id, status) VALUES (?, ?, 'absent')`, other, participant)
		requirePresenceSQLState(t, err, "42501")
		return nil
	}))
	var status string
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_sessions WHERE tenant_id = ? AND schedule_instance_id = ?`, other, instance).Scan(ctx, &status))
	require.Equal(t, "active", status)
	require.NoError(t, db.NewRaw(`SELECT status FROM active.activity_session_attendance WHERE tenant_id = ? AND instance_student_id = ?`, other, participant).Scan(ctx, &status))
	require.Equal(t, "present", status)

	// The other school's own session routes and mirrors normally.
	require.NoError(t, testpkg.WithTenantTx(t, t.Context(), db, other, func(ctx context.Context, tx testpkg.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE active.activity_session_attendance SET checked_out_at = now() WHERE instance_student_id = ?`, participant)
		return err
	}))
	var checkedOut *time.Time
	require.NoError(t, db.NewRaw(`SELECT checked_out_at FROM schedule.instance_students WHERE id = ?`, participant).Scan(ctx, &checkedOut))
	require.NotNil(t, checkedOut)
}
