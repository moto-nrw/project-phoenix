package compose_test

import (
	"context"
	"testing"
	"time"

	scheduler "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The sign-in events of a demo visit (IP address, user agent) are not erased
// when the demo school is retired; the 90-day window of the security log
// applies (#5543). The demo environment enforces it only through the nightly
// run, so the run must still reach a school the demo process has soft-deleted.
func TestNightlyCleanupDeletesSignInEventsOfARetiredDemoSchoolAfterNinetyDays(t *testing.T) {
	t.Parallel()
	admin := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	demoSchool, _ := testpkg.CreateTestTenant(t, admin)
	const slug = "ogs-demo-5543"
	_, err := admin.ExecContext(ctx, `INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status)
		VALUES (?, ?, '{}'::jsonb, 'ready')`, slug, demoSchool)
	require.NoError(t, err)
	visitor := testpkg.CreateTestAccount(t, admin, "demo-visit-5543@example.com").ID
	// Another school whose nightly run the moto team has halted: the run of
	// the demo school must never delete its overdue events.
	pausedSchool, _ := testpkg.CreateTestTenant(t, admin)
	_, err = admin.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.data_cleanup_enabled', 'false'::jsonb)`, pausedSchool)
	require.NoError(t, err)
	pausedUser := testpkg.CreateTestAccount(t, admin, "paused-school-5543@example.com").ID

	// The window is 90 times 24 hours before the run, evaluated in UTC
	// (records.RetentionDaysCutoff, migration 1.15.606), not 90 Berlin
	// calendar days. Events one hour either side of it keep the test
	// independent of a clock change, so a window of 89 or 91 days fails it.
	const windowHours = 90 * 24
	const insertSignIn = `INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, user_agent, created_at)
		VALUES (?, ?, 'login', true, '192.0.2.10', 'Demo-Browser', CURRENT_TIMESTAMP - make_interval(hours => ?)) RETURNING id`
	signInEvent := func(school, account int64, hoursAgo int) int64 {
		var id int64
		require.NoError(t, admin.NewRaw(insertSignIn, school, account, hoursAgo).Scan(ctx, &id))
		return id
	}
	expired := signInEvent(demoSchool, visitor, windowHours+1)
	kept := signInEvent(demoSchool, visitor, windowHours-1)
	pausedExpired := signInEvent(pausedSchool, pausedUser, windowHours+1)

	// The demo process retires a school once no access points to it: the
	// same database function soft-deletes it and erases its visitor.
	var hidden int
	require.NoError(t, admin.NewRaw(`SELECT hidden FROM platform.retire_expired_demo_schools(ARRAY[?]::text[])`, slug).Scan(ctx, &hidden))
	require.Equal(t, 1, hidden, "the demo school without an access is retired")
	var softDeleted bool
	require.NoError(t, admin.NewRaw(`SELECT deleted_at IS NOT NULL FROM platform.schools WHERE id = ?`, demoSchool).Scan(ctx, &softDeleted))
	require.True(t, softDeleted, "retiring soft-deletes the demo school")
	require.True(t, authEventExists(t, admin, expired), "retiring leaves the sign-in events to their window")

	serve := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, serve.Close()) })
	now := func() time.Time { return time.Date(2026, time.October, 14, 2, 0, 0, 0, calendar.Berlin) }
	deps, err := owners.NewSchedulerFileRetentionTestDependencies(testpkg.NewDatabase(t, serve), testpkg.TenantRuntime(t, serve), now, t.TempDir())
	require.NoError(t, err)
	var workerErr error
	deps.Tracer.Failure = func(_ context.Context, _, _ string, err error) { workerErr = err }
	scheduler.NewCleanupTestRunner(deps)()
	require.NoError(t, workerErr)

	assert.False(t, authEventExists(t, admin, expired), "the nightly run deletes a retired demo school's sign-in event older than 90 days")
	assert.True(t, authEventExists(t, admin, kept), "a sign-in event inside the 90-day window stays")
	assert.Equal(t, []int{1}, loggedAuthEventDeletions(t, admin, demoSchool), "the deletion is logged for the retired demo school")
	assert.True(t, authEventExists(t, admin, pausedExpired), "the run of the demo school never deletes another school's events")
	assert.Empty(t, loggedAuthEventDeletions(t, admin, pausedSchool), "a halted school gets no deletion-log entry")
}

func loggedAuthEventDeletions(t *testing.T, admin *bun.DB, school int64) []int {
	t.Helper()
	var logged []int
	require.NoError(t, admin.NewRaw(`SELECT records_deleted FROM audit.data_deletions
		WHERE tenant_id = ? AND deletion_type = 'auth_event_retention' ORDER BY id`, school).Scan(context.Background(), &logged))
	return logged
}

func authEventExists(t *testing.T, admin *bun.DB, id int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, admin.NewRaw(`SELECT EXISTS (SELECT 1 FROM audit.auth_events WHERE id = ?)`, id).Scan(context.Background(), &exists))
	return exists
}
