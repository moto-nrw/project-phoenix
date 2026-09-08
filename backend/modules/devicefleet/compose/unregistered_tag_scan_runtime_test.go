package compose_test

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Bounded local evidence for #2678, not a replacement for a runtime checkpoint.
func TestUnregisteredTagScanRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	rooms, err := repositories.NewFacilities(db)
	require.NoError(t, err)
	var measuring, observing bool
	var operations []map[string]any
	fleet, err := compose.New(compose.Dependencies{DB: db, Rooms: rooms, Observe: func(event compose.Observation) {
		if observing {
			operations = append(operations, map[string]any{"operation": event.Operation, "error_code": devicefleet.ErrorCode(event.Err), "queries": event.Stats.Queries, "rows": event.Stats.Rows, "duration_ms": float64(event.Duration) / float64(time.Millisecond)})
		}
	}})
	require.NoError(t, err)
	device := testpkg.CreateTestDevice(t, db, "scan-runtime")
	operatorID := newScanOperator(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	var commits, rollbacks, retries int
	measuredCtx = tenant.WithUnitOfWorkObserver(measuredCtx, func(event tenant.UnitOfWorkEvent) {
		if !observing || event.Kind != tenant.UnitOfWorkTransaction {
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
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	t.Logf("scan-runtime environment: go=%s postgres=%s concurrency=1 warmup=5 samples_per_operation=30", runtime.Version(), version)
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	deadlocksBefore := deadlocks()
	stop := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
		var waiting int
		err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
		return waiting, err
	})
	t.Cleanup(func() { _ = stop() })
	samples := make(map[string][]testpkg.RuntimeCheckpointSample)
	measure := func(name string, expected error, run func(context.Context) error) {
		counter.Reset()
		observing = measuring
		before, started := db.Stats(), time.Now()
		err := run(measuredCtx)
		elapsed, after := time.Since(started), db.Stats()
		observing = false
		if expected == nil {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, expected)
		}
		if measuring {
			rows, statements := counter.Rows()
			samples[name] = append(samples[name], testpkg.RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), RowsAffected: rows, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
		}
	}
	abort := errors.New("injected failure after authoritative scan write")
	cutoff := time.Now().Add(-devicefleet.UnregisteredTagScanRetentionDays * 24 * time.Hour)
	for iteration := range 35 {
		measuring = iteration >= 5
		input := devicefleet.RecordUnregisteredTagScan{TagUID: "RUNTIME-SCAN", DeviceID: &device.ID, ScannedAt: cutoff.Add(-24 * time.Hour)}
		var scan, aborted devicefleet.UnregisteredTagScan
		measure("record_rollback", abort, func(ctx context.Context) error {
			return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				aborted, err = fleet.RecordUnregisteredTagScan(txCtx, input)
				require.NoError(t, err)
				return abort
			})
		})
		_, err = fleet.FindUnregisteredTagScan(ctx, aborted.ID)
		require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound)
		measure("record", nil, func(ctx context.Context) error {
			scan, err = fleet.RecordUnregisteredTagScan(ctx, input)
			return err
		})
		measure("find", nil, func(ctx context.Context) error {
			found, err := fleet.FindUnregisteredTagScan(ctx, scan.ID)
			require.NotNil(t, found.DeviceIdentifier)
			return err
		})
		measure("list", nil, func(ctx context.Context) error {
			rows, err := fleet.ListUnregisteredTagScans(ctx, devicefleet.UnregisteredTagScanFilter{})
			require.Len(t, rows, 1)
			return err
		})
		resolve := devicefleet.ResolveUnregisteredTagScan{ID: scan.ID, OperatorID: operatorID}
		measure("resolve_rollback", abort, func(ctx context.Context) error {
			return testpkg.WithinAdminContext(t, ctx, db, func(txCtx context.Context) error {
				_, err := fleet.ResolveUnregisteredTagScan(txCtx, resolve)
				require.NoError(t, err)
				return abort
			})
		})
		found, err := fleet.FindUnregisteredTagScan(ctx, scan.ID)
		require.NoError(t, err)
		require.Nil(t, found.ResolvedAt)
		resolveAsOperator := func(ctx context.Context) error {
			return testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
				_, err := fleet.ResolveUnregisteredTagScan(adminCtx, resolve)
				return err
			})
		}
		measure("resolve", nil, resolveAsOperator)
		measure("resolve_duplicate", devicefleet.ErrUnregisteredTagScanResolved, resolveAsOperator)
		measure("retention_rollback", abort, func(ctx context.Context) error {
			return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				deleted, err := fleet.DeleteExpiredUnregisteredTagScans(txCtx, cutoff)
				require.NoError(t, err)
				require.EqualValues(t, 1, deleted)
				return abort
			})
		})
		_, err = fleet.FindUnregisteredTagScan(ctx, scan.ID)
		require.NoError(t, err, "retention rollback restores the authoritative row")
		measure("retention", nil, func(ctx context.Context) error {
			deleted, err := fleet.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
			require.EqualValues(t, 1, deleted)
			return err
		})
		_, err = fleet.FindUnregisteredTagScan(ctx, scan.ID)
		require.ErrorIs(t, err, devicefleet.ErrUnregisteredTagScanNotFound)
		deleted, err := fleet.DeleteExpiredUnregisteredTagScans(ctx, cutoff)
		require.NoError(t, err)
		require.Zero(t, deleted, "retry after successful retention is idempotent")
	}
	measuring = false
	locks := stop()
	require.Empty(t, locks.Error)
	percentiles := make(map[string]map[string]float64)
	for name, values := range samples {
		durations := make([]float64, 0, len(values))
		for _, value := range values {
			durations = append(durations, value.DurationMS)
		}
		slices.Sort(durations)
		percentiles[name] = map[string]float64{"p50_ms": durations[14], "p95_ms": durations[28]}
	}
	report, err := json.Marshal(map[string]any{"samples": samples, "percentiles": percentiles, "operations": operations, "commits": commits, "rollbacks": rollbacks, "serialization_retries": retries, "duplicate_prevention_conflicts": 30, "injected_failures": 90, "unexpected_errors": 0, "deadlocks": deadlocks() - deadlocksBefore, "lock_samples": locks})
	require.NoError(t, err)
	t.Logf("scan-runtime: %s", report)
}
