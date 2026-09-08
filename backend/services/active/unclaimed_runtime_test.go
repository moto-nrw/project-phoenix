package active_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestUnclaimedClaimRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	service := createActiveService(t, db)
	var roomID int64
	require.NoError(t, db.NewRaw("INSERT INTO facilities.rooms (tenant_id, name) VALUES (?, 'Schulhof') RETURNING id", testpkg.Tenant(t)).Scan(ctx, &roomID))
	activity := testpkg.CreateTestActivityGroup(t, db, "Unclaimed runtime")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, roomID)
	staff := testpkg.CreateTestStaff(t, db, "Unclaimed", "Runtime")
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	var samples []testpkg.RuntimeCheckpointSample
	var durations []float64
	stop := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
		var waiting int
		err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
		return waiting, err
	})
	t.Cleanup(func() { _ = stop() })
	for iteration := range 35 {
		counter.Reset()
		before, started := db.Stats(), time.Now()
		var supervisionID int64
		err := tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error {
			rows, err := service.GetUnclaimedActiveGroups(txCtx)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Equal(t, group.ID, rows[0].ID)
			claim, err := service.ClaimActiveGroup(txCtx, group.ID, staff.ID, "supervisor")
			require.NoError(t, err)
			supervisionID = claim.ID
			rows, err = service.GetUnclaimedActiveGroups(txCtx)
			require.NoError(t, err)
			require.Empty(t, rows)
			return nil
		})
		elapsed, after := time.Since(started), db.Stats()
		require.NoError(t, err)
		if iteration >= 5 {
			duration := float64(elapsed) / float64(time.Millisecond)
			samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: duration, Queries: counter.Total(), PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
			durations = append(durations, duration)
		}
		// Reset only the owned supervision outside the measured production path.
		_, err = db.NewRaw("DELETE FROM active.group_supervisors WHERE tenant_id = ? AND id = ?", testpkg.Tenant(t), supervisionID).Exec(ctx)
		require.NoError(t, err)
	}
	locks := stop()
	require.Empty(t, locks.Error)
	slices.Sort(durations)
	report, err := json.Marshal(map[string]any{"flow": "unclaimed-list-claim-list", "concurrency": 1, "warmup": 5, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "unexpected_errors": 0, "lock_samples": locks})
	require.NoError(t, err)
	t.Logf("unclaimed-runtime: %s", report)
}
