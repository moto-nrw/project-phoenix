package compose_test

import (
	"context"
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

func cleanupRunCount(t *testing.T, admin *bun.DB, school int64) int {
	t.Helper()
	var runs int
	require.NoError(t, admin.NewRaw(`SELECT count(*) FROM platform.cleanup_runs WHERE tenant_id = ? AND job_id = 'visit-cleanup'`, school).
		Scan(context.Background(), &runs))
	return runs
}

// TestNightlyCleanupStepFailureKeepsVisitRetention pins the invariant of
// #6288: a database error in one best-effort cleanup step of a school undoes
// only that step. The visit retention (GDPR) of the same school still
// commits, and the run counts as done.
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
	assert.Equal(t, 1, cleanupRunCount(t, admin, school.id), "a best-effort step failure keeps the run successful")
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
