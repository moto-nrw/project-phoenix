package compose_test

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Local flow evidence for #2692. This does not replace the accepted runtime
// checkpoint or claim a staging/production observation window.
func TestGroupRecoveryMigrationRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	var databaseVersion string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &databaseVersion))
	t.Logf("group-recovery-runtime environment: go=%s postgres=%s concurrency=1 warmup=5 samples_per_flow=30", runtime.Version(), databaseVersion)
	fixture := testpkg.CreateTestEndedActiveGroup(t, db, "Runtime")
	endGroup := func() { testpkg.EndTestActiveGroup(t, db, fixture) }
	ended := func() (groupEnded, supervisorEnded bool) { return testpkg.ActiveGroupEnded(t, db, fixture) }
	var observations []compose.Observation
	var measuring bool
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(event compose.Observation) {
		if measuring {
			observations = append(observations, event)
		}
	}})
	require.NoError(t, err)
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	var commits, rollbacks, retries int
	measuredCtx = tenant.WithUnitOfWorkObserver(measuredCtx, func(event tenant.UnitOfWorkEvent) {
		if !measuring || event.Kind != tenant.UnitOfWorkTransaction {
			return
		}
		retries += event.Retries
		if event.Result == tenant.UnitOfWorkCommitted {
			commits++
		}
		if event.Result == tenant.UnitOfWorkRolledBack {
			rollbacks++
		}
	})
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(context.Background(), &count))
		return count
	}
	abort := errors.New("injected recovery failure")
	restore := func(txCtx context.Context, now time.Time, fail bool) error {
		if err := module.LockOpenSupervisors(txCtx, fixture.GroupID); err != nil {
			return err
		}
		if err := module.LockSupervisors(txCtx, []int64{fixture.SupervisorID}); err != nil {
			return err
		}
		if err := module.RestoreGroup(txCtx, fixture.GroupID, now); err != nil {
			return err
		}
		if err := module.RestoreSupervisors(txCtx, []int64{fixture.SupervisorID}); err != nil {
			return err
		}
		if fail {
			return abort
		}
		return nil
	}
	for _, flow := range []string{"reopen-recovery", "rollback-after-supervisor-restore"} {
		t.Run(flow, func(t *testing.T) {
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			var duplicateRestoreRejections, injectedFailures int
			var stopSampling func() testpkg.RuntimeCheckpointLockSamples
			var deadlocksBefore int64
			for iteration := range 35 {
				if iteration == 5 {
					commits, rollbacks, retries = 0, 0, 0
					observations = nil
					deadlocksBefore = deadlocks()
					stopSampling = testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var waiting int
						err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
						return waiting, err
					})
					t.Cleanup(func() { _ = stopSampling() })
				}
				// Fixture reset is outside the measured query and transaction context.
				endGroup()
				counter.Reset()
				measuring = iteration >= 5
				now := time.Now()
				before, started := db.Stats(), time.Now()
				if flow == "rollback-after-supervisor-restore" {
					err := tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error { return restore(txCtx, now, true) })
					require.ErrorIs(t, err, abort)
					groupEnded, supervisorEnded := ended()
					require.True(t, groupEnded, "the group restore must roll back")
					require.True(t, supervisorEnded, "the supervisor restore must roll back")
				}
				require.NoError(t, tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error { return restore(txCtx, now, false) }))
				// A repeated restore of the same snapshot is rejected by the
				// unchanged-row check instead of silently succeeding.
				err := tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error { return module.RestoreGroup(txCtx, fixture.GroupID, now) })
				require.ErrorContains(t, err, "snapshot mismatch for active group")
				groupEnded, supervisorEnded := ended()
				require.False(t, groupEnded)
				require.False(t, supervisorEnded)
				elapsed, after := time.Since(started), db.Stats()
				measuring = false
				if iteration >= 5 {
					writes := counter.WriteRows()
					duration := float64(elapsed) / float64(time.Millisecond)
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: duration, Queries: counter.Total(), WriteRowsAffected: &writes, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
					durations = append(durations, duration)
					duplicateRestoreRejections++
					if flow != "reopen-recovery" {
						injectedFailures++
					}
				}
			}
			locks := stopSampling()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{
				"flow": flow, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28],
				"commits": commits, "rollbacks": rollbacks, "transaction_retries": retries,
				"injected_failures": injectedFailures, "unexpected_errors": 0,
				"duplicate_prevention_conflicts": duplicateRestoreRejections, "lock_samples": locks,
				"deadlocks": deadlocks() - deadlocksBefore, "operations": observations,
			})
			require.NoError(t, err)
			t.Logf("group-recovery-runtime: %s", report)
		})
	}
}
