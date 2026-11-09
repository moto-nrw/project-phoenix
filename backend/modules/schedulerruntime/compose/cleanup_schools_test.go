package compose_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
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
	// These scenarios exercise school-specific retention with history explicitly on.
	_, err := db.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value) VALUES (?, 'gdpr.attendance_log_enabled', 'true'::jsonb)`, id)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE platform.schools
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
	disabled := seedCleanupSchool(t, admin, true, true, 30)
	_, err := admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, 'false'::jsonb)`, paused.id, "gdpr.data_cleanup_enabled")
	require.NoError(t, err)
	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	db := testpkg.NewDatabase(t, serve)
	now := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	_, err = admin.ExecContext(context.Background(), `UPDATE config.setting_values SET value = 'false'::jsonb WHERE tenant_id = ? AND setting_key = 'gdpr.attendance_log_enabled'`, disabled.id)
	require.NoError(t, err)
	exit := calendar.NewDate(2026, 10, 13).BerlinMidnight().Add(12 * time.Hour)
	_, err = admin.ExecContext(context.Background(), `UPDATE active.visits SET entry_time = ?, exit_time = ?, created_at = ? WHERE tenant_id = ?`, exit.Add(-time.Hour), exit, exit.Add(-time.Hour), disabled.id)
	require.NoError(t, err)
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
	for _, id := range []int64{disabled.expired, disabled.retained, disabled.windowed} {
		assert.False(t, visitExists(t, admin, id), "history-off inactive schools delete prior-day completions regardless of consent retention")
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

// Recording only the tenant at this consumer boundary keeps storage behavior
// in the owners' suites; this contract covers all three scheduler recovery jobs.
type recordingFileCleanup struct{ schools []int64 }

func (r *recordingFileCleanup) CleanupOrphanedStaffDocumentFiles(ctx context.Context) (int, error) {
	r.schools = append(r.schools, testpkg.TenantIDFromContext(ctx))
	return 0, nil
}
func (r *recordingFileCleanup) CleanupOrphanedStudentDocumentFiles(ctx context.Context) (int, error) {
	r.schools = append(r.schools, testpkg.TenantIDFromContext(ctx))
	return 0, nil
}
func (r *recordingFileCleanup) CleanupOrphanedFiles(ctx context.Context) (int, error) {
	r.schools = append(r.schools, testpkg.TenantIDFromContext(ctx))
	return 0, nil
}

func TestFileCleanupIncludesInactiveAndSoftDeletedSchools(t *testing.T) {
	t.Parallel()
	admin, deps := cleanupTestDependencies(t, nil)
	active := seedCleanupSchool(t, admin, false, false, 0)
	inactive := seedCleanupSchool(t, admin, true, false, 0)
	deleted := seedCleanupSchool(t, admin, true, true, 0)
	staff, students, files := &recordingFileCleanup{}, &recordingFileCleanup{}, &recordingFileCleanup{}
	deps.StaffDocumentCleaner, deps.StudentDocumentCleaner, deps.FileStoreCleaner = staff, students, files
	scheduler.NewFileCleanupTestRunner(deps)()
	for name, cleaner := range map[string]*recordingFileCleanup{"staff": staff, "students": students, "files and photos": files} {
		for _, school := range []int64{active.id, inactive.id, deleted.id} {
			assert.Contains(t, cleaner.schools, school, "%s must drain cleanup intents regardless of school status", name)
		}
		assert.NotContains(t, cleaner.schools, int64(0), "%s must always have a tenant scope", name)
	}
}

// failingWorkSessionCleanup fails the way the work-session close of a row
// checked in after its day failed chk_work_sessions_checkout_after_checkin
// (#6288): with a database error inside the school's cleanup transaction.
type failingWorkSessionCleanup struct {
	mu      sync.Mutex
	schools []int64
}

func (f *failingWorkSessionCleanup) CleanupOpenSessions(ctx context.Context) (int, error) {
	f.mu.Lock()
	f.schools = append(f.schools, testpkg.TenantIDFromContext(ctx))
	f.mu.Unlock()
	return 0, testpkg.FailInCurrentTransaction(ctx)
}

type failingFeedbackCleanup struct {
	mu      sync.Mutex
	schools []int64
}

func (f *failingFeedbackCleanup) DeleteExpired(ctx context.Context) (int, error) {
	f.mu.Lock()
	f.schools = append(f.schools, testpkg.TenantIDFromContext(ctx))
	f.mu.Unlock()
	return 0, testpkg.FailInCurrentTransaction(ctx)
}

// cleanupRunCount counts the recorded successes of the school's nightly
// cleanup. A failed run leaves a row with its failure streak only (#6327).
func cleanupRunCount(t *testing.T, admin *bun.DB, school int64) int {
	t.Helper()
	var runs int
	require.NoError(t, admin.NewRaw(`SELECT count(*) FROM platform.cleanup_runs
		WHERE tenant_id = ? AND job_id = 'visit-cleanup' AND succeeded_at IS NOT NULL`, school).
		Scan(context.Background(), &runs))
	return runs
}

// TestNightlyCleanupStepFailureKeepsVisitRetention pins the invariant of
// #6288: a database error in one cleanup step of a school undoes only that
// step. The visit retention (GDPR) of the same school still commits, and the
// run stays due because every step is required.
func TestNightlyCleanupStepFailureKeepsVisitRetention(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	school := seedCleanupSchool(t, admin, false, false, 10)
	workSessions := &failingWorkSessionCleanup{}
	deps.WorkSessionCleanup = workSessions

	scheduler.NewCleanupTestRunner(deps)()

	require.Contains(t, workSessions.schools, school.id)
	assert.False(t, visitExists(t, admin, school.expired), "the failed work-session step must not roll back the visit retention")
	assert.True(t, visitExists(t, admin, school.retained))
	assert.Zero(t, cleanupRunCount(t, admin, school.id), "a failed step keeps the run due")
}

// TestNightlyCleanupRequiredStepFailureStaysDue covers a failing required
// step: its writes are undone and the run stays due for the retry, but the
// visit retention that already succeeded is committed instead of waiting for
// the failing step to heal.
func TestNightlyCleanupRequiredStepFailureStaysDue(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	school := seedCleanupSchool(t, admin, false, false, 10)
	feedback := &failingFeedbackCleanup{}
	deps.FeedbackCleaner = feedback

	scheduler.NewCleanupTestRunner(deps)()

	require.Contains(t, feedback.schools, school.id)
	assert.False(t, visitExists(t, admin, school.expired), "the visit retention of the school commits")
	assert.Zero(t, cleanupRunCount(t, admin, school.id), "an incomplete run is not recorded as done")
}

// cleanupSteps are the nightly cleanup's steps under the names the scheduler
// logs them with (#6288).
var cleanupSteps = []string{"visits", "supervisors", "attendance", "work_sessions", "feedback", "rejected_enrollments", "retention"}

// stepProbe makes one cleanup step of one school fail with a database error
// (fail) or report a partial failure (partial). Every other step of that
// school runs a statement in the school's transaction and records its error,
// so a step whose failure escaped its savepoint shows up in the next step.
type stepProbe struct {
	school        int64
	fail, partial string
	mu            sync.Mutex
	results       map[string]error
}

func (p *stepProbe) mine(ctx context.Context) bool {
	return testpkg.TenantIDFromContext(ctx) == p.school
}

func (p *stepProbe) run(ctx context.Context, step string) error {
	var err error
	if step == p.fail {
		err = testpkg.FailInCurrentTransaction(ctx)
	} else {
		err = testpkg.PingCurrentTransaction(ctx)
	}
	p.mu.Lock()
	p.results[step] = err
	p.mu.Unlock()
	return err
}

func (p *stepProbe) result(step string) (error, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	err, ran := p.results[step]
	return err, ran
}

type probedPresence struct {
	ports.PresenceCleanup
	probe *stepProbe
}

func (c probedPresence) CleanupExpiredVisits(ctx context.Context) (*ports.ExpiredVisitCleanup, error) {
	result, err := c.PresenceCleanup.CleanupExpiredVisits(ctx)
	if err != nil || !c.probe.mine(ctx) {
		return result, err
	}
	if err := c.probe.run(ctx, "visits"); err != nil {
		return nil, err
	}
	result.Success = c.probe.partial != "visits"
	return result, nil
}

func (c probedPresence) CleanupStaleSupervisors(ctx context.Context) (*ports.StaleSupervisorCleanup, error) {
	result, err := c.PresenceCleanup.CleanupStaleSupervisors(ctx)
	if err != nil || !c.probe.mine(ctx) {
		return result, err
	}
	if err := c.probe.run(ctx, "supervisors"); err != nil {
		return nil, err
	}
	result.Success = c.probe.partial != "supervisors"
	return result, nil
}

func (c probedPresence) CleanupStaleAttendance(ctx context.Context) (*ports.StaleAttendanceCleanup, error) {
	result, err := c.PresenceCleanup.CleanupStaleAttendance(ctx)
	if err != nil || !c.probe.mine(ctx) {
		return result, err
	}
	if err := c.probe.run(ctx, "attendance"); err != nil {
		return nil, err
	}
	if c.probe.partial == "attendance" {
		result.Success, result.Errors = false, []string{"record failed"}
	}
	return result, nil
}

type probedCleaners struct{ probe *stepProbe }

func (c probedCleaners) CleanupOpenSessions(ctx context.Context) (int, error) {
	if !c.probe.mine(ctx) {
		return 0, nil
	}
	return 0, c.probe.run(ctx, "work_sessions")
}

func (c probedCleaners) DeleteExpired(ctx context.Context) (int, error) {
	if !c.probe.mine(ctx) {
		return 0, nil
	}
	return 0, c.probe.run(ctx, "feedback")
}

func (c probedCleaners) CleanupRejectedEnrollments(ctx context.Context) (int, int64, int64, error) {
	if !c.probe.mine(ctx) {
		return 0, 0, 0, nil
	}
	return 0, 0, 0, c.probe.run(ctx, "rejected_enrollments")
}

// lockedBuffer collects the scheduler's JSON log lines across goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) records(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var records []map[string]any
	for _, line := range bytes.Split(b.buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		records = append(records, record)
	}
	return records
}

