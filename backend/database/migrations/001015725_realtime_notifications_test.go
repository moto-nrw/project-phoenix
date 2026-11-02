package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestRealtimeNotificationGrants pins 1.15.725 (#5504): the copies are school
// data under forced row-level security that are written once and deleted,
// never changed.
func TestRealtimeNotificationGrants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	privilege := func(action string) bool {
		var granted bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'platform.realtime_notifications', ?)`, action).Scan(ctx, &granted))
		return granted
	}
	assert.True(t, privilege("SELECT"))
	assert.True(t, privilege("INSERT"))
	assert.True(t, privilege("DELETE"))
	assert.False(t, privilege("UPDATE"), "a stored notification is never changed")

	var forced bool
	require.NoError(t, db.NewRaw(`SELECT relforcerowsecurity FROM pg_class WHERE oid = 'platform.realtime_notifications'::regclass`).Scan(ctx, &forced))
	assert.True(t, forced)
}

// TestRealtimeNotificationsAreInvisibleToOtherSchools reads a stored copy as
// the tenant role of another school.
func TestRealtimeNotificationsAreInvisibleToOtherSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school, other := testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.realtime_notifications (tenant_id, scope, content) VALUES (?, 'tenant', '{}') RETURNING id`, school).Scan(ctx, &id))
	count := func(tenantID int64) int {
		var visible int
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx bun.Tx) error {
			return tx.NewRaw(`SELECT count(*) FROM platform.realtime_notifications WHERE id = ?`, id).Scan(ctx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 1, count(school))
	assert.Zero(t, count(other), "another school read the notification")
}

// TestRealtimeNotificationsMigrationRoundTrip rolls 1.15.725 back and
// forward: the down drops the table and restores the 1.15.724 erasure plan,
// the up puts the table back into it.
func TestRealtimeNotificationsMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	require.NoError(t, realtimeNotificationsDown(ctx, db))
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('platform.realtime_notifications') IS NOT NULL`).Scan(ctx, &exists))
	assert.False(t, exists)
	for _, entry := range readErasurePlan(t, db) {
		assert.NotEqual(t, "platform.realtime_notifications", entry.Name)
	}

	require.NoError(t, realtimeNotificationsUp(ctx, db))
	names := map[string]bool{}
	for _, entry := range readErasurePlan(t, db) {
		names[entry.Name] = true
	}
	assert.True(t, names["platform.realtime_notifications"])
}
