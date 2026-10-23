package compose_test

import (
	"context"
	"errors"
	"testing"
	"time"

	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type cleanupSchool struct {
	id, expired, retained, windowed int64
}

func seedCleanupSchool(t *testing.T, db *bun.DB, inactive, deleted bool, retentionDays int) cleanupSchool {
	t.Helper()
	id, _ := testpkg.CreateTestTenant(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, id, "Test", "Frist", "2a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, id)
	insertVisit := func(daysAgo int) int64 {
		var visitID int64
		require.NoError(t, db.NewRaw(`INSERT INTO active.visits (tenant_id, student_id, active_group_id, entry_time, exit_time, created_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP - make_interval(days => ?, hours => 1),
			CURRENT_TIMESTAMP - make_interval(days => ?), CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`, id, student.ID, group.ID, daysAgo, daysAgo, daysAgo).
			Scan(context.Background(), &visitID))
		return visitID
	}
	school := cleanupSchool{id: id, expired: insertVisit(91), retained: insertVisit(5), windowed: insertVisit(20)}
	if retentionDays > 0 {
		_, err := db.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
			VALUES (?, ?, to_jsonb(?::integer))`, id, "gdpr.privacy_consent_retention_days", retentionDays)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(context.Background(), `UPDATE platform.schools
		SET active = ?, deleted_at = CASE WHEN ? THEN CURRENT_TIMESTAMP END WHERE id = ?`, !inactive, deleted, id)
	require.NoError(t, err)
	return school
}

func visitExists(t *testing.T, db *bun.DB, id int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM active.visits WHERE id = ?)`, id).Scan(context.Background(), &exists))
	return exists
}

func TestNightlyCleanupIncludesInactiveAndSoftDeletedSchools(t *testing.T) {
	t.Parallel()
	admin := testpkg.SetupIsolatedTestDB(t)
	active := seedCleanupSchool(t, admin, false, false, 0)
	inactive := seedCleanupSchool(t, admin, true, false, 10)
	deleted := seedCleanupSchool(t, admin, false, true, 10)
	paused := seedCleanupSchool(t, admin, true, false, 30)
	_, err := admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, 'false'::jsonb)`, paused.id, "gdpr.data_cleanup_enabled")
	require.NoError(t, err)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	db := testpkg.NewDatabase(t, serve)
	now := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	deps, err := owners.NewSchedulerCleanupTestDependencies(db, testpkg.TenantRuntime(t, serve), func() time.Time { return now })
	require.NoError(t, err)

	activeSchools := scheduler.ActiveSchoolsForTest(deps)
	assert.Contains(t, activeSchools, active.id)
	assert.NotContains(t, activeSchools, inactive.id)
	assert.NotContains(t, activeSchools, deleted.id)

	scheduler.NewCleanupTestRunner(deps)()

	for _, school := range []cleanupSchool{active, inactive, deleted} {
		assert.False(t, visitExists(t, admin, school.expired), "school %d: overdue visits must be deleted regardless of school status", school.id)
		assert.True(t, visitExists(t, admin, school.retained), "school %d: visits inside its retention window must stay", school.id)
		assert.Equal(t, school.id == active.id, visitExists(t, admin, school.windowed), "the school-specific retention window is authoritative")
		var successes int
		require.NoError(t, admin.NewRaw(`SELECT count(*) FROM platform.cleanup_runs WHERE tenant_id = ? AND job_id = 'visit-cleanup'`, school.id).
			Scan(context.Background(), &successes))
		assert.Equal(t, 1, successes, "school %d: success must commit with the deletion", school.id)
	}
	assert.True(t, visitExists(t, admin, paused.expired), "another school's paused cleanup must not be bypassed by RLS")
}

func cleanupTestDependencies(t *testing.T, now func() time.Time) (*bun.DB, ports.WorkerDependencies) {
	t.Helper()
	admin := testpkg.SetupIsolatedTestDB(t)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	db := testpkg.NewDatabase(t, serve)
	deps, err := owners.NewSchedulerCleanupTestDependencies(db, testpkg.TenantRuntime(t, serve), now)
	require.NoError(t, err)
	return admin, deps
}

func lastCleanupSuccess(t *testing.T, admin *bun.DB, school int64) time.Time {
	t.Helper()
	var last time.Time
	require.NoError(t, admin.NewRaw(`SELECT succeeded_at FROM platform.cleanup_runs WHERE tenant_id = ? AND job_id = 'visit-cleanup'`, school).
		Scan(context.Background(), &last))
	return last
}

func seedMissedCleanup(t *testing.T, admin *bun.DB, school int64, last time.Time) {
	t.Helper()
	_, err := admin.ExecContext(context.Background(), `INSERT INTO platform.cleanup_runs (tenant_id, job_id, succeeded_at)
		VALUES (?, 'visit-cleanup', ?)`, school, last)
	require.NoError(t, err)
}

func TestNightlyCleanupCatchesUpInactiveAndDeletedSchoolsAcrossRestart(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.October, 14, 14, 37, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	schools := []cleanupSchool{
		seedCleanupSchool(t, admin, true, false, 10),
		seedCleanupSchool(t, admin, true, true, 0),
	}
	for _, school := range schools {
		seedMissedCleanup(t, admin, school.id, now.Add(-48*time.Hour))
	}
	scheduler.NewCleanupTestRunner(deps)()
	for _, school := range schools {
		assert.False(t, visitExists(t, admin, school.expired), "a missed run must catch up outside the configured minute")
		assert.True(t, lastCleanupSuccess(t, admin, school.id).Equal(now))
	}

	now = now.Add(time.Minute)
	scheduler.NewCleanupTestRunner(deps)()
	for _, school := range schools {
		assert.True(t, lastCleanupSuccess(t, admin, school.id).Equal(now.Add(-time.Minute)), "a restarted process must not repeat today's successful run")
	}
}

type lateFailingCleanup struct {
	ports.PresenceCleanup
	school int64
}

func (c lateFailingCleanup) CleanupExpiredVisits(ctx context.Context) (*ports.ExpiredVisitCleanup, error) {
	result, err := c.PresenceCleanup.CleanupExpiredVisits(ctx)
	if err == nil && testpkg.TenantIDFromContext(ctx) == c.school {
		return result, errors.New("injected cleanup failure after deletions")
	}
	return result, err
}

func TestCleanupSettingsFailureDoesNotCommitOrStopOtherSchools(t *testing.T) {
	t.Parallel()
	testCleanupSchoolFailure(t, "settings")
}

func TestCleanupDeletionFailureDoesNotCommitOrStopOtherSchools(t *testing.T) {
	t.Parallel()
	testCleanupSchoolFailure(t, "after-deletions")
}

func testCleanupSchoolFailure(t *testing.T, failure string) {
	t.Helper()
	now := time.Date(2026, time.October, 14, 14, 37, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	failed := seedCleanupSchool(t, admin, true, false, 10)
	healthy := seedCleanupSchool(t, admin, true, true, 10)
	_, err := admin.ExecContext(context.Background(), `UPDATE platform.schools
        SET name = CASE WHEN id = ? THEN 'A-failing-cleanup' ELSE 'Z-healthy-cleanup' END WHERE id IN (?, ?)`, failed.id, failed.id, healthy.id)
	require.NoError(t, err)
	previous := now.Add(-48 * time.Hour)
	seedMissedCleanup(t, admin, failed.id, previous)
	seedMissedCleanup(t, admin, healthy.id, previous)
	goodCleanup := deps.ActiveCleanup
	if failure == "settings" {
		_, err := admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
					VALUES (?, ?, '"unreadable"'::jsonb)`, failed.id, "gdpr.data_cleanup_timeout_minutes")
		require.NoError(t, err)
	} else {
		deps.ActiveCleanup = lateFailingCleanup{PresenceCleanup: goodCleanup, school: failed.id}
	}
	var outcome string
	deps.Tracer.Run = func(_ ports.JobID, result string, _ time.Duration) { outcome = result }
	scheduler.NewCleanupTestRunner(deps)()
	assert.Equal(t, "failed", outcome, "a partial run must not report overall success")
	assert.True(t, visitExists(t, admin, failed.expired), "a failed school's deletions must roll back")
	assert.True(t, lastCleanupSuccess(t, admin, failed.id).Equal(previous))
	assert.False(t, visitExists(t, admin, healthy.expired), "another school's cleanup must still complete")
	assert.True(t, lastCleanupSuccess(t, admin, healthy.id).Equal(now))
	_, err = admin.ExecContext(context.Background(), `UPDATE config.setting_values SET value = '30'::jsonb
				WHERE tenant_id = ? AND setting_key = ?`, failed.id, "gdpr.data_cleanup_timeout_minutes")
	require.NoError(t, err)
	deps.ActiveCleanup = goodCleanup
	now = now.Add(time.Hour)
	scheduler.NewCleanupTestRunner(deps)()
	assert.Equal(t, "completed", outcome)
	assert.False(t, visitExists(t, admin, failed.expired), "the failed run stays due across a restart")
}