type cleanupScenario struct {
	admin  *bun.DB
	school cleanupSchool
	probe  *stepProbe
	logs   *lockedBuffer
}

// runCleanupScenario runs one nightly cleanup of a school whose given step
// fails with a database error (fail) or reports a partial failure (partial).
func runCleanupScenario(t *testing.T, fail, partial string) cleanupScenario {
	t.Helper()
	now := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	school := seedCleanupSchool(t, admin, false, false, 10)
	probe := &stepProbe{school: school.id, fail: fail, partial: partial, results: map[string]error{}}
	logs := &lockedBuffer{}
	deps.Logger = slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	deps.ActiveCleanup = probedPresence{PresenceCleanup: deps.ActiveCleanup, probe: probe}
	cleaners := probedCleaners{probe: probe}
	deps.WorkSessionCleanup, deps.FeedbackCleaner, deps.EnrollmentRejectedCleanup = cleaners, cleaners, cleaners
	deps.RetentionSteps = []ports.RetentionStep{{Kind: "probe", Run: func(ctx context.Context) (ports.RetentionOutcome, error) {
		if !probe.mine(ctx) {
			return ports.RetentionOutcome{}, nil
		}
		return ports.RetentionOutcome{}, probe.run(ctx, "retention")
	}}}
	deps.RetentionLog = func(context.Context, string, ports.RetentionOutcome) error { return nil }

	scheduler.NewCleanupTestRunner(deps)()
	return cleanupScenario{admin: admin, school: school, probe: probe, logs: logs}
}

