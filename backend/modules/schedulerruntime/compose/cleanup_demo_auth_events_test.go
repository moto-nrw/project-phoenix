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

	const insertSignIn = `INSERT INTO audit.auth_events (tenant_id, account_id, event_type, success, ip_address, user_agent, created_at)
		VALUES (?, ?, 'login', true, '192.0.2.10', 'Demo-Browser', CURRENT_TIMESTAMP - make_interval(days => ?)) RETURNING id`
	signInEvent := func(daysAgo int) int64 {
		var id int64
		require.NoError(t, admin.NewRaw(insertSignIn, demoSchool, visitor, daysAgo).Scan(ctx, &id))
		return id
	}
	expired, kept := signInEvent(91), signInEvent(89)

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
	var logged []int
	require.NoError(t, admin.NewRaw(`SELECT records_deleted FROM audit.data_deletions
		WHERE tenant_id = ? AND deletion_type = 'auth_event_retention' ORDER BY id`, demoSchool).Scan(ctx, &logged))
	assert.Equal(t, []int{1}, logged, "the deletion is logged for the retired demo school")
}

func authEventExists(t *testing.T, admin *bun.DB, id int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, admin.NewRaw(`SELECT EXISTS (SELECT 1 FROM audit.auth_events WHERE id = ?)`, id).Scan(context.Background(), &exists))
	return exists
}
