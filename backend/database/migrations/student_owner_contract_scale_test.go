package migrations

import (
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Match the observed production cardinalities with synthetic records. This is
// not a production-data clone and does not claim production distributions.
func TestStudentOwnerContractObservedProductionCardinalities(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	for school := range 12 {
		tenantID, _ := testpkg.CreateTestTenant(t, db)
		count := 120
		if school < 8 {
			count++
		}
		studentOwnerFixture(t, db, tenantID, count)
	}
	_, err := RunStudentOwnerBackfill(t.Context(), db, StudentOwnerBackfillOptions{})
	require.NoError(t, err)
	require.NoError(t, studentOwnerCutoverUp(t.Context(), db))
	require.NoError(t, studentCareAbsenceCompatibilityUp(t.Context(), db))
	_, err = db.ExecContext(t.Context(), `INSERT INTO active.student_status_days
		(tenant_id, student_id, date, status, reported_at)
		SELECT p.tenant_id, p.id, day.date, 'sick', '2026-09-01 08:00:00+02'::timestamptz
		FROM users.student_profiles p CROSS JOIN (VALUES (DATE '2026-09-01'), (DATE '2026-09-02')) day(date)
		ORDER BY p.tenant_id, p.id, day.date LIMIT 2830;
		UPDATE users.student_care_profiles SET sick = true, sick_since = '2026-09-19 08:00:00'
		WHERE membership_id IN (SELECT id FROM users.student_school_memberships ORDER BY id LIMIT 16)`)
	require.NoError(t, err)
	var profiles, history, sick int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM users.student_profiles),
		(SELECT count(*) FROM active.student_status_days),
		(SELECT count(*) FROM users.student_care_profiles WHERE sick)`).Scan(t.Context(), &profiles, &history, &sick))
	require.Equal(t, 1448, profiles)
	require.Equal(t, 2830, history)
	require.Equal(t, 16, sick)
	before := studentContractOwnerRows(t, db)
	var deadlocksBefore, deadlocksAfter int64
	require.NoError(t, db.NewRaw(`SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`).Scan(t.Context(), &deadlocksBefore))
	poolBefore := db.Stats()
	queries := testpkg.CaptureQueries(t, db)
	started := time.Now()
	require.NoError(t, contractStudentOwnerStorage(t.Context(), db))
	duration := time.Since(started)
	queries.Stop()
	poolAfter := db.Stats()
	require.NoError(t, db.NewRaw(`SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`).Scan(t.Context(), &deadlocksAfter))
	require.Equal(t, before, studentContractOwnerRows(t, db))
	var oldRelations int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM pg_class WHERE oid IN
		(to_regclass('users.students'), to_regclass('users.students_legacy'), to_regclass('users.expired_privacy_consents'))`).Scan(t.Context(), &oldRelations))
	require.Zero(t, oldRelations)
	require.ErrorContains(t, studentOwnerContractDown(t.Context(), db), "restore the verified pre-Contract backup")
	require.Equal(t, before, studentContractOwnerRows(t, db), "refused Down must not rewrite current owner data")
	t.Logf("Contract synthetic workload: schools=12 profiles=%d status_days=%d sick=%d duration=%s owner_and_history_snapshots_equal=true", profiles, history, sick, duration)
	t.Logf("Contract observed resources: bun_query_events=%d pool_wait_count_delta=%d pool_wait_duration_delta=%s database_deadlocks_before=%d database_deadlocks_after=%d successful_runs=1 unexpected_errors=0",
		queries.Total(), poolAfter.WaitCount-poolBefore.WaitCount, poolAfter.WaitDuration-poolBefore.WaitDuration, deadlocksBefore, deadlocksAfter)
}