// Invariant 1: every step runs in its own savepoint. A database error in one
// step aborts nothing else: the next statement of every other step succeeds.
func TestNightlyCleanupInvariant_EachStepRunsInItsOwnSavepoint(t *testing.T) {
	t.Parallel()
	openSharedCleanupDatabase(t)
	for _, failing := range cleanupSteps {
		t.Run(failing, func(t *testing.T) {
			run := runCleanupScenario(t, failing, "")
			for _, other := range cleanupSteps {
				if other == failing {
					continue
				}
				err, ran := run.probe.result(other)
				require.True(t, ran, "step %s must run", other)
				assert.NoError(t, err, "step %s must find the transaction usable after %s failed", other, failing)
			}
		})
	}
}

// Invariant 2: a step's result is success or failure; a partial failure is a
// failure. It keeps the run due and is logged like any other failure.
func TestNightlyCleanupInvariant_PartialFailureIsAFailure(t *testing.T) {
	t.Parallel()
	openSharedCleanupDatabase(t)
	for _, partial := range []string{"visits", "supervisors", "attendance"} {
		t.Run(partial, func(t *testing.T) {
			run := runCleanupScenario(t, "", partial)
			assert.Zero(t, cleanupRunCount(t, run.admin, run.school.id), "a partial failure keeps the run due")
			assert.True(t, loggedStepFailure(t, run, partial), "a partial failure is logged with school and step")
		})
	}
}

