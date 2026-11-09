package compose_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// schoolFailingFeedback fails the feedback step of one school while fail is
// set; other schools pass.
type schoolFailingFeedback struct {
	mu     sync.Mutex
	school int64
	fail   bool
}

func (f *schoolFailingFeedback) DeleteExpired(ctx context.Context) (int, error) {
	f.mu.Lock()
	fail := f.fail && testpkg.TenantIDFromContext(ctx) == f.school
	f.mu.Unlock()
	if fail {
		return 0, testpkg.FailInCurrentTransaction(ctx)
	}
	return 0, nil
}

func (f *schoolFailingFeedback) heal() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = false
}

type storedCleanupRun struct {
	SucceededAt *time.Time `bun:"succeeded_at"`
	Failures    int        `bun:"consecutive_failures"`
}

func storedRun(t *testing.T, admin *bun.DB, school int64) storedCleanupRun {
	t.Helper()
	var run storedCleanupRun
	require.NoError(t, admin.NewRaw(`SELECT succeeded_at, consecutive_failures FROM platform.cleanup_runs
		WHERE tenant_id = ? AND job_id = 'visit-cleanup'`, school).Scan(context.Background(), &run))
	return run
}

func visitDeletionAudits(t *testing.T, admin *bun.DB, school int64) int {
	t.Helper()
	var audits int
	require.NoError(t, admin.NewRaw(`SELECT count(*) FROM audit.data_deletions WHERE tenant_id = ?`, school).
		Scan(context.Background(), &audits))
	return audits
}

// TestNightlyCleanupFailureStreakIsPerSchool covers #6327 against the
// database and the RLS roles: a school whose required step keeps failing
// builds up a stored failure streak, backs off, alerts once per failed run
// from the third failure on and recovers with its first success. Every run
// builds a new scheduler, so the streak also survives a restart. Another
// school is neither delayed nor marked, and the visit retention of the
// failing school commits once, without duplicate deletion audits (#6288).
func TestNightlyCleanupFailureStreakIsPerSchool(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	now := start
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	failing := seedCleanupSchool(t, admin, false, false, 10)
	healthy := seedCleanupSchool(t, admin, false, false, 10)
	feedback := &schoolFailingFeedback{school: failing.id, fail: true}
	deps.FeedbackCleaner = feedback
	logs := &lockedBuffer{}
	deps.Logger = slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	// run starts a new Worker after the given time since the first run.
	run := func(after time.Duration) {
		now = start.Add(after)
		scheduler.NewCleanupTestRunner(deps)()
	}
	alerts := func(msg string) []map[string]any {
		var matched []map[string]any
		for _, record := range logs.records(t) {
			tenantID, _ := record["tenant_id"].(float64)
			if record["msg"] == msg && int64(tenantID) == failing.id {
				matched = append(matched, record)
			}
		}
		return matched
	}

	run(0)
	assert.False(t, visitExists(t, admin, failing.expired), "the visit retention of the failing school commits")
	audits := visitDeletionAudits(t, admin, failing.id)
	require.Positive(t, audits)
	assert.Equal(t, storedCleanupRun{Failures: 1}, storedRun(t, admin, failing.id), "a failed first run is stored as a failure, not as a success")
	healthyRun := storedRun(t, admin, healthy.id)
	require.NotNil(t, healthyRun.SucceededAt)
	assert.True(t, healthyRun.SucceededAt.Equal(start))
	assert.Zero(t, healthyRun.Failures, "the other school keeps no streak")

	run(59 * time.Minute)
	assert.Equal(t, 1, storedRun(t, admin, failing.id).Failures, "the restarted Worker waits one hour")
	run(time.Hour)
	assert.Equal(t, 2, storedRun(t, admin, failing.id).Failures)
	run(3*time.Hour - time.Minute)
	assert.Equal(t, 2, storedRun(t, admin, failing.id).Failures, "the restarted Worker waits two hours")
	assert.Empty(t, alerts("nightly cleanup failing repeatedly"), "two failures stay below the threshold")

	run(3 * time.Hour)
	assert.Equal(t, 3, storedRun(t, admin, failing.id).Failures)
	got := alerts("nightly cleanup failing repeatedly")
	require.Len(t, got, 1, "the third failed run alerts once")
	assert.Equal(t, "feedback", got[0]["failed_steps"])
	assert.Equal(t, "visit-cleanup", got[0]["job_id"])
	assert.InDelta(t, 3, got[0]["consecutive_failures"], 0)
	assert.Equal(t, audits, visitDeletionAudits(t, admin, failing.id), "retries write no duplicate deletion audits")
	assert.True(t, storedRun(t, admin, healthy.id).SucceededAt.Equal(start), "the healthy school is not rerun")

	feedback.heal()
	run(7 * time.Hour)
	recovered := storedRun(t, admin, failing.id)
	require.NotNil(t, recovered.SucceededAt)
	assert.True(t, recovered.SucceededAt.Equal(now))
	assert.Zero(t, recovered.Failures, "the success ends the streak")
	require.Len(t, alerts("nightly cleanup recovered"), 1)
	assert.Len(t, alerts("nightly cleanup failing repeatedly"), 1, "a success raises no alert")
}

// TestNightlyCleanupStoresTheStreakOfAnUnreadableSwitch: a school whose
// cleanup switch cannot be read fails every due run, and each tick of it
// used to roll back. Its streak must reach the database anyway, across
// restarts.
func TestNightlyCleanupStoresTheStreakOfAnUnreadableSwitch(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin)
	now := start
	admin, deps := cleanupTestDependencies(t, func() time.Time { return now })
	school := seedCleanupSchool(t, admin, false, false, 10)
	_, err := admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.data_cleanup_enabled', '"unreadable"'::jsonb)`, school.id)
	require.NoError(t, err)

	for _, after := range []time.Duration{0, time.Hour, 3 * time.Hour} {
		now = start.Add(after)
		scheduler.NewCleanupTestRunner(deps)()
	}

	assert.Equal(t, storedCleanupRun{Failures: 3}, storedRun(t, admin, school.id))
	assert.True(t, visitExists(t, admin, school.expired), "nothing runs while the switch cannot be read")
}