// Invariant 3: a step's failure prevents no other step. The other steps run
// and their writes commit; the failed step's own writes roll back.
func TestNightlyCleanupInvariant_FailedStepPreventsNoOtherStep(t *testing.T) {
	t.Parallel()
	openSharedCleanupDatabase(t)
	for _, failing := range cleanupSteps {
		t.Run(failing, func(t *testing.T) {
			run := runCleanupScenario(t, failing, "")
			for _, other := range cleanupSteps {
				_, ran := run.probe.result(other)
				assert.True(t, ran, "step %s must run although %s failed", other, failing)
			}
			assert.Equal(t, failing == "visits", visitExists(t, run.admin, run.school.expired),
				"the visit deletion commits unless the visit step itself failed")
			assert.True(t, visitExists(t, run.admin, run.school.retained))
		})
	}
}

// Invariant 4: the run stays due while any required step fails; every step is
// required. Without a failure the run is recorded as done.
func TestNightlyCleanupInvariant_RunStaysDueWhileAStepFails(t *testing.T) {
	t.Parallel()
	openSharedCleanupDatabase(t)
	for _, failing := range append([]string{""}, cleanupSteps...) {
		t.Run("fail="+failing, func(t *testing.T) {
			run := runCleanupScenario(t, failing, "")
			want := 0
			if failing == "" {
				want = 1
			}
			assert.Equal(t, want, cleanupRunCount(t, run.admin, run.school.id))
		})
	}
}

// Invariant 5: every failure is logged with school and step.
func TestNightlyCleanupInvariant_EveryFailureIsLoggedWithSchoolAndStep(t *testing.T) {
	t.Parallel()
	openSharedCleanupDatabase(t)
	for _, failing := range cleanupSteps {
		t.Run(failing, func(t *testing.T) {
			run := runCleanupScenario(t, failing, "")
			assert.True(t, loggedStepFailure(t, run, failing), "the failure of %s is logged with school and step", failing)
		})
	}
}

func loggedStepFailure(t *testing.T, run cleanupScenario, step string) bool {
	t.Helper()
	for _, record := range run.logs.records(t) {
		tenantID, _ := record["tenant_id"].(float64)
		if record["level"] == "ERROR" && record["step"] == step && int64(tenantID) == run.school.id {
			return true
		}
	}
	return false
}

// openSharedCleanupDatabase opens the isolated database of the top-level test
// before its subtests, so it lives until the last of them. The subtests run
// one after another: each runs the nightly cleanup over every school of that
// database, and its probe only touches its own school.
func openSharedCleanupDatabase(t *testing.T) {
	t.Helper()
	testpkg.SetupIsolatedTestDB(t)
}
